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

	if args.MaxEvictionsPerCycle == 0 {
		args.MaxEvictionsPerCycle = 5
	}
	if args.PopulationSize == 0 {
		args.PopulationSize = 50
	}
	if args.Generations == 0 {
		args.Generations = 100
	}
}
