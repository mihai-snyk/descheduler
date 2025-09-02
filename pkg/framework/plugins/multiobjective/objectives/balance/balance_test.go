package balance_test

import (
	"math"
	"testing"

	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/objectives/balance"
)

func TestBalanceObjective(t *testing.T) {
	scenarios := []struct {
		name           string
		assignment     []int
		pods           []balance.PodResources
		nodes          []balance.ResourceInfo
		config         balance.BalanceConfig
		expectedCPUStd float64
		expectedMemStd float64
		description    string
	}{
		{
			name:       "PerfectlyBalanced",
			assignment: []int{0, 1, 2, 0, 1, 2}, // 2 pods per node
			pods: []balance.PodResources{
				{CPURequest: 1000, MemRequest: 1e9}, // 1 CPU, 1GB
				{CPURequest: 1000, MemRequest: 1e9},
				{CPURequest: 1000, MemRequest: 1e9},
				{CPURequest: 1000, MemRequest: 1e9},
				{CPURequest: 1000, MemRequest: 1e9},
				{CPURequest: 1000, MemRequest: 1e9},
			},
			nodes: []balance.ResourceInfo{
				{CPUCapacity: 4000, MemCapacity: 4e9, CPUAllocated: 0, MemAllocated: 0},
				{CPUCapacity: 4000, MemCapacity: 4e9, CPUAllocated: 0, MemAllocated: 0},
				{CPUCapacity: 4000, MemCapacity: 4e9, CPUAllocated: 0, MemAllocated: 0},
			},
			config:         balance.DefaultBalanceConfig(),
			expectedCPUStd: 0.0, // All nodes at 50% utilization
			expectedMemStd: 0.0, // All nodes at 50% utilization
			description:    "All nodes equally loaded should have zero standard deviation",
		},
		{
			name:       "SingleOutlierNode",
			assignment: []int{0, 0, 0, 0, 1, 2}, // 4 pods on node 0, 1 each on others
			pods: []balance.PodResources{
				{CPURequest: 1000, MemRequest: 1e9},
				{CPURequest: 1000, MemRequest: 1e9},
				{CPURequest: 1000, MemRequest: 1e9},
				{CPURequest: 1000, MemRequest: 1e9},
				{CPURequest: 1000, MemRequest: 1e9},
				{CPURequest: 1000, MemRequest: 1e9},
			},
			nodes: []balance.ResourceInfo{
				{CPUCapacity: 4000, MemCapacity: 4e9, CPUAllocated: 0, MemAllocated: 0},
				{CPUCapacity: 4000, MemCapacity: 4e9, CPUAllocated: 0, MemAllocated: 0},
				{CPUCapacity: 4000, MemCapacity: 4e9, CPUAllocated: 0, MemAllocated: 0},
			},
			config:      balance.DefaultBalanceConfig(),
			description: "One heavily loaded node should create high standard deviation",
			// Node 0: 100% util, Node 1: 25% util, Node 2: 25% util
			// Mean: 50%, StdDev should be significant
		},
		{
			name:       "EmptyVsFullNodes",
			assignment: []int{0, 0, 0, 0}, // All pods on node 0
			pods: []balance.PodResources{
				{CPURequest: 1000, MemRequest: 1e9},
				{CPURequest: 1000, MemRequest: 1e9},
				{CPURequest: 1000, MemRequest: 1e9},
				{CPURequest: 1000, MemRequest: 1e9},
			},
			nodes: []balance.ResourceInfo{
				{CPUCapacity: 4000, MemCapacity: 4e9, CPUAllocated: 0, MemAllocated: 0},
				{CPUCapacity: 4000, MemCapacity: 4e9, CPUAllocated: 0, MemAllocated: 0},
				{CPUCapacity: 4000, MemCapacity: 4e9, CPUAllocated: 0, MemAllocated: 0},
			},
			config:      balance.DefaultBalanceConfig(),
			description: "All pods on one node creates maximum imbalance",
			// Node 0: 100%, Nodes 1&2: 0%
		},
		{
			name:       "DifferentNodeCapacities",
			assignment: []int{0, 0, 1, 1, 2}, // 2 pods on small nodes, 1 on large
			pods: []balance.PodResources{
				{CPURequest: 1000, MemRequest: 1e9},
				{CPURequest: 1000, MemRequest: 1e9},
				{CPURequest: 1000, MemRequest: 1e9},
				{CPURequest: 1000, MemRequest: 1e9},
				{CPURequest: 1000, MemRequest: 1e9},
			},
			nodes: []balance.ResourceInfo{
				{CPUCapacity: 2000, MemCapacity: 2e9, CPUAllocated: 0, MemAllocated: 0}, // Small
				{CPUCapacity: 2000, MemCapacity: 2e9, CPUAllocated: 0, MemAllocated: 0}, // Small
				{CPUCapacity: 8000, MemCapacity: 8e9, CPUAllocated: 0, MemAllocated: 0}, // Large
			},
			config:      balance.DefaultBalanceConfig(),
			description: "Different node capacities - small nodes at 100%, large at 12.5%",
		},
		{
			name:       "ExistingAllocations",
			assignment: []int{0, 1, 2}, // 1 pod per node
			pods: []balance.PodResources{
				{CPURequest: 1000, MemRequest: 1e9},
				{CPURequest: 1000, MemRequest: 1e9},
				{CPURequest: 1000, MemRequest: 1e9},
			},
			nodes: []balance.ResourceInfo{
				{CPUCapacity: 4000, MemCapacity: 4e9, CPUAllocated: 2000, MemAllocated: 2e9}, // Already 50%
				{CPUCapacity: 4000, MemCapacity: 4e9, CPUAllocated: 1000, MemAllocated: 1e9}, // Already 25%
				{CPUCapacity: 4000, MemCapacity: 4e9, CPUAllocated: 0, MemAllocated: 0},      // Empty
			},
			config:      balance.DefaultBalanceConfig(),
			description: "Nodes with existing allocations should be considered",
			// After assignment: Node 0: 75%, Node 1: 50%, Node 2: 25%
		},
		{
			name:       "ImbalancedResources",
			assignment: []int{0, 0, 1, 1},
			pods: []balance.PodResources{
				{CPURequest: 2000, MemRequest: 0.5e9}, // CPU heavy
				{CPURequest: 2000, MemRequest: 0.5e9}, // CPU heavy
				{CPURequest: 500, MemRequest: 2e9},    // Memory heavy
				{CPURequest: 500, MemRequest: 2e9},    // Memory heavy
			},
			nodes: []balance.ResourceInfo{
				{CPUCapacity: 4000, MemCapacity: 4e9, CPUAllocated: 0, MemAllocated: 0},
				{CPUCapacity: 4000, MemCapacity: 4e9, CPUAllocated: 0, MemAllocated: 0},
			},
			config:      balance.DefaultBalanceConfig(),
			description: "CPU and memory can be imbalanced differently",
			// Node 0: 100% CPU, 25% Mem; Node 1: 25% CPU, 100% Mem
		},
	}

	for _, tc := range scenarios {
		t.Run(tc.name, func(t *testing.T) {
			cost, result := balance.BalanceObjectiveWithDetails(tc.assignment, tc.pods, tc.nodes, tc.config)

			t.Logf("%s:", tc.description)
			t.Logf("  CPU StdDev: %.2f%% (normalized: %.4f)", result.CPUStdDev, result.NormalizedCPUStdDev)
			t.Logf("  Mem StdDev: %.2f%% (normalized: %.4f)", result.MemStdDev, result.NormalizedMemStdDev)
			t.Logf("  Total Cost: %.4f", cost)

			// Log node utilizations
			t.Logf("  Node utilizations:")
			for _, util := range result.NodeUtilizations {
				t.Logf("    Node %d: CPU=%.1f%%, Mem=%.1f%%",
					util.NodeIndex, util.CPUUtilization, util.MemUtilization)
			}

			// Verify specific expectations where provided
			if tc.expectedCPUStd >= 0 {
				tolerance := 0.1
				if diff := math.Abs(result.CPUStdDev - tc.expectedCPUStd); diff > tolerance {
					t.Errorf("CPU StdDev mismatch: got %.2f, expected %.2f",
						result.CPUStdDev, tc.expectedCPUStd)
				}
			}
		})
	}
}

