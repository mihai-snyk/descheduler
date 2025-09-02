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
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	frameworktypes "sigs.k8s.io/descheduler/pkg/framework/types"
)

const PluginName = "MultiObjective"

// MultiObjective is a plugin that implements multi-objective optimization using NSGA-II
type MultiObjective struct {
	logger klog.Logger
	handle frameworktypes.Handle
	args   *MultiObjectiveArgs
}

var _ frameworktypes.BalancePlugin = &MultiObjective{}

// New builds plugin from its arguments while passing a handle
func New(ctx context.Context, args runtime.Object, handle frameworktypes.Handle) (frameworktypes.Plugin, error) {
	multiObjectiveArgs, ok := args.(*MultiObjectiveArgs)
	if !ok {
		return nil, fmt.Errorf("want args to be of type MultiObjectiveArgs, got %T", args)
	}
	logger := klog.FromContext(ctx).WithValues("plugin", PluginName)

	return &MultiObjective{
		logger: logger,
		handle: handle,
		args:   multiObjectiveArgs,
	}, nil
}

// Name retrieves the plugin name
func (m *MultiObjective) Name() string {
	return PluginName
}

// Balance extension point implementation for the plugin
func (m *MultiObjective) Balance(ctx context.Context, nodes []*v1.Node) *frameworktypes.Status {
	logger := klog.FromContext(klog.NewContext(ctx, m.logger)).WithValues("ExtensionPoint", frameworktypes.BalanceExtensionPoint)
	logger.Info("MultiObjective balance plugin triggered!", "nodeCount", len(nodes))

	// TODO: Implement multi-objective optimization logic here
	// Balance plugins can analyze all pods across all nodes holistically

	return nil
}
