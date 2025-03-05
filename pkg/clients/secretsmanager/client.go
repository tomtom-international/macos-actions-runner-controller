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
