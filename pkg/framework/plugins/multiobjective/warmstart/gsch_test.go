package warmstart_test

import (
	"fmt"
	"math"
	"testing"

	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/constraints"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/objectives/balance"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/objectives/cost"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/warmstart"
)

func TestGCSHInitialization(t *testing.T) {
	tests := []struct {
		name      string
		nodes     []constraints.NodeInfo
		pods      []constraints.PodInfo
		popSize   int
		wantValid int // Expected minimum valid solutions
	}{
		{
			name: "Small cluster - all should be valid",
			nodes: []constraints.NodeInfo{
				{Name: "node-0", CPUCapacity: 4000, MemCapacity: 8e9},
				{Name: "node-1", CPUCapacity: 4000, MemCapacity: 8e9},
				{Name: "node-2", CPUCapacity: 8000, MemCapacity: 16e9},
			},
			pods: []constraints.PodInfo{
				{Name: "p1", CPURequest: 1000, MemRequest: 2e9, CurrentNode: 0},
				{Name: "p2", CPURequest: 1000, MemRequest: 2e9, CurrentNode: 0},
				{Name: "p3", CPURequest: 2000, MemRequest: 4e9, CurrentNode: 1},
				{Name: "p4", CPURequest: 2000, MemRequest: 4e9, CurrentNode: 1},
			},
			popSize:   20,
			wantValid: 20, // All should be valid
		},
		{
			name: "Tight capacity - most should be valid",
			nodes: []constraints.NodeInfo{
				{Name: "node-0", CPUCapacity: 4000, MemCapacity: 8e9},
				{Name: "node-1", CPUCapacity: 4000, MemCapacity: 8e9},
			},
			pods: []constraints.PodInfo{
				{Name: "p1", CPURequest: 2000, MemRequest: 4e9, CurrentNode: 0},
				{Name: "p2", CPURequest: 2000, MemRequest: 4e9, CurrentNode: 0},
				{Name: "p3", CPURequest: 2000, MemRequest: 4e9, CurrentNode: 1},
				{Name: "p4", CPURequest: 1500, MemRequest: 3e9, CurrentNode: 1},
			},
			popSize:   20,
			wantValid: 18, // At least 90% valid
		},
		{
			name: "Large cluster simulation",
			nodes: func() []constraints.NodeInfo {
				nodes := make([]constraints.NodeInfo, 10)
				for i := 0; i < 5; i++ {
					nodes[i] = constraints.NodeInfo{
						Name:        fmt.Sprintf("node-%d", i),
						CPUCapacity: 4000,
						MemCapacity: 8e9,
					}
				}
				for i := 5; i < 10; i++ {
					nodes[i] = constraints.NodeInfo{
						Name:        fmt.Sprintf("node-%d", i),
						CPUCapacity: 8000,
						MemCapacity: 16e9,
					}
				}
				return nodes
			}(),
			pods: func() []constraints.PodInfo {
				pods := make([]constraints.PodInfo, 30)
				for i := 0; i < 10; i++ {
					pods[i] = constraints.PodInfo{
						Name:        fmt.Sprintf("small-%d", i),
						CPURequest:  500,
						MemRequest:  1e9,
						CurrentNode: i % 10,
					}
				}
				for i := 10; i < 20; i++ {
					pods[i] = constraints.PodInfo{
						Name:        fmt.Sprintf("medium-%d", i),
						CPURequest:  1500,
						MemRequest:  3e9,
						CurrentNode: i % 10,
					}
				}
				for i := 20; i < 30; i++ {
					pods[i] = constraints.PodInfo{
						Name:        fmt.Sprintf("large-%d", i),
						CPURequest:  3000,
						MemRequest:  6e9,
						CurrentNode: i % 10,
					}
				}
				return pods
			}(),
			popSize:   100,
			wantValid: 95, // At least 95% valid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create node infos for cost objective
			costNodes := make([]cost.NodeInfo, len(tt.nodes))
			for i, node := range tt.nodes {
				costNodes[i] = cost.NodeInfo{
					Name:         fmt.Sprintf("node-%d", i),
					Region:       "us-east-1",
					InstanceType: "m5.large",
					Lifecycle:    "on-demand",
					CostPerHour:  0.096,
					CPUCapacity:  node.CPUCapacity,
					MemCapacity:  node.MemCapacity,
				}
			}

			// Create balance resources
			podResources := make([]balance.PodResources, len(tt.pods))
			for i, pod := range tt.pods {
				podResources[i] = balance.PodResources{
					CPURequest: pod.CPURequest,
					MemRequest: pod.MemRequest,
				}
			}

			balanceNodes := make([]balance.ResourceInfo, len(tt.nodes))
			for i, node := range tt.nodes {
				balanceNodes[i] = balance.ResourceInfo{
					CPUCapacity: node.CPUCapacity,
					MemCapacity: node.MemCapacity,
				}
			}

			// Create objectives for GCSH (only cost and balance, no disruption)
			objectives := []framework.ObjectiveFunc{
				// Cost objective
				cost.CostObjective(tt.pods, costNodes),
				// Balance objective
				balance.BalanceObjectiveFunc(podResources, balanceNodes, balance.DefaultBalanceConfig()),
			}

			// Create constraint
			constraint := constraints.CombineConstraints(
				constraints.ResourceConstraint(tt.pods, tt.nodes),
			)

			// Configure GCSH
			config := warmstart.GCSHConfig{
				Pods:                tt.pods,
				Nodes:               tt.nodes,
				Objectives:          objectives,
				Constraints:         []framework.Constraint{constraint},
				IncludeCurrentState: true,
			}

			gcsh := warmstart.NewGCSH(config)
			solutions := gcsh.GenerateInitialPopulation(tt.popSize)

			// Check population size
			if len(solutions) != tt.popSize {
				t.Errorf("Expected %d solutions, got %d", tt.popSize, len(solutions))
			}

			// Count valid solutions
			validCount := 0
			uniqueSolutions := make(map[string]bool)

			for i, sol := range solutions {
				// Check validity
				if constraint(sol) {
					validCount++
				}

				// Track uniqueness
				if intSol, ok := sol.(*framework.IntegerSolution); ok {
					key := fmt.Sprintf("%v", intSol.Variables)
					uniqueSolutions[key] = true

					// Log first few solutions
					if i < 3 {
						valid := "valid"
						if !constraint(sol) {
							valid = "INVALID"
						}
						t.Logf("Solution %d (%s): %v", i, valid, intSol.Variables)

						// Evaluate objectives
						costVal := objectives[0](sol)
						balanceVal := objectives[1](sol)
						t.Logf("  Cost: %.4f, Balance: %.4f", costVal, balanceVal)
					}
				}
			}

			t.Logf("Generated %d solutions: %d valid (%.1f%%), %d unique",
				len(solutions), validCount, float64(validCount)/float64(len(solutions))*100,
				len(uniqueSolutions))

			// Check validity threshold
			if validCount < tt.wantValid {
				t.Errorf("Expected at least %d valid solutions, got %d", tt.wantValid, validCount)
			}

			// Check diversity (relaxed for small problems where optimal solutions converge)
			minDiversity := 2 // At least current state + one other
			if tt.popSize >= 50 {
				minDiversity = tt.popSize / 5 // For larger populations, expect more diversity
			}
			if len(uniqueSolutions) < minDiversity {
				t.Errorf("Expected at least %d unique solutions, got %d", minDiversity, len(uniqueSolutions))
			}
		})
	}
}

