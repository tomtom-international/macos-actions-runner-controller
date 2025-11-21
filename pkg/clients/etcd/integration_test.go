//go:build integration

/*
 * Copyright 2025 TomTom N.V.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// To run integration tests: go test -tags=integration ./pkg/clients/etcd/...
// These tests require a running etcd instance. Set ETCD_ENDPOINTS env var to specify endpoints.
// Example: ETCD_ENDPOINTS=localhost:2379 go test -tags=integration ./pkg/clients/etcd/...

package etcd

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getEtcdEndpoints returns etcd endpoints from env var or skips test
func getEtcdEndpoints(t *testing.T) []string {
	endpoints := os.Getenv("ETCD_ENDPOINTS")
	if endpoints == "" {
		t.Skip("ETCD_ENDPOINTS not set, skipping integration test")
	}
	return strings.Split(endpoints, ",")
}

// TestIntegration_NewEtcdClient_ConnectionValidation tests successful connection
func TestIntegration_NewEtcdClient_ConnectionValidation(t *testing.T) {
	endpoints := getEtcdEndpoints(t)

	cfg := ClientConfig{
		Endpoints:      endpoints,
		RequestTimeout: 5,
	}

	client, err := NewEtcdClient(cfg)
	require.NoError(t, err, "should create client successfully")
	require.NotNil(t, client, "client should not be nil")

	defer client.Close()

	t.Logf("Successfully connected to etcd at %v", endpoints)
}

// TestIntegration_Put_Get tests basic Put and Get operations
func TestIntegration_Put_Get(t *testing.T) {
	endpoints := getEtcdEndpoints(t)

	cfg := ClientConfig{
		Endpoints:      endpoints,
		RequestTimeout: 5,
	}

	client, err := NewEtcdClient(cfg)
	require.NoError(t, err, "should create client successfully")
	defer client.Close()

	testKey := "/test/integration/key1"
	testValue := "test-value-123"

	err = client.Put(testKey, testValue)
	require.NoError(t, err, "Put should succeed")

	value, err := client.Get(testKey)
	require.NoError(t, err, "Get should succeed")
	assert.Equal(t, testValue, value, "retrieved value should match")

	t.Logf("Successfully Put and Get key=%s, value=%s", testKey, testValue)
}

// TestIntegration_GetByPrefix tests prefix queries
func TestIntegration_GetByPrefix(t *testing.T) {
	endpoints := getEtcdEndpoints(t)

	cfg := ClientConfig{
		Endpoints:      endpoints,
		RequestTimeout: 5,
	}

	client, err := NewEtcdClient(cfg)
	require.NoError(t, err, "should create client successfully")
	defer client.Close()

	prefix := "/test/integration/prefix"
	testData := map[string]string{
		prefix + "/key1": "value1",
		prefix + "/key2": "value2",
		prefix + "/key3": "value3",
	}

	for key, value := range testData {
		err := client.Put(key, value)
		require.NoError(t, err, "Put should succeed for key %s", key)
	}

	results, err := client.GetByPrefix(prefix)
	require.NoError(t, err, "GetByPrefix should succeed")
	assert.GreaterOrEqual(t, len(results), len(testData),
		"should retrieve at least as many keys as we put")

	for key, expectedValue := range testData {
		actualValue, exists := results[key]
		assert.True(t, exists, "key %s should exist in results", key)
		assert.Equal(t, expectedValue, actualValue,
			"value for key %s should match", key)
	}

	t.Logf("Successfully retrieved %d keys with prefix %s", len(results), prefix)
}

// TestIntegration_GetNonExistentKey tests getting a key that doesn't exist
func TestIntegration_GetNonExistentKey(t *testing.T) {
	endpoints := getEtcdEndpoints(t)

	cfg := ClientConfig{
		Endpoints:      endpoints,
		RequestTimeout: 5,
	}

	client, err := NewEtcdClient(cfg)
	require.NoError(t, err, "should create client successfully")
	defer client.Close()

	value, err := client.Get("/test/integration/nonexistent")
	require.NoError(t, err, "Get should not return error for non-existent key")
	assert.Empty(t, value, "value should be empty string for non-existent key")
}

// TestIntegration_GrantLease_PutWithLease tests lease operations
func TestIntegration_GrantLease_PutWithLease(t *testing.T) {
	endpoints := getEtcdEndpoints(t)

	cfg := ClientConfig{
		Endpoints:      endpoints,
		RequestTimeout: 5,
	}

	client, err := NewEtcdClient(cfg)
	require.NoError(t, err, "should create client successfully")
	defer client.Close()

	leaseID, err := client.GrantLease(5)
	require.NoError(t, err, "GrantLease should succeed")
	assert.NotZero(t, leaseID, "leaseID should not be zero")

	testKey := "/test/integration/lease-key"
	testValue := "lease-value"
	err = client.PutWithLease(testKey, testValue, leaseID)
	require.NoError(t, err, "PutWithLease should succeed")

	value, err := client.Get(testKey)
	require.NoError(t, err, "Get should succeed")
	assert.Equal(t, testValue, value, "value should match")

	t.Logf("Successfully created key with lease: leaseID=%d, key=%s", leaseID, testKey)
}

// TestIntegration_Watch tests watch functionality
func TestIntegration_Watch(t *testing.T) {
	endpoints := getEtcdEndpoints(t)

	cfg := ClientConfig{
		Endpoints:      endpoints,
		RequestTimeout: 5,
	}

	client, err := NewEtcdClient(cfg)
	require.NoError(t, err, "should create client successfully")
	defer client.Close()

	testKey := "/test/integration/watch-key"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	watchChan := client.Watch(ctx, testKey)

	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = client.Put(testKey, "watch-test-value")
	}()

	select {
	case watchResp := <-watchChan:
		require.NoError(t, watchResp.Err(), "watch should not have error")
		assert.NotEmpty(t, watchResp.Events, "should receive at least one event")
		t.Logf("Successfully received watch event for key %s", testKey)
	case <-ctx.Done():
		t.Fatal("timeout waiting for watch event")
	}
}

// TestIntegration_WatchWithPrefix tests watching with prefix
func TestIntegration_WatchWithPrefix(t *testing.T) {
	endpoints := getEtcdEndpoints(t)

	cfg := ClientConfig{
		Endpoints:      endpoints,
		RequestTimeout: 5,
	}

	client, err := NewEtcdClient(cfg)
	require.NoError(t, err, "should create client successfully")
	defer client.Close()

	prefix := "/test/integration/watch-prefix"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	watchChan := client.WatchWithPrefix(ctx, prefix)

	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = client.Put(prefix+"/key1", "value1")
		_ = client.Put(prefix+"/key2", "value2")
	}()

	eventsReceived := 0
	timeout := time.After(5 * time.Second)

	for eventsReceived < 2 {
		select {
		case watchResp := <-watchChan:
			require.NoError(t, watchResp.Err(), "watch should not have error")
			eventsReceived += len(watchResp.Events)
			t.Logf("Received %d events (total: %d)", len(watchResp.Events), eventsReceived)
		case <-timeout:
			t.Fatalf("timeout waiting for watch events, received %d events", eventsReceived)
		}
	}

	assert.GreaterOrEqual(t, eventsReceived, 2, "should receive at least 2 events")
}

// TestIntegration_Close_CancelsOperations tests that Close cancels in-flight operations
func TestIntegration_Close_CancelsOperations(t *testing.T) {
	endpoints := getEtcdEndpoints(t)

	cfg := ClientConfig{
		Endpoints:      endpoints,
		RequestTimeout: 30, // Long timeout
	}

	client, err := NewEtcdClient(cfg)
	require.NoError(t, err, "should create client successfully")

	watchCtx := context.Background()
	watchChan := client.Watch(watchCtx, "/test/integration/close-test")

	time.Sleep(100 * time.Millisecond)
	err = client.Close()
	require.NoError(t, err, "Close should succeed")

	err = client.Put("/test/key", "value")
	assert.Error(t, err, "Put should fail after client is closed")
	assert.Contains(t, err.Error(), "context canceled",
		"error should indicate context was canceled")

	select {
	case _, ok := <-watchChan:
		if ok {
			// If we received a response, it should have an error
			t.Log("Watch received response after close (expected)")
		} else {
			t.Log("Watch channel closed (expected)")
		}
	case <-time.After(2 * time.Second):
		// It's ok if watch doesn't close immediately
		t.Log("Watch didn't close immediately (acceptable)")
	}

	t.Log("Successfully verified Close cancels operations")
}

// TestIntegration_MultipleOperations tests multiple sequential operations
func TestIntegration_MultipleOperations(t *testing.T) {
	endpoints := getEtcdEndpoints(t)

	cfg := ClientConfig{
		Endpoints:      endpoints,
		RequestTimeout: 5,
	}

	client, err := NewEtcdClient(cfg)
	require.NoError(t, err, "should create client successfully")
	defer client.Close()

	for i := 0; i < 10; i++ {
		key := "/test/integration/multi/" + string(rune('a'+i))
		value := "value-" + string(rune('0'+i))

		err := client.Put(key, value)
		require.NoError(t, err, "Put %d should succeed", i)

		retrievedValue, err := client.Get(key)
		require.NoError(t, err, "Get %d should succeed", i)
		assert.Equal(t, value, retrievedValue, "value %d should match", i)
	}

	t.Log("Successfully performed 10 Put/Get operations")
}
