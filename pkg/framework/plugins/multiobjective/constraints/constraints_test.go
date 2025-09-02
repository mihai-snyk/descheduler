package constraints_test

import (
	"testing"

	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/constraints"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
)

func TestResourceConstraint(t *testing.T) {
	// Setup: 2 nodes with limited capacity
	nodes := []constraints.NodeInfo{
		{
			Name:         "node-1",
			CPUCapacity:  4000, // 4 CPUs
			MemCapacity:  8e9,  // 8GB
			CPUAllocated: 1000, // 1 CPU already used
			MemAllocated: 2e9,  // 2GB already used
		},
		{
			Name:         "node-2",
			CPUCapacity:  2000, // 2 CPUs
			MemCapacity:  4e9,  // 4GB
			CPUAllocated: 0,
			MemAllocated: 0,
		},
	}

	// Pods to place
	pods := []constraints.PodInfo{
		{Name: "pod-1", CPURequest: 1000, MemRequest: 2e9, CurrentNode: 0},
		{Name: "pod-2", CPURequest: 1000, MemRequest: 2e9, CurrentNode: 0},
		{Name: "pod-3", CPURequest: 1000, MemRequest: 2e9, CurrentNode: 0},
		{Name: "pod-4", CPURequest: 500, MemRequest: 1e9, CurrentNode: 1},
	}

	constraint := constraints.ResourceConstraint(pods, nodes)

	testCases := []struct {
		name       string
		assignment []int
		shouldPass bool
	}{
		{
			name:       "ValidDistribution",
			assignment: []int{0, 0, 1, 1}, // 2 pods per node
			shouldPass: true,
		},
		{
			name:       "Node1Overloaded",
			assignment: []int{0, 0, 0, 0}, // All pods on node-1
			shouldPass: false,             // Exceeds CPU capacity
		},
		{
			name:       "Node2Overloaded",
			assignment: []int{1, 1, 1, 1}, // All pods on node-2
			shouldPass: false,             // Exceeds capacity
		},
		{
			name:       "InvalidNodeIndex",
			assignment: []int{0, 1, 2, 0}, // Node 2 doesn't exist
			shouldPass: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bounds := make([]framework.IntBounds, len(pods))
			for i := range bounds {
				bounds[i] = framework.IntBounds{L: 0, H: len(nodes) - 1}
			}
			solution := framework.NewIntegerSolution(tc.assignment, bounds)

			result := constraint(solution)

			if result != tc.shouldPass {
				t.Errorf("Expected %v, got %v", tc.shouldPass, result)
			}
		})
	}
}

func TestPDBConstraint(t *testing.T) {
	// Setup: 3 replicas with maxUnavailable=1
	pods := []constraints.PodInfo{
		{
			Name:                   "app-0",
			CPURequest:             1000,
			MemRequest:             1e9,
			CurrentNode:            0,
			ReplicaSetName:         "app",
			MaxUnavailableReplicas: 1,
		},
		{
			Name:                   "app-1",
			CPURequest:             1000,
			MemRequest:             1e9,
			CurrentNode:            1,
			ReplicaSetName:         "app",
			MaxUnavailableReplicas: 1,
		},
		{
			Name:                   "app-2",
			CPURequest:             1000,
			MemRequest:             1e9,
			CurrentNode:            2,
			ReplicaSetName:         "app",
			MaxUnavailableReplicas: 1,
		},
	}

	constraint := constraints.PDBConstraint(pods)

	testCases := []struct {
		name       string
		assignment []int
		shouldPass bool
	}{
		{
			name:       "NoMovement",
			assignment: []int{0, 1, 2},
			shouldPass: true,
		},
		{
			name:       "MoveOnePod",
			assignment: []int{1, 1, 2},
			shouldPass: true,
		},
		{
			name:       "MoveTwoPods",
			assignment: []int{1, 0, 2},
			shouldPass: true, // Can be done sequentially
		},
		{
			name:       "MoveAllPods",
			assignment: []int{1, 2, 0},
			shouldPass: true, // Can be done sequentially with maxUnavailable=1
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bounds := make([]framework.IntBounds, len(pods))
			for i := range bounds {
				bounds[i] = framework.IntBounds{L: 0, H: 2}
			}
			solution := framework.NewIntegerSolution(tc.assignment, bounds)

			result := constraint(solution)

			if result != tc.shouldPass {
				t.Errorf("Expected %v, got %v", tc.shouldPass, result)
			}
		})
	}
}

