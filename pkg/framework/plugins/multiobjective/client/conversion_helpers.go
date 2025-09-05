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

package client

import (
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/descheduler/pkg/api/v1alpha1"
	framework "sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
)

// GenerateHintName generates a consistent name for a SchedulingHint based on cluster fingerprint
func GenerateHintName(clusterFingerprint string) string {
	return fmt.Sprintf("multiobjective-hints-%s", clusterFingerprint)
}

// ConvertOptimizationResults converts multi-objective optimization results to a SchedulingHint
func ConvertOptimizationResults(
	clusterFingerprint string,
	clusterNodes []string,
	originalReplicaSetDistribution []v1alpha1.ReplicaSetDistribution,
	results []OptimizationResult,
	nodes []framework.NodeInfo,
	pods []framework.PodInfo,
	deschedulerVersion string,
) *v1alpha1.SchedulingHint {

	now := metav1.Now()
	expirationTime := metav1.NewTime(now.Add(24 * time.Hour))

	hint := &v1alpha1.SchedulingHint{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "descheduler.io/v1alpha1",
			Kind:       "SchedulingHint",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: GenerateHintName(clusterFingerprint),
			Labels: map[string]string{
				"descheduler.io/plugin":              "multiobjective",
				"descheduler.io/cluster-fingerprint": clusterFingerprint,
			},
		},
		Spec: v1alpha1.SchedulingHintSpec{
			ClusterFingerprint:             clusterFingerprint,
			ClusterNodes:                   clusterNodes,
			OriginalReplicaSetDistribution: originalReplicaSetDistribution,
			ExpirationTime:                 &expirationTime,
			GeneratedAt:                    &now,
			DeschedulerVersion:             deschedulerVersion,
		},
	}

	// Convert results to solutions
	solutions := make([]v1alpha1.OptimizationSolution, 0, len(results))

	for i, result := range results {
		// Create ReplicaSet-level movements with slot tracking
		replicaSetMovements := createReplicaSetMovementsWithSlots(result.Assignment, pods, nodes)

		solution := v1alpha1.OptimizationSolution{
			Rank:          i + 1,
			WeightedScore: result.WeightedScore,
			Objectives: v1alpha1.ObjectiveValues{
				Cost:       result.Objectives[0],
				Disruption: result.Objectives[1],
				Balance:    result.Objectives[2],
			},
			MovementCount:       result.MovementCount,
			ReplicaSetMovements: replicaSetMovements,
		}
		solutions = append(solutions, solution)
	}

	hint.Spec.Solutions = solutions

	return hint
}

// OptimizationResult represents a single optimization result
type OptimizationResult struct {
	Assignment    []int
	Objectives    []float64
	WeightedScore float64
	MovementCount int
}

// createReplicaSetMovementsWithSlots creates ReplicaSet movements with atomic slot tracking
func createReplicaSetMovementsWithSlots(assignment []int, pods []framework.PodInfo, nodes []framework.NodeInfo) []v1alpha1.ReplicaSetMovement {
	// Group pods by ReplicaSet and analyze their target distribution
	replicaSetTargets := make(map[string]map[string]int) // RS key -> {node name -> count}
	replicaSetNamespaces := make(map[string]string)      // RS key -> namespace

	for i, targetNodeIdx := range assignment {
		pod := pods[i]
		rsKey := fmt.Sprintf("%s/%s", pod.Namespace, pod.ReplicaSetName)
		replicaSetNamespaces[rsKey] = pod.Namespace

		targetNodeName := ""
		if targetNodeIdx >= 0 && targetNodeIdx < len(nodes) {
			targetNodeName = nodes[targetNodeIdx].Name
		}

		if replicaSetTargets[rsKey] == nil {
			replicaSetTargets[rsKey] = make(map[string]int)
		}
		replicaSetTargets[rsKey][targetNodeName]++
	}

	// Create ReplicaSet movements only for those that have changes
	movements := make([]v1alpha1.ReplicaSetMovement, 0)

	for rsKey, targetDistribution := range replicaSetTargets {
		// Check if this ReplicaSet's distribution actually changed
		hasChanges := false
		currentDistribution := make(map[string]int)

		// Calculate current distribution
		for _, pod := range pods {
			if fmt.Sprintf("%s/%s", pod.Namespace, pod.ReplicaSetName) == rsKey {
				currentNodeName := ""
				if pod.Node >= 0 && pod.Node < len(nodes) {
					currentNodeName = nodes[pod.Node].Name
				}
				currentDistribution[currentNodeName]++
			}
		}

		// Compare current vs target distribution
		for nodeName, targetCount := range targetDistribution {
			if currentDistribution[nodeName] != targetCount {
				hasChanges = true
				break
			}
		}
		for nodeName, currentCount := range currentDistribution {
			if targetDistribution[nodeName] != currentCount {
				hasChanges = true
				break
			}
		}

		if hasChanges {
			// Extract ReplicaSet name from key
			parts := strings.SplitN(rsKey, "/", 2)
			if len(parts) != 2 {
				continue
			}

			// Initialize slot tracking
			availableSlots := make(map[string]int)
			scheduledCount := make(map[string]int)

			for nodeName, targetCount := range targetDistribution {
				currentCount := currentDistribution[nodeName]
				// Available slots = how many MORE pods need to be scheduled to this node
				if targetCount > currentCount {
					availableSlots[nodeName] = targetCount - currentCount
					scheduledCount[nodeName] = 0 // Start counting from 0
				}
			}

			// Only create movement if there are actual slots to fill
			if len(availableSlots) > 0 {
				movement := v1alpha1.ReplicaSetMovement{
					ReplicaSetName:     parts[1],
					Namespace:          parts[0],
					TargetDistribution: targetDistribution,
					AvailableSlots:     availableSlots,
					ScheduledCount:     scheduledCount,
					Reason:             "Multi-objective optimization: atomic slot reservation for ReplicaSet redistribution",
				}
				movements = append(movements, movement)
			}
		}
	}

	return movements
}
