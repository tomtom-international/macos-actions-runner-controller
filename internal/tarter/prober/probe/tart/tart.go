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

package tart

import (
	"fmt"

	"github.com/tomtom-international/macos-actions-runner-controller/internal/tarter/prober/probe"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/tart"
)

type tartProber struct {
	client *tart.Client
}

type Prober interface {
	Probe(tartVMName string) (probe.Result, string, error)
}

func New(tartClient *tart.Client) Prober {
	return tartProber{
		client: tartClient,
	}
}

func (t tartProber) Probe(tartVMName string) (probe.Result, string, error) {
	_, err := t.client.GetTartVMIP(tartVMName)
	if err != nil {
		return probe.Failure, fmt.Sprintf("Failed to get tart VM %s IP", tartVMName), nil
	}

	return probe.Success, "tart probe success", nil
}
