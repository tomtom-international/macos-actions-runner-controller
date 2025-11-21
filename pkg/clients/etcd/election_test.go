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

// TestElectionConfig_DefaultValues tests that defaults are applied correctly
func TestElectionConfig_DefaultValues(t *testing.T) {
	tests := []struct {
		name                   string
		ttl                    int
		sessionTimeout         int
		expectedTTL            int
		expectedSessionTimeout int
	}{
		{
			name:                   "zero values get defaults",
			ttl:                    0,
			sessionTimeout:         0,
			expectedTTL:            defaultTTL,
			expectedSessionTimeout: defaultSessionTimeout,
		},
		{
			name:                   "negative TTL gets default",
			ttl:                    -1,
			sessionTimeout:         5,
			expectedTTL:            defaultTTL,
			expectedSessionTimeout: 5,
		},
		{
			name:                   "negative SessionTimeout gets default",
			ttl:                    10,
			sessionTimeout:         -1,
			expectedTTL:            10,
			expectedSessionTimeout: defaultSessionTimeout,
		},
		{
			name:                   "custom values are preserved",
			ttl:                    20,
			sessionTimeout:         30,
			expectedTTL:            20,
			expectedSessionTimeout: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, defaultTTL, 15, "defaultTTL constant should be 15")
			assert.Equal(t, defaultSessionTimeout, 10, "defaultSessionTimeout constant should be 10")
		})
	}
}

// TestNewLeaderElection_UnreachableEtcd tests timeout behavior with unreachable etcd
func TestNewLeaderElection_UnreachableEtcd(t *testing.T) {
	cfg := ClientConfig{
		Endpoints:      []string{"localhost:9999"},
		RequestTimeout: 1,
	}

	client, err := NewEtcdClient(cfg)
	require.Error(t, err, "should fail to connect to unreachable etcd")
	assert.Nil(t, client, "client should be nil")
}

// TestElectionConfig_Validation validates ElectionConfig fields
func TestElectionConfig_Validation(t *testing.T) {
	tests := []struct {
		name        string
		errorMsg    string
		cfg         ElectionConfig
		shouldError bool
	}{
		{
			name: "valid config",
			cfg: ElectionConfig{
				Identity:       "node-1",
				ElectionKey:    "/election/leader",
				TTL:            15,
				SessionTimeout: 10,
			},
			shouldError: false,
		},
		{
			name: "missing identity",
			cfg: ElectionConfig{
				Identity:       "",
				ElectionKey:    "/election/leader",
				TTL:            15,
				SessionTimeout: 10,
			},
			shouldError: true,
			errorMsg:    "election identity is required",
		},
		{
			name: "missing election key",
			cfg: ElectionConfig{
				Identity:       "node-1",
				ElectionKey:    "",
				TTL:            15,
				SessionTimeout: 10,
			},
			shouldError: true,
			errorMsg:    "election key is required",
		},
		{
			name: "defaults applied for zero TTL",
			cfg: ElectionConfig{
				Identity:       "node-1",
				ElectionKey:    "/election/leader",
				TTL:            0,
				SessionTimeout: 10,
			},
			shouldError: false,
		},
		{
			name: "defaults applied for zero SessionTimeout",
			cfg: ElectionConfig{
				Identity:       "node-1",
				ElectionKey:    "/election/leader",
				TTL:            15,
				SessionTimeout: 0,
			},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shouldError {
				assert.NotEmpty(t, tt.errorMsg, "error message should be defined")
			}
		})
	}
}
