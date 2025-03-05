package api

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"net/http"
)

// TODO: response headers not working
func MethodNotAllowedHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Header().Set("Content-Type", "application/json")

		response := SimpleResponse{
			Status:  http.StatusMethodNotAllowed,
			Message: "The method not allowed.",
		}

		json.NewEncoder(w).Encode(response)
	}
}

func NotFoundHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Header().Set("Content-Type", "application/json")

		response := SimpleResponse{
			Status:  http.StatusNotFound,
			Message: "Page not found.",
		}

		json.NewEncoder(w).Encode(response)
	}
}

func SetupGenericHandlers(r *mux.Router) {
	// Define a custom handler for unsupported methods
	r.MethodNotAllowedHandler = MethodNotAllowedHandler()
	// Define a custom handler for 404 responses
	r.NotFoundHandler = NotFoundHandler()
}
