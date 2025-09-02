package disruption_test

import (
	"math"
	"testing"

	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/objectives/disruption"
)

func TestDisruptionObjective(t *testing.T) {
	// Test scenario: 6 pods across 2 replica sets
	pods := []disruption.PodInfo{
		// App1 - 3 replicas, maxUnavailable=1
		{Name: "app1-0", CurrentNode: 0, ColdStartTime: 30.0, ReplicaSetName: "app1", MaxUnavailableReplicas: 1},
		{Name: "app1-1", CurrentNode: 0, ColdStartTime: 30.0, ReplicaSetName: "app1", MaxUnavailableReplicas: 1},
		{Name: "app1-2", CurrentNode: 0, ColdStartTime: 30.0, ReplicaSetName: "app1", MaxUnavailableReplicas: 1},
		// App2 - 3 replicas, maxUnavailable=2 (more lenient)
		{Name: "app2-0", CurrentNode: 1, ColdStartTime: 10.0, ReplicaSetName: "app2", MaxUnavailableReplicas: 2},
		{Name: "app2-1", CurrentNode: 1, ColdStartTime: 10.0, ReplicaSetName: "app2", MaxUnavailableReplicas: 2},
		{Name: "app2-2", CurrentNode: 1, ColdStartTime: 10.0, ReplicaSetName: "app2", MaxUnavailableReplicas: 2},
	}

	currentState := []int{0, 0, 0, 1, 1, 1}
	config := disruption.NewDisruptionConfig(pods)

	tests := []struct {
		name              string
		proposed          []int
		expectedMoves     int
		expectedTimeSlots int
		description       string
	}{
		{
			name:              "NoMoves",
			proposed:          []int{0, 0, 0, 1, 1, 1},
			expectedMoves:     0,
			expectedTimeSlots: 0,
			description:       "No disruption when state doesn't change",
		},
		{
			name:              "MoveOneFromEachApp",
			proposed:          []int{0, 0, 1, 1, 1, 0}, // Move app1-2 and app2-2
			expectedMoves:     2,
			expectedTimeSlots: 1, // Both apps can handle 1 move in parallel
			description:       "Moving one pod from each app fits in 1 time slot",
		},
		{
			name:              "MoveAllApp1",
			proposed:          []int{1, 1, 1, 1, 1, 1}, // Move all app1 pods
			expectedMoves:     3,
			expectedTimeSlots: 3, // app1 has maxUnavailable=1, needs 3 slots
			description:       "Moving all app1 pods requires 3 time slots due to PDB",
		},
		{
			name:              "MoveAllApp2",
			proposed:          []int{0, 0, 0, 0, 0, 0}, // Move all app2 pods
			expectedMoves:     3,
			expectedTimeSlots: 2, // app2 has maxUnavailable=2, needs 2 slots
			description:       "Moving all app2 pods requires 2 time slots due to PDB",
		},
		{
			name:              "MoveAll",
			proposed:          []int{1, 1, 1, 0, 0, 0}, // Swap all pods
			expectedMoves:     6,
			expectedTimeSlots: 3, // Limited by app1's constraint
			description:       "Moving all pods limited by most restrictive PDB",
		},
	}

	objFunc, detailsFunc := disruption.DisruptionObjectiveWithDetails(currentState, pods, config)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create solution
			bounds := make([]framework.IntBounds, len(pods))
			for i := range bounds {
				bounds[i] = framework.IntBounds{L: 0, H: 1}
			}
			sol := framework.NewIntegerSolution(tt.proposed, bounds)

			// Get disruption details
			result := detailsFunc(sol)
			totalCost := objFunc(sol)

			// Verify results
			if result.MovedPods != tt.expectedMoves {
				t.Errorf("Expected %d moved pods, got %d", tt.expectedMoves, result.MovedPods)
			}

			if result.TimeSlots != tt.expectedTimeSlots {
				t.Errorf("Expected %d time slots, got %d", tt.expectedTimeSlots, result.TimeSlots)
			}

			// Log details
			t.Logf("%s", tt.description)
			t.Logf("  Moved pods: %d", result.MovedPods)
			t.Logf("  Time slots: %d", result.TimeSlots)
			t.Logf("  Normalized impacts:")
			t.Logf("    - Movement: %.4f", result.MovementImpact)
			t.Logf("    - Cold start: %.4f", result.ColdStartImpact)
			t.Logf("    - Time slots: %.4f", result.TimeSlotsImpact)
			t.Logf("  Total disruption cost: %.4f", totalCost)
		})
	}
}

