package resourcecost

import (
	"fmt"
	"log"

	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/objectives/cost"
)

// EffectiveCostObjective implements the combined Cost_used + Cost_waste optimization
// f_effective_cost(S_target) = Cost_used + Cost_waste
// where:
// - Cost_used = Σ(p∈P) [($/vCPU * Request_cpu(p)) + ($/GiB * Request_mem(p))]
// - Cost_waste = Σ(n∈N_active) [($/vCPU * IdleCPU(n)) + ($/GiB * IdleMemory(n))]
type EffectiveCostObjective struct {
	nodes           []framework.NodeInfo
	pods            []framework.PodInfo
	initialMinCost  float64 // initial minimum cost estimate
	initialMaxCost  float64 // initial maximum cost estimate
	currentMinCost  float64 // current best minimum seen (updated dynamically)
	currentMaxCost  float64 // current worst maximum seen (updated dynamically)
	pendingMinCost  float64 // best minimum in current generation
	pendingMaxCost  float64 // worst maximum in current generation
	evaluationCount int     // track evaluations for generation detection
	generationSize  int     // estimated generation size
}

// NewEffectiveCostObjective creates a new effective cost objective
func NewEffectiveCostObjective(nodes []framework.NodeInfo, pods []framework.PodInfo) *EffectiveCostObjective {
	obj := &EffectiveCostObjective{
		nodes: nodes,
		pods:  pods,
	}

	// Calculate initial bounds for normalization
	obj.calculateBounds()

	// Initialize dynamic tracking
	obj.currentMinCost = obj.initialMinCost
	obj.currentMaxCost = obj.initialMaxCost
	obj.pendingMinCost = obj.initialMinCost
	obj.pendingMaxCost = obj.initialMaxCost
	obj.evaluationCount = 0
	obj.generationSize = 0

	return obj
}

// getNodeResourceCosts returns the per-vCPU and per-GiB costs for a node
func (obj *EffectiveCostObjective) getNodeResourceCosts(nodeIdx int) (costPerVCPU, costPerGiB float64) {
	if nodeIdx < 0 || nodeIdx >= len(obj.nodes) {
		// Return default costs for invalid node index
		return 0.05, 0.01
	}

	node := obj.nodes[nodeIdx]

	// Get pricing data from the cost package
	regionPricing, ok := cost.AWSPricing[node.Region]
	if !ok {
		// Use fallback costs if region not found
		return 0.05, 0.01 // $0.05 per vCPU per hour, $0.01 per GiB per hour
	}

	instancePricing, ok := regionPricing[node.InstanceType]
	if !ok {
		// Use fallback costs if instance type not found
		return 0.05, 0.01
	}

	// Return per-resource costs based on lifecycle (spot vs on-demand)
	if node.InstanceLifecycle == "spot" {
		return instancePricing.SpotPerCPU, instancePricing.SpotPerGiB
	}
	return instancePricing.OnDemandPerCPU, instancePricing.OnDemandPerGiB
}

// calculateBounds computes min and max effective costs for normalization
func (obj *EffectiveCostObjective) calculateBounds() {
	// Calculate minimum cost: best possible packing (no waste) on cheapest resources
	obj.initialMinCost = obj.calculateMinimumCost()

	// Calculate maximum cost: worst case scenario with maximum waste
	obj.initialMaxCost = obj.calculateMaximumCost()

	// Ensure min < max
	if obj.initialMinCost >= obj.initialMaxCost {
		obj.initialMaxCost = obj.initialMinCost * 2.0
	}

	log.Printf("EffectiveCost initial bounds: min=%.6f, max=%.6f, ratio=%.2f",
		obj.initialMinCost, obj.initialMaxCost, obj.initialMaxCost/obj.initialMinCost)
}

// calculateMinimumCost estimates the minimum possible effective cost using efficient packing on cheapest nodes
func (obj *EffectiveCostObjective) calculateMinimumCost() float64 {
	return obj.ComputeLowerBound(obj.pods, obj.nodes)
}

