package disruption

import (
	"math"

	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
)

// PodInfo contains information about a pod relevant for disruption calculation
type PodInfo struct {
	Name                   string
	CurrentNode            int     // Current node assignment
	ColdStartTime          float64 // Startup probe time (failureThreshold * periodSeconds)
	ReplicaSetName         string  // For grouping pods by app
	MaxUnavailableReplicas int     // From PDB
}

// DisruptionResult contains the breakdown of disruption cost
type DisruptionResult struct {
	TotalCost       float64
	MovedPods       int
	MovementImpact  float64
	ColdStartImpact float64
	TimeSlots       int
	TimeSlotsImpact float64
}

// DisruptionConfig contains normalization and weight parameters
type DisruptionConfig struct {
	// Expected ranges for normalization (all components scaled to [0,1])
	MaxExpectedMoves     float64 // Expected maximum pods that might move
	MaxExpectedColdStart float64 // Maximum cold start time sum in seconds
	MaxExpectedTimeSlots float64 // Maximum time slots expected

	// Component weights after normalization (should sum to 1.0 for balanced contribution)
	MovementWeight  float64 // Weight for pod movement component
	ColdStartWeight float64 // Weight for cold start impact component
	TimeSlotWeight  float64 // Weight for time slot component

	// Movement penalty type
	MovementPenaltyType   string  // "linear", "sqrt", "log", "exp", "threshold"
	MovementPenaltyLambda float64 // Parameter for exp decay (higher = steeper)
}

// NewDisruptionConfig creates a DisruptionConfig adapted to the actual cluster
func NewDisruptionConfig(pods []PodInfo) DisruptionConfig {
	totalPods := len(pods)

	// Movement: exactly the number of pods (moving all = 1.0 impact)
	maxExpectedMoves := float64(totalPods)
	if maxExpectedMoves < 1 {
		maxExpectedMoves = 1
	}

	// Cold start: use a fixed baseline (60 seconds = significant disruption)
	// This keeps cold start impact consistent regardless of cluster size
	maxExpectedColdStart := 60.0 // 1 minute baseline

	// Time slots: based on most restrictive PDB and affected pods
	maxTimeSlots := calculateWorstCaseTimeSlots(pods)

	return DisruptionConfig{
		MaxExpectedMoves:     maxExpectedMoves,
		MaxExpectedColdStart: maxExpectedColdStart,
		MaxExpectedTimeSlots: float64(maxTimeSlots),

		MovementWeight:  0.70,
		ColdStartWeight: 0.10,
		TimeSlotWeight:  0.20,

		// Keep it simple with linear penalty
		MovementPenaltyType:   "linear",
		MovementPenaltyLambda: 1.0, // Not used for linear
	}
}

// NewDisruptionConfigWithPenalty creates a DisruptionConfig with a specific penalty type
func NewDisruptionConfigWithPenalty(pods []PodInfo, penaltyType string) DisruptionConfig {
	config := NewDisruptionConfig(pods)
	config.MovementPenaltyType = penaltyType
	// Keep the default lambda from NewDisruptionConfig
	return config
}

// DisruptionObjective creates a normalized objective function that minimizes disruption
// All components are normalized to [0,1] range before applying weights
// Total output is in range [0,1] when weights sum to 1.0
func DisruptionObjective(currentState []int, pods []PodInfo, config DisruptionConfig) framework.ObjectiveFunc {
	return func(sol framework.Solution) float64 {
		proposed := sol.(*framework.IntegerSolution).Variables

		result := calculateDisruption(currentState, proposed, pods, config)
		return result.TotalCost
	}
}

// DisruptionObjectiveWithDetails returns both the objective function and a details function
func DisruptionObjectiveWithDetails(currentState []int, pods []PodInfo, config DisruptionConfig) (framework.ObjectiveFunc, func(framework.Solution) DisruptionResult) {
	objFunc := DisruptionObjective(currentState, pods, config)

	detailsFunc := func(sol framework.Solution) DisruptionResult {
		proposed := sol.(*framework.IntegerSolution).Variables
		return calculateDisruption(currentState, proposed, pods, config)
	}

	return objFunc, detailsFunc
}

