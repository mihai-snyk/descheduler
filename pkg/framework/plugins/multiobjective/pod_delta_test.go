package multiobjective

import (
	"testing"

	"k8s.io/klog/v2"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
)

func TestDetectPodDelta(t *testing.T) {
	// Create a mock MultiObjective instance for testing
	m := &MultiObjective{
		logger: klog.NewKlogr(),
	}

	tests := []struct {
		name        string
		oldPods     []framework.PodInfo
		currentPods []framework.PodInfo
		wantAdded   int
		wantRemoved int
	}{
		{
			name:    "Pod appears - new ReplicaSet",
			oldPods: []framework.PodInfo{},
			currentPods: []framework.PodInfo{
				{Name: "app-1", Namespace: "default", ReplicaSetName: "app-rs"},
				{Name: "app-2", Namespace: "default", ReplicaSetName: "app-rs"},
			},
			wantAdded:   2,
			wantRemoved: 0,
		},
		{
			name: "Pod disappears - ReplicaSet removed",
			oldPods: []framework.PodInfo{
				{Name: "app-1", Namespace: "default", ReplicaSetName: "app-rs"},
				{Name: "app-2", Namespace: "default", ReplicaSetName: "app-rs"},
			},
			currentPods: []framework.PodInfo{},
			wantAdded:   0,
			wantRemoved: 2,
		},
		{
			name: "Pod appears - ReplicaSet scaled up",
			oldPods: []framework.PodInfo{
				{Name: "app-1", Namespace: "default", ReplicaSetName: "app-rs"},
			},
			currentPods: []framework.PodInfo{
				{Name: "app-1", Namespace: "default", ReplicaSetName: "app-rs"},
				{Name: "app-2", Namespace: "default", ReplicaSetName: "app-rs"},
				{Name: "app-3", Namespace: "default", ReplicaSetName: "app-rs"},
			},
			wantAdded:   2,
			wantRemoved: 0,
		},
		{
			name: "Pod disappears - ReplicaSet scaled down",
			oldPods: []framework.PodInfo{
				{Name: "app-1", Namespace: "default", ReplicaSetName: "app-rs"},
				{Name: "app-2", Namespace: "default", ReplicaSetName: "app-rs"},
				{Name: "app-3", Namespace: "default", ReplicaSetName: "app-rs"},
			},
			currentPods: []framework.PodInfo{
				{Name: "app-1", Namespace: "default", ReplicaSetName: "app-rs"},
			},
			wantAdded:   0,
			wantRemoved: 2,
		},
		{
			name: "Both appear and disappear - mixed changes",
			oldPods: []framework.PodInfo{
				{Name: "frontend-1", Namespace: "default", ReplicaSetName: "frontend-rs"},
				{Name: "frontend-2", Namespace: "default", ReplicaSetName: "frontend-rs"},
				{Name: "api-1", Namespace: "default", ReplicaSetName: "api-rs"},
				{Name: "api-2", Namespace: "default", ReplicaSetName: "api-rs"},
				{Name: "api-3", Namespace: "default", ReplicaSetName: "api-rs"},
			},
			currentPods: []framework.PodInfo{
				// frontend scaled up from 2 to 4 (+2)
				{Name: "frontend-1", Namespace: "default", ReplicaSetName: "frontend-rs"},
				{Name: "frontend-2", Namespace: "default", ReplicaSetName: "frontend-rs"},
				{Name: "frontend-3", Namespace: "default", ReplicaSetName: "frontend-rs"},
				{Name: "frontend-4", Namespace: "default", ReplicaSetName: "frontend-rs"},
				// api scaled down from 3 to 1 (-2)
				{Name: "api-1", Namespace: "default", ReplicaSetName: "api-rs"},
				// new ReplicaSet cache (+2)
				{Name: "cache-1", Namespace: "default", ReplicaSetName: "cache-rs"},
				{Name: "cache-2", Namespace: "default", ReplicaSetName: "cache-rs"},
			},
			wantAdded:   4, // frontend +2, cache +2
			wantRemoved: 2, // api -2
		},
		{
			name: "No changes - same ReplicaSets with same counts",
			oldPods: []framework.PodInfo{
				{Name: "app-1", Namespace: "default", ReplicaSetName: "app-rs"},
				{Name: "app-2", Namespace: "default", ReplicaSetName: "app-rs"},
			},
			currentPods: []framework.PodInfo{
				{Name: "app-1", Namespace: "default", ReplicaSetName: "app-rs"},
				{Name: "app-2", Namespace: "default", ReplicaSetName: "app-rs"},
			},
			wantAdded:   0,
			wantRemoved: 0,
		},
		{
			name: "Multiple namespaces - changes in different namespaces",
			oldPods: []framework.PodInfo{
				{Name: "app-1", Namespace: "prod", ReplicaSetName: "app-rs"},
				{Name: "app-2", Namespace: "prod", ReplicaSetName: "app-rs"},
			},
			currentPods: []framework.PodInfo{
				{Name: "app-1", Namespace: "prod", ReplicaSetName: "app-rs"},
				{Name: "app-2", Namespace: "prod", ReplicaSetName: "app-rs"},
				{Name: "app-3", Namespace: "prod", ReplicaSetName: "app-rs"},
				{Name: "test-1", Namespace: "dev", ReplicaSetName: "test-rs"},
			},
			wantAdded:   2, // prod/app-rs +1, dev/test-rs +1
			wantRemoved: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delta := m.detectPodDelta(tt.oldPods, tt.currentPods)

			if len(delta.AddedPods) != tt.wantAdded {
				t.Errorf("detectPodDelta() added pods = %d, want %d", len(delta.AddedPods), tt.wantAdded)
			}

			if len(delta.RemovedPods) != tt.wantRemoved {
				t.Errorf("detectPodDelta() removed pods = %d, want %d", len(delta.RemovedPods), tt.wantRemoved)
			}

			// Log the actual changes for debugging
			t.Logf("Added pods: %d, Removed pods: %d", len(delta.AddedPods), len(delta.RemovedPods))
			for _, pod := range delta.AddedPods {
				t.Logf("  Added: %s/%s (%s)", pod.Namespace, pod.Name, pod.ReplicaSetName)
			}
			for _, pod := range delta.RemovedPods {
				t.Logf("  Removed: %s/%s (%s)", pod.Namespace, pod.Name, pod.ReplicaSetName)
			}
		})
	}
}

func TestHasPodChanges(t *testing.T) {
	m := &MultiObjective{
		logger: klog.NewKlogr(),
	}

	tests := []struct {
		name  string
		delta PodDelta
		want  bool
	}{
		{
			name: "No changes",
			delta: PodDelta{
				AddedPods:   []framework.PodInfo{},
				RemovedPods: []framework.PodInfo{},
			},
			want: false,
		},
		{
			name: "Only additions",
			delta: PodDelta{
				AddedPods:   []framework.PodInfo{{Name: "new-pod"}},
				RemovedPods: []framework.PodInfo{},
			},
			want: true,
		},
		{
			name: "Only removals",
			delta: PodDelta{
				AddedPods:   []framework.PodInfo{},
				RemovedPods: []framework.PodInfo{{Name: "old-pod"}},
			},
			want: true,
		},
		{
			name: "Both additions and removals",
			delta: PodDelta{
				AddedPods:   []framework.PodInfo{{Name: "new-pod"}},
				RemovedPods: []framework.PodInfo{{Name: "old-pod"}},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.hasPodChanges(tt.delta)
			if got != tt.want {
				t.Errorf("hasPodChanges() = %v, want %v", got, tt.want)
			}
		})
	}
}