func TestOutlierSensitivity(t *testing.T) {
	// Test that standard deviation properly captures outliers
	nodes := []balance.ResourceInfo{
		{CPUCapacity: 4000, MemCapacity: 4e9, CPUAllocated: 0, MemAllocated: 0},
		{CPUCapacity: 4000, MemCapacity: 4e9, CPUAllocated: 0, MemAllocated: 0},
		{CPUCapacity: 4000, MemCapacity: 4e9, CPUAllocated: 0, MemAllocated: 0},
		{CPUCapacity: 4000, MemCapacity: 4e9, CPUAllocated: 0, MemAllocated: 0},
	}

	pods := []balance.PodResources{
		{CPURequest: 1000, MemRequest: 1e9},
		{CPURequest: 1000, MemRequest: 1e9},
		{CPURequest: 1000, MemRequest: 1e9},
		{CPURequest: 1000, MemRequest: 1e9},
		{CPURequest: 1000, MemRequest: 1e9},
		{CPURequest: 1000, MemRequest: 1e9},
		{CPURequest: 1000, MemRequest: 1e9},
		{CPURequest: 1000, MemRequest: 1e9},
	}

	config := balance.DefaultBalanceConfig()

	scenarios := []struct {
		name       string
		assignment []int
		desc       string
	}{
		{
			name:       "EvenDistribution",
			assignment: []int{0, 1, 2, 3, 0, 1, 2, 3}, // 2 pods per node
			desc:       "Even distribution: all nodes at 50%",
		},
		{
			name:       "ThreeCloseOneOutlier",
			assignment: []int{0, 0, 0, 0, 1, 2, 3, 3}, // 4 on node 0, 1 on 1&2, 2 on 3
			desc:       "One outlier at 100%, others at 25-50%",
		},
		{
			name:       "TwoHighTwoLow",
			assignment: []int{0, 0, 0, 1, 1, 1, 2, 3}, // 3 each on 0&1, 1 each on 2&3
			desc:       "Two nodes at 75%, two at 25%",
		},
	}

	var costs []float64
	for _, tc := range scenarios {
		cost, result := balance.BalanceObjectiveWithDetails(tc.assignment, pods, nodes, config)
		costs = append(costs, cost)

		t.Logf("%s - %s:", tc.name, tc.desc)
		t.Logf("  Cost: %.4f (CPU StdDev: %.2f%%, Mem StdDev: %.2f%%)",
			cost, result.CPUStdDev, result.MemStdDev)
		t.Logf("  Normalized: CPU=%.4f, Mem=%.4f",
			result.NormalizedCPUStdDev, result.NormalizedMemStdDev)

		// Calculate and show the actual utilizations
		utils := make([]float64, 4)
		for i, util := range result.NodeUtilizations {
			utils[i] = util.CPUUtilization
		}
		t.Logf("  Utilizations: [%.0f%%, %.0f%%, %.0f%%, %.0f%%]",
			utils[0], utils[1], utils[2], utils[3])
	}

	// Verify that outlier case has higher cost than even distribution
	if costs[1] <= costs[0] {
		t.Errorf("Outlier scenario should have higher cost than even distribution")
	}

	// Verify ordering: Even < TwoHighTwoLow < OneOutlier
	if !(costs[0] < costs[2] && costs[2] < costs[1]) {
		t.Errorf("Expected cost ordering: Even < TwoHighTwoLow < OneOutlier, got %.2f < %.2f < %.2f",
			costs[0], costs[2], costs[1])
	}
}

