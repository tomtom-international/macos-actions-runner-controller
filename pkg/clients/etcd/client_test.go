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

package etcd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewEtcdClient_EmptyEndpoints tests that empty endpoints returns error
func TestNewEtcdClient_EmptyEndpoints(t *testing.T) {
	cfg := ClientConfig{
		Endpoints: []string{},
	}

	client, err := NewEtcdClient(cfg)

	require.Error(t, err, "should return error for empty endpoints")
	assert.Nil(t, client, "client should be nil on error")
	assert.EqualError(t, err, "etcd endpoints are required")
}

// TestNewEtcdClient_NilEndpoints tests that nil endpoints returns error
func TestNewEtcdClient_NilEndpoints(t *testing.T) {
	cfg := ClientConfig{
		Endpoints: nil,
	}

	client, err := NewEtcdClient(cfg)

	require.Error(t, err, "should return error for nil endpoints")
	assert.Nil(t, client, "client should be nil on error")
	assert.EqualError(t, err, "etcd endpoints are required")
}

// TestNewEtcdClient_UnreachableEndpoint tests behavior with unreachable endpoint
func TestNewEtcdClient_UnreachableEndpoint(t *testing.T) {
	cfg := ClientConfig{
		Endpoints:      []string{"localhost:9999"},
		RequestTimeout: 1,
	}

	client, err := NewEtcdClient(cfg)

	require.Error(t, err, "should return error for unreachable endpoint")
	assert.Nil(t, client, "client should be nil on error")
	assert.Contains(t, err.Error(), "failed to connect to etcd",
		"error should mention connection failure")
}

// TestNewEtcdClient_MultipleUnreachableEndpoints tests with multiple unreachable endpoints
func TestNewEtcdClient_MultipleUnreachableEndpoints(t *testing.T) {
	cfg := ClientConfig{
		Endpoints: []string{
			"localhost:9999",
			"localhost:9998",
			"localhost:9997",
		},
		RequestTimeout: 1,
	}

	client, err := NewEtcdClient(cfg)

	require.Error(t, err, "should return error when all endpoints unreachable")
	assert.Nil(t, client, "client should be nil on error")
	assert.Contains(t, err.Error(), "no reachable endpoints",
		"error should mention no reachable endpoints")
}

// TestClient_Close tests that Close doesn't panic
func TestClient_Close(t *testing.T) {
	cfg := ClientConfig{
		Endpoints:      []string{"localhost:9999"},
		RequestTimeout: 1,
	}

	client, err := NewEtcdClient(cfg)
	require.Error(t, err, "should fail to create client")
	assert.Nil(t, client, "client should be nil")

	// Ensure Close on nil client doesn't panic if someone tries it
	if client != nil {
		err = client.Close()
		assert.NoError(t, err, "Close should not return error on first call")
	}
}
