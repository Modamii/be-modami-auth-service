package entity

import "gitlab.com/lifegoeson-libs/pkg-gokit/apperror"

var (
	ErrUnauthorized = apperror.ErrUnauthorized
	ErrForbidden    = apperror.ErrForbidden
	ErrNotFound     = apperror.ErrNotFound
	ErrBadRequest   = apperror.ErrBadRequest
	ErrInternal     = apperror.ErrInternal
)
