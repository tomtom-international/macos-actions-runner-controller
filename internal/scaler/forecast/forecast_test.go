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

package forecast

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tomtom-international/macos-actions-runner-controller/internal/scaler/scheduler"
)

func TestGetNextEvent(t *testing.T) {
	tz, _ := time.LoadLocation("UTC")
	service := NewService(tz)

	now := time.Now().In(tz)
	schedules := []scheduler.Schedule{
		{
			Name:        "schedule1",
			NextRunTime: now.Add(1 * time.Hour),
			Target: scheduler.ScalingTarget{
				ScalingGroupName: "group1",
				DesiredCapacity:  5,
			},
		},
		{
			Name:        "schedule2",
			NextRunTime: now.Add(2 * time.Hour),
			Target: scheduler.ScalingTarget{
				ScalingGroupName: "group2",
				DesiredCapacity:  10,
			},
		},
	}

	events, err := service.GetNextEvent(schedules)

	assert.NoError(t, err)
	assert.Len(t, events, 2)

	assert.Equal(t, "group1", events[0].ScalingGroup)
	assert.Equal(t, "schedule1", events[0].ScheduleName)
	assert.Equal(t, schedules[0].NextRunTime, events[0].TriggerTime)
	assert.Equal(t, 5, events[0].Action.DesiredCapacity)

	assert.Equal(t, "group2", events[1].ScalingGroup)
	assert.Equal(t, "schedule2", events[1].ScheduleName)
	assert.Equal(t, schedules[1].NextRunTime, events[1].TriggerTime)
	assert.Equal(t, 10, events[1].Action.DesiredCapacity)
}

func TestGetUpcomingEvents(t *testing.T) {
	tz, _ := time.LoadLocation("UTC")
	service := NewService(tz)

	schedules := []scheduler.Schedule{
		{
			Name:     "hourly-schedule",
			CronExpr: "0 0 * * * *",
			Target: scheduler.ScalingTarget{
				ScalingGroupName: "group1",
				DesiredCapacity:  5,
			},
		},
		{
			Name:     "daily-schedule",
			CronExpr: "0 0 12 * * *",
			Target: scheduler.ScalingTarget{
				ScalingGroupName: "group2",
				DesiredCapacity:  10,
			},
		},
	}

	events, err := service.GetUpcomingEvents(schedules, 1, tz)

	assert.NoError(t, err)

	hourlyCount := 0
	dailyCount := 0
	for _, event := range events {
		switch group := event.ScalingGroup; group {
		case "group1":
			hourlyCount++
		case "group2":
			dailyCount++
		}
	}

	assert.Greater(t, hourlyCount, 0)
	assert.LessOrEqual(t, hourlyCount, 24)
	assert.LessOrEqual(t, dailyCount, 1)

	for i := 1; i < len(events); i++ {
		assert.True(t, events[i-1].TriggerTime.Before(events[i].TriggerTime) ||
			events[i-1].TriggerTime.Equal(events[i].TriggerTime))
	}

	invalidSchedules := []scheduler.Schedule{
		{
			Name:     "invalid-schedule",
			CronExpr: "invalid",
			Target: scheduler.ScalingTarget{
				ScalingGroupName: "group3",
				DesiredCapacity:  15,
			},
		},
	}

	invalidEvents, err := service.GetUpcomingEvents(invalidSchedules, 1, tz)
	assert.NoError(t, err)
	assert.Empty(t, invalidEvents)
}

func TestGetUpcomingEventsWithNilTimezone(t *testing.T) {
	defaultTz, _ := time.LoadLocation("UTC")
	service := NewService(defaultTz)

	schedules := []scheduler.Schedule{
		{
			Name:     "test-schedule",
			CronExpr: "0 0 12 * * *",
			Target: scheduler.ScalingTarget{
				ScalingGroupName: "group1",
				DesiredCapacity:  5,
			},
		},
	}

	events, err := service.GetUpcomingEvents(schedules, 1, nil)

	assert.NoError(t, err)
	assert.NotEmpty(t, events)
	for _, event := range events {
		assert.Contains(t, event.TriggerTimeHuman, "UTC")
	}
}