// calculateMaximumCost estimates the maximum possible effective cost through poor placement and fragmentation
func (obj *EffectiveCostObjective) calculateMaximumCost() float64 {
	return obj.ComputeUpperBound(obj.pods, obj.nodes)
}

// ComputeLowerBound computes the theoretical minimum cost by packing pods onto the cheapest nodes efficiently
func (obj *EffectiveCostObjective) ComputeLowerBound(pods []framework.PodInfo, nodes []framework.NodeInfo) float64 {
	// Step 1: Calculate Average Cluster Costs
	totalCPUCapacity := 0.0
	totalMemCapacity := 0.0
	totalCPUCost := 0.0
	totalMemCost := 0.0

	for _, node := range nodes {
		costPerVCPU, costPerGiB := obj.getNodeResourceCosts(node.Idx)
		cpuCapacity := node.CPUCapacity / 1000.0 // Convert to cores
		memCapacity := node.MemCapacity / 1e9    // Convert to GiB

		totalCPUCapacity += cpuCapacity
		totalMemCapacity += memCapacity
		totalCPUCost += costPerVCPU * cpuCapacity
		totalMemCost += costPerGiB * memCapacity
	}

	avgCPUCost := totalCPUCost / totalCPUCapacity
	avgMemCost := totalMemCost / totalMemCapacity
	costRatio := avgCPUCost / avgMemCost // GiB per vCPU in cost terms

	// Step 2: Sort Nodes by Cost Efficiency (Cheapest First)
	type nodeEfficiency struct {
		idx           int
		costPerVCPU   float64
		costPerGiB    float64
		cpuCapacity   float64
		memCapacity   float64
		effectiveCost float64
		cpuUsed       float64
		memUsed       float64
	}

	nodeList := make([]nodeEfficiency, len(nodes))
	for i, node := range nodes {
		costPerVCPU, costPerGiB := obj.getNodeResourceCosts(node.Idx)
		cpuCapacity := node.CPUCapacity / 1000.0
		memCapacity := node.MemCapacity / 1e9
		effectiveCost := costPerVCPU + (costPerGiB * costRatio)

		nodeList[i] = nodeEfficiency{
			idx:           i,
			costPerVCPU:   costPerVCPU,
			costPerGiB:    costPerGiB,
			cpuCapacity:   cpuCapacity,
			memCapacity:   memCapacity,
			effectiveCost: effectiveCost,
			cpuUsed:       0.0,
			memUsed:       0.0,
		}
	}

	// Sort nodes ascending by effectiveCost (cheapest first)
	for i := 0; i < len(nodeList)-1; i++ {
		for j := i + 1; j < len(nodeList); j++ {
			if nodeList[i].effectiveCost > nodeList[j].effectiveCost {
				nodeList[i], nodeList[j] = nodeList[j], nodeList[i]
			}
		}
	}

	// Step 3: Sort Pods by Cost Footprint (Largest First)
	type podCostInfo struct {
		idx     int
		cpuReq  float64
		memReq  float64
		podCost float64
	}

	podList := make([]podCostInfo, len(pods))
	for i, pod := range pods {
		cpuReq := pod.CPURequest / 1000.0
		memReq := pod.MemRequest / 1e9
		podCost := (avgCPUCost * cpuReq) + (avgMemCost * memReq)

		podList[i] = podCostInfo{
			idx:     i,
			cpuReq:  cpuReq,
			memReq:  memReq,
			podCost: podCost,
		}
	}

	// Sort pods descending by podCost (largest first)
	for i := 0; i < len(podList)-1; i++ {
		for j := i + 1; j < len(podList); j++ {
			if podList[i].podCost < podList[j].podCost {
				podList[i], podList[j] = podList[j], podList[i]
			}
		}
	}

	// Step 4: Pack Pods using First-Fit
	placedPods := 0
	for _, pod := range podList {
		placed := false
		// Try each node in sorted order (cheapest first)
		for i := range nodeList {
			node := &nodeList[i]
			// Check if pod fits
			if pod.cpuReq <= (node.cpuCapacity-node.cpuUsed) &&
				pod.memReq <= (node.memCapacity-node.memUsed) {
				// Place pod on this node
				node.cpuUsed += pod.cpuReq
				node.memUsed += pod.memReq
				placed = true
				placedPods++
				break
			}
		}
		if !placed {
			// Pod couldn't be placed - this shouldn't happen in well-configured scenarios
			log.Printf("Warning: Pod %d couldn't be placed in ComputeLowerBound", pod.idx)
		}
	}

	// Step 4: Calculate usage cost
	usageCost := 0.0
	for _, node := range nodeList {
		if node.cpuUsed > 0 || node.memUsed > 0 {
			usageCost += (node.costPerVCPU * node.cpuUsed) + (node.costPerGiB * node.memUsed)
		}
	}

	// Step 5: Add Waste Cost (only for nodes that have at least one pod)
	wasteCost := 0.0
	for _, node := range nodeList {
		if node.cpuUsed > 0 || node.memUsed > 0 {
			idleCPU := node.cpuCapacity - node.cpuUsed
			idleMem := node.memCapacity - node.memUsed
			wasteCost += (node.costPerVCPU * idleCPU) + (node.costPerGiB * idleMem)
		}
	}

	totalCost := usageCost + wasteCost
	log.Printf("ComputeLowerBound: placed %d/%d pods, usage=$%.6f, waste=$%.6f, total=$%.6f",
		placedPods, len(pods), usageCost, wasteCost, totalCost)

	return totalCost
}

