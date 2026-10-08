package catalog

// A MetricsCatalogItem contains surface-level information about an available runtime metric.
type MetricsCatalogItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Unit        string `json:"unit"`

	// The kind of datapoint this metric consists of as per the OTel metrics data model.
	// See the [OTel documentation] for details on what point kinds exist.
	//
	// [OTel documentation]: https://opentelemetry.io/docs/specs/otel/metrics/data-model/#point-kinds
	PointKind string `json:"pointKind"`

	// List of services which have time series for this metric.
	Services []string `json:"services"`

	// List of telemetry keys for entities which have time series for this metric.
	Entities []string `json:"entities"`
}
