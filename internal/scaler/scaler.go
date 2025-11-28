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

package scaler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	sqsTypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/tomtom-international/macos-actions-runner-controller/internal/scaler/config"
	"github.com/tomtom-international/macos-actions-runner-controller/internal/scaler/forecast"
	"github.com/tomtom-international/macos-actions-runner-controller/internal/scaler/scheduler"
	"github.com/tomtom-international/macos-actions-runner-controller/internal/scaler/service"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/aws/asg"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/aws/ec2"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/aws/sqs"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/controller"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/clients/etcd"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
)

const leaderElectionKey = "/election/scaler/leader"

type Scaler struct {
	scheduleCfg        *config.Config
	scalingService     *service.ScalingService
	terminationService *service.TerminationService
	scheduler          *scheduler.Scheduler
	forecaster         *forecast.Service
	sqsClient          *sqs.SQSClient
	etcdClient         *etcd.Client
	leaderElection     *etcd.LeaderElection
	leaderIdentity     string
	leaderMu           sync.RWMutex
	dryRun             bool
	isLeader           bool
}

func NewScaler(
	cfg config.ScalerConfig,
	scheduleCfg *config.Config,
) (*Scaler, error) {

	etcdClient, err := etcd.NewEtcdClient(etcd.ClientConfig{
		Endpoints:   cfg.EtcdEndpoints,
		TLSEnabled:  cfg.EtcdTLSEnabled,
		TLSCertFile: cfg.EtcdTLSCertFile,
		TLSKeyFile:  cfg.EtcdTLSKeyFile,
		TLSCAFile:   cfg.EtcdTLSCAFile,
		Username:    cfg.EtcdUsername,
		Password:    cfg.EtcdPassword,
	})
	if err != nil {
		return nil, err
	}

	asgClient, err := asg.NewClient(asg.ClientConfig{Region: cfg.AwsRegion})
	if err != nil {
		return nil, fmt.Errorf("failed to create ASG client: %w", err)
	}

	ec2Client, err := ec2.NewClient(ec2.ClientConfig{Region: cfg.AwsRegion})
	if err != nil {
		return nil, fmt.Errorf("failed to create EC2 client: %w", err)
	}

	sqsClient, err := sqs.NewClient(
		&sqs.SQSConfig{
			QueueURL:  cfg.ScalerSqsQueueURL,
			AWSRegion: cfg.AwsRegion,
			ErrHandle: func(err error) {
				logger.Errorf("Sqs error: %s", err.Error())
			},
		},
	)
	if err != nil {
		return nil, err
	}

	controllerClient, err := controller.NewClient(controller.ClientConfig{
		Server:         cfg.MacosRunnerControllerURL,
		APIVersionPath: cfg.MacosRunnerControllerAPIVersion,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create Controller client: %w", err)
	}

	instanceCoordinator := service.NewInstanceCoordinator(service.InstanceCoordinatorConfig{}, ec2Client)
	nodeCoordinator := service.NewNodeCoordinator(controllerClient)

	// Create scaling service
	scalingService := service.NewScalingService(
		asgClient,
		sqsClient,
		instanceCoordinator,
		nodeCoordinator,
	)

	// Create termination service
	terminationService := service.NewTerminationService(
		service.TerminationServiceConfig{
			MaxTerminationRetries: cfg.MaxTerminationRetryCount,
		},
		asgClient,
		sqsClient,
		instanceCoordinator,
		nodeCoordinator,
	)

	// Create scheduler
	sch := scheduler.NewScheduler()
	// Create forecast service
	forecastSvc := forecast.NewService(time.UTC)

	identity, err := os.Hostname()
	if err != nil {
		identity = fmt.Sprintf("scaler-%d", time.Now().Unix())
	}

	// Create leader election
	logger.Debugf("Creating leader election with identity '%s'", identity)
	leaderElection, err := etcdClient.NewLeaderElection(etcd.ElectionConfig{
		Identity:    identity,
		ElectionKey: leaderElectionKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create leader election: %w", err)
	}

	logger.Debugf("Scaler initialized successfully")

	return &Scaler{
		scheduleCfg:        scheduleCfg,
		scalingService:     scalingService,
		terminationService: terminationService,
		scheduler:          sch,
		forecaster:         forecastSvc,
		sqsClient:          sqsClient,
		etcdClient:         etcdClient,
		dryRun:             cfg.DryRun,

		leaderElection: leaderElection,
		isLeader:       false,
		leaderIdentity: identity,
	}, nil
}

// Start begins the application, setting up schedules and starting the scheduler
func (s *Scaler) Start(ctx context.Context, wg *sync.WaitGroup) error {
	// Start leader election in background
	go s.runLeaderElection(ctx)

	// Start SQS listener (processing SQS messages can be done not only by leader)
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.listenForSqsMessage(ctx, s.processSqsMessage)
	}()

	return nil
}

// Stop gracefully shuts down the application
func (s *Scaler) Stop() error {
	logger.Infof("Stopping Scaler...")
	s.stepDown()

	if s.leaderElection != nil {
		if err := s.leaderElection.Close(); err != nil {
			logger.Errorf("Failed to close leader election: %v", err)
		}
	}

	if s.etcdClient != nil {
		if err := s.etcdClient.Close(); err != nil {
			return fmt.Errorf("failed to close etcd client: %w", err)
		}
		logger.Debugf("Etcd client closed")
	}

	return nil
}

// runLeaderElection handles the leader election lifecycle
func (s *Scaler) runLeaderElection(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			logger.Debugf("Stopping leader election due to context cancellation...")
			return
		default:
			// Campaign to become leader (blocks until we become leader)
			if err := s.leaderElection.Campaign(ctx); err != nil {
				logger.Errorf("Leader election campaign failed: %v", err)

				// Recreate leader election with fresh session and retry
				if recreateErr := s.recreateLeaderElection(); recreateErr != nil {
					logger.Errorf("Failed to recreate leader election: %v", recreateErr)
					time.Sleep(5 * time.Second)
					continue
				}

				logger.Debugf("Leader election recreated successfully, retrying campaign...")
				time.Sleep(2 * time.Second)
				continue
			}

			s.becomeLeader(ctx)
		}
	}
}

