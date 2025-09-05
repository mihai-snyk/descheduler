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
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"sigs.k8s.io/descheduler/pkg/api/v1alpha1"
	"sigs.k8s.io/descheduler/pkg/generated/clientset/versioned"
)

// SchedulingHintReservation provides atomic slot reservation for scheduling hints
type SchedulingHintReservation struct {
	clientset versioned.Interface
	logger    klog.Logger
}

// NewSchedulingHintReservation creates a new atomic reservation client
func NewSchedulingHintReservation(config *rest.Config, logger klog.Logger) (*SchedulingHintReservation, error) {
	clientset, err := versioned.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &SchedulingHintReservation{
		clientset: clientset,
		logger:    logger,
	}, nil
}

// TryReserveSlot attempts to atomically reserve a scheduling slot for a pod
// Returns the target node name if successful, empty string if no slots available
func (shr *SchedulingHintReservation) TryReserveSlot(ctx context.Context, pod *v1.Pod) string {
	// Extract ReplicaSet info from pod
	replicaSet, err := extractReplicaSetFromPod(pod)
	if err != nil {
		shr.logger.V(3).Info("Cannot extract ReplicaSet from pod", "pod", pod.Name, "error", err.Error())
		return ""
	}

	// Find active SchedulingHints
	hints, err := shr.findActiveHintsForReplicaSet(ctx, replicaSet)
	if err != nil {
		shr.logger.V(3).Info("Cannot find hints for ReplicaSet", "replicaSet", replicaSet, "error", err.Error())
		return ""
	}

	// Try to reserve a slot from any available hint (best solution first)
	for _, hint := range hints {
		targetNode := shr.tryReserveSlotFromHint(ctx, hint, replicaSet)
		if targetNode != "" {
			shr.logger.Info("Successfully reserved slot for pod",
				"pod", pod.Name,
				"replicaSet", replicaSet,
				"targetNode", targetNode,
				"hint", hint.Name)
			return targetNode
		}
	}

	shr.logger.V(2).Info("No available slots found for ReplicaSet", "replicaSet", replicaSet)
	return ""
}

// tryReserveSlotFromHint attempts to reserve a slot from a specific hint with retry logic
func (shr *SchedulingHintReservation) tryReserveSlotFromHint(ctx context.Context, hint *v1alpha1.SchedulingHint, replicaSet string) string {
	maxRetries := 5
	baseDelay := 10 * time.Millisecond

	for retry := 0; retry < maxRetries; retry++ {
		// Get fresh copy of the hint
		freshHint, err := shr.clientset.Descheduler().SchedulingHints().Get(ctx, hint.Name, metav1.GetOptions{})
		if err != nil {
			shr.logger.V(3).Info("Failed to get fresh hint", "hint", hint.Name, "error", err.Error())
			return ""
		}

		// Find the ReplicaSet movement in the best solution (rank 1)
		if len(freshHint.Spec.Solutions) == 0 {
			return ""
		}

		bestSolution := freshHint.Spec.Solutions[0] // Rank 1 solution
		var rsMovement *v1alpha1.ReplicaSetMovement

		for i := range bestSolution.ReplicaSetMovements {
			movement := &bestSolution.ReplicaSetMovements[i]
			rsKey := fmt.Sprintf("%s/%s", movement.Namespace, movement.ReplicaSetName)
			if rsKey == replicaSet {
				rsMovement = movement
				break
			}
		}

		if rsMovement == nil {
			return "" // No movement for this ReplicaSet
		}

		// Find a node with available slots
		var targetNode string
		for nodeName, availableSlots := range rsMovement.AvailableSlots {
			if availableSlots > 0 {
				targetNode = nodeName
				break
			}
		}

		if targetNode == "" {
			return "" // No available slots
		}

		// Atomically decrement the slot
		rsMovement.AvailableSlots[targetNode]--
		if rsMovement.ScheduledCount == nil {
			rsMovement.ScheduledCount = make(map[string]int)
		}
		rsMovement.ScheduledCount[targetNode]++

		// Try to update (will fail if another thread updated first)
		_, err = shr.clientset.Descheduler().SchedulingHints().Update(ctx, freshHint, metav1.UpdateOptions{})
		if err == nil {
			// Success!
			shr.logger.V(2).Info("Reserved slot successfully",
				"replicaSet", replicaSet,
				"targetNode", targetNode,
				"remainingSlots", rsMovement.AvailableSlots[targetNode],
				"scheduledCount", rsMovement.ScheduledCount[targetNode])
			return targetNode
		}

		// Conflict - another scheduler updated first, retry
		if errors.IsConflict(err) {
			delay := baseDelay * time.Duration(1<<retry) // Exponential backoff
			shr.logger.V(3).Info("Update conflict, retrying",
				"retry", retry+1,
				"maxRetries", maxRetries,
				"delay", delay,
				"error", err.Error())
			time.Sleep(delay)
			continue
		}

		// Other error - give up
		shr.logger.V(2).Info("Failed to update hint", "error", err.Error())
		return ""
	}

	shr.logger.V(2).Info("Failed to reserve slot after retries", "replicaSet", replicaSet, "retries", maxRetries)
	return ""
}

// findActiveHintsForReplicaSet finds SchedulingHints that contain movements for the given ReplicaSet
func (shr *SchedulingHintReservation) findActiveHintsForReplicaSet(ctx context.Context, replicaSet string) ([]*v1alpha1.SchedulingHint, error) {
	// List all active hints
	hintList, err := shr.clientset.Descheduler().SchedulingHints().List(ctx, metav1.ListOptions{
		LabelSelector: "descheduler.io/plugin=multiobjective",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list hints: %w", err)
	}

	// Filter for non-expired hints that contain our ReplicaSet
	activeHints := make([]*v1alpha1.SchedulingHint, 0)
	now := time.Now()

	for i := range hintList.Items {
		hint := &hintList.Items[i]

		// Skip expired hints
		if hint.Spec.ExpirationTime != nil && hint.Spec.ExpirationTime.Time.Before(now) {
			continue
		}

		// Check if any solution contains our ReplicaSet
		for _, solution := range hint.Spec.Solutions {
			for _, movement := range solution.ReplicaSetMovements {
				rsKey := fmt.Sprintf("%s/%s", movement.Namespace, movement.ReplicaSetName)
				if rsKey == replicaSet {
					activeHints = append(activeHints, hint)
					goto nextHint // Found match, check next hint
				}
			}
		}
	nextHint:
	}

	return activeHints, nil
}

// extractReplicaSetFromPod extracts the ReplicaSet identifier from a pod
func extractReplicaSetFromPod(pod *v1.Pod) (string, error) {
	// Find ReplicaSet owner reference
	for _, ownerRef := range pod.OwnerReferences {
		if ownerRef.Kind == "ReplicaSet" {
			return fmt.Sprintf("%s/%s", pod.Namespace, ownerRef.Name), nil
		}
	}
	return "", fmt.Errorf("pod %s has no ReplicaSet owner", pod.Name)
}