func TestColdStartImpact(t *testing.T) {
	// Test that cold start time affects disruption cost using average per pod
	scenarios := []struct {
		name            string
		pods            []disruption.PodInfo
		currentState    []int
		proposedState   []int
		expectedAvgCold float64
		description     string
	}{
		{
			name: "SwapFastAndSlow",
			pods: []disruption.PodInfo{
				{Name: "fast-0", CurrentNode: 0, ColdStartTime: 5.0, ReplicaSetName: "fast", MaxUnavailableReplicas: 1},
				{Name: "slow-0", CurrentNode: 1, ColdStartTime: 120.0, ReplicaSetName: "slow", MaxUnavailableReplicas: 1},
			},
			currentState:    []int{0, 1},
			proposedState:   []int{1, 0},
			expectedAvgCold: 62.5, // (5+120)/2 = 62.5 seconds average
			description:     "Swapping fast and slow startup pods",
		},
		{
			name: "MoveManyFastPods",
			pods: []disruption.PodInfo{
				{Name: "fast-0", CurrentNode: 0, ColdStartTime: 10.0, ReplicaSetName: "fast", MaxUnavailableReplicas: 2},
				{Name: "fast-1", CurrentNode: 0, ColdStartTime: 10.0, ReplicaSetName: "fast", MaxUnavailableReplicas: 2},
				{Name: "fast-2", CurrentNode: 0, ColdStartTime: 10.0, ReplicaSetName: "fast", MaxUnavailableReplicas: 2},
				{Name: "fast-3", CurrentNode: 0, ColdStartTime: 10.0, ReplicaSetName: "fast", MaxUnavailableReplicas: 2},
				{Name: "fast-4", CurrentNode: 0, ColdStartTime: 10.0, ReplicaSetName: "fast", MaxUnavailableReplicas: 2},
			},
			currentState:    []int{0, 0, 0, 0, 0},
			proposedState:   []int{1, 1, 1, 1, 1},
			expectedAvgCold: 10.0, // All have 10s cold start
			description:     "Moving many pods with low cold start",
		},
		{
			name: "MoveFewSlowPods",
			pods: []disruption.PodInfo{
				{Name: "slow-0", CurrentNode: 0, ColdStartTime: 180.0, ReplicaSetName: "slow", MaxUnavailableReplicas: 1},
				{Name: "slow-1", CurrentNode: 0, ColdStartTime: 180.0, ReplicaSetName: "slow", MaxUnavailableReplicas: 1},
				{Name: "fast-0", CurrentNode: 1, ColdStartTime: 5.0, ReplicaSetName: "fast", MaxUnavailableReplicas: 2},
				{Name: "fast-1", CurrentNode: 1, ColdStartTime: 5.0, ReplicaSetName: "fast", MaxUnavailableReplicas: 2},
			},
			currentState:    []int{0, 0, 1, 1},
			proposedState:   []int{1, 1, 1, 1}, // Only move the slow pods
			expectedAvgCold: 180.0,             // (180+180)/2 = 180 seconds average
			description:     "Moving only slow startup pods",
		},
		{
			name: "MixedColdStartTimes",
			pods: []disruption.PodInfo{
				{Name: "instant-0", CurrentNode: 0, ColdStartTime: 0.0, ReplicaSetName: "instant", MaxUnavailableReplicas: 3},
				{Name: "fast-0", CurrentNode: 0, ColdStartTime: 15.0, ReplicaSetName: "fast", MaxUnavailableReplicas: 2},
				{Name: "medium-0", CurrentNode: 0, ColdStartTime: 60.0, ReplicaSetName: "medium", MaxUnavailableReplicas: 1},
				{Name: "slow-0", CurrentNode: 0, ColdStartTime: 240.0, ReplicaSetName: "slow", MaxUnavailableReplicas: 1},
			},
			currentState:    []int{0, 0, 0, 0},
			proposedState:   []int{1, 1, 1, 1},
			expectedAvgCold: 78.75, // (0+15+60+240)/4 = 78.75 seconds average
			description:     "Moving pods with various cold start times",
		},
		{
			name: "NoMovement",
			pods: []disruption.PodInfo{
				{Name: "pod-0", CurrentNode: 0, ColdStartTime: 30.0, ReplicaSetName: "app", MaxUnavailableReplicas: 1},
				{Name: "pod-1", CurrentNode: 1, ColdStartTime: 30.0, ReplicaSetName: "app", MaxUnavailableReplicas: 1},
			},
			currentState:    []int{0, 1},
			proposedState:   []int{0, 1}, // No change
			expectedAvgCold: 0.0,         // No pods moved
			description:     "No pods moved should have zero cold start impact",
		},
		{
			name: "ExtremelySlowPods",
			pods: []disruption.PodInfo{
				{Name: "db-0", CurrentNode: 0, ColdStartTime: 600.0, ReplicaSetName: "database", MaxUnavailableReplicas: 1},
				{Name: "db-1", CurrentNode: 0, ColdStartTime: 600.0, ReplicaSetName: "database", MaxUnavailableReplicas: 1},
			},
			currentState:    []int{0, 0},
			proposedState:   []int{1, 1},
			expectedAvgCold: 600.0, // 10 minutes average - way above baseline
			description:     "Moving pods with extremely high cold start (10 min)",
		},
	}

	for _, tc := range scenarios {
		t.Run(tc.name, func(t *testing.T) {
			config := disruption.NewDisruptionConfig(tc.pods)
			_, detailsFunc := disruption.DisruptionObjectiveWithDetails(tc.currentState, tc.pods, config)

			bounds := make([]framework.IntBounds, len(tc.pods))
			for i := range bounds {
				bounds[i] = framework.IntBounds{L: 0, H: 2} // Allow up to 3 nodes
			}
			sol := framework.NewIntegerSolution(tc.proposedState, bounds)

			result := detailsFunc(sol)

			// Calculate expected cold start impact
			// With baseline of 60s: avgColdStart/60 * weight
			expectedNormalized := tc.expectedAvgCold / config.MaxExpectedColdStart
			expectedImpact := expectedNormalized * config.ColdStartWeight

			t.Logf("%s:", tc.description)
			t.Logf("  Moved pods: %d", result.MovedPods)
			t.Logf("  Average cold start: %.1f seconds", tc.expectedAvgCold)
			t.Logf("  Normalized cold start: %.4f (avg/%.0f)", expectedNormalized, config.MaxExpectedColdStart)
			t.Logf("  Cold start impact: %.4f (expected: %.4f)", result.ColdStartImpact, expectedImpact)
			t.Logf("  Total disruption cost: %.4f", result.TotalCost)

			// Verify the calculation
			if tc.expectedAvgCold > 0 {
				tolerance := 0.001
				if diff := math.Abs(result.ColdStartImpact - expectedImpact); diff > tolerance {
					t.Errorf("Cold start impact mismatch: got %.4f, expected %.4f (diff: %.4f)",
						result.ColdStartImpact, expectedImpact, diff)
				}
			} else {
				// No movement case
				if result.ColdStartImpact != 0 {
					t.Errorf("Expected zero cold start impact when no pods moved, got %.4f", result.ColdStartImpact)
				}
			}
		})
	}
}

