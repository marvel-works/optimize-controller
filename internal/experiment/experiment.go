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

package experiment

import (
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	"github.com/thestormforge/optimize-controller/internal/controller"
	"github.com/thestormforge/optimize-controller/internal/trial"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// HasTrialFinalizer is a finalizer that indicates an experiment has at least one trial
	HasTrialFinalizer = "hasTrialFinalizer.redskyops.dev"
)

// TODO Make the constant names better reflect the code, not the text
const (
	// PhaseCreated indicates that the experiment has been created on the remote server but is not receiving trials
	PhaseCreated string = "Created"
	// PhasePaused indicates that the experiment has been paused, i.e. the desired replica count is zero
	PhasePaused = "Paused"
	// PhaseEmpty indicates there is no record of trials being run in the cluster
	PhaseEmpty = "Never run" // TODO This is misleading, it could be that we already deleted the trials that ran
	// PhaseIdle indicates that the experiment is waiting for trials to be manually created
	PhaseIdle = "Idle"
	// PhaseRunning indicates that there are actively running trials for the experiment
	PhaseRunning = "Running"
	// PhaseCompleted indicates that the experiment has exhausted it's trial budget and is no longer expecting new trials
	PhaseCompleted = "Completed"
	// PhaseFailed indicates that the experiment has failed
	PhaseFailed = "Failed"
	// PhaseDeleted indicates that the experiment has been deleted and is waiting for trials to be cleaned up
	PhaseDeleted = "Deleted"
)

// UpdateStatus will ensure the experiment's status matches what is in the supplied trial list; returns true only if
// changes were necessary
func UpdateStatus(exp *redskyv1beta1.Experiment, trialList *redskyv1beta1.TrialList) bool {
	// Count the active trials
	activeTrials := int32(0)
	for i := range trialList.Items {
		t := &trialList.Items[i]
		if trial.IsActive(t) && !trial.IsAbandoned(t) {
			activeTrials++
		}
	}

	// Determine the phase
	phase := summarize(exp, activeTrials, len(trialList.Items))

	// Update the status object
	var dirty bool
	if exp.Status.Phase != phase {
		exp.Status.Phase = phase
		dirty = true
	}
	if exp.Status.ActiveTrials != activeTrials {
		exp.Status.ActiveTrials = activeTrials
		dirty = true
	}

	// If we made a change, record this in the metric gauges
	if dirty {
		controller.ExperimentTrials.WithLabelValues(exp.Name).Set(float64(len(trialList.Items)))
		controller.ExperimentActiveTrials.WithLabelValues(exp.Name).Set(float64(activeTrials))
		return true
	}
	return false
}

func summarize(exp *redskyv1beta1.Experiment, activeTrials int32, totalTrials int) string {
	remote := exp.Annotations[redskyv1beta1.AnnotationExperimentURL] != "" // TODO Or check for the server finalizer?

	if !exp.GetDeletionTimestamp().IsZero() {
		return PhaseDeleted
	}

	for _, c := range exp.Status.Conditions {
		switch c.Type {
		case redskyv1beta1.ExperimentFailed:
			if c.Status == corev1.ConditionTrue {
				return PhaseFailed
			}
		}
	}

	if activeTrials > 0 {
		return PhaseRunning
	}

	if exp.Replicas() == 0 {
		if remote && exp.Annotations[redskyv1beta1.AnnotationNextTrialURL] == "" {
			return PhaseCompleted
		}
		return PhasePaused
	}

	if totalTrials == 0 {
		if remote {
			return PhaseCreated
		}
		return PhaseEmpty
	}

	return PhaseIdle
}

func ApplyCondition(status *redskyv1beta1.ExperimentStatus, conditionType redskyv1beta1.ExperimentConditionType, conditionStatus corev1.ConditionStatus, reason, message string, time *metav1.Time) {
	if time == nil {
		now := metav1.Now()
		time = &now
	}

	newCondition := redskyv1beta1.ExperimentCondition{
		Type:               conditionType,
		Status:             conditionStatus,
		Reason:             reason,
		Message:            message,
		LastProbeTime:      *time,
		LastTransitionTime: *time,
	}

	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			if status.Conditions[i].Status != conditionStatus {
				status.Conditions[i] = newCondition
			} else {
				status.Conditions[i].LastProbeTime = *time
			}
			return
		}
	}

	status.Conditions = append(status.Conditions, newCondition)
}
