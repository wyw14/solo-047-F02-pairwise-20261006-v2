package repeater

import (
	"context"
	"fmt"
	"time"

	"github.com/wyw14/cry-103/internal/feed"
	"github.com/wyw14/cry-103/internal/store"
	"github.com/wyw14/cry-103/internal/telemetry"
)

type RampStep struct {
	Number  int     `json:"number"`
	Voltage float64 `json:"voltage"`
}

type RampController struct {
	actuator    *feed.Actuator
	repository  *telemetry.Repository
	window      telemetry.StabilityWindow
	checkpoints *store.Checkpoints
	now         func() time.Time
}

func NewRampController(actuator *feed.Actuator, repository *telemetry.Repository, window telemetry.StabilityWindow, checkpoints *store.Checkpoints, now func() time.Time) *RampController {
	return &RampController{
		actuator: actuator, repository: repository, window: window, checkpoints: checkpoints, now: now,
	}
}

func (c *RampController) ApplyStep(ctx context.Context, repeater *Repeater, step RampStep, samplesAfter uint64) (Snapshot, error) {
	command := feed.VoltageCommand{
		RepeaterID: repeater.Snapshot().ID, Step: step.Number, Voltage: step.Voltage, AcceptedAt: c.now().UTC(),
	}
	if err := c.actuator.ApplyVoltage(ctx, command); err != nil {
		return repeater.Snapshot(), err
	}
	samples := telemetry.CurrentSamples(c.repository, command.RepeaterID, samplesAfter)
	result, err := c.window.Evaluate(samples)
	if err != nil {
		return repeater.Snapshot(), err
	}
	if !result.Stable {
		return repeater.Snapshot(), fmt.Errorf("current for ramp step %d is not stable", step.Number)
	}
	if err := c.checkpoints.SaveRamp(store.RampCheckpoint{
		RepeaterID: command.RepeaterID, StableStep: step.Number, UpdatedAt: c.now().UTC(),
	}); err != nil {
		return repeater.Snapshot(), err
	}
	return repeater.update(func(current *Snapshot) {
		current.StableStep = step.Number
		current.UpdatedAt = c.now().UTC()
	}), nil
}

func (c *RampController) Recover(repeater *Repeater) (Snapshot, error) {
	checkpoint, found, err := c.checkpoints.LoadRamp(repeater.Snapshot().ID)
	if err != nil || !found {
		return repeater.Snapshot(), err
	}
	return repeater.update(func(current *Snapshot) {
		current.StableStep = checkpoint.StableStep
		current.UpdatedAt = checkpoint.UpdatedAt.UTC()
	}), nil
}
