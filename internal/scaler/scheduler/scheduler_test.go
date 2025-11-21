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

package scheduler

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockActionHandler is a mock implementation of ActionHandler for testing
type MockActionHandler struct {
	executions map[string]int
	mock.Mock
	mu sync.Mutex
}

func NewMockActionHandler() *MockActionHandler {
	return &MockActionHandler{
		executions: make(map[string]int),
	}
}

func (m *MockActionHandler) Handle(ctx context.Context, target ScalingTarget) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	args := m.Called(ctx, target)

	key := target.ScalingGroupName + "-" + string(rune(target.DesiredCapacity))
	m.executions[key]++

	return args.Error(0)
}

func (m *MockActionHandler) GetExecutionCount(scalingGroupName string, desiredCapacity int) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := scalingGroupName + "-" + string(rune(desiredCapacity))
	return m.executions[key]
}

// TestNewScheduler tests the creation of a new scheduler
func TestNewScheduler(t *testing.T) {
	scheduler := NewScheduler()

	assert.NotNil(t, scheduler)
	assert.NotNil(t, scheduler.cron)
	assert.NotNil(t, scheduler.schedules)
	assert.Empty(t, scheduler.schedules)
}

// TestAddSchedule tests adding schedules to the scheduler
func TestAddSchedule(t *testing.T) {
	scheduler := NewScheduler()
	mockHandler := NewMockActionHandler()

	target := ScalingTarget{
		ScalingGroupName: "test-group",
		DesiredCapacity:  5,
		DryRun:           true,
	}

	mockHandler.On("Handle", mock.Anything, target).Return(nil)

	err := scheduler.AddSchedule("* * * * * *", "test-schedule", target, mockHandler.Handle)
	require.NoError(t, err)

	assert.Len(t, scheduler.schedules, 1)
	assert.Contains(t, scheduler.schedules, "test-schedule")

	err = scheduler.AddSchedule("invalid-cron", "invalid-schedule", target, mockHandler.Handle)
	assert.Error(t, err)
}

// TestGetSchedules tests retrieving all schedules
func TestGetSchedules(t *testing.T) {
	scheduler := NewScheduler()
	mockHandler := NewMockActionHandler()

	target := []ScalingTarget{
		{ScalingGroupName: "group-1", DesiredCapacity: 5},
		{ScalingGroupName: "group-2", DesiredCapacity: 3},
	}

	for i, action := range target {
		mockHandler.On("Handle", mock.Anything, action).Return(nil)
		name := fmt.Sprintf("schedule-%d", i)
		err := scheduler.AddSchedule("* * * * * *", name, action, mockHandler.Handle)
		require.NoError(t, err)
	}

	// Get schedules
	schedules := scheduler.GetSchedules()

	assert.Len(t, schedules, 2)
	assert.ElementsMatch(t, []string{"schedule-0", "schedule-1"}, []string{
		schedules[0].Name,
		schedules[1].Name,
	})
	assert.Equal(t, "group-1", schedules[0].Target.ScalingGroupName)
	assert.Equal(t, "group-2", schedules[1].Target.ScalingGroupName)
	assert.Equal(t, 5, schedules[0].Target.DesiredCapacity)
	assert.Equal(t, 3, schedules[1].Target.DesiredCapacity)
}

// TestSchedulerExecutesJobs tests that the scheduler actually executes jobs
func TestSchedulerExecutesJobs(t *testing.T) {
	scheduler := NewScheduler()
	mockHandler := NewMockActionHandler()

	target := ScalingTarget{
		ScalingGroupName: "test-group",
		DesiredCapacity:  5,
	}

	mockHandler.On("Handle", mock.Anything, target).Return(nil)

	err := scheduler.AddSchedule("* * * * * *", "test-schedule", target, mockHandler.Handle)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2100*time.Millisecond)
	defer cancel()

	scheduler.Start()

	<-ctx.Done()
	scheduler.Stop()

	// The test runs for 2.1 seconds with a job scheduled every second,
	// so it should execute at least twice
	execCount := mockHandler.GetExecutionCount("test-group", 5)
	assert.GreaterOrEqual(t, execCount, 2, "Expected at least 2 executions")
}

// TestSchedulerStop tests stopping the scheduler
func TestSchedulerStop(t *testing.T) {
	scheduler := NewScheduler()

	target := ScalingTarget{ScalingGroupName: "group-1", DesiredCapacity: 5}

	var executed = false
	handler := func(ctx context.Context, target ScalingTarget) error {
		executed = true
		return nil
	}
	err := scheduler.AddSchedule("* * * * * *", "test-schedule", target, handler)
	require.NoError(t, err)

	scheduler.Start()

	time.Sleep(1500 * time.Millisecond)
	assert.True(t, executed, "Scheduler should have executed the job")

	scheduler.Stop()

	executed = false

	time.Sleep(1500 * time.Millisecond)

	assert.False(t, executed, "No executions should occur after stopping")
}

// TestConcurrentAccess tests that the scheduler handles concurrent access correctly
func TestConcurrentAccess(t *testing.T) {
	scheduler := NewScheduler()
	mockHandler := NewMockActionHandler()

	baseTarget := ScalingTarget{
		ScalingGroupName: "test-group",
		DesiredCapacity:  5,
	}

	mockHandler.On("Handle", mock.Anything, mock.AnythingOfType("ScalingTarget")).Return(nil)

	err := scheduler.AddSchedule("* * * * * *", "test-schedule", baseTarget, mockHandler.Handle)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	scheduler.Start()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = scheduler.GetSchedules()
		}()
	}

	<-ctx.Done()
	scheduler.Stop()
	wg.Wait()
	// the race detector will catch if there is a race
}

// TestUpdateNextRunTime tests that next run times are updated correctly
func TestUpdateNextRunTime(t *testing.T) {
	scheduler := NewScheduler()
	mockHandler := NewMockActionHandler()

	target := ScalingTarget{
		ScalingGroupName: "test-group",
		DesiredCapacity:  5,
	}

	mockHandler.On("Handle", mock.Anything, target).Return(nil)

	err := scheduler.AddSchedule("* * * * * *", "test-schedule", target, mockHandler.Handle)
	require.NoError(t, err)

	scheduler.Start()
	time.Sleep(100 * time.Millisecond)

	schedules := scheduler.GetSchedules()
	require.Len(t, schedules, 1)

	assert.False(t, schedules[0].NextRunTime.IsZero())
	assert.True(t, schedules[0].NextRunTime.After(time.Now()))

	scheduler.Stop()
}

// TestCreateJob tests the job creation function
func TestCreateJob(t *testing.T) {
	scheduler := NewScheduler()
	mockHandler := NewMockActionHandler()

	target := ScalingTarget{
		ScalingGroupName: "test-group",
		DesiredCapacity:  5,
	}

	scheduler.handler = mockHandler.Handle

	mockHandler.On("Handle", mock.Anything, target).Return(nil)

	job := scheduler.createJob("test-job", target)

	job()

	mockHandler.AssertNumberOfCalls(t, "Handle", 1)
}
