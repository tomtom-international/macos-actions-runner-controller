package http

import (
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/prober/probe"
	"time"
)

func New() Prober {
	return httpProber{}
}

type Prober interface {
	Probe(timeout time.Duration) (probe.Result, string, error)
}

type httpProber struct{}

func (g httpProber) Probe(timeout time.Duration) (probe.Result, string, error) {
	// TODO: JUST MOPCKING THE PROBE
	return probe.Success, "HTTP probe success", nil
}
