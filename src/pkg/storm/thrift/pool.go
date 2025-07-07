package thrift

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
)

var (
	ErrPoolClosed    = errors.New("connection pool is closed")
	ErrPoolExhausted = errors.New("connection pool exhausted")
	ErrConnClosed    = errors.New("connection is closed")
)

// PooledConnection wraps a Thrift connection with pool metadata
type PooledConnection struct {
	conn      *thrift.TSocket
	transport thrift.TTransport
	protocol  thrift.TProtocol
	client    *NimbusClient
	pool      *ConnectionPool
	createdAt time.Time
	lastUsed  time.Time
	inUse     bool
	mu        sync.Mutex
}

// Close returns the connection to the pool
func (pc *PooledConnection) Close() error {
	pc.mu.Lock()
	pool := pc.pool
	pc.mu.Unlock()

	if pool == nil {
		// Connection has been invalidated, close it directly
		pc.mu.Lock()
		transport := pc.transport
		pc.mu.Unlock()

		if transport != nil {
			return transport.Close()
		}
		return nil
	}

	// Return to pool
	pool.put(pc)
	return nil
}

// Invalidate marks the connection as invalid and removes it from the pool
func (pc *PooledConnection) Invalidate() {
	pc.mu.Lock()
	pool := pc.pool
	pc.pool = nil
	transport := pc.transport
	pc.transport = nil
	pc.mu.Unlock()

	// Close transport if needed
	if transport != nil {
		transport.Close()
	}

	// Update pool stats if this was from a pool
	if pool != nil {
		pool.mu.Lock()
		pool.created--
		pool.mu.Unlock()
	}
}

// IsValid checks if the connection is still valid
func (pc *PooledConnection) IsValid() bool {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	return pc.isValidLocked()
}

// isValidLocked checks validity without locking (must be called with lock held)
func (pc *PooledConnection) isValidLocked() bool {
	if pc.transport == nil || !pc.transport.IsOpen() {
		return false
	}

	// Check if connection has been idle too long
	if pc.pool != nil && pc.pool.config.MaxIdleTime > 0 {
		if time.Since(pc.lastUsed) > pc.pool.config.MaxIdleTime {
			return false
		}
	}

	return true
}

// ConnectionPoolConfig defines configuration for the connection pool
type ConnectionPoolConfig struct {
	// Maximum number of connections in the pool
	MaxConnections int
	// Minimum number of idle connections
	MinIdleConnections int
	// Maximum idle time before connection is closed
	MaxIdleTime time.Duration
	// Maximum lifetime of a connection
	MaxLifetime time.Duration
	// Connection configuration
	ClientConfig *ThriftClientConfig
}

// DefaultConnectionPoolConfig returns default pool configuration
func DefaultConnectionPoolConfig() *ConnectionPoolConfig {
	return &ConnectionPoolConfig{
		MaxConnections:     10,
		MinIdleConnections: 2,
		MaxIdleTime:        5 * time.Minute,
		MaxLifetime:        30 * time.Minute,
		ClientConfig:       DefaultThriftClientConfig(),
	}
}

// ConnectionPool manages a pool of Thrift connections
type ConnectionPool struct {
	config      *ConnectionPoolConfig
	connections chan *PooledConnection
	factory     ConnectionFactory
	mu          sync.RWMutex
	closed      bool
	created     int
	ctx         context.Context
	cancel      context.CancelFunc
}

// ConnectionFactory creates new Thrift connections
type ConnectionFactory func(config *ThriftClientConfig) (*PooledConnection, error)

// DefaultConnectionFactory creates standard Thrift connections
func DefaultConnectionFactory(config *ThriftClientConfig) (*PooledConnection, error) {
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	cfg := &thrift.TConfiguration{
		ConnectTimeout: config.ConnectionTimeout,
		SocketTimeout:  config.RequestTimeout,
	}

	socket := thrift.NewTSocketConf(addr, cfg)

	var transport thrift.TTransport = socket

	// Add TLS if configured
	if config.UseTLS {
		// TODO: Implement TLS support
		// For now, TLS is not supported
		return nil, fmt.Errorf("TLS support not yet implemented")
	}

	// Use framed transport (required by Storm)
	transport = thrift.NewTFramedTransport(transport)

	// Open connection
	if err := transport.Open(); err != nil {
		return nil, fmt.Errorf("failed to open connection: %w", err)
	}

	protocolFactory := thrift.NewTBinaryProtocolFactoryConf(&thrift.TConfiguration{})
	protocol := protocolFactory.GetProtocol(transport)
	client := NewNimbusClientProtocol(transport, protocol, protocol)

	return &PooledConnection{
		conn:      socket,
		transport: transport,
		protocol:  protocol,
		client:    client,
		createdAt: time.Now(),
		lastUsed:  time.Now(),
	}, nil
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(config *ConnectionPoolConfig) (*ConnectionPool, error) {
	if config.MaxConnections <= 0 {
		return nil, errors.New("MaxConnections must be > 0")
	}

	ctx, cancel := context.WithCancel(context.Background())

	pool := &ConnectionPool{
		config:      config,
		connections: make(chan *PooledConnection, config.MaxConnections),
		factory:     DefaultConnectionFactory,
		ctx:         ctx,
		cancel:      cancel,
	}

	// Pre-create minimum idle connections
	for i := 0; i < config.MinIdleConnections; i++ {
		conn, err := pool.createConnection()
		if err != nil {
			// Clean up any created connections
			pool.Close()
			return nil, fmt.Errorf("failed to create initial connections: %w", err)
		}
		pool.connections <- conn
	}

	// Start maintenance goroutine
	if config.MinIdleConnections > 0 {
		go pool.maintainPool()
	}

	return pool, nil
}

// Get retrieves a connection from the pool
func (p *ConnectionPool) Get(ctx context.Context) (*PooledConnection, error) {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return nil, ErrPoolClosed
	}
	p.mu.RUnlock()

	select {
	case conn := <-p.connections:
		conn.mu.Lock()
		if conn.isValidLocked() {
			conn.inUse = true
			conn.lastUsed = time.Now()
			conn.mu.Unlock()
			return conn, nil
		}
		conn.mu.Unlock()

		// Connection is invalid, close it and try to create a new one
		conn.Invalidate()

		return p.createAndGet(ctx)

	case <-ctx.Done():
		return nil, ctx.Err()

	default:
		// No available connections, try to create a new one
		return p.createAndGet(ctx)
	}
}