// calculateDisruption computes the normalized disruption cost with all components
func calculateDisruption(currentState, proposed []int, pods []PodInfo, config DisruptionConfig) DisruptionResult {
	result := DisruptionResult{}

	// Track which pods need to move
	movedPods := make([]int, 0)
	totalColdStart := 0.0

	// Calculate raw impacts
	for i, pod := range pods {
		if currentState[i] != proposed[i] {
			// Pod needs to move (δ(p) = 1)
			movedPods = append(movedPods, i)
			result.MovedPods++
			totalColdStart += pod.ColdStartTime
		}
	}

	// Calculate time slots needed based on PDB constraints
	result.TimeSlots = calculateTimeSlots(movedPods, pods)

	// Calculate movement impact per replica set (weighted average)
	// This captures the relative disruption to each service
	normalizedMovement := calculateReplicaSetWeightedMovement(currentState, proposed, pods, config)

	// Cold start: average per pod (not total) to avoid scaling with pod count
	normalizedColdStart := 0.0
	if result.MovedPods > 0 {
		normalizedColdStart = totalColdStart / (float64(result.MovedPods) * config.MaxExpectedColdStart)
	}
	normalizedTimeSlots := float64(result.TimeSlots) / config.MaxExpectedTimeSlots

	// Apply weights to normalized components
	result.MovementImpact = normalizedMovement * config.MovementWeight
	result.ColdStartImpact = normalizedColdStart * config.ColdStartWeight
	result.TimeSlotsImpact = normalizedTimeSlots * config.TimeSlotWeight

	// Total normalized cost (will be in [0,1] if weights sum to 1.0, but can exceed if cold start is high)
	result.TotalCost = result.MovementImpact + result.ColdStartImpact + result.TimeSlotsImpact

	return result
}

// calculateReplicaSetWeightedMovement calculates movement disruption normalized per replica set
// This gives a more accurate measure of disruption impact on individual services
func calculateReplicaSetWeightedMovement(currentState, proposed []int, pods []PodInfo, config DisruptionConfig) float64 {
	// Group pods by replica set
	replicaSets := make(map[string]*struct {
		totalPods int
		movedPods int
	})

	// Initialize replica sets and count movements
	for i, pod := range pods {
		if replicaSets[pod.ReplicaSetName] == nil {
			replicaSets[pod.ReplicaSetName] = &struct {
				totalPods int
				movedPods int
			}{}
		}
		rs := replicaSets[pod.ReplicaSetName]
		rs.totalPods++

		if currentState[i] != proposed[i] {
			rs.movedPods++
		}
	}

	// Calculate weighted average disruption across replica sets
	// Weight by replica set size to prevent small RSs from dominating
	totalWeight := 0.0
	weightedDisruption := 0.0

	for _, rs := range replicaSets {
		if rs.totalPods == 0 {
			continue
		}

		// Movement ratio for this replica set
		rsMovementRatio := float64(rs.movedPods) / float64(rs.totalPods)

		// Apply the configured penalty function to the ratio
		rsPenalty := applyPenaltyFunction(rsMovementRatio, config.MovementPenaltyType, config.MovementPenaltyLambda)

		// Weight by replica set size
		weight := float64(rs.totalPods)
		weightedDisruption += rsPenalty * weight
		totalWeight += weight
	}

	if totalWeight > 0 {
		return weightedDisruption / totalWeight
	}

	return 0.0
}

// applyPenaltyFunction applies the configured penalty function to a movement ratio
func applyPenaltyFunction(ratio float64, penaltyType string, lambda float64) float64 {
	if ratio == 0 {
		return 0.0
	}

	switch penaltyType {
	case "linear":
		return ratio
	case "sqrt":
		return math.Sqrt(ratio)
	case "exponential":
		// f(x) = (e^(λx) - 1) / (e^λ - 1)
		return (math.Exp(lambda*ratio) - 1) / (math.Exp(lambda) - 1)
	default:
		return ratio
	}
}

