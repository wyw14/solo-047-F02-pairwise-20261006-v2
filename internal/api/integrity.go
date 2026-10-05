package api

import (
	"fmt"
	"sort"

	"github.com/wyw14/cry-103/internal/route"
	"github.com/wyw14/cry-103/internal/span"
	"github.com/wyw14/cry-103/internal/topology"
)

type IntegrityFinding struct {
	Code     string `json:"code"`
	AssetID  string `json:"asset_id"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

func auditIntegrity(spans []span.Snapshot, routes []route.Snapshot, topologies []topology.Snapshot) []IntegrityFinding {
	spanByID := make(map[string]span.Snapshot, len(spans))
	for _, item := range spans {
		spanByID[item.ID] = item
	}
	topologyBySpan := make(map[string]topology.Snapshot, len(topologies))
	for _, item := range topologies {
		topologyBySpan[item.SpanID] = item
	}
	var findings []IntegrityFinding
	for _, path := range routes {
		cable, spanExists := spanByID[path.SpanID]
		crossConnect, topologyExists := topologyBySpan[path.SpanID]
		if !spanExists {
			findings = append(findings, IntegrityFinding{
				Code: "route-without-span", AssetID: path.ID, Severity: "critical",
				Message: fmt.Sprintf("route %s references missing span %s", path.ID, path.SpanID),
			})
			continue
		}
		if !topologyExists {
			findings = append(findings, IntegrityFinding{
				Code: "route-without-topology", AssetID: path.ID, Severity: "critical",
				Message: fmt.Sprintf("route %s has no cross-connect topology", path.ID),
			})
			continue
		}
		if !crossConnect.PrimaryOpen && !crossConnect.ProtectOpen {
			findings = append(findings, IntegrityFinding{
				Code: "dual-active-cross-connect", AssetID: path.ID, Severity: "critical",
				Message: "primary and protection cross-connects are both closed",
			})
		}
		if path.ActivePath != crossConnect.ActivePath {
			findings = append(findings, IntegrityFinding{
				Code: "route-topology-disagreement", AssetID: path.ID, Severity: "warning",
				Message: fmt.Sprintf("route reports %s while topology reports %s", path.ActivePath, crossConnect.ActivePath),
			})
		}
		if cable.State == span.StateRecovered && path.ActivePath != topology.PathPrimary {
			findings = append(findings, IntegrityFinding{
				Code: "recovered-span-on-protection", AssetID: cable.ID, Severity: "warning",
				Message: "recovered span still carries traffic on the protection path",
			})
		}
		if path.TrafficBPS > 0 && ((path.ActivePath == topology.PathPrimary && crossConnect.PrimaryOpen) ||
			(path.ActivePath == topology.PathProtection && crossConnect.ProtectOpen)) {
			findings = append(findings, IntegrityFinding{
				Code: "traffic-on-open-cross-connect", AssetID: path.ID, Severity: "critical",
				Message: fmt.Sprintf("route reports traffic on open %s cross-connect", path.ActivePath),
			})
		}
	}
	for _, cable := range spans {
		if _, exists := topologyBySpan[cable.ID]; !exists {
			findings = append(findings, IntegrityFinding{
				Code: "span-without-topology", AssetID: cable.ID, Severity: "critical",
				Message: "span has no registered physical topology",
			})
		}
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Severity == findings[j].Severity {
			if findings[i].AssetID == findings[j].AssetID {
				return findings[i].Code < findings[j].Code
			}
			return findings[i].AssetID < findings[j].AssetID
		}
		return findings[i].Severity < findings[j].Severity
	})
	return findings
}
