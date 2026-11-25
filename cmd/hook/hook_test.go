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

package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/google/go-github/v61/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testRunnerID int64 = 42

var testBackoff = backoffConfig{
	initialInterval: time.Millisecond,
	maxInterval:     time.Millisecond,
	maxElapsedTime:  10 * time.Millisecond,
	multiplier:      1,
}

type getResponse struct {
	runners *github.Runners
	err     error
}

type fakeRunnerAPI struct {
	onGet     func(call int)
	block     chan struct{}
	removeErr error
	responses []getResponse
	removed   []int64
	mu        sync.Mutex
	calls     int
}

func (f *fakeRunnerAPI) GetRunnerByName(string) (*github.Runners, error) {
	if f.block != nil {
		<-f.block
	}
	f.mu.Lock()
	call := f.calls
	f.calls++
	resp := getResponse{runners: &github.Runners{}}
	if len(f.responses) > 0 {
		resp = f.responses[min(call, len(f.responses)-1)]
	}
	onGet := f.onGet
	f.mu.Unlock()
	if onGet != nil {
		onGet(call)
	}
	return resp.runners, resp.err
}

func (f *fakeRunnerAPI) RemoveRunner(id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed = append(f.removed, id)
	return f.removeErr
}

func (f *fakeRunnerAPI) removedIDs() []int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int64(nil), f.removed...)
}

func (f *fakeRunnerAPI) getCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func blockingAPI(t *testing.T) *fakeRunnerAPI {
	block := make(chan struct{})
	t.Cleanup(func() { close(block) })
	return &fakeRunnerAPI{block: block}
}

func runnerResponse(status string, busy bool) getResponse {
	return getResponse{runners: &github.Runners{
		TotalCount: 1,
		Runners: []*github.Runner{{
			ID:     github.Int64(testRunnerID),
			Status: github.String(status),
			Busy:   github.Bool(busy),
		}},
	}}
}

func notFound() getResponse {
	return getResponse{runners: &github.Runners{}}
}

func newTestWatcher(api runnerAPI) *statusWatcher {
	return &statusWatcher{
		api:            api,
		name:           "runner-1",
		backoff:        testBackoff,
		offlineLimit:   2,
		finishedChecks: 1,
	}
}

func TestStatusWatcherCheck_OnlineKeepsWaiting(t *testing.T) {
	w := newTestWatcher(&fakeRunnerAPI{responses: []getResponse{runnerResponse("online", false)}})

	_, done := w.check(context.Background())

	assert.False(t, done)
}

func TestStatusWatcherCheck_StopsOnCheckAfterOfflineLimit(t *testing.T) {
	w := newTestWatcher(&fakeRunnerAPI{responses: []getResponse{runnerResponse("offline", false)}})

	for range w.offlineLimit {
		_, done := w.check(context.Background())
		require.False(t, done)
	}
	exit, done := w.check(context.Background())

	assert.True(t, done)
	assert.Equal(t, exitOfflineLimit, exit)
}

func TestStatusWatcherCheck_FinishesAfterFinishedChecks(t *testing.T) {
	w := newTestWatcher(&fakeRunnerAPI{responses: []getResponse{notFound()}})

	_, done := w.check(context.Background())
	require.False(t, done)
	exit, done := w.check(context.Background())

	assert.True(t, done)
	assert.Equal(t, exitFinished, exit)
}

func TestStatusWatcherCheck_LookupErrorKeepsWaiting(t *testing.T) {
	w := newTestWatcher(&fakeRunnerAPI{responses: []getResponse{{err: errors.New("bad credentials")}}})

	_, done := w.check(context.Background())

	assert.False(t, done)
}

func TestStatusWatcherCheck_CancelAbortsRetryBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := newTestWatcher(&fakeRunnerAPI{
		responses: []getResponse{{err: errors.New("connection refused")}},
		onGet:     func(int) { cancel() },
	})
	w.backoff = backoffConfig{initialInterval: time.Minute, maxInterval: time.Minute, maxElapsedTime: time.Hour, multiplier: 1}

	start := time.Now()
	exit, done := w.check(ctx)

	assert.True(t, done)
	assert.Equal(t, exitSignal, exit)
	assert.Less(t, time.Since(start), time.Second)
}

func TestStatusWatcherCheck_LookupDeadlineKeepsWaiting(t *testing.T) {
	w := newTestWatcher(&fakeRunnerAPI{responses: []getResponse{{err: context.DeadlineExceeded}}})

	_, done := w.check(context.Background())

	assert.False(t, done)
}

func TestStatusWatcherWait_ReturnsOfflineLimit(t *testing.T) {
	w := newTestWatcher(&fakeRunnerAPI{responses: []getResponse{runnerResponse("offline", false)}})

	assert.Equal(t, exitOfflineLimit, w.wait(context.Background(), time.Millisecond))
}

func TestStatusWatcherWait_ReturnsFinished(t *testing.T) {
	w := newTestWatcher(&fakeRunnerAPI{responses: []getResponse{runnerResponse("online", false), notFound()}})

	assert.Equal(t, exitFinished, w.wait(context.Background(), time.Millisecond))
}

