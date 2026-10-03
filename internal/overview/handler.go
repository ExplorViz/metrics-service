package overview

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
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
	mux.HandleFunc("GET /v3/landscapes/{landscapeToken}/metrics", h.getLandscapeMetrics)
}

func (h *Handler) getLandscapeMetrics(w http.ResponseWriter, r *http.Request) {
	lt := r.PathValue("landscapeToken")
	if lt == "" {
		http.Error(w, "Missing or invalid landscape token in path parameter", http.StatusBadRequest)
		return
	}

	query := r.URL.Query()
	params := MetricsSearchParams{
		SearchString:           strOrNil(query.Get("searchString")),
		IncludeDescription:     query.Get("includeDescription") == "true",
		IncludeAttributeKeys:   query.Get("includeAttributeKeys") == "true",
		IncludeAttributeValues: query.Get("includeAttributeValues") == "true",
		PointKind:              strOrNil(query.Get("pointKind")),
		ServiceName:            strOrNil(query.Get("serviceName")),
		TelemetryKey:           strOrNil(query.Get("telemetryKey")),
		Limit:                  parseUintOrNil(query.Get("limit")),
	}

	switch sortBy := query.Get("sortBy"); sortBy {
	case "", "nameAsc":
		params.SortBy = SortNameAsc
	case "nameDesc":
		params.SortBy = SortNameDesc
	default:
		http.Error(w, fmt.Sprintf(`Invalid value %s for parameter "sortBy"`, sortBy), http.StatusBadRequest)
		return
	}

	cursorName := query.Get("cursorName")
	cursorPointKind := query.Get("cursorPointKind")

	if cursorName != "" && cursorPointKind != "" {
		params.Cursor = &MetricsSearchCursor{
			Name:      cursorName,
			PointKind: cursorPointKind,
		}
	} else if cursorName != "" || cursorPointKind != "" {
		http.Error(w, "Provided some, but not all cursor values", http.StatusBadRequest)
		return
	}

	metrics, err := h.repo.findMetricsOverview(r.Context(), lt, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func strOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func parseUintOrNil(s string) *uint64 {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return nil
	}
	return &v
}
