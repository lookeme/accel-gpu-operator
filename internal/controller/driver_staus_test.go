package controller

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	accelv1alpha1 "github.com/lookeme/accel-gpu-operator/api/v1alpha1"
)

func gs(selected, desired, ready, updated int32) accelv1alpha1.DriverGroupStatus {
	return accelv1alpha1.DriverGroupStatus{
		SelectedNodes: selected, DesiredPods: desired, ReadyPods: ready, UpdatedPods: updated,
	}
}

func TestReadyCondition(t *testing.T) {
	tests := []struct {
		name       string
		groups     []accelv1alpha1.DriverGroupStatus
		observed   bool
		wantStatus metav1.ConditionStatus
		wantReason string
	}{
		{"no groups", nil, true, metav1.ConditionFalse, "NoMatchingNodes"},
		{"baseline all ready", []accelv1alpha1.DriverGroupStatus{gs(2, 2, 2, 2), gs(1, 1, 1, 1)},
			true, metav1.ConditionTrue, "AllPodsReady"},
		{"pods starting", []accelv1alpha1.DriverGroupStatus{gs(2, 2, 1, 2), gs(1, 1, 1, 1)},
			true, metav1.ConditionFalse, "RolloutInProgress"},
		{"template not yet observed", []accelv1alpha1.DriverGroupStatus{gs(2, 2, 2, 2)},
			false, metav1.ConditionFalse, "RolloutInProgress"},
		{"old pods still running", []accelv1alpha1.DriverGroupStatus{gs(2, 2, 2, 1)},
			true, metav1.ConditionFalse, "RolloutInProgress"},
		{"all selected nodes unschedulable", []accelv1alpha1.DriverGroupStatus{gs(1, 0, 0, 0)},
			true, metav1.ConditionFalse, "NoSchedulablePods"},
		{"one node NotReady, rest healthy", []accelv1alpha1.DriverGroupStatus{gs(2, 1, 1, 1)},
			true, metav1.ConditionTrue, "AllPodsReady"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := readyCondition(tt.groups, tt.observed)
			if c.Status != tt.wantStatus || c.Reason != tt.wantReason {
				t.Errorf("got %s/%s (%q), want %s/%s", c.Status, c.Reason, c.Message, tt.wantStatus, tt.wantReason)
			}
		})
	}
}
