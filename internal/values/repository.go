package values

import (
	"context"
	"log/slog"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type Repository struct {
	Conn driver.Conn
}

type MetricValuesRequest struct {
	Name         string
	ServiceName  string
	TelemetryKey string
	Window       MetricsWindow
	Aggregation  MetricsAggregation
}

type MetricsWindow int

const (
	Window1H MetricsWindow = iota
	Window4H
	Window24H
)

type MetricsAggregation int

const (
	AggregationAverage MetricsAggregation = iota
	AggregationSum
	AggregationMinimum
	AggregationMaximum
)

// findGaugeMetricValues searches the database for values of a metric with point kind "gauge" in the given landscape.
// The metric and window over which to collect values is specified by [MetricValuesRequest].
func (r *Repository) findGaugeMetricValues(ctx context.Context, landscapeToken string, req MetricValuesRequest) (GaugeMetricValues, error) {
	queryParams := make([]any, 0, 7)

	var conditions strings.Builder

	conditions.WriteString("ExplorvizTokenId = @landscapeToken AND MetricName = @metricName")
	queryParams = append(
		queryParams,
		clickhouse.Named("landscapeToken", landscapeToken),
		clickhouse.Named("metricName", req.Name),
	)

	if req.ServiceName != "" {
		conditions.WriteString(" AND firstNonDefault(ExplorvizServiceName, ServiceName) = @serviceName")
		queryParams = append(queryParams, clickhouse.Named("serviceName", req.ServiceName))
	}

	if req.TelemetryKey != "" {
		conditions.WriteString(" AND ExplorvizTelemetryKey = @telemetryKey")
		queryParams = append(queryParams, clickhouse.Named("telemetryKey", req.TelemetryKey))
	}

	var interval string
	switch req.Window {
	case Window1H:
		interval = "1 HOUR"
	case Window4H:
		interval = "4 HOUR"
	case Window24H:
		interval = "24 HOUR"
	default:
		slog.Error("received invalid window size, defaulting to 1h", "Window", req.Window)
		interval = "1 HOUR"
	}

	var aggregationFunc string
	switch req.Aggregation {
	case AggregationAverage:
		aggregationFunc = "avg"
	case AggregationSum:
		aggregationFunc = "sum"
	case AggregationMinimum:
		aggregationFunc = "min"
	case AggregationMaximum:
		aggregationFunc = "max"
	}

	query := `
		WITH (
			SELECT max(TimeUnix)
			FROM otel_metrics_gauge
			WHERE ` + conditions.String() + `
		) AS latest_timestamp
		SELECT
			` + aggregationFunc + `(Value) AS Value,
			toUnixTimestamp(toStartOfFiveMinutes(TimeUnix)) AS TimeUnixMilli
		FROM otel_metrics_gauge
		WHERE
			TimeUnix >= latest_timestamp - INTERVAL ` + interval + `
			AND TimeUnix <= latest_timestamp
			AND ` + conditions.String() + `
		GROUP BY TimeUnixMilli
		ORDER BY TimeUnixMilli`

	points := []GaugeMetricPoint{}

	err := r.Conn.Select(ctx, &points, query, queryParams...)
	if err != nil {
		return GaugeMetricValues{}, err
	}

	var start uint32 = 0
	var end uint32 = 0
	if len(points) > 0 {
		start = points[0].TimeUnixMilli
		end = points[len(points)-1].TimeUnixMilli
	}

	metricVals := GaugeMetricValues{
		Points:         points,
		StartUnixMilli: start,
		EndUnixMilli:   end,
	}

	return metricVals, nil
}