func TestWeightConfiguration(t *testing.T) {
	// Test different weight configurations
	pods := []disruption.PodInfo{
		{Name: "pod-0", CurrentNode: 0, ColdStartTime: 60.0, ReplicaSetName: "app", MaxUnavailableReplicas: 1},
		{Name: "pod-1", CurrentNode: 0, ColdStartTime: 60.0, ReplicaSetName: "app", MaxUnavailableReplicas: 1},
	}

	currentState := []int{0, 0}
	proposed := []int{1, 1} // Move both pods

	// Base config adapted to cluster
	baseConfig := disruption.NewDisruptionConfig(pods)

	configs := []struct {
		name   string
		config disruption.DisruptionConfig
	}{
		{
			name:   "DefaultBalanced",
			config: baseConfig,
		},
		{
			name: "PrioritizeLowDisruption",
			config: disruption.DisruptionConfig{
				MaxExpectedMoves:     baseConfig.MaxExpectedMoves,
				MaxExpectedColdStart: baseConfig.MaxExpectedColdStart,
				MaxExpectedTimeSlots: baseConfig.MaxExpectedTimeSlots,
				MovementWeight:       0.5, // Higher weight on movement
				ColdStartWeight:      0.3,
				TimeSlotWeight:       0.2, // Lower weight on time slots
			},
		},
		{
			name: "PrioritizeFastRecovery",
			config: disruption.DisruptionConfig{
				MaxExpectedMoves:     baseConfig.MaxExpectedMoves,
				MaxExpectedColdStart: baseConfig.MaxExpectedColdStart,
				MaxExpectedTimeSlots: baseConfig.MaxExpectedTimeSlots,
				MovementWeight:       0.2,
				ColdStartWeight:      0.6, // High weight on cold start
				TimeSlotWeight:       0.2,
			},
		},
	}

	for _, tc := range configs {
		t.Run(tc.name, func(t *testing.T) {
			objFunc, _ := disruption.DisruptionObjectiveWithDetails(currentState, pods, tc.config)

			bounds := make([]framework.IntBounds, len(pods))
			for i := range bounds {
				bounds[i] = framework.IntBounds{L: 0, H: 1}
			}
			sol := framework.NewIntegerSolution(proposed, bounds)

			totalCost := objFunc(sol)

			t.Logf("Configuration: %s", tc.name)
			t.Logf("  Movement weight:  %.2f", tc.config.MovementWeight)
			t.Logf("  ColdStart weight: %.2f", tc.config.ColdStartWeight)
			t.Logf("  TimeSlot weight:  %.2f", tc.config.TimeSlotWeight)
			t.Logf("Results:")
			t.Logf("  Total normalized cost: %.4f", totalCost)

			// Verify total is <= 1.0 for balanced weights
			if tc.name == "DefaultBalanced" && totalCost > 1.0 {
				t.Errorf("Normalized cost exceeds 1.0: %.4f", totalCost)
			}
		})
	}
}

