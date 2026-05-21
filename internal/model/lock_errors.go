package model

import "errors"

var (
	ErrLockNotFound = errors.New("lock not found")
	ErrLockNotOwned = errors.New("lock is owned by another user")
)
