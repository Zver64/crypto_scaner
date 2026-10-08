package main

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"crypto-scanner/internal/apiclient"
)

// cliError is a failure that run prints like an API error: --json writes it
// in the API error shape. api marks an error the API returned, whose text
// starts with its code; status is its HTTP status, or 0 for a local error.
type cliError struct {
	api       bool
	status    int
	code      string
	message   string
	details   any
	requestID string
}

// failure returns a local error with code.
func failure(code, format string, args ...any) error {
	return &cliError{code: code, message: fmt.Sprintf(format, args...)}
}

// usageError returns a local error of a flag or argument value.
func usageError(format string, args ...any) error {
	return failure("usage", format, args...)
}

// Error returns the text form: an API error's code and message, a local
// error's message, and every listed problem on a line of its own.
func (e *cliError) Error() string {
	text := e.message
	if e.api {
		text = e.code + ": " + text
	}
	if problems, ok := e.details.([]string); ok {
		text += "\n  " + strings.Join(problems, "\n  ")
	}
	return text
}

// response returns the error in the API error shape.
func (e *cliError) response() apiclient.ErrorResponse {
	return apiclient.ErrorResponse{
		Error:     apiclient.APIError{Code: apiclient.APIErrorCode(e.code), Message: e.message, Details: e.details},
		RequestId: e.requestID,
	}
}

// describe returns err as a cliError. Transport errors name the server, and
// any other error comes from cobra's flag and argument checks.
func (c *cli) describe(err error) *cliError {
	var failed *cliError
	var transport *url.Error
	var syntax *json.SyntaxError
	var mistyped *json.UnmarshalTypeError
	switch {
	case errors.As(err, &failed):
		return failed
	case errors.As(err, &transport):
		server := c.server
		if c.profile != "" {
			server += " (profile " + c.profile + ")"
		}
		return &cliError{code: "unreachable", message: fmt.Sprintf("cannot reach %s: %v; is the backend running?", server, transport.Err)}
	case errors.As(err, &syntax) || errors.As(err, &mistyped):
		return &cliError{code: "unexpected_response", message: fmt.Sprintf("unexpected API response from %s: %v; check the profile's server URL", c.server, err)}
	}
	return &cliError{code: "usage", message: err.Error()}
}

// apiResponse is a generated response; every operation declares these errors.
type apiResponse interface {
	StatusCode() int
	GetJSON401() *apiclient.ErrorResponse
	GetJSON403() *apiclient.ErrorResponse
	GetJSON500() *apiclient.ErrorResponse
}

// check returns nil when a generated response holds its expected success, and
// otherwise the API error it decoded, among the operation's own failures and
// the common ones, or its HTTP status.
func (c *cli) check(response apiResponse, succeeded bool, failures ...*apiclient.ErrorResponse) error {
	if succeeded {
		return nil
	}
	status := response.StatusCode()
	decoded := cmp.Or(append(failures, response.GetJSON401(), response.GetJSON403(), response.GetJSON500())...)
	switch {
	case decoded != nil && decoded.Error.Code != "":
		failed := &cliError{
			api: true, status: status, code: string(decoded.Error.Code), message: decoded.Error.Message,
			details: decoded.Error.Details, requestID: decoded.RequestId,
		}
		if status == http.StatusUnauthorized {
			failed.message += " (" + c.tokenHint() + ")"
		}
		return failed
	case status >= 200 && status < 300:
		return &cliError{status: status, code: "unexpected_response", message: fmt.Sprintf("unexpected API response (HTTP %d) from %s; check the server URL and proxy", status, c.server)}
	default:
		return &cliError{status: status, code: "unexpected_response", message: fmt.Sprintf("HTTP %d from %s; check the profile's server URL", status, c.server)}
	}
}

// tokenHint tells how to replace a token the server refused: login replaces
// the token of a profile, and login itself needs a new one.
func (c *cli) tokenHint() string {
	if c.profile == "" {
		return "revoked or wrong token? create one in the Mini App settings"
	}
	return fmt.Sprintf("revoked or wrong token? run scanner login %s --server %s", c.profile, c.server)
}
