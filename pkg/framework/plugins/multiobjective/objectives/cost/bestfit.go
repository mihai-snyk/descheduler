package cost

import (
	"log"
	"sort"

	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/constraints"
)

// BestFitDecreasing implements the Best Fit Decreasing bin packing algorithm
// for finding a good lower bound on the minimum cost
func BestFitDecreasing(pods []constraints.PodInfo, nodes []NodeInfo) float64 {
	if len(nodes) == 0 || len(pods) == 0 {
		return 0
	}

	// Sort pods by size (largest first) for better packing
	type indexedPod struct {
		idx  int
		pod  constraints.PodInfo
		size float64
	}

	indexedPods := make([]indexedPod, len(pods))
	for i, pod := range pods {
		// Normalize resources to create a combined size metric
		cpuScore := pod.CPURequest / 1000.0 // Normalize to cores
		memScore := pod.MemRequest / 1e9    // Normalize to GB
		indexedPods[i] = indexedPod{
			idx:  i,
			pod:  pod,
			size: cpuScore + memScore,
		}
	}

	// Sort by size descending (largest pods first)
	sort.Slice(indexedPods, func(i, j int) bool {
		return indexedPods[i].size > indexedPods[j].size
	})

	// Create node state tracking
	type nodeState struct {
		node         NodeInfo
		cpuRemaining float64
		memRemaining float64
		active       bool
		costPerUnit  float64
	}

	nodeStates := make([]nodeState, len(nodes))
	for i, node := range nodes {
		// Cost per normalized capacity unit
		totalCapacity := node.CPUCapacity/1000.0 + node.MemCapacity/1e9
		if totalCapacity == 0 {
			totalCapacity = 1 // Avoid division by zero
		}
		nodeStates[i] = nodeState{
			node:         node,
			cpuRemaining: node.CPUCapacity,
			memRemaining: node.MemCapacity,
			active:       false,
			costPerUnit:  node.CostPerHour / totalCapacity,
		}
	}

	// Sort nodes by cost efficiency
	sort.Slice(nodeStates, func(i, j int) bool {
		return nodeStates[i].costPerUnit < nodeStates[j].costPerUnit
	})

	// Best Fit Decreasing: For each pod, find the best fitting node
	unplacedPods := 0
	for _, iPod := range indexedPods {
		pod := iPod.pod
		bestNodeIdx := -1
		bestFit := -1.0

		// First, try to fit in already active nodes (best fit)
		for i := range nodeStates {
			if !nodeStates[i].active {
				continue
			}

			if nodeStates[i].cpuRemaining >= pod.CPURequest &&
				nodeStates[i].memRemaining >= pod.MemRequest {

				// Calculate how well this pod fits (smaller remainder = better fit)
				cpuFit := (nodeStates[i].cpuRemaining - pod.CPURequest) / nodeStates[i].node.CPUCapacity
				memFit := (nodeStates[i].memRemaining - pod.MemRequest) / nodeStates[i].node.MemCapacity
				fit := cpuFit + memFit

				if bestNodeIdx == -1 || fit < bestFit {
					bestNodeIdx = i
					bestFit = fit
				}
			}
		}

		// If no active node fits, activate the cheapest node that fits
		if bestNodeIdx == -1 {
			for i := range nodeStates {
				if nodeStates[i].active {
					continue
				}

				if nodeStates[i].cpuRemaining >= pod.CPURequest &&
					nodeStates[i].memRemaining >= pod.MemRequest {
					bestNodeIdx = i
					nodeStates[i].active = true
					break
				}
			}
		}

		// If we found a node, assign the pod
		if bestNodeIdx != -1 {
			nodeStates[bestNodeIdx].cpuRemaining -= pod.CPURequest
			nodeStates[bestNodeIdx].memRemaining -= pod.MemRequest
		} else {
			// Could not place this pod
			unplacedPods++
		}
	}

	// Calculate total cost of active nodes
	totalCost := 0.0
	activeCount := 0
	for _, ns := range nodeStates {
		if ns.active {
			totalCost += ns.node.CostPerHour
			activeCount++
		}
	}

	if unplacedPods > 0 {
		log.Printf("Warning: BFD could not place %d pods", unplacedPods)
		// For minimum cost calculation, we should only return the cost of nodes
		// that could be successfully used. If BFD fails, it means the problem
		// is more constrained and the true minimum might be higher.
		// Don't add artificial penalties as they skew normalization.
	}

	return totalCost
}

