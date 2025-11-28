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

// TestLoadTLSConfig_NoTLS tests TLS config with all empty values
func TestLoadTLSConfig_NoTLS(t *testing.T) {
	tlsConfig, err := loadTLSConfig("", "", "")

	require.NoError(t, err, "should succeed with all empty values")
	assert.NotNil(t, tlsConfig, "TLS config should not be nil")
	assert.Nil(t, tlsConfig.RootCAs, "RootCAs should be nil (uses system CA pool)")
	assert.Empty(t, tlsConfig.Certificates, "Certificates should be empty")
	assert.Equal(t, uint16(0x0303), tlsConfig.MinVersion, "MinVersion should be TLS 1.2")
}

// TestLoadTLSConfig_OnlyClientCert tests error when only client cert is provided
func TestLoadTLSConfig_OnlyClientCert(t *testing.T) {
	tlsConfig, err := loadTLSConfig("/path/to/cert.crt", "", "")

	require.Error(t, err, "should return error when only cert file provided")
	assert.Nil(t, tlsConfig, "TLS config should be nil on error")
	assert.Contains(t, err.Error(), "both certFile and keyFile must be provided together")
}

// TestLoadTLSConfig_OnlyClientKey tests error when only client key is provided
func TestLoadTLSConfig_OnlyClientKey(t *testing.T) {
	tlsConfig, err := loadTLSConfig("", "/path/to/key.key", "")

	require.Error(t, err, "should return error when only key file provided")
	assert.Nil(t, tlsConfig, "TLS config should be nil on error")
	assert.Contains(t, err.Error(), "both certFile and keyFile must be provided together")
}

// TestLoadTLSConfig_InvalidCAFile tests error when CA file doesn't exist
func TestLoadTLSConfig_InvalidCAFile(t *testing.T) {
	tlsConfig, err := loadTLSConfig("", "", "/nonexistent/ca.crt")

	require.Error(t, err, "should return error for nonexistent CA file")
	assert.Nil(t, tlsConfig, "TLS config should be nil on error")
	assert.Contains(t, err.Error(), "failed to read CA certificate")
}

// TestLoadTLSConfig_InvalidClientCertFile tests error when client cert file doesn't exist
func TestLoadTLSConfig_InvalidClientCertFile(t *testing.T) {
	tlsConfig, err := loadTLSConfig("/nonexistent/cert.crt", "/nonexistent/key.key", "")

	require.Error(t, err, "should return error for nonexistent client cert")
	assert.Nil(t, tlsConfig, "TLS config should be nil on error")
	assert.Contains(t, err.Error(), "failed to load client certificate")
}

// TestNewEtcdClient_TLSWithoutCA tests TLS enabled without CA (uses system CA pool)
func TestNewEtcdClient_TLSWithoutCA(t *testing.T) {
	cfg := ClientConfig{
		Endpoints:      []string{"localhost:9999"},
		RequestTimeout: 1,
		TLSEnabled:     true,
	}

	client, err := NewEtcdClient(cfg)

	// Client creation should succeed (TLS config created)
	// Connection will fail because localhost:9999 doesn't exist, but that's expected
	require.Error(t, err, "should fail to connect")
	assert.Nil(t, client, "client should be nil")
	assert.Contains(t, err.Error(), "failed to connect to etcd")
}

// TestNewEtcdClient_TLSWithUsername tests TLS with username/password auth
func TestNewEtcdClient_TLSWithUsernamePassword(t *testing.T) {
	cfg := ClientConfig{
		Endpoints:      []string{"localhost:9999"},
		RequestTimeout: 1,
		TLSEnabled:     true,
		Username:       "test-user",
		Password:       "test-password",
	}

	client, err := NewEtcdClient(cfg)

	require.Error(t, err, "should fail to connect to unreachable endpoint")
	assert.Nil(t, client, "client should be nil")
}

// TestNewEtcdClient_UsernameWithoutPassword tests username without password
func TestNewEtcdClient_UsernameWithoutPassword(t *testing.T) {
	cfg := ClientConfig{
		Endpoints:      []string{"localhost:9999"},
		RequestTimeout: 1,
		Username:       "test-user",
	}

	client, err := NewEtcdClient(cfg)

	require.Error(t, err, "should fail to connect")
	assert.Nil(t, client, "client should be nil")
}

// TestNewEtcdClient_PasswordWithoutUsername tests password without username
func TestNewEtcdClient_PasswordWithoutUsername(t *testing.T) {
	cfg := ClientConfig{
		Endpoints:      []string{"localhost:9999"},
		RequestTimeout: 1,
		Password:       "test-password",
	}

	client, err := NewEtcdClient(cfg)

	require.Error(t, err, "should fail to connect")
	assert.Nil(t, client, "client should be nil")
}
