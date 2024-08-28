package workos_errors

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

// TryGetHTTPError returns an error when the http response contains invalid
// status code.
func TryGetHTTPError(r *http.Response) error {
	if r.StatusCode >= 200 && r.StatusCode < 300 {
		return nil
	}

	var msg, code string
	var errors []string
	var fieldErrors []FieldError

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		msg = err.Error()
	} else if isJsonResponse(r) {
		msg, code, errors, fieldErrors = getJsonErrorMessage(body, r.StatusCode)
	} else {
		msg = string(body)
	}

	return HTTPError{
		Code:        r.StatusCode,
		Status:      r.Status,
		RequestID:   r.Header.Get("X-Request-ID"),
		Message:     msg,
		ErrorCode:   code,
		Errors:      errors,
		FieldErrors: fieldErrors,
	}
}

func isJsonResponse(r *http.Response) bool {
	return strings.Contains(r.Header.Get("Content-Type"), "application/json")
}

// ErrorWithMessage struct to hold a single error with a message
type ErrorWithMessage struct {
	Message string `json:"message"`
}

// ResponseErrors is a custom type that can handle unmarshaling []string or []ErrorWithMessage
type ResponseErrors []string

func (e *ResponseErrors) UnmarshalJSON(data []byte) error {
	// Try unmarshaling as []string
	var stringErrors []string
	if err := json.Unmarshal(data, &stringErrors); err == nil {
		*e = stringErrors
		return nil
	}

	// Try unmarshaling as []ErrorWithMessage
	var structErrors []ErrorWithMessage
	if err := json.Unmarshal(data, &structErrors); err == nil {
		for _, se := range structErrors {
			*e = append(*e, se.Message)
		}
		return nil
	}

	return errors.New("errors field is not a valid format")
}

func getJsonErrorMessage(b []byte, statusCode int) (string, string, []string, []FieldError) {
	if statusCode == 422 {
		var unprocesableEntityPayload struct {
			Message          string       `json:"message"`
			Error            string       `json:"error"`
			ErrorDescription string       `json:"error_description"`
			FieldErrors      []FieldError `json:"errors"`
			Code             string       `json:"code"`
		}

		if err := json.Unmarshal(b, &unprocesableEntityPayload); err != nil {
			return string(b), "", nil, nil
		}

		return unprocesableEntityPayload.Message, unprocesableEntityPayload.Code, nil, unprocesableEntityPayload.FieldErrors
	}

	var payload struct {
		Message          string         `json:"message"`
		Error            string         `json:"error"`
		ErrorDescription string         `json:"error_description"`
		Errors           ResponseErrors `json:"errors"`
		Code             string         `json:"code"`
	}

	if err := json.Unmarshal(b, &payload); err != nil {
		return string(b), "", nil, nil
	}

	if payload.Error != "" && payload.ErrorDescription != "" {
		return fmt.Sprintf("%s %s", payload.Error, payload.ErrorDescription), "", nil, nil
	} else if payload.Message != "" && len(payload.Errors) == 0 {
		return payload.Message, "", nil, nil
	} else if payload.Message != "" && len(payload.Errors) > 0 {
		return payload.Message, payload.Code, payload.Errors, nil
	}

	return string(b), "", nil, nil
}

// HTTPError represents an http error.
type HTTPError struct {
	Code        int
	Status      string
	RequestID   string
	Message     string
	ErrorCode   string
	Errors      []string
	FieldErrors []FieldError
}

type FieldError struct {
	Field string
	Code  string
}

func (e HTTPError) Error() string {
	return fmt.Sprintf("%s: request id %q: %s", e.Status, e.RequestID, e.Message)
}
