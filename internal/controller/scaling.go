package controller

import (
	batchv1 "github.com/am-miracle/scaledjob-operator/api/v1"
)

// calculateDesiredJobs returns how many Jobs should be running given the current
// queue depth. It scales linearly with depth (one Job per threshold items),
// then clamps the result to [minReplicas, maxReplicas].
func calculateDesiredJobs(depth int64, spec batchv1.ScaledJobSpec) int32 {
	minReplicas := int32(0)
	if spec.MinReplicas != nil {
		minReplicas = *spec.MinReplicas
	}

	if spec.Threshold < 1 {
		return minReplicas
	}

	if depth == 0 {
		return minReplicas
	}

	// integer ceiling without floating point: (depth + threshold - 1) / threshold
	desired := int32((depth + int64(spec.Threshold) - 1) / int64(spec.Threshold))

	desired = max(desired, minReplicas)
	desired = min(desired, spec.MaxReplicas)
	return desired
}
