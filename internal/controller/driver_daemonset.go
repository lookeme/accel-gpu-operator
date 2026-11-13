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
	labelManagedBy  = "app.kubernetes.io/managed-by"
	labelDriver     = "accel.example.com/driver"
	labelGroup      = "accel.example.com/group"
	managerName     = "accel-operator"
	fakeDriverImage = "busybox:1.36" // конкретный тег → imagePullPolicy по умолчанию IfNotPresent
	readyFile       = "/tmp/driver-ready"

	validationsDir    = "/run/accel/validations"
	validationsVolume = "validations"
	driverReadyFile   = validationsDir + "/driver-ready"
)

const fakeInstallScript = `trap 'exit 0' TERM
echo "installing accel driver $DRIVER_VERSION ($DRIVER_GROUP) on $NODE_NAME ..."
sleep 10
touch ` + readyFile + `
echo "job done: accel driver $DRIVER_VERSION loaded on $NODE_NAME"
while true; do sleep 1; done`

// buildDriverDaemonSet — чистая функция: CR + группа → желаемый DaemonSet.
// В кластер ничего не пишет.
// buildDriverDaemonSet — чистая функция: CR + группа → желаемый DaemonSet.
func buildDriverDaemonSet(d *accelv1alpha1.AccelDriver, g nodematch.Group, ns, repo string, fake bool) *appsv1.DaemonSet {
	// Стабильные лейблы: попадут в неизменяемый spec.selector, поэтому без версии.
	selLabels := map[string]string{
		labelManagedBy: managerName,
		labelDriver:    d.Name,
		labelGroup:     g.Key,
	}

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
					Containers:   []corev1.Container{driverContainer(d, g, repo, fake)},
				},
			},
		},
	}
}

func driverContainer(d *accelv1alpha1.AccelDriver, g nodematch.Group, repo string, fake bool) corev1.Container {
	if !fake {
		return corev1.Container{
			Name:  "driver",
			Image: fmt.Sprintf("%s:%s-%s", repo, d.Spec.Version, g.Key),
		}
	}

	return corev1.Container{
		Name:    "driver",
		Image:   fakeDriverImage,
		Command: []string{"sh", "-c", fakeInstallScript},
		Env: []corev1.EnvVar{
			{Name: "DRIVER_VERSION", Value: d.Spec.Version},
			{Name: "DRIVER_GROUP", Value: g.Key},
			{Name: "NODE_NAME", ValueFrom: &corev1.EnvVarSource{
				// APIVersion указываем явно: иначе API-сервер допишет "v1" → вечный diff
				FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "spec.nodeName"},
			}},
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{Command: []string{"test", "-f", readyFile}},
			},
			// Все поля, которые API-сервер заполняет по умолчанию, задаём явно — та же ловушка
			PeriodSeconds:    2,
			TimeoutSeconds:   1,
			SuccessThreshold: 1,
			FailureThreshold: 3,
		},
	}
}
