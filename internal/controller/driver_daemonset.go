package controller

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	accelv1alpha1 "github.com/lookeme/accel-gpu-operator/api/v1alpha1"
	"github.com/lookeme/accel-gpu-operator/internal/nodematch"
)

const (
	labelManagedBy = "app.kubernetes.io/managed-by"
	labelDriver    = "accel.example.com/driver"
	labelGroup     = "accel.example.com/group"
	managerName    = "accel-operator"
)

// buildDriverDaemonSet — чистая функция: CR + группа → желаемый DaemonSet.
// В кластер ничего не пишет.
func buildDriverDaemonSet(d *accelv1alpha1.AccelDriver, g nodematch.Group, ns, repo string) *appsv1.DaemonSet {
	// Эти лейблы стабильны: попадут в spec.selector, который НЕЛЬЗЯ менять после создания.
	// Поэтому версии драйвера здесь нет.
	selLabels := map[string]string{
		labelManagedBy: managerName,
		labelDriver:    d.Name,
		labelGroup:     g.Key,
	}

	// Куда ставить: селектор CR + OS-лейблы группы.
	// (matchExpressions из CR пока не поддерживаем — нужен nodeAffinity.)
	nodeSel := map[string]string{}
	for k, v := range d.Spec.NodeSelector.MatchLabels {
		nodeSel[k] = v
	}
	nodeSel[nodematch.LabelOSID] = g.OSID
	nodeSel[nodematch.LabelOSVersion] = g.OSVersion

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("accel-driver-%s-%s", d.Name, g.Key),
			Namespace: ns,
			Labels:    selLabels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: selLabels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: selLabels},
				Spec: corev1.PodSpec{
					NodeSelector: nodeSel,
					Containers: []corev1.Container{{
						Name:  "driver",
						Image: fmt.Sprintf("%s:%s-%s", repo, d.Spec.Version, g.Key),
					}},
				},
			},
		},
	}
}
