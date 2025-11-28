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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const defaultRequestTimeout = 5

type ClientConfig struct {
	Endpoints      []string
	RequestTimeout int // in seconds

	TLSEnabled  bool
	TLSCertFile string // Path to client certificate
	TLSKeyFile  string // Path to client key
	TLSCAFile   string // Path to CA certificate

	Username string
	Password string
}

type Client struct {
	ctx    context.Context
	client *clientv3.Client
	cancel context.CancelFunc
	cfg    ClientConfig
}

func NewEtcdClient(cfg ClientConfig) (*Client, error) {
	if len(cfg.Endpoints) == 0 {
		return nil, fmt.Errorf("etcd endpoints are required")
	}

	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = defaultRequestTimeout
	}

	ctx, cancel := context.WithCancel(context.Background())

	etcdConfig := clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: time.Duration(cfg.RequestTimeout) * time.Second,
	}

	if cfg.TLSEnabled {
		tlsConfig, err := loadTLSConfig(cfg.TLSCertFile, cfg.TLSKeyFile, cfg.TLSCAFile)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to load TLS config: %w", err)
		}
		etcdConfig.TLS = tlsConfig
	}

	if cfg.Username != "" && cfg.Password != "" {
		etcdConfig.Username = cfg.Username
		etcdConfig.Password = cfg.Password
	}

	client, err := clientv3.New(etcdConfig)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}

	// Verify that client can connect to at least one endpoint
	connCtx, connCancel := context.WithTimeout(context.Background(), time.Duration(cfg.RequestTimeout)*time.Second)
	defer connCancel()

	connected := false
	var lastErr error
	for _, endpoint := range cfg.Endpoints {
		_, err := client.Status(connCtx, endpoint)
		if err == nil {
			connected = true
			break
		}
		lastErr = err
	}

	if !connected {
		client.Close()
		cancel()
		if lastErr != nil {
			return nil, fmt.Errorf("failed to connect to etcd: no reachable endpoints: %w", lastErr)
		}
		return nil, fmt.Errorf("failed to connect to etcd: no reachable endpoints")
	}

	etcdClient := &Client{
		cfg:    cfg,
		client: client,
		ctx:    ctx,
		cancel: cancel,
	}

	return etcdClient, nil
}

func (c *Client) Put(key, value string) error {
	ctx, cancel := context.WithTimeout(c.ctx, time.Duration(c.cfg.RequestTimeout)*time.Second)
	defer cancel()
	_, err := c.client.Put(ctx, key, value)
	return err
}

func (c *Client) PutWithLease(key, value string, leaseID clientv3.LeaseID) error {
	ctx, cancel := context.WithTimeout(c.ctx, time.Duration(c.cfg.RequestTimeout)*time.Second)
	defer cancel()
	_, err := c.client.Put(ctx, key, value, clientv3.WithLease(leaseID))
	return err
}

func (c *Client) Get(key string) (string, error) {
	ctx, cancel := context.WithTimeout(c.ctx, time.Duration(c.cfg.RequestTimeout)*time.Second)
	defer cancel()
	resp, err := c.client.Get(ctx, key)
	if err != nil {
		return "", err
	}
	if len(resp.Kvs) > 0 {
		return string(resp.Kvs[0].Value), nil
	}
	return "", nil
}

func (c *Client) GetByPrefix(prefix string) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(c.ctx, time.Duration(c.cfg.RequestTimeout)*time.Second)
	defer cancel()
	resp, err := c.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, kv := range resp.Kvs {
		result[string(kv.Key)] = string(kv.Value)
	}
	return result, nil
}

func (c *Client) Watch(ctx context.Context, key string) clientv3.WatchChan {
	return c.client.Watch(ctx, key)
}

func (c *Client) WatchWithPrefix(ctx context.Context, prefix string) clientv3.WatchChan {
	return c.client.Watch(ctx, prefix, clientv3.WithPrefix())
}

func (c *Client) Close() error {
	c.cancel()

	if err := c.client.Close(); err != nil {
		return fmt.Errorf("failed to close etcd client: %s", err.Error())
	}
	return nil
}

func (c *Client) GrantLease(ttl int) (clientv3.LeaseID, error) {
	ctx, cancel := context.WithTimeout(c.ctx, time.Duration(c.cfg.RequestTimeout)*time.Second)
	defer cancel()
	lease, err := c.client.Grant(ctx, int64(ttl))
	return lease.ID, err
}

// loadTLSConfig creates a TLS configuration for etcd client authentication
func loadTLSConfig(certFile, keyFile, caFile string) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	// Load custom CA certificate if provided (for self-signed certs)
	// If not provided, system CA pool is used
	if caFile != "" {
		caCert, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	// Load client certificate and key if provided (for mutual TLS)
	if certFile != "" && keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	} else if certFile != "" || keyFile != "" {
		return nil, fmt.Errorf("both certFile and keyFile must be provided together for client certificate authentication")
	}

	return tlsConfig, nil
}
