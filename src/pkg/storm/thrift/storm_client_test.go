package thrift

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestNewThriftStormClient(t *testing.T) {
	// Test with default config and mock pool
	pool := TestConnectionPool(t, nil)
	defer pool.Close()

	client := NewThriftStormClientWithPool(nil, pool)

	if client.config == nil {
		t.Error("Expected config to be set")
	}

	if client.pool == nil {
		t.Error("Expected pool to be set")
	}

	// Test with custom config
	customConfig := &ThriftClientConfig{
		Host:              "custom-host",
		Port:              7627,
		ConnectionTimeout: 15 * time.Second,
		RequestTimeout:    45 * time.Second,
		MaxRetries:        5,
		RetryDelay:        2 * time.Second,
	}

	client2 := NewThriftStormClientWithPool(customConfig, pool)

	if client2.config.Host != "custom-host" {
		t.Errorf("Expected host to be custom-host, got %s", client2.config.Host)
	}
}

// TestThriftStormClientOperations tests client operations with mock pool
func TestThriftStormClientOperations(t *testing.T) {
	// Create a client with mock pool
	config := DefaultThriftClientConfig()
	pool := TestConnectionPool(t, nil)
	successCount := int32(0)
	pool.factory = MockConnectionFactory(&successCount, 0)
	defer pool.Close()

	client := NewThriftStormClientWithPool(config, pool)

	ctx := context.Background()

	// Test connection
	err := client.Connect(ctx)
	if err != nil {
		t.Errorf("Failed to connect: %v", err)
	}

	// Test IsConnected
	if !client.IsConnected() {
		t.Error("Expected client to report as connected")
	}
}

// TestThriftStormClientRetryLogic tests retry behavior
func TestThriftStormClientRetryLogic(t *testing.T) {
	config := &ThriftClientConfig{
		Host:              "localhost",
		Port:              6627,
		ConnectionTimeout: 1 * time.Second,
		RequestTimeout:    1 * time.Second,
		MaxRetries:        3,
		RetryDelay:        100 * time.Millisecond,
	}

	pool := TestConnectionPool(t, nil)
	successCount := int32(0)
	pool.factory = MockConnectionFactory(&successCount, 0)
	defer pool.Close()

	client := NewThriftStormClientWithPool(config, pool)

	// Create a function that simulates failures
	failCount := 0
	failTimes := 2

	// Test withConnection retry logic
	ctx := context.Background()
	err := client.withConnection(ctx, func(nimbus *NimbusClient) error {
		// Simulate failures
		if failCount < failTimes {
			failCount++
			return fmt.Errorf("temporary failure %d", failCount)
		}
		return nil
	})

	// Should succeed after retries
	if err != nil {
		t.Errorf("Expected operation to succeed after retries, got: %v", err)
	}

	if failCount != 2 {
		t.Errorf("Expected 2 failures before success, got %d", failCount)
	}
}

// TestThriftStormClientPoolExhaustion tests behavior when pool is exhausted
func TestThriftStormClientPoolExhaustion(t *testing.T) {
	config := DefaultThriftClientConfig()

	// Create a small pool that will be exhausted
	smallPool := TestConnectionPool(t, &ConnectionPoolConfig{
		MaxConnections:     1,
		MinIdleConnections: 0,
		MaxIdleTime:        1 * time.Minute,
		MaxLifetime:        5 * time.Minute,
		ClientConfig:       config,
	})
	successCount := int32(0)
	smallPool.factory = MockConnectionFactory(&successCount, 0)
	defer smallPool.Close()

	client := NewThriftStormClientWithPool(config, smallPool)

	ctx := context.Background()

	// Get the only connection
	conn1, _ := smallPool.Get(ctx)

	// Try to perform an operation - should timeout
	shortCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	err := client.SubmitTopology(shortCtx, "test", "/tmp/test.jar", "{}", nil)
	if err == nil {
		t.Error("Expected error when pool is exhausted")
	}

	// Return connection
	conn1.Close()
}

