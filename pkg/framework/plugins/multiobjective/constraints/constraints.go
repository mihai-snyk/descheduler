package constraints

import (
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
)

// PodInfo contains essential information about a pod for constraint checking
type PodInfo struct {
	Name                   string
	CPURequest             float64 // in millicores
	MemRequest             float64 // in bytes
	CurrentNode            int     // current node assignment
	ReplicaSetName         string  // for PDB calculations
	MaxUnavailableReplicas int     // for PDB calculations
}

// NodeInfo contains node capacity information
type NodeInfo struct {
	Name         string
	CPUCapacity  float64 // in millicores
	MemCapacity  float64 // in bytes
	CPUAllocated float64 // already allocated (excluding pods we're moving)
	MemAllocated float64 // already allocated (excluding pods we're moving)
}

// ResourceConstraint creates a constraint function that checks resource capacity
func ResourceConstraint(pods []PodInfo, nodes []NodeInfo) framework.Constraint {
	return func(sol framework.Solution) bool {
		intSol, ok := sol.(*framework.IntegerSolution)
		if !ok {
			return false
		}

		assignment := intSol.Variables

		// Calculate resource usage per node
		nodeResources := make([]struct {
			cpuUsed float64
			memUsed float64
		}, len(nodes))

		// Start with existing allocations
		for i, node := range nodes {
			nodeResources[i].cpuUsed = node.CPUAllocated
			nodeResources[i].memUsed = node.MemAllocated
		}

		// Add pod assignments
		for podIdx, nodeIdx := range assignment {
			if nodeIdx < 0 || nodeIdx >= len(nodes) {
				return false // Invalid node index
			}

			nodeResources[nodeIdx].cpuUsed += pods[podIdx].CPURequest
			nodeResources[nodeIdx].memUsed += pods[podIdx].MemRequest
		}

		// Check capacity constraints
		for nodeIdx, node := range nodes {
			if nodeResources[nodeIdx].cpuUsed > node.CPUCapacity {
				return false // CPU capacity exceeded
			}
			if nodeResources[nodeIdx].memUsed > node.MemCapacity {
				return false // Memory capacity exceeded
			}
		}

		return true
	}
}

// PDBConstraint creates a constraint function that checks PDB compliance
func PDBConstraint(pods []PodInfo) framework.Constraint {
	// Build current state
	currentState := make([]int, len(pods))
	for i, pod := range pods {
		currentState[i] = pod.CurrentNode
	}

	return func(sol framework.Solution) bool {
		intSol, ok := sol.(*framework.IntegerSolution)
		if !ok {
			return false
		}

		proposed := intSol.Variables

		// Group pods by replica set to check PDB constraints
		replicaSets := make(map[string]struct {
			pods           []int
			maxUnavailable int
		})

		// Build replica set information
		for i, pod := range pods {
			if pod.ReplicaSetName != "" {
				rs := replicaSets[pod.ReplicaSetName]
				rs.pods = append(rs.pods, i)
				rs.maxUnavailable = pod.MaxUnavailableReplicas
				replicaSets[pod.ReplicaSetName] = rs
			}
		}

		// For each replica set, check if we can move the pods
		for _, rs := range replicaSets {
			// If maxUnavailable is 0, we cannot move any pods
			if rs.maxUnavailable <= 0 {
				// Check if any pod needs to move
				for _, podIdx := range rs.pods {
					if currentState[podIdx] != proposed[podIdx] {
						return false // Cannot move any pod when maxUnavailable=0
					}
				}
			}
			// If maxUnavailable > 0, we can move pods sequentially
			// This is a simplified check - in production, the descheduler
			// would need to execute moves respecting the PDB at each step
		}

		return true
	}
}

// CombineConstraints combines multiple constraints into one
func CombineConstraints(constraints ...framework.Constraint) framework.Constraint {
	return func(sol framework.Solution) bool {
		for _, constraint := range constraints {
			if !constraint(sol) {
				return false
			}
		}
		return true
	}
}