func TestPDBMaxUnavailableZero(t *testing.T) {
	// Test that maxUnavailable=0 blocks all movements
	pods := []constraints.PodInfo{
		{
			Name:                   "critical-0",
			CPURequest:             1000,
			MemRequest:             1e9,
			CurrentNode:            0,
			ReplicaSetName:         "critical-app",
			MaxUnavailableReplicas: 0, // No pod can be unavailable
		},
		{
			Name:                   "critical-1",
			CPURequest:             1000,
			MemRequest:             1e9,
			CurrentNode:            1,
			ReplicaSetName:         "critical-app",
			MaxUnavailableReplicas: 0,
		},
	}

	constraint := constraints.PDBConstraint(pods)

	testCases := []struct {
		name       string
		assignment []int
		shouldPass bool
	}{
		{
			name:       "NoMovement",
			assignment: []int{0, 1},
			shouldPass: true,
		},
		{
			name:       "MoveOnePod",
			assignment: []int{1, 1},
			shouldPass: false, // Cannot move any pod
		},
		{
			name:       "SwapPods",
			assignment: []int{1, 0},
			shouldPass: false, // Cannot move any pod
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bounds := make([]framework.IntBounds, len(pods))
			for i := range bounds {
				bounds[i] = framework.IntBounds{L: 0, H: 1}
			}
			solution := framework.NewIntegerSolution(tc.assignment, bounds)

			result := constraint(solution)

			if result != tc.shouldPass {
				t.Errorf("Expected %v, got %v", tc.shouldPass, result)
			}
		})
	}
}

func TestCombinedConstraints(t *testing.T) {
	// Test combining resource and PDB constraints
	nodes := []constraints.NodeInfo{
		{Name: "node-1", CPUCapacity: 2000, MemCapacity: 4e9},
		{Name: "node-2", CPUCapacity: 2000, MemCapacity: 4e9},
	}

	pods := []constraints.PodInfo{
		{
			Name:                   "app-0",
			CPURequest:             1500,
			MemRequest:             3e9,
			CurrentNode:            0,
			ReplicaSetName:         "app",
			MaxUnavailableReplicas: 0, // Cannot move
		},
		{
			Name:        "web-0",
			CPURequest:  500,
			MemRequest:  1e9,
			CurrentNode: 1,
		},
		{
			Name:        "extra-pod",
			CPURequest:  100, // Even 100 CPU would exceed when combined
			MemRequest:  0.1e9,
			CurrentNode: 1,
		},
	}

	resourceConstraint := constraints.ResourceConstraint(pods, nodes)
	pdbConstraint := constraints.PDBConstraint(pods)
	combined := constraints.CombineConstraints(resourceConstraint, pdbConstraint)

	testCases := []struct {
		name       string
		assignment []int
		shouldPass bool
		reason     string
	}{
		{
			name:       "NoChange",
			assignment: []int{0, 1, 1},
			shouldPass: true,
			reason:     "No constraints violated",
		},
		{
			name:       "ViolatePDB",
			assignment: []int{1, 1, 1}, // Move app-0 (violates PDB)
			shouldPass: false,
			reason:     "PDB violation (maxUnavailable=0)",
		},
		{
			name:       "ExactCapacity",
			assignment: []int{0, 0, 1}, // app-0 and web-0 on node-1 (exactly at capacity)
			shouldPass: true,
			reason:     "Exactly at capacity should be allowed",
		},
		{
			name:       "ExceedCapacity",
			assignment: []int{0, 0, 0}, // All three pods on node-1 (exceeds capacity)
			shouldPass: false,
			reason:     "1500 + 500 + 100 = 2100 CPU exceeds node-1's 2000 CPU capacity",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bounds := make([]framework.IntBounds, len(pods))
			for i := range bounds {
				bounds[i] = framework.IntBounds{L: 0, H: 1}
			}
			solution := framework.NewIntegerSolution(tc.assignment, bounds)

			result := combined(solution)

			if result != tc.shouldPass {
				t.Errorf("Expected %v, got %v. Reason: %s", tc.shouldPass, result, tc.reason)
			}
		})
	}
}
