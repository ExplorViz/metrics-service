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

// findSumMetricValues searches the database for values of a metric with point kind "sum" in the given landscape.
// The metric and window over which to collect values is specified by the provided [MetricValuesRequest].
func (r *Repository) findSumMetricValues(ctx context.Context, landscapeToken string, req MetricValuesRequest) (SumMetricValues, error) {
	queryParams := make([]any, 0, 4)

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

	var windowSize string
	var binningSize string
	switch req.Window {
	case Window1H:
		windowSize = "1 HOUR"
		binningSize = "15 SECOND"
	case Window4H:
		windowSize = "4 HOUR"
		binningSize = "1 MINUTE"
	case Window24H:
		windowSize = "24 HOUR"
		binningSize = "6 MINUTE"
	default:
		slog.Error("received invalid window size, defaulting to 1h", "Window", req.Window)
		windowSize = "1 HOUR"
		binningSize = "15 SECOND"
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
			FROM otel_metrics_sum
			WHERE ` + conditions.String() + `
		) AS latest_timestamp
		SELECT
			` + aggregationFunc + `(TimeSeriesValue) AS Value,
			toUnixTimestamp64Milli(toDateTime64(TimeBinned, 3)) AS TimeUnixMilli
		FROM (
			SELECT
				toStartOfInterval(TimeUnix, INTERVAL ` + binningSize + `) AS TimeBinned,
				argMax(Value, TimeUnix) AS TimeSeriesValue // Choose the latest value within each bin
			FROM otel_metrics_sum
			WHERE
				TimeUnix >= latest_timestamp - INTERVAL ` + windowSize + `
				AND TimeUnix <= latest_timestamp
				AND ` + conditions.String() + `
			// Group by time bin and time series identity, since points from different
			// time series (e.g. for different service instances) should not be conflated
			GROUP BY TimeBinned, ScopeName, ScopeVersion, ResourceAttributes, Attributes
		)
		GROUP BY TimeUnixMilli
		ORDER BY TimeUnixMilli`

	points := []SumMetricPoint{}

	err := r.Conn.Select(ctx, &points, query, queryParams...)
	if err != nil {
		return SumMetricValues{}, err
	}

	var start int64 = 0
	var end int64 = 0
	if len(points) > 0 {
		start = points[0].TimeUnixMilli
		end = points[len(points)-1].TimeUnixMilli
	}

	metricVals := SumMetricValues{
		Points:         points,
		StartUnixMilli: start,
		EndUnixMilli:   end,
	}

	return metricVals, nil
}

// findGaugeMetricValues searches the database for values of a metric with point kind "gauge" in the given landscape.
// The metric and window over which to collect values is specified by the provided [MetricValuesRequest].
func (r *Repository) findGaugeMetricValues(ctx context.Context, landscapeToken string, req MetricValuesRequest) (GaugeMetricValues, error) {
	queryParams := make([]any, 0, 4)

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

	var windowSize string
	var binningSize string
	switch req.Window {
	case Window1H:
		windowSize = "1 HOUR"
		binningSize = "15 SECOND"
	case Window4H:
		windowSize = "4 HOUR"
		binningSize = "1 MINUTE"
	case Window24H:
		windowSize = "24 HOUR"
		binningSize = "6 MINUTE"
	default:
		slog.Error("received invalid window size, defaulting to 1h", "Window", req.Window)
		windowSize = "1 HOUR"
		binningSize = "15 SECOND"
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
			toUnixTimestamp64Milli(toDateTime64(toStartOfInterval(TimeUnix, INTERVAL ` + binningSize + `), 3)) AS TimeUnixMilli
		FROM otel_metrics_gauge
		WHERE
			TimeUnix >= latest_timestamp - INTERVAL ` + windowSize + `
			AND TimeUnix <= latest_timestamp
			AND ` + conditions.String() + `
		GROUP BY TimeUnixMilli
		ORDER BY TimeUnixMilli`

	points := []GaugeMetricPoint{}

	err := r.Conn.Select(ctx, &points, query, queryParams...)
	if err != nil {
		return GaugeMetricValues{}, err
	}

	var start int64 = 0
	var end int64 = 0
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

// findHistogramMetricValues searches the database for values of a metric with point kind "histogram" in the given landscape.
// The metric and window over which to collect values is specified by the provided [MetricValuesRequest].
func (r *Repository) findHistogramMetricValue(ctx context.Context, landscapeToken string, req MetricValuesRequest) (HistogramMetricValues, error) {
	queryParams := make([]any, 0, 4)

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

	type HistogramValuesResult struct {
		Bounds     []float64
		Counts     []uint64
		TotalSum   float64
		TotalCount uint64
	}

	query := `
		WITH latest_points_per_series AS (
			SELECT
				argMax(BucketCounts, TimeUnix) AS BucketCounts,
				argMax(ExplicitBounds, TimeUnix) AS ExplicitBounds,
				argMax(Count, TimeUnix) AS Count,
				argMax(Sum, TimeUnix) AS Sum
			FROM otel_metrics_histogram
			WHERE ` + conditions.String() + `
			GROUP BY ScopeName, ScopeVersion, ResourceAttributes, Attributes
		)

		SELECT
			sumForEach(BucketCounts) AS Counts,
			any(ExplicitBounds) AS Bounds,
			sum(Count) AS TotalCount,
			sum(Sum) AS TotalSum
		FROM latest_points_per_series;`

	res := HistogramValuesResult{}
	if err := r.Conn.QueryRow(ctx, query, queryParams...).ScanStruct(&res); err != nil {
		return HistogramMetricValues{}, err
	}

	vals := HistogramMetricValues{
		Sum:   res.TotalSum,
		Count: res.TotalCount,
	}

	vals.Points = make([]HistogramMetricPoint, 0, len(res.Counts))

	for i, bound := range res.Bounds {
		vals.Points = append(vals.Points, HistogramMetricPoint{
			Bound: &bound,
			Count: res.Counts[i],
		})
	}
	vals.Points = append(vals.Points, HistogramMetricPoint{
		Count: res.Counts[len(res.Counts)-1],
	})

	return vals, nil
}
