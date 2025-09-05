/*
Copyright 2024 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package multiobjective

import (
	"fmt"
	"testing"

	"k8s.io/klog/v2"

	framework "sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
)

func TestDeduplicateResults(t *testing.T) {
	// Create a mock MultiObjective instance
	m := &MultiObjective{
		logger: klog.NewKlogr(),
	}

	// Test data with duplicates and no-movement solutions
	pods := []framework.PodInfo{
		{Idx: 0, Name: "pod-1", Node: 0},
		{Idx: 1, Name: "pod-2", Node: 1},
	}

	results := []solutionResult{
		// Current state (no movements) - best score
		{assignment: []int{0, 1}, objectives: []float64{0.1, 0.0, 0.2}, normalizedScore: 0.1, movementCount: 0},
		// Current state (no movements) - worse score (should be filtered)
		{assignment: []int{0, 1}, objectives: []float64{0.2, 0.0, 0.3}, normalizedScore: 0.2, movementCount: 0},
		// Good movement solution
		{assignment: []int{1, 0}, objectives: []float64{0.3, 0.5, 0.1}, normalizedScore: 0.3, movementCount: 2},
		// Duplicate of above (should be filtered)
		{assignment: []int{1, 0}, objectives: []float64{0.3, 0.5, 0.1}, normalizedScore: 0.3, movementCount: 2},
		// Small movement with poor score (should be filtered)
		{assignment: []int{0, 0}, objectives: []float64{0.8, 0.1, 0.4}, normalizedScore: 0.8, movementCount: 1},
		// Another good solution
		{assignment: []int{1, 1}, objectives: []float64{0.4, 0.3, 0.2}, normalizedScore: 0.4, movementCount: 1},
	}

	// Test deduplication
	filtered := m.deduplicateResults(results, pods)

	// Verify results
	if len(filtered) == 0 {
		t.Fatal("Expected some filtered results")
	}

	// Should have removed duplicates
	if len(filtered) >= len(results) {
		t.Errorf("Expected filtering to reduce results, got %d from %d", len(filtered), len(results))
	}

	// Check that only one no-movement solution remains
	noMovementCount := 0
	for _, result := range filtered {
		if result.movementCount == 0 {
			noMovementCount++
		}
	}

	if noMovementCount != 1 {
		t.Errorf("Expected exactly 1 no-movement solution, got %d", noMovementCount)
	}

	// Check that the best no-movement solution was kept
	foundBestNoMovement := false
	for _, result := range filtered {
		if result.movementCount == 0 && result.normalizedScore == 0.1 {
			foundBestNoMovement = true
			break
		}
	}

	if !foundBestNoMovement {
		t.Error("Expected to keep the best no-movement solution (score 0.1)")
	}

	// Verify no exact duplicates remain
	seenAssignments := make(map[string]bool)
	for _, result := range filtered {
		key := fmt.Sprintf("%v", result.assignment)
		if seenAssignments[key] {
			t.Errorf("Found duplicate assignment in filtered results: %v", result.assignment)
		}
		seenAssignments[key] = true
	}

	t.Logf("✅ Deduplication test passed!")
	t.Logf("   Original solutions: %d", len(results))
	t.Logf("   Filtered solutions: %d", len(filtered))
	t.Logf("   Duplicates removed: %d", len(results)-len(filtered))
}
