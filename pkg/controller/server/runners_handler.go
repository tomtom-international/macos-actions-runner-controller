package server

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	core "github.com/tomtom-international/macos-actions-runner-controller/pkg/core/api"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/core/types"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/utils"
	"net/http"
)

func GetRunnerListHandler(c ControllerInterface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == http.MethodOptions {
			return
		}

		var resp []byte
		var err error

		runners, err := c.GetRunnerList()

		if err == nil {
			resp, err = json.Marshal(runners)
		}

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := core.SimpleResponse{
				Status:  http.StatusInternalServerError,
				Message: fmt.Sprintf("Failed to list runners: %v", err.Error()),
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		w.Write(resp)
	}
}

func GetRunnerHandler(c ControllerInterface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		vars := mux.Vars(r)
		runnerID := utils.UID(vars["runner_uid"])
		if runnerID == "" {
			w.WriteHeader(http.StatusBadRequest)
			response := core.SimpleResponse{
				Status:  http.StatusBadRequest,
				Message: "Runner id is required",
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		var resp []byte
		var err error

		runner, err := c.GetRunnerInfo(runnerID)

		if err == nil {
			if runner == nil {
				w.WriteHeader(http.StatusNotFound)
				response := core.SimpleResponse{
					Status:  http.StatusNotFound,
					Message: fmt.Sprintf("Runner %s not found", runnerID),
				}
				json.NewEncoder(w).Encode(response)
				return
			}
			resp, err = json.Marshal(runner)
		}

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := core.SimpleResponse{
				Status:  http.StatusInternalServerError,
				Message: fmt.Sprintf("Failed to get runner: %v", err.Error()),
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		w.Write(resp)
	}
}

func GetRunnerStatusUpdateHandler(c ControllerInterface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		vars := mux.Vars(r)
		runnerID := utils.UID(vars["runner_uid"])
		if runnerID == "" {
			w.WriteHeader(http.StatusBadRequest)
			response := core.SimpleResponse{
				Status:  http.StatusBadRequest,
				Message: "Runner id is required",
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		var body types.RunnerStatusUpdate
		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			response := core.SimpleResponse{
				Status:  http.StatusBadRequest,
				Message: "Failed to read runner status update request",
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		runner, err := c.ProcessRunnersStatusUpdate(body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := core.SimpleResponse{
				Status:  http.StatusInternalServerError,
				Message: fmt.Sprintf("Failed to update runner status: %v", err.Error()),
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		if runner == nil {
			w.WriteHeader(http.StatusNotFound)
			response := core.SimpleResponse{
				Status:  http.StatusNotFound,
				Message: fmt.Sprintf("Runner %s not found", runnerID),
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(runner)
	}
}