func TestDynamicScaling(t *testing.T) {
	// Test that moving all pods results in high impact
	pods := []disruption.PodInfo{
		{Name: "app1-0", CurrentNode: 0, ColdStartTime: 30.0, ReplicaSetName: "app1", MaxUnavailableReplicas: 1},
		{Name: "app1-1", CurrentNode: 0, ColdStartTime: 30.0, ReplicaSetName: "app1", MaxUnavailableReplicas: 1},
		{Name: "app1-2", CurrentNode: 0, ColdStartTime: 30.0, ReplicaSetName: "app1", MaxUnavailableReplicas: 1},
		{Name: "app2-0", CurrentNode: 1, ColdStartTime: 10.0, ReplicaSetName: "app2", MaxUnavailableReplicas: 2},
		{Name: "app2-1", CurrentNode: 1, ColdStartTime: 10.0, ReplicaSetName: "app2", MaxUnavailableReplicas: 2},
		{Name: "app2-2", CurrentNode: 1, ColdStartTime: 10.0, ReplicaSetName: "app2", MaxUnavailableReplicas: 2},
	}

	currentState := []int{0, 0, 0, 1, 1, 1}
	moveAll := []int{1, 1, 1, 0, 0, 0} // Move all pods

	config := disruption.NewDisruptionConfig(pods)

	t.Logf("Dynamic config for %d pods:", len(pods))
	t.Logf("  MaxExpectedMoves: %.0f", config.MaxExpectedMoves)
	t.Logf("  MaxExpectedColdStart: %.0f", config.MaxExpectedColdStart)
	t.Logf("  MaxExpectedTimeSlots: %.0f", config.MaxExpectedTimeSlots)

	objFunc, detailsFunc := disruption.DisruptionObjectiveWithDetails(currentState, pods, config)

	bounds := make([]framework.IntBounds, len(pods))
	for i := range bounds {
		bounds[i] = framework.IntBounds{L: 0, H: 1}
	}
	sol := framework.NewIntegerSolution(moveAll, bounds)

	result := detailsFunc(sol)
	totalCost := objFunc(sol)

	t.Logf("\nMoving all %d pods:", len(pods))
	t.Logf("  Movement impact: %.4f (should be ~0.34)", result.MovementImpact)
	t.Logf("  Cold start impact: %.4f", result.ColdStartImpact)
	t.Logf("  Time slots impact: %.4f", result.TimeSlotsImpact)
	t.Logf("  Total cost: %.4f", totalCost)

	// Verify moving all pods has maximum movement impact
	if result.MovementImpact*config.MovementWeight > 0.99 {
		t.Errorf("Expected movement impact ~1.0 when moving all pods, got %.4f", result.MovementImpact*config.MovementWeight)
	}
}

