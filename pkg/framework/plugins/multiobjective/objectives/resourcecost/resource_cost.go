package resourcecost

import (
	"log"
	"math"

	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
)

// NodeEfficiency represents the resource per dollar efficiency of a node type
type NodeEfficiency struct {
	CPUPerDollar float64 // vCPU-hours per dollar
	MemPerDollar float64 // GiB-hours per dollar
	Combined     float64 // Combined efficiency metric
}

// ResourceCostObjective minimizes cost by placing pods on nodes with better resource/$ ratios
type ResourceCostObjective struct {
	nodeEfficiencies map[int]NodeEfficiency
	maxCost          float64
	minCost          float64
}

// nodeWithEfficiency is used for sorting nodes by efficiency
type nodeWithEfficiency struct {
	idx        int
	node       NodeInfo
	efficiency float64
	usedCPU    float64
	usedMem    float64
}

// NewResourceCostObjective creates a new resource cost objective
func NewResourceCostObjective(nodes []NodeInfo, pods []PodInfo) *ResourceCostObjective {
	obj := &ResourceCostObjective{
		nodeEfficiencies: make(map[int]NodeEfficiency),
	}

	// Calculate efficiency for each node
	for i, node := range nodes {
		if node.HourlyCost > 0 {
			// Calculate resource per dollar ratios
			cpuPerDollar := node.CPUCapacity / node.HourlyCost
			memPerDollar := (node.MemCapacity / 1e9) / node.HourlyCost // Convert to GiB

			// Combined efficiency - normalized to account for different instance types
			// Normalize each metric relative to expected ratios
			// Standard ratio is ~4:1 (4 vCPU : 16 GB) for general purpose
			normalizedCPU := cpuPerDollar / 20.0 // ~20 CPU/$ is typical for general purpose
			normalizedMem := memPerDollar / 80.0 // ~80 GiB/$ is typical for general purpose
			combined := (normalizedCPU + normalizedMem) / 2

			obj.nodeEfficiencies[i] = NodeEfficiency{
				CPUPerDollar: cpuPerDollar,
				MemPerDollar: memPerDollar,
				Combined:     combined,
			}

			log.Printf("Node %d (%s %s): %.1f CPU/$/hr, %.1f GiB/$/hr, combined: %.1f",
				i, node.InstanceType, node.InstanceLifecycle,
				cpuPerDollar, memPerDollar, combined)
		}
	}

	// Calculate min/max costs for normalization
	obj.calculateBounds(nodes, pods)

	return obj
}

// ResourceCostObjectiveFunc returns a function that calculates resource cost for a solution
func ResourceCostObjectiveFunc(nodes []NodeInfo, pods []PodInfo) framework.ObjectiveFunc {
	obj := NewResourceCostObjective(nodes, pods)

	return func(sol framework.Solution) float64 {
		// Cast to IntegerSolution to access variables
		intSol, ok := sol.(*framework.IntegerSolution)
		if !ok {
			return 1.0 // Maximum cost if invalid solution type
		}

		totalCost := 0.0

		// Calculate cost for each pod based on its node's efficiency
		for podIdx, nodeIdx := range intSol.Variables {
			if nodeIdx < 0 || nodeIdx >= len(nodes) {
				continue // Skip unassigned pods
			}

			pod := pods[podIdx]
			efficiency := obj.nodeEfficiencies[nodeIdx]

			if efficiency.CPUPerDollar > 0 && efficiency.MemPerDollar > 0 {
				// Calculate pod-specific efficiency based on its resource profile
				cpuNorm := pod.CPU
				memNorm := pod.Mem / 1e9 // Convert to GiB
				totalResources := cpuNorm + memNorm

				if totalResources > 0 {
					// Weight efficiency based on pod's resource distribution
					cpuWeight := cpuNorm / totalResources
					memWeight := memNorm / totalResources

					// Pod-specific efficiency for this node
					podSpecificEfficiency := (cpuWeight * efficiency.CPUPerDollar) +
						(memWeight * efficiency.MemPerDollar)

					// Cost is resource usage divided by efficiency
					podCost := totalResources / podSpecificEfficiency
					totalCost += podCost
				}
			}
		}

		// Normalize
		if obj.maxCost == obj.minCost {
			return 0.0
		}
		return (totalCost - obj.minCost) / (obj.maxCost - obj.minCost)
	}
}

func (obj *ResourceCostObjective) calculateBounds(nodes []NodeInfo, pods []PodInfo) {
	// Create sorted node lists by efficiency
	nodesByEfficiency := make([]nodeWithEfficiency, 0, len(nodes))
	for idx, node := range nodes {
		if eff, ok := obj.nodeEfficiencies[idx]; ok && eff.Combined > 0 {
			nodesByEfficiency = append(nodesByEfficiency, nodeWithEfficiency{
				idx:        idx,
				node:       node,
				efficiency: eff.Combined,
			})
		}
	}

	// Sort by efficiency (descending for minCost)
	sortedForMin := make([]nodeWithEfficiency, len(nodesByEfficiency))
	copy(sortedForMin, nodesByEfficiency)
	for i := 0; i < len(sortedForMin)-1; i++ {
		for j := i + 1; j < len(sortedForMin); j++ {
			if sortedForMin[i].efficiency < sortedForMin[j].efficiency {
				sortedForMin[i], sortedForMin[j] = sortedForMin[j], sortedForMin[i]
			}
		}
	}

	// Sort by efficiency (ascending for maxCost)
	sortedForMax := make([]nodeWithEfficiency, len(nodesByEfficiency))
	copy(sortedForMax, nodesByEfficiency)
	for i := 0; i < len(sortedForMax)-1; i++ {
		for j := i + 1; j < len(sortedForMax); j++ {
			if sortedForMax[i].efficiency > sortedForMax[j].efficiency {
				sortedForMax[i], sortedForMax[j] = sortedForMax[j], sortedForMax[i]
			}
		}
	}

	// Calculate minCost using best-fit on most efficient nodes
	obj.minCost = obj.calculatePackingCost(sortedForMin, pods, "most efficient")

	// Calculate maxCost using best-fit on least efficient nodes
	obj.maxCost = obj.calculatePackingCost(sortedForMax, pods, "least efficient")

	// Ensure min < max
	if obj.minCost >= obj.maxCost {
		// If something went wrong, use simple bounds
		obj.maxCost = obj.minCost * 2
	}

	log.Printf("ResourceCost bounds: min=%.2f (best packing), max=%.2f (worst packing)",
		obj.minCost, obj.maxCost)
}

