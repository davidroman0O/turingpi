package common

import (
	"time"

	"github.com/davidroman0O/gostage"
	"github.com/davidroman0O/turingpi/workflows/actions"
)

// WaitAction is a generic action to wait for a specified time
type WaitAction struct {
	actions.TuringPiAction
	seconds int
}

// NewWaitAction creates a new wait action
func NewWaitAction(seconds int) *WaitAction {
	return &WaitAction{
		TuringPiAction: actions.NewTuringPiAction(
			"wait",
			"Waits for a specified number of seconds",
		),
		seconds: seconds,
	}
}

// Execute implements the Action interface
func (a *WaitAction) Execute(ctx *gostage.ActionContext) error {
	ctx.Logger.Info("Waiting for %d seconds", a.seconds)
	select {
	case <-ctx.GoContext.Done():
		return nil
	case <-time.After(time.Duration(a.seconds) * time.Second):
		return nil
	}
}
