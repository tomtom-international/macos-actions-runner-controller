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

package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
)

type watchFake struct {
	*FakeController
	nodes   chan chan types.WatcherNodesUpdate
	runners chan chan types.WatcherRunnersUpdate
}

func newWatchFake() *watchFake {
	w := &watchFake{
		FakeController: &FakeController{},
		nodes:          make(chan chan types.WatcherNodesUpdate, 1),
		runners:        make(chan chan types.WatcherRunnersUpdate, 1),
	}
	w.On("AddNodesWatcher", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		w.nodes <- args.Get(1).(chan types.WatcherNodesUpdate)
	})
	w.On("AddRunnersWatcher", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		w.runners <- args.Get(1).(chan types.WatcherRunnersUpdate)
	})
	w.On("RemoveNodesWatcherHandler", mock.Anything).Maybe()
	w.On("RemoveRunnersWatcherHandler", mock.Anything).Maybe()
	return w
}

func watchURL(srv *httptest.Server) string {
	return "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/watch"
}

func dialWatch(t *testing.T, srv *httptest.Server, origin string) (conn *websocket.Conn, status int, err error) {
	t.Helper()
	header := http.Header{}
	if origin != "" {
		header.Set("Origin", origin)
	}
	conn, resp, err := websocket.DefaultDialer.Dial(watchURL(srv), header)
	if resp != nil {
		status = resp.StatusCode
		resp.Body.Close()
	}
	if conn != nil {
		t.Cleanup(func() { conn.Close() })
	}
	return conn, status, err
}

func receiveUpdates(t *testing.T, f *watchFake, conn *websocket.Conn) {
	t.Helper()
	nodes := <-f.nodes
	runners := <-f.runners
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))

	nodes <- types.WatcherNodesUpdate{UpdatedNodes: []types.Node{testNode}}
	var gotNodes types.WatcherNodesUpdate
	require.NoError(t, conn.ReadJSON(&gotNodes))
	assert.Equal(t, []types.Node{testNode}, gotNodes.UpdatedNodes)

	runners <- types.WatcherRunnersUpdate{UpdatedRunners: []types.Runner{testRunner}}
	var gotRunners types.WatcherRunnersUpdate
	require.NoError(t, conn.ReadJSON(&gotRunners))
	assert.Equal(t, []types.Runner{testRunner}, gotRunners.UpdatedRunners)
}

func TestWatchUpgradesThroughNewServer(t *testing.T) {
	f := newWatchFake()
	srv := httptest.NewServer(NewServer(f, "*"))
	defer srv.Close()

	conn, status, err := dialWatch(t, srv, "")

	require.NoError(t, err)
	assert.Equal(t, http.StatusSwitchingProtocols, status)
	receiveUpdates(t, f, conn)
}

func TestWatchNonUpgradeGetFails(t *testing.T) {
	f := newWatchFake()

	w := serve(NewServer(f, "*"), http.MethodGet, "/ws/watch", "")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	f.AssertNotCalled(t, "AddNodesWatcher", mock.Anything, mock.Anything)
}

func TestWatchOriginCheck(t *testing.T) {
	cases := []struct {
		allowed string
		origin  string
		accept  bool
	}{
		{"*", "", true},
		{"*", "https://anything.example.net", true},
		{"", "https://anything.example.net", true},
		{"https://console.example.com", "", true},
		{"https://console.example.com", "https://console.example.com", true},
		{"https://console.example.com/", "https://console.example.com", true},
		{"https://console.example.com", "https://console.example.com/", true},
		{"https://a.example.com, https://b.example.com", "https://b.example.com", true},
		{"*.example.org", "https://team.example.org", true},
		{"https://console.example.com", "https://evil.example.net", false},
		{"https://console.example.com", "http://console.example.com", false},
		{"https://a.example.com,https://b.example.com", "https://c.example.com", false},
		{"*.example.org", "https://example.net", false},
	}
	for _, tc := range cases {
		t.Run(tc.allowed+" <- "+tc.origin, func(t *testing.T) {
			f := newWatchFake()
			srv := httptest.NewServer(NewServer(f, tc.allowed))
			defer srv.Close()

			conn, status, err := dialWatch(t, srv, tc.origin)

			if tc.accept {
				require.NoError(t, err)
				assert.Equal(t, http.StatusSwitchingProtocols, status)
				receiveUpdates(t, f, conn)
				return
			}
			require.ErrorIs(t, err, websocket.ErrBadHandshake)
			assert.Equal(t, http.StatusForbidden, status)
			f.AssertNotCalled(t, "AddNodesWatcher", mock.Anything, mock.Anything)
		})
	}
}

func TestWatchConnectionOutlivesWriteTimeout(t *testing.T) {
	f := newWatchFake()
	srv := httptest.NewUnstartedServer(NewServer(f, "*"))
	srv.Config.WriteTimeout = time.Second
	srv.Start()
	defer srv.Close()

	conn, _, err := dialWatch(t, srv, "")
	require.NoError(t, err)

	time.Sleep(3 * time.Second)

	receiveUpdates(t, f, conn)
}