func TestStatusWatcherWait_ReturnsSignalWhenCancelledDuringCheck(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := newTestWatcher(&fakeRunnerAPI{
		responses: []getResponse{runnerResponse("online", false)},
		onGet:     func(int) { cancel() },
	})

	assert.Equal(t, exitSignal, w.wait(ctx, time.Millisecond))
}

func TestStatusWatcherWait_SkipsLookupWhenAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	api := &fakeRunnerAPI{responses: []getResponse{runnerResponse("online", false)}}

	assert.Equal(t, exitSignal, newTestWatcher(api).wait(ctx, time.Millisecond))
	assert.Zero(t, api.getCalls())
}

const runnerURLPathWithFive = "/orgs/acme/actions/runners/1505"

func githubStatusError(code int, message string) *github.ErrorResponse {
	return &github.ErrorResponse{
		Message: message,
		Response: &http.Response{
			StatusCode: code,
			Request: &http.Request{
				Method: http.MethodDelete,
				URL:    &url.URL{Scheme: "https", Host: "api.github.com", Path: runnerURLPathWithFive},
			},
		},
	}
}

func TestWithRetry_ClassifiesGitHubErrors(t *testing.T) {
	forbidden := githubStatusError(http.StatusForbidden, "")
	tests := []struct {
		err       error
		name      string
		wantCalls int
		wantErr   bool
	}{
		{name: "403 permission", err: githubStatusError(http.StatusForbidden, "Resource not accessible by integration"),
			wantCalls: 1, wantErr: true},
		{name: "422 validation", err: githubStatusError(http.StatusUnprocessableEntity, "Validation Failed"), wantCalls: 1, wantErr: true},
		{name: "404", err: githubStatusError(http.StatusNotFound, "Not Found"), wantCalls: 1, wantErr: true},
		{name: "primary rate limit", err: &github.RateLimitError{Response: forbidden.Response}, wantCalls: 2},
		{name: "secondary rate limit", err: &github.AbuseRateLimitError{Response: forbidden.Response}, wantCalls: 2},
		{name: "429", err: githubStatusError(http.StatusTooManyRequests, "Too Many Requests"), wantCalls: 2},
		{name: "500", err: githubStatusError(http.StatusInternalServerError, "Server Error"), wantCalls: 2},
		{name: "wrapped 502", err: fmt.Errorf("failed to remove runner: %w", githubStatusError(http.StatusBadGateway, "")), wantCalls: 2},
		{name: "untyped connection error", err: errors.New("connection refused"), wantCalls: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			err := withRetry(context.Background(), testBackoff, func() error {
				calls++
				if calls == 1 {
					return tt.err
				}
				return nil
			})

			assert.Equal(t, tt.wantCalls, calls)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestDeregisterRunner_RemovesIdleRunner(t *testing.T) {
	api := &fakeRunnerAPI{responses: []getResponse{runnerResponse("offline", false)}}

	require.NoError(t, deregisterRunner(api, "runner-1", testBackoff, time.Second))

	assert.Equal(t, []int64{testRunnerID}, api.removedIDs())
}

func TestDeregisterRunner_SkipsBusyRunner(t *testing.T) {
	api := &fakeRunnerAPI{responses: []getResponse{runnerResponse("online", true)}}

	err := deregisterRunner(api, "runner-1", testBackoff, time.Second)

	assert.ErrorIs(t, err, errRunnerBusy)
	assert.Empty(t, api.removedIDs())
}

func TestDeregisterRunner_NotFoundIsNoop(t *testing.T) {
	api := &fakeRunnerAPI{responses: []getResponse{notFound()}}

	require.NoError(t, deregisterRunner(api, "runner-1", testBackoff, time.Second))

	assert.Empty(t, api.removedIDs())
}

func TestDeregisterRunner_TreatsRemoveNotFoundAsGone(t *testing.T) {
	api := &fakeRunnerAPI{
		responses: []getResponse{runnerResponse("offline", false)},
		removeErr: fmt.Errorf("failed to remove runner %d: %w", testRunnerID, githubStatusError(http.StatusNotFound, "Not Found")),
	}

	require.NoError(t, deregisterRunner(api, "runner-1", testBackoff, time.Second))

	assert.Equal(t, []int64{testRunnerID}, api.removedIDs())
}

func TestDeregisterRunner_ReturnsLookupError(t *testing.T) {
	lookupErr := errors.New("bad credentials")
	api := &fakeRunnerAPI{responses: []getResponse{{err: lookupErr}}}

	err := deregisterRunner(api, "runner-1", testBackoff, time.Second)

	assert.ErrorIs(t, err, lookupErr)
	assert.Empty(t, api.removedIDs())
}

func TestDeregisterRunner_ReturnsRemoveError(t *testing.T) {
	removeErr := errors.New("bad credentials")
	api := &fakeRunnerAPI{responses: []getResponse{runnerResponse("offline", false)}, removeErr: removeErr}

	assert.ErrorIs(t, deregisterRunner(api, "runner-1", testBackoff, time.Second), removeErr)
}

func TestDeregisterRunner_ReturnsPermissionErrorWithoutRetry(t *testing.T) {
	api := &fakeRunnerAPI{
		responses: []getResponse{runnerResponse("offline", false)},
		removeErr: fmt.Errorf("failed to remove runner %d: %w", testRunnerID,
			githubStatusError(http.StatusForbidden, "Resource not accessible by integration")),
	}

	err := deregisterRunner(api, "runner-1", testBackoff, time.Second)

	var ghErr *github.ErrorResponse
	require.ErrorAs(t, err, &ghErr)
	assert.Equal(t, http.StatusForbidden, ghErr.Response.StatusCode)
	assert.Equal(t, []int64{testRunnerID}, api.removedIDs())
}

func TestDeregisterRunner_TreatsRemoveUnprocessableAsBusy(t *testing.T) {
	api := &fakeRunnerAPI{
		responses: []getResponse{runnerResponse("offline", false)},
		removeErr: fmt.Errorf("failed to remove runner %d: %w", testRunnerID,
			githubStatusError(http.StatusUnprocessableEntity, `Bad request - Runner "runner-1" is still running a job`)),
	}

	err := deregisterRunner(api, "runner-1", testBackoff, time.Second)

	assert.ErrorIs(t, err, errRunnerBusy)
	assert.Equal(t, []int64{testRunnerID}, api.removedIDs())
}

func TestDeregisterRunner_TimesOutOnHungLookup(t *testing.T) {
	api := blockingAPI(t)

	start := time.Now()
	err := deregisterRunner(api, "runner-1", testBackoff, 20*time.Millisecond)

	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), time.Second)
}

func newTestDeps(api *fakeRunnerAPI, send func(string)) hookDeps {
	return hookDeps{
		watcher:       newTestWatcher(api),
		send:          send,
		initialWait:   time.Millisecond,
		checkInterval: time.Millisecond,
		deregTimeout:  time.Second,
	}
}

func TestRun_CancelledBeforeStartSkipsSendAndDeregisters(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	api := &fakeRunnerAPI{responses: []getResponse{runnerResponse("offline", false)}}
	sent := false

	exit, err := run(ctx, newTestDeps(api, func(string) { sent = true }), "request")

	require.NoError(t, err)
	assert.Equal(t, exitSignal, exit)
	assert.False(t, sent)
	assert.Equal(t, []int64{testRunnerID}, api.removedIDs())
}

func TestRun_SignalDuringInitialWaitDeregisters(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api := &fakeRunnerAPI{responses: []getResponse{runnerResponse("offline", false)}}
	deps := newTestDeps(api, func(string) { cancel() })
	deps.initialWait = time.Hour

	exit, err := run(ctx, deps, "request")

	require.NoError(t, err)
	assert.Equal(t, exitSignal, exit)
	assert.Equal(t, []int64{testRunnerID}, api.removedIDs())
}

func TestRun_OfflineLimitDeregisters(t *testing.T) {
	api := &fakeRunnerAPI{responses: []getResponse{runnerResponse("offline", false)}}

	exit, err := run(context.Background(), newTestDeps(api, func(string) {}), "request")

	require.NoError(t, err)
	assert.Equal(t, exitOfflineLimit, exit)
	assert.Equal(t, []int64{testRunnerID}, api.removedIDs())
}

func TestRun_FinishedDoesNotRemove(t *testing.T) {
	api := &fakeRunnerAPI{responses: []getResponse{notFound()}}

	exit, err := run(context.Background(), newTestDeps(api, func(string) {}), "request")

	require.NoError(t, err)
	assert.Equal(t, exitFinished, exit)
	assert.Empty(t, api.removedIDs())
}

func TestRun_BusyRunnerOnSignalIsKept(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	api := &fakeRunnerAPI{responses: []getResponse{runnerResponse("online", true)}}

	exit, err := run(ctx, newTestDeps(api, func(string) {}), "request")

	assert.Equal(t, exitSignal, exit)
	assert.ErrorIs(t, err, errRunnerBusy)
	assert.Empty(t, api.removedIDs())
}

func TestExitCode(t *testing.T) {
	failed := errors.New("403 Forbidden")
	tests := []struct {
		err  error
		name string
		exit loopExit
		want int
	}{
		{name: "finished", exit: exitFinished, want: 0},
		{name: "offline limit removed", exit: exitOfflineLimit, want: 0},
		{name: "offline limit delete failed", exit: exitOfflineLimit, err: failed, want: 0},
		{name: "signal removed", exit: exitSignal, want: 0},
		{name: "signal runner busy", exit: exitSignal, err: errRunnerBusy, want: 0},
		{name: "signal delete timed out", exit: exitSignal, err: context.DeadlineExceeded, want: 1},
		{name: "signal delete failed", exit: exitSignal, err: failed, want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, exitCode(tt.exit, tt.err))
		})
	}
}
