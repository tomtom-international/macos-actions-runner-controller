//go:build integration

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

// To run integration tests: go test -tags=integration ./pkg/clients/etcd/...
// These tests require a running etcd instance. Set ETCD_ENDPOINTS env var to specify endpoints.
// Example: ETCD_ENDPOINTS=localhost:2379 go test -tags=integration -v -run TestIntegration_Election ./pkg/clients/etcd/...

package etcd

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_Election_NewLeaderElection tests creating a leader election
func TestIntegration_Election_NewLeaderElection(t *testing.T) {
	endpoints := getEtcdEndpoints(t)

	clientCfg := ClientConfig{
		Endpoints:      endpoints,
		RequestTimeout: 5,
	}

	client, err := NewEtcdClient(clientCfg)
	require.NoError(t, err, "should create client successfully")
	defer client.Close()

	electionCfg := ElectionConfig{
		Identity:       "test-node-1",
		ElectionKey:    "/test/election/leader",
		TTL:            5,
		SessionTimeout: 10,
	}

	election, err := client.NewLeaderElection(electionCfg)
	require.NoError(t, err, "should create leader election successfully")
	require.NotNil(t, election, "election should not be nil")
	defer election.Close()

	t.Logf("Successfully created leader election for identity '%s'", electionCfg.Identity)
}

// TestIntegration_Election_CampaignAndBecomeLeader tests campaigning and becoming leader
func TestIntegration_Election_CampaignAndBecomeLeader(t *testing.T) {
	endpoints := getEtcdEndpoints(t)

	clientCfg := ClientConfig{
		Endpoints:      endpoints,
		RequestTimeout: 5,
	}

	client, err := NewEtcdClient(clientCfg)
	require.NoError(t, err, "should create client successfully")
	defer client.Close()

	electionCfg := ElectionConfig{
		Identity:       "test-node-campaign",
		ElectionKey:    "/test/election/campaign",
		TTL:            5,
		SessionTimeout: 10,
	}

	election, err := client.NewLeaderElection(electionCfg)
	require.NoError(t, err, "should create leader election successfully")
	defer election.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	campaignDone := make(chan error, 1)
	go func() {
		campaignDone <- election.Campaign(ctx)
	}()

	select {
	case err := <-campaignDone:
		require.NoError(t, err, "Campaign should succeed")
	case <-ctx.Done():
		t.Fatal("Campaign timed out")
	}

	assert.True(t, election.IsLeader(), "should be leader after successful campaign")

	t.Log("Successfully campaigned and became leader")
}

// TestIntegration_Election_IsLeader tests the IsLeader check
func TestIntegration_Election_IsLeader(t *testing.T) {
	endpoints := getEtcdEndpoints(t)

	clientCfg := ClientConfig{
		Endpoints:      endpoints,
		RequestTimeout: 5,
	}

	client, err := NewEtcdClient(clientCfg)
	require.NoError(t, err, "should create client successfully")
	defer client.Close()

	electionCfg := ElectionConfig{
		Identity:       "test-node-isleader",
		ElectionKey:    "/test/election/isleader",
		TTL:            5,
		SessionTimeout: 10,
	}

	election, err := client.NewLeaderElection(electionCfg)
	require.NoError(t, err, "should create leader election successfully")
	defer election.Close()

	initialLeader := election.IsLeader()
	assert.False(t, initialLeader, "should not be leader before campaign ")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = election.Campaign(ctx)
	require.NoError(t, err, "Campaign should succeed")

	assert.True(t, election.IsLeader(), "should be leader after campaign")

	t.Log("IsLeader correctly identifies leadership status")
}

// TestIntegration_Election_Resign tests resigning from leadership
func TestIntegration_Election_Resign(t *testing.T) {
	endpoints := getEtcdEndpoints(t)

	clientCfg := ClientConfig{
		Endpoints:      endpoints,
		RequestTimeout: 5,
	}

	client, err := NewEtcdClient(clientCfg)
	require.NoError(t, err, "should create client successfully")
	defer client.Close()

	electionCfg := ElectionConfig{
		Identity:       "test-node-resign",
		ElectionKey:    "/test/election/resign",
		TTL:            5,
		SessionTimeout: 10,
	}

	election, err := client.NewLeaderElection(electionCfg)
	require.NoError(t, err, "should create leader election successfully")
	defer election.Close()

	ctx := context.Background()
	campaignCtx, campaignCancel := context.WithTimeout(ctx, 10*time.Second)
	defer campaignCancel()

	err = election.Campaign(campaignCtx)
	require.NoError(t, err, "Campaign should succeed")
	assert.True(t, election.IsLeader(), "should be leader")

	resignCtx, resignCancel := context.WithTimeout(ctx, 5*time.Second)
	defer resignCancel()

	err = election.Resign(resignCtx)
	require.NoError(t, err, "Resign should succeed")

	time.Sleep(100 * time.Millisecond)
	assert.False(t, election.IsLeader(), "should not be leader after resign")

	t.Log("Successfully resigned from leadership")
}

