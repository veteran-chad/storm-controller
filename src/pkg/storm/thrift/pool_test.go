package thrift

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
)

func TestDefaultConnectionPoolConfig(t *testing.T) {
	config := DefaultConnectionPoolConfig()

	if config.MaxConnections != 10 {
		t.Errorf("Expected max connections to be 10, got %d", config.MaxConnections)
	}

	if config.MinIdleConnections != 2 {
		t.Errorf("Expected min idle connections to be 2, got %d", config.MinIdleConnections)
	}

	if config.MaxIdleTime != 5*time.Minute {
		t.Errorf("Expected max idle time to be 5m, got %v", config.MaxIdleTime)
	}

	if config.MaxLifetime != 30*time.Minute {
		t.Errorf("Expected max lifetime to be 30m, got %v", config.MaxLifetime)
	}

	if config.ClientConfig == nil {
		t.Error("Expected client config to be set")
	}
}

// MockConnectionFactory for testing
func MockConnectionFactory(successCount *int32, failAfter int) ConnectionFactory {
	count := int32(0)
	mu := &sync.Mutex{}

	return func(config *ThriftClientConfig) (*PooledConnection, error) {
		mu.Lock()
		defer mu.Unlock()

		count++
		if failAfter > 0 && int(count) > failAfter {
			return nil, fmt.Errorf("mock connection failed")
		}

		*successCount++

		// Create a mock connection
		mockTransport := &mockTransport{open: true}

		return &PooledConnection{
			transport: mockTransport,
			protocol:  thrift.NewTBinaryProtocolFactoryConf(&thrift.TConfiguration{}).GetProtocol(mockTransport),
			client:    &NimbusClient{},
			createdAt: time.Now(),
			lastUsed:  time.Now(),
		}, nil
	}
}

// mockTransport implements thrift.TTransport for testing
type mockTransport struct {
	open bool
}

func (m *mockTransport) Open() error {
	m.open = true
	return nil
}

func (m *mockTransport) Close() error {
	m.open = false
	return nil
}

func (m *mockTransport) IsOpen() bool {
	return m.open
}

func (m *mockTransport) Read(p []byte) (n int, err error) {
	return 0, nil
}

func (m *mockTransport) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func (m *mockTransport) Flush(ctx context.Context) error {
	return nil
}

func (m *mockTransport) RemainingBytes() uint64 {
	return 0
}

func TestConnectionPoolCreation(t *testing.T) {
	config := &ConnectionPoolConfig{
		MaxConnections:     5,
		MinIdleConnections: 0, // Don't pre-create connections
		MaxIdleTime:        1 * time.Minute,
		MaxLifetime:        5 * time.Minute,
		ClientConfig:       DefaultThriftClientConfig(),
	}

	successCount := int32(0)
	pool, err := NewConnectionPool(config)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	if pool == nil {
		t.Fatal("Expected pool to be created")
	}
	pool.factory = MockConnectionFactory(&successCount, 0)
	defer func() { _ = pool.Close() }()

	stats := pool.Stats()
	if stats.MaxAllowed != 5 {
		t.Errorf("Expected max allowed to be 5, got %d", stats.MaxAllowed)
	}
}

func TestConnectionPoolInvalidConfig(t *testing.T) {
	config := &ConnectionPoolConfig{
		MaxConnections: 0, // Invalid
		ClientConfig:   DefaultThriftClientConfig(),
	}

	_, err := NewConnectionPool(config)
	if err == nil {
		t.Error("Expected error for invalid config")
	}
}

func TestConnectionPoolGetAndPut(t *testing.T) {
	pool := TestConnectionPool(t, &ConnectionPoolConfig{
		MaxConnections:     3,
		MinIdleConnections: 0,
		MaxIdleTime:        1 * time.Minute,
		MaxLifetime:        5 * time.Minute,
		ClientConfig:       DefaultThriftClientConfig(),
	})
	successCount := int32(0)
	pool.factory = MockConnectionFactory(&successCount, 0)
	defer func() { _ = pool.Close() }()

	ctx := context.Background()

	// Get a connection
	conn1, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Failed to get connection: %v", err)
	}

	// Check stats
	stats := pool.Stats()
	if stats.InUse != 1 {
		t.Errorf("Expected 1 connection in use, got %d", stats.InUse)
	}

	// Return connection
	_ = conn1.Close()

	// Check stats again
	stats = pool.Stats()
	if stats.InUse != 0 {
		t.Errorf("Expected 0 connections in use, got %d", stats.InUse)
	}
}

