package errs

import "errors"

type Error struct {
	Code    int    `json:"exit_code"`
	Kind    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"status,omitempty"`
	Details any    `json:"details,omitempty"`
}

func (e *Error) Error() string { return e.Message }
func New(code int, kind, message string) *Error {
	return &Error{Code: code, Kind: kind, Message: message}
}
func As(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		result := *e
		result.Message = err.Error()
		return &result
	}
	return New(1, "error", err.Error())
}