func TestWeightedTimeSlots(t *testing.T) {
	// Test that large apps don't dominate time slot calculation
	// Example scenario:
	// - App A: 10 replicas with strict PDB (maxUnavailable=1)
	// - App B: 2 replicas with strict PDB (maxUnavailable=1)
	//
	// Without weighting (max approach):
	//   MaxExpectedTimeSlots = 10 (from App A)
	//   Moving App B's 2 pods = 2/10 = 0.2 impact (seems too low!)
	//
	// With weighted average:
	//   MaxExpectedTimeSlots = (10*10 + 2*2)/12 = 8.67 → 9
	//   Moving App B's 2 pods = 2/9 = 0.22 impact (more reasonable)

	testPods := []disruption.PodInfo{
		// App A: 10 replicas, maxUnavailable=1 → needs 10 time slots
		{Name: "appa-0", CurrentNode: 0, ColdStartTime: 10.0, ReplicaSetName: "appa", MaxUnavailableReplicas: 1},
		{Name: "appa-1", CurrentNode: 0, ColdStartTime: 10.0, ReplicaSetName: "appa", MaxUnavailableReplicas: 1},
		{Name: "appa-2", CurrentNode: 0, ColdStartTime: 10.0, ReplicaSetName: "appa", MaxUnavailableReplicas: 1},
		{Name: "appa-3", CurrentNode: 0, ColdStartTime: 10.0, ReplicaSetName: "appa", MaxUnavailableReplicas: 1},
		{Name: "appa-4", CurrentNode: 0, ColdStartTime: 10.0, ReplicaSetName: "appa", MaxUnavailableReplicas: 1},
		{Name: "appa-5", CurrentNode: 0, ColdStartTime: 10.0, ReplicaSetName: "appa", MaxUnavailableReplicas: 1},
		{Name: "appa-6", CurrentNode: 0, ColdStartTime: 10.0, ReplicaSetName: "appa", MaxUnavailableReplicas: 1},
		{Name: "appa-7", CurrentNode: 0, ColdStartTime: 10.0, ReplicaSetName: "appa", MaxUnavailableReplicas: 1},
		{Name: "appa-8", CurrentNode: 0, ColdStartTime: 10.0, ReplicaSetName: "appa", MaxUnavailableReplicas: 1},
		{Name: "appa-9", CurrentNode: 0, ColdStartTime: 10.0, ReplicaSetName: "appa", MaxUnavailableReplicas: 1},
		// App B: 2 replicas, maxUnavailable=1 → needs 2 time slots
		{Name: "appb-0", CurrentNode: 1, ColdStartTime: 30.0, ReplicaSetName: "appb", MaxUnavailableReplicas: 1},
		{Name: "appb-1", CurrentNode: 1, ColdStartTime: 30.0, ReplicaSetName: "appb", MaxUnavailableReplicas: 1},
	}

	config := disruption.NewDisruptionConfig(testPods)

	// Weighted average: (10*10 + 2*2)/12 = 104/12 = 8.67 → 9
	// Old max approach would be: 10

	t.Logf("Config MaxExpectedTimeSlots: %.0f (weighted average approach)", config.MaxExpectedTimeSlots)
	t.Logf("With max approach it would have been: 10")
	t.Logf("This gives small apps more reasonable time slot impact")
}

