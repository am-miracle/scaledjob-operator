package controller

import (
	"strings"
	"testing"

	k8sbatchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	batchv1 "github.com/am-miracle/scaledjob-operator/api/v1"
)

func jobWithCondition(condType k8sbatchv1.JobConditionType, status corev1.ConditionStatus) k8sbatchv1.Job {
	return k8sbatchv1.Job{
		Status: k8sbatchv1.JobStatus{
			Conditions: []k8sbatchv1.JobCondition{
				{Type: condType, Status: status},
			},
		},
	}
}

func makeSJ(name, namespace string) *batchv1.ScaledJob {
	return &batchv1.ScaledJob{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: batchv1.ScaledJobSpec{
			QueueName:    "q",
			RedisAddress: "redis:6379",
			Threshold:    10,
			MaxReplicas:  5,
			JobTemplate: k8sbatchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "worker"}},
				Spec: k8sbatchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers:    []corev1.Container{{Name: "w", Image: "busybox"}},
							RestartPolicy: corev1.RestartPolicyNever,
						},
					},
				},
			},
		},
	}
}

func TestBuildJob(t *testing.T) {
	sj := makeSJ("my-scaledjob", "production")
	job := buildJob(sj)

	if job.Namespace != sj.Namespace {
		t.Errorf("namespace = %q, want %q", job.Namespace, sj.Namespace)
	}
	if !strings.HasPrefix(job.GenerateName, sj.Name+"-") {
		t.Errorf("GenerateName = %q, want prefix %q", job.GenerateName, sj.Name+"-")
	}
	if job.Labels[labelOwner] != sj.Name {
		t.Errorf("label %q = %q, want %q", labelOwner, job.Labels[labelOwner], sj.Name)
	}
	if job.Labels["app"] != "worker" {
		t.Errorf("template label app = %q, want %q", job.Labels["app"], "worker")
	}
	if len(job.Spec.Template.Spec.Containers) != 1 {
		t.Errorf("containers = %d, want 1", len(job.Spec.Template.Spec.Containers))
	}
}

func TestIsTerminalJob(t *testing.T) {
	tests := []struct {
		name     string
		job      k8sbatchv1.Job
		expected bool
	}{
		{
			name:     "running job is not terminal",
			job:      k8sbatchv1.Job{},
			expected: false,
		},
		{
			name:     "Complete=True is terminal",
			job:      jobWithCondition(k8sbatchv1.JobComplete, corev1.ConditionTrue),
			expected: true,
		},
		{
			name:     "Failed=True is terminal",
			job:      jobWithCondition(k8sbatchv1.JobFailed, corev1.ConditionTrue),
			expected: true,
		},
		{
			name:     "Complete=False is not terminal",
			job:      jobWithCondition(k8sbatchv1.JobComplete, corev1.ConditionFalse),
			expected: false,
		},
		{
			name:     "Failed=False is not terminal",
			job:      jobWithCondition(k8sbatchv1.JobFailed, corev1.ConditionFalse),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTerminalJob(tt.job)
			if got != tt.expected {
				t.Errorf("isTerminalJob() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCountActiveJobs(t *testing.T) {
	running := k8sbatchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "running"}}
	completed := jobWithCondition(k8sbatchv1.JobComplete, corev1.ConditionTrue)
	failed := jobWithCondition(k8sbatchv1.JobFailed, corev1.ConditionTrue)

	tests := []struct {
		name     string
		jobs     k8sbatchv1.JobList
		expected int32
	}{
		{
			name:     "running job counts as active",
			jobs:     k8sbatchv1.JobList{Items: []k8sbatchv1.Job{running}},
			expected: 1,
		},
		{
			name:     "completed job does not count",
			jobs:     k8sbatchv1.JobList{Items: []k8sbatchv1.Job{completed}},
			expected: 0,
		},
		{
			name:     "failed job does not count",
			jobs:     k8sbatchv1.JobList{Items: []k8sbatchv1.Job{failed}},
			expected: 0,
		},
		{
			name:     "mixed list returns only non-terminal jobs",
			jobs:     k8sbatchv1.JobList{Items: []k8sbatchv1.Job{running, completed, failed, running}},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countActiveJobs(tt.jobs)
			if got != tt.expected {
				t.Errorf("countActiveJobs() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestControllerOwnedJobs(t *testing.T) {
	owner := makeSJ("current", "default")
	owner.UID = types.UID("current-uid")

	owned := k8sbatchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "owned",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         batchv1.GroupVersion.String(),
					Kind:               "ScaledJob",
					Name:               owner.Name,
					UID:                owner.UID,
					Controller:         ptrBool(true),
					BlockOwnerDeletion: ptrBool(true),
				},
			},
		},
	}
	staleSameName := k8sbatchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "stale",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         batchv1.GroupVersion.String(),
					Kind:               "ScaledJob",
					Name:               owner.Name,
					UID:                types.UID("old-uid"),
					Controller:         ptrBool(true),
					BlockOwnerDeletion: ptrBool(true),
				},
			},
		},
	}
	unowned := k8sbatchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "unowned"}}

	got := controllerOwnedJobs(k8sbatchv1.JobList{
		Items: []k8sbatchv1.Job{owned, staleSameName, unowned},
	}, owner)

	if len(got.Items) != 1 {
		t.Fatalf("owned jobs = %d, want 1", len(got.Items))
	}
	if got.Items[0].Name != "owned" {
		t.Errorf("owned job = %q, want %q", got.Items[0].Name, "owned")
	}
}

func ptrBool(v bool) *bool { return &v }
