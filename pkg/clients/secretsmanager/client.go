package secretsmanager

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	u "github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
)

// SCM is a Secrets Manager client
type SCM struct {
	service *secretsmanager.Client
}

func New(region string) (*SCM, error) {
	conf, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	svc := secretsmanager.NewFromConfig(conf)
	return &SCM{
		service: svc,
	}, nil
}

func (smc *SCM) GetSecret(secretName string) (u.Int32String, error) {
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	}

	result, err := smc.service.GetSecretValue(context.TODO(), input)
	if err != nil {
		return u.FromString(""), err
	}

	return u.FromString(*result.SecretString), nil
}
