/*
 * Copyright 2025 TomTom N.V.
 * Copyright 2014 The Kubernetes Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package tarter

import "errors"

type TarterErrors struct {
	Reason  StatusReason
	Code    string
	Message string
}

// TarterStatus is exposed by errors that can be converted to a tarterErrors.Status object
// for finer grained details.
type TarterStatus interface {
	Status() StatusReason
}

// TarterStatusCode is exposed by errors that can be converted to a tarterErrors.StatusCode object
// for finer grained details.
type TarterStatusCode interface {
	StatusCode() string
}

type StatusReason string

const (
	StatusReasonUnknown    StatusReason = ""
	StatusReasonNotFound   StatusReason = "NotFound"
	StatusReasonBadRequest StatusReason = "BadRequest"
)

// Error implements the Error interface.
func (e *TarterErrors) Error() string {
	return e.Message
}

// Status allows access to error's status without having to know the detailed workings
// of TarterErrors.
func (e *TarterErrors) Status() StatusReason {
	return e.Reason
}

// StatusCode allows access to error's code field without having to know the detailed workings
// of TarterErrors.
func (e *TarterErrors) StatusCode() string {
	return e.Code
}

func getStatusReason(statusCode int) StatusReason {
	switch statusCode {
	case 400:
		return StatusReasonBadRequest
	case 404:
		return StatusReasonNotFound
	default:
		return StatusReasonUnknown
	}
}

func ErrorCode(err error) string {
	if code, ok := err.(TarterStatusCode); ok || errors.As(err, &code) {
		return code.(*TarterErrors).Code
	}
	return ""
}

// ReasonForError returns the StatusReason for a particular error.
func ReasonForError(err error) StatusReason {
	if status, ok := err.(TarterStatus); ok || errors.As(err, &status) {
		return status.Status()
	}
	return StatusReasonUnknown
}
