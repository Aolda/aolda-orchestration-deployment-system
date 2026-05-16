package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
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

type CompositeMetricsReader struct {
	Primary  MetricsReader
	Fallback MetricsReader
}

type ErrorMetricsReader struct {
	Err error
}

type prometheusMetricDefinition struct {
	Key          string
	Label        string
	Unit         string
	Queries      []string
	BatchQueries []prometheusBatchQuery
}

type prometheusBatchQuery struct {
	Query        string
	GroupLabel   string
	GroupFromPod bool
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
	Metric map[string]string `json:"metric"`
	Values [][]any           `json:"values"`
}

func (r CompositeMetricsReader) Read(ctx context.Context, record Record, duration time.Duration, step time.Duration) ([]MetricSeries, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.Primary == nil && r.Fallback == nil {
		return nil, fmt.Errorf("metrics reader is not configured")
	}

	if r.Primary == nil {
		return r.Fallback.Read(ctx, record, duration, step)
	}

	primary, err := r.Primary.Read(ctx, record, duration, step)
	if err != nil {
		if r.Fallback == nil {
			return nil, err
		}
		return r.Fallback.Read(ctx, record, duration, step)
	}
	if r.Fallback == nil {
		return primary, nil
	}

	fallback, err := r.Fallback.Read(ctx, record, duration, step)
	if err != nil {
		return primary, nil
	}

	return mergeMetricSeries(primary, fallback), nil
}

func (r CompositeMetricsReader) ReadMany(ctx context.Context, records []Record, duration time.Duration, step time.Duration) (map[string][]MetricSeries, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.Primary == nil && r.Fallback == nil {
		return nil, fmt.Errorf("metrics reader is not configured")
	}

	if r.Primary == nil {
		return readManyMetrics(ctx, r.Fallback, records, duration, step)
	}

	primary, err := readManyMetrics(ctx, r.Primary, records, duration, step)
	if err != nil {
		if r.Fallback == nil {
			return primary, err
		}
		fallback, fallbackErr := readManyMetrics(ctx, r.Fallback, records, duration, step)
		if fallbackErr != nil {
			return primary, err
		}
		var batchErr BatchMetricsReadError
		if errors.As(err, &batchErr) {
			return mergeMetricMaps(records, primary, fallback), nil
		}
		return fallback, nil
	}
	if r.Fallback == nil {
		return primary, nil
	}

	fallback, err := readManyMetrics(ctx, r.Fallback, records, duration, step)
	if err != nil {
		var batchErr BatchMetricsReadError
		if errors.As(err, &batchErr) {
			return mergeMetricMaps(records, primary, fallback), nil
		}
		return primary, nil
	}

	return mergeMetricMaps(records, primary, fallback), nil
}

func mergeMetricMaps(records []Record, primary map[string][]MetricSeries, fallback map[string][]MetricSeries) map[string][]MetricSeries {
	merged := make(map[string][]MetricSeries, len(records))
	for _, record := range records {
		merged[record.ID] = mergeMetricSeries(primary[record.ID], fallback[record.ID])
	}
	return merged
}

func readManyMetrics(ctx context.Context, reader MetricsReader, records []Record, duration time.Duration, step time.Duration) (map[string][]MetricSeries, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if reader == nil {
		return nil, fmt.Errorf("metrics reader is not configured")
	}
	if batchReader, ok := reader.(BatchMetricsReader); ok {
		return batchReader.ReadMany(ctx, records, duration, step)
	}

	items := make(map[string][]MetricSeries, len(records))
	for _, record := range records {
		metrics, err := reader.Read(ctx, record, duration, step)
		if err != nil {
			return nil, err
		}
		items[record.ID] = metrics
	}
	return items, nil
}

func (r ErrorMetricsReader) Read(ctx context.Context, record Record, duration time.Duration, step time.Duration) ([]MetricSeries, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.Err == nil {
		return []MetricSeries{}, nil
	}
	return nil, r.Err
}

