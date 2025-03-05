package handlers

import (
	"encoding/json"
	"fmt"
	core "github.com/tomtom-international/macos-actions-runner-controller/pkg/core/api"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter"
	"net/http"
)

func GetStandaloneConfigHandler(t *tarter.Tarter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp, err := json.Marshal(t.GetStandaloneConfig())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := core.SimpleResponse{
				Status:  http.StatusInternalServerError,
				Message: fmt.Sprintf("Failed to marshal standalone config: %v", err),
			}

			json.NewEncoder(w).Encode(response)
			return
		}
		w.Write(resp)
	}
}

func RestartStandaloneRunnersHandler(t *tarter.Tarter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		t.RestartStandaloneRunners()

		response := core.SimpleResponse{
			Message: "Standalone config restarted",
		}
		json.NewEncoder(w).Encode(response)
	}
}
