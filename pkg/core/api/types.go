package api

type SimpleResponse struct {
	Status  int    `json:"status,omitempty"`
	Message string `json:"message"`
}
