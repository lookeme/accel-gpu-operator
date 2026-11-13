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

	"github.com/lookeme/accel-gpu-operator/internal/nodematch"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	accelv1alpha1 "github.com/lookeme/accel-gpu-operator/api/v1alpha1"
)

// AccelDriverReconciler reconciles a AccelDriver object
type AccelDriverReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	Namespace  string
	DriverRepo string
	FakeDriver bool
}

// +kubebuilder:rbac:groups=accel.example.com,resources=acceldrivers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=accel.example.com,resources=acceldrivers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=accel.example.com,resources=acceldrivers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AccelDriver object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.25.0/pkg/reconcile
func (r *AccelDriverReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var driver accelv1alpha1.AccelDriver

	if err := r.Get(ctx, req.NamespacedName, &driver); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	sel, err := metav1.LabelSelectorAsSelector(driver.Spec.NodeSelector)

	if err != nil {
		log.Error(err, "invalid nodeSelector; leaving existing DaemonSets untouched")
		// повтор не поможет — сообщаем пользователю через status
		return ctrl.Result{}, r.updateStatus(ctx, &driver, driver.Status, metav1.Condition{
			Type: condReady, Status: metav1.ConditionFalse,
			Reason: "InvalidSelector", Message: err.Error(),
		})
	}

	var nodes corev1.NodeList
	if err := r.List(ctx, &nodes, client.MatchingLabelsSelector{Selector: sel}); err != nil {
		log.Error(err, "unable to list nodes")
	}

	groups, unknown := nodematch.GroupByOS(nodes.Items)

	desiredNames := make(map[string]struct{}, len(groups)) // Go-шный HashSet
	groupStatuses := make([]accelv1alpha1.DriverGroupStatus, 0, len(groups))
	allObserved := true
	// 4. Пока только показываем, что БЫ создали
	for _, g := range groups {
		desired := buildDriverDaemonSet(&driver, g, r.Namespace, r.DriverRepo, r.FakeDriver)
		desiredNames[desired.Name] = struct{}{}

		ds, op, err := r.applyDriverDaemonSet(ctx, &driver, desired)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("apply DaemonSet %s: %w", desired.Name, err)
		}
		log.Info("driver DaemonSet", "name", ds.Name, "op", op, "nodes", g.Nodes)

		groupStatuses = append(groupStatuses, groupStatus(g, ds))
		if !dsObserved(ds) {
			allObserved = false
		}
	}
	// 5. Удалить устаревшие
	if err := r.deleteStaleDaemonSets(ctx, &driver, desiredNames); err != nil {
		return ctrl.Result{}, err
	}
	if len(unknown) > 0 {
		log.Info("selected nodes without OS labels, skipped", "nodes", unknown)
	}
	// 5. Удалить наши DaemonSet'ы, чьих групп больше нет
	if err := r.deleteStaleDaemonSets(ctx, &driver, desiredNames); err != nil {
		return ctrl.Result{}, err
	}

	// 6. Status
	st := accelv1alpha1.AccelDriverStatus{
		SelectedNodes: int32(len(nodes.Items)),
		Groups:        groupStatuses,
	}
	for _, g := range groupStatuses {
		st.DesiredPods += g.DesiredPods
		st.ReadyPods += g.ReadyPods
	}
	if err := r.updateStatus(ctx, &driver, st,
		readyCondition(groupStatuses, allObserved),
		degradedCondition(unknown),
	); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("reconcile done",
		"selectedNodes", st.SelectedNodes, "groups", len(groups),
		"readyPods", st.ReadyPods, "desiredPods", st.DesiredPods)
	return ctrl.Result{}, nil
}

