package route

import (
	"context"
	"fmt"
	"time"
)

type Drainer struct {
	delay time.Duration
}

func NewDrainer(delay time.Duration) *Drainer {
	return &Drainer{delay: delay}
}

func (d *Drainer) Drain(ctx context.Context, route *Route, at time.Time) (Snapshot, error) {
	before := route.Snapshot()
	route.update(StateDraining, before.ActivePath, before.TrafficBPS, at)
	if d.delay > 0 {
		timer := time.NewTimer(d.delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return route.Snapshot(), fmt.Errorf("route drain interrupted: %w", ctx.Err())
		case <-timer.C:
		}
	}
	return route.update(StateDraining, before.ActivePath, 0, at.Add(d.delay)), nil
}

func (d *Drainer) Delay() time.Duration {
	return d.delay
}