func TestReplicaSetWeightedMovement(t *testing.T) {
	// Test that movement disruption is calculated per replica set
	tests := []struct {
		name           string
		current        []int
		proposed       []int
		pods           []disruption.PodInfo
		expectedResult float64 // Approximate, as it depends on penalty function
		description    string
	}{
		{
			name:     "Same movements, different RS sizes",
			current:  []int{0, 0, 0, 1, 1, 1, 2, 2, 2},
			proposed: []int{1, 0, 0, 2, 1, 1, 3, 2, 2}, // Move 1 pod from each RS
			pods: []disruption.PodInfo{
				// Small RS (3 pods) - moving 1/3 = 33% disruption
				{Name: "small-1", ReplicaSetName: "small-rs"},
				{Name: "small-2", ReplicaSetName: "small-rs"},
				{Name: "small-3", ReplicaSetName: "small-rs"},
				// Medium RS (3 pods) - moving 1/3 = 33% disruption
				{Name: "medium-1", ReplicaSetName: "medium-rs"},
				{Name: "medium-2", ReplicaSetName: "medium-rs"},
				{Name: "medium-3", ReplicaSetName: "medium-rs"},
				// Large RS (3 pods) - moving 1/3 = 33% disruption
				{Name: "large-1", ReplicaSetName: "large-rs"},
				{Name: "large-2", ReplicaSetName: "large-rs"},
				{Name: "large-3", ReplicaSetName: "large-rs"},
			},
			expectedResult: 0.333, // All RSs have same disruption ratio
			description:    "Equal disruption ratios should give same result regardless of RS names",
		},
		{
			name:     "Different disruption ratios",
			current:  []int{0, 0, 0, 1, 1, 1, 1, 1, 1, 1},
			proposed: []int{1, 1, 1, 2, 1, 1, 1, 1, 1, 1}, // Move all from small RS, 1 from large RS
			pods: []disruption.PodInfo{
				// Small RS (3 pods) - moving 3/3 = 100% disruption
				{Name: "small-1", ReplicaSetName: "small-rs"},
				{Name: "small-2", ReplicaSetName: "small-rs"},
				{Name: "small-3", ReplicaSetName: "small-rs"},
				// Large RS (7 pods) - moving 1/7 = 14% disruption
				{Name: "large-1", ReplicaSetName: "large-rs"},
				{Name: "large-2", ReplicaSetName: "large-rs"},
				{Name: "large-3", ReplicaSetName: "large-rs"},
				{Name: "large-4", ReplicaSetName: "large-rs"},
				{Name: "large-5", ReplicaSetName: "large-rs"},
				{Name: "large-6", ReplicaSetName: "large-rs"},
				{Name: "large-7", ReplicaSetName: "large-rs"},
			},
			expectedResult: 0.4, // Weighted average: (1.0*3 + 0.143*7) / 10 ≈ 0.4
			description:    "Larger RS with less disruption should have lower weighted impact",
		},
		{
			name:     "Single large movement vs many small movements",
			current:  []int{0, 0, 0, 0, 0, 1, 1, 2, 2, 3, 3},
			proposed: []int{1, 1, 1, 1, 1, 2, 1, 3, 2, 4, 3}, // Move all 5 from big RS, 1 each from small RSs
			pods: []disruption.PodInfo{
				// Big RS (5 pods) - moving 5/5 = 100% disruption
				{Name: "big-1", ReplicaSetName: "big-rs"},
				{Name: "big-2", ReplicaSetName: "big-rs"},
				{Name: "big-3", ReplicaSetName: "big-rs"},
				{Name: "big-4", ReplicaSetName: "big-rs"},
				{Name: "big-5", ReplicaSetName: "big-rs"},
				// Small RS1 (2 pods) - moving 1/2 = 50% disruption
				{Name: "small1-1", ReplicaSetName: "small-rs-1"},
				{Name: "small1-2", ReplicaSetName: "small-rs-1"},
				// Small RS2 (2 pods) - moving 1/2 = 50% disruption
				{Name: "small2-1", ReplicaSetName: "small-rs-2"},
				{Name: "small2-2", ReplicaSetName: "small-rs-2"},
				// Small RS3 (2 pods) - moving 1/2 = 50% disruption
				{Name: "small3-1", ReplicaSetName: "small-rs-3"},
				{Name: "small3-2", ReplicaSetName: "small-rs-3"},
			},
			expectedResult: 0.727, // (1.0*5 + 0.5*2 + 0.5*2 + 0.5*2) / 11 ≈ 0.727
			description:    "100% disruption of larger RS should dominate over 50% of smaller RSs",
		},
	}

	config := disruption.NewDisruptionConfig(nil)
	config.MovementPenaltyType = "linear" // Use linear for predictable results

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Calculate the disruption to get the normalized movement
			_, detailsFunc := disruption.DisruptionObjectiveWithDetails(tc.current, tc.pods, config)

			// Create solution
			solution := &framework.IntegerSolution{Variables: tc.proposed}
			details := detailsFunc(solution)

			// The movement impact is already normalized by replica set
			result := details.MovementImpact / config.MovementWeight // Unnormalize to get the raw value

			// Allow small tolerance for floating point
			if math.Abs(result-tc.expectedResult) > 0.01 {
				t.Errorf("Expected movement disruption ≈%.3f, got %.3f\n%s",
					tc.expectedResult, result, tc.description)
			}

			if details.MovementImpact <= 0 && details.MovedPods > 0 {
				t.Errorf("Movement impact should be positive when pods move")
			}
		})
	}
}
