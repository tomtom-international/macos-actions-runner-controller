package results

import (
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/prober/probe"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
)

type Manager interface {
	SetResult(runnerID utils.UID, result probe.Result)
}
