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
	"time"

	"github.com/robfig/cron/v3"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
)

// ScalingTarget represents a scaling operation to be performed
type ScalingTarget struct {
	ScalingGroupName string
	DesiredCapacity  int
	DryRun           bool
}

// ActionHandler is a function that executes a scaling action for a given target
type ActionHandler func(ctx context.Context, action ScalingTarget) error

// Schedule represents a single scheduled scaling target
type Schedule struct {
	NextRunTime time.Time
	Name        string
	CronExpr    string
	Target      ScalingTarget
	ID          cron.EntryID
}

// Scheduler manages scheduled scaling targets and actions
type Scheduler struct {
	cron      *cron.Cron
	schedules map[string]Schedule
	handler   ActionHandler
	mu        sync.RWMutex
}

// NewScheduler creates a new scheduler
func NewScheduler() *Scheduler {
	cronScheduler := cron.New(cron.WithSeconds())

	return &Scheduler{
		cron:      cronScheduler,
		schedules: make(map[string]Schedule),
	}
}

// AddSchedule adds a new schedule to the scheduler
func (s *Scheduler) AddSchedule(cronExpr string, name string, target ScalingTarget, handler ActionHandler) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.handler == nil {
		s.handler = handler
	}

	// Create a job that will execute our handler action for the target
	job := s.createJob(name, target)

	id, err := s.cron.AddFunc(cronExpr, job)
	if err != nil {
		return fmt.Errorf("failed to add schedule '%s': %w", name, err)
	}

	s.schedules[name] = Schedule{
		ID:       id,
		Name:     name,
		CronExpr: cronExpr,
		Target:   target,
	}

	logger.Infof("Schedule '%s' added for node group '%s' with cron '%s'",
		name, target.ScalingGroupName, cronExpr)

	return nil
}

// createJob returns a function that executes the handler action for the target
func (s *Scheduler) createJob(name string, target ScalingTarget) func() {
	return func() {
		ctx := context.Background()
		logger.Infof("Executing schedule '%s' for node group '%s'", name, target.ScalingGroupName)

		if err := s.handler(ctx, target); err != nil {
			logger.Warnf("Schedule '%s' executed with error: %v", name, err)
		} else {
			logger.Infof("Schedule '%s' executed successfully", name)
		}

		s.updateNextRunTime(name)
	}
}

// updateNextRunTime updates the next run time for a schedule
func (s *Scheduler) updateNextRunTime(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	schedule, exists := s.schedules[name]
	if !exists {
		return
	}

	entry := s.cron.Entry(schedule.ID)
	schedule.NextRunTime = entry.Next
	s.schedules[name] = schedule
}

// GetSchedules returns all schedules
func (s *Scheduler) GetSchedules() []Schedule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Schedule, 0, len(s.schedules))
	for _, schedule := range s.schedules {
		result = append(result, schedule)
	}

	return result
}

// Start begins the scheduler
func (s *Scheduler) Start() {
	logger.Debugf("Starting Scheduler...")
	s.cron.Start()
	// Update all next run times
	for name := range s.schedules {
		s.updateNextRunTime(name)
	}
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	logger.Debugf("Stopping Scheduler...")
	s.cron.Stop()
}