// createAndGet creates a new connection if possible
func (p *ConnectionPool) createAndGet(ctx context.Context) (*PooledConnection, error) {
	p.mu.Lock()
	if p.created >= p.config.MaxConnections {
		p.mu.Unlock()

		// Wait for a connection to become available
		select {
		case conn := <-p.connections:
			conn.mu.Lock()
			if conn.isValidLocked() {
				conn.inUse = true
				conn.lastUsed = time.Now()
				conn.mu.Unlock()
				return conn, nil
			}
			conn.mu.Unlock()

			conn.Invalidate()
			// Try again
			return p.createAndGet(ctx)

		case <-ctx.Done():
			return nil, ctx.Err()

		case <-time.After(5 * time.Second):
			return nil, ErrPoolExhausted
		}
	}
	p.mu.Unlock()

	conn, err := p.createConnection()
	if err != nil {
		return nil, err
	}

	conn.mu.Lock()
	conn.inUse = true
	conn.mu.Unlock()

	return conn, nil
}

// createConnection creates a new connection
func (p *ConnectionPool) createConnection() (*PooledConnection, error) {
	conn, err := p.factory(p.config.ClientConfig)
	if err != nil {
		return nil, err
	}

	conn.pool = p

	p.mu.Lock()
	p.created++
	p.mu.Unlock()

	return conn, nil
}

// put returns a connection to the pool
func (p *ConnectionPool) put(conn *PooledConnection) {
	conn.mu.Lock()
	conn.inUse = false
	conn.lastUsed = time.Now()
	conn.mu.Unlock()

	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		conn.Invalidate()
		return
	}
	p.mu.RUnlock()

	if !conn.IsValid() {
		conn.Invalidate()
		return
	}

	select {
	case p.connections <- conn:
		// Connection returned to pool
	default:
		// Pool is full, close the connection
		conn.Invalidate()
	}
}

// maintainPool runs periodic maintenance on the pool
func (p *ConnectionPool) maintainPool() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.cleanupIdleConnections()
			p.ensureMinimumConnections()

		case <-p.ctx.Done():
			return
		}
	}
}

// cleanupIdleConnections removes idle connections that have expired
func (p *ConnectionPool) cleanupIdleConnections() {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return
	}
	p.mu.RUnlock()

	// Check all available connections
	available := len(p.connections)
	for i := 0; i < available; i++ {
		select {
		case conn := <-p.connections:
			if !conn.IsValid() ||
				(p.config.MaxLifetime > 0 && time.Since(conn.createdAt) > p.config.MaxLifetime) {
				conn.Invalidate()
			} else {
				// Put it back
				p.connections <- conn
			}
		default:
			return
		}
	}
}

// ensureMinimumConnections creates connections to maintain minimum idle count
func (p *ConnectionPool) ensureMinimumConnections() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	current := len(p.connections)
	needed := p.config.MinIdleConnections - current

	for i := 0; i < needed && p.created < p.config.MaxConnections; i++ {
		conn, err := p.factory(p.config.ClientConfig)
		if err != nil {
			// Log error but continue
			continue
		}

		conn.pool = p
		p.created++

		select {
		case p.connections <- conn:
			// Added to pool
		default:
			// Pool is full
			conn.Invalidate()
			break
		}
	}
}

// Close closes all connections in the pool
func (p *ConnectionPool) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()

	// Cancel maintenance goroutine
	p.cancel()

	// Close all connections
	close(p.connections)
	for conn := range p.connections {
		conn.Invalidate()
	}

	return nil
}

// Stats returns current pool statistics
type PoolStats struct {
	Created    int
	Available  int
	InUse      int
	MaxAllowed int
}

// Stats returns current pool statistics
func (p *ConnectionPool) Stats() PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	available := len(p.connections)
	return PoolStats{
		Created:    p.created,
		Available:  available,
		InUse:      p.created - available,
		MaxAllowed: p.config.MaxConnections,
	}
}