func TestEdgeCases(t *testing.T) {
	// Test edge cases
	testCases := []struct {
		name   string
		nodes  []balance.ResourceInfo
		pods   []balance.PodResources
		assign []int
		desc   string
	}{
		{
			name:   "EmptyCluster",
			nodes:  []balance.ResourceInfo{},
			pods:   []balance.PodResources{},
			assign: []int{},
			desc:   "Empty cluster should have zero cost",
		},
		{
			name: "ZeroCapacityNode",
			nodes: []balance.ResourceInfo{
				{CPUCapacity: 0, MemCapacity: 0, CPUAllocated: 0, MemAllocated: 0},
				{CPUCapacity: 4000, MemCapacity: 4e9, CPUAllocated: 0, MemAllocated: 0},
			},
			pods: []balance.PodResources{
				{CPURequest: 1000, MemRequest: 1e9},
			},
			assign: []int{1}, // Assign to valid node
			desc:   "Zero capacity nodes should be handled gracefully",
		},
		{
			name: "OvercommittedNode",
			nodes: []balance.ResourceInfo{
				{CPUCapacity: 2000, MemCapacity: 2e9, CPUAllocated: 3000, MemAllocated: 3e9},
			},
			pods: []balance.PodResources{
				{CPURequest: 1000, MemRequest: 1e9},
			},
			assign: []int{0},
			desc:   "Over 100% utilization should be handled",
		},
	}

	config := balance.DefaultBalanceConfig()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cost, result := balance.BalanceObjectiveWithDetails(tc.assign, tc.pods, tc.nodes, config)

			t.Logf("%s:", tc.desc)
			t.Logf("  Cost: %.4f", cost)

			// Should not panic or return NaN/Inf
			if math.IsNaN(cost) || math.IsInf(cost, 0) {
				t.Errorf("Cost should be a valid number, got %v", cost)
			}

			// Log any node utilizations
			for _, util := range result.NodeUtilizations {
				t.Logf("  Node %d: CPU=%.1f%%, Mem=%.1f%%",
					util.NodeIndex, util.CPUUtilization, util.MemUtilization)
			}
		})
	}
}

