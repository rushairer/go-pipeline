package gopipeline

import "errors"

// errors

var (
	ErrContextIsClosed  = errors.New("context is canceled")
	ErrPerformLoopError = errors.New("perform loop error")
	ErrChannelIsClosed  = errors.New("channel is closed")
)