// ComputeUpperBound computes the theoretical maximum cost through poor placement and fragmentation
func (obj *EffectiveCostObjective) ComputeUpperBound(pods []framework.PodInfo, nodes []framework.NodeInfo) float64 {
	// Step 1: Sort Nodes by Cost (Most Expensive First)
	type nodeInfo struct {
		idx         int
		costPerVCPU float64
		costPerGiB  float64
		cpuCapacity float64
		memCapacity float64
		totalCost   float64
		cpuUsed     float64
		memUsed     float64
	}

	nodeList := make([]nodeInfo, len(nodes))
	for i, node := range nodes {
		costPerVCPU, costPerGiB := obj.getNodeResourceCosts(node.Idx)
		cpuCapacity := node.CPUCapacity / 1000.0
		memCapacity := node.MemCapacity / 1e9
		totalCost := costPerVCPU + costPerGiB

		nodeList[i] = nodeInfo{
			idx:         i,
			costPerVCPU: costPerVCPU,
			costPerGiB:  costPerGiB,
			cpuCapacity: cpuCapacity,
			memCapacity: memCapacity,
			totalCost:   totalCost,
			cpuUsed:     0.0,
			memUsed:     0.0,
		}
	}

	// Sort nodes descending by totalCost (most expensive first)
	for i := 0; i < len(nodeList)-1; i++ {
		for j := i + 1; j < len(nodeList); j++ {
			if nodeList[i].totalCost < nodeList[j].totalCost {
				nodeList[i], nodeList[j] = nodeList[j], nodeList[i]
			}
		}
	}

	// Step 2: Spread Pods Across All Nodes using round-robin
	nodeIndex := 0
	placedPods := 0

	for _, pod := range pods {
		cpuReq := pod.CPURequest / 1000.0
		memReq := pod.MemRequest / 1e9
		placed := false
		attempts := 0

		// Try round-robin placement
		for attempts < len(nodeList) {
			node := &nodeList[nodeIndex%len(nodeList)]

			// Check if pod fits
			if cpuReq <= (node.cpuCapacity-node.cpuUsed) &&
				memReq <= (node.memCapacity-node.memUsed) {
				// Place pod on this node
				node.cpuUsed += cpuReq
				node.memUsed += memReq
				placed = true
				placedPods++
				nodeIndex++ // Move to next node for next pod
				break
			}

			nodeIndex++
			attempts++
		}

		// If round-robin fails, fall back to any available node
		if !placed {
			for i := range nodeList {
				node := &nodeList[i]
				if cpuReq <= (node.cpuCapacity-node.cpuUsed) &&
					memReq <= (node.memCapacity-node.memUsed) {
					node.cpuUsed += cpuReq
					node.memUsed += memReq
					placed = true
					placedPods++
					break
				}
			}
		}

		if !placed {
			log.Printf("Warning: Pod %d couldn't be placed in ComputeUpperBound", pod.Idx)
		}
	}

	// Calculate usage cost
	usageCost := 0.0
	for _, node := range nodeList {
		if node.cpuUsed > 0 || node.memUsed > 0 {
			usageCost += (node.costPerVCPU * node.cpuUsed) + (node.costPerGiB * node.memUsed)
		}
	}

	// Step 3: Add Waste Cost for ALL Nodes (simulates worst case where all nodes are powered on)
	wasteCost := 0.0
	activeNodes := 0
	for _, node := range nodeList {
		idleCPU := node.cpuCapacity - node.cpuUsed
		idleMem := node.memCapacity - node.memUsed
		wasteCost += (node.costPerVCPU * idleCPU) + (node.costPerGiB * idleMem)
		if node.cpuUsed > 0 || node.memUsed > 0 {
			activeNodes++
		}
	}

	totalCost := usageCost + wasteCost
	log.Printf("ComputeUpperBound: placed %d/%d pods on %d active nodes (of %d total), usage=$%.6f, waste=$%.6f, total=$%.6f",
		placedPods, len(pods), activeNodes, len(nodes), usageCost, wasteCost, totalCost)

	return totalCost
}

