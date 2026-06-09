package controller

import (
	"testing"

	batchv1 "github.com/am-miracle/scaledjob-operator/api/v1"
)

func ptr32(v int32) *int32 { return &v }

func TestCalculateDesiredJobs(t *testing.T) {
	tests := []struct {
		name     string
		depth    int64
		spec     batchv1.ScaledJobSpec
		expected int32
	}{
		{
			name:     "zero depth and no min replicas returns 0",
			depth:    0,
			spec:     batchv1.ScaledJobSpec{Threshold: 10, MaxReplicas: 10},
			expected: 0,
		},
		{
			name:     "zero depth with min replicas returns the min",
			depth:    0,
			spec:     batchv1.ScaledJobSpec{Threshold: 10, MaxReplicas: 10, MinReplicas: ptr32(2)},
			expected: 2,
		},
		{
			name:     "depth below threshold returns 1",
			depth:    5,
			spec:     batchv1.ScaledJobSpec{Threshold: 10, MaxReplicas: 10},
			expected: 1,
		},
		{
			name:     "depth exactly at threshold returns 1",
			depth:    10,
			spec:     batchv1.ScaledJobSpec{Threshold: 10, MaxReplicas: 10},
			expected: 1,
		},
		{
			name:     "depth above threshold rounds up",
			depth:    25,
			spec:     batchv1.ScaledJobSpec{Threshold: 10, MaxReplicas: 10},
			expected: 3,
		},
		{
			name:     "desired above max returns max",
			depth:    1000,
			spec:     batchv1.ScaledJobSpec{Threshold: 10, MaxReplicas: 5},
			expected: 5,
		},
		{
			name:     "min replicas greater than calculated desired returns min",
			depth:    5,
			spec:     batchv1.ScaledJobSpec{Threshold: 10, MaxReplicas: 10, MinReplicas: ptr32(3)},
			expected: 3,
		},
		{
			name:     "invalid threshold does not panic",
			depth:    50,
			spec:     batchv1.ScaledJobSpec{Threshold: 0, MaxReplicas: 10},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateDesiredJobs(tt.depth, tt.spec)
			if got != tt.expected {
				t.Errorf("calculateDesiredJobs(%d, ...) = %d, want %d", tt.depth, got, tt.expected)
			}
		})
	}
}
