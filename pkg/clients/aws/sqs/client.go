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
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

const (
	defaultMinBackoff       = 1 * time.Second
	defaultMaxBackoff       = 60 * time.Second
	defaultRetryMaxAttempts = 3
)

type SQSClient struct {
	client         *sqs.Client
	config         *SQSConfig
	errHandle      func(error)
	currentBackoff time.Duration
}

type SQSConfig struct {
	ErrHandle        func(error)
	QueueURL         string
	AWSRegion        string
	MaxBackoff       time.Duration
	MinBackoff       time.Duration
	RetryMaxAttempts int
}

// NewClient creates a new SQSClient with the provided configuration.
// It loads AWS credentials from the environment and establishes a connection.
// Returns an error if the configuration is invalid or AWS credentials cannot be loaded.
func NewClient(sqsCfg *SQSConfig) (*SQSClient, error) {
	err := validateSQSConfig(sqsCfg)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(sqsCfg.AWSRegion),
		config.WithRetryMaxAttempts(sqsCfg.RetryMaxAttempts),
		config.WithRetryMode(aws.RetryModeStandard),
	)
	if err != nil {
		return nil, err
	}

	c := sqs.NewFromConfig(cfg)

	return &SQSClient{
		client:         c,
		config:         sqsCfg,
		currentBackoff: sqsCfg.MinBackoff,
		errHandle:      sqsCfg.ErrHandle,
	}, nil
}

func validateSQSConfig(cfg *SQSConfig) error {
	if cfg == nil {
		return errors.New("sqsConfig is required")
	}
	if cfg.QueueURL == "" {
		return errors.New("queueURL is required")
	}
	if cfg.AWSRegion == "" {
		return errors.New("region is required")
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = defaultMaxBackoff
	}
	if cfg.MinBackoff <= 0 {
		cfg.MinBackoff = defaultMinBackoff
	}
	if cfg.RetryMaxAttempts <= 0 {
		cfg.RetryMaxAttempts = defaultRetryMaxAttempts
	}
	if cfg.ErrHandle == nil {
		cfg.ErrHandle = func(err error) {}
	}
	return nil
}

// SendMessage sends a message to the configured SQS queue.
// Returns the AWS SendMessageOutput containing the message ID, or an error if the send fails.
func (c *SQSClient) SendMessage(ctx context.Context, messageBody string) (*sqs.SendMessageOutput, error) {
	result, err := c.client.SendMessage(ctx, &sqs.SendMessageInput{
		MessageBody: aws.String(messageBody),
		QueueUrl:    aws.String(c.config.QueueURL),
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// DeleteMessage removes a message from the queue using its receipt handle.
// The receipt handle is obtained from ReceiveMessages.
// Returns an error if the message cannot be deleted.
func (c *SQSClient) DeleteMessage(ctx context.Context, receiptHandle string) (*sqs.DeleteMessageOutput, error) {
	result, err := c.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(c.config.QueueURL),
		ReceiptHandle: aws.String(receiptHandle),
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// ReceiveMessages polls the SQS queue for up to maxMessages messages.
// It uses long polling with waitTimeSeconds to reduce empty responses.
// Returns a slice of messages or an error if the receive operation fails.
func (c *SQSClient) ReceiveMessages(ctx context.Context, maxMessages int32, waitTimeSeconds int32) ([]types.Message, error) {
	result, err := c.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(c.config.QueueURL),
		MaxNumberOfMessages: maxMessages,
		WaitTimeSeconds:     waitTimeSeconds,
	})

	if err != nil {
		return nil, err
	}

	return result.Messages, nil
}

// CalculateNextBackoff computes the next backoff duration using an exponential
// strategy with jitter to prevent synchronized polling.
//
// Returns:
//
//	A time.Duration representing the next backoff period.
//
// The function doubles the current backoff time, then applies a random jitter
// (subtracting up to 25% of the doubled value) to prevent multiple instances
// from synchronizing their polling cycles. The result is capped at the specified
// maximum duration.
func (c *SQSClient) calculateNextBackoff() time.Duration {
	// Double the current backoff
	next := c.currentBackoff * 2

	// Apply jitter (randomness) to prevent synchronized polling
	jitter := time.Duration(rand.Int63n(int64(next / 4)))
	next -= jitter

	// Ensure we don't exceed the maximum
	if next > c.config.MaxBackoff {
		c.currentBackoff = c.config.MaxBackoff
	} else {
		c.currentBackoff = next
	}

	return c.currentBackoff
}

func (c *SQSClient) resetBackoff() {
	c.currentBackoff = c.config.MinBackoff
}

// PollForMessages continuously polls the SQS queue and invokes the handler for each message.
// It implements exponential backoff with jitter when no messages are available.
// Successfully processed messages are automatically deleted from the queue.
// Polling stops when the context is canceled. Handler errors are sent to the configured error handler.
//
//gocyclo:ignore
func (c *SQSClient) PollForMessages(ctx context.Context, handler func(message *types.Message) error) {
	errChan := make(chan error, 100)
	pollCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		defer close(errChan)
		for {
			select {
			case <-pollCtx.Done():
				return
			case err := <-errChan:
				if err != nil && c.errHandle != nil {
					c.errHandle(err)
				}
			}
		}
	}()

	var backoff time.Duration

	for {
		select {
		case <-ctx.Done():
			// Context was canceled - exit gracefully without error
			return
		default:
			messages, err := c.ReceiveMessages(ctx, 10, 20)
			if err != nil {
				// Check if the error is due to context cancellation
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					// Context canceled while receiving messages, stopping polling
					return
				}
				errChan <- fmt.Errorf("failed to receive messages: %w", err)

				// Increase backoff on errors
				backoff = c.calculateNextBackoff()
				time.Sleep(backoff)
				continue
			}

			if len(messages) == 0 {
				// No messages found, increase the backoff
				backoff = c.calculateNextBackoff()
				time.Sleep(backoff)
			} else {
				// Messages found, reset backoff
				c.resetBackoff()

				for _, msg := range messages {
					err = handler(&msg)
					if err != nil {
						continue
					}

					_, err = c.DeleteMessage(ctx, *msg.ReceiptHandle)
					if err != nil {
						errChan <- fmt.Errorf("failed to delete message %s from queue %s: %w", *msg.MessageId,
							c.config.QueueURL, err)
						continue
					}
				}
			}
		}
	}
}
