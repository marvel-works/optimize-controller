package trial

import (
	"context"
	"time"

	okeanosv1alpha1 "github.com/gramLabs/okeanos/pkg/apis/okeanos/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// StabilityError indicates that the cluster has not reached a sufficiently stable state
type StabilityError struct {
	// The reference to the object that has not yet stabilized
	TargetRef corev1.ObjectReference
	// The minimum amount of time until the object is expected to stabilize, if left unspecified there is no expectation of stability
	RetryAfter time.Duration
}

func (e *StabilityError) Error() string {
	// TODO Make something nice
	return "not stable"
}

// Iterates over all of the supplied patches and ensures that the targets are in a "stable" state (where "stable"
// is determined by the object kind).
func waitForStableState(r client.Reader, ctx context.Context, patches []okeanosv1alpha1.PatchOperation) error {
	for _, p := range patches {
		switch p.TargetRef.Kind {
		case "StatefulSet":
			ss := &appsv1.StatefulSet{}
			if err, ok := get(r, ctx, p.TargetRef, ss); err != nil {
				if ok {
					continue
				}
				return err
			}

			// TODO We also need to check for errors, if there are failures we never launch the job

			if ss.Status.ReadyReplicas < ss.Status.Replicas {
				return &StabilityError{TargetRef: p.TargetRef, RetryAfter: 5 * time.Second}
			}

		case "Deployment":
			d := &appsv1.Deployment{}
			if err, ok := get(r, ctx, p.TargetRef, d); err != nil {
				if ok {
					continue
				}
				return err
			}

			for _, c := range d.Status.Conditions {
				if c.Type == appsv1.DeploymentReplicaFailure {
					return &StabilityError{TargetRef: p.TargetRef}
				}
			}

			if d.Status.ReadyReplicas < d.Status.Replicas {
				return &StabilityError{TargetRef: p.TargetRef, RetryAfter: 5 * time.Second}
			}
		}
	}
	return nil
}

// Helper that executes a Get and checks for ignorable errors
func get(r client.Reader, ctx context.Context, ref corev1.ObjectReference, obj runtime.Object) (error, bool) {
	if err := r.Get(ctx, client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}, obj); err != nil {
		if errors.IsNotFound(err) {
			return err, true
		}
		return err, false
	}
	return nil, true
}