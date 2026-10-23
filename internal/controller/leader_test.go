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
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var testLeaderTimings = leaderTimings{
	checkPeriod:   5 * time.Millisecond,
	retryDelay:    5 * time.Millisecond,
	resignTimeout: 100 * time.Millisecond,
}

// fakeElector wins every campaign unless campaignErr is set or blockUntil
// is non-nil, and stays leader while leader is true.
type fakeElector struct {
	campaignErr error
	blockUntil  <-chan struct{} // Campaign blocks until closed or ctx done
	leader      atomic.Bool
	resigned    atomic.Int32
	closed      atomic.Int32
}

func (f *fakeElector) Campaign(ctx context.Context) error {
	if f.blockUntil != nil {
		select {
		case <-f.blockUntil:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if f.campaignErr != nil {
		return f.campaignErr
	}
	f.leader.Store(true)
	return nil
}

func (f *fakeElector) IsLeader() bool { return f.leader.Load() }

func (f *fakeElector) Resign(context.Context) error {
	f.resigned.Add(1)
	f.leader.Store(false)
	return nil
}

func (f *fakeElector) Close() error {
	f.closed.Add(1)
	return nil
}

// electorSequence hands out the given electors in order, then keeps
// returning the last one.
func electorSequence(electors ...*fakeElector) (newElectorFunc, *atomic.Int32) {
	var calls atomic.Int32
	return func() (elector, error) {
		n := int(calls.Add(1))
		if n > len(electors) {
			n = len(electors)
		}
		return electors[n-1], nil
	}, &calls
}

// startRunAsLeader runs runAsLeader in the background and returns a stop
// function that cancels it and waits until it has fully returned.
func startRunAsLeader(t *testing.T, newElector newElectorFunc, work func(context.Context)) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		runAsLeader(ctx, testLeaderTimings, newElector, work)
	}()
	var once sync.Once
	stop := func() {
		once.Do(func() {
			cancel()
			waitFor(t, done, "runAsLeader did not return after cancel")
		})
	}
	t.Cleanup(stop)
	return stop
}

func TestRunAsLeader_DoesNotRunWorkWithoutLeadership(t *testing.T) {
	el := &fakeElector{blockUntil: make(chan struct{})} // another replica is leader
	newElector, _ := electorSequence(el)

	var ran atomic.Bool
	stop := startRunAsLeader(t, newElector, func(context.Context) { ran.Store(true) })
	time.Sleep(50 * time.Millisecond)
	stop()

	assert.False(t, ran.Load(), "work must not run while another replica holds leadership")
}

func TestRunAsLeader_StopsWorkWhenLeadershipIsLost(t *testing.T) {
	first := &fakeElector{}
	second := &fakeElector{blockUntil: make(chan struct{})} // never wins again
	newElector, _ := electorSequence(first, second)

	started := make(chan struct{})
	stopped := make(chan struct{})
	stop := startRunAsLeader(t, newElector, func(ctx context.Context) {
		close(started)
		<-ctx.Done()
		close(stopped)
	})

	waitFor(t, started, "work did not start after winning the election")
	first.leader.Store(false) // another replica took over
	waitFor(t, stopped, "work was not stopped after losing leadership")
	stop()

	assert.Equal(t, int32(1), first.resigned.Load())
	assert.Equal(t, int32(1), first.closed.Load())
}

func TestRunAsLeader_RecreatesElectorAfterCampaignError(t *testing.T) {
	broken := &fakeElector{campaignErr: errors.New("etcd session expired")}
	healthy := &fakeElector{}
	newElector, calls := electorSequence(broken, healthy)

	started := make(chan struct{})
	var once sync.Once
	stop := startRunAsLeader(t, newElector, func(ctx context.Context) {
		once.Do(func() { close(started) })
		<-ctx.Done()
	})

	waitFor(t, started, "work did not start after the elector was recreated")
	stop()

	assert.Equal(t, int32(1), broken.closed.Load(), "failed elector must be closed")
	assert.GreaterOrEqual(t, calls.Load(), int32(2))
}

func TestRunAsLeader_ResignsOnContextCancel(t *testing.T) {
	el := &fakeElector{}
	newElector, _ := electorSequence(el)

	started := make(chan struct{})
	stop := startRunAsLeader(t, newElector, func(ctx context.Context) {
		close(started)
		<-ctx.Done()
	})

	waitFor(t, started, "work did not start")
	stop()

	assert.Equal(t, int32(1), el.resigned.Load())
	assert.Equal(t, int32(1), el.closed.Load())
}

func waitFor(t *testing.T, ch <-chan struct{}, msg string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal(msg)
	}
}

func TestElectionIdentity_UniquePerReplicaOnSameHost(t *testing.T) {
	a, b := electionIdentity(), electionIdentity()

	assert.NotEqual(t, a, b, "two replicas on the same hostname must not share an election identity")
	assert.NotEmpty(t, a)
}
