package application

import (
	"context"
	"hash/fnv"
	"time"
)

type LocalMetricsReader struct{}

func (LocalMetricsReader) Read(ctx context.Context, record Record) ([]MetricSeries, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	seed := hashRecord(record.ID)
	now := time.Now().UTC().Truncate(5 * time.Minute)

	return []MetricSeries{
		buildMetricSeries(now, seed, "request_rate", "Requests", "rpm", 120, 17, false),
		buildMetricSeries(now, seed+11, "error_rate", "Error Rate", "%", 1.3, 0.35, true),
		buildMetricSeries(now, seed+23, "latency_p95", "P95 Latency", "ms", 180, 22, false),
		buildMetricSeries(now, seed+31, "cpu_usage", "CPU Usage", "cores", 0.42, 0.08, false),
		buildMetricSeries(now, seed+47, "memory_usage", "Memory Usage", "MiB", 286, 15, true),
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
) MetricSeries {
	points := make([]MetricPoint, 0, 12)
	for offset := 11; offset >= 0; offset-- {
		value := base + float64((int(seed)+offset*7)%9)*delta
		var pointValue *float64
		if !withGaps || offset%5 != 0 {
			pointValue = &value
		}

		points = append(points, MetricPoint{
			Timestamp: start.Add(-time.Duration(offset) * 5 * time.Minute),
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
