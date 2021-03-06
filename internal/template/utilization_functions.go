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

package template

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"text/template"
)

// https://github.com/prometheus/prometheus/blob/3240cf83f08e448e0b96a4a1f96c0e8b2d51cf61/util/strutil/strconv.go#L23
var invalidLabelCharRE = regexp.MustCompile(`[^a-zA-Z0-9_]`)

func cpuUtilization(data MetricData, labelArgs ...string) (string, error) {
	cpuUtilizationQueryTemplate := `
scalar(
  round(
    (
      sum(
        sum(
          increase(container_cpu_usage_seconds_total{container="", image=""}[{{ .Range }}])
        ) by (pod)
        *
        on (pod) group_left kube_pod_labels{{ .Labels }}
      )
      /
      sum(
        sum_over_time(kube_pod_container_resource_requests_cpu_cores[{{ .Range }}:1s])
        *
        on (pod) group_left kube_pod_labels{{ .Labels }}
      )
    )
  * 100, 0.0001)
)`

	return renderUtilization(cpuUtilizationQueryTemplate, data, labelArgs...)
}

func memoryUtilization(data MetricData, labelArgs ...string) (string, error) {
	memoryUtilizationQueryTemplate := `
scalar(
  round(
    (
      avg(
        max(
          container_memory_max_usage_bytes
        ) by (pod)
        *
        on (pod) group_left kube_pod_labels{{ .Labels }}
        /
        sum(
          kube_pod_container_resource_requests_memory_bytes
        ) by (pod)
      )
    )
  * 100, 0.0001)
)`

	return renderUtilization(memoryUtilizationQueryTemplate, data, labelArgs...)
}

func cpuRequests(data MetricData, labelArgs ...string) (string, error) {
	cpuResourcesQueryTemplate := `
scalar(
  sum(
    avg_over_time(kube_pod_container_resource_requests_cpu_cores[{{ .Range }}])
    *
    on (pod) group_left kube_pod_labels{{ .Labels }}
  )
)`

	return renderUtilization(cpuResourcesQueryTemplate, data, labelArgs...)
}

func memoryRequests(data MetricData, labelArgs ...string) (string, error) {
	memoryResourcesQueryTemplate := `
scalar(
  sum(
    avg_over_time(kube_pod_container_resource_requests_memory_bytes[{{ .Range }}])
    *
    on (pod) group_left kube_pod_labels{{ .Labels }}
  )
)`

	return renderUtilization(memoryResourcesQueryTemplate, data, labelArgs...)
}

func renderUtilization(query string, data MetricData, labelArgs ...string) (string, error) {
	tmpl := template.Must(template.New("query").Parse(query))

	// Always include the trial namespace
	labels := []string{fmt.Sprintf("namespace=\"%s\"", data.Trial.Namespace)}

	for _, label := range strings.Split(strings.Join(labelArgs, ","), ",") {
		if label == "" {
			continue
		}

		kvpair := strings.Split(label, "=")
		if len(kvpair) != 2 {
			return "", fmt.Errorf("invalid label for utilization query, expected key=value, got: %s", label)
		}

		// Sanitize label name for prometheus usage
		kvpair[0] = invalidLabelCharRE.ReplaceAllString(kvpair[0], "_")

		labels = append(labels, fmt.Sprintf("label_%s=\"%s\"", kvpair[0], kvpair[1]))
	}

	input := struct {
		MetricData
		Labels string
	}{
		data,
		fmt.Sprintf("{%s}", strings.Join(labels, ",")),
	}

	var output bytes.Buffer
	if err := tmpl.Execute(&output, input); err != nil {
		return "", err
	}

	return output.String(), nil
}