func (r ErrorMetricsReader) ReadMany(ctx context.Context, records []Record, duration time.Duration, step time.Duration) (map[string][]MetricSeries, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.Err != nil {
		return nil, r.Err
	}

	items := make(map[string][]MetricSeries, len(records))
	for _, record := range records {
		items[record.ID] = []MetricSeries{}
	}
	return items, nil
}

func (r PrometheusMetricsReader) Read(ctx context.Context, record Record, duration time.Duration, step time.Duration) ([]MetricSeries, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	now := r.now().UTC()

	// Use provided duration/step if greater than zero, otherwise fallback to defaults
	queryStep := step
	if queryStep <= 0 {
		queryStep = r.metricStep()
	}
	queryWindow := duration
	if queryWindow <= 0 {
		queryWindow = r.metricRange()
	}

	end := now.Truncate(queryStep)
	start := end.Add(-queryWindow).Add(queryStep)
	if start.After(end) {
		start = end
	}

	metrics := make([]MetricSeries, 0, len(prometheusMetricDefinitions))
	for _, definition := range prometheusMetricDefinitions {
		points, err := r.queryMetricPoints(ctx, record, definition.Queries, start, end, queryStep)
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

func (r PrometheusMetricsReader) ReadMany(ctx context.Context, records []Record, duration time.Duration, step time.Duration) (map[string][]MetricSeries, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	items := make(map[string][]MetricSeries, len(records))
	if len(records) == 0 {
		return items, nil
	}

	start, end, queryStep := r.metricWindow(duration, step)
	for _, record := range records {
		items[record.ID] = emptyMetricSeries(start, end, queryStep)
	}

	failedIDs := map[string]struct{}{}
	failures := []string{}
	for _, group := range groupRecordsByNamespace(records) {
		if err := r.readNamespaceMetrics(ctx, group, start, end, queryStep, items); err != nil {
			for _, record := range group {
				failedIDs[record.ID] = struct{}{}
			}
			failures = append(failures, err.Error())
		}
	}
	if len(failures) > 0 {
		return items, BatchMetricsReadError{
			Err:                  fmt.Errorf("prometheus metrics batch query failed: %s", strings.Join(failures, "; ")),
			FailedApplicationIDs: failedIDs,
		}
	}
	return items, nil
}

func (r PrometheusMetricsReader) readNamespaceMetrics(
	ctx context.Context,
	records []Record,
	start time.Time,
	end time.Time,
	step time.Duration,
	items map[string][]MetricSeries,
) error {
	if len(records) == 0 {
		return nil
	}

	for index, definition := range prometheusMetricDefinitions {
		valuesByApplicationID, err := r.queryMetricPointsMany(ctx, records, definition, start, end, step)
		if err != nil {
			return err
		}
		for _, record := range records {
			values, ok := valuesByApplicationID[record.ID]
			if !ok {
				continue
			}
			series := MetricSeries{
				Key:    definition.Key,
				Label:  definition.Label,
				Unit:   definition.Unit,
				Points: buildMetricPointsFromValues(start, end, step, values),
			}
			current := items[record.ID]
			if index < len(current) {
				current[index] = series
			} else {
				current = append(current, series)
			}
			items[record.ID] = current
		}
	}

	return nil
}

func (r PrometheusMetricsReader) metricWindow(duration time.Duration, step time.Duration) (time.Time, time.Time, time.Duration) {
	now := r.now().UTC()

	queryStep := step
	if queryStep <= 0 {
		queryStep = r.metricStep()
	}
	queryWindow := duration
	if queryWindow <= 0 {
		queryWindow = r.metricRange()
	}

	end := now.Truncate(queryStep)
	start := end.Add(-queryWindow).Add(queryStep)
	if start.After(end) {
		start = end
	}
	return start, end, queryStep
}

func emptyMetricSeries(start time.Time, end time.Time, step time.Duration) []MetricSeries {
	metrics := make([]MetricSeries, 0, len(prometheusMetricDefinitions))
	for _, definition := range prometheusMetricDefinitions {
		metrics = append(metrics, MetricSeries{
			Key:    definition.Key,
			Label:  definition.Label,
			Unit:   definition.Unit,
			Points: buildEmptyMetricPoints(start, end, step),
		})
	}
	return metrics
}

func groupRecordsByNamespace(records []Record) [][]Record {
	groups := [][]Record{}
	indexByNamespace := map[string]int{}
	for _, record := range records {
		namespace := strings.TrimSpace(record.Namespace)
		index, ok := indexByNamespace[namespace]
		if !ok {
			index = len(groups)
			indexByNamespace[namespace] = index
			groups = append(groups, []Record{})
		}
		groups[index] = append(groups[index], record)
	}
	return groups
}

func (r PrometheusMetricsReader) queryMetricPointsMany(
	ctx context.Context,
	records []Record,
	definition prometheusMetricDefinition,
	start time.Time,
	end time.Time,
	step time.Duration,
) (map[string]map[int64]float64, error) {
	valuesByApplicationID := make(map[string]map[int64]float64, len(records))
	if len(records) == 0 {
		return valuesByApplicationID, nil
	}

	appByName := make(map[string]string, len(records))
	for _, record := range records {
		appByName[record.Name] = record.ID
	}

	for _, batchQuery := range definition.BatchQueries {
		resolvedAtQueryStart := make(map[string]struct{}, len(valuesByApplicationID))
		for applicationID := range valuesByApplicationID {
			resolvedAtQueryStart[applicationID] = struct{}{}
		}

		query := renderPrometheusBatchQuery(batchQuery.Query, records)
		valuesByGroup, found, err := r.queryRangeGrouped(ctx, query, batchQuery.GroupLabel, start, end, step)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}

		for groupValue, values := range valuesByGroup {
			applicationID, ok := appByName[groupValue]
			if batchQuery.GroupFromPod {
				applicationID, ok = applicationIDForPod(records, groupValue)
			}
			if !ok {
				continue
			}
			if _, alreadyResolved := resolvedAtQueryStart[applicationID]; alreadyResolved {
				continue
			}
			mergeMetricValueMap(valuesByApplicationID, applicationID, values)
		}

		if len(valuesByApplicationID) == len(records) {
			break
		}
	}

	return valuesByApplicationID, nil
}

func mergeMetricValueMap(target map[string]map[int64]float64, applicationID string, values map[int64]float64) {
	if len(values) == 0 {
		return
	}
	current, ok := target[applicationID]
	if !ok {
		current = make(map[int64]float64, len(values))
		target[applicationID] = current
	}
	for timestamp, value := range values {
		current[timestamp] += value
	}
}

func applicationIDForPod(records []Record, podName string) (string, bool) {
	for _, record := range records {
		if podNameMatchesApplication(podName, record.Name) {
			return record.ID, true
		}
	}
	return "", false
}

func podNameMatchesApplication(podName string, appName string) bool {
	podPattern := regexp.MustCompile("^" + regexp.QuoteMeta(appName) + `-[a-z0-9]+(?:-[a-z0-9]+)?$`)
	return podPattern.MatchString(strings.TrimSpace(podName))
}

func mergeMetricSeries(primary []MetricSeries, fallback []MetricSeries) []MetricSeries {
	if len(primary) == 0 {
		return fallback
	}

	fallbackByKey := make(map[string]MetricSeries, len(fallback))
	for _, series := range fallback {
		fallbackByKey[series.Key] = series
	}

	merged := make([]MetricSeries, 0, len(primary)+len(fallback))
	for _, series := range primary {
		fallbackSeries, ok := fallbackByKey[series.Key]
		if !ok {
			merged = append(merged, series)
			continue
		}

		merged = append(merged, mergeSingleMetricSeries(series, fallbackSeries))
		delete(fallbackByKey, series.Key)
	}

	for _, series := range fallbackByKey {
		merged = append(merged, series)
	}

	return merged
}

func metricSeriesHasValues(series MetricSeries) bool {
	for _, point := range series.Points {
		if point.Value != nil {
			return true
		}
	}
	return false
}

func mergeSingleMetricSeries(primary MetricSeries, fallback MetricSeries) MetricSeries {
	if len(primary.Points) == 0 {
		return fallback
	}
	if !metricSeriesHasValues(primary) && metricSeriesHasValues(fallback) {
		return mergeMetricPoints(primary, fallback)
	}
	if !metricSeriesHasValues(fallback) {
		return primary
	}
	return mergeMetricPoints(primary, fallback)
}

func mergeMetricPoints(primary MetricSeries, fallback MetricSeries) MetricSeries {
	if len(primary.Points) == 0 {
		return fallback
	}

	fallbackByTimestamp := make(map[int64]*float64, len(fallback.Points))
	for _, point := range fallback.Points {
		if point.Value == nil {
			continue
		}
		valueCopy := *point.Value
		fallbackByTimestamp[point.Timestamp.Unix()] = &valueCopy
	}

	merged := primary
	merged.Points = make([]MetricPoint, 0, len(primary.Points))
	for _, point := range primary.Points {
		if point.Value == nil {
			if fallbackValue, ok := fallbackByTimestamp[point.Timestamp.Unix()]; ok {
				merged.Points = append(merged.Points, MetricPoint{
					Timestamp: point.Timestamp,
					Value:     fallbackValue,
				})
				continue
			}
		}
		merged.Points = append(merged.Points, point)
	}

	if merged.Label == "" {
		merged.Label = fallback.Label
	}
	if merged.Unit == "" {
		merged.Unit = fallback.Unit
	}
	return merged
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
		BatchQueries: []prometheusBatchQuery{
			{
				Query:      `sum by (destination_app) (rate(istio_requests_total{reporter="destination",destination_workload_namespace="{{namespace}}",destination_app=~"{{appMatcher}}"}[5m])) * 60`,
				GroupLabel: "destination_app",
			},
			{
				Query:      `sum by (application) (rate(http_server_requests_seconds_count{namespace="{{namespace}}",application=~"{{appMatcher}}"}[5m])) * 60`,
				GroupLabel: "application",
			},
		},
	},
	{
		Key:   "error_rate",
		Label: "Error Rate",
		Unit:  "%",
		Queries: []string{
			`100 * (sum(rate(istio_requests_total{reporter="destination",destination_workload_namespace="{{namespace}}",destination_app="{{appName}}",response_code=~"5.."}[5m])) / clamp_min(sum(rate(istio_requests_total{reporter="destination",destination_workload_namespace="{{namespace}}",destination_app="{{appName}}"}[5m])), 0.001))`,
			`100 * (sum(rate(http_server_requests_seconds_count{namespace="{{namespace}}",application="{{appName}}",status=~"5.."}[5m])) / clamp_min(sum(rate(http_server_requests_seconds_count{namespace="{{namespace}}",application="{{appName}}"}[5m])), 0.001))`,
		},
		BatchQueries: []prometheusBatchQuery{
			{
				Query:      `100 * (((sum by (destination_app) (rate(istio_requests_total{reporter="destination",destination_workload_namespace="{{namespace}}",destination_app=~"{{appMatcher}}",response_code=~"5.."}[5m]))) or on (destination_app) (0 * sum by (destination_app) (rate(istio_requests_total{reporter="destination",destination_workload_namespace="{{namespace}}",destination_app=~"{{appMatcher}}"}[5m])))) / clamp_min(sum by (destination_app) (rate(istio_requests_total{reporter="destination",destination_workload_namespace="{{namespace}}",destination_app=~"{{appMatcher}}"}[5m])), 0.001))`,
				GroupLabel: "destination_app",
			},
			{
				Query:      `100 * (((sum by (application) (rate(http_server_requests_seconds_count{namespace="{{namespace}}",application=~"{{appMatcher}}",status=~"5.."}[5m]))) or on (application) (0 * sum by (application) (rate(http_server_requests_seconds_count{namespace="{{namespace}}",application=~"{{appMatcher}}"}[5m])))) / clamp_min(sum by (application) (rate(http_server_requests_seconds_count{namespace="{{namespace}}",application=~"{{appMatcher}}"}[5m])), 0.001))`,
				GroupLabel: "application",
			},
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
		BatchQueries: []prometheusBatchQuery{
			{
				Query:      `histogram_quantile(0.95, sum by (destination_app, le) (rate(istio_request_duration_milliseconds_bucket{reporter="destination",destination_workload_namespace="{{namespace}}",destination_app=~"{{appMatcher}}"}[5m])))`,
				GroupLabel: "destination_app",
			},
			{
				Query:      `histogram_quantile(0.95, sum by (destination_app, le) (rate(istio_request_duration_seconds_bucket{reporter="destination",destination_workload_namespace="{{namespace}}",destination_app=~"{{appMatcher}}"}[5m]))) * 1000`,
				GroupLabel: "destination_app",
			},
			{
				Query:      `histogram_quantile(0.95, sum by (application, le) (rate(http_server_requests_seconds_bucket{namespace="{{namespace}}",application=~"{{appMatcher}}"}[5m]))) * 1000`,
				GroupLabel: "application",
			},
		},
	},
	{
		Key:   "cpu_usage",
		Label: "CPU Usage",
		Unit:  "cores",
		Queries: []string{
			`sum(rate(container_cpu_usage_seconds_total{namespace="{{namespace}}",pod=~"{{podRegex}}",container!="",container!="POD"}[5m]))`,
		},
		BatchQueries: []prometheusBatchQuery{
			{
				Query:        `sum by (pod) (rate(container_cpu_usage_seconds_total{namespace="{{namespace}}",pod=~"{{podMatcher}}",container!="",container!="POD"}[5m]))`,
				GroupLabel:   "pod",
				GroupFromPod: true,
			},
		},
	},
	{
		Key:   "memory_usage",
		Label: "Memory Usage",
		Unit:  "MiB",
		Queries: []string{
			`sum(container_memory_working_set_bytes{namespace="{{namespace}}",pod=~"{{podRegex}}",container!="",container!="POD"}) / 1024 / 1024`,
		},
		BatchQueries: []prometheusBatchQuery{
			{
				Query:        `sum by (pod) (container_memory_working_set_bytes{namespace="{{namespace}}",pod=~"{{podMatcher}}",container!="",container!="POD"}) / 1024 / 1024`,
				GroupLabel:   "pod",
				GroupFromPod: true,
			},
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

func (r PrometheusMetricsReader) queryRangeGrouped(
	ctx context.Context,
	query string,
	groupLabel string,
	start time.Time,
	end time.Time,
	step time.Duration,
) (map[string]map[int64]float64, bool, error) {
	if strings.TrimSpace(groupLabel) == "" {
		return nil, false, fmt.Errorf("prometheus grouped query requires a group label")
	}
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

	values := make(map[string]map[int64]float64)
	for _, series := range payload.Data.Result {
		groupValue := strings.TrimSpace(series.Metric[groupLabel])
		if groupValue == "" {
			continue
		}
		seriesValues := values[groupValue]
		if seriesValues == nil {
			seriesValues = map[int64]float64{}
			values[groupValue] = seriesValues
		}
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
			seriesValues[timestamp] += value
		}
		if len(seriesValues) == 0 {
			delete(values, groupValue)
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

func renderPrometheusBatchQuery(template string, records []Record) string {
	namespace := ""
	appNames := make([]string, 0, len(records))
	for _, record := range records {
		if namespace == "" {
			namespace = record.Namespace
		}
		if strings.TrimSpace(record.Name) != "" {
			appNames = append(appNames, prometheusRegexLiteral(record.Name))
		}
	}

	appMatcher := strings.Join(appNames, "|")
	podMatcher := "(?:" + appMatcher + `)-[a-z0-9]+(?:-[a-z0-9]+)?`
	replacer := strings.NewReplacer(
		"{{namespace}}", namespace,
		"{{appMatcher}}", appMatcher,
		"{{podMatcher}}", podMatcher,
	)
	return replacer.Replace(template)
}

func prometheusRegexLiteral(value string) string {
	quoted := regexp.QuoteMeta(value)
	quoted = strings.ReplaceAll(quoted, `\`, `\\`)
	quoted = strings.ReplaceAll(quoted, `"`, `\"`)
	return quoted
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
