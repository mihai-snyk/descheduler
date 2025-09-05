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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SchedulingHint is a cluster-scoped resource that contains hints from the descheduler
// about optimal pod placements for the scheduler to consume
type SchedulingHint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SchedulingHintSpec   `json:"spec,omitempty"`
	Status SchedulingHintStatus `json:"status,omitempty"`
}

// SchedulingHintSpec defines the desired state of SchedulingHint
type SchedulingHintSpec struct {
	// ClusterFingerprint is a SHA256 hash of the cluster state (sorted node and pod names)
	// This field is used by multi-objective optimization to match hints to cluster states
	ClusterFingerprint string `json:"clusterFingerprint"`

	// Solutions contains optimization solutions from multi-objective algorithms
	Solutions []OptimizationSolution `json:"solutions"`

	// ExpirationTime is when these hints should no longer be used
	ExpirationTime *metav1.Time `json:"expirationTime"`

	// GeneratedAt indicates when these hints were generated
	GeneratedAt *metav1.Time `json:"generatedAt"`

	// DeschedulerVersion is the version of descheduler that generated these hints
	DeschedulerVersion string `json:"deschedulerVersion,omitempty"`
}

// SchedulingHintStatus defines the observed state of SchedulingHint
type SchedulingHintStatus struct {
	// Phase represents the current phase of the scheduling hints
	// +kubebuilder:validation:Enum=Active;Expired;Applied
	Phase SchedulingHintPhase `json:"phase,omitempty"`

	// AppliedMovements is the number of movements that have been applied
	AppliedMovements int `json:"appliedMovements,omitempty"`

	// LastAppliedTime is when movements were last applied
	LastAppliedTime *metav1.Time `json:"lastAppliedTime,omitempty"`

	// Conditions represent the latest available observations of the hint's current state
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// SchedulingHintPhase represents the phase of scheduling hints
type SchedulingHintPhase string

const (
	// SchedulingHintPhaseActive indicates the hints are active and can be used
	SchedulingHintPhaseActive SchedulingHintPhase = "Active"

	// SchedulingHintPhaseExpired indicates the hints have expired
	SchedulingHintPhaseExpired SchedulingHintPhase = "Expired"

	// SchedulingHintPhaseApplied indicates the hints have been applied
	SchedulingHintPhaseApplied SchedulingHintPhase = "Applied"
)

// OptimizationSolution represents a single solution from multi-objective optimization
type OptimizationSolution struct {
	// Rank is the solution rank in Pareto front (1 = best)
	Rank int `json:"rank"`

	// WeightedScore is the weighted objective score
	WeightedScore float64 `json:"weightedScore"`

	// Objectives contains the individual objective values
	Objectives ObjectiveValues `json:"objectives"`

	// MovementCount is the total number of pod movements in this solution
	MovementCount int `json:"movementCount"`

	// ReplicaSetMovements contains ReplicaSet-level movement recommendations
	ReplicaSetMovements []ReplicaSetMovement `json:"replicaSetMovements"`
}

// ObjectiveValues contains the values for each optimization objective
type ObjectiveValues struct {
	// Cost is the effective cost objective value
	Cost float64 `json:"cost"`

	// Disruption is the disruption objective value
	Disruption float64 `json:"disruption"`

	// Balance is the balance objective value
	Balance float64 `json:"balance"`
}

// ReplicaSetMovement represents a ReplicaSet-level movement recommendation with atomic slot tracking
type ReplicaSetMovement struct {
	// ReplicaSetName is the name of the ReplicaSet
	ReplicaSetName string `json:"replicaSetName"`

	// Namespace is the namespace of the ReplicaSet
	Namespace string `json:"namespace"`

	// TargetDistribution specifies how replicas should be distributed across nodes
	// Key: node name, Value: target number of replicas
	TargetDistribution map[string]int `json:"targetDistribution"`

	// AvailableSlots tracks remaining scheduling slots (decrements as pods are scheduled)
	// Key: node name, Value: remaining available slots for atomic reservation
	AvailableSlots map[string]int `json:"availableSlots"`

	// ScheduledCount tracks how many pods have been successfully scheduled to each node
	// Key: node name, Value: number of pods already scheduled via this hint
	ScheduledCount map[string]int `json:"scheduledCount,omitempty"`

	// Reason provides the optimization rationale for this movement
	Reason string `json:"reason"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SchedulingHintList contains a list of SchedulingHint
type SchedulingHintList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SchedulingHint `json:"items"`
}
