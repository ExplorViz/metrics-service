package values

import "encoding/json"

type SumMetricPoint struct {
	Value         float64 `json:"value"`
	TimeUnixMilli int64   `json:"timeUnixMilli"`
}

// A SumMetricValues represents the values of a particular metric
// of "sum" point kind for some time slice and window size.
type SumMetricValues struct {
	Points         []SumMetricPoint `json:"points"`
	StartUnixMilli int64            `json:"startUnixMilli"`
	EndUnixMilli   int64            `json:"endUnixMilli"`
}

func (v SumMetricValues) MarshalJSON() ([]byte, error) {
	type Alias SumMetricValues

	return json.Marshal(struct {
		Alias
		PointKind string `json:"pointKind"`
	}{
		Alias:     (Alias)(v),
		PointKind: "sum",
	})
}

type GaugeMetricPoint struct {
	Value         float64 `json:"value"`
	TimeUnixMilli int64   `json:"timeUnixMilli"`
}

// A GaugeMetricValues represents the values of a particular metric
// of "gauge" point kind for some time slice and window size.
type GaugeMetricValues struct {
	Points         []GaugeMetricPoint `json:"points"`
	StartUnixMilli int64              `json:"startUnixMilli"`
	EndUnixMilli   int64              `json:"endUnixMilli"`
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

type HistogramMetricValues struct {
	Points []HistogramMetricPoint
	Bounds []float64
}

type HistogramMetricPoint struct {
	BucketCounts []uint64
}
