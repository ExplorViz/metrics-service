package overview

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type Repository struct {
	Conn driver.Conn
}

type MetricsSearchParams struct {
	SearchString           *string
	IncludeDescription     bool
	IncludeAttributeKeys   bool
	IncludeAttributeValues bool
	PointKind              *string
	ServiceName            *string
	TelemetryKey           *string

	SortBy MetricsSorting

	// Limits the number of retrieved rows, for use with paginiation.
	Limit *uint64

	// Specifies the last received metric from the previous request.
	Cursor *MetricsSearchCursor
}

type MetricsSorting int

const (
	SortNameAsc MetricsSorting = iota
	SortNameDesc
)

// A MetricsSearchCursor specifies the last seen metric from a prior request.
// This can be used for pagination.
type MetricsSearchCursor struct {
	Name      string
	PointKind string
}

// findMetricsOverview searches the database for available metrics associated with the given landscape.
// The search space can be restricted using a variety of filter options (see [MetricsSearchParams]).
func (r *Repository) findMetricsOverview(ctx context.Context, landscapeToken string, params MetricsSearchParams) ([]MetricOverview, error) {
	queryParams := make([]any, 0, 7)

	// Conditions that are applied to each individual table before they are unioned
	var preconditions strings.Builder

	// Conditions that are applied to the unioned results of all the tables
	var postconditions strings.Builder

	preconditions.WriteString("ExplorvizTokenId = @landscapeToken")
	queryParams = append(queryParams, clickhouse.Named("landscapeToken", landscapeToken))

	if params.SearchString != nil {
		preconditions.WriteString(" AND (hasAllTokens(MetricName, @searchString)")
		queryParams = append(queryParams, clickhouse.Named("searchString", *params.SearchString))

		if params.IncludeDescription {
			preconditions.WriteString(` OR hasAllTokens(MetricDescription, @searchString)`)
		}

		if params.IncludeAttributeKeys {
			preconditions.WriteString(`
				OR (
					hasAllTokens(mapKeys(Attributes), @searchString)
					OR hasAllTokens(mapKeys(ScopeAttributes), @searchString)
					OR hasAllTokens(mapKeys(ResourceAttributes), @searchString)
				)`)
		}

		if params.IncludeAttributeValues {
			preconditions.WriteString(`
				OR (
					hasAllTokens(mapValues(Attributes), @searchString)
					OR hasAllTokens(mapValues(ScopeAttributes), @searchString)
					OR hasAllTokens(mapValues(ResourceAttributes), @searchString)
				)`)
		}

		preconditions.WriteString(")")
	}

	if params.ServiceName != nil {
		postconditions.WriteString("has(Services, @serviceName)")
		queryParams = append(queryParams, clickhouse.Named("serviceName", *params.ServiceName))
	}

	if params.TelemetryKey != nil {
		if postconditions.Len() > 0 {
			postconditions.WriteString(" AND ")
		}
		postconditions.WriteString("has(Entities, @telemetryKey)")
		queryParams = append(queryParams, clickhouse.Named("telemetryKey", *params.TelemetryKey))
	}

	if params.Cursor != nil {
		if postconditions.Len() > 0 {
			postconditions.WriteString(" AND ")
		}

		switch params.SortBy {
		case SortNameAsc:
			postconditions.WriteString(`(Name > @cursorName OR (Name = @cursorName AND PointKind > @cursorPointKind))`)
		case SortNameDesc:
			postconditions.WriteString(`(Name < @cursorName OR (Name = @cursorName AND PointKind > @cursorPointKind))`)
		default:
			slog.Error("received invalid sorting order", "SortBy", params.SortBy)
		}

		queryParams = append(queryParams, clickhouse.Named("cursorName", params.Cursor.Name))
		queryParams = append(queryParams, clickhouse.Named("cursorPointKind", params.Cursor.PointKind))
	}

	ordering := ""
	switch params.SortBy {
	case SortNameAsc:
		ordering += " ORDER BY Name ASC, PointKind ASC"
	case SortNameDesc:
		ordering += " ORDER BY Name DESC, PointKind ASC"
	default:
		slog.Error("received invalid sorting order", "SortBy", params.SortBy)
	}

	queryLimit := ""
	if params.Limit != nil {
		queryLimit = " LIMIT @limit"
		queryParams = append(queryParams, clickhouse.Named("limit", *params.Limit))
	}

	preconditionsStr := preconditions.String()
	postconditionsStr := ""
	if postconditions.Len() > 0 {
		postconditionsStr = "WHERE " + postconditions.String()
	}

	tableQueries := make([]string, 0, 4)
	if params.PointKind != nil {
		switch *params.PointKind {
		case "sum":
			tableQueries = append(tableQueries, buildMetricsQuery("otel_metrics_sum", "sum", preconditionsStr))
		case "gauge":
			tableQueries = append(tableQueries, buildMetricsQuery("otel_metrics_gauge", "gauge", preconditionsStr))
		case "histogram":
			tableQueries = append(tableQueries, buildMetricsQuery("otel_metrics_histogram", "histogram", preconditionsStr))
		case "exp_histogram":
			tableQueries = append(tableQueries, buildMetricsQuery("otel_metrics_exponential_histogram", "exp_histogram", preconditionsStr))
		default:
			return []MetricOverview{}, fmt.Errorf("received invalid point kind %s", *params.PointKind)
		}
	} else {
		tableQueries = append(
			tableQueries,
			buildMetricsQuery("otel_metrics_sum", "sum", preconditionsStr),
			buildMetricsQuery("otel_metrics_gauge", "gauge", preconditionsStr),
			buildMetricsQuery("otel_metrics_histogram", "histogram", preconditionsStr),
			buildMetricsQuery("otel_metrics_exponential_histogram", "exp_histogram", preconditionsStr),
		)
	}

	query := `
		WITH metrics_overview AS (
			` + strings.Join(tableQueries, " UNION ALL ") + `
		)
		SELECT
			Name,
			Description,
			Unit,
			Services,
			Entities,
			PointKind
		FROM metrics_overview
		` + postconditionsStr + ordering + queryLimit

	metrics := []MetricOverview{}

	err := r.Conn.Select(ctx, &metrics, query, queryParams...)
	if err != nil {
		return []MetricOverview{}, err
	}

	return metrics, nil
}

func buildMetricsQuery(tableName string, pointKind string, conditionsStr string) string {
	return `
		SELECT
			MetricName AS Name,
			topK(1)(MetricDescription)[1] AS Description,
			topK(1)(MetricUnit)[1] AS Unit,
			groupUniqArray(ServiceName) AS Services,
			groupUniqArrayIf(ExplorvizTelemetryKey, ExplorvizTelemetryKey <> '') AS Entities,
			'` + pointKind + `' AS PointKind
		FROM ` + tableName + `
		WHERE
			ServiceName <> '' AND ` + conditionsStr + `
		GROUP BY MetricName
	`
}
