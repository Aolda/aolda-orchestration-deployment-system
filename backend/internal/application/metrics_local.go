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

	return []MetricSeries{}, nil
}

func (r LocalMetricsReader) ReadMany(ctx context.Context, records []Record, duration time.Duration, step time.Duration) (map[string][]MetricSeries, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	items := make(map[string][]MetricSeries, len(records))
	for _, record := range records {
		metrics, err := r.Read(ctx, record, duration, step)
		if err != nil {
			return nil, err
		}
		items[record.ID] = metrics
	}
	return items, nil
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
