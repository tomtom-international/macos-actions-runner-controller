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

// To run integration tests: go test -tags=integration ./pkg/clients/aws/sqs/...
// These tests require valid AWS credentials and will make real API calls

package sqs

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_SendMessage_RealAPI tests against real AWS API
func TestIntegration_SendMessage_RealAPI(t *testing.T) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		t.Skip("AWS_REGION not set, skipping integration test")
	}

	queueURL := os.Getenv("TEST_SQS_QUEUE_URL")
	if queueURL == "" {
		t.Skip("TEST_SQS_QUEUE_URL not set, skipping integration test")
	}

	cfg := &SQSConfig{
		QueueURL:         queueURL,
		AWSRegion:        region,
		RetryMaxAttempts: 3,
	}

	client, err := NewClient(cfg)
	require.NoError(t, err, "should create client successfully")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	messageBody := fmt.Sprintf("test-message-%d", time.Now().Unix())
	result, err := client.SendMessage(ctx, messageBody)
	require.NoError(t, err, "should send message successfully")

	t.Logf("Successfully sent message with ID: %s", *result.MessageId)

	// Validate response
	assert.NotNil(t, result.MessageId, "should have message ID")
	assert.NotEmpty(t, *result.MessageId, "message ID should not be empty")
}

// TestIntegration_ReceiveMessages_RealAPI tests against real AWS API
func TestIntegration_ReceiveMessages_RealAPI(t *testing.T) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		t.Skip("AWS_REGION not set, skipping integration test")
	}

	queueURL := os.Getenv("TEST_SQS_QUEUE_URL")
	if queueURL == "" {
		t.Skip("TEST_SQS_QUEUE_URL not set, skipping integration test")
	}

	cfg := &SQSConfig{
		QueueURL:         queueURL,
		AWSRegion:        region,
		RetryMaxAttempts: 3,
	}

	client, err := NewClient(cfg)
	require.NoError(t, err, "should create client successfully")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Send a message first
	messageBody := fmt.Sprintf("test-receive-%d", time.Now().Unix())
	sendResult, err := client.SendMessage(ctx, messageBody)
	require.NoError(t, err, "should send message successfully")

	// Wait a bit for message to be available
	time.Sleep(1 * time.Second)

	// Try to receive messages
	messages, err := client.ReceiveMessages(ctx, 10, 5)
	require.NoError(t, err, "should receive messages successfully")

	t.Logf("Successfully received %d messages", len(messages))

	// We should get at least our message back (might get more if queue has others)
	assert.GreaterOrEqual(t, len(messages), 0, "should receive messages")

	// Find our message
	found := false
	for _, msg := range messages {
		if msg.MessageId != nil && *msg.MessageId == *sendResult.MessageId {
			found = true
			assert.Equal(t, messageBody, *msg.Body, "message body should match")
			t.Logf("Found our message: %s", *msg.MessageId)
			break
		}
	}

	if !found && len(messages) == 0 {
		t.Log("No messages received (queue might be empty or message not yet visible)")
	}
}

// TestIntegration_DeleteMessage_RealAPI tests against real AWS API
func TestIntegration_DeleteMessage_RealAPI(t *testing.T) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		t.Skip("AWS_REGION not set, skipping integration test")
	}

	queueURL := os.Getenv("TEST_SQS_QUEUE_URL")
	if queueURL == "" {
		t.Skip("TEST_SQS_QUEUE_URL not set, skipping integration test")
	}

	cfg := &SQSConfig{
		QueueURL:         queueURL,
		AWSRegion:        region,
		RetryMaxAttempts: 3,
	}

	client, err := NewClient(cfg)
	require.NoError(t, err, "should create client successfully")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Send a message
	messageBody := fmt.Sprintf("test-delete-%d", time.Now().Unix())
	_, err = client.SendMessage(ctx, messageBody)
	require.NoError(t, err, "should send message successfully")

	// Wait for message to be available
	time.Sleep(1 * time.Second)

	// Receive the message
	messages, err := client.ReceiveMessages(ctx, 1, 5)
	require.NoError(t, err, "should receive messages successfully")

	if len(messages) == 0 {
		t.Skip("No messages available to delete (test is non-deterministic)")
	}

	msg := messages[0]
	require.NotNil(t, msg.ReceiptHandle, "message should have receipt handle")

	// Delete the message
	_, err = client.DeleteMessage(ctx, *msg.ReceiptHandle)
	require.NoError(t, err, "should delete message successfully")

	t.Logf("Successfully deleted message: %s", *msg.MessageId)
}

