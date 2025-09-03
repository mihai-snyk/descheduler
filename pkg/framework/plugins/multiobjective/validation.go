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

	"k8s.io/apimachinery/pkg/runtime"
)

// ValidateMultiObjectiveArgs validates the MultiObjective plugin arguments
func ValidateMultiObjectiveArgs(obj runtime.Object) error {
	args := obj.(*MultiObjectiveArgs)

	// Validate weights if provided
	if args.Weights != nil {
		if args.Weights.Cost < 0 || args.Weights.Cost > 1 {
			return fmt.Errorf("cost weight must be between 0 and 1, got %v", args.Weights.Cost)
		}
		if args.Weights.Disruption < 0 || args.Weights.Disruption > 1 {
			return fmt.Errorf("disruption weight must be between 0 and 1, got %v", args.Weights.Disruption)
		}
		if args.Weights.Balance < 0 || args.Weights.Balance > 1 {
			return fmt.Errorf("balance weight must be between 0 and 1, got %v", args.Weights.Balance)
		}

		// Weights should sum to 1 (with some tolerance for floating point)
		sum := args.Weights.Cost + args.Weights.Disruption + args.Weights.Balance
		if sum > 0 && (sum < 0.99 || sum > 1.01) {
			return fmt.Errorf("weights should sum to 1.0, got %v", sum)
		}
	}

	return nil
}
