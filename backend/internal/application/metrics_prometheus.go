package application

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type PrometheusMetricsReader struct {
	BaseURL string
	Client  *http.Client
	Range   time.Duration
	Step    time.Duration
	Now     func() time.Time
}

type prometheusMetricDefinition struct {
	Key     string
	Label   string
	Unit    string
	Queries []string
}

type prometheusQueryResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Data   struct {
		ResultType string                     `json:"resultType"`
		Result     []prometheusMatrixResponse `json:"result"`
	} `json:"data"`
}

type prometheusMatrixResponse struct {
	Values [][]any `json:"values"`
}

func (r PrometheusMetricsReader) Read(ctx context.Context, record Record) ([]MetricSeries, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	now := r.now().UTC()
	step := r.metricStep()
	window := r.metricRange()
	end := now.Truncate(step)
	start := end.Add(-window).Add(step)
	if start.After(end) {
		start = end
	}

	metrics := make([]MetricSeries, 0, len(prometheusMetricDefinitions))
	for _, definition := range prometheusMetricDefinitions {
		points, err := r.queryMetricPoints(ctx, record, definition.Queries, start, end, step)
		if err != nil {
			return nil, err
		}

		metrics = append(metrics, MetricSeries{
			Key:    definition.Key,
			Label:  definition.Label,
			Unit:   definition.Unit,
			Points: points,
		})
	}

	return metrics, nil
}

var prometheusMetricDefinitions = []prometheusMetricDefinition{
	{
		Key:   "request_rate",
		Label: "Requests",
		Unit:  "rpm",
		Queries: []string{
			`sum(rate(istio_requests_total{reporter="destination",destination_workload_namespace="{{namespace}}",destination_app="{{appName}}"}[5m])) * 60`,
			`sum(rate(http_server_requests_seconds_count{namespace="{{namespace}}",application="{{appName}}"}[5m])) * 60`,
		},
	},
	{
		Key:   "error_rate",
		Label: "Error Rate",
		Unit:  "%",
		Queries: []string{
			`100 * sum(rate(istio_requests_total{reporter="destination",destination_workload_namespace="{{namespace}}",destination_app="{{appName}}",response_code=~"5.."}[5m])) / clamp_min(sum(rate(istio_requests_total{reporter="destination",destination_workload_namespace="{{namespace}}",destination_app="{{appName}}"}[5m])), 0.001)`,
			`100 * sum(rate(http_server_requests_seconds_count{namespace="{{namespace}}",application="{{appName}}",status=~"5.."}[5m])) / clamp_min(sum(rate(http_server_requests_seconds_count{namespace="{{namespace}}",application="{{appName}}"}[5m])), 0.001)`,
		},
	},
	{
		Key:   "latency_p95",
		Label: "P95 Latency",
		Unit:  "ms",
		Queries: []string{
			`histogram_quantile(0.95, sum(rate(istio_request_duration_milliseconds_bucket{reporter="destination",destination_workload_namespace="{{namespace}}",destination_app="{{appName}}"}[5m])) by (le))`,
			`histogram_quantile(0.95, sum(rate(istio_request_duration_seconds_bucket{reporter="destination",destination_workload_namespace="{{namespace}}",destination_app="{{appName}}"}[5m])) by (le)) * 1000`,
			`histogram_quantile(0.95, sum(rate(http_server_requests_seconds_bucket{namespace="{{namespace}}",application="{{appName}}"}[5m])) by (le)) * 1000`,
		},
	},
	{
		Key:   "cpu_usage",
		Label: "CPU Usage",
		Unit:  "cores",
		Queries: []string{
			`sum(rate(container_cpu_usage_seconds_total{namespace="{{namespace}}",pod=~"{{podRegex}}",container!="",container!="POD"}[5m]))`,
		},
	},
	{
		Key:   "memory_usage",
		Label: "Memory Usage",
		Unit:  "MiB",
		Queries: []string{
			`sum(container_memory_working_set_bytes{namespace="{{namespace}}",pod=~"{{podRegex}}",container!="",container!="POD"}) / 1024 / 1024`,
		},
	},
}

func (r PrometheusMetricsReader) queryMetricPoints(
	ctx context.Context,
	record Record,
	queries []string,
	start time.Time,
	end time.Time,
	step time.Duration,
) ([]MetricPoint, error) {
	empty := buildEmptyMetricPoints(start, end, step)
	for _, query := range queries {
		values, found, err := r.queryRange(ctx, renderPrometheusQuery(query, record), start, end, step)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		return buildMetricPointsFromValues(start, end, step, values), nil
	}

	return empty, nil
}