// BestFitDecreasingWithDetails returns both the cost and the assignments
func BestFitDecreasingWithDetails(pods []constraints.PodInfo, nodes []NodeInfo) (float64, []int) {
	if len(nodes) == 0 || len(pods) == 0 {
		return 0, nil
	}

	// Sort pods by size (largest first) for better packing
	type indexedPod struct {
		idx  int
		pod  constraints.PodInfo
		size float64
	}

	indexedPods := make([]indexedPod, len(pods))
	for i, pod := range pods {
		// Normalize resources to create a combined size metric
		cpuScore := pod.CPURequest / 1000.0 // Normalize to cores
		memScore := pod.MemRequest / 1e9    // Normalize to GB
		indexedPods[i] = indexedPod{
			idx:  i,
			pod:  pod,
			size: cpuScore + memScore,
		}
	}

	// Sort by size descending (largest pods first)
	sort.Slice(indexedPods, func(i, j int) bool {
		return indexedPods[i].size > indexedPods[j].size
	})

	// Create node state tracking
	type nodeState struct {
		node         NodeInfo
		cpuRemaining float64
		memRemaining float64
		active       bool
		costPerUnit  float64
	}

	nodeStates := make([]nodeState, len(nodes))
	for i, node := range nodes {
		// Cost per normalized capacity unit
		totalCapacity := node.CPUCapacity/1000.0 + node.MemCapacity/1e9
		if totalCapacity == 0 {
			totalCapacity = 1 // Avoid division by zero
		}
		nodeStates[i] = nodeState{
			node:         node,
			cpuRemaining: node.CPUCapacity,
			memRemaining: node.MemCapacity,
			active:       false,
			costPerUnit:  node.CostPerHour / totalCapacity,
		}
	}

	// Sort nodes by cost efficiency
	sort.Slice(nodeStates, func(i, j int) bool {
		return nodeStates[i].costPerUnit < nodeStates[j].costPerUnit
	})

	// Initialize assignments (-1 means unassigned)
	assignments := make([]int, len(pods))
	for i := range assignments {
		assignments[i] = -1
	}

	// Best Fit Decreasing: For each pod, find the best fitting node
	for _, iPod := range indexedPods {
		pod := iPod.pod
		podIdx := iPod.idx
		bestNodeIdx := -1
		bestFit := -1.0

		// First, try to fit in already active nodes (best fit)
		for i := range nodeStates {
			if !nodeStates[i].active {
				continue
			}

			if nodeStates[i].cpuRemaining >= pod.CPURequest &&
				nodeStates[i].memRemaining >= pod.MemRequest {

				// Calculate how well this pod fits (smaller remainder = better fit)
				cpuFit := (nodeStates[i].cpuRemaining - pod.CPURequest) / nodeStates[i].node.CPUCapacity
				memFit := (nodeStates[i].memRemaining - pod.MemRequest) / nodeStates[i].node.MemCapacity
				fit := cpuFit + memFit

				if bestNodeIdx == -1 || fit < bestFit {
					bestNodeIdx = i
					bestFit = fit
				}
			}
		}

		// If no active node fits, activate the cheapest node that fits
		if bestNodeIdx == -1 {
			for i := range nodeStates {
				if nodeStates[i].active {
					continue
				}

				if nodeStates[i].cpuRemaining >= pod.CPURequest &&
					nodeStates[i].memRemaining >= pod.MemRequest {
					bestNodeIdx = i
					nodeStates[i].active = true
					break
				}
			}
		}

		// If we found a node, assign the pod
		if bestNodeIdx != -1 {
			nodeStates[bestNodeIdx].cpuRemaining -= pod.CPURequest
			nodeStates[bestNodeIdx].memRemaining -= pod.MemRequest
			assignments[podIdx] = bestNodeIdx
		}
	}

	// Calculate total cost of active nodes
	totalCost := 0.0
	activeCount := 0
	for _, ns := range nodeStates {
		if ns.active {
			totalCost += ns.node.CostPerHour
			activeCount++
		}
	}

	// Handle unassigned pods
	unassignedCount := 0
	for i, nodeIdx := range assignments {
		if nodeIdx == -1 {
			unassignedCount++
			// Assign to first node that has capacity (even if not optimal)
			for j := range nodeStates {
				if nodeStates[j].cpuRemaining >= pods[i].CPURequest &&
					nodeStates[j].memRemaining >= pods[i].MemRequest {
					assignments[i] = j
					if !nodeStates[j].active {
						nodeStates[j].active = true
						totalCost += nodeStates[j].node.CostPerHour
					}
					break
				}
			}
		}
	}

	return totalCost, assignments
}
