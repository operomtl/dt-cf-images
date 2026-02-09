package api

import (
	"encoding/json"
	"log"
	"net/http"
)

// Response is the standard Cloudflare API response envelope.
type Response struct {
	Result   interface{}  `json:"result"`
	Success  bool         `json:"success"`
	Errors   []APIError   `json:"errors"`
	Messages []APIMessage `json:"messages"`
}

// APIMessage represents a single message in the Cloudflare API response.
type APIMessage struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// APIError represents a single error in the Cloudflare API response.
type APIError struct {
	Code             int             `json:"code"`
	Message          string          `json:"message"`
	DocumentationURL string          `json:"documentation_url,omitempty"`
	Source           *APIErrorSource `json:"source,omitempty"`
}

// APIErrorSource identifies the field that caused the error.
type APIErrorSource struct {
	Pointer string `json:"pointer"`
}

// ResultInfo carries pagination metadata for list endpoints.
type ResultInfo struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Count      int `json:"count"`
	TotalCount int `json:"total_count"`
	TotalPages int `json:"total_pages,omitempty"`
}

// SuccessResponse builds a successful Cloudflare API response.
func SuccessResponse(result interface{}) Response {
	return Response{
		Result:   result,
		Success:  true,
		Errors:   []APIError{},
		Messages: []APIMessage{},
	}
}

// ErrorResponse builds an error Cloudflare API response.
func ErrorResponse(code int, message string) Response {
	return Response{
		Result:  nil,
		Success: false,
		Errors: []APIError{
			{Code: code, Message: message},
		},
		Messages: []APIMessage{},
	}
}

// PaginatedResponse builds a successful response that includes result_info for pagination.
func PaginatedResponse(result interface{}, info ResultInfo) map[string]interface{} {
	return map[string]interface{}{
		"result":      result,
		"success":     true,
		"errors":      []APIError{},
		"messages":    []APIMessage{},
		"result_info": info,
	}
}

// WriteJSON serialises resp as JSON and writes it to w with the given HTTP status code.
func WriteJSON(w http.ResponseWriter, status int, resp interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("WriteJSON: failed to encode response: %v", err)
	}
}
