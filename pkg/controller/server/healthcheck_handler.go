package server

import (
	"encoding/json"
	core "github.com/tomtom-international/macos-actions-runner-controller/pkg/core/api"
	"net/http"
)

func HealthCheckHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")

		response := core.SimpleResponse{
			Status:  http.StatusOK,
			Message: "Healthy",
		}

		json.NewEncoder(w).Encode(response)
	}
}
