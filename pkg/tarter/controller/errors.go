package controller

import "errors"

type ControllerErrors struct {
	Reason  StatusReason
	Code    string
	Message string
}

// ControllerStatus is exposed by errors that can be converted to a controller.Status object
// for finer grained details.
type ControllerStatus interface {
	Status() StatusReason
}

// ControllerStatusCode is exposed by errors that can be converted to a controller.StatusCode object
// for finer grained details.
type ControllerStatusCode interface {
	StatusCode() string
}

type StatusReason string

const (
	StatusReasonAlreadyExists StatusReason = "AlreadyExists"
	StatusReasonUnknown       StatusReason = ""
	StatusReasonNotFound      StatusReason = "NotFound"
	StatusReasonBadRequest    StatusReason = "BadRequest"
)

// Error implements the Error interface.
func (e *ControllerErrors) Error() string {
	return e.Message
}

// Status allows access to error's status without having to know the detailed workings
// of ControllerErrors.
func (e *ControllerErrors) Status() StatusReason {
	return e.Reason
}

// StatusCode allows access to error's code field without having to know the detailed workings
// of ControllerErrors.
func (e *ControllerErrors) StatusCode() string {
	return e.Code
}

func getStatusReason(statusCode int) StatusReason {
	switch statusCode {
	case 400:
		return StatusReasonBadRequest
	case 404:
		return StatusReasonNotFound
	case 409:
		return StatusReasonAlreadyExists
	default:
		return StatusReasonUnknown
	}
}

func IsAlreadyExists(err error) bool {
	return ReasonForError(err) == StatusReasonAlreadyExists
}

func ErrorCode(err error) string {
	if code, ok := err.(ControllerStatusCode); ok || errors.As(err, &code) {
		return code.(*ControllerErrors).Code
	}
	return ""
}

// ReasonForError returns the StatusReason for a particular error.
func ReasonForError(err error) StatusReason {
	if status, ok := err.(ControllerStatus); ok || errors.As(err, &status) {
		return status.Status()
	}
	return StatusReasonUnknown
}
