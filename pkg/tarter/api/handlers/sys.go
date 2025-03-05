package handlers

import (
	"encoding/json"
	"fmt"
	core "github.com/tomtom-international/macos-actions-runner-controller/pkg/core/api"
	"github.com/tomtom-international/macos-actions-runner-controller/pkg/tarter"
	"net/http"
)

func ListRunnerWorkersHandler(t *tarter.Tarter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workers := t.ListRunnerWorkers()
		w.Header().Set("Content-Type", "application/json")
		resp, err := json.Marshal(workers)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := core.SimpleResponse{
				Status:  http.StatusInternalServerError,
				Message: fmt.Sprintf("Failed to marshal runner workers: %v", err),
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		w.Write(resp)
	}
}
