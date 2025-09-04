package multiobjective_test

import (
	"fmt"
	"testing"

	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/algorithms"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/constraints"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/objectives/balance"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/objectives/cost"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/objectives/disruption"
)

// TestMultiObjectiveOptimization3 runs sequential optimization scenarios with ORIGINAL cost objective
func TestMultiObjectiveOptimization3(t *testing.T) {
	testCases := []struct {
		name             string
		nodes            []NodeConfig
		pods             []PodConfig
		weightProfile    WeightProfile
		populationSize   int
		maxGenerations   int
		expectedBehavior string
	}{
		{
			name: "LargeCluster_MixedWorkloads",
			nodes: []NodeConfig{
				// Production nodes - stable, on-demand
				{Name: "prod-1", CPU: 8000, Mem: 16e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
				{Name: "prod-2", CPU: 8000, Mem: 16e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
				{Name: "prod-3", CPU: 8000, Mem: 16e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
				{Name: "prod-4", CPU: 8000, Mem: 16e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
				// Development nodes - mix of spot and on-demand
				{Name: "dev-1", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
				{Name: "dev-2", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
				{Name: "dev-3", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "on-demand"},
				// Worker nodes - compute optimized
				{Name: "worker-1", CPU: 16000, Mem: 16e9, Type: "c5.2xlarge", Region: "us-east-1", Lifecycle: "spot"},
				{Name: "worker-2", CPU: 16000, Mem: 16e9, Type: "c5.2xlarge", Region: "us-east-1", Lifecycle: "spot"},
				// Memory optimized nodes
				{Name: "mem-1", CPU: 4000, Mem: 32e9, Type: "r5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
				{Name: "mem-2", CPU: 4000, Mem: 32e9, Type: "r5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
			},
			pods: []PodConfig{
				// Frontend pods (12 replicas)
				{Name: "frontend-1", CPU: 500, Mem: 1e9, Node: 0, RS: "frontend", MaxUnavail: 2},
				{Name: "frontend-2", CPU: 500, Mem: 1e9, Node: 0, RS: "frontend", MaxUnavail: 2},
				{Name: "frontend-3", CPU: 500, Mem: 1e9, Node: 1, RS: "frontend", MaxUnavail: 2},
				{Name: "frontend-4", CPU: 500, Mem: 1e9, Node: 1, RS: "frontend", MaxUnavail: 2},
				{Name: "frontend-5", CPU: 500, Mem: 1e9, Node: 2, RS: "frontend", MaxUnavail: 2},
				{Name: "frontend-6", CPU: 500, Mem: 1e9, Node: 2, RS: "frontend", MaxUnavail: 2},
				{Name: "frontend-7", CPU: 500, Mem: 1e9, Node: 3, RS: "frontend", MaxUnavail: 2},
				{Name: "frontend-8", CPU: 500, Mem: 1e9, Node: 3, RS: "frontend", MaxUnavail: 2},
				{Name: "frontend-9", CPU: 500, Mem: 1e9, Node: 4, RS: "frontend", MaxUnavail: 2},
				{Name: "frontend-10", CPU: 500, Mem: 1e9, Node: 4, RS: "frontend", MaxUnavail: 2},
				{Name: "frontend-11", CPU: 500, Mem: 1e9, Node: 5, RS: "frontend", MaxUnavail: 2},
				{Name: "frontend-12", CPU: 500, Mem: 1e9, Node: 5, RS: "frontend", MaxUnavail: 2},
				// API pods (8 replicas)
				{Name: "api-1", CPU: 1000, Mem: 2e9, Node: 0, RS: "api", MaxUnavail: 1},
				{Name: "api-2", CPU: 1000, Mem: 2e9, Node: 1, RS: "api", MaxUnavail: 1},
				{Name: "api-3", CPU: 1000, Mem: 2e9, Node: 2, RS: "api", MaxUnavail: 1},
				{Name: "api-4", CPU: 1000, Mem: 2e9, Node: 3, RS: "api", MaxUnavail: 1},
				{Name: "api-5", CPU: 1000, Mem: 2e9, Node: 0, RS: "api", MaxUnavail: 1},
				{Name: "api-6", CPU: 1000, Mem: 2e9, Node: 1, RS: "api", MaxUnavail: 1},
				{Name: "api-7", CPU: 1000, Mem: 2e9, Node: 2, RS: "api", MaxUnavail: 1},
				{Name: "api-8", CPU: 1000, Mem: 2e9, Node: 3, RS: "api", MaxUnavail: 1},
				// Cache pods (6 replicas) - need 6GB each, so careful with placement
				{Name: "cache-1", CPU: 1000, Mem: 6e9, Node: 9, RS: "cache", MaxUnavail: 2},  // mem-1 (32GB)
				{Name: "cache-2", CPU: 1000, Mem: 6e9, Node: 10, RS: "cache", MaxUnavail: 2}, // mem-2 (32GB)
				{Name: "cache-3", CPU: 1000, Mem: 6e9, Node: 9, RS: "cache", MaxUnavail: 2},  // mem-1 (32GB)
				{Name: "cache-4", CPU: 1000, Mem: 6e9, Node: 10, RS: "cache", MaxUnavail: 2}, // mem-2 (32GB)
				{Name: "cache-5", CPU: 1000, Mem: 6e9, Node: 0, RS: "cache", MaxUnavail: 2},  // prod-1 (16GB)
				{Name: "cache-6", CPU: 1000, Mem: 6e9, Node: 1, RS: "cache", MaxUnavail: 2},  // prod-2 (16GB)
				// Worker pods (4 replicas)
				{Name: "worker-job-1", CPU: 4000, Mem: 4e9, Node: 7, RS: "worker", MaxUnavail: 3},
				{Name: "worker-job-2", CPU: 4000, Mem: 4e9, Node: 7, RS: "worker", MaxUnavail: 3},
				{Name: "worker-job-3", CPU: 4000, Mem: 4e9, Node: 8, RS: "worker", MaxUnavail: 3},
				{Name: "worker-job-4", CPU: 4000, Mem: 4e9, Node: 8, RS: "worker", MaxUnavail: 3},
				{Name: "test-runner-1", CPU: 1500, Mem: 3e9, Node: 4, RS: "test", MaxUnavail: 2},
				{Name: "test-runner-2", CPU: 1500, Mem: 3e9, Node: 5, RS: "test", MaxUnavail: 2},
			},
			weightProfile:    WeightProfile{Cost: 0.80, Disruption: 0.20, Balance: 0.00},
			populationSize:   400,
			maxGenerations:   1000,
			expectedBehavior: "Should consolidate non-critical workloads to spot nodes while respecting PDBs",
		},
		{
			name: "BadToGood_OriginalCostMigration",
			nodes: []NodeConfig{
				// BAD cost pool - expensive on-demand instances (same capacity for fair comparison)
				{Name: "bad-1", CPU: 4000, Mem: 8e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"}, // $0.192/hour
				{Name: "bad-2", CPU: 4000, Mem: 8e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
				{Name: "bad-3", CPU: 4000, Mem: 8e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
				{Name: "bad-4", CPU: 4000, Mem: 8e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
				// GOOD cost pool - cheap spot instances (same capacity for fair comparison)
				{Name: "good-1", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"}, // $0.0685/hour
				{Name: "good-2", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
				{Name: "good-3", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
				{Name: "good-4", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
			},
			pods: []PodConfig{
				// Group A: Similar pods on BAD nodes (intentionally expensive placement)
				{Name: "app-a-1", CPU: 1000, Mem: 2e9, Node: 0, RS: "app-a", MaxUnavail: 2}, // bad-1
				{Name: "app-a-2", CPU: 1000, Mem: 2e9, Node: 0, RS: "app-a", MaxUnavail: 2}, // bad-1
				{Name: "app-a-3", CPU: 1000, Mem: 2e9, Node: 1, RS: "app-a", MaxUnavail: 2}, // bad-2
				{Name: "app-a-4", CPU: 1000, Mem: 2e9, Node: 1, RS: "app-a", MaxUnavail: 2}, // bad-2
				{Name: "web-a-1", CPU: 500, Mem: 1e9, Node: 2, RS: "web-a", MaxUnavail: 1},  // bad-3
				{Name: "web-a-2", CPU: 500, Mem: 1e9, Node: 2, RS: "web-a", MaxUnavail: 1},  // bad-3
				{Name: "web-a-3", CPU: 500, Mem: 1e9, Node: 3, RS: "web-a", MaxUnavail: 1},  // bad-4
				{Name: "web-a-4", CPU: 500, Mem: 1e9, Node: 3, RS: "web-a", MaxUnavail: 1},  // bad-4
				// Group B: Similar pods on GOOD nodes (already cheap placement)
				{Name: "app-b-1", CPU: 1000, Mem: 2e9, Node: 4, RS: "app-b", MaxUnavail: 2}, // good-1
				{Name: "app-b-2", CPU: 1000, Mem: 2e9, Node: 4, RS: "app-b", MaxUnavail: 2}, // good-1
				{Name: "app-b-3", CPU: 1000, Mem: 2e9, Node: 5, RS: "app-b", MaxUnavail: 2}, // good-2
				{Name: "app-b-4", CPU: 1000, Mem: 2e9, Node: 5, RS: "app-b", MaxUnavail: 2}, // good-2
				{Name: "web-b-1", CPU: 500, Mem: 1e9, Node: 6, RS: "web-b", MaxUnavail: 1},  // good-3
				{Name: "web-b-2", CPU: 500, Mem: 1e9, Node: 6, RS: "web-b", MaxUnavail: 1},  // good-3
				{Name: "web-b-3", CPU: 500, Mem: 1e9, Node: 7, RS: "web-b", MaxUnavail: 1},  // good-4
				{Name: "web-b-4", CPU: 500, Mem: 1e9, Node: 7, RS: "web-b", MaxUnavail: 1},  // good-4
			},
			weightProfile:    WeightProfile{Cost: 0.90, Disruption: 0.10, Balance: 0.00},
			populationSize:   200,
			maxGenerations:   500,
			expectedBehavior: "Should migrate pods from expensive on-demand nodes to cheap spot nodes (original cost objective)",
		},
		{
			name: "ExtremeNodeCostDifference",
			nodes: []NodeConfig{
				// EXPENSIVE pool - high hourly cost nodes
				{Name: "expensive-1", CPU: 4000, Mem: 32e9, Type: "r5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"}, // $0.252/hour
				{Name: "expensive-2", CPU: 4000, Mem: 32e9, Type: "r5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
				{Name: "expensive-3", CPU: 4000, Mem: 32e9, Type: "r5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
				// CHEAP pool - low hourly cost nodes
				{Name: "cheap-1", CPU: 4000, Mem: 8e9, Type: "t3.xlarge", Region: "us-east-1", Lifecycle: "spot"}, // $0.0604/hour
				{Name: "cheap-2", CPU: 4000, Mem: 8e9, Type: "t3.xlarge", Region: "us-east-1", Lifecycle: "spot"},
				{Name: "cheap-3", CPU: 4000, Mem: 8e9, Type: "t3.xlarge", Region: "us-east-1", Lifecycle: "spot"},
			},
			pods: []PodConfig{
				// Light workloads on EXPENSIVE nodes (node-level waste)
				{Name: "light-1", CPU: 1500, Mem: 2e9, Node: 0, RS: "light", MaxUnavail: 1}, // expensive-1: light usage of expensive node
				{Name: "light-2", CPU: 1500, Mem: 2e9, Node: 1, RS: "light", MaxUnavail: 1}, // expensive-2: light usage of expensive node
				{Name: "light-3", CPU: 1500, Mem: 2e9, Node: 2, RS: "light", MaxUnavail: 1}, // expensive-3: light usage of expensive node
				// Balanced workloads on CHEAP nodes (good fit)
				{Name: "balanced-1", CPU: 1500, Mem: 3e9, Node: 3, RS: "balanced", MaxUnavail: 2}, // cheap-1: good usage of cheap node
				{Name: "balanced-2", CPU: 1500, Mem: 3e9, Node: 4, RS: "balanced", MaxUnavail: 2}, // cheap-2: good usage of cheap node
				{Name: "balanced-3", CPU: 1500, Mem: 3e9, Node: 5, RS: "balanced", MaxUnavail: 2}, // cheap-3: good usage of cheap node
			},
			weightProfile:    WeightProfile{Cost: 0.95, Disruption: 0.05, Balance: 0.00},
			populationSize:   150,
			maxGenerations:   300,
			expectedBehavior: "Should migrate light workloads from expensive nodes to cheap nodes (original cost objective)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runSequentialOptimizationOriginal(t, tc, 5) // Run 5 sequential optimization rounds with original cost
		})
	}
}

type EvolutionStep struct {
	round            int
	bestSol          Analysis
	allSols          []Analysis
	changes          []string
	targetMovements  int
	appliedMovements int
}

// Note: This test uses the same sequential optimization infrastructure as integration2
// but with the original cost objective instead of the effective cost objective
// For now, using simplified implementation - full implementation can be added if needed
func runSequentialOptimizationOriginal(t *testing.T, tc struct {
	name             string
	nodes            []NodeConfig
	pods             []PodConfig
	weightProfile    WeightProfile
	populationSize   int
	maxGenerations   int
	expectedBehavior string
}, numRounds int) {
	t.Logf("\n🚀 SEQUENTIAL OPTIMIZATION SIMULATION (ORIGINAL COST): %s", tc.name)
	t.Logf("Expected behavior: %s", tc.expectedBehavior)
	t.Logf("Weights: Cost=%.2f, Disruption=%.2f, Balance=%.2f",
		tc.weightProfile.Cost, tc.weightProfile.Disruption, tc.weightProfile.Balance)
	t.Logf("Running %d sequential optimization rounds...\n", numRounds)

	// Convert to framework types once
	nodes := make([]framework.NodeInfo, len(tc.nodes))
	for i, n := range tc.nodes {
		c, err := cost.GetInstanceCost(n.Region, n.Type, n.Lifecycle)
		if err != nil {
			t.Errorf("Failed to get cost for node %s: %v", n.Name, err)
		}
		nodes[i] = framework.NodeInfo{
			Idx:               i,
			Name:              n.Name,
			CPUCapacity:       n.CPU,
			MemCapacity:       n.Mem,
			InstanceType:      n.Type,
			InstanceLifecycle: n.Lifecycle,
			Region:            n.Region,
			HourlyCost:        c,
		}
	}

	// Store the TRUE initial cluster state (before any optimization)
	initialClusterState := make([]int, len(tc.pods))
	for i, pod := range tc.pods {
		initialClusterState[i] = pod.Node
	}

	// Current pod configuration (will be updated each round)
	currentPods := make([]PodConfig, len(tc.pods))
	copy(currentPods, tc.pods)

	// Track evolution over time
	evolutionHistory := []EvolutionStep{}

	// Keep ALL previous solutions for seeding
	var previousSolutions []Analysis

	for round := 0; round < numRounds; round++ {
		t.Logf("\n================================================================================")
		t.Logf("🔄 OPTIMIZATION ROUND %d (ORIGINAL COST OBJECTIVE)", round+1)
		t.Logf("================================================================================")

		// Convert current pods to framework types
		pods := make([]framework.PodInfo, len(currentPods))
		for i, p := range currentPods {
			pods[i] = framework.PodInfo{
				Idx:                    i,
				Name:                   p.Name,
				CPURequest:             p.CPU,
				MemRequest:             p.Mem,
				Node:                   p.Node,
				ReplicaSetName:         p.RS,
				MaxUnavailableReplicas: p.MaxUnavail,
			}
		}

		// Show current state
		t.Logf("\nStarting state for round %d:", round+1)
		showCurrentStateOriginal(t, currentPods, tc.nodes)

		// Calculate and show current cluster cost
		currentAssignment := make([]int, len(currentPods))
		for i, pod := range currentPods {
			currentAssignment[i] = pod.Node
		}
		currentCost, currentBalance := calculateActualMetrics(currentAssignment, tc.nodes, tc.pods)
		t.Logf("💰 Current cluster cost: $%.2f/hour, Load balance: %.1f%%", currentCost, currentBalance)

		// Run optimization
		bestSol, allSols := runSingleOptimizationRoundOriginal(t, tc, nodes, pods, round+1)

		// Show immediate cost impact of best solution
		bestCost, bestBalance := calculateActualMetrics(bestSol.Assignment, tc.nodes, tc.pods)
		costDelta := currentCost - bestCost
		balanceDelta := currentBalance - bestBalance
		t.Logf("🎯 Best solution impact: Cost $%.2f→$%.2f (Δ$%.2f), Balance %.1f%%→%.1f%% (Δ%.1f%%)",
			currentCost, bestCost, costDelta, currentBalance, bestBalance, balanceDelta)

		// Track changes from this round
		var changes []string
		if round > 0 {
			prevBest := evolutionHistory[round-1].bestSol
			costDelta := prevBest.Cost - bestSol.Cost
			disruptionDelta := bestSol.Disruption - prevBest.Disruption
			balanceDelta := prevBest.Balance - bestSol.Balance

			changes = append(changes, fmt.Sprintf("Cost: %+.4f", costDelta))
			changes = append(changes, fmt.Sprintf("Disruption: %+.4f", disruptionDelta))
			changes = append(changes, fmt.Sprintf("Balance: %+.4f", balanceDelta))
		}

		// Calculate applied movements for tracking
		appliedMovements := 0
		if round < numRounds-1 {
			feasibleMoves := calculateFeasibleMovementsOriginal(bestSol.Assignment, currentPods, tc.nodes)
			appliedMovements = len(feasibleMoves)
		} else {
			appliedMovements = bestSol.Movements // Last round - no more execution needed
		}

		// Store evolution step
		evolutionHistory = append(evolutionHistory, EvolutionStep{
			round:            round + 1,
			bestSol:          bestSol,
			allSols:          allSols,
			changes:          changes,
			targetMovements:  bestSol.Movements,
			appliedMovements: appliedMovements,
		})

		// Update previous solutions for diversity seeding
		previousSolutions = append(previousSolutions, allSols...)
		t.Logf("📚 Total solutions in diversity pool: %d", len(previousSolutions))

		// Apply the best solution incrementally for next round (if not the last round)
		if round < numRounds-1 {
			// Calculate which pods can actually move respecting PDBs
			feasibleMoves := calculateFeasibleMovementsOriginal(bestSol.Assignment, currentPods, tc.nodes)

			t.Logf("\n🎯 Target state has %d movements, but PDBs allow only %d movements in this iteration",
				bestSol.Movements, len(feasibleMoves))

			// Apply only the feasible movements
			actualApplied := 0
			for i, targetNode := range bestSol.Assignment {
				if targetNode != currentPods[i].Node {
					// Check if this pod is in the feasible moves list
					podCanMove := false
					for _, feasiblePod := range feasibleMoves {
						if feasiblePod == currentPods[i].Name {
							podCanMove = true
							break
						}
					}

					if podCanMove {
						t.Logf("   ✅ Moving %s: %s → %s",
							currentPods[i].Name,
							tc.nodes[currentPods[i].Node].Name,
							tc.nodes[targetNode].Name)
						currentPods[i].Node = targetNode
						actualApplied++
					} else {
						t.Logf("   ⏸️  Deferring %s: %s → %s (PDB constraint)",
							currentPods[i].Name,
							tc.nodes[currentPods[i].Node].Name,
							tc.nodes[targetNode].Name)
					}
				}
			}

			t.Logf("\n📊 Applied %d movements out of %d target movements", actualApplied, bestSol.Movements)
			t.Logf("   Remaining %d movements will be considered in future rounds", bestSol.Movements-actualApplied)
		}
	}

	// Show evolution summary
	showEvolutionSummaryOriginal(t, evolutionHistory, tc.weightProfile, tc.nodes, tc.pods, initialClusterState)
}

func runOptimizationTestOriginal(t *testing.T, tc struct {
	name             string
	nodes            []NodeConfig
	pods             []PodConfig
	weightProfile    WeightProfile
	populationSize   int
	maxGenerations   int
	expectedBehavior string
}) {
	t.Logf("\n=== Scenario: %s ===", tc.name)
	t.Logf("Expected behavior: %s", tc.expectedBehavior)
	t.Logf("Weights: Cost=%.2f, Disruption=%.2f, Balance=%.2f",
		tc.weightProfile.Cost, tc.weightProfile.Disruption, tc.weightProfile.Balance)

	// Convert to framework types
	nodes := make([]framework.NodeInfo, len(tc.nodes))
	for i, n := range tc.nodes {
		c, err := cost.GetInstanceCost(n.Region, n.Type, n.Lifecycle)
		if err != nil {
			t.Errorf("Failed to get cost for node %s: %v", n.Name, err)
		}
		nodes[i] = framework.NodeInfo{
			Idx:               i,
			Name:              n.Name,
			CPUCapacity:       n.CPU,
			MemCapacity:       n.Mem,
			InstanceType:      n.Type,
			InstanceLifecycle: n.Lifecycle,
			Region:            n.Region,
			HourlyCost:        c,
		}
	}

	pods := make([]framework.PodInfo, len(tc.pods))
	for i, p := range tc.pods {
		pods[i] = framework.PodInfo{
			Idx:                    i,
			Name:                   p.Name,
			CPURequest:             p.CPU,
			MemRequest:             p.Mem,
			Node:                   p.Node,
			ReplicaSetName:         p.RS,
			MaxUnavailableReplicas: p.MaxUnavail,
		}
	}

	// Create problem with ORIGINAL cost objective
	problem := createKubernetesProblemOriginal(nodes, pods, tc.weightProfile)

	// Configure and run NSGA-II
	config := algorithms.NSGA2Config{
		PopulationSize:       tc.populationSize,
		MaxGenerations:       tc.maxGenerations,
		CrossoverProbability: 0.9,
		MutationProbability:  0.3,
		TournamentSize:       3,
		ParallelExecution:    true,
	}

	// Run NSGA-II
	nsga2 := algorithms.NewNSGAII(config, problem)
	population := nsga2.Run()

	// Get Pareto front
	fronts := algorithms.NonDominatedSort(population)
	if len(fronts) == 0 || len(fronts[0]) == 0 {
		t.Fatal("No solutions found in Pareto front")
	}

	paretoFront := fronts[0]
	t.Logf("\nFound %d Pareto-optimal solutions", len(paretoFront))

	// Analyze solutions
	analyses := make([]Analysis, len(paretoFront))
	for i, sol := range paretoFront {
		intSol := sol.Solution.(*framework.IntegerSolution)
		obj := problem.Evaluate(sol.Solution)

		movements := 0
		for j, node := range intSol.Variables {
			if node != pods[j].Node {
				movements++
			}
		}

		analyses[i] = Analysis{
			Assignment:    intSol.Variables,
			Cost:          obj[0],
			Disruption:    obj[1],
			Balance:       obj[2],
			WeightedTotal: obj[0]*tc.weightProfile.Cost + obj[1]*tc.weightProfile.Disruption + obj[2]*tc.weightProfile.Balance,
			Movements:     movements,
		}
	}

	// Sort by weighted total
	for i := 0; i < len(analyses)-1; i++ {
		for j := i + 1; j < len(analyses); j++ {
			if analyses[i].WeightedTotal > analyses[j].WeightedTotal {
				analyses[i], analyses[j] = analyses[j], analyses[i]
			}
		}
	}

	// Show top 10 solutions with movement analysis
	maxToShow := 10
	if len(analyses) < maxToShow {
		maxToShow = len(analyses)
	}

	t.Logf("\nTop %d solutions by weighted total:", maxToShow)
	for i := 0; i < maxToShow; i++ {
		a := analyses[i]
		isInitial := a.Movements == 0
		marker := ""
		if isInitial {
			marker = " [INITIAL STATE]"
		}
		if i == 0 {
			marker += " [BEST]"
		}

		// Add movement category for clarity
		var category string
		switch {
		case a.Movements == 0:
			category = "No change"
		case a.Movements <= 2:
			category = "Minimal change"
		case a.Movements <= 5:
			category = "Small change"
		case a.Movements <= 10:
			category = "Medium change"
		default:
			category = "Large change"
		}

		t.Logf("\n%d. [%s] Movements: %d%s", i+1, category, a.Movements, marker)
		t.Logf("   Objectives: Cost=%.4f, Disruption=%.4f, Balance=%.4f, Weighted=%.4f",
			a.Cost, a.Disruption, a.Balance, a.WeightedTotal)
		t.Logf("   Assignment: %v", a.Assignment)

		// Show specific movements for top 5 solutions
		if a.Movements > 0 && i < 5 {
			showDetailedMovementsOriginal(t, a.Assignment, pods, tc.nodes, "   ")
		}
	}
}

func showDetailedMovementsOriginal(t *testing.T, assignment []int, pods []framework.PodInfo, nodes []NodeConfig, prefix string) {
	t.Logf("%sSpecific pod movements:", prefix)
	movementsByType := make(map[string][]string)

	for j, targetNode := range assignment {
		if targetNode != pods[j].Node {
			currentNodeName := nodes[pods[j].Node].Name
			targetNodeName := nodes[targetNode].Name
			currentType := nodes[pods[j].Node].Type
			targetType := nodes[targetNode].Type
			currentLifecycle := nodes[pods[j].Node].Lifecycle
			targetLifecycle := nodes[targetNode].Lifecycle

			// Categorize movement type
			var movementType string
			if currentLifecycle == "on-demand" && targetLifecycle == "spot" {
				movementType = "💰 On-demand → Spot (cost saving)"
			} else if currentLifecycle == "spot" && targetLifecycle == "on-demand" {
				movementType = "🛡️ Spot → On-demand (reliability)"
			} else if currentType != targetType {
				movementType = fmt.Sprintf("🔄 Instance type change (%s → %s)", currentType, targetType)
			} else {
				movementType = "🔀 Same type migration"
			}

			podInfo := fmt.Sprintf("%s: %s → %s [%.1f cores, %.1f GiB]",
				pods[j].Name, currentNodeName, targetNodeName,
				pods[j].CPURequest/1000.0, pods[j].MemRequest/1e9)

			if movementsByType[movementType] == nil {
				movementsByType[movementType] = []string{}
			}
			movementsByType[movementType] = append(movementsByType[movementType], podInfo)
		}
	}

	// Display movements grouped by type
	for movementType, movements := range movementsByType {
		t.Logf("%s  %s:", prefix, movementType)
		for _, movement := range movements {
			t.Logf("%s    - %s", prefix, movement)
		}
	}
}

// createKubernetesProblemOriginal creates a MOO problem for pod scheduling with ORIGINAL cost objective
func createKubernetesProblemOriginal(nodes []framework.NodeInfo, pods []framework.PodInfo, weights WeightProfile) *KubernetesProblemOriginal {
	// Create objectives - using ORIGINAL cost objective
	originalCostObj := cost.CostObjective(pods, nodes)

	// Convert for disruption objective
	disruptionPods := make([]framework.PodInfo, len(pods))
	for i, p := range pods {
		disruptionPods[i] = framework.PodInfo{
			Name:                   p.Name,
			Node:                   p.Node,
			ColdStartTime:          0.0, // Default 10s cold start
			ReplicaSetName:         p.ReplicaSetName,
			MaxUnavailableReplicas: p.MaxUnavailableReplicas,
		}
	}
	currentState := make([]int, len(pods))
	for i, p := range pods {
		currentState[i] = p.Node
	}
	disruptionConfig := disruption.NewDisruptionConfig(disruptionPods)
	disruptionObj := disruption.DisruptionObjective(currentState, disruptionPods, disruptionConfig)

	balanceConfig := balance.DefaultBalanceConfig()
	balanceObj := balance.BalanceObjectiveFunc(pods, nodes, balanceConfig)

	// Create constraints
	resourceConstraint := constraints.ResourceConstraint(pods, nodes)

	// Calculate max possible cost
	maxPossibleCost := 0.0
	for _, node := range nodes {
		maxPossibleCost += node.HourlyCost
	}

	return &KubernetesProblemOriginal{
		nodes:               nodes,
		pods:                pods,
		costObjective:       originalCostObj,
		disruptionObjective: disruptionObj,
		balanceObjective:    balanceObj,
		constraint:          resourceConstraint,
		maxPossibleCost:     maxPossibleCost,
	}
}

// KubernetesProblemOriginal implements framework.Problem for original cost objective
type KubernetesProblemOriginal struct {
	nodes               []framework.NodeInfo
	pods                []framework.PodInfo
	costObjective       framework.ObjectiveFunc
	disruptionObjective framework.ObjectiveFunc
	balanceObjective    framework.ObjectiveFunc
	constraint          framework.Constraint
	maxPossibleCost     float64
}

func (kp *KubernetesProblemOriginal) Name() string {
	return "KubernetesPodScheduling_OriginalCost"
}

func (kp *KubernetesProblemOriginal) ObjectiveFuncs() []framework.ObjectiveFunc {
	return []framework.ObjectiveFunc{
		kp.costObjective,
		kp.disruptionObjective,
		kp.balanceObjective,
	}
}

func (kp *KubernetesProblemOriginal) Constraints() []framework.Constraint {
	return []framework.Constraint{kp.constraint}
}

func (kp *KubernetesProblemOriginal) Bounds() []framework.Bounds {
	bounds := make([]framework.Bounds, len(kp.pods))
	for i := range bounds {
		bounds[i] = framework.Bounds{
			L: 0,
			H: float64(len(kp.nodes) - 1),
		}
	}
	return bounds
}

func (kp *KubernetesProblemOriginal) Initialize(size int) []framework.Solution {
	solutions := make([]framework.Solution, size)
	for i := 0; i < size; i++ {
		bounds := make([]framework.IntBounds, len(kp.pods))
		variables := make([]int, len(kp.pods))

		for j, pod := range kp.pods {
			bounds[j] = framework.IntBounds{L: 0, H: len(kp.nodes) - 1}
			variables[j] = pod.Node
		}

		solutions[i] = &framework.IntegerSolution{
			Variables: variables,
			Bounds:    bounds,
		}
	}
	return solutions
}

func (kp *KubernetesProblemOriginal) TrueParetoFront(size int) []framework.ObjectiveSpacePoint {
	return nil
}

func (kp *KubernetesProblemOriginal) Evaluate(solution framework.Solution) []float64 {
	return []float64{
		kp.costObjective(solution),
		kp.disruptionObjective(solution),
		kp.balanceObjective(solution),
	}
}

// Helper functions for sequential optimization
func runSingleOptimizationRoundOriginal(t *testing.T, tc struct {
	name             string
	nodes            []NodeConfig
	pods             []PodConfig
	weightProfile    WeightProfile
	populationSize   int
	maxGenerations   int
	expectedBehavior string
}, nodes []framework.NodeInfo, pods []framework.PodInfo, round int) (Analysis, []Analysis) {
	// Create problem for this round with ORIGINAL cost objective
	problem := createKubernetesProblemOriginal(nodes, pods, tc.weightProfile)

	// Configure and run NSGA-II
	config := algorithms.NSGA2Config{
		PopulationSize:       tc.populationSize,
		MaxGenerations:       tc.maxGenerations,
		CrossoverProbability: 0.9,
		MutationProbability:  0.3,
		TournamentSize:       3,
		ParallelExecution:    true,
	}

	// Run NSGA-II
	nsga2 := algorithms.NewNSGAII(config, problem)
	population := nsga2.Run()

	// Get Pareto front
	fronts := algorithms.NonDominatedSort(population)
	if len(fronts) == 0 || len(fronts[0]) == 0 {
		t.Fatal("No solutions found in Pareto front")
	}

	paretoFront := fronts[0]
	t.Logf("Round %d: Found %d Pareto-optimal solutions", round, len(paretoFront))

	// Analyze solutions
	analyses := make([]Analysis, len(paretoFront))
	for i, sol := range paretoFront {
		intSol := sol.Solution.(*framework.IntegerSolution)
		obj := problem.Evaluate(sol.Solution)

		movements := 0
		for j, node := range intSol.Variables {
			if node != pods[j].Node {
				movements++
			}
		}

		analyses[i] = Analysis{
			Assignment:    intSol.Variables,
			Cost:          obj[0],
			Disruption:    obj[1],
			Balance:       obj[2],
			WeightedTotal: obj[0]*tc.weightProfile.Cost + obj[1]*tc.weightProfile.Disruption + obj[2]*tc.weightProfile.Balance,
			Movements:     movements,
		}
	}

	// Sort by weighted total to find best
	for i := 0; i < len(analyses)-1; i++ {
		for j := i + 1; j < len(analyses); j++ {
			if analyses[i].WeightedTotal > analyses[j].WeightedTotal {
				analyses[i], analyses[j] = analyses[j], analyses[i]
			}
		}
	}

	// Deduplicate solutions
	uniqueAnalyses := []Analysis{}
	seenAssignments := make(map[string]bool)

	for _, a := range analyses {
		key := fmt.Sprintf("%v", a.Assignment)
		if !seenAssignments[key] {
			seenAssignments[key] = true
			uniqueAnalyses = append(uniqueAnalyses, a)
		}
	}

	// Show top 5 solutions for each round
	maxToShow := 5
	if len(uniqueAnalyses) < maxToShow {
		maxToShow = len(uniqueAnalyses)
	}

	t.Logf("\nTop %d solutions for round %d:", maxToShow, round)
	for i := 0; i < maxToShow; i++ {
		a := uniqueAnalyses[i]
		isInitial := a.Movements == 0
		marker := ""
		if isInitial {
			marker = " [CURRENT STATE]"
		}
		if i == 0 {
			marker += " [BEST]"
		}

		// Add movement category for clarity
		var category string
		switch {
		case a.Movements == 0:
			category = "No change"
		case a.Movements <= 2:
			category = "Minimal change"
		case a.Movements <= 5:
			category = "Small change"
		case a.Movements <= 10:
			category = "Medium change"
		default:
			category = "Large change"
		}

		t.Logf("\n%d. [%s] Movements: %d%s", i+1, category, a.Movements, marker)
		t.Logf("   Objectives: Cost=%.4f, Disruption=%.4f, Balance=%.4f, Weighted=%.4f",
			a.Cost, a.Disruption, a.Balance, a.WeightedTotal)
		t.Logf("   Assignment: %v", a.Assignment)

		// Show specific movements for all solutions in top 5
		if a.Movements > 0 {
			showDetailedMovementsOriginal(t, a.Assignment, pods, tc.nodes, "   ")
		} else {
			t.Logf("   📋 No pod movements (current state)")
		}
	}

	return uniqueAnalyses[0], uniqueAnalyses // Return best solution and all solutions
}

func showCurrentStateOriginal(t *testing.T, pods []PodConfig, nodes []NodeConfig) {
	// Show node distribution
	nodeUsage := make(map[int][]string)
	nodeCPU := make(map[int]float64)
	nodeMem := make(map[int]float64)

	for _, pod := range pods {
		if pod.Node >= 0 && pod.Node < len(nodes) {
			nodeUsage[pod.Node] = append(nodeUsage[pod.Node], pod.Name)
			nodeCPU[pod.Node] += pod.CPU
			nodeMem[pod.Node] += pod.Mem
		}
	}

	t.Logf("Current cluster state:")
	for i, node := range nodes {
		podCount := len(nodeUsage[i])
		cpuUtil := 0.0
		memUtil := 0.0
		if node.CPU > 0 {
			cpuUtil = (nodeCPU[i] / node.CPU) * 100
		}
		if node.Mem > 0 {
			memUtil = (nodeMem[i] / node.Mem) * 100
		}

		t.Logf("  %s (%s %s): %d pods, CPU: %.1f%%, Mem: %.1f%%",
			node.Name, node.Type, node.Lifecycle, podCount, cpuUtil, memUtil)
	}
}

func calculateFeasibleMovementsOriginal(targetAssignment []int, pods []PodConfig, nodes []NodeConfig) []string {
	feasibleMoves := []string{}

	// Group pods by replica set
	replicaSets := make(map[string][]int) // RS name -> pod indices
	for i, pod := range pods {
		if replicaSets[pod.RS] == nil {
			replicaSets[pod.RS] = []int{}
		}
		replicaSets[pod.RS] = append(replicaSets[pod.RS], i)
	}

	// For each replica set, determine how many pods we can move
	for _, podIndices := range replicaSets {
		// Find maxUnavailable for this RS
		maxUnavailable := 1
		if len(podIndices) > 0 {
			maxUnavailable = pods[podIndices[0]].MaxUnavail
			if maxUnavailable <= 0 {
				continue // Cannot move any pods from this RS
			}
		}

		// Count how many pods need to move from this RS
		podsToMove := []int{}
		for _, idx := range podIndices {
			if targetAssignment[idx] != pods[idx].Node {
				podsToMove = append(podsToMove, idx)
			}
		}

		// Can only move up to maxUnavailable pods
		moveCount := len(podsToMove)
		if moveCount > maxUnavailable {
			moveCount = maxUnavailable
		}

		// Add the feasible moves
		for i := 0; i < moveCount; i++ {
			feasibleMoves = append(feasibleMoves, pods[podsToMove[i]].Name)
		}
	}

	return feasibleMoves
}

func showEvolutionSummaryOriginal(t *testing.T, history []EvolutionStep, weights WeightProfile, nodes []NodeConfig, pods []PodConfig, initialClusterState []int) {
	t.Logf("\n================================================================================")
	t.Logf("📊 CLUSTER EVOLUTION SUMMARY (ORIGINAL COST OBJECTIVE)")
	t.Logf("================================================================================")

	t.Logf("\nObjective evolution over %d rounds:", len(history))
	t.Logf("Round | Target/Applied |   Cost   | Disruption | Balance  | Weighted | Changes from Previous")
	t.Logf("------|----------------|----------|------------|----------|----------|----------------------")

	for _, step := range history {
		changesStr := "Initial state"
		if len(step.changes) > 0 {
			changesStr = fmt.Sprintf("%v", step.changes)
		}

		movementStr := fmt.Sprintf("%2d/%2d", step.targetMovements, step.appliedMovements)
		if step.targetMovements == step.appliedMovements {
			movementStr = fmt.Sprintf("  %2d  ", step.targetMovements) // No PDB constraints
		}

		t.Logf("  %2d  |     %s     |  %.4f  |   %.4f   |  %.4f  |  %.4f  | %s",
			step.round, movementStr, step.bestSol.Cost, step.bestSol.Disruption,
			step.bestSol.Balance, step.bestSol.WeightedTotal, changesStr)
	}

	// Show convergence analysis
	if len(history) > 1 {
		firstRound := history[0].bestSol
		lastRound := history[len(history)-1].bestSol

		totalCostImprovement := firstRound.Cost - lastRound.Cost
		totalDisruptionCost := lastRound.Disruption - firstRound.Disruption
		totalBalanceImprovement := firstRound.Balance - lastRound.Balance
		totalWeightedImprovement := firstRound.WeightedTotal - lastRound.WeightedTotal

		t.Logf("\n📈 Overall improvement from Round 1 to Round %d:", len(history))
		t.Logf("   Cost improvement: %+.4f", totalCostImprovement)
		t.Logf("   Disruption cost: %+.4f", totalDisruptionCost)
		t.Logf("   Balance improvement: %+.4f", totalBalanceImprovement)
		t.Logf("   Total weighted improvement: %+.4f", totalWeightedImprovement)

		// Show incremental execution analysis
		totalTargetMovements := 0
		totalAppliedMovements := 0
		for _, step := range history {
			totalTargetMovements += step.targetMovements
			totalAppliedMovements += step.appliedMovements
		}

		t.Logf("\n🎯 Incremental Execution Analysis:")
		t.Logf("   Total target movements across all rounds: %d", totalTargetMovements)
		t.Logf("   Total applied movements across all rounds: %d", totalAppliedMovements)
		if totalTargetMovements > 0 {
			executionRate := float64(totalAppliedMovements) / float64(totalTargetMovements) * 100
			t.Logf("   Execution rate: %.1f%% (PDB constraints)", executionRate)
		}

		// Show round-by-round execution progress
		t.Logf("\n📋 Round-by-round execution:")
		for _, step := range history {
			if step.targetMovements > 0 {
				rate := float64(step.appliedMovements) / float64(step.targetMovements) * 100
				t.Logf("   Round %d: %d/%d movements (%.1f%% executed)",
					step.round, step.appliedMovements, step.targetMovements, rate)
			} else {
				t.Logf("   Round %d: No movements needed", step.round)
			}
		}

		// Check for convergence
		lastFewRounds := 3
		if len(history) >= lastFewRounds {
			converged := true
			baseWeighted := history[len(history)-lastFewRounds].bestSol.WeightedTotal
			for j := len(history) - lastFewRounds + 1; j < len(history); j++ {
				if history[j].bestSol.WeightedTotal < baseWeighted-0.001 { // Significant improvement
					converged = false
					break
				}
			}

			if converged {
				t.Logf("   🎯 Optimization appears to have converged (no significant improvement in last %d rounds)", lastFewRounds)
			} else {
				t.Logf("   🔄 Optimization still improving (significant changes in recent rounds)")
			}
		}
	}

	// Show final best solution details with actual costs and percentages
	if len(history) > 0 {
		finalBest := history[len(history)-1].bestSol
		t.Logf("\n🏆 FINAL BEST SOLUTION DETAILS:")
		t.Logf("   Total movements from initial: %d", finalBest.Movements)
		t.Logf("   Final objectives: Cost=%.4f, Disruption=%.4f, Balance=%.4f, Weighted=%.4f",
			finalBest.Cost, finalBest.Disruption, finalBest.Balance, finalBest.WeightedTotal)

		// Calculate actual dollar cost and balance percentage
		actualCost, balancePercent := calculateActualMetrics(finalBest.Assignment, nodes, pods)
		t.Logf("\n💰 ACTUAL COST & BALANCE METRICS:")
		t.Logf("   Total cluster cost: $%.2f/hour", actualCost)
		t.Logf("   Load balance: %.1f%% (lower = better balanced)", balancePercent)

		// Compare with TRUE initial cluster state (before any optimization)
		trueInitialCost, trueInitialBalance := calculateActualMetrics(initialClusterState, nodes, pods)
		costSavings := trueInitialCost - actualCost
		balanceImprovement := trueInitialBalance - balancePercent

		t.Logf("\n📊 IMPROVEMENT FROM TRUE INITIAL STATE:")
		t.Logf("   Initial cluster cost: $%.2f/hour, Load balance: %.1f%%", trueInitialCost, trueInitialBalance)
		t.Logf("   Final cluster cost: $%.2f/hour, Load balance: %.1f%%", actualCost, balancePercent)
		t.Logf("   Cost savings: $%.2f/hour (%.1f%%)", costSavings, (costSavings/trueInitialCost)*100)
		t.Logf("   Balance improvement: %.1f percentage points", balanceImprovement)

		// Annual savings estimate
		annualSavings := costSavings * 24 * 365
		t.Logf("   Estimated annual savings: $%.0f", annualSavings)

		// Show detailed pod migration analysis from TRUE initial state to final state
		showPodMigrationAnalysisOriginal(t, initialClusterState, finalBest.Assignment, nodes, pods)
	}
}

// calculateActualMetrics calculates actual dollar cost and balance percentage for a solution
func calculateActualMetrics(assignment []int, nodes []NodeConfig, pods []PodConfig) (float64, float64) {
	// Calculate actual dollar cost
	activeNodes := make(map[int]bool)
	for _, nodeIdx := range assignment {
		if nodeIdx >= 0 && nodeIdx < len(nodes) {
			activeNodes[nodeIdx] = true
		}
	}

	totalCost := 0.0
	for nodeIdx := range activeNodes {
		// Get the hourly cost for this node
		c, err := cost.GetInstanceCost(nodes[nodeIdx].Region, nodes[nodeIdx].Type, nodes[nodeIdx].Lifecycle)
		if err == nil {
			totalCost += c
		}
	}

	// Calculate balance percentage (coefficient of variation)
	nodeCounts := make([]int, len(nodes))
	for _, nodeIdx := range assignment {
		if nodeIdx >= 0 && nodeIdx < len(nodes) {
			nodeCounts[nodeIdx]++
		}
	}

	// Calculate mean and standard deviation of pod counts across active nodes
	activeCount := len(activeNodes)
	if activeCount == 0 {
		return totalCost, 0.0
	}

	totalPods := 0
	for nodeIdx := range activeNodes {
		totalPods += nodeCounts[nodeIdx]
	}

	mean := float64(totalPods) / float64(activeCount)

	variance := 0.0
	for nodeIdx := range activeNodes {
		diff := float64(nodeCounts[nodeIdx]) - mean
		variance += diff * diff
	}
	variance /= float64(activeCount)

	stdDev := 0.0
	if variance > 0 {
		stdDev = variance * variance // Simplified - just use variance squared for relative comparison
	}

	// Balance percentage: coefficient of variation * 100
	balancePercent := 0.0
	if mean > 0 {
		balancePercent = (stdDev / mean) * 100
	}

	return totalCost, balancePercent
}

// showPodMigrationAnalysisOriginal shows detailed analysis of pod movements from initial to final state
func showPodMigrationAnalysisOriginal(t *testing.T, initialAssignment []int, finalAssignment []int, nodes []NodeConfig, pods []PodConfig) {
	t.Logf("\n🔄 DETAILED POD MIGRATION ANALYSIS (ORIGINAL COST OBJECTIVE):")
	t.Logf("================================================================================")

	// Group movements by migration type
	migrationTypes := make(map[string][]string)
	stayedPods := []string{}
	totalMovements := 0

	for i, pod := range pods {
		initialNode := initialAssignment[i]
		finalNode := finalAssignment[i]

		if initialNode == finalNode {
			// Pod didn't move
			nodeInfo := fmt.Sprintf("%s (%s %s)", nodes[initialNode].Name, nodes[initialNode].Type, nodes[initialNode].Lifecycle)
			stayedPods = append(stayedPods, fmt.Sprintf("%s: stayed on %s", pod.Name, nodeInfo))
		} else {
			// Pod moved
			totalMovements++
			initialNodeInfo := nodes[initialNode]
			finalNodeInfo := nodes[finalNode]

			// Categorize the migration
			var migrationType string
			if initialNodeInfo.Lifecycle == "on-demand" && finalNodeInfo.Lifecycle == "spot" {
				migrationType = "💰 On-demand → Spot (cost optimization)"
			} else if initialNodeInfo.Lifecycle == "spot" && finalNodeInfo.Lifecycle == "on-demand" {
				migrationType = "🛡️ Spot → On-demand (reliability upgrade)"
			} else if initialNodeInfo.Type != finalNodeInfo.Type {
				migrationType = fmt.Sprintf("🔄 Instance type change (%s → %s)", initialNodeInfo.Type, finalNodeInfo.Type)
			} else {
				migrationType = "🔀 Same type migration"
			}

			// Calculate cost impact for this pod
			initialCost, _ := cost.GetInstanceCost(initialNodeInfo.Region, initialNodeInfo.Type, initialNodeInfo.Lifecycle)
			finalCost, _ := cost.GetInstanceCost(finalNodeInfo.Region, finalNodeInfo.Type, finalNodeInfo.Lifecycle)

			podCostImpact := ""
			if initialCost > 0 && finalCost > 0 {
				costDelta := initialCost - finalCost
				podCostImpact = fmt.Sprintf(" [Δ$%.3f/hr per node]", costDelta)
			}

			migrationInfo := fmt.Sprintf("%s: %s (%s %s) → %s (%s %s) [%.1f cores, %.1f GiB]%s",
				pod.Name,
				initialNodeInfo.Name, initialNodeInfo.Type, initialNodeInfo.Lifecycle,
				finalNodeInfo.Name, finalNodeInfo.Type, finalNodeInfo.Lifecycle,
				pod.CPU/1000.0, pod.Mem/1e9,
				podCostImpact)

			if migrationTypes[migrationType] == nil {
				migrationTypes[migrationType] = []string{}
			}
			migrationTypes[migrationType] = append(migrationTypes[migrationType], migrationInfo)
		}
	}

	t.Logf("📊 Migration Summary: %d pods moved, %d pods stayed", totalMovements, len(stayedPods))

	// Show migrations by type
	if len(migrationTypes) > 0 {
		t.Logf("\n🚀 Pod Migrations by Type:")
		for migrationType, migrations := range migrationTypes {
			t.Logf("\n%s (%d pods):", migrationType, len(migrations))
			for _, migration := range migrations {
				t.Logf("   - %s", migration)
			}
		}
	}

	// Show pods that stayed (for completeness)
	if len(stayedPods) > 0 && len(stayedPods) <= 10 { // Only show if not too many
		t.Logf("\n✅ Pods that stayed in place (%d pods):", len(stayedPods))
		for _, stayed := range stayedPods {
			t.Logf("   - %s", stayed)
		}
	} else if len(stayedPods) > 10 {
		t.Logf("\n✅ %d pods stayed in place (already optimally placed)", len(stayedPods))
	}

	// Show node utilization changes
	t.Logf("\n📈 Node Utilization Changes:")
	showNodeUtilizationChangesOriginal(t, initialAssignment, finalAssignment, nodes, pods)
}

// showNodeUtilizationChangesOriginal shows how node utilization changed from initial to final state
func showNodeUtilizationChangesOriginal(t *testing.T, initialAssignment []int, finalAssignment []int, nodes []NodeConfig, pods []PodConfig) {
	// Calculate initial utilization
	initialNodeUsage := make(map[int]struct {
		pods     int
		cpu, mem float64
	})
	for i, pod := range pods {
		nodeIdx := initialAssignment[i]
		if nodeIdx >= 0 && nodeIdx < len(nodes) {
			usage := initialNodeUsage[nodeIdx]
			usage.pods++
			usage.cpu += pod.CPU
			usage.mem += pod.Mem
			initialNodeUsage[nodeIdx] = usage
		}
	}

	// Calculate final utilization
	finalNodeUsage := make(map[int]struct {
		pods     int
		cpu, mem float64
	})
	for i, pod := range pods {
		nodeIdx := finalAssignment[i]
		if nodeIdx >= 0 && nodeIdx < len(nodes) {
			usage := finalNodeUsage[nodeIdx]
			usage.pods++
			usage.cpu += pod.CPU
			usage.mem += pod.Mem
			finalNodeUsage[nodeIdx] = usage
		}
	}

	// Show changes for each node
	for i, node := range nodes {
		initialUsage := initialNodeUsage[i]
		finalUsage := finalNodeUsage[i]

		initialCpuPct := (initialUsage.cpu / node.CPU) * 100
		finalCpuPct := (finalUsage.cpu / node.CPU) * 100
		initialMemPct := (initialUsage.mem / node.Mem) * 100
		finalMemPct := (finalUsage.mem / node.Mem) * 100

		// Get node cost for context
		nodeCost, _ := cost.GetInstanceCost(node.Region, node.Type, node.Lifecycle)

		// Show status change
		var status string
		if initialUsage.pods == 0 && finalUsage.pods == 0 {
			status = "⚫ Unused"
		} else if initialUsage.pods > 0 && finalUsage.pods == 0 {
			status = "📴 Deactivated"
		} else if initialUsage.pods == 0 && finalUsage.pods > 0 {
			status = "🟢 Activated"
		} else if finalUsage.pods > initialUsage.pods {
			status = "📈 More pods"
		} else if finalUsage.pods < initialUsage.pods {
			status = "📉 Fewer pods"
		} else {
			status = "➡️ Same"
		}

		t.Logf("   %s %s (%s %s, $%.3f/hr): %d→%d pods, CPU %.1f%%→%.1f%%, Mem %.1f%%→%.1f%%",
			status, node.Name, node.Type, node.Lifecycle, nodeCost,
			initialUsage.pods, finalUsage.pods,
			initialCpuPct, finalCpuPct, initialMemPct, finalMemPct)
	}
}
