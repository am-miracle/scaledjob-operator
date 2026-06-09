package controller

import (
	k8sbatchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

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
