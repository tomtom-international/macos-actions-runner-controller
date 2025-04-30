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

package etcd

import (
	"context"
	"fmt"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type EtcdClient struct {
	client *clientv3.Client

	ctx       context.Context
	cancel    context.CancelFunc
	ErrChan   chan error
	errHandle func(error)
}

const requestTimeout = 5

func NewEtcdClient(endpoints []string, errHandle func(error)) (*EtcdClient, error) {
	ctx, cancel := context.WithCancel(context.Background())

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: requestTimeout * time.Second,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}

	etcdClient := &EtcdClient{
		client:    client,
		ctx:       ctx,
		cancel:    cancel,
		ErrChan:   make(chan error, 100),
		errHandle: errHandle,
	}

	// TODO: should monitor errors be implemented in the client or in the manager?
	go etcdClient.monitorErrors()

	return etcdClient, nil
}

func (e *EtcdClient) Put(key, value string) error {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout*time.Second)
	defer cancel()
	_, err := e.client.Put(ctx, key, value)
	return err
}

func (e *EtcdClient) PutWithLease(key, value string, leaseID clientv3.LeaseID) error {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout*time.Second)
	defer cancel()
	_, err := e.client.Put(ctx, key, value, clientv3.WithLease(leaseID))
	return err
}

func (e *EtcdClient) Get(key string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout*time.Second)
	defer cancel()
	resp, err := e.client.Get(ctx, key)
	if err != nil {
		return "", err
	}
	if len(resp.Kvs) > 0 {
		return string(resp.Kvs[0].Value), nil
	}
	return "", nil
}

func (e *EtcdClient) GetByPrefix(prefix string) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout*time.Second)
	defer cancel()
	resp, err := e.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, kv := range resp.Kvs {
		result[string(kv.Key)] = string(kv.Value)
	}
	return result, nil
}

func (e *EtcdClient) Watch(ctx context.Context, key string) clientv3.WatchChan {
	return e.client.Watch(ctx, key)
}

func (e *EtcdClient) WatchWithPrefix(ctx context.Context, prefix string) clientv3.WatchChan {
	return e.client.Watch(ctx, prefix, clientv3.WithPrefix())
}

func (e *EtcdClient) Close() error {
	e.cancel()

	if err := e.client.Close(); err != nil {
		return fmt.Errorf("failed to close etcd client: %s", err.Error())
	}
	close(e.ErrChan)
	return nil
}

func (e *EtcdClient) monitorErrors() {
	for err := range e.ErrChan {
		e.errHandle(err)
	}
}

func (e *EtcdClient) GrantLease(ttl int) (clientv3.LeaseID, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout*time.Second)
	defer cancel()
	lease, err := e.client.Grant(ctx, int64(ttl))
	return lease.ID, err
}
