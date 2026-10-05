package span

import (
	"errors"
	"math"
	"sort"
)

type Reflection struct {
	DistanceKM float64 `json:"distance_km"`
	LossDB     float64 `json:"loss_db"`
	Direction  string  `json:"direction"`
}

type FaultLocation struct {
	DistanceKM float64      `json:"distance_km"`
	Confidence float64      `json:"confidence"`
	Evidence   []Reflection `json:"evidence"`
}

func Locate(lengthKM float64, reflections []Reflection) (FaultLocation, error) {
	if lengthKM <= 0 || len(reflections) == 0 {
		return FaultLocation{}, errors.New("span length and reflections are required")
	}
	valid := make([]Reflection, 0, len(reflections))
	for _, item := range reflections {
		if item.DistanceKM >= 0 && item.DistanceKM <= lengthKM && item.LossDB > 0 {
			valid = append(valid, item)
		}
	}
	if len(valid) == 0 {
		return FaultLocation{}, errors.New("no reflection is inside span bounds")
	}
	sort.Slice(valid, func(i, j int) bool { return valid[i].LossDB > valid[j].LossDB })
	primary := valid[0]
	confidence := math.Min(1, primary.LossDB/8)
	if len(valid) > 1 && math.Abs(valid[0].DistanceKM-valid[1].DistanceKM) < 2 {
		confidence = math.Min(1, confidence+0.15)
	}
	return FaultLocation{DistanceKM: primary.DistanceKM, Confidence: confidence, Evidence: valid}, nil
}