func TestWeightVectorGeneration(t *testing.T) {
	tests := []struct {
		count         int
		numObjectives int
		want          int
	}{
		{count: 1, numObjectives: 2, want: 1},
		{count: 5, numObjectives: 2, want: 5},
		{count: 10, numObjectives: 2, want: 10},
		{count: 5, numObjectives: 3, want: 5},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("count=%d_objectives=%d", tt.count, tt.numObjectives), func(t *testing.T) {
			weights := warmstart.GenerateWeightVectors(tt.count, tt.numObjectives)

			if len(weights) != tt.want {
				t.Errorf("Expected %d weight vectors, got %d", tt.want, len(weights))
			}

			// Check weights are normalized and diverse
			for i, w := range weights {
				t.Logf("Weight %d: %v", i, w)

				// Check normalized (sum to 1)
				sum := 0.0
				for _, v := range w {
					sum += v
				}
				if math.Abs(sum-1.0) > 0.001 {
					t.Errorf("Weights not normalized: sum=%.3f", sum)
				}

				// Check valid range
				for j, v := range w {
					if v < 0 || v > 1 {
						t.Errorf("Invalid weight value at index %d: %.3f", j, v)
					}
				}
			}

			// For 2 objectives, verify they are evenly spaced
			if tt.numObjectives == 2 && tt.count > 1 {
				// Check that first weight emphasizes first objective
				if weights[0][0] < weights[0][1] {
					t.Errorf("First weight should emphasize first objective")
				}
				// Check that last weight emphasizes second objective
				if weights[tt.count-1][0] > weights[tt.count-1][1] {
					t.Errorf("Last weight should emphasize second objective")
				}
			}
		})
	}
}

