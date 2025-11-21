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
	"sort"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/tomtom-international/macos-actions-runner-controller/internal/scaler/scheduler"
)

// Event represents a forecasted scaling event
type Event struct {
	ScalingGroup     string    `json:"scaling_group"`
	ScheduleName     string    `json:"schedule_name"`
	TriggerTime      time.Time `json:"trigger_time"`
	TriggerTimeHuman string    `json:"trigger_time_human"`
	Action           struct {
		DesiredCapacity int `json:"desired_capacity"`
	} `json:"action"`
}

// Service provides forecasting capabilities for scaling events
type Service struct {
	timezone *time.Location
}

// NewService creates a new forecast service
func NewService(defaultTZ *time.Location) *Service {
	return &Service{
		timezone: defaultTZ,
	}
}

// GetNextEvent returns the next schedule events
func (s *Service) GetNextEvent(schedules []scheduler.Schedule) ([]Event, error) {
	var events []Event
	for _, sch := range schedules {
		event := Event{
			ScalingGroup:     sch.Target.ScalingGroupName,
			ScheduleName:     sch.Name,
			TriggerTime:      sch.NextRunTime,
			TriggerTimeHuman: sch.NextRunTime.Format("Mon, 02 Jan 2006 3:04 PM (MST)"),
			Action: struct {
				DesiredCapacity int `json:"desired_capacity"`
			}{
				DesiredCapacity: sch.Target.DesiredCapacity,
			},
		}

		events = append(events, event)
	}

	return events, nil
}

// GetUpcomingEvents returns upcoming scaling events for the next N days
func (s *Service) GetUpcomingEvents(schedules []scheduler.Schedule, days int, tz *time.Location) ([]Event, error) {
	if tz == nil {
		tz = s.timezone
	}

	now := time.Now().In(tz)
	endTime := now.Add(time.Duration(days) * 24 * time.Hour)

	specParser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	var events []Event

	for _, sch := range schedules {
		cronSchedule, err := specParser.Parse(sch.CronExpr)
		if err != nil {
			continue // Skip invalid schedules
		}
		// Calculate next occurrences
		nextTime := now
		for {
			nextTime = cronSchedule.Next(nextTime)
			if nextTime.After(endTime) {
				break
			}
			event := Event{
				ScalingGroup:     sch.Target.ScalingGroupName,
				ScheduleName:     sch.Name,
				TriggerTime:      nextTime,
				TriggerTimeHuman: nextTime.Format("Mon, 02 Jan 2006 3:04 PM (MST)"),
				Action: struct {
					DesiredCapacity int `json:"desired_capacity"`
				}{
					DesiredCapacity: sch.Target.DesiredCapacity,
				},
			}

			events = append(events, event)
		}
	}
	// Sort events by trigger time
	sort.Slice(events, func(i, j int) bool {
		return events[i].TriggerTime.Before(events[j].TriggerTime)
	})
	return events, nil
}
