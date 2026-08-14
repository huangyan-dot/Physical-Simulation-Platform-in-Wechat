package service

import "errors"

// service 层统一错误，controller 据此映射 HTTP 状态码
var (
	ErrBadRequest = errors.New("bad request")
	ErrNotFound   = errors.New("not found")
	ErrForbidden  = errors.New("forbidden")
	ErrConflict   = errors.New("conflict")
)