// EffectiveCostObjectiveFunc returns an objective function that calculates effective cost
func EffectiveCostObjectiveFunc(nodes []framework.NodeInfo, pods []framework.PodInfo) framework.ObjectiveFunc {
	obj := NewEffectiveCostObjective(nodes, pods)

	return func(sol framework.Solution) float64 {
		// Cast to IntegerSolution to access pod assignments
		intSol, ok := sol.(*framework.IntegerSolution)
		if !ok {
			return 1.0 // Maximum normalized cost if invalid solution type
		}

		effectiveCost := obj.calculateEffectiveCost(intSol.Variables)

		// Track the best and worst costs seen in this generation
		if effectiveCost < obj.pendingMinCost {
			obj.pendingMinCost = effectiveCost
		}
		if effectiveCost > obj.pendingMaxCost {
			obj.pendingMaxCost = effectiveCost
		}

		obj.evaluationCount++

		// Heuristic: detect generation boundary and update bounds
		// Similar to the original cost objective's approach
		if obj.generationSize == 0 && obj.evaluationCount >= 100 {
			// First generation - estimate size
			obj.generationSize = obj.evaluationCount
		} else if obj.generationSize > 0 && obj.evaluationCount%obj.generationSize == 0 {
			// Generation boundary detected - update both bounds
			boundsUpdated := false

			if obj.pendingMinCost < obj.currentMinCost {
				log.Printf("EffectiveCost: Generation boundary - updating minimum from %.6f to %.6f",
					obj.currentMinCost, obj.pendingMinCost)
				obj.currentMinCost = obj.pendingMinCost
				boundsUpdated = true
			}

			if obj.pendingMaxCost > obj.currentMaxCost {
				log.Printf("EffectiveCost: Generation boundary - updating maximum from %.6f to %.6f",
					obj.currentMaxCost, obj.pendingMaxCost)
				obj.currentMaxCost = obj.pendingMaxCost
				boundsUpdated = true
			}

			if boundsUpdated {
				ratio := obj.currentMaxCost / obj.currentMinCost
				log.Printf("EffectiveCost: Updated bounds - min=%.6f, max=%.6f, ratio=%.2f",
					obj.currentMinCost, obj.currentMaxCost, ratio)
			}
		}

		// Normalize using current bounds (not initial bounds)
		normalizedCost := (effectiveCost - obj.currentMinCost) / (obj.currentMaxCost - obj.currentMinCost)

		// Allow negative and >1 values - they indicate we found solutions outside expected bounds!
		// This provides valuable feedback about bound quality

		// Debug: Log if we're getting values outside expected bounds
		if normalizedCost < 0 {
			log.Printf("EffectiveCost: Found solution better than current minimum! actual=%.6f, currentMin=%.6f, normalized=%.6f",
				effectiveCost, obj.currentMinCost, normalizedCost)
		}
		if normalizedCost > 1 {
			log.Printf("EffectiveCost: Found solution worse than current maximum! actual=%.6f, currentMax=%.6f, normalized=%.6f",
				effectiveCost, obj.currentMaxCost, normalizedCost)
		}

		return normalizedCost
	}
}