func TestWeightConfiguration(t *testing.T) {
	// Test different weight configurations
	nodes := []balance.ResourceInfo{
		{CPUCapacity: 4000, MemCapacity: 4e9, CPUAllocated: 0, MemAllocated: 0},
		{CPUCapacity: 4000, MemCapacity: 4e9, CPUAllocated: 0, MemAllocated: 0},
	}

	// Create imbalanced scenario: high CPU on node 0, high mem on node 1
	pods := []balance.PodResources{
		{CPURequest: 3000, MemRequest: 1e9}, // CPU heavy on node 0
		{CPURequest: 1000, MemRequest: 3e9}, // Mem heavy on node 1
	}
	assignment := []int{0, 1}

	configs := []struct {
		name   string
		config balance.BalanceConfig
	}{
		{
			name:   "Balanced",
			config: balance.DefaultBalanceConfig(),
		},
		{
			name:   "CPUOnly",
			config: balance.BalanceConfig{CPUWeight: 1.0, MemWeight: 0.0},
		},
		{
			name:   "MemOnly",
			config: balance.BalanceConfig{CPUWeight: 0.0, MemWeight: 1.0},
		},
		{
			name:   "CPUBiased",
			config: balance.BalanceConfig{CPUWeight: 0.8, MemWeight: 0.2},
		},
	}

	for _, tc := range configs {
		t.Run(tc.name, func(t *testing.T) {
			cost, result := balance.BalanceObjectiveWithDetails(assignment, pods, nodes, tc.config)

			t.Logf("Config %s (CPU weight=%.1f, Mem weight=%.1f):",
				tc.name, tc.config.CPUWeight, tc.config.MemWeight)
			t.Logf("  CPU StdDev: %.2f%% (normalized: %.4f, weighted: %.4f)",
				result.CPUStdDev, result.NormalizedCPUStdDev, result.WeightedCPU)
			t.Logf("  Mem StdDev: %.2f%% (normalized: %.4f, weighted: %.4f)",
				result.MemStdDev, result.NormalizedMemStdDev, result.WeightedMem)
			t.Logf("  Total Cost: %.4f", cost)

			// Verify weights are applied correctly to normalized values
			expectedCost := result.NormalizedCPUStdDev*tc.config.CPUWeight + result.NormalizedMemStdDev*tc.config.MemWeight
			if diff := math.Abs(cost - expectedCost); diff > 0.001 {
				t.Errorf("Cost calculation error: got %.4f, expected %.4f", cost, expectedCost)
			}
		})
	}
}

