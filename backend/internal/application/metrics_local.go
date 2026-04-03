package application

import (
	"context"
	"hash/fnv"
	"time"
)

type LocalMetricsReader struct{}

func (LocalMetricsReader) Read(ctx context.Context, record Record, duration time.Duration, step time.Duration) ([]MetricSeries, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	queryStep := step
	if queryStep <= 0 {
		queryStep = 5 * time.Minute
	}
	queryWindow := duration
	if queryWindow <= 0 {
		queryWindow = time.Hour
	}

	seed := hashRecord(record.ID)
	now := time.Now().UTC().Truncate(queryStep)

	return []MetricSeries{
		buildMetricSeries(now, seed, "request_rate", "Requests", "rpm", 0.0, 0.0, false, queryWindow, queryStep),
		buildMetricSeries(now, seed+11, "error_rate", "Error Rate", "%", 0.0, 0.0, true, queryWindow, queryStep),
		buildMetricSeries(now, seed+23, "latency_p95", "P95 Latency", "ms", 0.0, 0.0, false, queryWindow, queryStep),
		buildMetricSeries(now, seed+31, "cpu_usage", "CPU Usage", "cores", 0.0, 0.0, false, queryWindow, queryStep),
		buildMetricSeries(now, seed+47, "memory_usage", "Memory Usage", "MiB", 0.0, 0.0, true, queryWindow, queryStep),
	}, nil
}

func buildMetricSeries(
	start time.Time,
	seed uint32,
	key string,
	label string,
	unit string,
	base float64,
	delta float64,
	withGaps bool,
	duration time.Duration,
	step time.Duration,
) MetricSeries {
	numPoints := int(duration / step)
	if numPoints <= 0 {
		numPoints = 12
	}
	points := make([]MetricPoint, 0, numPoints)
	for offset := numPoints - 1; offset >= 0; offset-- {
		value := base + float64((int(seed)+offset*7)%9)*delta
		var pointValue *float64
		if !withGaps || offset%5 != 0 {
			pointValue = &value
		}

		points = append(points, MetricPoint{
			Timestamp: start.Add(-time.Duration(offset) * step),
			Value:     pointValue,
		})
	}

	return MetricSeries{
		Key:    key,
		Label:  label,
		Unit:   unit,
		Points: points,
	}
}

func hashRecord(value string) uint32 {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(value))
	return hasher.Sum32()
}
