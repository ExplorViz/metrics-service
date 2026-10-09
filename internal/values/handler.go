package values

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Handler struct {
	repo Repository
}

func NewHandler(r Repository) Handler {
	return Handler{
		repo: r,
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v3/landscapes/{landscapeToken}/metric", h.getMetricValues)
}

func (h *Handler) getMetricValues(w http.ResponseWriter, r *http.Request) {
	lt := r.PathValue("landscapeToken")
	if lt == "" {
		http.Error(w, "Missing or invalid landscape token in path parameter", http.StatusBadRequest)
		return
	}

	query := r.URL.Query()
	params := MetricValuesRequest{
		Name:         query.Get("name"),
		ServiceName:  query.Get("service"),
		TelemetryKey: query.Get("telemetryKey"),
	}

	switch window := query.Get("window"); window {
	case "", "1h":
		params.Window = Window1H
	case "4h":
		params.Window = Window4H
	case "24h":
		params.Window = Window24H
	default:
		http.Error(w, fmt.Sprintf(`Invalid value %s for parameter "window"`, window), http.StatusBadRequest)
		return
	}

	switch aggregation := query.Get("aggregation"); aggregation {
	case "avg":
		params.Aggregation = AggregationAverage
	case "sum":
		params.Aggregation = AggregationSum
	case "min":
		params.Aggregation = AggregationMinimum
	case "max":
		params.Aggregation = AggregationMaximum
	default:
		http.Error(w, fmt.Sprintf(`Invalid value %s for parameter "aggregation"`, aggregation), http.StatusBadRequest)
		return
	}

	var vals any
	var err error

	switch pointKind := query.Get("pointKind"); pointKind {
	case "sum":
		vals, err = h.repo.findSumMetricValues(r.Context(), lt, params)
	case "gauge":
		vals, err = h.repo.findGaugeMetricValues(r.Context(), lt, params)
	case "histogram":
		vals, err = h.repo.findHistogramMetricValue(r.Context(), lt, params)
	default:
		http.Error(w, fmt.Sprintf(`Invalid value %s for parameter "pointKind"`, pointKind), http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(vals); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
