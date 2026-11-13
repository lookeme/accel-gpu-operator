package controller

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	accelv1alpha1 "github.com/lookeme/accel-gpu-operator/api/v1alpha1"
	"github.com/lookeme/accel-gpu-operator/internal/nodematch"
)

func testDriver() *accelv1alpha1.AccelDriver {
	return &accelv1alpha1.AccelDriver{
		ObjectMeta: metav1.ObjectMeta{Name: "production"},
		Spec: accelv1alpha1.AccelDriverSpec{
			Version: "1.2.0",
			NodeSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"gpu": "true"},
			},
		},
	}
}

var rhel = nodematch.Group{Key: "rhel-9", OSID: "rhel", OSVersion: "9"}

func TestBuildDriverDaemonSet(t *testing.T) {
	ds := buildDriverDaemonSet(testDriver(), rhel, "accel-system", "registry.example.com/accel-driver", false)

	if got, want := ds.Name, "accel-driver-production-rhel-9"; got != want {
		t.Errorf("name = %q, want %q", got, want)
	}
	if got, want := ds.Spec.Template.Spec.Containers[0].Image,
		"registry.example.com/accel-driver:1.2.0-rhel-9"; got != want {
		t.Errorf("image = %q, want %q", got, want)
	}
	ns := ds.Spec.Template.Spec.NodeSelector
	if ns["gpu"] != "true" || ns[nodematch.LabelOSID] != "rhel" || ns[nodematch.LabelOSVersion] != "9" {
		t.Errorf("nodeSelector = %v", ns)
	}
	if _, ok := ds.Spec.Selector.MatchLabels["version"]; ok {
		t.Errorf("selector must not contain version (immutable field)")
	}
}

func TestBuildDriverDaemonSetFake(t *testing.T) {
	ds := buildDriverDaemonSet(testDriver(), rhel, "accel-system", "unused", true)
	c := ds.Spec.Template.Spec.Containers[0]

	if c.Image != fakeDriverImage {
		t.Errorf("image = %q, want %q", c.Image, fakeDriverImage)
	}

	env := map[string]string{}
	for _, e := range c.Env {
		env[e.Name] = e.Value
	}
	if env["DRIVER_VERSION"] != "1.2.0" || env["DRIVER_GROUP"] != "rhel-9" {
		t.Errorf("env = %v", env)
	}

	if c.ReadinessProbe == nil || c.ReadinessProbe.Exec == nil {
		t.Fatalf("readiness probe missing")
	}
	if c.ReadinessProbe.TimeoutSeconds == 0 || c.ReadinessProbe.FailureThreshold == 0 {
		t.Errorf("probe defaults must be explicit to avoid update loop: %+v", c.ReadinessProbe)
	}
}
