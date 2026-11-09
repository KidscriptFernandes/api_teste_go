package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const MaxBodyBytes = 1 << 20 // 1 MiB

type ErrorResponse struct {
	Error string `json:"error"`
}

// DecodeError carries a client-facing message together with the HTTP status
// that best describes what went wrong while reading the request body.
type DecodeError struct {
	Status  int
	Message string
}

func (e *DecodeError) Error() string { return e.Message }

func JSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	if payload == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, ErrorResponse{Error: message})
}

func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return newDecodeError(err)
	}

	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return &DecodeError{
			Status:  http.StatusBadRequest,
			Message: "the request body must contain a single JSON object",
		}
	}

	return nil
}

func newDecodeError(err error) error {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return &DecodeError{
			Status:  http.StatusRequestEntityTooLarge,
			Message: fmt.Sprintf("the request body must not exceed %d bytes", MaxBodyBytes),
		}
	}

	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) || errors.Is(err, io.ErrUnexpectedEOF) {
		return &DecodeError{
			Status:  http.StatusBadRequest,
			Message: "the request body contains malformed JSON",
		}
	}

	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return &DecodeError{
			Status:  http.StatusBadRequest,
			Message: fmt.Sprintf("the field %q has an invalid type", typeErr.Field),
		}
	}

	if strings.HasPrefix(err.Error(), "json: unknown field") {
		return &DecodeError{
			Status:  http.StatusBadRequest,
			Message: "the request body contains an unknown field",
		}
	}

	if errors.Is(err, io.EOF) {
		return &DecodeError{
			Status:  http.StatusBadRequest,
			Message: "the request body is required",
		}
	}

	return &DecodeError{
		Status:  http.StatusBadRequest,
		Message: "the request body is invalid",
	}
}
