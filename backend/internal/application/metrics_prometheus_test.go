package application

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestPrometheusMetricsReaderReadUsesFallbacksAndEmptySeries(t *testing.T) {
	now := time.Date(2026, 4, 2, 4, 0, 0, 0, time.UTC)
	start := now.Add(-55 * time.Minute)
	end := now

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")

		response := map[string]any{
			"status": "success",
			"data": map[string]any{
				"resultType": "matrix",
				"result":     []any{},
			},
		}

		switch query {
		case renderPrometheusQuery(prometheusMetricDefinitions[0].Queries[0], testRecord()):
			response["data"] = matrixResponse(start, end, 5*time.Minute, 120)
		case renderPrometheusQuery(prometheusMetricDefinitions[2].Queries[1], testRecord()):
			response["data"] = matrixResponse(start, end, 5*time.Minute, 250)
		case renderPrometheusQuery(prometheusMetricDefinitions[3].Queries[0], testRecord()):
			response["data"] = matrixResponse(start, end, 5*time.Minute, 0.5)
		}

		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	reader := PrometheusMetricsReader{
		BaseURL: server.URL,
		Client:  server.Client(),
		Range:   time.Hour,
		Step:    5 * time.Minute,
		Now: func() time.Time {
			return now
		},
	}

	metrics, err := reader.Read(context.Background(), testRecord())
	if err != nil {
		t.Fatalf("read metrics: %v", err)
	}

	if len(metrics) != 5 {
		t.Fatalf("expected 5 metric series, got %d", len(metrics))
	}

	if metrics[0].Points[0].Value == nil || *metrics[0].Points[0].Value != 120 {
		t.Fatalf("expected request_rate to contain data, got %#v", metrics[0].Points[0].Value)
	}

	if metrics[1].Points[0].Value != nil {
		t.Fatalf("expected error_rate to be empty when prometheus has no data")
	}

	if metrics[2].Points[0].Value == nil || *metrics[2].Points[0].Value != 250 {
		t.Fatalf("expected latency fallback query to return data, got %#v", metrics[2].Points[0].Value)
	}

	if metrics[3].Points[0].Value == nil || *metrics[3].Points[0].Value != 0.5 {
		t.Fatalf("expected cpu_usage to contain data, got %#v", metrics[3].Points[0].Value)
	}

	if metrics[4].Points[0].Value != nil {
		t.Fatalf("expected memory_usage to be empty when prometheus has no data")
	}
}

func TestPrometheusMetricsReaderBuildsExpectedPointCount(t *testing.T) {
	reader := PrometheusMetricsReader{
		Range: time.Hour,
		Step:  5 * time.Minute,
		Now: func() time.Time {
			return time.Date(2026, 4, 2, 4, 0, 0, 0, time.UTC)
		},
	}

	points := buildEmptyMetricPoints(
		reader.Now().UTC().Add(-55*time.Minute),
		reader.Now().UTC(),
		reader.Step,
	)
	if len(points) != 12 {
		t.Fatalf("expected 12 points, got %d", len(points))
	}
}

func matrixResponse(start time.Time, end time.Time, step time.Duration, value float64) map[string]any {
	values := make([][]any, 0, int(end.Sub(start)/step)+1)
	for current := start; !current.After(end); current = current.Add(step) {
		values = append(values, []any{float64(current.Unix()), formatFloat(value)})
	}

	return map[string]any{
		"resultType": "matrix",
		"result": []map[string]any{
			{
				"metric": map[string]string{},
				"values": values,
			},
		},
	}
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func testRecord() Record {
	return Record{
		Name:      "vault-smoke-20260402-0304",
		Namespace: "project-a",
	}
}
