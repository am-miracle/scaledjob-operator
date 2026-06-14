/*
Copyright 2026.

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

package controller

import (
	"context"
	"fmt"
	"time"

	k8sbatchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	batchv1 "github.com/am-miracle/scaledjob-operator/api/v1"
	"github.com/am-miracle/scaledjob-operator/internal/queue"
)

// ScaledJobReconciler reconciles a ScaledJob object
type ScaledJobReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// QueueFactory creates a queue client per ScaledJob using the address
	// from spec.redisAddress. Injected so tests can substitute a fake.
	QueueFactory queue.Factory

	// Clock is called to get the current time when writing LastScaleTime.
	// Injected so tests can control time without sleeping.
	Clock func() metav1.Time
}

// +kubebuilder:rbac:groups=batch.miracle.dev,resources=scaledjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch.miracle.dev,resources=scaledjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch.miracle.dev,resources=scaledjobs/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

func (r *ScaledJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var sj batchv1.ScaledJob
	if err := r.Get(ctx, req.NamespacedName, &sj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Read queue depth. On failure, mark the condition and requeue sooner
	// than the normal 30s interval so we recover quickly from a blip.
	qClient := r.QueueFactory.ForAddress(sj.Spec.RedisAddress)
	depth, err := qClient.Depth(ctx, sj.Spec.QueueName)
	if err != nil {
		log.Error(err, "Failed to read queue depth", "queue", sj.Spec.QueueName)
		setCondition(&sj, metav1.Condition{
			Type:    condQueueConnected,
			Status:  metav1.ConditionFalse,
			Reason:  "RedisUnreachable",
			Message: err.Error(),
		})
		setCondition(&sj, metav1.Condition{
			Type:    condReady,
			Status:  metav1.ConditionFalse,
			Reason:  "QueueUnavailable",
			Message: "Could not read queue depth",
		})
		if statusErr := r.Status().Update(ctx, &sj); statusErr != nil {
			log.Error(statusErr, "Failed to update status after queue error")
		}
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Use the label as a cheap candidate filter, then verify controller owner
	// UID so stale same-name Jobs from a deleted ScaledJob are ignored.
	var jobList k8sbatchv1.JobList
	if err := r.List(ctx, &jobList,
		client.InNamespace(req.Namespace),
		client.MatchingLabels{labelOwner: sj.Name},
	); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing jobs: %w", err)
	}

	active := countActiveJobs(controllerOwnedJobs(jobList, &sj))
	desired := calculateDesiredJobs(depth, sj.Spec)

	log.Info("Reconciling ScaledJob",
		"queue", sj.Spec.QueueName,
		"depth", depth,
		"active", active,
		"desired", desired,
	)

	// Create Jobs until active reaches desired. We never delete running Jobs
	// when desired drops — they run to completion naturally.
	scaled := false
	for active < desired {
		job := buildJob(&sj)
		if err := ctrl.SetControllerReference(&sj, job, r.Scheme); err != nil {
			return ctrl.Result{}, fmt.Errorf("setting controller reference: %w", err)
		}
		if err := r.Create(ctx, job); err != nil {
			return ctrl.Result{}, fmt.Errorf("creating job: %w", err)
		}
		log.Info("Created Job", "job", job.GenerateName, "namespace", job.Namespace)
		active++
		scaled = true
	}

	if scaled {
		now := r.Clock()
		sj.Status.LastScaleTime = &now
	}
	sj.Status.QueueDepth = depth
	sj.Status.ActiveJobs = active
	sj.Status.DesiredJobs = desired

	setCondition(&sj, metav1.Condition{
		Type:    condQueueConnected,
		Status:  metav1.ConditionTrue,
		Reason:  "Connected",
		Message: "Successfully read queue depth",
	})
	setCondition(&sj, metav1.Condition{
		Type:    condReady,
		Status:  metav1.ConditionTrue,
		Reason:  "Reconciled",
		Message: "ScaledJob reconciled successfully",
	})

	if err := r.Status().Update(ctx, &sj); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating status: %w", err)
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ScaledJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.ScaledJob{}).
		Owns(&k8sbatchv1.Job{}).
		Named("scaledjob").
		Complete(r)
}