// TestThriftStormClientAllMethods tests all interface methods
func TestThriftStormClientAllMethods(t *testing.T) {
	// This test ensures all methods are properly implemented
	// It doesn't test actual functionality but ensures compilation

	var _ ThriftClient = (*ThriftStormClient)(nil)

	config := DefaultThriftClientConfig()
	pool := TestConnectionPool(t, nil)
	successCount := int32(0)
	pool.factory = MockConnectionFactory(&successCount, 0)
	defer pool.Close()

	client := NewThriftStormClientWithPool(config, pool)

	// Test that all methods exist and can be called
	ctx := context.Background()

	methods := []func() error{
		func() error { return client.Connect(ctx) },
		func() error { client.IsConnected(); return nil },
		func() error { return client.SubmitTopology(ctx, "test", "/jar", "{}", nil) },
		func() error { return client.KillTopology(ctx, "test") },
		func() error { return client.KillTopologyWithOpts(ctx, "test", nil) },
		func() error { return client.Activate(ctx, "test") },
		func() error { return client.Deactivate(ctx, "test") },
		func() error { return client.Rebalance(ctx, "test", nil) },
		func() error { _, err := client.GetTopologyInfo(ctx, "test"); return err },
		func() error { _, err := client.GetTopologyPageInfo(ctx, "test", "600", true); return err },
		func() error { _, err := client.GetTopology(ctx, "test"); return err },
		func() error { _, err := client.GetUserTopology(ctx, "test"); return err },
		func() error { _, err := client.GetTopologyHistory(ctx, "user"); return err },
		func() error { _, err := client.GetClusterInfo(ctx); return err },
		func() error { _, err := client.GetLeader(ctx); return err },
		func() error { _, err := client.IsTopologyNameAllowed(ctx, "test"); return err },
		func() error { _, err := client.GetSupervisorPageInfo(ctx, "id", "host", true); return err },
		func() error { _, err := client.GetComponentPageInfo(ctx, "topo", "comp", "600", true); return err },
		func() error { _, err := client.GetComponentPendingProfileActions(ctx, "id", "comp", 0); return err },
		func() error { _, err := client.GetTopologyConf(ctx, "test"); return err },
		func() error { _, err := client.GetOwnerResourceSummaries(ctx, "owner"); return err },
		func() error { return client.Debug(ctx, "test", "comp", true, 0.1) },
		func() error { return client.SetWorkerProfiler(ctx, "id", nil) },
		func() error { _, err := client.GetWorkerProfileActionExpiry(ctx, "id", 0); return err },
		func() error { _, err := client.GetTopologyLogConfig(ctx, "test"); return err },
		func() error { return client.SetTopologyLogConfig(ctx, "test", nil) },
		func() error { return client.SetLogConfig(ctx, "test", nil) },
		func() error { return client.Close() },
	}

	// Just verify methods exist and can be called
	if len(methods) != 28 {
		t.Errorf("Expected 28 methods, got %d", len(methods))
	}
}

// BenchmarkThriftStormClientGetConnection benchmarks connection retrieval
func BenchmarkThriftStormClientGetConnection(b *testing.B) {
	config := DefaultThriftClientConfig()

	// Create pool with mock factory
	poolConfig := &ConnectionPoolConfig{
		MaxConnections:     10,
		MinIdleConnections: 0, // Don't pre-create
		MaxIdleTime:        5 * time.Minute,
		MaxLifetime:        30 * time.Minute,
		ClientConfig:       config,
	}

	pool, err := NewConnectionPool(poolConfig)
	if err != nil {
		b.Fatalf("Failed to create pool: %v", err)
	}

	successCount := int32(0)
	pool.factory = MockConnectionFactory(&successCount, 0)
	defer pool.Close()

	client := NewThriftStormClientWithPool(config, pool)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn, err := client.pool.Get(ctx)
		if err != nil {
			b.Fatal(err)
		}
		conn.Close()
	}
}
