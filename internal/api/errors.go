package api

import "net/http"

// BadRequest writes a 400 error response.
func BadRequest(w http.ResponseWriter, msg string) {
	WriteJSON(w, http.StatusBadRequest, ErrorResponse(9400, msg))
}

// Unauthorized writes a 401 error response.
func Unauthorized(w http.ResponseWriter) {
	WriteJSON(w, http.StatusUnauthorized, ErrorResponse(9401, "Authentication required"))
}

// NotFound writes a 404 error response.
func NotFound(w http.ResponseWriter, msg string) {
	WriteJSON(w, http.StatusNotFound, ErrorResponse(9404, msg))
}

// Conflict writes a 409 error response.
func Conflict(w http.ResponseWriter, msg string) {
	WriteJSON(w, http.StatusConflict, ErrorResponse(9409, msg))
}

// UnprocessableEntity writes a 422 error response.
func UnprocessableEntity(w http.ResponseWriter, msg string) {
	WriteJSON(w, http.StatusUnprocessableEntity, ErrorResponse(9422, msg))
}

// TooLarge writes a 413 error response.
func TooLarge(w http.ResponseWriter, msg string) {
	WriteJSON(w, http.StatusRequestEntityTooLarge, ErrorResponse(9413, msg))
}