// TestIntegration_Election_MultipleCompetitors tests multiple nodes competing for leadership
func TestIntegration_Election_MultipleCompetitors(t *testing.T) {
	endpoints := getEtcdEndpoints(t)

	electionKey := "/test/election/multi-competitor"
	numCompetitors := 3

	var elections []*LeaderElection
	var clients []*Client
	defer func() {
		for _, e := range elections {
			e.Close()
		}
		for _, c := range clients {
			c.Close()
		}
	}()

	for i := 0; i < numCompetitors; i++ {
		clientCfg := ClientConfig{
			Endpoints:      endpoints,
			RequestTimeout: 5,
		}

		client, err := NewEtcdClient(clientCfg)
		require.NoError(t, err, "should create client %d successfully", i)
		clients = append(clients, client)

		electionCfg := ElectionConfig{
			Identity:       "competitor-" + string(rune('A'+i)),
			ElectionKey:    electionKey,
			TTL:            5,
			SessionTimeout: 10,
		}

		election, err := client.NewLeaderElection(electionCfg)
		require.NoError(t, err, "should create election %d successfully", i)
		elections = append(elections, election)
	}

	var wg sync.WaitGroup
	leaderCh := make(chan int, numCompetitors)

	for i := 0; i < numCompetitors; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			err := elections[idx].Campaign(ctx)
			if err == nil {
				leaderCh <- idx
			}
		}(i)
	}

	var leaderIdx int
	select {
	case leaderIdx = <-leaderCh:
		t.Logf("Competitor %d became leader", leaderIdx)
	case <-time.After(20 * time.Second):
		t.Fatal("No competitor became leader within timeout")
	}

	time.Sleep(500 * time.Millisecond)
	leaderCount := 0
	for i, election := range elections {
		if election.IsLeader() {
			leaderCount++
			t.Logf("Competitor %d is leader", i)
		}
	}

	assert.Equal(t, 1, leaderCount, "exactly one competitor should be leader")

	t.Logf("Successfully tested %d competitors, leader: %d", numCompetitors, leaderIdx)
}

// TestIntegration_Election_Observe tests watching for leader changes
func TestIntegration_Election_Observe(t *testing.T) {
	endpoints := getEtcdEndpoints(t)

	electionKey := "/test/election/observe"

	clientCfg1 := ClientConfig{
		Endpoints:      endpoints,
		RequestTimeout: 5,
	}
	client1, err := NewEtcdClient(clientCfg1)
	require.NoError(t, err, "should create client1 successfully")
	defer client1.Close()

	electionCfg1 := ElectionConfig{
		Identity:       "observer-leader",
		ElectionKey:    electionKey,
		TTL:            5,
		SessionTimeout: 10,
	}
	election1, err := client1.NewLeaderElection(electionCfg1)
	require.NoError(t, err, "should create election1 successfully")
	defer election1.Close()

	clientCfg2 := ClientConfig{
		Endpoints:      endpoints,
		RequestTimeout: 5,
	}
	client2, err := NewEtcdClient(clientCfg2)
	require.NoError(t, err, "should create client2 successfully")
	defer client2.Close()

	electionCfg2 := ElectionConfig{
		Identity:       "observer-follower",
		ElectionKey:    electionKey,
		TTL:            5,
		SessionTimeout: 10,
	}
	election2, err := client2.NewLeaderElection(electionCfg2)
	require.NoError(t, err, "should create election2 successfully")
	defer election2.Close()

	observeCtx, observeCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer observeCancel()

	leaderCh := election2.Observe(observeCtx)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = election1.Campaign(ctx)
	}()

	select {
	case leader := <-leaderCh:
		assert.Equal(t, "observer-leader", leader, "should observe the correct leader")
		t.Logf("Observed leader: %s", leader)
	case <-time.After(10 * time.Second):
		t.Fatal("Did not observe leader within timeout")
	}

	t.Log("Successfully observed leader changes")
}

