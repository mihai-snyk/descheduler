package disruption

import (
	"fmt"
	"testing"
)

func TestPenaltyComparison(t *testing.T) {
	// Create test pods
	pods := []PodInfo{
		{Name: "pod-0", CurrentNode: 0, ColdStartTime: 10.0, ReplicaSetName: "app", MaxUnavailableReplicas: 1},
		{Name: "pod-1", CurrentNode: 0, ColdStartTime: 10.0, ReplicaSetName: "app", MaxUnavailableReplicas: 1},
		{Name: "pod-2", CurrentNode: 0, ColdStartTime: 10.0, ReplicaSetName: "app", MaxUnavailableReplicas: 1},
		{Name: "pod-3", CurrentNode: 0, ColdStartTime: 10.0, ReplicaSetName: "app", MaxUnavailableReplicas: 1},
		{Name: "pod-4", CurrentNode: 0, ColdStartTime: 10.0, ReplicaSetName: "app", MaxUnavailableReplicas: 1},
		{Name: "pod-5", CurrentNode: 0, ColdStartTime: 10.0, ReplicaSetName: "app", MaxUnavailableReplicas: 1},
	}

	penaltyTypes := []string{"linear", "sqrt", "log", "exp", "threshold"}

	t.Log("\n=== Movement Penalty Comparison ===")
	t.Log("Total pods: 6")
	t.Log("\nMoved Pods | Linear | Sqrt   | Log    | Exp    | Threshold")
	t.Log("-----------|--------|--------|--------|--------|----------")

	for movedPods := 0; movedPods <= 6; movedPods++ {
		fmt.Printf("%-10d |", movedPods)

		for _, penaltyType := range penaltyTypes {
			config := NewDisruptionConfigWithPenalty(pods, penaltyType)
			penalty := calculateMovementPenalty(movedPods, len(pods), config)
			fmt.Printf(" %6.4f |", penalty)
		}
		fmt.Println()
	}

	// Show impact on disruption score for moving 1 pod
	t.Log("\n=== Disruption Score for Moving 1 Pod ===")
	currentState := []int{0, 0, 0, 0, 0, 0}
	proposed := []int{1, 0, 0, 0, 0, 0} // Move first pod

	for _, penaltyType := range penaltyTypes {
		config := NewDisruptionConfigWithPenalty(pods, penaltyType)
		result := calculateDisruption(currentState, proposed, pods, config)

		t.Logf("%s penalty: Total=%.4f (Movement=%.4f, ColdStart=%.4f, TimeSlots=%.4f)",
			penaltyType, result.TotalCost, result.MovementImpact,
			result.ColdStartImpact, result.TimeSlotsImpact)
	}
}

// TestSmallCostImprovement shows how different penalties affect cost trade-offs
func TestSmallCostImprovement(t *testing.T) {
	pods := []PodInfo{
		{Name: "pod-0", CurrentNode: 0, ColdStartTime: 10.0, ReplicaSetName: "app", MaxUnavailableReplicas: 1},
		{Name: "pod-1", CurrentNode: 0, ColdStartTime: 10.0, ReplicaSetName: "app", MaxUnavailableReplicas: 1},
		{Name: "pod-2", CurrentNode: 0, ColdStartTime: 10.0, ReplicaSetName: "app", MaxUnavailableReplicas: 1},
		{Name: "pod-3", CurrentNode: 0, ColdStartTime: 10.0, ReplicaSetName: "app", MaxUnavailableReplicas: 1},
		{Name: "pod-4", CurrentNode: 0, ColdStartTime: 10.0, ReplicaSetName: "app", MaxUnavailableReplicas: 1},
		{Name: "pod-5", CurrentNode: 0, ColdStartTime: 10.0, ReplicaSetName: "app", MaxUnavailableReplicas: 1},
	}

	// Simulate MixedNodeTypes_CostFocused scenario
	// Current: all on expensive node (normalized cost = 0.6341)
	// Alternative: move 1 pod to cheap node (normalized cost = 0.6585)
	costIncrease := 0.6585 - 0.6341 // 0.0244

	weights := struct {
		cost       float64
		disruption float64
		balance    float64
	}{0.60, 0.20, 0.20}

	t.Log("\n=== Cost vs Disruption Trade-off ===")
	t.Logf("Cost increase from move: %.4f (weighted: %.4f)",
		costIncrease, costIncrease*weights.cost)

	currentState := []int{0, 0, 0, 0, 0, 0}
	proposed := []int{1, 0, 0, 0, 0, 0} // Move one pod

	for _, penaltyType := range []string{"linear", "sqrt", "log", "threshold"} {
		config := NewDisruptionConfigWithPenalty(pods, penaltyType)
		result := calculateDisruption(currentState, proposed, pods, config)

		disruptionPenalty := result.TotalCost * weights.disruption
		costPenalty := costIncrease * weights.cost

		t.Logf("\n%s penalty:", penaltyType)
		t.Logf("  Disruption score: %.4f (weighted: %.4f)", result.TotalCost, disruptionPenalty)
		t.Logf("  Cost penalty: %.4f", costPenalty)

		if disruptionPenalty < costPenalty {
			t.Logf("  ✓ Move is WORTH IT (disruption < cost penalty)")
		} else {
			t.Logf("  ✗ Move NOT worth it (disruption > cost penalty)")
		}
	}
}