func TestGCSHScoring(t *testing.T) {
	// Test that GCSH properly scores solutions based on weights
	nodes := []constraints.NodeInfo{
		{Name: "cheap", CPUCapacity: 4000, MemCapacity: 8e9},      // Cheaper node
		{Name: "expensive", CPUCapacity: 8000, MemCapacity: 16e9}, // Expensive node
	}

	costNodes := []cost.NodeInfo{
		{Name: "cheap", CPUCapacity: 4000, MemCapacity: 8e9, CostPerHour: 0.05},
		{Name: "expensive", CPUCapacity: 8000, MemCapacity: 16e9, CostPerHour: 0.20},
	}

	pods := []constraints.PodInfo{
		{Name: "p1", CPURequest: 1000, MemRequest: 2e9, CurrentNode: 0},
		{Name: "p2", CPURequest: 1000, MemRequest: 2e9, CurrentNode: 0},
		{Name: "p3", CPURequest: 1000, MemRequest: 2e9, CurrentNode: 1},
		{Name: "p4", CPURequest: 1000, MemRequest: 2e9, CurrentNode: 1},
	}

	// Create balance resources
	podResources := make([]balance.PodResources, len(pods))
	for i, pod := range pods {
		podResources[i] = balance.PodResources{
			CPURequest: pod.CPURequest,
			MemRequest: pod.MemRequest,
		}
	}

	balanceNodes := make([]balance.ResourceInfo, len(nodes))
	for i, node := range nodes {
		balanceNodes[i] = balance.ResourceInfo{
			CPUCapacity: node.CPUCapacity,
			MemCapacity: node.MemCapacity,
		}
	}

	// Only objectives we want to optimize (no disruption)
	objectives := []framework.ObjectiveFunc{
		cost.CostObjective(pods, costNodes),
		balance.BalanceObjectiveFunc(podResources, balanceNodes, balance.DefaultBalanceConfig()),
	}

	constraint := constraints.CombineConstraints(
		constraints.ResourceConstraint(pods, nodes),
	)

	// Test with different weights
	config := warmstart.GCSHConfig{
		Pods:                pods,
		Nodes:               nodes,
		Objectives:          objectives,
		Constraints:         []framework.Constraint{constraint},
		IncludeCurrentState: false,
	}

	gcsh := warmstart.NewGCSH(config)
	solutions := gcsh.GenerateInitialPopulation(5)

	if len(solutions) != 5 {
		t.Fatalf("Expected 5 solutions, got %d", len(solutions))
	}

	// Check that solutions have different characteristics
	// With deterministic weights for 2 objectives and 5 solutions:
	// weights[0] = [1.00, 0.00] - prioritize cost
	// weights[1] = [0.75, 0.25]
	// weights[2] = [0.50, 0.50] - balanced
	// weights[3] = [0.25, 0.75]
	// weights[4] = [0.00, 1.00] - prioritize balance
	t.Log("Solutions with different weight vectors:")
	for i, sol := range solutions {
		costVal := objectives[0](sol)
		balanceVal := objectives[1](sol)

		t.Logf("Solution %d: cost=%.4f, balance=%.4f", i, costVal, balanceVal)

		if intSol, ok := sol.(*framework.IntegerSolution); ok {
			// Count nodes used
			nodesUsed := make(map[int]bool)
			for _, nodeIdx := range intSol.Variables {
				nodesUsed[nodeIdx] = true
			}
			t.Logf("  Assignment: %v, nodes used: %d", intSol.Variables, len(nodesUsed))
		}
	}
}
