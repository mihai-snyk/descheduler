// Package warmstart provides a Greedy Constructive State Heuristic (GCSH) for
// generating high-quality initial populations for multi-objective optimization.
//
// The GCSH algorithm creates diverse solutions by using different weight vectors
// for the objectives, systematically sweeping from cost-focused to balance-focused
// solutions. This provides NSGA-II with a strong initial approximation of the
// Pareto front rather than starting from random solutions.
package warmstart

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"sort"

	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
)

// ObjectiveWeights defines the weights for objectives
type ObjectiveWeights []float64

// GCSHConfig contains configuration for the Greedy Constructive State Heuristic
type GCSHConfig struct {
	Pods                []framework.PodInfo
	Nodes               []framework.NodeInfo
	Objectives          []framework.ObjectiveFunc // Only objectives to optimize (e.g., cost, balance - not disruption)
	Constraints         []framework.Constraint
	IncludeCurrentState bool
}

// GenerateWeightVectors creates evenly distributed weight vectors
func GenerateWeightVectors(count int, numObjectives int) []ObjectiveWeights {
	weights := make([]ObjectiveWeights, count)

	// Simple linear interpolation
	for i := 0; i < count; i++ {
		weights[i] = make(ObjectiveWeights, numObjectives)

		if count == 1 {
			// Single weight - equal distribution
			for j := 0; j < numObjectives; j++ {
				weights[i][j] = 1.0 / float64(numObjectives)
			}
		} else {
			// Linear interpolation for 2 objectives (which is what we use)
			t := float64(i) / float64(count-1)
			weights[i][0] = 1.0 - t
			weights[i][1] = t
		}
	}

	return weights
}

// GCSH implements the Greedy Constructive State Heuristic
type GCSH struct {
	config GCSHConfig
}

// NewGCSH creates a new GCSH instance
func NewGCSH(config GCSHConfig) *GCSH {
	return &GCSH{config: config}
}

// GenerateInitialPopulation creates a diverse initial population using GCSH
func (g *GCSH) GenerateInitialPopulation(popSize int) []framework.Solution {
	// Seed random for reproducibility
	rand.Seed(int64(popSize))

	// Generate weight vectors
	numWeights := popSize
	startIdx := 0

	// Handle current state inclusion
	if g.config.IncludeCurrentState && popSize > 1 {
		numWeights = popSize - 1
		startIdx = 1
	}

	// Generate random weight vectors for the objectives we're optimizing
	numObjectives := len(g.config.Objectives)
	weightVectors := GenerateWeightVectors(numWeights, numObjectives)

	// Create solutions array
	solutions := make([]framework.Solution, popSize)

	// Add current state if requested
	if g.config.IncludeCurrentState && popSize > 0 {
		solutions[0] = g.createCurrentStateSolution()
		log.Printf("GCSH: Added current state as baseline (0 disruption)")
	}

	// Generate solutions using GCSH with different weights
	for i := 0; i < numWeights; i++ {
		idx := startIdx + i
		solution := g.constructSolution(weightVectors[i])
		solutions[idx] = solution
	}

	// Check uniqueness of generated solutions
	uniqueCount := 0
	uniqueMap := make(map[string]bool)
	for _, sol := range solutions {
		if intSol, ok := sol.(*framework.IntegerSolution); ok {
			key := fmt.Sprintf("%v", intSol.Variables)
			if !uniqueMap[key] {
				uniqueMap[key] = true
				uniqueCount++
			}
		}
	}

	log.Printf("GCSH: Generated %d initial solutions (%d unique)", len(solutions), uniqueCount)
	return solutions
}

