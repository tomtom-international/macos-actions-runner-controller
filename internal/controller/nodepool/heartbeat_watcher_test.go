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

package nodepool

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// newTestWatcher builds a watcher whose n-th Watch call returns streams[n].
// Once the streams run out, Watch returns a channel that never yields.
func newTestWatcher(streams ...func() clientv3.WatchChan) (*heartbeatWatcher, *int32) {
	var calls int32
	ctx, cancel := context.WithCancel(context.Background())
	w := &heartbeatWatcher{
		ctx:         ctx,
		ctxCancel:   cancel,
		nodeManager: NewManager(nil, "nodes", 0),
		nodeID:      "node-1",
		watchKey:    "/nodes/node-1",
		pause:       true,
	}
	w.watch = func(context.Context, string) clientv3.WatchChan {
		n := atomic.AddInt32(&calls, 1)
		if int(n) <= len(streams) {
			return streams[n-1]()
		}
		return make(chan clientv3.WatchResponse)
	}
	return w, &calls
}

func closedStream() clientv3.WatchChan {
	ch := make(chan clientv3.WatchResponse)
	close(ch)
	return ch
}

func failedStream() clientv3.WatchChan {
	ch := make(chan clientv3.WatchResponse, 1)
	ch <- clientv3.WatchResponse{Canceled: true}
	return ch
}

func deleteStream() clientv3.WatchChan {
	ch := make(chan clientv3.WatchResponse, 1)
	ch <- clientv3.WatchResponse{Events: []*clientv3.Event{{
		Type: mvccpb.DELETE,
		Kv:   &mvccpb.KeyValue{Key: []byte("/nodes/node-1")},
	}}}
	return ch
}

func runWithTimeout(t *testing.T, w *heartbeatWatcher, timeout time.Duration) bool {
	t.Helper()
	done := make(chan struct{})
	go func() {
		w.run()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		w.ctxCancel()
		<-done
		return false
	}
}

func TestHeartbeatWatcher_RecreatesWatchAfterChannelCloses(t *testing.T) {
	w, calls := newTestWatcher(closedStream, deleteStream)

	finished := runWithTimeout(t, w, 5*time.Second)

	assert.True(t, finished, "watcher should re-create the watch and see the delete event")
	assert.Equal(t, int32(2), atomic.LoadInt32(calls), "Watch should be called again after the channel closes")
}

func TestHeartbeatWatcher_RecreatesWatchAfterWatchError(t *testing.T) {
	w, calls := newTestWatcher(failedStream, deleteStream)

	finished := runWithTimeout(t, w, 5*time.Second)

	assert.True(t, finished, "watcher should re-create the watch after a canceled response")
	assert.Equal(t, int32(2), atomic.LoadInt32(calls), "Watch should be called again after a watch error")
}

func TestHeartbeatWatcher_StopsWhenContextCanceledDuringReconnect(t *testing.T) {
	w, calls := newTestWatcher(closedStream)
	w.ctxCancel()

	finished := runWithTimeout(t, w, 2*time.Second)

	assert.True(t, finished, "watcher should return when its context is canceled")
	assert.LessOrEqual(t, atomic.LoadInt32(calls), int32(1))
}
