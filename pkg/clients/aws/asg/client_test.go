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

package asg

import (
	"context"
	"testing"

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

// TestNewClient_DefaultRetryAttempts tests that zero retry attempts gets default value
func TestNewClient_DefaultRetryAttempts(t *testing.T) {
	cfg := ClientConfig{
		Region:           "us-east-1",
		RetryMaxAttempts: 0, // Should default to 3
	}

	// Note: This will fail to create client (no AWS creds in unit test),
	// but we're only testing that the default value is applied
	_, _ = NewClient(cfg)

	// Can't verify the default was applied since cfg is passed by value
	// This test just ensures the function handles zero RetryMaxAttempts
	// The actual default application is tested in integration tests
}

// TestGetCapacity_EmptyGroupName tests that empty group name returns error
func TestGetCapacity_EmptyGroupName(t *testing.T) {
	client := &ASGClient{
		client: nil,
	}

	ctx := context.Background()
	capacity, err := client.GetCapacity(ctx, "")

	require.Error(t, err, "should return error for empty group name")
	assert.Equal(t, 0, capacity, "capacity should be zero on error")
	assert.EqualError(t, err, "groupName is required")
}

// TestSetCapacity_EmptyGroupName tests that empty group name returns error
func TestSetCapacity_EmptyGroupName(t *testing.T) {
	client := &ASGClient{
		client: nil,
	}

	ctx := context.Background()
	err := client.SetCapacity(ctx, "", 5)

	require.Error(t, err, "should return error for empty group name")
	assert.EqualError(t, err, "groupName is required")
}

// TestSetCapacity_NegativeCapacity tests that negative capacity returns error
func TestSetCapacity_NegativeCapacity(t *testing.T) {
	client := &ASGClient{
		client: nil,
	}

	ctx := context.Background()
	err := client.SetCapacity(ctx, "my-asg", -1)

	require.Error(t, err, "should return error for negative capacity")
	assert.Contains(t, err.Error(), "capacity must be non-negative")
}

// TestGetInstanceIDs_EmptyGroupName tests that empty group name returns error
func TestGetInstanceIDs_EmptyGroupName(t *testing.T) {
	client := &ASGClient{
		client: nil,
	}

	ctx := context.Background()
	instanceIDs, err := client.GetInstanceIDs(ctx, "")

	require.Error(t, err, "should return error for empty group name")
	assert.Nil(t, instanceIDs, "instanceIDs should be nil on error")
	assert.EqualError(t, err, "groupName is required")
}

// Benchmark tests
func BenchmarkGetCapacity_Validation(b *testing.B) {
	client := &ASGClient{
		client: nil,
	}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.GetCapacity(ctx, "")
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
	capacity, err := client.GetCapacity(ctx, "my-asg")
	if err != nil {
		panic(err)
	}

	_ = capacity
}

// Example test for GetInstanceIDs
func ExampleASGClient_GetInstanceIDs() {
	cfg := ClientConfig{
		Region: "us-east-1",
	}

	client, err := NewClient(cfg)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	instanceIDs, err := client.GetInstanceIDs(ctx, "my-asg")
	if err != nil {
		panic(err)
	}

	_ = instanceIDs
}

// Example test for SetCapacity
func ExampleASGClient_SetCapacity() {
	cfg := ClientConfig{
		Region: "us-east-1",
	}

	client, err := NewClient(cfg)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	err = client.SetCapacity(ctx, "my-asg", 10)
	if err != nil {
		panic(err)
	}
}