// constructSolution builds a single solution using greedy construction with given weights
func (g *GCSH) constructSolution(weights ObjectiveWeights) framework.Solution {
	numPods := len(g.config.Pods)
	numNodes := len(g.config.Nodes)

	// Create solution with bounds
	vars := make([]int, numPods)
	bounds := make([]framework.IntBounds, numPods)
	for i := range vars {
		vars[i] = -1 // Initialize as unassigned
		bounds[i] = framework.IntBounds{L: 0, H: numNodes - 1}
	}
	sol := framework.NewIntegerSolution(vars, bounds)

	// Sort pods by size (largest first for better packing)
	type podWithIndex struct {
		index    int
		pod      framework.PodInfo
		priority float64
	}

	pods := make([]podWithIndex, numPods)
	for i, pod := range g.config.Pods {
		pods[i] = podWithIndex{
			index:    i,
			pod:      pod,
			priority: pod.CPURequest/1000.0 + pod.MemRequest/1e9, // Normalized size
		}
	}

	// Add randomness to pod ordering for diversity
	// Use a small random factor to shuffle similarly-sized pods
	sort.Slice(pods, func(i, j int) bool {
		// Add random factor (up to 20% of priority) to create variety
		priorityI := pods[i].priority * (0.8 + rand.Float64()*0.4)
		priorityJ := pods[j].priority * (0.8 + rand.Float64()*0.4)
		return priorityI > priorityJ
	})

	// Initialize node tracking with available resources
	type nodeState struct {
		cpuAvailable float64
		memAvailable float64
	}
	nodes := make([]nodeState, len(g.config.Nodes))
	for i, node := range g.config.Nodes {
		nodes[i] = nodeState{
			cpuAvailable: node.CPUCapacity,
			memAvailable: node.MemCapacity,
		}
	}

	// Place each pod greedily
	for _, p := range pods {
		pod := p.pod
		podIdx := p.index
		bestNode := -1
		bestScore := math.Inf(1)

		// Try each node
		for nodeIdx, node := range nodes {
			// Check if pod fits
			if node.cpuAvailable < pod.CPURequest || node.memAvailable < pod.MemRequest {
				continue
			}

			// Temporarily place pod
			sol.Variables[podIdx] = nodeIdx
			nodes[nodeIdx].cpuAvailable -= pod.CPURequest
			nodes[nodeIdx].memAvailable -= pod.MemRequest

			// Check constraints
			valid := true
			for _, constraint := range g.config.Constraints {
				if !constraint(sol) {
					valid = false
					break
				}
			}

			if valid {
				// Calculate greedy score
				score := g.calculateGreedyScore(sol, weights)
				if score < bestScore {
					bestScore = score
					bestNode = nodeIdx
				}
			}

			// Revert temporary placement
			sol.Variables[podIdx] = -1
			nodes[nodeIdx].cpuAvailable += pod.CPURequest
			nodes[nodeIdx].memAvailable += pod.MemRequest
		}

		// Place pod on best node
		if bestNode != -1 {
			sol.Variables[podIdx] = bestNode
			nodes[bestNode].cpuAvailable -= pod.CPURequest
			nodes[bestNode].memAvailable -= pod.MemRequest
		} else {
			// No valid placement found, try any node that fits
			for i, node := range nodes {
				if node.cpuAvailable >= pod.CPURequest && node.memAvailable >= pod.MemRequest {
					sol.Variables[podIdx] = i
					nodes[i].cpuAvailable -= pod.CPURequest
					nodes[i].memAvailable -= pod.MemRequest
					break
				}
			}
		}
	}

	// Verify all pods are placed
	for i, nodeIdx := range sol.Variables {
		if nodeIdx == -1 {
			log.Printf("Warning: Pod %d could not be placed", i)
			// Place on first node with capacity as fallback
			for j, node := range nodes {
				if node.cpuAvailable >= g.config.Pods[i].CPURequest &&
					node.memAvailable >= g.config.Pods[i].MemRequest {
					sol.Variables[i] = j
					break
				}
			}
		}
	}

	return sol
}

// calculateGreedyScore computes the weighted score for partial solution
func (g *GCSH) calculateGreedyScore(sol framework.Solution, weights ObjectiveWeights) float64 {
	score := 0.0

	// Calculate weighted sum of all objectives
	for i, objFunc := range g.config.Objectives {
		if i < len(weights) {
			objValue := objFunc(sol)
			score += weights[i] * objValue
		}
	}

	return score
}

// createCurrentStateSolution creates a solution representing the current cluster state
func (g *GCSH) createCurrentStateSolution() framework.Solution {
	numPods := len(g.config.Pods)
	numNodes := len(g.config.Nodes)

	vars := make([]int, numPods)
	bounds := make([]framework.IntBounds, numPods)
	for i, pod := range g.config.Pods {
		vars[i] = pod.Node
		bounds[i] = framework.IntBounds{L: 0, H: numNodes - 1}
	}

	return framework.NewIntegerSolution(vars, bounds)
}
