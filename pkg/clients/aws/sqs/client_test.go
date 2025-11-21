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

package sqs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewClient_NilConfig tests that nil config returns error
func TestNewClient_NilConfig(t *testing.T) {
	client, err := NewClient(nil)

	require.Error(t, err, "should return error for nil config")
	assert.Nil(t, client, "client should be nil on error")
	assert.EqualError(t, err, "sqsConfig is required")
}

// TestNewClient_EmptyQueueURL tests that empty queue URL returns error
func TestNewClient_EmptyQueueURL(t *testing.T) {
	cfg := &SQSConfig{
		QueueURL:         "",
		AWSRegion:        "us-east-1",
		RetryMaxAttempts: 3,
	}

	client, err := NewClient(cfg)

	require.Error(t, err, "should return error for empty queue URL")
	assert.Nil(t, client, "client should be nil on error")
	assert.EqualError(t, err, "queueURL is required")
}

// TestNewClient_EmptyRegion tests that empty region returns error
func TestNewClient_EmptyRegion(t *testing.T) {
	cfg := &SQSConfig{
		QueueURL:         "https://sqs.us-east-1.amazonaws.com/123456789012/my-queue",
		AWSRegion:        "",
		RetryMaxAttempts: 3,
	}

	client, err := NewClient(cfg)

	require.Error(t, err, "should return error for empty region")
	assert.Nil(t, client, "client should be nil on error")
	assert.EqualError(t, err, "region is required")
}

// TestNewClient_DefaultValues tests that default values are applied
func TestNewClient_DefaultValues(t *testing.T) {
	cfg := &SQSConfig{
		QueueURL:  "https://sqs.us-east-1.amazonaws.com/123456789012/my-queue",
		AWSRegion: "us-east-1",
	}

	_, _ = NewClient(cfg) // Will fail without AWS creds, but validation should pass

	assert.Equal(t, defaultMaxBackoff, cfg.MaxBackoff, "should apply default MaxBackoff")
	assert.Equal(t, defaultMinBackoff, cfg.MinBackoff, "should apply default MinBackoff")
	assert.Equal(t, defaultRetryMaxAttempts, cfg.RetryMaxAttempts, "should apply default RetryMaxAttempts")
	assert.NotNil(t, cfg.ErrHandle, "should apply default ErrHandle")
}

// TestNewClient_CustomErrHandle tests that custom error handler is preserved
func TestNewClient_CustomErrHandle(t *testing.T) {
	customCalled := false
	customHandler := func(err error) {
		customCalled = true
	}

	cfg := &SQSConfig{
		QueueURL:  "https://sqs.us-east-1.amazonaws.com/123456789012/my-queue",
		AWSRegion: "us-east-1",
		ErrHandle: customHandler,
	}

	_, _ = NewClient(cfg)

	assert.NotNil(t, cfg.ErrHandle, "should preserve custom ErrHandle")
	cfg.ErrHandle(nil) // Call it
	assert.True(t, customCalled, "custom error handler should be called")
}

// TestValidateSQSConfig_AllErrors tests all validation error cases
func TestValidateSQSConfig_AllErrors(t *testing.T) {
	tests := []struct {
		name        string
		config      *SQSConfig
		expectedErr string
	}{
		{
			name:        "nil config",
			config:      nil,
			expectedErr: "sqsConfig is required",
		},
		{
			name: "empty queue URL",
			config: &SQSConfig{
				AWSRegion: "us-east-1",
			},
			expectedErr: "queueURL is required",
		},
		{
			name: "empty region",
			config: &SQSConfig{
				QueueURL: "https://sqs.us-east-1.amazonaws.com/123456789012/my-queue",
			},
			expectedErr: "region is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSQSConfig(tt.config)
			require.Error(t, err, "should return error")
			assert.EqualError(t, err, tt.expectedErr)
		})
	}
}

// TestValidateSQSConfig_AppliesDefaults tests that defaults are applied correctly
func TestValidateSQSConfig_AppliesDefaults(t *testing.T) {
	cfg := &SQSConfig{
		QueueURL:  "https://sqs.us-east-1.amazonaws.com/123456789012/my-queue",
		AWSRegion: "us-east-1",
	}

	err := validateSQSConfig(cfg)

	require.NoError(t, err, "should not return error for valid config")
	assert.Equal(t, defaultMaxBackoff, cfg.MaxBackoff)
	assert.Equal(t, defaultMinBackoff, cfg.MinBackoff)
	assert.Equal(t, defaultRetryMaxAttempts, cfg.RetryMaxAttempts)
	assert.NotNil(t, cfg.ErrHandle)
}

// TestCalculateNextBackoff tests backoff calculation logic
func TestCalculateNextBackoff(t *testing.T) {
	client := &SQSClient{
		config: &SQSConfig{
			MinBackoff: 1 * time.Second,
			MaxBackoff: 60 * time.Second,
		},
		currentBackoff: 1 * time.Second,
	}

	next := client.calculateNextBackoff()
	assert.GreaterOrEqual(t, next, 1*time.Second, "should be at least min backoff")
	assert.LessOrEqual(t, next, 4*time.Second, "should be at most 2x with jitter")

	for i := 0; i < 10; i++ {
		next = client.calculateNextBackoff()
	}

	assert.Equal(t, 60*time.Second, client.currentBackoff, "should cap at max backoff")
	assert.Equal(t, 60*time.Second, next, "should return max backoff")
}

// TestResetBackoff tests backoff reset logic
func TestResetBackoff(t *testing.T) {
	client := &SQSClient{
		config: &SQSConfig{
			MinBackoff: 1 * time.Second,
			MaxBackoff: 60 * time.Second,
		},
		currentBackoff: 30 * time.Second,
	}

	client.resetBackoff()

	assert.Equal(t, 1*time.Second, client.currentBackoff, "should reset to min backoff")
}

// Note: SQS client does not validate empty message bodies, receipt handles, or zero max messages
// at the client level. These are validated by AWS API. Therefore, we don't have unit tests
// for these scenarios - they are covered by integration tests instead.

// Benchmark tests
func BenchmarkCalculateNextBackoff(b *testing.B) {
	client := &SQSClient{
		config: &SQSConfig{
			MinBackoff: 1 * time.Second,
			MaxBackoff: 60 * time.Second,
		},
		currentBackoff: 1 * time.Second,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.calculateNextBackoff()
		if i%100 == 0 {
			client.resetBackoff()
		}
	}
}

func BenchmarkValidateSQSConfig(b *testing.B) {
	cfg := &SQSConfig{
		QueueURL:  "https://sqs.us-east-1.amazonaws.com/123456789012/my-queue",
		AWSRegion: "us-east-1",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validateSQSConfig(cfg)
	}
}
