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
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"math/rand"
	"time"
)

type SQSClient struct {
	service  *sqs.Client
	QueueURL string
}

func NewClient(region, queueURL string) (*SQSClient, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	svc := sqs.NewFromConfig(cfg)

	return &SQSClient{
		service:  svc,
		QueueURL: queueURL,
	}, nil
}

func (c *SQSClient) SendMessage(ctx context.Context, messageBody string) (*sqs.SendMessageOutput, error) {
	result, err := c.service.SendMessage(ctx, &sqs.SendMessageInput{
		MessageBody: aws.String(messageBody),
		QueueUrl:    aws.String(c.QueueURL),
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *SQSClient) DeleteMessage(ctx context.Context, receiptHandle string) (*sqs.DeleteMessageOutput, error) {
	result, err := c.service.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(c.QueueURL),
		ReceiptHandle: aws.String(receiptHandle),
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *SQSClient) ReceiveMessages(ctx context.Context, maxMessages int32, waitTimeSeconds int32) ([]types.Message, error) {
	result, err := c.service.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(c.QueueURL),
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
// Parameters:
//   - current: The current backoff duration.
//   - max: The maximum allowable backoff duration.
//
// Returns:
//
//	A time.Duration representing the next backoff period.
//
// The function doubles the current backoff time, then applies a random jitter
// (subtracting up to 25% of the doubled value) to prevent multiple instances
// from synchronizing their polling cycles. The result is capped at the specified
// maximum duration.
func (c *SQSClient) CalculateNextBackoff(current, maxDuration time.Duration) time.Duration {
	// Double the current backoff
	next := current * 2

	// Apply jitter (randomness) to prevent synchronized polling
	jitter := time.Duration(rand.Int63n(int64(next / 4)))
	next -= jitter

	// Ensure we don't exceed the maximum
	if next > maxDuration {
		return maxDuration
	}

	return next
}