func TestConnectionPoolConcurrency(t *testing.T) {
	config := &ConnectionPoolConfig{
		MaxConnections:     10,
		MinIdleConnections: 0, // Don't pre-create
		MaxIdleTime:        1 * time.Minute,
		MaxLifetime:        5 * time.Minute,
		ClientConfig:       DefaultThriftClientConfig(),
	}

	successCount := int32(0)
	pool, err := NewConnectionPool(config)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	pool.factory = MockConnectionFactory(&successCount, 0)
	defer func() { _ = pool.Close() }()

	ctx := context.Background()
	var wg sync.WaitGroup
	errors := make(chan error, 20)

	// Spawn 20 goroutines to get/put connections
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			conn, err := pool.Get(ctx)
			if err != nil {
				errors <- err
				return
			}

			// Simulate some work
			time.Sleep(10 * time.Millisecond)

			_ = conn.Close()
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent operation failed: %v", err)
	}

	// All connections should be returned
	stats := pool.Stats()
	if stats.InUse != 0 {
		t.Errorf("Expected 0 connections in use, got %d", stats.InUse)
	}
}

func TestConnectionPoolExhaustion(t *testing.T) {
	config := &ConnectionPoolConfig{
		MaxConnections:     2,
		MinIdleConnections: 0,
		MaxIdleTime:        1 * time.Minute,
		MaxLifetime:        5 * time.Minute,
		ClientConfig:       DefaultThriftClientConfig(),
	}

	successCount := int32(0)
	pool, err := NewConnectionPool(config)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	pool.factory = MockConnectionFactory(&successCount, 0)
	defer func() { _ = pool.Close() }()

	ctx := context.Background()

	// Get all connections
	conn1, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Failed to get connection 1: %v", err)
	}
	defer func() { _ = conn1.Close() }()

	conn2, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Failed to get connection 2: %v", err)
	}
	defer func() { _ = conn2.Close() }()

	// Try to get another connection with short timeout
	shortCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	_, err = pool.Get(shortCtx)
	if err == nil {
		t.Error("Expected error when pool is exhausted")
	}
}

func TestConnectionInvalidation(t *testing.T) {
	config := &ConnectionPoolConfig{
		MaxConnections:     3,
		MinIdleConnections: 0, // Don't pre-create
		MaxIdleTime:        1 * time.Minute,
		MaxLifetime:        5 * time.Minute,
		ClientConfig:       DefaultThriftClientConfig(),
	}

	successCount := int32(0)
	pool, err := NewConnectionPool(config)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	pool.factory = MockConnectionFactory(&successCount, 0)
	defer func() { _ = pool.Close() }()

	ctx := context.Background()

	// Get a connection
	conn, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Failed to get connection: %v", err)
	}

	// Note initial stats
	initialStats := pool.Stats()
	initialInUse := initialStats.InUse

	// Invalidate it
	conn.Invalidate()

	// Close should not return it to pool
	_ = conn.Close()

	// After invalidation and close, the connection should be destroyed
	afterStats := pool.Stats()
	if afterStats.Available > 0 {
		t.Error("Expected no available connections after invalidation")
	}

	// Created count should decrease after the close
	if afterStats.Created != initialStats.Created-1 {
		t.Errorf("Expected created count to be %d, got %d", initialStats.Created-1, afterStats.Created)
	}

	// InUse should also decrease
	if afterStats.InUse != initialInUse-1 {
		t.Errorf("Expected InUse to be %d, got %d", initialInUse-1, afterStats.InUse)
	}
}

func TestConnectionPoolClose(t *testing.T) {
	config := &ConnectionPoolConfig{
		MaxConnections:     3,
		MinIdleConnections: 0, // Don't pre-create
		MaxIdleTime:        1 * time.Minute,
		MaxLifetime:        5 * time.Minute,
		ClientConfig:       DefaultThriftClientConfig(),
	}

	successCount := int32(0)
	pool, err := NewConnectionPool(config)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	pool.factory = MockConnectionFactory(&successCount, 0)

	// Close the pool
	err = pool.Close()
	if err != nil {
		t.Fatalf("Failed to close pool: %v", err)
	}

	// Try to get connection from closed pool
	ctx := context.Background()
	_, err = pool.Get(ctx)
	if err != ErrPoolClosed {
		t.Errorf("Expected ErrPoolClosed, got %v", err)
	}
}

func TestPooledConnectionIsValid(t *testing.T) {
	conn := &PooledConnection{
		transport: &mockTransport{open: true},
		createdAt: time.Now(),
		lastUsed:  time.Now(),
	}

	// Should be valid initially
	if !conn.IsValid() {
		t.Error("Expected connection to be valid")
	}

	// Close transport
	conn.transport.(*mockTransport).open = false
	if conn.IsValid() {
		t.Error("Expected connection to be invalid when transport is closed")
	}

	// Test with idle timeout
	conn.transport.(*mockTransport).open = true
	conn.lastUsed = time.Now().Add(-10 * time.Minute)
	conn.pool = &ConnectionPool{
		config: &ConnectionPoolConfig{
			MaxIdleTime: 5 * time.Minute,
		},
	}

	if conn.IsValid() {
		t.Error("Expected connection to be invalid when idle too long")
	}
}
