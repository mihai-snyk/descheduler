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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
)

const (
	// Algorithm defaults
	DefaultPopulationSize       = 400
	DefaultMaxGenerations       = 1000
	DefaultCrossoverProbability = 0.90
	DefaultMutationProbability  = 0.30
	DefaultTournamentSize       = 3

	// Weight defaults
	DefaultWeightCost       = 0.33
	DefaultWeightDisruption = 0.33
	DefaultWeightBalance    = 0.34
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

func RegisterDefaults(scheme *runtime.Scheme) error {
	klog.V(5).InfoS("Registering defaults", "pluginName", PluginName)
	scheme.AddTypeDefaultingFunc(&MultiObjectiveArgs{}, func(obj interface{}) {
		SetDefaults_MultiObjectiveArgs(obj.(*MultiObjectiveArgs))
	})
	return nil
}

func SetDefaults_MultiObjectiveArgs(obj runtime.Object) {
	args := obj.(*MultiObjectiveArgs)

	if args.Weights == nil {
		args.Weights = &WeightConfig{
			Cost:       DefaultWeightCost,
			Disruption: DefaultWeightDisruption,
			Balance:    DefaultWeightBalance,
		}
	} else {
		// Set individual weight defaults if not specified
		if args.Weights.Cost == 0 && args.Weights.Disruption == 0 && args.Weights.Balance == 0 {
			args.Weights.Cost = DefaultWeightCost
			args.Weights.Disruption = DefaultWeightDisruption
			args.Weights.Balance = DefaultWeightBalance
		}
	}
}
