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
	// Hints contains the list of scheduling hints
	Hints []PodSchedulingHint `json:"hints"`

	// ExpirationTime is when these hints should no longer be used
	ExpirationTime *metav1.Time `json:"expirationTime,omitempty"`

	// DeschedulerGeneration tracks which descheduler run created these hints
	DeschedulerGeneration int64 `json:"deschedulerGeneration,omitempty"`
}

// PodSchedulingHint represents a hint for a single pod
type PodSchedulingHint struct {
	// PodName is the name of the pod
	PodName string `json:"podName"`

	// PodNamespace is the namespace of the pod
	PodNamespace string `json:"podNamespace"`

	// TargetNode is the recommended node for this pod
	TargetNode string `json:"targetNode"`

	// Priority indicates how strongly this hint should be followed (higher = stronger)
	Priority int32 `json:"priority,omitempty"`

	// Reason explains why this placement is recommended
	Reason string `json:"reason,omitempty"`
}

// SchedulingHintStatus defines the observed state of SchedulingHint
type SchedulingHintStatus struct {
	// AppliedHints tracks which hints have been used by the scheduler
	AppliedHints []AppliedHint `json:"appliedHints,omitempty"`
}

// AppliedHint represents a hint that was used by the scheduler
type AppliedHint struct {
	PodName   string       `json:"podName"`
	AppliedAt *metav1.Time `json:"appliedAt"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SchedulingHintList contains a list of SchedulingHint
type SchedulingHintList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SchedulingHint `json:"items"`
}