// TestIntegration_Election_LeaderResignAndReelection tests leader resigning and reelection
func TestIntegration_Election_LeaderResignAndReelection(t *testing.T) {
	endpoints := getEtcdEndpoints(t)

	electionKey := "/test/election/resign-reelect"

	var elections []*LeaderElection
	var clients []*Client
	identities := []string{"reelect-node-1", "reelect-node-2"}

	for _, identity := range identities {
		clientCfg := ClientConfig{
			Endpoints:      endpoints,
			RequestTimeout: 5,
		}
		client, err := NewEtcdClient(clientCfg)
		require.NoError(t, err, "should create client successfully")
		clients = append(clients, client)

		electionCfg := ElectionConfig{
			Identity:       identity,
			ElectionKey:    electionKey,
			TTL:            5,
			SessionTimeout: 10,
		}
		election, err := client.NewLeaderElection(electionCfg)
		require.NoError(t, err, "should create election successfully")
		elections = append(elections, election)
	}

	defer func() {
		for _, e := range elections {
			e.Close()
		}
		for _, c := range clients {
			c.Close()
		}
	}()

	ctx := context.Background()
	campaignCtx1, cancel1 := context.WithTimeout(ctx, 10*time.Second)
	defer cancel1()

	err := elections[0].Campaign(campaignCtx1)
	require.NoError(t, err, "first campaign should succeed")
	assert.True(t, elections[0].IsLeader(), "first node should be leader")

	campaign2Done := make(chan error, 1)
	go func() {
		campaignCtx2, cancel2 := context.WithTimeout(ctx, 20*time.Second)
		defer cancel2()
		campaign2Done <- elections[1].Campaign(campaignCtx2)
	}()

	time.Sleep(500 * time.Millisecond)

	resignCtx, resignCancel := context.WithTimeout(ctx, 5*time.Second)
	defer resignCancel()

	err = elections[0].Resign(resignCtx)
	require.NoError(t, err, "resign should succeed")

	select {
	case err := <-campaign2Done:
		require.NoError(t, err, "second campaign should succeed after first resigns")
	case <-time.After(10 * time.Second):
		t.Fatal("Second node did not become leader after first resigned")
	}

	time.Sleep(500 * time.Millisecond)
	assert.False(t, elections[0].IsLeader(), "first node should no longer be leader")
	assert.True(t, elections[1].IsLeader(), "second node should be leader")

	t.Log("Successfully tested leader resignation and reelection")
}

// TestIntegration_Election_Close tests closing an election
func TestIntegration_Election_Close(t *testing.T) {
	endpoints := getEtcdEndpoints(t)

	clientCfg := ClientConfig{
		Endpoints:      endpoints,
		RequestTimeout: 5,
	}

	client, err := NewEtcdClient(clientCfg)
	require.NoError(t, err, "should create client successfully")
	defer client.Close()

	electionCfg := ElectionConfig{
		Identity:       "test-node-close",
		ElectionKey:    "/test/election/close",
		TTL:            5,
		SessionTimeout: 10,
	}

	election, err := client.NewLeaderElection(electionCfg)
	require.NoError(t, err, "should create leader election successfully")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = election.Campaign(ctx)
	require.NoError(t, err, "Campaign should succeed")
	assert.True(t, election.IsLeader(), "should be leader")

	err = election.Close()
	require.NoError(t, err, "Close should not return error")

	time.Sleep(100 * time.Millisecond)

	t.Log("Successfully closed election")
}

// TestIntegration_Election_SessionExpiry tests behavior when session expires
func TestIntegration_Election_SessionExpiry(t *testing.T) {
	endpoints := getEtcdEndpoints(t)

	clientCfg := ClientConfig{
		Endpoints:      endpoints,
		RequestTimeout: 5,
	}

	client, err := NewEtcdClient(clientCfg)
	require.NoError(t, err, "should create client successfully")
	defer client.Close()

	electionCfg := ElectionConfig{
		Identity:       "test-node-expiry",
		ElectionKey:    "/test/election/expiry",
		TTL:            2,
		SessionTimeout: 10,
	}

	election, err := client.NewLeaderElection(electionCfg)
	require.NoError(t, err, "should create leader election successfully")
	defer election.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = election.Campaign(ctx)
	require.NoError(t, err, "Campaign should succeed")
	assert.True(t, election.IsLeader(), "should be leader initially")

	client.Close()

	// Wait for session to expire
	time.Sleep(5 * time.Second)

	isLeader := election.IsLeader()
	assert.False(t, isLeader, "should not be leader after session expires")

	t.Log("Successfully tested session expiry behavior")
}
