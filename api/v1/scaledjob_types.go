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

package v1

import (
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ScaledJobSpec defines the desired state of ScaledJob

// +kubebuilder:printcolumn:name="Queue",type=string,JSONPath=".spec.queueName"
// +kubebuilder:printcolumn:name="Depth",type=integer,JSONPath=".status.queueDepth"
// +kubebuilder:printcolumn:name="Active",type=integer,JSONPath=".status.activeJobs"
// +kubebuilder:printcolumn:name="Desired",type=integer,JSONPath=".status.desiredJobs"
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status"
type ScaledJobSpec struct {
	// QueueName is the Redis list key to monitor.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	QueueName string `json:"queueName"`

	// RedisAddress is the host:port of the Redis instance.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	RedisAddress string `json:"redisAddress"`

	// Threshold is the queue depth per Job.
	// At depth 50 with threshold 10, the operator targets 5 Jobs.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	Threshold int32 `json:"threshold"`

	// MinReplicas is the minimum number of Jobs to maintain.
	// Defaults to 0 — no Jobs run when the queue is empty.
	// +kubebuilder:default=0
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// MaxReplicas is the hard ceiling on Jobs created.
	// The operator will never exceed this regardless of queue depth.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	MaxReplicas int32 `json:"maxReplicas"`

	// JobTemplate is the Job spec to use when creating workers.
	JobTemplate batchv1.JobTemplateSpec `json:"jobTemplate"`
}

// ScaledJobStatus defines the observed state of ScaledJob.
type ScaledJobStatus struct {
	// QueueDepth is the last observed depth of the queue.
	QueueDepth int64 `json:"queueDepth"`

	// ActiveJobs is the number of Jobs currently running.
	ActiveJobs int32 `json:"activeJobs"`

	// DesiredJobs is what the operator calculated on the last reconcile.
	DesiredJobs int32 `json:"desiredJobs"`

	// LastScaleTime is when the operator last created or stopped creating Jobs.
	// +optional
	LastScaleTime *metav1.Time `json:"lastScaleTime,omitempty"`

	// Conditions follows the standard Kubernetes condition pattern.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ScaledJob is the Schema for the scaledjobs API
type ScaledJob struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ScaledJob
	// +required
	Spec ScaledJobSpec `json:"spec"`

	// status defines the observed state of ScaledJob
	// +optional
	Status ScaledJobStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ScaledJobList contains a list of ScaledJob
type ScaledJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ScaledJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ScaledJob{}, &ScaledJobList{})
}