func (r PrometheusMetricsReader) queryRange(
	ctx context.Context,
	query string,
	start time.Time,
	end time.Time,
	step time.Duration,
) (map[int64]float64, bool, error) {
	if strings.TrimSpace(r.BaseURL) == "" {
		return nil, false, fmt.Errorf("prometheus URL is required")
	}

	endpoint, err := url.Parse(strings.TrimRight(r.BaseURL, "/") + "/api/v1/query_range")
	if err != nil {
		return nil, false, fmt.Errorf("parse prometheus URL: %w", err)
	}

	params := endpoint.Query()
	params.Set("query", query)
	params.Set("start", strconv.FormatFloat(float64(start.Unix()), 'f', 0, 64))
	params.Set("end", strconv.FormatFloat(float64(end.Unix()), 'f', 0, 64))
	params.Set("step", strconv.FormatFloat(step.Seconds(), 'f', 0, 64))
	endpoint.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, false, fmt.Errorf("build prometheus request: %w", err)
	}

	resp, err := r.httpClient().Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("query prometheus: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, false, fmt.Errorf("prometheus API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var payload prometheusQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, false, fmt.Errorf("decode prometheus response: %w", err)
	}
	if payload.Status != "success" {
		if payload.Error == "" {
			payload.Error = "unknown error"
		}
		return nil, false, fmt.Errorf("prometheus query failed: %s", payload.Error)
	}

	if len(payload.Data.Result) == 0 {
		return nil, false, nil
	}

	values := make(map[int64]float64)
	for _, series := range payload.Data.Result {
		for _, rawValue := range series.Values {
			if len(rawValue) != 2 {
				continue
			}

			timestamp, ok := decodePrometheusTimestamp(rawValue[0])
			if !ok {
				continue
			}
			value, ok := decodePrometheusValue(rawValue[1])
			if !ok {
				continue
			}
			values[timestamp] += value
		}
	}

	if len(values) == 0 {
		return nil, false, nil
	}

	return values, true, nil
}

func renderPrometheusQuery(template string, record Record) string {
	podRegex := record.Name + `-[a-z0-9]+(?:-[a-z0-9]+)?`
	replacer := strings.NewReplacer(
		"{{namespace}}", record.Namespace,
		"{{appName}}", record.Name,
		"{{podRegex}}", podRegex,
	)
	return replacer.Replace(template)
}

func buildMetricPointsFromValues(
	start time.Time,
	end time.Time,
	step time.Duration,
	values map[int64]float64,
) []MetricPoint {
	points := make([]MetricPoint, 0, int(end.Sub(start)/step)+1)
	for current := start; !current.After(end); current = current.Add(step) {
		timestamp := current.Unix()
		point := MetricPoint{Timestamp: current}
		if value, ok := values[timestamp]; ok {
			valueCopy := value
			point.Value = &valueCopy
		}
		points = append(points, point)
	}
	return points
}

func buildEmptyMetricPoints(start time.Time, end time.Time, step time.Duration) []MetricPoint {
	points := make([]MetricPoint, 0, int(end.Sub(start)/step)+1)
	for current := start; !current.After(end); current = current.Add(step) {
		points = append(points, MetricPoint{Timestamp: current})
	}
	return points
}

func decodePrometheusTimestamp(raw any) (int64, bool) {
	switch value := raw.(type) {
	case float64:
		return int64(value), true
	case json.Number:
		parsed, err := value.Int64()
		if err != nil {
			floatValue, floatErr := value.Float64()
			if floatErr != nil {
				return 0, false
			}
			return int64(floatValue), true
		}
		return parsed, true
	default:
		return 0, false
	}
}

func decodePrometheusValue(raw any) (float64, bool) {
	switch value := raw.(type) {
	case string:
		parsed, err := strconv.ParseFloat(value, 64)
		return parsed, err == nil
	case json.Number:
		parsed, err := value.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

func (r PrometheusMetricsReader) httpClient() *http.Client {
	if r.Client != nil {
		return r.Client
	}
	return http.DefaultClient
}

func (r PrometheusMetricsReader) metricRange() time.Duration {
	if r.Range > 0 {
		return r.Range
	}
	return time.Hour
}

func (r PrometheusMetricsReader) metricStep() time.Duration {
	if r.Step > 0 {
		return r.Step
	}
	return 5 * time.Minute
}

func (r PrometheusMetricsReader) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}
