package balance

import (
	"math"

	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
)

// ResourceInfo represents resource capacity and allocation for a node
type ResourceInfo struct {
	CPUCapacity  float64 // in millicores
	CPUAllocated float64 // in millicores
	MemCapacity  float64 // in bytes
	MemAllocated float64 // in bytes
}

// PodResources represents the resource requirements of a pod
type PodResources struct {
	CPURequest float64 // in millicores
	MemRequest float64 // in bytes
}

// BalanceConfig contains weights and normalization parameters
type BalanceConfig struct {
	CPUWeight float64
	MemWeight float64

	// Normalization parameters
	// Maximum expected standard deviation for normalization
	// Default is 50 (theoretical max for 0-100% range)
	MaxStdDev float64
}

// DefaultBalanceConfig returns a balanced configuration
func DefaultBalanceConfig() BalanceConfig {
	return BalanceConfig{
		CPUWeight: 0.5,
		MemWeight: 0.5,
		MaxStdDev: 50.0, // Theoretical max for 0-100% range
	}
}

// BalanceResult contains detailed balance metrics
type BalanceResult struct {
	CPUStdDev           float64
	MemStdDev           float64
	NormalizedCPUStdDev float64
	NormalizedMemStdDev float64
	WeightedCPU         float64
	WeightedMem         float64
	TotalCost           float64
	NodeUtilizations    []NodeUtilization
}

// NodeUtilization tracks utilization for a specific node
type NodeUtilization struct {
	NodeIndex      int
	CPUUtilization float64 // percentage (0-100)
	MemUtilization float64 // percentage (0-100)
}

// BalanceObjective calculates the load balance cost using standard deviation
func BalanceObjective(assignment []int, pods []PodResources, nodes []ResourceInfo, config BalanceConfig) float64 {
	result := calculateBalance(assignment, pods, nodes, config)
	return result.TotalCost
}

// BalanceObjectiveWithDetails returns both cost and detailed metrics
func BalanceObjectiveWithDetails(assignment []int, pods []PodResources, nodes []ResourceInfo, config BalanceConfig) (float64, BalanceResult) {
	result := calculateBalance(assignment, pods, nodes, config)
	return result.TotalCost, result
}

// BalanceObjectiveFunc returns a function compatible with the optimization framework
func BalanceObjectiveFunc(pods []PodResources, nodes []ResourceInfo, config BalanceConfig) func(framework.Solution) float64 {
	return func(sol framework.Solution) float64 {
		intSol, ok := sol.(*framework.IntegerSolution)
		if !ok {
			return math.Inf(1)
		}
		return BalanceObjective(intSol.Variables, pods, nodes, config)
	}
}

// BalanceObjectiveFuncWithDetails returns a details function for the framework
func BalanceObjectiveFuncWithDetails(pods []PodResources, nodes []ResourceInfo, config BalanceConfig) func(framework.Solution) interface{} {
	return func(sol framework.Solution) interface{} {
		intSol, ok := sol.(*framework.IntegerSolution)
		if !ok {
			return BalanceResult{TotalCost: math.Inf(1)}
		}
		_, result := BalanceObjectiveWithDetails(intSol.Variables, pods, nodes, config)
		return result
	}
}

// calculateBalance computes the load balance metrics
func calculateBalance(assignment []int, pods []PodResources, nodes []ResourceInfo, config BalanceConfig) BalanceResult {
	numNodes := len(nodes)
	if numNodes == 0 {
		return BalanceResult{TotalCost: 0}
	}

	// Calculate allocated resources per node based on assignment
	nodeAllocations := make([]ResourceInfo, numNodes)
	for i := range nodeAllocations {
		nodeAllocations[i] = ResourceInfo{
			CPUCapacity: nodes[i].CPUCapacity,
			MemCapacity: nodes[i].MemCapacity,
			// Start with existing allocations (pods not in our assignment)
			CPUAllocated: nodes[i].CPUAllocated,
			MemAllocated: nodes[i].MemAllocated,
		}
	}

	// Add pod resources to assigned nodes
	for podIdx, nodeIdx := range assignment {
		if nodeIdx >= 0 && nodeIdx < numNodes {
			nodeAllocations[nodeIdx].CPUAllocated += pods[podIdx].CPURequest
			nodeAllocations[nodeIdx].MemAllocated += pods[podIdx].MemRequest
		}
	}

	// Calculate utilization percentages
	utilizations := make([]NodeUtilization, numNodes)
	cpuUtils := make([]float64, numNodes)
	memUtils := make([]float64, numNodes)

	for i, node := range nodeAllocations {
		cpuUtil := 0.0
		if node.CPUCapacity > 0 {
			cpuUtil = (node.CPUAllocated / node.CPUCapacity) * 100
		}

		memUtil := 0.0
		if node.MemCapacity > 0 {
			memUtil = (node.MemAllocated / node.MemCapacity) * 100
		}

		utilizations[i] = NodeUtilization{
			NodeIndex:      i,
			CPUUtilization: cpuUtil,
			MemUtilization: memUtil,
		}

		cpuUtils[i] = cpuUtil
		memUtils[i] = memUtil
	}

	// Calculate standard deviations
	cpuStdDev := standardDeviation(cpuUtils)
	memStdDev := standardDeviation(memUtils)

	// Normalize standard deviations
	normalizedCPU := cpuStdDev / config.MaxStdDev
	normalizedMem := memStdDev / config.MaxStdDev

	// Apply weights to normalized values
	weightedCPU := normalizedCPU * config.CPUWeight
	weightedMem := normalizedMem * config.MemWeight

	return BalanceResult{
		CPUStdDev:           cpuStdDev,
		MemStdDev:           memStdDev,
		NormalizedCPUStdDev: normalizedCPU,
		NormalizedMemStdDev: normalizedMem,
		WeightedCPU:         weightedCPU,
		WeightedMem:         weightedMem,
		TotalCost:           weightedCPU + weightedMem,
		NodeUtilizations:    utilizations,
	}
}

// standardDeviation calculates the standard deviation of a slice of values
func standardDeviation(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Calculate mean
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	// Calculate variance
	variance := 0.0
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(values))

	// Return standard deviation
	return math.Sqrt(variance)
}
