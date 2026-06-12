package controller

import (
	"maps"

	k8sbatchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	batchv1 "github.com/am-miracle/scaledjob-operator/api/v1"
)

const labelOwner = "scaledjob.miracle.dev/name"

// buildJob constructs a Job from the ScaledJob's template
// It does not set the controller reference — the caller must do that
// because setting an owner reference requires the reconciler's scheme
func buildJob(sj *batchv1.ScaledJob) *k8sbatchv1.Job {
	labels := make(map[string]string, len(sj.Spec.JobTemplate.Labels)+1)
	maps.Copy(labels, sj.Spec.JobTemplate.Labels)
	labels[labelOwner] = sj.Name

	return &k8sbatchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: sj.Name + "-",
			Namespace:    sj.Namespace,
			Labels:       labels,
		},
		Spec: sj.Spec.JobTemplate.Spec,
	}
}

// isTerminalJob reports whether a Job has finished, either successfully or not
// We check conditions rather than status counters because a Job can have
// Succeeded > 0 while still running (partial completions in indexed jobs)
func isTerminalJob(job k8sbatchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if c.Status != corev1.ConditionTrue {
			continue
		}
		if c.Type == k8sbatchv1.JobComplete || c.Type == k8sbatchv1.JobFailed {
			return true
		}
	}
	return false
}

func countActiveJobs(jobs k8sbatchv1.JobList) int32 {
	var count int32
	for _, job := range jobs.Items {
		if !isTerminalJob(job) {
			count++
		}
	}
	return count
}
