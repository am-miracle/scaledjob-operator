# scaledjob-operator

scaledjob-operator is a Kubernetes operator written in Go that watches a custom resource called ScaledJob and automatically creates Kubernetes Jobs in response to Redis queue depth.

When a queue fills up, the operator creates more Jobs to drain it. When the queue empties, it stops creating Jobs and lets the running ones finish. The operator is the only thing that touches the Jobs, you never scale manually.

The goal is to demonstrate a complete, production-shaped operator: a well-defined CRD, a correct reconcile loop, external state reading, and observable status.

This project was set up using [Kubebuilder](https://book.kubebuilder.io/quick-start.html).