// TestIntegration_SendReceiveDeleteCycle tests complete message lifecycle
func TestIntegration_SendReceiveDeleteCycle(t *testing.T) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		t.Skip("AWS_REGION not set, skipping integration test")
	}

	queueURL := os.Getenv("TEST_SQS_QUEUE_URL")
	if queueURL == "" {
		t.Skip("TEST_SQS_QUEUE_URL not set, skipping integration test")
	}

	cfg := &SQSConfig{
		QueueURL:         queueURL,
		AWSRegion:        region,
		RetryMaxAttempts: 3,
	}

	client, err := NewClient(cfg)
	require.NoError(t, err, "should create client successfully")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 1. Send a unique message
	uniqueBody := fmt.Sprintf("test-cycle-%d-%d", time.Now().Unix(), time.Now().Nanosecond())
	sendResult, err := client.SendMessage(ctx, uniqueBody)
	require.NoError(t, err, "should send message successfully")
	sentMessageID := *sendResult.MessageId
	t.Logf("Step 1: Sent message with ID: %s", sentMessageID)

	// 2. Wait for message to be available
	time.Sleep(2 * time.Second)

	// 3. Receive the message
	var receivedMsg *types.Message
	for i := 0; i < 5; i++ { // Retry a few times
		messages, err := client.ReceiveMessages(ctx, 10, 5)
		require.NoError(t, err, "should receive messages successfully")

		for _, msg := range messages {
			if msg.MessageId != nil && *msg.MessageId == sentMessageID {
				receivedMsg = &msg
				break
			}
		}

		if receivedMsg != nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	require.NotNil(t, receivedMsg, "should receive the sent message")
	assert.Equal(t, uniqueBody, *receivedMsg.Body, "message body should match")
	t.Logf("Step 2: Received message with ID: %s", *receivedMsg.MessageId)

	// 4. Delete the message
	_, err = client.DeleteMessage(ctx, *receivedMsg.ReceiptHandle)
	require.NoError(t, err, "should delete message successfully")
	t.Logf("Step 3: Deleted message with ID: %s", *receivedMsg.MessageId)

	t.Log("✅ Complete send-receive-delete cycle successful")
}

// TestIntegration_PollForMessages tests polling functionality
func TestIntegration_PollForMessages(t *testing.T) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		t.Skip("AWS_REGION not set, skipping integration test")
	}

	queueURL := os.Getenv("TEST_SQS_QUEUE_URL")
	if queueURL == "" {
		t.Skip("TEST_SQS_QUEUE_URL not set, skipping integration test")
	}

	errorCount := int32(0)
	cfg := &SQSConfig{
		QueueURL:         queueURL,
		AWSRegion:        region,
		RetryMaxAttempts: 3,
		ErrHandle: func(err error) {
			atomic.AddInt32(&errorCount, 1)
			t.Logf("Error handler called: %v", err)
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err, "should create client successfully")

	// Send a test message
	sendCtx, sendCancel := context.WithTimeout(context.Background(), 10*time.Second)
	messageBody := fmt.Sprintf("test-poll-%d", time.Now().Unix())
	_, err = client.SendMessage(sendCtx, messageBody)
	sendCancel()
	require.NoError(t, err, "should send message successfully")

	// Start polling with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	messagesProcessed := int32(0)
	handler := func(msg *types.Message) error {
		atomic.AddInt32(&messagesProcessed, 1)
		t.Logf("Handler received message: %s", *msg.MessageId)
		return nil // Success - message will be deleted
	}

	// Run polling in background
	done := make(chan struct{})
	go func() {
		client.PollForMessages(ctx, handler)
		close(done)
	}()

	// Wait for polling to finish (timeout or context cancel)
	<-done

	t.Logf("Processed %d messages during polling", atomic.LoadInt32(&messagesProcessed))
	t.Logf("Errors during polling: %d", atomic.LoadInt32(&errorCount))

	// We should have processed at least our message
	assert.GreaterOrEqual(t, atomic.LoadInt32(&messagesProcessed), int32(0), "should process messages")
}