// calculateEffectiveCost computes the total effective cost for a solution
func (obj *EffectiveCostObjective) calculateEffectiveCost(assignment []int) float64 {
	// Track resource usage per node
	nodeUsage := make(map[int]NodeUsage)
	for i := range obj.nodes {
		nodeUsage[i] = NodeUsage{
			CPUUsed: 0.0,
			MemUsed: 0.0,
		}
	}

	// Calculate Cost_used: sum of pod resource costs
	costUsed := 0.0
	for podIdx, nodeIdx := range assignment {
		if nodeIdx < 0 || nodeIdx >= len(obj.nodes) {
			continue // Skip unassigned pods
		}

		pod := obj.pods[podIdx]
		costPerVCPU, costPerGiB := obj.getNodeResourceCosts(nodeIdx)

		// Convert pod requests to appropriate units
		podCPUCores := pod.CPURequest / 1000.0 // millicores to cores
		podMemGiB := pod.MemRequest / 1e9      // bytes to GiB

		// Add to used cost
		costUsed += (podCPUCores * costPerVCPU) + (podMemGiB * costPerGiB)

		// Track usage for waste calculation
		nodeUsage[nodeIdx] = NodeUsage{
			CPUUsed: nodeUsage[nodeIdx].CPUUsed + podCPUCores,
			MemUsed: nodeUsage[nodeIdx].MemUsed + podMemGiB,
		}
	}

	// Calculate Cost_waste: sum of idle resource costs on active nodes
	costWaste := 0.0
	for nodeIdx, usage := range nodeUsage {
		// Only consider nodes that have at least one pod (active nodes)
		if usage.CPUUsed > 0 || usage.MemUsed > 0 {
			node := obj.nodes[nodeIdx]
			costPerVCPU, costPerGiB := obj.getNodeResourceCosts(nodeIdx)

			// Calculate idle resources
			nodeCPUCapacity := node.CPUCapacity / 1000.0 // millicores to cores
			nodeMemCapacity := node.MemCapacity / 1e9    // bytes to GiB

			idleCPU := nodeCPUCapacity - usage.CPUUsed
			idleMem := nodeMemCapacity - usage.MemUsed

			// Ensure idle resources are non-negative
			if idleCPU < 0 {
				idleCPU = 0
			}
			if idleMem < 0 {
				idleMem = 0
			}

			// Add to waste cost
			costWaste += (idleCPU * costPerVCPU) + (idleMem * costPerGiB)
		}
	}

	effectiveCost := costUsed + costWaste

	return effectiveCost
}

// NodeUsage tracks resource usage on a node
type NodeUsage struct {
	CPUUsed float64 // in cores
	MemUsed float64 // in GiB
}

// GetNodeResourceCosts returns the per-resource costs for a node (for testing)
func (obj *EffectiveCostObjective) GetNodeResourceCosts(nodeIdx int) (costPerVCPU, costPerGiB float64, err error) {
	if nodeIdx < 0 || nodeIdx >= len(obj.nodes) {
		return 0, 0, fmt.Errorf("invalid node index %d", nodeIdx)
	}
	costPerVCPU, costPerGiB = obj.getNodeResourceCosts(nodeIdx)
	return costPerVCPU, costPerGiB, nil
}
