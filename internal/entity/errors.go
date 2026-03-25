package entity

import "fmt"

type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error { return e.Err }

func NewAppError(code int, message string, err error) *AppError {
	return &AppError{Code: code, Message: message, Err: err}
}

var (
	ErrUnauthorized = &AppError{Code: 401, Message: "unauthorized"}
	ErrForbidden    = &AppError{Code: 403, Message: "forbidden"}
	ErrNotFound     = &AppError{Code: 404, Message: "not found"}
	ErrBadRequest   = &AppError{Code: 400, Message: "bad request"}
	ErrInternal     = &AppError{Code: 500, Message: "internal server error"}
)
