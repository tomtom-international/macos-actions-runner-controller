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

// To run integration tests: go test -tags=integration ./pkg/clients/aws/asg/...
// These tests require valid AWS credentials and will make real API calls

package asg

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_GetCapacity_RealAPI tests against real AWS API
func TestIntegration_GetCapacity_RealAPI(t *testing.T) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		t.Skip("AWS_REGION not set, skipping integration test")
	}

	asgName := os.Getenv("TEST_ASG_NAME")
	if asgName == "" {
		t.Skip("TEST_ASG_NAME not set, skipping integration test")
	}

	cfg := ClientConfig{
		Region:           region,
		RetryMaxAttempts: 3,
	}

	client, err := NewClient(cfg)
	require.NoError(t, err, "should create client successfully")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	capacity, err := client.GetCapacity(ctx, asgName)
	require.NoError(t, err, "should get capacity successfully")

	t.Logf("Successfully retrieved capacity for ASG '%s': %d", asgName, capacity)

	// Validate response
	assert.GreaterOrEqual(t, capacity, 0, "capacity should be non-negative")
}

// TestIntegration_GetInstanceIDs_RealAPI tests against real AWS API
func TestIntegration_GetInstanceIDs_RealAPI(t *testing.T) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		t.Skip("AWS_REGION not set, skipping integration test")
	}

	asgName := os.Getenv("TEST_ASG_NAME")
	if asgName == "" {
		t.Skip("TEST_ASG_NAME not set, skipping integration test")
	}

	cfg := ClientConfig{
		Region:           region,
		RetryMaxAttempts: 3,
	}

	client, err := NewClient(cfg)
	require.NoError(t, err, "should create client successfully")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	instanceIDs, err := client.GetInstanceIDs(ctx, asgName)
	require.NoError(t, err, "should get instance IDs successfully")

	t.Logf("Successfully retrieved %d instance IDs from ASG '%s'", len(instanceIDs), asgName)

	// Validate response structure
	assert.NotNil(t, instanceIDs, "should return non-nil slice")
	for i, instanceID := range instanceIDs {
		assert.NotEmpty(t, instanceID, "instance ID %d should not be empty", i)
		t.Logf("Instance %d: %s", i, instanceID)
	}
}

// TestIntegration_SetCapacity_RealAPI tests against real AWS API
func TestIntegration_SetCapacity_RealAPI(t *testing.T) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		t.Skip("AWS_REGION not set, skipping integration test")
	}

	asgName := os.Getenv("TEST_ASG_NAME")
	if asgName == "" {
		t.Skip("TEST_ASG_NAME not set, skipping integration test")
	}

	// Only run if ALLOW_ASG_MODIFICATION is set (safety check)
	if os.Getenv("ALLOW_ASG_MODIFICATION") != "true" {
		t.Skip("ALLOW_ASG_MODIFICATION not set to 'true', skipping capacity modification test")
	}

	cfg := ClientConfig{
		Region:           region,
		RetryMaxAttempts: 3,
	}

	client, err := NewClient(cfg)
	require.NoError(t, err, "should create client successfully")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Get current capacity
	originalCapacity, err := client.GetCapacity(ctx, asgName)
	require.NoError(t, err, "should get original capacity successfully")
	t.Logf("Original capacity: %d", originalCapacity)

	// Set to same capacity (safe operation)
	err = client.SetCapacity(ctx, asgName, originalCapacity)
	require.NoError(t, err, "should set capacity successfully")
	t.Logf("Successfully set capacity to %d", originalCapacity)

	// Verify capacity was set
	newCapacity, err := client.GetCapacity(ctx, asgName)
	require.NoError(t, err, "should get new capacity successfully")
	assert.Equal(t, originalCapacity, newCapacity, "capacity should match what was set")
}

// TestIntegration_GetCapacity_NonExistentASG tests error handling for non-existent ASG
func TestIntegration_GetCapacity_NonExistentASG(t *testing.T) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		t.Skip("AWS_REGION not set, skipping integration test")
	}

	cfg := ClientConfig{
		Region:           region,
		RetryMaxAttempts: 3,
	}

	client, err := NewClient(cfg)
	require.NoError(t, err, "should create client successfully")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use a non-existent ASG name
	nonExistentASG := "non-existent-asg-" + strconv.FormatInt(time.Now().Unix(), 10)

	_, err = client.GetCapacity(ctx, nonExistentASG)
	require.Error(t, err, "should return error for non-existent ASG")
	assert.Contains(t, err.Error(), "not found", "error should mention ASG not found")

	t.Logf("Got expected error: %v", err)
}

// TestIntegration_ContextTimeout tests timeout handling
func TestIntegration_ContextTimeout(t *testing.T) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		t.Skip("AWS_REGION not set, skipping integration test")
	}

	asgName := os.Getenv("TEST_ASG_NAME")
	if asgName == "" {
		t.Skip("TEST_ASG_NAME not set, skipping integration test")
	}

	cfg := ClientConfig{
		Region:           region,
		RetryMaxAttempts: 3,
	}

	client, err := NewClient(cfg)
	require.NoError(t, err, "should create client successfully")

	// Use very short timeout to trigger timeout error
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	_, err = client.GetCapacity(ctx, asgName)
	assert.Error(t, err, "should return timeout error")

	t.Logf("Got expected error: %v", err)
}

// BenchmarkIntegration_GetCapacity benchmarks real API calls
func BenchmarkIntegration_GetCapacity(b *testing.B) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		b.Skip("AWS_REGION not set, skipping benchmark")
	}

	asgName := os.Getenv("TEST_ASG_NAME")
	if asgName == "" {
		b.Skip("TEST_ASG_NAME not set, skipping benchmark")
	}

	cfg := ClientConfig{
		Region:           region,
		RetryMaxAttempts: 3,
	}

	client, err := NewClient(cfg)
	require.NoError(b, err, "should create client successfully")

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.GetCapacity(ctx, asgName)
		require.NoError(b, err, "should get capacity successfully")
	}
}

// BenchmarkIntegration_GetInstanceIDs benchmarks real API calls
func BenchmarkIntegration_GetInstanceIDs(b *testing.B) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		b.Skip("AWS_REGION not set, skipping benchmark")
	}

	asgName := os.Getenv("TEST_ASG_NAME")
	if asgName == "" {
		b.Skip("TEST_ASG_NAME not set, skipping benchmark")
	}

	cfg := ClientConfig{
		Region:           region,
		RetryMaxAttempts: 3,
	}

	client, err := NewClient(cfg)
	require.NoError(b, err, "should create client successfully")

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.GetInstanceIDs(ctx, asgName)
		require.NoError(b, err, "should get instance IDs successfully")
	}
}