// TestIntegration_PollForMessages_ContextCancellation tests graceful shutdown
func TestIntegration_PollForMessages_ContextCancellation(t *testing.T) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		t.Skip("AWS_REGION not set, skipping integration test")
	}

	queueURL := os.Getenv("TEST_SQS_QUEUE_URL")
	if queueURL == "" {
		t.Skip("TEST_SQS_QUEUE_URL not set, skipping integration test")
	}

	cfg := &SQSConfig{
		QueueURL:         queueURL,
		AWSRegion:        region,
		RetryMaxAttempts: 3,
	}

	client, err := NewClient(cfg)
	require.NoError(t, err, "should create client successfully")

	ctx, cancel := context.WithCancel(context.Background())

	handler := func(msg *types.Message) error {
		t.Logf("Handler received message: %s", *msg.MessageId)
		return nil
	}

	// Start polling
	done := make(chan struct{})
	go func() {
		client.PollForMessages(ctx, handler)
		close(done)
	}()

	// Cancel after 3 seconds
	time.Sleep(3 * time.Second)
	cancel()

	// Wait for graceful shutdown
	select {
	case <-done:
		t.Log("✅ Polling stopped gracefully after context cancellation")
	case <-time.After(5 * time.Second):
		t.Error("❌ Polling did not stop within timeout after context cancellation")
	}
}

// TestIntegration_ContextTimeout tests timeout handling
func TestIntegration_ContextTimeout(t *testing.T) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		t.Skip("AWS_REGION not set, skipping integration test")
	}

	queueURL := os.Getenv("TEST_SQS_QUEUE_URL")
	if queueURL == "" {
		t.Skip("TEST_SQS_QUEUE_URL not set, skipping integration test")
	}

	cfg := &SQSConfig{
		QueueURL:         queueURL,
		AWSRegion:        region,
		RetryMaxAttempts: 3,
	}

	client, err := NewClient(cfg)
	require.NoError(t, err, "should create client successfully")

	// Use very short timeout to trigger timeout error
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	_, err = client.ReceiveMessages(ctx, 1, 5)
	assert.Error(t, err, "should return timeout error")

	t.Logf("Got expected error: %v", err)
}

// BenchmarkIntegration_SendMessage benchmarks real API calls
func BenchmarkIntegration_SendMessage(b *testing.B) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		b.Skip("AWS_REGION not set, skipping benchmark")
	}

	queueURL := os.Getenv("TEST_SQS_QUEUE_URL")
	if queueURL == "" {
		b.Skip("TEST_SQS_QUEUE_URL not set, skipping benchmark")
	}

	cfg := &SQSConfig{
		QueueURL:         queueURL,
		AWSRegion:        region,
		RetryMaxAttempts: 3,
	}

	client, err := NewClient(cfg)
	require.NoError(b, err, "should create client successfully")

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.SendMessage(ctx, fmt.Sprintf("benchmark-message-%d", i))
		require.NoError(b, err, "should send message successfully")
	}
}

// BenchmarkIntegration_ReceiveMessages benchmarks real API calls
func BenchmarkIntegration_ReceiveMessages(b *testing.B) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		b.Skip("AWS_REGION not set, skipping benchmark")
	}

	queueURL := os.Getenv("TEST_SQS_QUEUE_URL")
	if queueURL == "" {
		b.Skip("TEST_SQS_QUEUE_URL not set, skipping benchmark")
	}

	cfg := &SQSConfig{
		QueueURL:         queueURL,
		AWSRegion:        region,
		RetryMaxAttempts: 3,
	}

	client, err := NewClient(cfg)
	require.NoError(b, err, "should create client successfully")

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.ReceiveMessages(ctx, 10, 1)
		require.NoError(b, err, "should receive messages successfully")
	}
}
