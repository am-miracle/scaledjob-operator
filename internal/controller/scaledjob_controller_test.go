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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	kbatchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	batchv1 "github.com/am-miracle/scaledjob-operator/api/v1"
	"github.com/am-miracle/scaledjob-operator/internal/queue"
)

var _ = Describe("ScaledJob Controller", func() {
	const namespace = "default"

	ctx := context.Background()

	// resourceName and namespacedName are unique per spec so tests are fully
	// isolated without needing to block on garbage collection between specs.
	var (
		resourceName   string
		namespacedName types.NamespacedName
	)

	BeforeEach(func() {
		resourceName = fmt.Sprintf("test-%d", time.Now().UnixNano())
		namespacedName = types.NamespacedName{Name: resourceName, Namespace: namespace}
	})

	newReconciler := func(depth int64, queueErr error) *ScaledJobReconciler {
		return &ScaledJobReconciler{
			Client:       k8sClient,
			Scheme:       k8sClient.Scheme(),
			QueueFactory: &queue.FakeFactory{Client: &queue.FakeClient{DepthValue: depth, Err: queueErr}},
			Clock:        metav1.Now,
		}
	}

	defaultSpec := batchv1.ScaledJobSpec{
		QueueName:    "test-queue",
		RedisAddress: "redis:6379",
		Threshold:    10,
		MaxReplicas:  5,
		JobTemplate: kbatchv1.JobTemplateSpec{
			Spec: kbatchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers:    []corev1.Container{{Name: "worker", Image: "busybox:latest"}},
						RestartPolicy: corev1.RestartPolicyNever,
					},
				},
			},
		},
	}

	createSJ := func(spec batchv1.ScaledJobSpec) {
		sj := &batchv1.ScaledJob{
			ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: namespace},
			Spec:       spec,
		}
		Expect(k8sClient.Create(ctx, sj)).To(Succeed())
	}

	// createJob pre-creates a Job owned by the test ScaledJob.
	// Pass a non-empty condType to mark the job terminal.
	//
	// Kubernetes 1.31+ enforces condition ordering:
	//   Complete requires SuccessCriteriaMet=True to be set first.
	//   Failed    requires FailureTarget=True to be set first.
	// Both also require startTime; Complete additionally requires completionTime.
	createJob := func(condType kbatchv1.JobConditionType) {
		sj := &batchv1.ScaledJob{}
		Expect(k8sClient.Get(ctx, namespacedName, sj)).To(Succeed())

		job := &kbatchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: resourceName + "-",
				Namespace:    namespace,
				Labels:       map[string]string{labelOwner: resourceName},
			},
			Spec: kbatchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers:    []corev1.Container{{Name: "worker", Image: "busybox:latest"}},
						RestartPolicy: corev1.RestartPolicyNever,
					},
				},
			},
		}
		Expect(ctrl.SetControllerReference(sj, job, k8sClient.Scheme())).To(Succeed())
		Expect(k8sClient.Create(ctx, job)).To(Succeed())

		if condType != "" {
			now := metav1.Now()
			job.Status.StartTime = &now

			switch condType {
			case kbatchv1.JobComplete:
				job.Status.CompletionTime = &now
				job.Status.Conditions = []kbatchv1.JobCondition{
					{Type: kbatchv1.JobSuccessCriteriaMet, Status: corev1.ConditionTrue},
					{Type: kbatchv1.JobComplete, Status: corev1.ConditionTrue},
				}
			case kbatchv1.JobFailed:
				job.Status.Conditions = []kbatchv1.JobCondition{
					{Type: kbatchv1.JobFailureTarget, Status: corev1.ConditionTrue},
					{Type: kbatchv1.JobFailed, Status: corev1.ConditionTrue},
				}
			}
			Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())
		}
	}

	listOwned := func() []kbatchv1.Job {
		var jl kbatchv1.JobList
		Expect(k8sClient.List(ctx, &jl,
			client.InNamespace(namespace),
			client.MatchingLabels{labelOwner: resourceName},
		)).To(Succeed())
		return jl.Items
	}

	// AfterEach issues best-effort deletes. Because resourceName is unique
	// per spec, leftovers don't affect subsequent tests.
	AfterEach(func() {
		_ = k8sClient.DeleteAllOf(ctx, &kbatchv1.Job{},
			client.InNamespace(namespace),
			client.MatchingLabels{labelOwner: resourceName},
		)
		sj := &batchv1.ScaledJob{}
		if err := k8sClient.Get(ctx, namespacedName, sj); err == nil {
			_ = k8sClient.Delete(ctx, sj)
		}
	})

	It("missing ScaledJob returns no error", func() {
		_, err := newReconciler(0, nil).Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: "does-not-exist", Namespace: namespace},
		})
		Expect(err).NotTo(HaveOccurred())
	})

	Context("with a ScaledJob", func() {
		BeforeEach(func() { createSJ(defaultSpec) })

		It("queue depth 0 creates no Jobs when minReplicas is 0", func() {
			_, err := newReconciler(0, nil).Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(listOwned()).To(BeEmpty())
		})

		It("queue depth above threshold creates expected Jobs", func() {
			// depth=25, threshold=10 → ceil(25/10) = 3
			_, err := newReconciler(25, nil).Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(listOwned()).To(HaveLen(3))
		})

		It("existing active Jobs reduce how many new Jobs are created", func() {
			createJob("") // one already-running Job
			// depth=30 → desired=3, active=1, should create 2 more
			_, err := newReconciler(30, nil).Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(listOwned()).To(HaveLen(3))
		})

		It("same-label Jobs without matching controller owner are ignored", func() {
			staleJob := &kbatchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: resourceName + "-",
					Namespace:    namespace,
					Labels:       map[string]string{labelOwner: resourceName},
				},
				Spec: kbatchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers:    []corev1.Container{{Name: "worker", Image: "busybox:latest"}},
							RestartPolicy: corev1.RestartPolicyNever,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, staleJob)).To(Succeed())

			// depth=10 → desired=1. The stale same-label Job must not count as active.
			_, err := newReconciler(10, nil).Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(listOwned()).To(HaveLen(2))

			updated := &batchv1.ScaledJob{}
			Expect(k8sClient.Get(ctx, namespacedName, updated)).To(Succeed())
			Expect(updated.Status.ActiveJobs).To(Equal(int32(1)))
		})

		It("completed Jobs do not count as active", func() {
			createJob(kbatchv1.JobComplete)
			createJob(kbatchv1.JobComplete)
			// depth=20 → desired=2, active=0, should create 2 new
			_, err := newReconciler(20, nil).Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(listOwned()).To(HaveLen(4)) // 2 completed + 2 new
		})

		It("failed Jobs do not count as active", func() {
			createJob(kbatchv1.JobFailed)
			// depth=10 → desired=1, active=0, should create 1 new
			_, err := newReconciler(10, nil).Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(listOwned()).To(HaveLen(2)) // 1 failed + 1 new
		})

		It("status records queueDepth, activeJobs, and desiredJobs", func() {
			_, err := newReconciler(25, nil).Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
			Expect(err).NotTo(HaveOccurred())

			updated := &batchv1.ScaledJob{}
			Expect(k8sClient.Get(ctx, namespacedName, updated)).To(Succeed())
			Expect(updated.Status.QueueDepth).To(Equal(int64(25)))
			Expect(updated.Status.DesiredJobs).To(Equal(int32(3)))
			Expect(updated.Status.ActiveJobs).To(Equal(int32(3)))
		})

		It("Redis error sets QueueConnected=False, Ready=False, and requeues sooner", func() {
			_, err := newReconciler(0, nil).Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
			Expect(err).NotTo(HaveOccurred())

			result, err := newReconciler(0, fmt.Errorf("connection refused")).Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically("<", 30*time.Second))

			updated := &batchv1.ScaledJob{}
			Expect(k8sClient.Get(ctx, namespacedName, updated)).To(Succeed())
			queueCond := apimeta.FindStatusCondition(updated.Status.Conditions, condQueueConnected)
			Expect(queueCond).NotTo(BeNil())
			Expect(queueCond.Status).To(Equal(metav1.ConditionFalse))
			readyCond := apimeta.FindStatusCondition(updated.Status.Conditions, condReady)
			Expect(readyCond).NotTo(BeNil())
			Expect(readyCond.Status).To(Equal(metav1.ConditionFalse))
			Expect(readyCond.Reason).To(Equal("QueueUnavailable"))
		})

		Context("status conditions", func() {
			reconcileOK := func() *batchv1.ScaledJob {
				_, err := newReconciler(0, nil).Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
				Expect(err).NotTo(HaveOccurred())
				updated := &batchv1.ScaledJob{}
				Expect(k8sClient.Get(ctx, namespacedName, updated)).To(Succeed())
				return updated
			}

			It("successful reconcile sets QueueConnected=True", func() {
				updated := reconcileOK()
				cond := apimeta.FindStatusCondition(updated.Status.Conditions, condQueueConnected)
				Expect(cond).NotTo(BeNil())
				Expect(cond.Status).To(Equal(metav1.ConditionTrue))
				Expect(cond.Reason).To(Equal("Connected"))
			})

			It("successful reconcile sets Ready=True", func() {
				updated := reconcileOK()
				cond := apimeta.FindStatusCondition(updated.Status.Conditions, condReady)
				Expect(cond).NotTo(BeNil())
				Expect(cond.Status).To(Equal(metav1.ConditionTrue))
				Expect(cond.Reason).To(Equal("Reconciled"))
			})

			It("condition ObservedGeneration matches the ScaledJob generation", func() {
				updated := reconcileOK()
				for _, cond := range updated.Status.Conditions {
					Expect(cond.ObservedGeneration).To(Equal(updated.Generation),
						"condition %s has wrong ObservedGeneration", cond.Type)
				}
			})
		})
	})
})