func (obj *ResourceCostObjective) calculatePackingCost(sortedNodes []nodeWithEfficiency, pods []PodInfo, packingType string) float64 {
	// Reset node usage
	for i := range sortedNodes {
		sortedNodes[i].usedCPU = 0
		sortedNodes[i].usedMem = 0
	}

	// Sort pods by resource size (largest first) for better packing
	type podWithSize struct {
		pod  PodInfo
		size float64
	}
	sortedPods := make([]podWithSize, len(pods))
	for i, pod := range pods {
		sortedPods[i] = podWithSize{
			pod:  pod,
			size: pod.CPU + (pod.Mem / 1e9), // Combined resource score
		}
	}
	// Sort descending by size
	for i := 0; i < len(sortedPods)-1; i++ {
		for j := i + 1; j < len(sortedPods); j++ {
			if sortedPods[i].size < sortedPods[j].size {
				sortedPods[i], sortedPods[j] = sortedPods[j], sortedPods[i]
			}
		}
	}

	totalCost := 0.0
	unplacedPods := 0

	// Try to place each pod using best-fit on the sorted nodes
	for _, podWithSize := range sortedPods {
		pod := podWithSize.pod
		bestNodeIdx := -1
		bestFit := math.MaxFloat64

		// Find the best fitting node (tightest fit that still has room)
		for i := range sortedNodes {
			node := &sortedNodes[i]
			cpuRemaining := node.node.CPUCapacity - node.usedCPU
			memRemaining := node.node.MemCapacity - node.usedMem

			if pod.CPU <= cpuRemaining && pod.Mem <= memRemaining {
				// Calculate how well this pod fits (lower is better)
				cpuFitRatio := pod.CPU / cpuRemaining
				memFitRatio := pod.Mem / memRemaining
				fitScore := math.Max(cpuFitRatio, memFitRatio) // Use the tighter constraint

				if fitScore < bestFit {
					bestFit = fitScore
					bestNodeIdx = i
				}
			}
		}

		if bestNodeIdx >= 0 {
			// Place pod on best node
			node := &sortedNodes[bestNodeIdx]
			node.usedCPU += pod.CPU
			node.usedMem += pod.Mem

			// Calculate pod-specific cost based on resource profile
			cpuNorm := pod.CPU
			memNorm := pod.Mem / 1e9
			totalResources := cpuNorm + memNorm

			if totalResources > 0 {
				cpuWeight := cpuNorm / totalResources
				memWeight := memNorm / totalResources

				// Get node efficiency from the original data
				nodeEff := obj.nodeEfficiencies[node.idx]
				podSpecificEfficiency := (cpuWeight * nodeEff.CPUPerDollar) +
					(memWeight * nodeEff.MemPerDollar)

				podCost := totalResources / podSpecificEfficiency
				totalCost += podCost
			}
		} else {
			unplacedPods++
		}
	}

	if unplacedPods > 0 {
		log.Printf("Warning: %d pods couldn't be placed in %s packing", unplacedPods, packingType)

		// Add cost for unplaced pods using average pod profile
		avgCPU := 0.0
		avgMem := 0.0
		for _, pod := range pods {
			avgCPU += pod.CPU
			avgMem += pod.Mem / 1e9
		}
		avgCPU /= float64(len(pods))
		avgMem /= float64(len(pods))
		avgTotal := avgCPU + avgMem

		if avgTotal > 0 {
			cpuWeight := avgCPU / avgTotal
			memWeight := avgMem / avgTotal

			// Use the penalty node's actual efficiencies
			penaltyNode := sortedNodes[len(sortedNodes)-1]
			if packingType == "most efficient" {
				// Find least efficient node with capacity
				for i := len(sortedNodes) - 1; i >= 0; i-- {
					if sortedNodes[i].usedCPU < sortedNodes[i].node.CPUCapacity ||
						sortedNodes[i].usedMem < sortedNodes[i].node.MemCapacity {
						penaltyNode = sortedNodes[i]
						break
					}
				}
			}

			penaltyEff := obj.nodeEfficiencies[penaltyNode.idx]
			podSpecificPenaltyEff := (cpuWeight * penaltyEff.CPUPerDollar) +
				(memWeight * penaltyEff.MemPerDollar)

			totalCost += float64(unplacedPods) * avgTotal / podSpecificPenaltyEff
		}
	}

	return totalCost
}

// NodeInfo represents node information needed for resource cost calculation
type NodeInfo struct {
	CPUCapacity       float64
	MemCapacity       float64
	HourlyCost        float64
	InstanceType      string
	InstanceLifecycle string // "on-demand" or "spot"
}

// PodInfo represents pod resource requirements
type PodInfo struct {
	CPU float64
	Mem float64
}
