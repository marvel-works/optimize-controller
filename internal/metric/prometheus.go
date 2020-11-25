/*
Copyright 2020 GramLabs, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package metric

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/go-logr/logr"
	prom "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
)

// CaptureError describes problems that arise while capturing Prometheus metric values.
type CaptureError struct {
	// A description of what went wrong
	Message string
	// The URL that was used to capture the metric
	Address string
	// The metric query that failed
	Query string
	// The completion time at which the query was executed
	CompletionTime time.Time
	// The minimum amount of time until the metric is expected to be available
	RetryAfter time.Duration
}

func (e *CaptureError) Error() string {
	return e.Message
}

func capturePrometheusMetric(ctx context.Context, log logr.Logger, m *redskyv1beta1.Metric, completionTime time.Time) (value float64, valueError float64, err error) {
	// Get the Prometheus API
	c, err := prom.NewClient(prom.Config{Address: m.URL})
	if err != nil {
		return 0, 0, err
	}
	promAPI := promv1.NewAPI(c)

	// Make sure Prometheus is ready
	queryTime, err := checkReady(ctx, promAPI, completionTime)
	if err != nil {
		return 0, 0, err
	}

	// If we needed to adjust the query time, log it so we have a record of the actual time being used
	if !queryTime.Equal(completionTime) {
		log.WithValues("queryTime", queryTime).Info("Adjusted completion time for Prometheus query")
	}

	// Execute query
	value, err = queryScalar(ctx, promAPI, m.Query, queryTime)
	if err != nil {
		return 0, 0, err
	}
	if math.IsNaN(value) {
		err := &CaptureError{Message: "metric data not available", Address: m.URL, Query: m.Query, CompletionTime: completionTime}
		if strings.HasPrefix(m.Query, "scalar(") {
			err.Message += " (the scalar function may have received an input vector whose size is not 1)"
		}
		return 0, 0, err
	}

	// Execute the error query (if configured)
	if m.ErrorQuery != "" {
		valueError, err = queryScalar(ctx, promAPI, m.ErrorQuery, queryTime)
		if err != nil {
			return 0, 0, err
		}
	}

	return value, valueError, nil
}

func checkReady(ctx context.Context, api promv1.API, t time.Time) (time.Time, error) {
	targets, err := api.Targets(ctx)
	if err != nil {
		return t, err
	}

	queryTime := t
	for _, target := range targets.Active {
		if target.Health != promv1.HealthGood {
			continue
		}

		if target.LastScrape.Before(t) {
			// TODO Can we make a more informed delay?
			return t, &CaptureError{RetryAfter: 5 * time.Second}
		}

		if target.LastScrape.After(queryTime) {
			queryTime = target.LastScrape
		}
	}

	return queryTime, nil
}

func queryScalar(ctx context.Context, api promv1.API, q string, t time.Time) (float64, error) {
	v, _, err := api.Query(ctx, q, t)
	if err != nil {
		return 0, err
	}

	if v.Type() != model.ValScalar {
		return 0, fmt.Errorf("expected scalar query result, got %s", v.Type())
	}

	return float64(v.(*model.Scalar).Value), nil
}
