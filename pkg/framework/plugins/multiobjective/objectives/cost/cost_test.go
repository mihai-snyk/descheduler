package cost_test

import (
	"testing"

	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/constraints"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/objectives/cost"
)

func TestCostObjectiveNormalization(t *testing.T) {
	// Create test nodes with known costs
	nodes := []cost.NodeInfo{
		{Name: "node-1", CostPerHour: 0.10}, // $0.10/hour
		{Name: "node-2", CostPerHour: 0.20}, // $0.20/hour
		{Name: "node-3", CostPerHour: 0.30}, // $0.30/hour
		{Name: "node-4", CostPerHour: 0.40}, // $0.40/hour
	}
	// Total max cost = $1.00/hour

	pods := []constraints.PodInfo{
		{Name: "pod-1", CurrentNode: 0},
		{Name: "pod-2", CurrentNode: 0},
		{Name: "pod-3", CurrentNode: 1},
		{Name: "pod-4", CurrentNode: 2},
	}

	costObj := cost.CostObjective(pods, nodes)

	testCases := []struct {
		name          string
		assignment    []int
		expectedNodes []bool  // which nodes should be active
		expectedCost  float64 // normalized cost
		description   string
	}{
		{
			name:          "AllNodesActive",
			assignment:    []int{0, 1, 2, 3}, // one pod per node
			expectedNodes: []bool{true, true, true, true},
			expectedCost:  1.0, // 100% of infrastructure
			description:   "All nodes active should give normalized cost of 1.0",
		},
		{
			name:          "TwoNodesActive",
			assignment:    []int{0, 0, 1, 1}, // pods on first two nodes
			expectedNodes: []bool{true, true, false, false},
			expectedCost:  0.3, // ($0.10 + $0.20) / $1.00 = 0.3
			description:   "Two cheapest nodes active",
		},
		{
			name:          "SingleNodeActive",
			assignment:    []int{2, 2, 2, 2}, // all pods on node 3
			expectedNodes: []bool{false, false, true, false},
			expectedCost:  0.3, // $0.30 / $1.00 = 0.3
			description:   "Single node consolidation",
		},
		{
			name:          "ExpensiveNodesOnly",
			assignment:    []int{2, 2, 3, 3}, // pods on expensive nodes
			expectedNodes: []bool{false, false, true, true},
			expectedCost:  0.7, // ($0.30 + $0.40) / $1.00 = 0.7
			description:   "Using expensive nodes increases normalized cost",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create solution
			bounds := make([]framework.IntBounds, len(pods))
			for i := range bounds {
				bounds[i] = framework.IntBounds{L: 0, H: len(nodes) - 1}
			}
			solution := framework.NewIntegerSolution(tc.assignment, bounds)

			// Calculate cost
			normalizedCost := costObj(solution)

			t.Logf("%s:", tc.description)
			t.Logf("  Assignment: %v", tc.assignment)
			t.Logf("  Active nodes: %v", tc.expectedNodes)
			t.Logf("  Normalized cost: %.2f (expected: %.2f)", normalizedCost, tc.expectedCost)

			// Check if normalized cost matches expected
			if abs(normalizedCost-tc.expectedCost) > 0.001 {
				t.Errorf("Expected normalized cost %.2f, got %.2f", tc.expectedCost, normalizedCost)
			}

			// Verify it's in [0,1] range
			if normalizedCost < 0 || normalizedCost > 1 {
				t.Errorf("Normalized cost %.2f is outside [0,1] range", normalizedCost)
			}
		})
	}
}

