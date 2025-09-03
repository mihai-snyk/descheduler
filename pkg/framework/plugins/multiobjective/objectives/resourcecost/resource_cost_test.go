package resourcecost

import (
	"testing"

	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
)

func TestResourceCostObjective(t *testing.T) {
	tests := []struct {
		name     string
		nodes    []NodeInfo
		pods     []PodInfo
		solution []int
		wantDesc string
	}{
		{
			name: "Prefer spot over on-demand",
			nodes: []NodeInfo{
				{
					CPUCapacity:       8000,
					MemCapacity:       16e9,
					HourlyCost:        0.192,
					InstanceType:      "m5.xlarge",
					InstanceLifecycle: "on-demand",
				},
				{
					CPUCapacity:       8000,
					MemCapacity:       16e9,
					HourlyCost:        0.0685,
					InstanceType:      "m5.xlarge",
					InstanceLifecycle: "spot",
				},
			},
			pods: []PodInfo{
				{CPU: 1000, Mem: 2e9},
				{CPU: 1000, Mem: 2e9},
			},
			solution: []int{0, 0}, // Both on expensive on-demand
			wantDesc: "Should have higher cost when pods are on on-demand nodes",
		},
		{
			name: "Same pods on spot",
			nodes: []NodeInfo{
				{
					CPUCapacity:       8000,
					MemCapacity:       16e9,
					HourlyCost:        0.192,
					InstanceType:      "m5.xlarge",
					InstanceLifecycle: "on-demand",
				},
				{
					CPUCapacity:       8000,
					MemCapacity:       16e9,
					HourlyCost:        0.0685,
					InstanceType:      "m5.xlarge",
					InstanceLifecycle: "spot",
				},
			},
			pods: []PodInfo{
				{CPU: 1000, Mem: 2e9},
				{CPU: 1000, Mem: 2e9},
			},
			solution: []int{1, 1}, // Both on cheaper spot
			wantDesc: "Should have lower cost when pods are on spot nodes",
		},
		{
			name: "Different instance types",
			nodes: []NodeInfo{
				{
					CPUCapacity:       4000,
					MemCapacity:       8e9,
					HourlyCost:        0.096,
					InstanceType:      "m5.large",
					InstanceLifecycle: "on-demand",
				},
				{
					CPUCapacity:       4000,
					MemCapacity:       32e9,
					HourlyCost:        0.2,
					InstanceType:      "r5.xlarge",
					InstanceLifecycle: "on-demand",
				},
				{
					CPUCapacity:       16000,
					MemCapacity:       16e9,
					HourlyCost:        0.242,
					InstanceType:      "c5.2xlarge",
					InstanceLifecycle: "on-demand",
				},
			},
			pods: []PodInfo{
				{CPU: 2000, Mem: 4e9},  // Balanced pod
				{CPU: 500, Mem: 8e9},    // Memory-heavy pod
				{CPU: 4000, Mem: 2e9},   // CPU-heavy pod
			},
			solution: []int{0, 1, 2}, // Each on different node type
			wantDesc: "Should calculate costs based on node efficiency",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objFunc := ResourceCostObjectiveFunc(tt.nodes, tt.pods)
			
			// Create solution
			sol := framework.NewIntegerSolution(tt.solution, nil)
			
			// Calculate cost
			cost := objFunc(sol)
			
			t.Logf("%s: cost = %.4f", tt.wantDesc, cost)
			
			// For comparison tests
			if tt.name == "Prefer spot over on-demand" {
				// Calculate cost for spot solution
				spotSol := framework.NewIntegerSolution([]int{1, 1}, nil)
				spotCost := objFunc(spotSol)
				
				if cost <= spotCost {
					t.Errorf("On-demand cost (%.4f) should be higher than spot cost (%.4f)", 
						cost, spotCost)
				}
				
				t.Logf("On-demand cost: %.4f, Spot cost: %.4f (%.1f%% savings)",
					cost, spotCost, (1-spotCost/cost)*100)
			}
		})
	}
}

func TestResourceEfficiencyCalculation(t *testing.T) {
	nodes := []NodeInfo{
		{
			CPUCapacity:       8000,
			MemCapacity:       16e9,
			HourlyCost:        0.192,
			InstanceType:      "m5.xlarge",
			InstanceLifecycle: "on-demand",
		},
		{
			CPUCapacity:       8000,
			MemCapacity:       16e9,
			HourlyCost:        0.0685,
			InstanceType:      "m5.xlarge",
			InstanceLifecycle: "spot",
		},
		{
			CPUCapacity:       4000,
			MemCapacity:       32e9,
			HourlyCost:        0.2,
			InstanceType:      "r5.xlarge",
			InstanceLifecycle: "on-demand",
		},
		{
			CPUCapacity:       16000,
			MemCapacity:       16e9,
			HourlyCost:        0.242,
			InstanceType:      "c5.2xlarge",
			InstanceLifecycle: "on-demand",
		},
	}
	
	pods := []PodInfo{{CPU: 1000, Mem: 2e9}} // Dummy pod for initialization
	
	obj := NewResourceCostObjective(nodes, pods)
	
	// Verify efficiency calculations
	for i, node := range nodes {
		eff := obj.nodeEfficiencies[i]
		t.Logf("Node %s %s: CPU/$ = %.1f, Mem/$ = %.1f, Combined = %.1f",
			node.InstanceType, node.InstanceLifecycle,
			eff.CPUPerDollar, eff.MemPerDollar, eff.Combined)
	}
	
	// Spot should be most efficient
	if obj.nodeEfficiencies[1].Combined <= obj.nodeEfficiencies[0].Combined {
		t.Errorf("Spot instance should have better efficiency than on-demand")
	}
}
