package cost

import (
	"log"
	"time"

	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
)

// CostObjective creates a cost objective function compatible with the framework
// Uses Deb's normalization: (f - f_min) / (f_max - f_min)
func CostObjective(pods []framework.PodInfo, nodes []framework.NodeInfo) framework.ObjectiveFunc {
	// Pre-compute bounds once - these are constant for the problem instance

	// Calculate max possible cost (all nodes active)
	maxCost := 0.0
	for _, node := range nodes {
		maxCost += node.HourlyCost
	}

	// Calculate initial minimum cost estimate using bin packing solver
	log.Printf("Computing minimum cost bound for %d pods and %d nodes...", len(pods), len(nodes))
	startTime := time.Now()
	initialMinCost := BinPackMinCost(pods, nodes)
	log.Printf("Initial minimum cost estimate (BFD): %.4f (computed in %v)", initialMinCost, time.Since(startTime))

	// Track minimum costs - one for current use, one being updated
	currentMinCost := initialMinCost // Used for normalization
	pendingMinCost := initialMinCost // Tracks best seen in current generation
	evaluationCount := 0
	generationSize := 0 // Will be set on first batch of evaluations

	// Return the objective function with generation-aware minimum tracking
	return func(sol framework.Solution) float64 {
		solution := sol.(*framework.IntegerSolution)
		// Track which nodes have pods assigned
		nodeHasPods := make([]bool, len(nodes))

		for _, nodeIdx := range solution.Variables {
			if nodeIdx >= 0 && nodeIdx < len(nodes) {
				nodeHasPods[nodeIdx] = true
			}
		}

		// Sum cost of all nodes that have at least one pod
		// Empty nodes can be terminated and thus cost nothing
		totalCost := 0.0
		for i, hasPods := range nodeHasPods {
			if hasPods {
				totalCost += nodes[i].HourlyCost
			}
		}

		// Track the best cost seen in this generation
		if totalCost < pendingMinCost {
			pendingMinCost = totalCost
		}

		evaluationCount++

		// Heuristic: detect generation boundary
		// NSGA-II typically evaluates population_size solutions per generation
		// We detect this by looking for evaluation patterns
		if generationSize == 0 && evaluationCount >= 100 {
			// First generation - estimate size
			generationSize = evaluationCount
		} else if generationSize > 0 && evaluationCount%generationSize == 0 {
			// Generation boundary detected - update the minimum
			if pendingMinCost < currentMinCost {
				log.Printf("Generation boundary: updating minimum cost from %.4f to %.4f",
					currentMinCost, pendingMinCost)
				currentMinCost = pendingMinCost
			}
		}

		// Use Deb's normalization with current generation's minimum
		if maxCost == currentMinCost {
			return 0.0 // All solutions have same cost
		}

		// Calculate normalized value - can be negative if solution is better than current min
		normalized := (totalCost - currentMinCost) / (maxCost - currentMinCost)

		// Log if we found a solution better than our current minimum
		if normalized < 0 {
			log.Printf("Found solution better than current min! Cost: %.4f (current min: %.4f), normalized: %.4f",
				totalCost, currentMinCost, normalized)
		}

		return normalized
	}
}

// BinPackMinCost computes the minimum cost for the actual bin packing problem
// Uses Best Fit Decreasing approximation for all cases (no LP solver dependency)
func BinPackMinCost(pods []framework.PodInfo, nodes []framework.NodeInfo) float64 {
	return BestFitDecreasing(pods, nodes)
}

// BinPackMinCostWithDetails returns both the minimum cost and the pod assignments
// Returns the cost and a slice where assignments[i] = j means pod i is on node j
func BinPackMinCostWithDetails(pods []framework.PodInfo, nodes []framework.NodeInfo) (float64, []int) {
	return BestFitDecreasingWithDetails(pods, nodes)
}
