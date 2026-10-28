package utils

import (
	"encoding/json"
	"net/http"
)

type ErrorResponse struct {
	Error   string            `json:"error"`
	Details map[string]string `json:"details,omitempty"`
}

// JSON sends a JSON response with status code and data.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// ErrorJSON sends a formatted JSON error response.
func ErrorJSON(w http.ResponseWriter, status int, message string) {
	JSON(w, status, ErrorResponse{Error: message})
}

// ValidationErrorJSON sends a structured validation failure response.
func ValidationErrorJSON(w http.ResponseWriter, details map[string]string) {
	JSON(w, http.StatusBadRequest, ErrorResponse{
		Error:   "validation failed",
		Details: details,
	})
}