func TestNormalization(t *testing.T) {
	// Test that normalization works correctly
	nodes := []balance.ResourceInfo{
		{CPUCapacity: 4000, MemCapacity: 4e9, CPUAllocated: 0, MemAllocated: 0},
		{CPUCapacity: 4000, MemCapacity: 4e9, CPUAllocated: 0, MemAllocated: 0},
		{CPUCapacity: 4000, MemCapacity: 4e9, CPUAllocated: 0, MemAllocated: 0},
		{CPUCapacity: 4000, MemCapacity: 4e9, CPUAllocated: 0, MemAllocated: 0},
	}

	pods := []balance.PodResources{
		{CPURequest: 2000, MemRequest: 2e9},
		{CPURequest: 2000, MemRequest: 2e9},
		{CPURequest: 2000, MemRequest: 2e9},
		{CPURequest: 2000, MemRequest: 2e9},
	}

	scenarios := []struct {
		name              string
		assignment        []int
		config            balance.BalanceConfig
		expectedNormRange string
	}{
		{
			name:              "EvenDistribution",
			assignment:        []int{0, 1, 2, 3}, // One per node
			config:            balance.DefaultBalanceConfig(),
			expectedNormRange: "very low (near 0)",
		},
		{
			name:              "ModerateImbalance",
			assignment:        []int{0, 0, 1, 2}, // Two on node 0, one each on 1&2, none on 3
			config:            balance.DefaultBalanceConfig(),
			expectedNormRange: "moderate (0.2-0.4)",
		},
		{
			name:              "ExtremeImbalance",
			assignment:        []int{0, 0, 0, 0}, // All on one node
			config:            balance.DefaultBalanceConfig(),
			expectedNormRange: "high (0.8-1.0)",
		},
		{
			name:              "TheoreticalMax",
			assignment:        []int{0, 0, 1, 1}, // Two nodes at 100%, two at 0%
			config:            balance.DefaultBalanceConfig(),
			expectedNormRange: "maximum (1.0)",
		},
		{
			name:       "CustomMaxStdDev",
			assignment: []int{0, 0, 1, 1},
			config: balance.BalanceConfig{
				CPUWeight: 0.5,
				MemWeight: 0.5,
				MaxStdDev: 25.0, // Half of default
			},
			expectedNormRange: "double normalized (2.0)",
		},
	}

	for _, tc := range scenarios {
		t.Run(tc.name, func(t *testing.T) {
			cost, result := balance.BalanceObjectiveWithDetails(tc.assignment, pods, nodes, tc.config)

			t.Logf("Scenario: %s", tc.name)
			t.Logf("  Expected range: %s", tc.expectedNormRange)
			t.Logf("  Raw StdDev: CPU=%.2f%%, Mem=%.2f%%", result.CPUStdDev, result.MemStdDev)
			t.Logf("  Normalized: CPU=%.4f, Mem=%.4f", result.NormalizedCPUStdDev, result.NormalizedMemStdDev)
			t.Logf("  Total normalized cost: %.4f", cost)

			// Verify normalization is working
			if tc.config.MaxStdDev > 0 {
				// For most cases, normalized values should be between 0 and 1
				if tc.name != "CustomMaxStdDev" {
					if result.NormalizedCPUStdDev > 1.0 || result.NormalizedMemStdDev > 1.0 {
						t.Errorf("Normalized values should not exceed 1.0 for standard config")
					}
				}

				// Verify the calculation
				expectedNormCPU := result.CPUStdDev / tc.config.MaxStdDev
				if diff := math.Abs(result.NormalizedCPUStdDev - expectedNormCPU); diff > 0.001 {
					t.Errorf("CPU normalization mismatch: got %.4f, expected %.4f",
						result.NormalizedCPUStdDev, expectedNormCPU)
				}
			}
		})
	}
}

func TestFrameworkIntegration(t *testing.T) {
	// Test integration with optimization framework
	nodes := []balance.ResourceInfo{
		{CPUCapacity: 4000, MemCapacity: 4e9, CPUAllocated: 0, MemAllocated: 0},
		{CPUCapacity: 4000, MemCapacity: 4e9, CPUAllocated: 0, MemAllocated: 0},
	}

	pods := []balance.PodResources{
		{CPURequest: 2000, MemRequest: 2e9},
		{CPURequest: 2000, MemRequest: 2e9},
	}

	config := balance.DefaultBalanceConfig()
	objFunc := balance.BalanceObjectiveFunc(pods, nodes, config)
	detailsFunc := balance.BalanceObjectiveFuncWithDetails(pods, nodes, config)

	// Test with IntegerSolution
	bounds := []framework.IntBounds{{L: 0, H: 1}, {L: 0, H: 1}}
	solution := framework.NewIntegerSolution([]int{0, 1}, bounds)

	cost := objFunc(solution)
	details := detailsFunc(solution)

	t.Logf("Framework integration test:")
	t.Logf("  Cost from objective function: %.4f", cost)

	if result, ok := details.(balance.BalanceResult); ok {
		t.Logf("  Details: CPU StdDev=%.2f%% (norm: %.4f), Mem StdDev=%.2f%% (norm: %.4f)",
			result.CPUStdDev, result.NormalizedCPUStdDev, result.MemStdDev, result.NormalizedMemStdDev)
	} else {
		t.Errorf("Details function should return BalanceResult")
	}

	// Test with wrong solution type
	wrongSolution := &framework.BinarySolution{}
	wrongCost := objFunc(wrongSolution)
	if !math.IsInf(wrongCost, 1) {
		t.Errorf("Expected +Inf for wrong solution type, got %.4f", wrongCost)
	}
}
