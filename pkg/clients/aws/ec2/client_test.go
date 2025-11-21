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

package ec2

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewClient_EmptyRegion tests that empty region returns error
func TestNewClient_EmptyRegion(t *testing.T) {
	cfg := ClientConfig{
		Region:           "",
		RetryMaxAttempts: 3,
	}

	client, err := NewClient(cfg)

	require.Error(t, err, "should return error for empty region")
	assert.Nil(t, client, "client should be nil on error")
	assert.EqualError(t, err, "region is required")
}

// TestGetInstances_EmptySlice tests that empty instance IDs returns empty slice
func TestGetInstances_EmptySlice(t *testing.T) {
	client := &EC2Client{
		client: nil,
	}

	ctx := context.Background()
	instances, err := client.GetInstances(ctx, []string{})

	assert.NoError(t, err, "should not return error for empty instance IDs")
	assert.NotNil(t, instances, "should return non-nil slice")
	assert.Empty(t, instances, "should return empty slice")
}

// TestGetInstances_NilSlice tests that nil instance IDs returns empty slice
func TestGetInstances_NilSlice(t *testing.T) {
	client := &EC2Client{
		client: nil,
	}

	ctx := context.Background()
	instances, err := client.GetInstances(ctx, nil)

	assert.NoError(t, err, "should not return error for nil instance IDs")
	assert.NotNil(t, instances, "should return non-nil slice")
	assert.Empty(t, instances, "should return empty slice")
}

// Benchmark tests
func BenchmarkGetInstances_EmptySlice(b *testing.B) {
	client := &EC2Client{
		client: nil,
	}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.GetInstances(ctx, []string{})
	}
}

// Example test showing expected usage
func ExampleNewClient() {
	cfg := ClientConfig{
		Region:           "us-east-1",
		RetryMaxAttempts: 3,
	}

	client, err := NewClient(cfg)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	hosts, err := client.GetDedicatedHosts(ctx)
	if err != nil {
		panic(err)
	}

	_ = hosts
}

// Example test for GetInstances
func ExampleEC2Client_GetInstances() {
	cfg := ClientConfig{
		Region: "us-east-1",
	}

	client, err := NewClient(cfg)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	instanceIDs := []string{"i-1234567890abcdef0", "i-0987654321fedcba0"}

	instances, err := client.GetInstances(ctx, instanceIDs)
	if err != nil {
		panic(err)
	}

	_ = instances
}

// TestInstanceStructure validates that Instance types are being used correctly
func TestInstanceStructure(t *testing.T) {
	var _ []types.Instance
	var _ []types.Host
	var _ types.Reservation
}
