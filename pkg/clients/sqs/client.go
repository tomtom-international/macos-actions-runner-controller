package sqs

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
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
