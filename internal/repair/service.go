package repair

import (
	"sync"

	"github.com/wyw14/cry-103/internal/feed"
	"github.com/wyw14/cry-103/internal/route"
	"github.com/wyw14/cry-103/internal/span"
)

type Assets struct {
	mu       sync.RWMutex
	spans    map[string]*span.Span
	routes   map[string]*route.Route
	circuits map[string]*feed.Circuit
}

func NewAssets() *Assets {
	return &Assets{
		spans: make(map[string]*span.Span), routes: make(map[string]*route.Route), circuits: make(map[string]*feed.Circuit),
	}
}

func (a *Assets) AddSpan(target *span.Span) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.spans[target.Snapshot().ID] = target
}

func (a *Assets) AddRoute(target *route.Route) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.routes[target.Snapshot().ID] = target
}

func (a *Assets) AddCircuit(target *feed.Circuit) {
	a.mu.Lock()
	defer a.mu.Unlock()
	snapshot := target.Snapshot()
	a.circuits[snapshot.StationID+"/"+snapshot.ID] = target
}

func (a *Assets) Span(id string) (*span.Span, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	value, exists := a.spans[id]
	return value, exists
}

func (a *Assets) Route(id string) (*route.Route, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	value, exists := a.routes[id]
	return value, exists
}

func (a *Assets) Circuit(stationID, circuitID string) (*feed.Circuit, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	value, exists := a.circuits[stationID+"/"+circuitID]
	return value, exists
}
