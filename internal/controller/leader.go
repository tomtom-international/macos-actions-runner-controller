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

package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/etcd"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
)

const schedulerElectionKey = "/election/controller/scheduler"

// leaderTimings groups the leadership loop intervals so tests can shorten them.
type leaderTimings struct {
	checkPeriod   time.Duration // how often leadership is re-checked
	retryDelay    time.Duration // wait before a new campaign after a failure
	resignTimeout time.Duration
}

var defaultLeaderTimings = leaderTimings{
	checkPeriod:   5 * time.Second,
	retryDelay:    2 * time.Second,
	resignTimeout: 5 * time.Second,
}

// elector is the subset of etcd.LeaderElection used by the controller.
type elector interface {
	Campaign(ctx context.Context) error
	IsLeader() bool
	Resign(ctx context.Context) error
	Close() error
}

// newElectorFunc builds a fresh elector. A failed campaign usually means the
// etcd session expired, so the elector is rebuilt instead of reused.
type newElectorFunc func() (elector, error)

func etcdElectorFactory(client *etcd.Client) newElectorFunc {
	identity := electionIdentity()
	return func() (elector, error) {
		return client.NewLeaderElection(etcd.ElectionConfig{
			Identity:    identity,
			ElectionKey: schedulerElectionKey,
		})
	}
}

// electionIdentity is the normalized hostname plus a random suffix, so two
// replicas that share a hostname (docker-compose, cloned VMs) never both see
// themselves as the leader.
func electionIdentity() string {
	host, err := utils.GetHostname()
	if err != nil {
		host = "controller"
	}
	return fmt.Sprintf("%s-%s", host, utils.NewShortUUID())
}

// runAsLeader runs work only while this replica holds the scheduler
// leadership. Every controller replica runs the scheduler loop otherwise,
// and two replicas can bind the same pending runner to different nodes in
// the same tick: the runner keeps only the last NodeID written, so the
// other binding is never released and that node loses the slot for good.
//
// work receives a context that is canceled when leadership is lost.
func runAsLeader(ctx context.Context, timings leaderTimings, newElector newElectorFunc, work func(ctx context.Context)) {
	for ctx.Err() == nil {
		el, err := newElector()
		if err != nil {
			logger.Errorf("Failed to create scheduler leader election: %v", err)
			sleepCtx(ctx, timings.retryDelay)
			continue
		}

		if err := el.Campaign(ctx); err != nil {
			if ctx.Err() == nil {
				logger.Errorf("Scheduler leader election campaign failed: %v", err)
			}
			_ = el.Close()
			sleepCtx(ctx, timings.retryDelay)
			continue
		}

		logger.Infof("Became scheduler leader, starting scheduler")
		leadWhileLeader(ctx, timings, el, work)
		_ = el.Close()
	}
}

// leadWhileLeader runs work until ctx is canceled or leadership is lost,
// then resigns so another replica can take over without waiting for the
// session TTL.
func leadWhileLeader(ctx context.Context, timings leaderTimings, el elector, work func(ctx context.Context)) {
	workCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		work(workCtx)
	}()

	ticker := time.NewTicker(timings.checkPeriod)
	defer ticker.Stop()

loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-done:
			break loop
		case <-ticker.C:
			if !el.IsLeader() {
				logger.Warnf("Lost scheduler leadership, stopping scheduler")
				break loop
			}
		}
	}

	cancel()
	<-done

	resignCtx, resignCancel := context.WithTimeout(context.Background(), timings.resignTimeout)
	defer resignCancel()
	if err := el.Resign(resignCtx); err != nil {
		logger.Errorf("Failed to resign scheduler leadership: %v", err)
	}
}

func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