func TestCostObjectiveWithRealPricing(t *testing.T) {
	// Test with actual AWS pricing
	nodes := []cost.NodeInfo{
		cost.NewNodeInfo("node-1", "us-east-1", "t3.micro", "on-demand"),  // $0.0104/hour
		cost.NewNodeInfo("node-2", "us-east-1", "t3.small", "on-demand"),  // $0.0208/hour
		cost.NewNodeInfo("node-3", "us-east-1", "m5.large", "on-demand"),  // $0.0960/hour
		cost.NewNodeInfo("node-4", "us-east-1", "m5.xlarge", "on-demand"), // $0.1920/hour
	}

	pods := []constraints.PodInfo{
		{Name: "small-app-1"},
		{Name: "small-app-2"},
		{Name: "medium-app-1"},
		{Name: "large-app-1"},
	}

	costObj := cost.CostObjective(pods, nodes)

	// All nodes active
	allActiveAssignment := []int{0, 1, 2, 3}
	bounds := make([]framework.IntBounds, len(pods))
	for i := range bounds {
		bounds[i] = framework.IntBounds{L: 0, H: len(nodes) - 1}
	}

	solution := framework.NewIntegerSolution(allActiveAssignment, bounds)
	normalizedCost := costObj(solution)

	t.Logf("Real AWS pricing test:")
	t.Logf("  t3.micro:  $%.4f/hour", nodes[0].CostPerHour)
	t.Logf("  t3.small:  $%.4f/hour", nodes[1].CostPerHour)
	t.Logf("  m5.large:  $%.4f/hour", nodes[2].CostPerHour)
	t.Logf("  m5.xlarge: $%.4f/hour", nodes[3].CostPerHour)
	t.Logf("  Total max cost: $%.4f/hour", nodes[0].CostPerHour+nodes[1].CostPerHour+nodes[2].CostPerHour+nodes[3].CostPerHour)
	t.Logf("  Normalized cost (all active): %.4f", normalizedCost)

	// Should be 1.0 when all nodes are active
	if abs(normalizedCost-1.0) > 0.001 {
		t.Errorf("Expected normalized cost 1.0 for all nodes active, got %.4f", normalizedCost)
	}

	// Consolidate to just the large instance
	consolidatedAssignment := []int{2, 2, 2, 2}
	consolidatedSolution := framework.NewIntegerSolution(consolidatedAssignment, bounds)
	consolidatedCost := costObj(consolidatedSolution)

	totalMaxCost := nodes[0].CostPerHour + nodes[1].CostPerHour + nodes[2].CostPerHour + nodes[3].CostPerHour
	expectedNormalized := nodes[2].CostPerHour / totalMaxCost

	t.Logf("  Consolidated to m5.large: %.4f (expected: %.4f)", consolidatedCost, expectedNormalized)

	if abs(consolidatedCost-expectedNormalized) > 0.001 {
		t.Errorf("Expected normalized cost %.4f, got %.4f", expectedNormalized, consolidatedCost)
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// TestShowBinPackingSolutions demonstrates the bin packing solutions for various scenarios
func TestShowBinPackingSolutions(t *testing.T) {
	scenarios := []struct {
		name  string
		nodes []cost.NodeInfo
		pods  []constraints.PodInfo
	}{
		{
			name: "MixedNodeTypes_CostFocused",
			nodes: []cost.NodeInfo{
				{Name: "t3.small-spot-1", CPUCapacity: 2000, MemCapacity: 4e9, CostPerHour: 0.0074},
				{Name: "t3.small-spot-2", CPUCapacity: 2000, MemCapacity: 4e9, CostPerHour: 0.0074},
				{Name: "m5.large", CPUCapacity: 4000, MemCapacity: 8e9, CostPerHour: 0.0960},
				{Name: "m5.xlarge", CPUCapacity: 8000, MemCapacity: 16e9, CostPerHour: 0.1920},
			},
			pods: []constraints.PodInfo{
				{Name: "small-1", CPURequest: 500, MemRequest: 1e9},
				{Name: "small-2", CPURequest: 500, MemRequest: 1e9},
				{Name: "small-3", CPURequest: 500, MemRequest: 1e9},
				{Name: "medium-1", CPURequest: 1000, MemRequest: 2e9},
				{Name: "medium-2", CPURequest: 1000, MemRequest: 2e9},
				{Name: "large-1", CPURequest: 2000, MemRequest: 4e9},
			},
		},
		{
			name: "SmallCluster_BestFit",
			nodes: []cost.NodeInfo{
				{Name: "small-1", CPUCapacity: 2000, MemCapacity: 4e9, CostPerHour: 0.05},
				{Name: "small-2", CPUCapacity: 2000, MemCapacity: 4e9, CostPerHour: 0.05},
				{Name: "large-1", CPUCapacity: 4000, MemCapacity: 8e9, CostPerHour: 0.12},
			},
			pods: []constraints.PodInfo{
				{Name: "pod-1", CPURequest: 1500, MemRequest: 3e9},
				{Name: "pod-2", CPURequest: 1500, MemRequest: 3e9},
				{Name: "pod-3", CPURequest: 500, MemRequest: 1e9},
			},
		},
		{
			name: "MixedSizes_Packing",
			nodes: []cost.NodeInfo{
				{Name: "tiny", CPUCapacity: 1000, MemCapacity: 2e9, CostPerHour: 0.02},
				{Name: "small", CPUCapacity: 2000, MemCapacity: 4e9, CostPerHour: 0.05},
				{Name: "medium", CPUCapacity: 4000, MemCapacity: 8e9, CostPerHour: 0.10},
				{Name: "large", CPUCapacity: 8000, MemCapacity: 16e9, CostPerHour: 0.20},
			},
			pods: []constraints.PodInfo{
				// Mix of pod sizes to test best fit
				{Name: "large-pod-1", CPURequest: 3000, MemRequest: 6e9},
				{Name: "large-pod-2", CPURequest: 3000, MemRequest: 6e9},
				{Name: "medium-pod-1", CPURequest: 1500, MemRequest: 3e9},
				{Name: "medium-pod-2", CPURequest: 1500, MemRequest: 3e9},
				{Name: "small-pod-1", CPURequest: 800, MemRequest: 1.5e9},
				{Name: "small-pod-2", CPURequest: 800, MemRequest: 1.5e9},
				{Name: "tiny-pod-1", CPURequest: 400, MemRequest: 0.8e9},
				{Name: "tiny-pod-2", CPURequest: 400, MemRequest: 0.8e9},
			},
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			t.Logf("\n=== %s ===", sc.name)

			// Calculate total requirements
			totalCPU := 0.0
			totalMem := 0.0
			for _, pod := range sc.pods {
				totalCPU += pod.CPURequest
				totalMem += pod.MemRequest
			}
			t.Logf("Total requirements: CPU=%v, Memory=%.0fGB", totalCPU, totalMem/1e9)
			t.Log("\nPods:")
			for _, pod := range sc.pods {
				t.Logf("  %s: CPU=%v, Memory=%.0fGB", pod.Name, pod.CPURequest, pod.MemRequest/1e9)
			}

			// Get bin packing solution
			minCost, assignments := cost.BinPackMinCostWithDetails(sc.pods, sc.nodes)

			t.Logf("\nBest Fit Decreasing Solution: Minimum cost = $%.4f/hr", minCost)
			t.Log("\nPod assignments:")

			// Show pod assignments
			nodeUsage := make(map[int][]string)
			nodeCPU := make(map[int]float64)
			nodeMem := make(map[int]float64)

			for i, nodeIdx := range assignments {
				pod := sc.pods[i]
				node := sc.nodes[nodeIdx]
				t.Logf("  %s -> %s", pod.Name, node.Name)

				nodeUsage[nodeIdx] = append(nodeUsage[nodeIdx], pod.Name)
				nodeCPU[nodeIdx] += pod.CPURequest
				nodeMem[nodeIdx] += pod.MemRequest
			}

			// Show node utilization
			t.Log("\nNode utilization:")
			totalCost := 0.0
			for nodeIdx, pods := range nodeUsage {
				node := sc.nodes[nodeIdx]
				cpuUtil := nodeCPU[nodeIdx] / node.CPUCapacity * 100
				memUtil := nodeMem[nodeIdx] / node.MemCapacity * 100

				t.Logf("  %s: $%.4f/hr", node.Name, node.CostPerHour)
				t.Logf("    - Pods: %v", pods)
				t.Logf("    - CPU: %.0f/%.0f (%.1f%% utilized)", nodeCPU[nodeIdx], node.CPUCapacity, cpuUtil)
				t.Logf("    - Memory: %.0fGB/%.0fGB (%.1f%% utilized)",
					nodeMem[nodeIdx]/1e9, node.MemCapacity/1e9, memUtil)

				totalCost += node.CostPerHour
			}

			t.Logf("\nTotal cost: $%.4f/hr", totalCost)

			// Compare with current state
			t.Log("\nComparison:")
			currentCost := sc.nodes[3].CostPerHour // All on m5.xlarge
			t.Logf("  Current (all on m5.xlarge): $%.4f/hr", currentCost)
			t.Logf("  Optimal (bin packing): $%.4f/hr", minCost)
			t.Logf("  Savings: $%.4f/hr (%.1f%%)", currentCost-minCost, (currentCost-minCost)/currentCost*100)
		})
	}
}

func TestBestFitDecreasingApproximation(t *testing.T) {
	// Test that BFD gives reasonable approximations
	tests := []struct {
		name        string
		nodes       []cost.NodeInfo
		pods        []constraints.PodInfo
		maxExpected float64 // Maximum expected cost
		description string
	}{
		{
			name: "Perfect packing possible",
			nodes: []cost.NodeInfo{
				{Name: "n1", CPUCapacity: 4000, MemCapacity: 8e9, CostPerHour: 0.10},
				{Name: "n2", CPUCapacity: 4000, MemCapacity: 8e9, CostPerHour: 0.10},
			},
			pods: []constraints.PodInfo{
				{Name: "p1", CPURequest: 2000, MemRequest: 4e9},
				{Name: "p2", CPURequest: 2000, MemRequest: 4e9},
				{Name: "p3", CPURequest: 2000, MemRequest: 4e9},
				{Name: "p4", CPURequest: 2000, MemRequest: 4e9},
			},
			maxExpected: 0.20, // Should fit perfectly in 2 nodes
			description: "BFD should achieve optimal packing when pods fit perfectly",
		},
		{
			name: "Requires smart bin selection",
			nodes: []cost.NodeInfo{
				{Name: "cheap1", CPUCapacity: 2000, MemCapacity: 4e9, CostPerHour: 0.05},
				{Name: "cheap2", CPUCapacity: 2000, MemCapacity: 4e9, CostPerHour: 0.05},
				{Name: "expensive", CPUCapacity: 4000, MemCapacity: 8e9, CostPerHour: 0.15},
			},
			pods: []constraints.PodInfo{
				{Name: "p1", CPURequest: 1500, MemRequest: 3e9},
				{Name: "p2", CPURequest: 1500, MemRequest: 3e9},
				{Name: "p3", CPURequest: 500, MemRequest: 1e9},
			},
			maxExpected: 0.15, // Optimal would be 0.10 (2 cheap nodes)
			description: "BFD prioritizes cost efficiency, should prefer cheaper nodes",
		},
		{
			name: "MixedNodeTypes scenario",
			nodes: []cost.NodeInfo{
				{Name: "t3.small-spot-1", CPUCapacity: 2000, MemCapacity: 4e9, CostPerHour: 0.0074},
				{Name: "t3.small-spot-2", CPUCapacity: 2000, MemCapacity: 4e9, CostPerHour: 0.0074},
				{Name: "m5.large", CPUCapacity: 4000, MemCapacity: 8e9, CostPerHour: 0.0960},
				{Name: "m5.xlarge", CPUCapacity: 8000, MemCapacity: 16e9, CostPerHour: 0.1920},
			},
			pods: []constraints.PodInfo{
				{Name: "small-1", CPURequest: 500, MemRequest: 1e9},
				{Name: "small-2", CPURequest: 500, MemRequest: 1e9},
				{Name: "small-3", CPURequest: 500, MemRequest: 1e9},
				{Name: "medium-1", CPURequest: 1000, MemRequest: 2e9},
				{Name: "medium-2", CPURequest: 1000, MemRequest: 2e9},
				{Name: "large-1", CPURequest: 2000, MemRequest: 4e9},
			},
			maxExpected: 0.12, // BFD should find good solution close to optimal 0.1034
			description: "Real-world scenario with mixed instance types",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Test: %s", tt.description)

			result := cost.BinPackMinCost(tt.pods, tt.nodes)
			t.Logf("BFD minimum cost: $%.4f/hr", result)

			if result > tt.maxExpected {
				t.Errorf("Cost %.4f exceeds expected maximum %.4f", result, tt.maxExpected)
			}

			// Show assignment details
			_, assignments := cost.BinPackMinCostWithDetails(tt.pods, tt.nodes)
			nodeUsage := make(map[int][]string)
			for i, nodeIdx := range assignments {
				if nodeIdx >= 0 && nodeIdx < len(tt.nodes) {
					nodeUsage[nodeIdx] = append(nodeUsage[nodeIdx], tt.pods[i].Name)
				}
			}

			t.Log("BFD assignment:")
			totalCost := 0.0
			for nodeIdx := 0; nodeIdx < len(tt.nodes); nodeIdx++ {
				if pods, used := nodeUsage[nodeIdx]; used {
					node := tt.nodes[nodeIdx]
					totalCost += node.CostPerHour
					t.Logf("  %s: %v ($%.4f/hr)", node.Name, pods, node.CostPerHour)
				}
			}
			t.Logf("Total cost: $%.4f/hr", totalCost)
		})
	}
}
