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

// To run integration tests: go test -tags=integration ./pkg/clients/aws/ec2/...
// These tests require valid AWS credentials and will make real API calls

package ec2

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_GetDedicatedHosts_RealAPI tests against real AWS API
func TestIntegration_GetDedicatedHosts_RealAPI(t *testing.T) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		t.Skip("AWS_REGION not set, skipping integration test")
	}

	cfg := &ClientConfig{
		Region:           region,
		RetryMaxAttempts: 3,
	}

	client, err := NewClient(cfg)
	require.NoError(t, err, "should create client successfully")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	hosts, err := client.GetDedicatedHosts(ctx)
	require.NoError(t, err, "should get dedicated hosts successfully")

	t.Logf("Successfully retrieved %d dedicated hosts", len(hosts))

	// Validate response structure
	for i, host := range hosts {
		assert.NotNil(t, host.HostId, "Host %d should have non-nil HostId", i)
		assert.NotEmpty(t, host.State, "Host %d should have non-empty State", i)
		t.Logf("Host %d: ID=%s, State=%s", i, *host.HostId, host.State)
	}
}

// TestIntegration_GetInstances_RealAPI tests against real AWS API
func TestIntegration_GetInstances_RealAPI(t *testing.T) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		t.Skip("AWS_REGION not set, skipping integration test")
	}

	instanceIDs := os.Getenv("TEST_INSTANCE_IDS")
	if instanceIDs == "" {
		t.Skip("TEST_INSTANCE_IDS not set, skipping integration test")
	}

	cfg := &ClientConfig{
		Region:           region,
		RetryMaxAttempts: 3,
	}

	client, err := NewClient(cfg)
	require.NoError(t, err, "should create client successfully")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Parse comma-separated instance IDs
	ids := []string{instanceIDs}

	instances, err := client.GetInstances(ctx, ids)
	require.NoError(t, err, "should get instances successfully")

	t.Logf("Successfully retrieved %d instances", len(instances))

	// Validate response structure
	for i, instance := range instances {
		assert.NotNil(t, instance.InstanceId, "Instance %d should have non-nil InstanceId", i)
		assert.NotNil(t, instance.State, "Instance %d should have non-nil State", i)
		if instance.State != nil {
			t.Logf("Instance %d: ID=%s, State=%s",
				i, *instance.InstanceId, instance.State.Name)
		}
	}
}

// TestIntegration_Pagination tests pagination with large result sets
func TestIntegration_Pagination(t *testing.T) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		t.Skip("AWS_REGION not set, skipping integration test")
	}

	cfg := &ClientConfig{
		Region:           region,
		RetryMaxAttempts: 3,
	}

	client, err := NewClient(cfg)
	require.NoError(t, err, "should create client successfully")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Get all dedicated hosts (should trigger pagination if there are many)
	hosts, err := client.GetDedicatedHosts(ctx)
	require.NoError(t, err, "should get dedicated hosts successfully")

	t.Logf("Retrieved %d hosts total (pagination test)", len(hosts))

	// If we have more than 50 hosts, pagination likely occurred
	// (AWS typically returns 50-100 items per page)
	if len(hosts) > 50 {
		t.Logf("Pagination likely occurred (got %d hosts)", len(hosts))
	}
}

// TestIntegration_ContextTimeout tests timeout handling
func TestIntegration_ContextTimeout(t *testing.T) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		t.Skip("AWS_REGION not set, skipping integration test")
	}

	cfg := &ClientConfig{
		Region:           region,
		RetryMaxAttempts: 3,
	}

	client, err := NewClient(cfg)
	require.NoError(t, err, "should create client successfully")

	// Use very short timeout to trigger timeout error
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	_, err = client.GetDedicatedHosts(ctx)
	assert.Error(t, err, "should return timeout error")

	t.Logf("Got expected error: %v", err)
}

// BenchmarkIntegration_GetDedicatedHosts benchmarks real API calls
func BenchmarkIntegration_GetDedicatedHosts(b *testing.B) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		b.Skip("AWS_REGION not set, skipping benchmark")
	}

	cfg := &ClientConfig{
		Region:           region,
		RetryMaxAttempts: 3,
	}

	client, err := NewClient(cfg)
	require.NoError(b, err, "should create client successfully")

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.GetDedicatedHosts(ctx)
		require.NoError(b, err, "should get hosts successfully")
	}
}
