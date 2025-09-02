package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
)

// Solution represents a specific pod assignment
type Solution struct {
	Name       string
	Assignment []int
}

// AnalysisConfig contains all tunable parameters
type AnalysisConfig struct {
	// Objective weights (should sum to 1.0)
	CostWeight       float64
	DisruptionWeight float64
	BalanceWeight    float64

	// Disruption component weights
	MovementWeight  float64
	ColdStartWeight float64
	TimeSlotWeight  float64

	// Other parameters
	MaxStdDev       float64 // For balance normalization
	ColdStartPerPod float64 // Seconds
}

func main() {
	// Default config - can be overridden by command line args
	config := AnalysisConfig{
		CostWeight:       0.60,
		DisruptionWeight: 0.20,
		BalanceWeight:    0.20,

		MovementWeight:  0.15,
		ColdStartWeight: 0.20,
		TimeSlotWeight:  0.20,

		MaxStdDev:       50.0,
		ColdStartPerPod: 10.0,
	}

	// Parse command line args if provided
	if len(os.Args) > 1 {
		config.CostWeight, _ = strconv.ParseFloat(os.Args[1], 64)
	}
	if len(os.Args) > 2 {
		config.DisruptionWeight, _ = strconv.ParseFloat(os.Args[2], 64)
	}
	if len(os.Args) > 3 {
		config.BalanceWeight, _ = strconv.ParseFloat(os.Args[3], 64)
	}

	// MixedNodeTypes scenario
	nodes := []NodeInfo{
		{Name: "node-1", Type: "t3.small", Region: "us-east-1", Lifecycle: "spot", CPU: 2000, Mem: 4e9},
		{Name: "node-2", Type: "t3.small", Region: "us-east-1", Lifecycle: "spot", CPU: 2000, Mem: 4e9},
		{Name: "node-3", Type: "m5.large", Region: "us-east-1", Lifecycle: "on-demand", CPU: 4000, Mem: 8e9},
		{Name: "node-4", Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand", CPU: 8000, Mem: 16e9},
	}

	pods := []PodInfo{
		{Name: "small-1", CPU: 500, Mem: 1e9, CurrentNode: 3, RS: "small", MaxUnavail: 2},
		{Name: "small-2", CPU: 500, Mem: 1e9, CurrentNode: 3, RS: "small", MaxUnavail: 2},
		{Name: "small-3", CPU: 500, Mem: 1e9, CurrentNode: 3, RS: "small", MaxUnavail: 2},
		{Name: "medium-1", CPU: 1000, Mem: 2e9, CurrentNode: 3, RS: "medium", MaxUnavail: 1},
		{Name: "medium-2", CPU: 1000, Mem: 2e9, CurrentNode: 3, RS: "medium", MaxUnavail: 1},
		{Name: "large-1", CPU: 2000, Mem: 4e9, CurrentNode: 3, RS: "large", MaxUnavail: 1},
	}

	// Solutions to analyze
	solutions := []Solution{
		{Name: "Current State", Assignment: []int{3, 3, 3, 3, 3, 3}},
		{Name: "Pack on t3.small", Assignment: []int{0, 0, 0, 2, 2, 2}},
		{Name: "Balanced 3 nodes", Assignment: []int{0, 1, 1, 0, 2, 2}},
		{Name: "Move 1 pod", Assignment: []int{3, 1, 3, 3, 3, 3}},
		{Name: "Move 2 pods", Assignment: []int{3, 1, 3, 3, 0, 3}},
	}

	fmt.Printf("\n=== Solution Analysis Tool ===\n")
	fmt.Printf("Weights: Cost=%.2f, Disruption=%.2f, Balance=%.2f\n",
		config.CostWeight, config.DisruptionWeight, config.BalanceWeight)
	fmt.Printf("Disruption components: Movement=%.2f, ColdStart=%.2f, TimeSlot=%.2f\n\n",
		config.MovementWeight, config.ColdStartWeight, config.TimeSlotWeight)

	// Analyze each solution
	results := make([]SolutionResult, len(solutions))
	for i, sol := range solutions {
		results[i] = analyzeSolution(sol, nodes, pods, config)
		printDetailedAnalysis(results[i], config)
	}

	// Rank solutions
	fmt.Println("\n=== RANKING BY WEIGHTED TOTAL ===")
	ranked := rankSolutions(results)
	for i, r := range ranked {
		fmt.Printf("%d. %s: Weighted=%.4f (Cost=%.4f, Disr=%.4f, Bal=%.4f)\n",
			i+1, r.Name, r.WeightedTotal, r.Cost.Normalized,
			r.Disruption.Normalized, r.Balance.Normalized)
	}

	// Show sensitivity analysis
	fmt.Println("\n=== SENSITIVITY ANALYSIS ===")
	fmt.Println("How rankings change with different weights:")
	testWeights := []struct {
		cost, disr, bal float64
		desc            string
	}{
		{0.80, 0.10, 0.10, "Cost-focused"},
		{0.10, 0.80, 0.10, "Disruption-averse"},
		{0.10, 0.10, 0.80, "Balance-focused"},
		{0.33, 0.33, 0.34, "Equal weights"},
	}

	for _, tw := range testWeights {
		tempConfig := config
		tempConfig.CostWeight = tw.cost
		tempConfig.DisruptionWeight = tw.disr
		tempConfig.BalanceWeight = tw.bal

		// Re-analyze with new weights
		for i := range results {
			results[i] = analyzeSolution(solutions[i], nodes, pods, tempConfig)
		}

		ranked := rankSolutions(results)
		fmt.Printf("\n%s (%.0f%%/%.0f%%/%.0f%%): %s wins with %.4f\n",
			tw.desc, tw.cost*100, tw.disr*100, tw.bal*100,
			ranked[0].Name, ranked[0].WeightedTotal)
	}
}

type NodeInfo struct {
	Name      string
	Type      string
	Region    string
	Lifecycle string
	CPU       float64
	Mem       float64
}

type PodInfo struct {
	Name        string
	CPU         float64
	Mem         float64
	CurrentNode int
	RS          string
	MaxUnavail  int
}

type ObjectiveBreakdown struct {
	Raw        float64
	Normalized float64
	Weighted   float64
	Details    map[string]float64
}

type SolutionResult struct {
	Name          string
	Assignment    []int
	Cost          ObjectiveBreakdown
	Disruption    ObjectiveBreakdown
	Balance       ObjectiveBreakdown
	WeightedTotal float64
	Movements     int
}

func analyzeSolution(sol Solution, nodes []NodeInfo, pods []PodInfo, config AnalysisConfig) SolutionResult {
	result := SolutionResult{
		Name:       sol.Name,
		Assignment: sol.Assignment,
	}

	// Count movements
	for i, node := range sol.Assignment {
		if node != pods[i].CurrentNode {
			result.Movements++
		}
	}

	// COST ANALYSIS
	activeNodes := make(map[int]bool)
	for _, node := range sol.Assignment {
		activeNodes[node] = true
	}

	totalCost := 0.0
	maxPossibleCost := 0.0
	for i, node := range nodes {
		nodeCost := getNodeCost(node)
		maxPossibleCost += nodeCost
		if activeNodes[i] {
			totalCost += nodeCost
		}
	}

	result.Cost = ObjectiveBreakdown{
		Raw:        totalCost,
		Normalized: totalCost / maxPossibleCost,
		Weighted:   (totalCost / maxPossibleCost) * config.CostWeight,
		Details: map[string]float64{
			"activeNodes":     float64(len(activeNodes)),
			"maxPossibleCost": maxPossibleCost,
		},
	}

	// DISRUPTION ANALYSIS
	movedPods := result.Movements
	totalPods := len(pods)

	// Movement component
	movementRatio := float64(movedPods) / float64(totalPods)
	movementNormalized := movementRatio
	movementWeighted := movementNormalized * config.MovementWeight

	// Cold start component
	totalColdStart := float64(movedPods) * config.ColdStartPerPod
	coldStartNormalized := 0.0
	if movedPods > 0 {
		coldStartNormalized = totalColdStart / (float64(movedPods) * 60.0) // 60s baseline
	}
	coldStartWeighted := coldStartNormalized * config.ColdStartWeight

	// Time slots (simplified - assumes worst case)
	timeSlots := 1
	if movedPods > 2 {
		timeSlots = 2
	}
	if movedPods > 4 {
		timeSlots = 3
	}
	timeSlotsNormalized := float64(timeSlots) / 3.0 // Assume max 3 slots
	timeSlotsWeighted := timeSlotsNormalized * config.TimeSlotWeight

	totalDisruption := movementWeighted + coldStartWeighted + timeSlotsWeighted

	result.Disruption = ObjectiveBreakdown{
		Raw:        float64(movedPods), // Use movements as raw
		Normalized: totalDisruption,
		Weighted:   totalDisruption * config.DisruptionWeight,
		Details: map[string]float64{
			"movementRatio":     movementRatio,
			"movementWeighted":  movementWeighted,
			"coldStartTotal":    totalColdStart,
			"coldStartWeighted": coldStartWeighted,
			"timeSlots":         float64(timeSlots),
			"timeSlotsWeighted": timeSlotsWeighted,
		},
	}

	// BALANCE ANALYSIS
	nodeUtils := make([]float64, len(nodes))
	for i := range nodes {
		cpuUsed := 0.0
		memUsed := 0.0
		for j, assignment := range sol.Assignment {
			if assignment == i {
				cpuUsed += pods[j].CPU
				memUsed += pods[j].Mem
			}
		}
		cpuUtil := (cpuUsed / nodes[i].CPU) * 100
		memUtil := (memUsed / nodes[i].Mem) * 100
		nodeUtils[i] = (cpuUtil + memUtil) / 2
	}

	// Calculate standard deviation
	mean := 0.0
	for _, util := range nodeUtils {
		mean += util
	}
	mean /= float64(len(nodeUtils))

	variance := 0.0
	for _, util := range nodeUtils {
		variance += (util - mean) * (util - mean)
	}
	variance /= float64(len(nodeUtils))
	stdDevFloat := math.Sqrt(variance)

	result.Balance = ObjectiveBreakdown{
		Raw:        stdDevFloat,
		Normalized: stdDevFloat / config.MaxStdDev,
		Weighted:   (stdDevFloat / config.MaxStdDev) * config.BalanceWeight,
		Details: map[string]float64{
			"node0Util": nodeUtils[0],
			"node1Util": nodeUtils[1],
			"node2Util": nodeUtils[2],
			"node3Util": nodeUtils[3],
			"mean":      mean,
		},
	}

	// Total weighted score
	result.WeightedTotal = result.Cost.Weighted + result.Disruption.Weighted + result.Balance.Weighted

	return result
}

func getNodeCost(node NodeInfo) float64 {
	// Simplified cost lookup
	costs := map[string]float64{
		"t3.small-spot":       0.0074,
		"t3.small-on-demand":  0.0208,
		"m5.large-on-demand":  0.0960,
		"m5.xlarge-on-demand": 0.1920,
	}

	key := fmt.Sprintf("%s-%s", node.Type, node.Lifecycle)
	if cost, ok := costs[key]; ok {
		return cost
	}
	return 0.1 // Default
}

func printDetailedAnalysis(r SolutionResult, config AnalysisConfig) {
	fmt.Printf("\n=== %s ===\n", r.Name)
	fmt.Printf("Assignment: %v\n", r.Assignment)
	fmt.Printf("Movements: %d\n", r.Movements)

	fmt.Println("\nCOST BREAKDOWN:")
	fmt.Printf("  Raw cost: $%.4f/hr\n", r.Cost.Raw)
	fmt.Printf("  Active nodes: %.0f\n", r.Cost.Details["activeNodes"])
	fmt.Printf("  Max possible: $%.4f/hr\n", r.Cost.Details["maxPossibleCost"])
	fmt.Printf("  Normalized: %.4f (%.4f / %.4f)\n", r.Cost.Normalized, r.Cost.Raw, r.Cost.Details["maxPossibleCost"])
	fmt.Printf("  Weighted: %.4f (%.4f × %.2f)\n", r.Cost.Weighted, r.Cost.Normalized, config.CostWeight)

	fmt.Println("\nDISRUPTION BREAKDOWN:")
	fmt.Printf("  Moved pods: %.0f\n", r.Disruption.Raw)
	fmt.Printf("  Movement ratio: %.4f\n", r.Disruption.Details["movementRatio"])
	fmt.Printf("  - Movement component: %.4f (weight %.2f)\n", r.Disruption.Details["movementWeighted"], config.MovementWeight)
	fmt.Printf("  - Cold start total: %.0fs\n", r.Disruption.Details["coldStartTotal"])
	fmt.Printf("  - Cold start component: %.4f (weight %.2f)\n", r.Disruption.Details["coldStartWeighted"], config.ColdStartWeight)
	fmt.Printf("  - Time slots: %.0f\n", r.Disruption.Details["timeSlots"])
	fmt.Printf("  - Time slot component: %.4f (weight %.2f)\n", r.Disruption.Details["timeSlotsWeighted"], config.TimeSlotWeight)
	fmt.Printf("  Normalized total: %.4f\n", r.Disruption.Normalized)
	fmt.Printf("  Weighted: %.4f (%.4f × %.2f)\n", r.Disruption.Weighted, r.Disruption.Normalized, config.DisruptionWeight)

	fmt.Println("\nBALANCE BREAKDOWN:")
	fmt.Printf("  Node utilizations: [%.1f%%, %.1f%%, %.1f%%, %.1f%%]\n",
		r.Balance.Details["node0Util"], r.Balance.Details["node1Util"],
		r.Balance.Details["node2Util"], r.Balance.Details["node3Util"])
	fmt.Printf("  Mean utilization: %.1f%%\n", r.Balance.Details["mean"])
	fmt.Printf("  StdDev (approx): %.2f\n", r.Balance.Raw)
	fmt.Printf("  Normalized: %.4f (%.2f / %.1f)\n", r.Balance.Normalized, r.Balance.Raw, config.MaxStdDev)
	fmt.Printf("  Weighted: %.4f (%.4f × %.2f)\n", r.Balance.Weighted, r.Balance.Normalized, config.BalanceWeight)

	fmt.Printf("\nWEIGHTED TOTAL: %.4f\n", r.WeightedTotal)
}

func rankSolutions(results []SolutionResult) []SolutionResult {
	// Simple bubble sort
	sorted := make([]SolutionResult, len(results))
	copy(sorted, results)

	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-i-1; j++ {
			if sorted[j].WeightedTotal > sorted[j+1].WeightedTotal {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}

	return sorted
}
