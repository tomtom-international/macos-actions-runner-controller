package api

import (
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/api/handlers"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/state"

	"github.com/gorilla/mux"
)

func SetupRoutes(r *mux.Router, sm *state.StateManager) {
	r.HandleFunc("/runners", handlers.ListRunnersHandler(sm)).Methods("GET")
	r.HandleFunc("/runners/{id}", handlers.GetRunnerHandler(sm)).Methods("GET")
	r.HandleFunc("/runners/{id}/stop", handlers.StopRunnerHandler(sm)).Methods("PUT")
}

func SetupSysRoutes(r *mux.Router, t *tarter.Tarter) {
	r.HandleFunc("/sys/workers", handlers.ListRunnerWorkersHandler(t)).Methods("GET")
}

func SetupStandaloneTarterRoutes(r *mux.Router, t *tarter.Tarter) {
	r.HandleFunc("/sys/standalone/config", handlers.GetStandaloneConfigHandler(t)).Methods("GET")
	r.HandleFunc("/sys/standalone/restart", handlers.RestartStandaloneRunnersHandler(t)).Methods("PUT")
}
