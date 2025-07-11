package thrift

import (
	"testing"
	"time"
)

// TestConnectionPool creates a connection pool for testing with mock factory
func TestConnectionPool(t *testing.T, config *ConnectionPoolConfig) *ConnectionPool {
	if config == nil {
		config = &ConnectionPoolConfig{
			MaxConnections:     5,
			MinIdleConnections: 0,
			MaxIdleTime:        1 * time.Minute,
			MaxLifetime:        5 * time.Minute,
			ClientConfig:       DefaultThriftClientConfig(),
		}
	}

	// Always set MinIdleConnections to 0 for tests
	config.MinIdleConnections = 0

	pool, err := NewConnectionPool(config)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}

	// Note: The mock factory should be set by the test itself
	// pool.factory = MockConnectionFactory(&successCount, 0)

	return pool
}

// WaitForPoolConnections waits for the pool to have the expected number of connections
func WaitForPoolConnections(pool *ConnectionPool, expected int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		stats := pool.Stats()
		if stats.Available == expected {
			return true
		}
		<-ticker.C
	}
	return false
}

// DrainPool removes all connections from the pool
func DrainPool(pool *ConnectionPool) []*PooledConnection {
	var connections []*PooledConnection
	stats := pool.Stats()

loop:
	for i := 0; i < stats.Available; i++ {
		select {
		case conn := <-pool.connections:
			connections = append(connections, conn)
		default:
			break loop
		}
	}

	return connections
}
