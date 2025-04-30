/*
 * Copyright 2025 TomTom N.V.
 * Copyright 2015 The Kubernetes Authors.
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

package prober

import (
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/logger"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/prober/probe"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/prober/results"
	pt "github.com/tomtom-international/macos-actions-runner-controller/pkg/prober/types"
	tt "github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/types"
	"time"
)

type worker struct {
	// Where to store these workers results.
	resultsManager results.Manager
	// Channel for stopping the probe.
	stopCh chan struct{}
	// Describes the probe configuration (read-only)
	spec         *pt.Probe
	probeTarget  *pt.ProbeTarget
	probeManager *ProberManager
	// The probe value during the initial delay.
	initialValue probe.Result
	// The last probe result for this worker.
	lastResult probe.Result
	// The type of the worker.
	probeType pt.ProbeType
	// How many times in a row the probe has returned the same result.
	resultRun int
}

func newWorker(
	m *ProberManager,
	probeType pt.ProbeType,
	target *pt.ProbeTarget,
	resultsManager results.Manager) *worker {
	w := &worker{
		stopCh:         make(chan struct{}, 1),
		probeManager:   m,
		probeType:      probeType,
		probeTarget:    target,
		resultsManager: resultsManager,
		spec:           target.StartupProbe,
	}
	switch probeType {
	case pt.Liveness:
		w.spec = target.LivenessProbe
		w.initialValue = probe.Success
	case pt.Startup:
		w.spec = target.StartupProbe
		w.initialValue = probe.Unknown
	}
	return w
}

// run periodically probes the target.
func (w *worker) run() {
	probeTickerPeriod := time.Duration(w.spec.PeriodSeconds) * time.Second

	probeTicker := time.NewTicker(probeTickerPeriod)

	defer func() {
		// Clean up.
		probeTicker.Stop()
		logger.Debugf("Removing Prober %v probe for target %v ...", w.probeType, w.probeTarget.ID)
		w.probeManager.removeWorker(w.probeTarget.ID, w.probeTarget.Name, w.probeType)
	}()

probeLoop:
	for w.doProbe() {
		select {
		case <-w.stopCh:
			break probeLoop
		case <-probeTicker.C:
		}
	}
}

func (w *worker) stop() {
	select {
	case w.stopCh <- struct{}{}:
	default: // Non-blocking.
	}
}

func (w *worker) doProbe() (keepGoing bool) {
	if w.probeTarget.Type == pt.ProbeTargetTypeTartRunner {
		targetState, ok := w.probeManager.stateManager.GetRunnerState(w.probeTarget.ID)
		if !ok {
			// Either the target has not been created yet, or it was already deleted.
			logger.Infof("No status for %s target with %s id and name %s",
				w.probeTarget.Type, w.probeTarget.ID, w.probeTarget.Name)
			return true
		}

		// Worker should be terminated if target is stopped.
		if targetState.Status != tt.Running {
			if targetState.Status == tt.Stopping && w.probeType == pt.Startup {
				logger.Infof("Target stop was requested before target has fully started")
			}
			logger.Infof("Target is stopped. Exiting probe worker for %s target with %s id terminated.",
				w.probeTarget.Type, w.probeTarget.ID)
			return false
		}

		// Disabled Probe for InitialDelaySeconds.
		if int32(time.Since(targetState.StartedAt).Seconds()) < w.spec.InitialDelaySeconds {
			return true
		}
	}
	// TODO: Implement tarter health check for probe by Tarter Controller
	if w.probeTarget.Type == pt.ProbeTargetTypeTarter {
		return false
	}

	result, err := w.probeManager.prober.probe(w.probeType, w.probeTarget)
	if err != nil {
		// Prober error, throw away the result.
		return true
	}

	if w.lastResult == result {
		w.resultRun++
	} else {
		w.lastResult = result
		w.resultRun = 1
	}

	if (result == probe.Failure && w.resultRun < int(w.spec.FailureThreshold)) ||
		(result == probe.Success && w.resultRun < int(w.spec.SuccessThreshold)) {
		// Success or failure is below threshold - leave the probe state unchanged.
		return true
	}

	w.resultsManager.SetResult(w.probeTarget.ID, result)

	if w.probeType == pt.Startup && w.resultRun >= int(w.spec.SuccessThreshold) {
		// Target passed startup check. Notifying result manager and stop probing.
		return false
	}

	if (w.probeType == pt.Liveness || w.probeType == pt.Startup) && result == probe.Failure {
		// The target fails a liveness/startup check, it will need to be restarted.
		// Notifying result manager and stop probing.
		return false
	}

	return true
}
