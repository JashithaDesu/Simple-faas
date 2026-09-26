package controller

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	faasv1alpha1 "mini-faas/api/v1alpha1"
)

// FunctionReconciler reconciles a Function object.
type FunctionReconciler struct {
	client.Client
}

// Reconcile is the heart of the whole project: given a Function object,
// make the cluster's actual state (a Deployment) match what it should be.
// This function must be idempotent — running it 10 times with no changes
// must produce the same end state as running it once.
func (r *FunctionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 1. Fetch the Function CR that triggered this reconcile.
	var fn faasv1alpha1.Function
	if err := r.Get(ctx, req.NamespacedName, &fn); err != nil {
		if apierrors.IsNotFound(err) {
			// Function was deleted — Kubernetes garbage-collects the
			// Deployment automatically if we set an owner reference
			// (see ensureDeployment below), so there's nothing to do.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// 2. Ensure the backing Deployment exists (create it if this is the
	// first time we've seen this Function).
	dep, err := r.ensureDeployment(ctx, &fn)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := r.ensureService(ctx, &fn); err != nil {
		return ctrl.Result{}, err
	}

	// 3. Decide the desired replica count.
	desiredReplicas := *dep.Spec.Replicas
	idleTimeout := time.Duration(fn.Spec.IdleTimeoutSeconds) * time.Second
	if idleTimeout == 0 {
		idleTimeout = 5 * time.Minute
	}
	if desiredReplicas < fn.Spec.MinReplicas {
		desiredReplicas = fn.Spec.MinReplicas
	}

	if fn.Spec.TriggeredReplicas != nil && *fn.Spec.TriggeredReplicas > desiredReplicas {
		desiredReplicas = *fn.Spec.TriggeredReplicas
	}

	clearTrigger := false
	if fn.Status.LastRequestTime != nil {
		idleSince := time.Since(fn.Status.LastRequestTime.Time)
		if idleSince >= idleTimeout {
			if desiredReplicas > fn.Spec.MinReplicas {
				logger.Info("scaling function to floor after idle timeout",
					"function", fn.Name, "idleSince", idleSince)
				desiredReplicas = fn.Spec.MinReplicas
			}
			if fn.Spec.TriggeredReplicas != nil {
				clearTrigger = true
			}
		}
	}

	needsUpdate := false
	if desiredReplicas != *dep.Spec.Replicas {
		dep.Spec.Replicas = &desiredReplicas
		needsUpdate = true
	}
	container := &dep.Spec.Template.Spec.Containers[0]
	if container.Image != fn.Spec.Image {
		container.Image = fn.Spec.Image
		needsUpdate = true
	}
	if container.Ports[0].ContainerPort != fn.Spec.Port {
		container.Ports[0].ContainerPort = fn.Spec.Port
		needsUpdate = true
	}
	if needsUpdate {
		if err := r.Update(ctx, dep); err != nil {
			return ctrl.Result{}, err
		}
	}
	if clearTrigger {
		patch := client.MergeFrom(fn.DeepCopy())
		fn.Spec.TriggeredReplicas = nil
		if err := r.Patch(ctx, &fn, patch); err != nil {
			return ctrl.Result{}, err
		}
	}

	// 4. Write observed state back onto the Function's status subresource.
	// This is the "status" your controller owns — not raw pod status,
	// which Kubernetes already tracks in etcd independently.
	phase := faasv1alpha1.PhaseCold
	if dep.Status.ReadyReplicas > 0 {
		phase = faasv1alpha1.PhaseWarm
	} else if desiredReplicas > 0 {
		phase = faasv1alpha1.PhaseScaling
	}

	fn.Status.Phase = phase
	fn.Status.Replicas = dep.Status.ReadyReplicas
	fn.Status.ObservedGeneration = fn.Generation
	if err := r.Status().Update(ctx, &fn); err != nil {
		return ctrl.Result{}, err
	}

	// 5. Requeue so we re-check idle timeout even with no new events —
	// otherwise a function with no further requests would never get
	// re-evaluated for scale-down.
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// ensureService creates a ClusterIP Service for a Function if it doesn't
// already exist, so the gateway can reach the function's pods by a stable
// DNS name (<name>.<namespace>.svc.cluster.local) instead of a pod IP.
func (r *FunctionReconciler) ensureService(ctx context.Context, fn *faasv1alpha1.Function) error {
	var svc corev1.Service
	err := r.Get(ctx, types.NamespacedName{Name: fn.Name, Namespace: fn.Namespace}, &svc)
	if err == nil {
		return nil // already exists
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	newSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fn.Name,
			Namespace: fn.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(fn, faasv1alpha1.GroupVersion.WithKind("Function")),
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"faas-function": fn.Name},
			Ports: []corev1.ServicePort{
				{
					Port:       fn.Spec.Port,
					TargetPort: intstrFromInt(fn.Spec.Port),
				},
			},
		},
	}
	return r.Create(ctx, newSvc)
}

// ensureDeployment creates the backing Deployment for a Function if it
// doesn't already exist, and returns the current object either way.
func (r *FunctionReconciler) ensureDeployment(ctx context.Context, fn *faasv1alpha1.Function) (*appsv1.Deployment, error) {
	var dep appsv1.Deployment
	err := r.Get(ctx, types.NamespacedName{Name: fn.Name, Namespace: fn.Namespace}, &dep)
	if err == nil {
		return &dep, nil
	}
	if !apierrors.IsNotFound(err) {
		return nil, err
	}

	replicas := fn.Spec.MinReplicas
	newDep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fn.Name,
			Namespace: fn.Namespace,
			// Owner reference: deleting the Function auto-deletes this
			// Deployment via Kubernetes garbage collection.
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(fn, faasv1alpha1.GroupVersion.WithKind("Function")),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"faas-function": fn.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"faas-function": fn.Name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            "function",
						Image:           fn.Spec.Image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         fn.Spec.Command,
						Ports: []corev1.ContainerPort{
							{ContainerPort: fn.Spec.Port},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								TCPSocket: &corev1.TCPSocketAction{
									Port: intstrFromInt(fn.Spec.Port),
								},
							},
							InitialDelaySeconds: 1,
							PeriodSeconds:       1,
						},
					}},
				},
			},
		},
	}

	if err := r.Create(ctx, newDep); err != nil {
		return nil, err
	}
	return newDep, nil
}

func (r *FunctionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&faasv1alpha1.Function{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
