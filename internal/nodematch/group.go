package nodematch

import (
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

const (
	LabelOSID      = "feature.node.kubernetes.io/system-os_release.ID"
	LabelOSVersion = "feature.node.kubernetes.io/system-os_release.VERSION_ID"
)

// Group — ноды, которым подходит один и тот же образ драйвера.
type Group struct {
	Key       string   // "ubuntu-24.04" — пойдёт в имя DaemonSet
	OSID      string   // "ubuntu"
	OSVersion string   // "24.04"
	Nodes     []string // имена нод, отсортированы
}

// GroupByOS группирует ноды по OS-лейблам NFD.
// Результат детерминирован: группы по Key, ноды и unknown — по имени.
func GroupByOS(nodes []corev1.Node) (groups []Group, unknown []string) {
	byKey := map[string]*Group{}

	for _, n := range nodes {
		// чтение отсутствующего ключа (и даже nil-map) возвращает ""
		id := n.Labels[LabelOSID]
		ver := n.Labels[LabelOSVersion]
		if id == "" || ver == "" {
			unknown = append(unknown, n.Name)
			continue
		}

		key := id + "-" + ver
		g, ok := byKey[key]
		if !ok {
			g = &Group{Key: key, OSID: id, OSVersion: ver}
			byKey[key] = g
		}
		g.Nodes = append(g.Nodes, n.Name)
	}

	// обход map в Go случайный → собираем в срез и сортируем
	for _, g := range byKey {
		slices.Sort(g.Nodes)
		groups = append(groups, *g)
	}
	slices.SortFunc(groups, func(a, b Group) int { return strings.Compare(a.Key, b.Key) })
	slices.Sort(unknown)

	return groups, unknown
}
