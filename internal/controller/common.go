package controller

import "errors"

// Common Errors.
var (
	// ErrEnqueueing is returned whenever there is an error enqueueing additional resources.
	ErrEnqueueing = errors.New("enqueueing error")
)
