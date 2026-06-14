package controller

import (
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	batchv1 "github.com/am-miracle/scaledjob-operator/api/v1"
)

const (
	condQueueConnected = "QueueConnected"
	condReady          = "Ready"
)

// setCondition stamps ObservedGeneration from the ScaledJob onto the condition
// before writing it, so callers can't forget to set it.
func setCondition(sj *batchv1.ScaledJob, condition metav1.Condition) {
	condition.ObservedGeneration = sj.Generation
	apimeta.SetStatusCondition(&sj.Status.Conditions, condition)
}