// calculateMovementPenalty applies different penalty functions for pod movements
func calculateMovementPenalty(movedPods int, totalPods int, config DisruptionConfig) float64 {
	if movedPods == 0 {
		return 0.0
	}

	ratio := float64(movedPods) / float64(totalPods)

	switch config.MovementPenaltyType {
	case "linear":
		// Original linear penalty
		return ratio

	case "sqrt":
		// Square root gives diminishing penalty
		// Moving 1/100 pods = 0.1 penalty instead of 0.01
		// Moving 4/100 pods = 0.2 penalty instead of 0.04
		return math.Sqrt(ratio)

	case "log":
		// Logarithmic is even more forgiving for small moves
		// log(1 + x) / log(2) to normalize
		return math.Log(1+ratio) / math.Log(2)

	case "exp":
		// Exponential decay: 1 - e^(-λx)
		// Starts gentle, ramps up for many moves
		return 1 - math.Exp(-config.MovementPenaltyLambda*ratio)

	case "threshold":
		// Different rates for small vs large movements
		if movedPods <= 2 {
			// Very gentle for 1-2 pods
			return float64(movedPods) * 0.05
		} else if movedPods <= 5 {
			// Moderate for 3-5 pods
			return 0.1 + float64(movedPods-2)*0.1
		} else {
			// Steeper penalty for many moves
			return 0.4 + math.Sqrt(float64(movedPods-5)/float64(totalPods))
		}

	default:
		// Fallback to linear
		return ratio
	}
}

// calculateTimeSlots determines how many time slots are needed to respect PDB constraints
func calculateTimeSlots(movedPods []int, pods []PodInfo) int {
	if len(movedPods) == 0 {
		return 0
	}

	// Group moved pods by replica set
	replicaGroups := make(map[string][]int)
	for _, podIdx := range movedPods {
		rsName := pods[podIdx].ReplicaSetName
		if rsName != "" {
			replicaGroups[rsName] = append(replicaGroups[rsName], podIdx)
		}
	}

	// Calculate time slots needed
	maxTimeSlots := 1

	for _, podIndices := range replicaGroups {
		if len(podIndices) == 0 {
			continue
		}

		// Get PDB constraint (maxUnavailable) from first pod in group
		maxUnavailable := pods[podIndices[0]].MaxUnavailableReplicas
		if maxUnavailable <= 0 {
			maxUnavailable = 1 // Default to 1 if not specified
		}

		// Number of time slots needed for this replica set
		timeSlotsNeeded := int(math.Ceil(float64(len(podIndices)) / float64(maxUnavailable)))

		if timeSlotsNeeded > maxTimeSlots {
			maxTimeSlots = timeSlotsNeeded
		}
	}

	return maxTimeSlots
}

// calculateWorstCaseTimeSlots estimates expected time slots using weighted average
// This prevents a single large app from dominating the calculation
func calculateWorstCaseTimeSlots(pods []PodInfo) int {
	// Group pods by replica set to understand PDB impact
	replicaSets := make(map[string]struct {
		count          int
		maxUnavailable int
	})

	totalPods := 0
	for _, pod := range pods {
		if pod.ReplicaSetName != "" {
			rs := replicaSets[pod.ReplicaSetName]
			rs.count++
			rs.maxUnavailable = pod.MaxUnavailableReplicas
			replicaSets[pod.ReplicaSetName] = rs
			totalPods++
		}
	}

	if totalPods == 0 {
		return 1
	}

	// Calculate weighted average of time slots
	weightedSum := 0.0

	for _, rs := range replicaSets {
		if rs.maxUnavailable <= 0 {
			rs.maxUnavailable = 1
		}

		// Time slots needed if all pods in this RS need to move
		timeSlotsNeeded := (rs.count + rs.maxUnavailable - 1) / rs.maxUnavailable

		// Weight by the proportion of pods this app represents
		weight := float64(rs.count) / float64(totalPods)
		weightedSum += float64(timeSlotsNeeded) * weight
	}

	// Round up to get integer time slots
	expectedTimeSlots := int(math.Ceil(weightedSum))
	if expectedTimeSlots < 1 {
		expectedTimeSlots = 1
	}

	return expectedTimeSlots
}