// recreateLeaderElection closes the old leader election and creates a new one with a fresh session
func (s *Scaler) recreateLeaderElection() error {
	// Close old leader election (ignoring errors as the session may already be invalid)
	if s.leaderElection != nil {
		_ = s.leaderElection.Close()
	}

	logger.Debugf("Recreating leader election with identity '%s'", s.leaderIdentity)
	leaderElection, err := s.etcdClient.NewLeaderElection(etcd.ElectionConfig{
		Identity:    s.leaderIdentity,
		ElectionKey: leaderElectionKey,
	})
	if err != nil {
		return fmt.Errorf("failed to recreate leader election: %w", err)
	}
	s.leaderElection = leaderElection
	return nil
}

// becomeLeader starts scheduler and runs as leader
func (s *Scaler) becomeLeader(ctx context.Context) {
	s.leaderMu.Lock()
	s.isLeader = true
	s.leaderMu.Unlock()

	logger.Debugf("Leader adding schedules...")
	if err := s.setupSchedules(); err != nil {
		logger.Errorf("Failed to setup schedules: %v", err)
		return
	}

	s.scheduler.Start()
	defer s.scheduler.Stop()

	// Run as leader until context is canceled or lose leadership
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Debugf("Context canceled, stepping down as leader...")
			s.stepDown()
			return
		case <-ticker.C:
			if !s.leaderElection.IsLeader() {
				logger.Warnf("Lost leadership, stepping down...")
				s.stepDown()
				return
			}
		}
	}
}

// stepDown gracefully steps down from leadership
func (s *Scaler) stepDown() {
	s.leaderMu.Lock()
	defer s.leaderMu.Unlock()

	if !s.isLeader {
		return
	}

	logger.Debugf("Stepping down from leadership...")
	s.isLeader = false

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.leaderElection.Resign(ctx); err != nil {
		logger.Errorf("Failed to resign leadership: %v", err)
	}
}

// IsLeader returns whether this replica is currently the leader
func (s *Scaler) IsLeader() bool {
	s.leaderMu.RLock()
	defer s.leaderMu.RUnlock()
	return s.isLeader
}

func (s *Scaler) listenForSqsMessage(ctx context.Context, handlerFunc func(message *sqsTypes.Message) error) {
	logger.Debugf("Listening for new messages from SQS queue")
	s.sqsClient.PollForMessages(ctx, handlerFunc)
}

func (s *Scaler) processSqsMessage(msg *sqsTypes.Message) error {
	logger.Debugf("Processing new message from SQS queue")
	var message service.ScalingOperation
	err := json.Unmarshal([]byte(*msg.Body), &message)
	if err != nil {
		logger.Errorf("Failed to unmarshal message %s from queue: %s", *msg.MessageId, err.Error())
		return err
	}
	err = s.terminationService.ProcessNodeTermination(&message)
	if err != nil {
		logger.Errorf("Failed to process termination message %s from queue: %s", *msg.MessageId, err.Error())
		return err
	}
	return nil
}

// setupSchedules configures the scheduler with all schedules from config
func (s *Scaler) setupSchedules() error {
	// For each Scaling Group in config, register scaling schedules
	for _, group := range s.scheduleCfg.ScalingGroups {
		for _, schedule := range group.Schedules {
			// Create scaling target
			target := scheduler.ScalingTarget{
				ScalingGroupName: group.Name,
				DesiredCapacity:  schedule.DesiredCapacity,
				DryRun:           s.dryRun,
			}

			// Register with scheduler
			if err := s.scheduler.AddSchedule(schedule.Cron, schedule.Name, target, s.schedulerHandler); err != nil {
				return err
			}
		}
	}
	return nil
}

// GetNextScalerEvent returns upcoming scaling events
func (s *Scaler) GetNextScalerEvent() ([]forecast.Event, error) {
	schedules := s.scheduler.GetSchedules()
	return s.forecaster.GetNextEvent(schedules)
}

func (s *Scaler) GetUpcomingScalerEvents(days int, tz *time.Location) ([]forecast.Event, error) {
	schedules := s.scheduler.GetSchedules()
	return s.forecaster.GetUpcomingEvents(schedules, days, tz)
}

// schedulerHandler performs the actual scaling action against the specified target
func (s *Scaler) schedulerHandler(ctx context.Context, target scheduler.ScalingTarget) error {
	err := s.scalingService.ProcessScalingAction(ctx, target.ScalingGroupName, target.DesiredCapacity, target.DryRun)
	if err != nil {
		logger.Errorf("Failed to handle schedule event for scaling group %s, error: %v",
			target.ScalingGroupName,
			err,
		)
		return err
	}
	return nil
}