// applyDriverDaemonSet создаёт DaemonSet или обновляет только наши поля в существующем.
func (r *AccelDriverReconciler) applyDriverDaemonSet(
	ctx context.Context,
	driver *accelv1alpha1.AccelDriver,
	desired *appsv1.DaemonSet,
) (*appsv1.DaemonSet, controllerutil.OperationResult, error) {
	ds := &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{
		Name:      desired.Name,
		Namespace: desired.Namespace,
	}}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, ds, func() error {
		ds.Labels = desired.Labels

		// spec.selector неизменяем — задаём только при создании
		if ds.CreationTimestamp.IsZero() {
			ds.Spec.Selector = desired.Spec.Selector
		}

		ds.Spec.Template.Labels = desired.Spec.Template.Labels
		ds.Spec.Template.Spec.NodeSelector = desired.Spec.Template.Spec.NodeSelector

		// Меняем только свои поля контейнера, не заменяя его целиком:
		// иначе defaults API-сервера дают бесконечный цикл update → event → reconcile.
		if len(ds.Spec.Template.Spec.Containers) != 1 {
			ds.Spec.Template.Spec.Containers = []corev1.Container{{}}
		}
		c := &ds.Spec.Template.Spec.Containers[0]
		want := desired.Spec.Template.Spec.Containers[0]
		c.Name = want.Name
		c.Image = want.Image
		c.Command = want.Command
		c.Env = want.Env
		c.ReadinessProbe = want.ReadinessProbe

		// ownerReference: GC при удалении CR + события для Owns()
		return controllerutil.SetControllerReference(driver, ds, r.Scheme)
	})

	return ds, op, err
}

// deleteStaleDaemonSets удаляет DaemonSet'ы этого CR, имён которых нет в keep.
func (r *AccelDriverReconciler) deleteStaleDaemonSets(
	ctx context.Context,
	driver *accelv1alpha1.AccelDriver,
	keep map[string]struct{},
) error {
	log := logf.FromContext(ctx)

	var existing appsv1.DaemonSetList
	if err := r.List(ctx, &existing,
		client.InNamespace(r.Namespace),
		client.MatchingLabels{labelManagedBy: managerName, labelDriver: driver.Name},
	); err != nil {
		return err
	}

	for i := range existing.Items {
		ds := &existing.Items[i]
		if _, ok := keep[ds.Name]; ok {
			continue
		}
		// лейблы может поставить кто угодно — удаляем только то, чем владеет этот CR
		if !metav1.IsControlledBy(ds, driver) {
			log.Info("stale-looking DaemonSet not controlled by us, skipping", "name", ds.Name)
			continue
		}
		if err := r.Delete(ctx, ds); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("delete DaemonSet %s: %w", ds.Name, err)
		}
		log.Info("driver DaemonSet", "name", ds.Name, "op", "deleted")
	}
	return nil
}

// nodeToDrivers: любое изменение Node → перепроверить ВСЕ AccelDriver.
// Только совпавшие возвращать нельзя: нода, потерявшая лейбл,
// уже ни с чем не совпадает, и её старый драйвер никто бы не перепроверил.
func (r *AccelDriverReconciler) nodeToDrivers(ctx context.Context, _ client.Object) []reconcile.Request {
	var list accelv1alpha1.AccelDriverList
	if err := r.List(ctx, &list); err != nil {
		logf.FromContext(ctx).Error(err, "list AccelDrivers")
		return nil
	}
	reqs := make([]reconcile.Request, 0, len(list.Items))
	for _, d := range list.Items {
		reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: d.Name}})
	}
	return reqs
}

// updateStatus пишет status, только если он реально изменился.
// Иначе каждая запись → событие For → Reconcile → запись → ... (та же ловушка цикла).
func (r *AccelDriverReconciler) updateStatus(
	ctx context.Context,
	driver *accelv1alpha1.AccelDriver,
	newStatus accelv1alpha1.AccelDriverStatus,
	conds ...metav1.Condition,
) error {
	old := driver.Status.DeepCopy()

	// условия берём старые: SetStatusCondition меняет lastTransitionTime,
	// только когда Status действительно сменился
	newStatus.Conditions = driver.Status.Conditions
	for _, c := range conds {
		c.ObservedGeneration = driver.Generation
		meta.SetStatusCondition(&newStatus.Conditions, c)
	}
	newStatus.ObservedGeneration = driver.Generation

	if equality.Semantic.DeepEqual(*old, newStatus) {
		return nil
	}
	driver.Status = newStatus
	return r.Status().Update(ctx, driver)
}

func (r *AccelDriverReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&accelv1alpha1.AccelDriver{}).
		Watches(&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(r.nodeToDrivers)).
		Named("acceldriver").
		Complete(r)
}
