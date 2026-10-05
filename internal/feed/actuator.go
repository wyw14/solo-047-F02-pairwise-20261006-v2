package feed

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type VoltageCommand struct {
	RepeaterID string    `json:"repeater_id"`
	Step       int       `json:"step"`
	Voltage    float64   `json:"voltage"`
	AcceptedAt time.Time `json:"accepted_at"`
}

type Actuator struct {
	mu       sync.Mutex
	commands []VoltageCommand
}

func NewActuator() *Actuator {
	return &Actuator{}
}

func (a *Actuator) ApplyVoltage(ctx context.Context, command VoltageCommand) error {
	if command.RepeaterID == "" || command.Step < 1 || command.Voltage <= 0 {
		return fmt.Errorf("invalid repeater voltage command")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.commands = append(a.commands, command)
	return nil
}

func (a *Actuator) Commands() []VoltageCommand {
	a.mu.Lock()
	defer a.mu.Unlock()
	result := make([]VoltageCommand, len(a.commands))
	copy(result, a.commands)
	return result
}
