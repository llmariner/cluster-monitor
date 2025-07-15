package prometheus

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	promapi "github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// Client is a Prometheus client.
type Client struct {
	promAPI v1.API
	logger  logr.Logger
}

// NewClient returns a new Client.
func NewClient(promURL string, logger logr.Logger) (*Client, error) {
	client, err := promapi.NewClient(promapi.Config{Address: promURL})
	if err != nil {
		return nil, err
	}

	return &Client{
		promAPI: v1.NewAPI(client),
		logger:  logger.WithName("prometheus"),
	}, nil
}

// QueryDCGMMetric queries a DCGM metric for a specific time range.
func (c *Client) QueryDCGMMetric(ctx context.Context, metricName string, r v1.Range) (*DCGMMetricSamples, error) {
	v, err := c.queryRange(ctx, metricName, r)
	if err != nil {
		return nil, err
	}

	m := map[DCGMMetricKey][]model.SamplePair{}

	for _, stream := range v {
		k, err := NewDCGMMetricKey(stream.Metric)
		if err != nil {
			return nil, err
		}
		m[k] = append(m[k], stream.Values...)
	}

	return &DCGMMetricSamples{
		ValuesByKey: m,
	}, nil
}

func (c *Client) queryRange(ctx context.Context, query string, r v1.Range) (model.Matrix, error) {
	val, warns, err := c.promAPI.QueryRange(ctx, query, r)
	if err != nil {
		return nil, err
	}
	if len(warns) > 0 {
		c.logger.Info("Igoring warnings from Prometheus: %+v", warns)
	}
	if val.Type() != model.ValMatrix {
		return nil, fmt.Errorf("unexpected type: %+v", val.Type())
	}
	v, ok := val.(model.Matrix)
	if !ok {
		return nil, fmt.Errorf("convert value to Matrix: %+v", val)
	}
	return v, nil
}
