package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	core "github.com/tomtom-international/macos-actions-runner-controller/pkg/core/api"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter/state"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
	"net/http"
)

func ListRunnersHandler(sm *state.StateManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		status := r.URL.Query().Get("status")

		var resp []byte
		var err error

		if status == "all" {
			resp, err = json.Marshal(sm.ListRunners())
		} else {
			resp, err = json.Marshal(sm.ListActiveRunners())
		}

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := core.SimpleResponse{
				Status:  http.StatusInternalServerError,
				Message: fmt.Sprintf("Failed to marshal runners from state: %v", err),
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		w.Write(resp)
	}
}

func GetRunnerHandler(sm *state.StateManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		vars := mux.Vars(r)
		runnerID := utils.UID(vars["id"])
		if runnerID == "" {
			w.WriteHeader(http.StatusBadRequest)
			response := core.SimpleResponse{
				Status:  http.StatusBadRequest,
				Message: "Runner id is required.",
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		runner, exists := sm.GetRunnerState(runnerID)
		if !exists {
			w.WriteHeader(http.StatusNotFound)
			response := core.SimpleResponse{
				Status:  http.StatusNotFound,
				Message: fmt.Sprintf("The runner with id %v not found.", runnerID),
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		resp, err := json.Marshal(runner)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := core.SimpleResponse{
				Status:  http.StatusInternalServerError,
				Message: fmt.Sprintf("Failed to marshal runner from state: %v", err),
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		w.Write(resp)
	}
}

func StopRunnerHandler(sm *state.StateManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		vars := mux.Vars(r)
		runnerID := utils.UID(vars["id"])
		if runnerID == "" {
			w.WriteHeader(http.StatusBadRequest)
			response := core.SimpleResponse{
				Status:  http.StatusBadRequest,
				Message: "Runner id is required.",
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		exists := sm.StopRunner(runnerID)
		if !exists {
			w.WriteHeader(http.StatusBadRequest)
			response := core.SimpleResponse{
				Status:  http.StatusBadRequest,
				Message: fmt.Sprintf("The runner with id %v not found.", runnerID),
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		response := core.SimpleResponse{
			Message: fmt.Sprintf("Runner with id %v is stopping", runnerID),
		}
		json.NewEncoder(w).Encode(response)
	}
}
