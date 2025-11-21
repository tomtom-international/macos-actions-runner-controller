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

	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"go.etcd.io/etcd/client/v3/concurrency"
)

const (
	defaultSessionTimeout = 10 // seconds
	defaultTTL            = 15 // seconds
)

type ElectionConfig struct {
	Identity       string // Unique replica ID
	ElectionKey    string // Key used for leader election
	TTL            int    // Session TTL in seconds
	SessionTimeout int    // Timeout for session creation in seconds
}

// LeaderElection handles leader election using etcd
type LeaderElection struct {
	session  *concurrency.Session
	election *concurrency.Election
	client   *Client
	identity string // Unique replica ID
}

// NewLeaderElection creates a new leader election instance
func (c *Client) NewLeaderElection(cfg ElectionConfig) (*LeaderElection, error) {
	if cfg.TTL <= 0 {
		cfg.TTL = defaultTTL
	}
	if cfg.SessionTimeout <= 0 {
		cfg.SessionTimeout = defaultSessionTimeout
	}
	if cfg.Identity == "" {
		return nil, fmt.Errorf("election identity is required")
	}
	if cfg.ElectionKey == "" {
		return nil, fmt.Errorf("election key is required")
	}

	type sessionResult struct {
		session *concurrency.Session
		err     error
	}

	sessionCh := make(chan sessionResult, 1)

	go func() {
		session, err := concurrency.NewSession(c.client, concurrency.WithTTL(cfg.TTL))
		sessionCh <- sessionResult{session: session, err: err}
	}()

	select {
	case res := <-sessionCh:
		if res.err != nil {
			return nil, fmt.Errorf("failed to create etcd session: %w", res.err)
		}
		election := concurrency.NewElection(res.session, cfg.ElectionKey)
		return &LeaderElection{
			session:  res.session,
			election: election,
			client:   c,
			identity: cfg.Identity,
		}, nil
	case <-time.After(time.Duration(cfg.SessionTimeout) * time.Second):
		return nil, fmt.Errorf("timeout creating leader election: etcd may be unavailable")
	}
}

// Campaign starts campaigning to become leader
// Blocks until this instance becomes the leader
func (le *LeaderElection) Campaign(ctx context.Context) error {
	logger.Infof("Starting leader election campaign as '%s'", le.identity)

	if err := le.election.Campaign(ctx, le.identity); err != nil {
		return fmt.Errorf("failed to campaign for leadership: %w", err)
	}

	logger.Infof("Became leader: '%s'", le.identity)
	return nil
}

// IsLeader returns true if this instance is the current leader
func (le *LeaderElection) IsLeader() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := le.election.Leader(ctx)
	if err != nil {
		return false
	}

	return string(resp.Kvs[0].Value) == le.identity
}

// Resign gives up leadership voluntarily
func (le *LeaderElection) Resign(ctx context.Context) error {
	logger.Infof("Resigning from leadership: '%s'", le.identity)

	if err := le.election.Resign(ctx); err != nil {
		return fmt.Errorf("failed to resign: %w", err)
	}

	return nil
}

// Observe watches for leader changes (for standby replicas)
func (le *LeaderElection) Observe(ctx context.Context) <-chan string {
	leaderCh := make(chan string, 1)

	go func() {
		defer close(leaderCh)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Watch for leader changes
				respCh := le.election.Observe(ctx)
				for resp := range respCh {
					if len(resp.Kvs) > 0 {
						leader := string(resp.Kvs[0].Value)
						logger.Infof("Leader changed to: '%s'", leader)
						select {
						case leaderCh <- leader:
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}
	}()

	return leaderCh
}

// Close closes the election session
func (le *LeaderElection) Close() error {
	return le.session.Close()
}
