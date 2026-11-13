package controller

import (
	"fmt"
	"strings"

	accelv1alpha1 "github.com/lookeme/accel-gpu-operator/api/v1alpha1"
	"github.com/lookeme/accel-gpu-operator/internal/nodematch"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	condReady    = "Ready"
	condDegraded = "Degraded"
)

// groupStatus сводит наш план (g) и факт Kubernetes (ds.Status) в одну запись.
func groupStatus(g nodematch.Group, ds *appsv1.DaemonSet) accelv1alpha1.DriverGroupStatus {
	return accelv1alpha1.DriverGroupStatus{
		Name:          g.Key,
		DaemonSet:     ds.Name,
		SelectedNodes: int32(len(g.Nodes)),
		DesiredPods:   ds.Status.DesiredNumberScheduled,
		ReadyPods:     ds.Status.NumberReady,
		UpdatedPods:   ds.Status.UpdatedNumberScheduled,
	}
}

// dsObserved: контроллер DaemonSet уже обработал последнюю версию шаблона.
// Иначе его status описывает ещё СТАРЫЙ шаблон, и верить цифрам нельзя.
func dsObserved(ds *appsv1.DaemonSet) bool {
	return ds.Status.ObservedGeneration >= ds.Generation
}

func readyCondition(groups []accelv1alpha1.DriverGroupStatus, allObserved bool) metav1.Condition {
	c := metav1.Condition{Type: condReady, Status: metav1.ConditionFalse}

	if len(groups) == 0 {
		c.Reason = "NoMatchingNodes"
		c.Message = "no selected nodes with known OS"
		return c
	}

	var selected, desired, ready, updated int32
	for _, g := range groups {
		selected += g.SelectedNodes
		desired += g.DesiredPods
		ready += g.ReadyPods
		updated += g.UpdatedPods
	}

	c.Message = fmt.Sprintf("%d/%d driver pods ready", ready, desired)
	if gap := selected - desired; gap > 0 {
		c.Message += fmt.Sprintf("; %d selected node(s) not scheduled (NotReady or tainted?)", gap)
	}

	switch {
	case desired == 0:
		c.Reason = "NoSchedulablePods"
	case !allObserved || updated < desired || ready < desired:
		c.Reason = "RolloutInProgress"
	default:
		c.Status = metav1.ConditionTrue
		c.Reason = "AllPodsReady"
	}
	return c
}

func degradedCondition(unknown []string) metav1.Condition {
	if len(unknown) == 0 {
		return metav1.Condition{
			Type: condDegraded, Status: metav1.ConditionFalse,
			Reason: "AsExpected", Message: "all selected nodes have OS labels",
		}
	}
	return metav1.Condition{
		Type: condDegraded, Status: metav1.ConditionTrue,
		Reason:  "UnknownOS",
		Message: "selected nodes without OS labels: " + strings.Join(unknown, ", "),
	}
}
