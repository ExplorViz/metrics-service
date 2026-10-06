package values

import "encoding/json"

// A GaugeMetricValues represents the values of a particular metric
// of "gauge" point kind for some time slice and window size.
type GaugeMetricValues struct {
	Points         []GaugeMetricPoint `json:"points"`
	StartUnixMilli uint32             `json:"startUnixMilli"`
	EndUnixMilli   uint32             `json:"endUnixMilli"`
}

func (v GaugeMetricValues) MarshalJSON() ([]byte, error) {
	type Alias GaugeMetricValues

	return json.Marshal(struct {
		Alias
		PointKind string `json:"pointKind"`
	}{
		Alias:     (Alias)(v),
		PointKind: "gauge",
	})
}

type GaugeMetricPoint struct {
	Value         float64 `json:"value"`
	TimeUnixMilli uint32  `json:"timeUnixMilli"`
}

type HistogramMetricValues struct {
	Points []HistogramMetricPoint
	Bounds []float64
}

type HistogramMetricPoint struct {
	BucketCounts []uint64
}
