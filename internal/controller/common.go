package controller

import (
	"errors"
	"time"
)

const (
	defaultRequeuePeriod = 10 * time.Second
)

// Common Errors.
var (
	// ErrEnqueueing is returned whenever there is an error enqueueing additional resources.
	ErrEnqueueing = errors.New("enqueueing error")
)
