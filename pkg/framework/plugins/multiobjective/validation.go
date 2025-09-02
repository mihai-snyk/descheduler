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

	if args.MaxEvictionsPerCycle < 0 {
		return fmt.Errorf("maxEvictionsPerCycle must be non-negative")
	}

	if args.PopulationSize < 1 {
		return fmt.Errorf("populationSize must be at least 1")
	}

	if args.Generations < 1 {
		return fmt.Errorf("generations must be at least 1")
	}

	return nil
}
