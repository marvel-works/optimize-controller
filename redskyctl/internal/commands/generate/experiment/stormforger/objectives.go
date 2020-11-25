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

package stormforger

import (
	redskyappsv1alpha1 "github.com/redskyops/redskyops-controller/api/apps/v1alpha1"
	redskyv1beta1 "github.com/redskyops/redskyops-controller/api/v1beta1"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/generate/experiment/k8s"
	corev1 "k8s.io/api/core/v1"
)

// addStormForgerObjectives adds metrics for objectives supported by StormForger.
func addStormForgerObjectives(app *redskyappsv1alpha1.Application, list *corev1.List) error {
	for i := range app.Objectives {
		obj := &app.Objectives[i]
		switch {

		case obj.Latency != nil:
			addStormForgerLatencyMetric(obj, list)

		}
	}

	return nil
}

func addStormForgerLatencyMetric(obj *redskyappsv1alpha1.Objective, list *corev1.List) {
	var m string
	switch redskyappsv1alpha1.FixLatency(obj.Latency.LatencyType) {
	case redskyappsv1alpha1.LatencyMinimum:
		m = "min"
	case redskyappsv1alpha1.LatencyMaximum:
		m = "max"
	case redskyappsv1alpha1.LatencyMean:
		m = "mean"
	case redskyappsv1alpha1.LatencyPercentile50:
		m = "median"
	case redskyappsv1alpha1.LatencyPercentile95:
		m = "percentile_95"
	case redskyappsv1alpha1.LatencyPercentile99:
		m = "percentile_99"
	default:
		// This is not a latency measure that StormForger can produce, skip it
		return
	}

	// Filter the metric to match what was sent to the Push Gateway
	exp := k8s.FindOrAddExperiment(list)
	exp.Spec.Metrics = append(exp.Spec.Metrics, redskyv1beta1.Metric{
		Name:     obj.Name,
		Minimize: true,
		Type:     redskyv1beta1.MetricPrometheus,
		Query:    `scalar(` + m + `{job="trialRun",instance="{{ .Trial.Name }}"})`,
		Min:      obj.Min,
		Max:      obj.Max,
		Optimize: obj.Optimize,
	})
	obj.Implemented = true
}
