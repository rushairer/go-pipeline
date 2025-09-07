package gopipeline

import "errors"

// errors

var (
	ErrContextIsClosed  = errors.New("context is canceld")
	ErrAddedError       = errors.New("added error")
	ErrPerformLoopError = errors.New("perform loop error")
	ErrChannelIsClosed  = errors.New("channel is closed")
)
