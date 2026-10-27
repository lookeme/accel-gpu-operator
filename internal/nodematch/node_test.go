package nodematch

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func node(name, osID, osVer string) corev1.Node {
	labels := map[string]string{}
	if osID != "" {
		labels[LabelOSID] = osID
	}
	if osVer != "" {
		labels[LabelOSVersion] = osVer
	}
	return corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
}

func TestGroupByOS(t *testing.T) {
	baseline := []Group{
		{Key: "rhel-9", OSID: "rhel", OSVersion: "9", Nodes: []string{"worker-4"}},
		{Key: "ubuntu-24.04", OSID: "ubuntu", OSVersion: "24.04",
			Nodes: []string{"worker-1", "worker-2", "worker-3"}},
	}

	tests := []struct {
		name        string
		nodes       []corev1.Node
		wantGroups  []Group
		wantUnknown []string
	}{
		{
			name: "five-node baseline (worker-5 already filtered by selector)",
			nodes: []corev1.Node{
				node("worker-1", "ubuntu", "24.04"),
				node("worker-2", "ubuntu", "24.04"),
				node("worker-3", "ubuntu", "24.04"),
				node("worker-4", "rhel", "9"),
			},
			wantGroups: baseline,
		},
		{
			name: "input order does not matter",
			nodes: []corev1.Node{
				node("worker-4", "rhel", "9"),
				node("worker-3", "ubuntu", "24.04"),
				node("worker-1", "ubuntu", "24.04"),
				node("worker-2", "ubuntu", "24.04"),
			},
			wantGroups: baseline,
		},
		{
			name: "missing OS labels go to unknown",
			nodes: []corev1.Node{
				node("worker-y", "", ""),
				node("worker-1", "ubuntu", "24.04"),
				node("worker-x", "ubuntu", ""),
			},
			wantGroups: []Group{
				{Key: "ubuntu-24.04", OSID: "ubuntu", OSVersion: "24.04", Nodes: []string{"worker-1"}},
			},
			wantUnknown: []string{"worker-x", "worker-y"},
		},
		{
			name: "no nodes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotGroups, gotUnknown := GroupByOS(tt.nodes)
			if !reflect.DeepEqual(gotGroups, tt.wantGroups) {
				t.Errorf("groups:\n got  %+v\n want %+v", gotGroups, tt.wantGroups)
			}
			if !reflect.DeepEqual(gotUnknown, tt.wantUnknown) {
				t.Errorf("unknown:\n got  %v\n want %v", gotUnknown, tt.wantUnknown)
			}
		})
	}
}
