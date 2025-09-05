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
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"log"
	"sort"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"sigs.k8s.io/descheduler/pkg/api/v1alpha1"
	podutil "sigs.k8s.io/descheduler/pkg/descheduler/pod"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/algorithms"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/client"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/constraints"
	framework "sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/objectives/balance"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/objectives/cost"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/objectives/disruption"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/objectives/resourcecost"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/warmstart"
	frameworktypes "sigs.k8s.io/descheduler/pkg/framework/types"
	"sigs.k8s.io/descheduler/pkg/generated/clientset/versioned"
	"sigs.k8s.io/descheduler/pkg/utils"
)

const PluginName = "MultiObjective"

// MultiObjective is a plugin that implements multi-objective optimization using NSGA-II
type MultiObjective struct {
	logger    klog.Logger
	handle    frameworktypes.Handle
	args      *MultiObjectiveArgs
	podFilter podutil.FilterFunc
}

var _ frameworktypes.BalancePlugin = &MultiObjective{}

// New builds plugin from its arguments while passing a handle
func New(ctx context.Context, args runtime.Object, handle frameworktypes.Handle) (frameworktypes.Plugin, error) {
	multiObjectiveArgs, ok := args.(*MultiObjectiveArgs)
	if !ok {
		return nil, fmt.Errorf("want args to be of type MultiObjectiveArgs, got %T", args)
	}
	logger := klog.FromContext(ctx).WithValues("plugin", PluginName)

	// Create the pod filter that excludes system pods
	podFilter := func(pod *v1.Pod) bool {
		// Exclude kube-system namespace pods
		if pod.Namespace == "kube-system" {
			return false
		}

		// Exclude pods that shouldn't be considered for descheduling
		return pod.Status.Phase == v1.PodRunning &&
			!utils.IsDaemonsetPod(pod.OwnerReferences) &&
			!utils.IsMirrorPod(pod) &&
			!utils.IsStaticPod(pod)
	}

	return &MultiObjective{
		logger:    logger,
		handle:    handle,
		args:      multiObjectiveArgs,
		podFilter: podFilter,
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

	// Suppress verbose logging
	log.SetOutput(ioutil.Discard)
	defer log.SetOutput(nil) // Reset after function

	// Filter out control plane nodes
	workerNodes := make([]*v1.Node, 0, len(nodes))
	for _, node := range nodes {
		// Only add workers (skip control plane nodes)
		if _, ok := node.Labels["node-role.kubernetes.io/control-plane"]; !ok {
			workerNodes = append(workerNodes, node)
		}
	}

	if len(workerNodes) == 0 {
		logger.Info("No worker nodes found to optimize")
		return nil
	}

	logger.Info("Filtered nodes",
		"totalNodes", len(nodes),
		"workerNodes", len(workerNodes),
		"controlPlaneNodes", len(nodes)-len(workerNodes))

	// Get all pods across all nodes using the same approach as other plugins
	allPods := make([]*v1.Pod, 0)
	for _, node := range workerNodes {
		pods, err := podutil.ListAllPodsOnANode(node.Name, m.handle.GetPodsAssignedToNodeFunc(), m.podFilter)
		if err != nil {
			return &frameworktypes.Status{
				Err: fmt.Errorf("error listing pods on node %s: %v", node.Name, err),
			}
		}
		allPods = append(allPods, pods...)
	}

	// Get PDBs for the cluster
	pdbs, err := m.getPDBs(ctx)
	if err != nil {
		logger.V(2).Error(err, "Failed to get PDBs")
		// Continue without PDBs, using defaults
		pdbs = nil
	} else {
		logger.Info("Found PodDisruptionBudgets", "count", len(pdbs))
		for _, pdb := range pdbs {
			logger.V(2).Info("PDB details",
				"name", pdb.Name,
				"namespace", pdb.Namespace,
				"maxUnavailable", pdb.Spec.MaxUnavailable,
				"minAvailable", pdb.Spec.MinAvailable)
		}
	}

	// Convert to internal format and print cluster statistics
	nodeInfos, podInfos, err := m.convertToInternalFormat(workerNodes, allPods, pdbs)
	if err != nil {
		logger.Error(err, "Failed to convert nodes/pods to internal format")
		return &frameworktypes.Status{
			Err: err,
		}
	}
	m.printClusterStatistics(logger, nodeInfos, podInfos)

	// Print algorithm configuration
	m.printAlgorithmConfig(logger)

	// Try to fetch existing solutions for seeding
	existingSolutions := m.fetchExistingSolutions(ctx, nodeInfos, podInfos)

	// Run multi-objective optimization with seeding
	results := m.runOptimization(logger, nodeInfos, podInfos, existingSolutions)

	// Display top results
	m.displayTopResults(logger, results, nodeInfos, podInfos)

	// Get the best solution and calculate intermediate moves
	if len(results) > 0 {
		bestSolution := results[0]

		// Calculate feasible intermediate moves respecting PDBs
		feasibleMoves := m.calculateFeasibleMovements(bestSolution.assignment, podInfos)

		// Print intermediate moves (without actually evicting)
		m.printIntermediateMoves(logger, feasibleMoves, bestSolution.assignment, nodeInfos, podInfos)

		// Create cluster fingerprint
		clusterFingerprint := m.createClusterFingerprint(nodeInfos, podInfos)
		logger.Info("Cluster fingerprint", "fingerprint", clusterFingerprint)

		// Store all solutions as scheduling hints
		err := m.storeSchedulingHints(ctx, results, nodeInfos, podInfos, clusterFingerprint)
		if err != nil {
			logger.Error(err, "Failed to store scheduling hints")
			// Don't fail the plugin, just log the error
		}
	}

	return nil
}

type solutionResult struct {
	assignment      []int
	objectives      []float64
	normalizedScore float64
	movementCount   int
}

func (m *MultiObjective) getPDBs(ctx context.Context) ([]*policyv1.PodDisruptionBudget, error) {
	pdbList, err := m.handle.ClientSet().PolicyV1().PodDisruptionBudgets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	pdbs := make([]*policyv1.PodDisruptionBudget, 0, len(pdbList.Items))
	for i := range pdbList.Items {
		pdbs = append(pdbs, &pdbList.Items[i])
	}

	return pdbs, nil
}

func (m *MultiObjective) getMaxUnavailableForPod(pod *v1.Pod, pdbs []*policyv1.PodDisruptionBudget) (int, bool) {
	// Default to 1 if no PDB found
	defaultMaxUnavailable := 1

	if len(pdbs) == 0 {
		return defaultMaxUnavailable, false
	}

	// Check each PDB to see if it matches this pod
	for _, pdb := range pdbs {
		// Check if PDB is in the same namespace
		if pdb.Namespace != pod.Namespace {
			continue
		}

		// Check if the PDB's selector matches the pod
		selector, err := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
		if err != nil {
			continue
		}

		if selector.Matches(labels.Set(pod.Labels)) {
			// Found a matching PDB
			maxUnavail := defaultMaxUnavailable

			if pdb.Spec.MaxUnavailable != nil {
				if pdb.Spec.MaxUnavailable.Type == intstr.Int {
					maxUnavail = pdb.Spec.MaxUnavailable.IntValue()
				}
				// For percentage type, we'd need to know the total replicas
				// For now, use default
			}
			// If minAvailable is set instead, we can't easily calculate maxUnavailable
			// without knowing total replicas, so use default

			m.logger.V(3).Info("Found matching PDB for pod",
				"pod", pod.Name,
				"namespace", pod.Namespace,
				"pdb", pdb.Name,
				"maxUnavailable", maxUnavail)

			return maxUnavail, true
		}
	}

	return defaultMaxUnavailable, false
}

func (m *MultiObjective) convertToInternalFormat(nodes []*v1.Node, pods []*v1.Pod, pdbs []*policyv1.PodDisruptionBudget) ([]framework.NodeInfo, []framework.PodInfo, error) {
	// Convert nodes
	nodeMap := make(map[string]int)
	nodeInfos := make([]framework.NodeInfo, 0, len(nodes))

	for i, node := range nodes {
		nodeMap[node.Name] = i

		// Get node capacity
		cpuCap := node.Status.Capacity.Cpu().MilliValue()
		memCap := node.Status.Capacity.Memory().Value()

		// Get instance info from labels/annotations
		instanceType, ok := node.Labels["node.kubernetes.io/instance-type"]
		if !ok {
			return nil, nil, fmt.Errorf("failed to get instance type %s", node.Name)
		}

		// Get region from labels/annotations
		region, ok := node.Labels["topology.kubernetes.io/region"]
		if !ok {
			return nil, nil, fmt.Errorf("failed to get region %s", node.Name)
		}

		lifecycle := "on-demand"
		if node.Labels["karpenter.sh/capacity-type"] == "spot" ||
			node.Labels["eks.amazonaws.com/capacityType"] == "spot" {
			lifecycle = "spot"
		}

		// Get cost from scraped pricing data
		hourlyCost, err := cost.GetInstanceCost(region, instanceType, lifecycle)
		if err != nil {
			return nil, nil, err
		}

		nodeInfos = append(nodeInfos, framework.NodeInfo{
			Idx:               i,
			Name:              node.Name,
			CPUCapacity:       float64(cpuCap),
			MemCapacity:       float64(memCap),
			InstanceType:      instanceType,
			InstanceLifecycle: lifecycle,
			Region:            region,
			HourlyCost:        hourlyCost,
		})
	}

	// Convert pods and calculate node usage
	podInfos := make([]framework.PodInfo, 0, len(pods))
	for i, pod := range pods {
		// Pods are already filtered, no need to check again

		nodeIdx, exists := nodeMap[pod.Spec.NodeName]
		if !exists {
			continue // Pod not on any of our nodes
		}

		// Calculate pod resources
		var cpuReq, memReq int64
		for _, container := range pod.Spec.Containers {
			cpuReq += container.Resources.Requests.Cpu().MilliValue()
			memReq += container.Resources.Requests.Memory().Value()
		}

		// Get replica set info
		rs := ""
		for _, owner := range pod.OwnerReferences {
			if owner.Kind == "ReplicaSet" {
				rs = owner.Name
				break
			}
		}
		if rs == "" {
			return nil, nil, fmt.Errorf("found pod without owner %s", pod.Name)
		}

		// Get MaxUnavailable from PDB if exists
		maxUnavailable, _ := m.getMaxUnavailableForPod(pod, pdbs)

		podInfo := framework.PodInfo{
			Idx:                    i,
			Name:                   pod.Name,
			Namespace:              pod.Namespace,
			Node:                   nodeIdx,
			CPURequest:             float64(cpuReq),
			MemRequest:             float64(memReq),
			ReplicaSetName:         rs,
			ColdStartTime:          0.0, // Default 0 seconds
			MaxUnavailableReplicas: maxUnavailable,
		}

		podInfos = append(podInfos, podInfo)

		// Update node usage
		nodeInfos[nodeIdx].CPUUsed += float64(cpuReq)
		nodeInfos[nodeIdx].MemUsed += float64(memReq)
	}

	return nodeInfos, podInfos, nil
}

func (m *MultiObjective) printClusterStatistics(logger klog.Logger, nodes []framework.NodeInfo, pods []framework.PodInfo) {
	// Count pods with PDB protection
	replicaSetsWithPDB := make(map[string]int)
	for _, pod := range pods {
		// We'll consider a pod PDB-protected if we found a matching PDB
		// (even if maxUnavailable is 1, it's still explicitly protected)
		if pod.MaxUnavailableReplicas >= 0 {
			replicaSetsWithPDB[pod.ReplicaSetName] = pod.MaxUnavailableReplicas
		}
	}

	logger.Info("Cluster statistics",
		"totalNodes", len(nodes),
		"totalPods", len(pods))

	// Node statistics
	var totalCPUCap, totalMemCap, totalCPUUsed, totalMemUsed float64
	var onDemandCount, spotCount int

	for _, node := range nodes {
		totalCPUCap += node.CPUCapacity
		totalMemCap += node.MemCapacity
		totalCPUUsed += node.CPUUsed
		totalMemUsed += node.MemUsed

		if node.InstanceLifecycle == "spot" {
			spotCount++
		} else {
			onDemandCount++
		}

		cpuUtil := (node.CPUUsed / node.CPUCapacity) * 100
		memUtil := (node.MemUsed / node.MemCapacity) * 100

		logger.V(2).Info("Node details",
			"node", node.Name,
			"instanceType", node.InstanceType,
			"lifecycle", node.InstanceLifecycle,
			"cpuCores", fmt.Sprintf("%.0f/%.0f", node.CPUUsed/1000, node.CPUCapacity/1000),
			"cpuUtilization", fmt.Sprintf("%.1f%%", cpuUtil),
			"memoryGB", fmt.Sprintf("%.1f/%.1f", node.MemUsed/1e9, node.MemCapacity/1e9),
			"memUtilization", fmt.Sprintf("%.1f%%", memUtil))
	}

	logger.Info("Node types",
		"onDemand", onDemandCount,
		"spot", spotCount)

	logger.Info("Cluster resources",
		"cpuCores", fmt.Sprintf("%.0f/%.0f", totalCPUUsed/1000, totalCPUCap/1000),
		"cpuUtilization", fmt.Sprintf("%.1f%%", (totalCPUUsed/totalCPUCap)*100),
		"memoryGB", fmt.Sprintf("%.1f/%.1f", totalMemUsed/1e9, totalMemCap/1e9),
		"memUtilization", fmt.Sprintf("%.1f%%", (totalMemUsed/totalMemCap)*100))
}

func (m *MultiObjective) printAlgorithmConfig(logger klog.Logger) {
	logger.Info("Algorithm configuration",
		"weightCost", m.args.Weights.Cost,
		"weightDisruption", m.args.Weights.Disruption,
		"weightBalance", m.args.Weights.Balance,
		"populationSize", DefaultPopulationSize,
		"generations", DefaultMaxGenerations,
		"crossoverProbability", DefaultCrossoverProbability,
		"mutationProbability", DefaultMutationProbability)
}

func (m *MultiObjective) runOptimization(logger klog.Logger, nodes []framework.NodeInfo, pods []framework.PodInfo, existingSolutions []solutionResult) []solutionResult {

	// Create objectives
	weights := m.args.Weights
	if weights == nil {
		weights = &WeightConfig{
			Cost:       m.args.Weights.Cost,
			Disruption: m.args.Weights.Disruption,
			Balance:    m.args.Weights.Balance,
		}
	}

	// Get current state for disruption objective
	currentState := make([]int, len(pods))
	for i, p := range pods {
		currentState[i] = p.Node
	}

	disruptionConfig := disruption.NewDisruptionConfig(pods)
	balanceConfig := balance.DefaultBalanceConfig()

	// Create objective functions - using new effective cost objective
	objectives := []framework.ObjectiveFunc{
		resourcecost.EffectiveCostObjectiveFunc(nodes, pods),
		disruption.DisruptionObjective(currentState, pods, disruptionConfig),
		balance.BalanceObjectiveFunc(pods, nodes, balanceConfig),
	}

	// Create constraints
	constraintFuncs := []framework.Constraint{
		constraints.ResourceConstraint(pods, nodes),
	}

	// Create problem with optional seeding
	var problem framework.Problem
	baseProblem := createKubernetesProblem(nodes, pods, objectives, constraintFuncs)

	// If we have existing solutions, create a seeded problem
	if len(existingSolutions) > 0 {
		logger.Info("Seeding optimization with existing solutions", "existingSolutions", len(existingSolutions))
		problem = m.createSeededProblem(baseProblem, existingSolutions)
	} else {
		logger.Info("No existing solutions found - using random initialization")
		problem = baseProblem
	}

	// Configure NSGA-II
	config := algorithms.NSGA2Config{
		PopulationSize:       int(DefaultPopulationSize),
		MaxGenerations:       int(DefaultMaxGenerations),
		CrossoverProbability: DefaultCrossoverProbability,
		MutationProbability:  DefaultMutationProbability,
		TournamentSize:       DefaultTournamentSize,
		ParallelExecution:    true,
	}

	// Run optimization
	logger.Info("Running multi-objective optimization...")
	nsga2 := algorithms.NewNSGAII(config, problem)
	population := nsga2.Run()

	// Get Pareto front
	fronts := algorithms.NonDominatedSort(population)
	if len(fronts) == 0 || len(fronts[0]) == 0 {
		logger.Info("No solutions found in Pareto front - returning original state")
		return []solutionResult{}
	}

	paretoFront := fronts[0]
	logger.Info("Found Pareto-optimal solutions", "count", len(paretoFront))
	logger.Info("")
	logger.Info("")

	// Convert solutions to results
	results := make([]solutionResult, 0, len(paretoFront))
	for _, sol := range paretoFront {
		intSol := sol.Solution.(*framework.IntegerSolution)

		// Count movements
		movements := 0
		for i, newNode := range intSol.Variables {
			if newNode != pods[i].Node {
				movements++
			}
		}

		// Calculate weighted score
		weightedScore := weights.Cost*sol.Value[0] +
			weights.Disruption*sol.Value[1] +
			weights.Balance*sol.Value[2]

		results = append(results, solutionResult{
			assignment:      intSol.Variables,
			objectives:      sol.Value,
			normalizedScore: weightedScore,
			movementCount:   movements,
		})
	}

	// Sort by weighted score
	sort.Slice(results, func(i, j int) bool {
		return results[i].normalizedScore < results[j].normalizedScore
	})

	return results
}

func (m *MultiObjective) displayTopResults(logger klog.Logger, results []solutionResult, nodes []framework.NodeInfo, pods []framework.PodInfo) {
	// Show top 10 or all if fewer
	count := 10
	if len(results) < count {
		count = len(results)
	}

	logger.Info("Top optimization solutions", "displaying", count, "totalParetoOptimal", len(results))

	for i := 0; i < count; i++ {
		r := results[i]
		movementPercent := (float64(r.movementCount) / float64(len(pods))) * 100

		logger.Info("Solution",
			"rank", i+1,
			"effectiveCostObjective", fmt.Sprintf("%.4f", r.objectives[0]),
			"disruptionObjective", fmt.Sprintf("%.4f", r.objectives[1]),
			"balanceObjective", fmt.Sprintf("%.6f", r.objectives[2]),
			"weightedScore", fmt.Sprintf("%.4f", r.normalizedScore),
			"podMovements", r.movementCount,
			"totalPods", len(pods),
			"movementPercent", fmt.Sprintf("%.1f%%", movementPercent))

		// Show movement summary by node
		movements := make(map[string]int)
		for j, newNode := range r.assignment {
			if newNode != pods[j].Node {
				fromNode := nodes[pods[j].Node].Name
				toNode := nodes[newNode].Name
				key := fmt.Sprintf("%s->%s", fromNode, toNode)
				movements[key]++
			}
		}

		if len(movements) > 0 {
			movStrs := make([]string, 0, len(movements))
			for k, v := range movements {
				movStrs = append(movStrs, fmt.Sprintf("%s: %d", k, v))
			}
			sort.Strings(movStrs)
			logger.V(2).Info("Movement details",
				"solution", i+1,
				"movements", strings.Join(movStrs, ", "))
		}

		// Calculate feasible movements respecting PDBs
		feasibleMoves := m.calculateFeasibleMovements(r.assignment, pods)
		logger.Info("Execution plan (respecting PDBs)",
			"solutionRank", i+1,
			"feasibleMovesFirstIteration", len(feasibleMoves))

		// Group by replica set for clarity
		movesByRS := make(map[string][]string)
		for _, podIdx := range feasibleMoves {
			rs := pods[podIdx].ReplicaSetName
			movesByRS[rs] = append(movesByRS[rs], pods[podIdx].Name)
		}

		for rs, moves := range movesByRS {
			logger.Info("Feasible moves by replica set",
				"solutionRank", i+1,
				"replicaSet", rs,
				"podCount", len(moves),
				"pods", strings.Join(moves, ", "))
		}
		logger.Info("")
		logger.Info("")
	}

	logger.Info("Optimization complete", "totalParetoOptimalSolutions", len(results))
}

// calculateFeasibleMovements determines which pods can actually be moved in the first iteration
// while respecting PDB constraints (maxUnavailable)
func (m *MultiObjective) calculateFeasibleMovements(targetAssignment []int, pods []framework.PodInfo) []int {
	feasibleMoves := []int{}

	// Group pods by replica set
	replicaSets := make(map[string][]int) // RS name -> pod indices
	for i, pod := range pods {
		if replicaSets[pod.ReplicaSetName] == nil {
			replicaSets[pod.ReplicaSetName] = []int{}
		}
		replicaSets[pod.ReplicaSetName] = append(replicaSets[pod.ReplicaSetName], i)
	}

	// For each replica set, determine how many pods we can move
	for _, podIndices := range replicaSets {
		// Find maxUnavailable for this RS
		maxUnavailable := 1
		if len(podIndices) > 0 {
			maxUnavailable = pods[podIndices[0]].MaxUnavailableReplicas
			if maxUnavailable <= 0 {
				continue // Cannot move any pods from this RS
			}
		}

		// Count how many pods need to move from this RS
		podsToMove := []int{}
		for _, idx := range podIndices {
			if targetAssignment[idx] != pods[idx].Node {
				podsToMove = append(podsToMove, idx)
			}
		}

		// Can only move up to maxUnavailable pods
		moveCount := len(podsToMove)
		if moveCount > maxUnavailable {
			moveCount = maxUnavailable
		}

		// Add the feasible moves
		for i := 0; i < moveCount; i++ {
			feasibleMoves = append(feasibleMoves, podsToMove[i])
		}
	}

	return feasibleMoves
}

// KubernetesProblem implementation
type KubernetesProblem struct {
	nodes       []framework.NodeInfo
	pods        []framework.PodInfo
	objectives  []framework.ObjectiveFunc
	constraints []framework.Constraint
}

func createKubernetesProblem(nodes []framework.NodeInfo, pods []framework.PodInfo,
	objectives []framework.ObjectiveFunc, constraints []framework.Constraint) *KubernetesProblem {
	return &KubernetesProblem{
		nodes:       nodes,
		pods:        pods,
		objectives:  objectives,
		constraints: constraints,
	}
}

func (kp *KubernetesProblem) Evaluate(sol framework.Solution) ([]float64, error) {
	values := make([]float64, len(kp.objectives))
	for i, obj := range kp.objectives {
		values[i] = obj(sol)
	}
	return values, nil
}

func (kp *KubernetesProblem) VariableCount() int {
	return len(kp.pods)
}

func (kp *KubernetesProblem) ObjectiveCount() int {
	return len(kp.objectives)
}

func (kp *KubernetesProblem) ObjectiveFuncs() []framework.ObjectiveFunc {
	return kp.objectives
}

func (kp *KubernetesProblem) Name() string {
	return "KubernetesPodScheduling"
}

func (kp *KubernetesProblem) Constraints() []framework.Constraint {
	return kp.constraints
}

func (kp *KubernetesProblem) Bounds() []framework.Bounds {
	bounds := make([]framework.Bounds, len(kp.pods))
	nodeCount := len(kp.nodes)
	for i := range kp.pods {
		bounds[i] = framework.Bounds{
			L: 0,
			H: float64(nodeCount - 1),
		}
	}
	return bounds
}

func (kp *KubernetesProblem) TrueParetoFront(numPoints int) []framework.ObjectiveSpacePoint {
	// NP-hard complete, that's why we write this insanity
	return nil
}

func (kp *KubernetesProblem) Initialize(popSize int) []framework.Solution {
	// Use GCSH for warm start
	gcshConfig := warmstart.GCSHConfig{
		Nodes:               kp.nodes,
		Pods:                kp.pods,
		Objectives:          []framework.ObjectiveFunc{kp.objectives[0], kp.objectives[2]}, // Effective cost and balance
		Constraints:         kp.constraints,
		IncludeCurrentState: true,
	}

	gcsh := warmstart.NewGCSH(gcshConfig)
	return gcsh.GenerateInitialPopulation(popSize)
}

// printIntermediateMoves prints the feasible moves that can be executed immediately
func (m *MultiObjective) printIntermediateMoves(logger klog.Logger, feasibleMoves []int, targetAssignment []int, nodes []framework.NodeInfo, pods []framework.PodInfo) {
	if len(feasibleMoves) == 0 {
		logger.Info("No intermediate moves possible due to PDB constraints")
		return
	}

	logger.Info("INTERMEDIATE MOVES (respecting PDB constraints)", "feasibleMoves", len(feasibleMoves), "totalTargetMoves", m.countTotalMoves(targetAssignment, pods))

	// Group moves by type for better understanding
	movesByType := make(map[string][]string)

	for _, podIdx := range feasibleMoves {
		pod := pods[podIdx]
		currentNode := nodes[pod.Node]
		targetNode := nodes[targetAssignment[podIdx]]

		// Categorize move type
		var moveType string
		if currentNode.InstanceLifecycle == "on-demand" && targetNode.InstanceLifecycle == "spot" {
			moveType = "On-demand → Spot (cost saving)"
		} else if currentNode.InstanceLifecycle == "spot" && targetNode.InstanceLifecycle == "on-demand" {
			moveType = "Spot → On-demand (reliability)"
		} else if currentNode.InstanceType != targetNode.InstanceType {
			moveType = fmt.Sprintf("Instance type change (%s → %s)", currentNode.InstanceType, targetNode.InstanceType)
		} else {
			moveType = "Same type migration"
		}

		moveInfo := fmt.Sprintf("%s: %s → %s", pod.Name, currentNode.Name, targetNode.Name)
		movesByType[moveType] = append(movesByType[moveType], moveInfo)
	}

	// Print moves by type
	for moveType, moves := range movesByType {
		logger.Info("Move type", "type", moveType, "count", len(moves))
		for _, move := range moves {
			logger.V(1).Info("Scheduled move", "details", move)
		}
	}
}

// countTotalMoves counts total movements in a solution
func (m *MultiObjective) countTotalMoves(assignment []int, pods []framework.PodInfo) int {
	count := 0
	for i, newNode := range assignment {
		if newNode != pods[i].Node {
			count++
		}
	}
	return count
}

// createClusterFingerprint creates a deterministic hash of the current cluster state using ReplicaSets
func (m *MultiObjective) createClusterFingerprint(nodes []framework.NodeInfo, pods []framework.PodInfo) string {
	// Collect node names (nodes are stable)
	nodeNames := make([]string, len(nodes))
	for i, node := range nodes {
		nodeNames[i] = node.Name
	}
	sort.Strings(nodeNames) // Sort for deterministic ordering

	// Collect ReplicaSet info instead of individual pods
	// Group pods by ReplicaSet and count replicas per node
	replicaSetInfo := make(map[string]map[string]int) // RS -> {node -> count}

	for _, pod := range pods {
		rsKey := fmt.Sprintf("%s/%s", pod.Namespace, pod.ReplicaSetName)
		nodeName := ""
		if pod.Node >= 0 && pod.Node < len(nodes) {
			nodeName = nodes[pod.Node].Name
		}

		if replicaSetInfo[rsKey] == nil {
			replicaSetInfo[rsKey] = make(map[string]int)
		}
		replicaSetInfo[rsKey][nodeName]++
	}

	// Create deterministic string representation
	replicaSetKeys := make([]string, 0, len(replicaSetInfo))
	for rsKey := range replicaSetInfo {
		replicaSetKeys = append(replicaSetKeys, rsKey)
	}
	sort.Strings(replicaSetKeys)

	var replicaSetSpecs []string
	for _, rsKey := range replicaSetKeys {
		nodeDistribution := replicaSetInfo[rsKey]

		// Sort node names for deterministic ordering
		nodeNames := make([]string, 0, len(nodeDistribution))
		for nodeName := range nodeDistribution {
			nodeNames = append(nodeNames, nodeName)
		}
		sort.Strings(nodeNames)

		// Create spec like "namespace/rs-name:node1=2,node2=3"
		var nodePairs []string
		for _, nodeName := range nodeNames {
			count := nodeDistribution[nodeName]
			nodePairs = append(nodePairs, fmt.Sprintf("%s=%d", nodeName, count))
		}

		rsSpec := fmt.Sprintf("%s:%s", rsKey, strings.Join(nodePairs, ","))
		replicaSetSpecs = append(replicaSetSpecs, rsSpec)
	}

	// Create fingerprint from nodes and ReplicaSet distribution
	clusterSpec := fmt.Sprintf("nodes:%s|replicasets:%s",
		strings.Join(nodeNames, ","),
		strings.Join(replicaSetSpecs, ";"))

	// Create hash
	hash := sha256.Sum256([]byte(clusterSpec))
	return fmt.Sprintf("%x", hash)[:16] // Use first 16 characters for readability
}

// storeSchedulingHints stores all solutions as scheduling hints using the generated typed client
func (m *MultiObjective) storeSchedulingHints(ctx context.Context, results []solutionResult, nodes []framework.NodeInfo, pods []framework.PodInfo, clusterFingerprint string) error {
	// Deduplicate and filter results before storing
	filteredResults := m.deduplicateResults(results, pods)

	// Convert optimization results to SchedulingHint format
	optimizationResults := make([]client.OptimizationResult, len(filteredResults))
	for i, result := range filteredResults {
		optimizationResults[i] = client.OptimizationResult{
			Assignment:    result.assignment,
			Objectives:    result.objectives,
			WeightedScore: result.normalizedScore,
			MovementCount: result.movementCount,
		}
	}

	// Convert to SchedulingHint CR
	hint := client.ConvertOptimizationResults(
		clusterFingerprint,
		optimizationResults,
		nodes,
		pods,
		"descheduler-dev", // TODO: Get actual version
	)

	m.logger.Info("Creating SchedulingHint CR",
		"name", hint.Name,
		"clusterFingerprint", clusterFingerprint,
		"originalSolutions", len(results),
		"filteredSolutions", len(filteredResults),
		"totalReplicaSetMovements", func() int {
			total := 0
			for _, solution := range hint.Spec.Solutions {
				total += len(solution.ReplicaSetMovements)
			}
			return total
		}(),
		"expirationTime", hint.Spec.ExpirationTime.Format(time.RFC3339))

	// Try to create the SchedulingHint using the generated typed client
	err := m.createSchedulingHintCR(ctx, hint)
	if err != nil {
		m.logger.Error(err, "Failed to create SchedulingHint CR - continuing in simulation mode",
			"name", hint.Name)
		// Don't fail the plugin, just log the error and continue
		return nil
	}

	m.logger.Info("Successfully created SchedulingHint CR",
		"name", hint.Name)

	return nil
}

// createSchedulingHintCR creates the SchedulingHint CR using the generated typed client
func (m *MultiObjective) createSchedulingHintCR(ctx context.Context, hint *v1alpha1.SchedulingHint) error {
	// Try to get REST config - first in-cluster, then kubeconfig
	config, err := m.getRESTConfig()
	if err != nil {
		return fmt.Errorf("failed to get REST config: %w", err)
	}

	// Create our generated clientset
	clientset, err := versioned.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create SchedulingHint clientset: %w", err)
	}

	// Check if a hint with this name already exists
	existingHint, err := clientset.Descheduler().SchedulingHints().Get(ctx, hint.Name, metav1.GetOptions{})
	if err == nil {
		// Update existing hint
		existingHint.Spec = hint.Spec
		existingHint.Labels = hint.Labels

		updated, err := clientset.Descheduler().SchedulingHints().Update(ctx, existingHint, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update existing SchedulingHint: %w", err)
		}

		m.logger.Info("Updated existing SchedulingHint CR",
			"name", updated.Name,
			"resourceVersion", updated.ResourceVersion)
		return nil
	}

	// Create new hint
	created, err := clientset.Descheduler().SchedulingHints().Create(ctx, hint, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create SchedulingHint: %w", err)
	}

	m.logger.Info("Created new SchedulingHint CR",
		"name", created.Name,
		"resourceVersion", created.ResourceVersion,
		"uid", created.UID)

	// Update status to Active
	created.Status.Phase = v1alpha1.SchedulingHintPhaseActive
	created.Status.AppliedMovements = 0

	_, err = clientset.Descheduler().SchedulingHints().UpdateStatus(ctx, created, metav1.UpdateOptions{})
	if err != nil {
		m.logger.Error(err, "Failed to update SchedulingHint status", "name", created.Name)
		// Don't fail for status update errors
	}

	return nil
}

// getRESTConfig gets the REST config for creating custom resource clients
func (m *MultiObjective) getRESTConfig() (*rest.Config, error) {
	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// Fallback to kubeconfig
	m.logger.V(2).Info("In-cluster config not available, trying kubeconfig", "error", err.Error())

	// Try default kubeconfig locations
	kubeconfig := clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename()
	config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
	}

	return config, nil
}

// deduplicateResults removes duplicate solutions and filters out low-value results
func (m *MultiObjective) deduplicateResults(results []solutionResult, pods []framework.PodInfo) []solutionResult {
	if len(results) == 0 {
		return results
	}

	// Track seen assignments to avoid duplicates
	seenAssignments := make(map[string]bool)
	filteredResults := make([]solutionResult, 0)

	// Keep track of the best no-movement solution (current state)
	var bestNoMovementSolution *solutionResult

	// First pass: find the best no-movement solution
	for i, result := range results {
		if result.movementCount == 0 {
			if bestNoMovementSolution == nil || result.normalizedScore < bestNoMovementSolution.normalizedScore {
				bestNoMovementSolution = &results[i]
			}
		}
	}

	for _, result := range results {
		// Create a key for the assignment to detect duplicates
		assignmentKey := fmt.Sprintf("%v", result.assignment)

		// Skip if we've seen this exact assignment before
		if seenAssignments[assignmentKey] {
			m.logger.V(3).Info("Skipping duplicate solution", "movements", result.movementCount, "score", result.normalizedScore)
			continue
		}

		// Mark this assignment as seen and add to results
		seenAssignments[assignmentKey] = true
		filteredResults = append(filteredResults, result)

		m.logger.V(3).Info("Keeping solution",
			"rank", len(filteredResults),
			"movements", result.movementCount,
			"score", result.normalizedScore)
	}

	return filteredResults
}

// fetchExistingSolutions tries to fetch existing SchedulingHints to seed the optimization
func (m *MultiObjective) fetchExistingSolutions(ctx context.Context, nodes []framework.NodeInfo, pods []framework.PodInfo) []solutionResult {
	// Get current cluster fingerprint
	currentFingerprint := m.createClusterFingerprint(nodes, pods)

	// Try to get REST config
	config, err := m.getRESTConfig()
	if err != nil {
		m.logger.V(2).Info("Cannot fetch existing solutions - no cluster access", "error", err.Error())
		return nil
	}

	// Create clientset
	clientset, err := versioned.NewForConfig(config)
	if err != nil {
		m.logger.V(2).Info("Cannot create clientset for fetching solutions", "error", err.Error())
		return nil
	}

	// First try to get hints for exact cluster fingerprint match
	exactMatch, err := clientset.Descheduler().SchedulingHints().Get(ctx, client.GenerateHintName(currentFingerprint), metav1.GetOptions{})
	if err == nil {
		m.logger.Info("Found exact cluster fingerprint match",
			"fingerprint", currentFingerprint,
			"solutions", len(exactMatch.Spec.Solutions))
		return m.convertSchedulingHintToSolutions(exactMatch, nodes, pods)
	}

	m.logger.V(2).Info("No exact cluster fingerprint match, looking for latest hints",
		"currentFingerprint", currentFingerprint, "error", err.Error())

	// No exact match - get the most recent SchedulingHint from any cluster state
	allHints, err := clientset.Descheduler().SchedulingHints().List(ctx, metav1.ListOptions{
		LabelSelector: "descheduler.io/plugin=multiobjective",
	})
	if err != nil {
		m.logger.V(2).Info("Cannot list existing SchedulingHints", "error", err.Error())
		return nil
	}

	if len(allHints.Items) == 0 {
		m.logger.Info("No existing SchedulingHints found - starting fresh")
		return nil
	}

	// Find the most recent hint by creation timestamp
	var latestHint *v1alpha1.SchedulingHint
	for i := range allHints.Items {
		hint := &allHints.Items[i]
		if latestHint == nil || hint.CreationTimestamp.After(latestHint.CreationTimestamp.Time) {
			latestHint = hint
		}
	}

	if latestHint != nil {
		m.logger.Info("Found latest SchedulingHint for seeding",
			"name", latestHint.Name,
			"fingerprint", latestHint.Spec.ClusterFingerprint,
			"age", time.Since(latestHint.CreationTimestamp.Time).Round(time.Second),
			"solutions", len(latestHint.Spec.Solutions))

		return m.convertSchedulingHintToSolutions(latestHint, nodes, pods)
	}

	return nil
}

// convertSchedulingHintToSolutions converts a SchedulingHint CR back to solutionResult format
func (m *MultiObjective) convertSchedulingHintToSolutions(hint *v1alpha1.SchedulingHint, nodes []framework.NodeInfo, pods []framework.PodInfo) []solutionResult {
	solutions := make([]solutionResult, 0, len(hint.Spec.Solutions))

	// Create node name to index mapping
	nodeNameToIdx := make(map[string]int)
	for i, node := range nodes {
		nodeNameToIdx[node.Name] = i
	}

	// Create pod name to index mapping
	podNameToIdx := make(map[string]int)
	for i, pod := range pods {
		podKey := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
		podNameToIdx[podKey] = i
	}

	for _, solution := range hint.Spec.Solutions {
		// Reconstruct assignment from movements
		assignment := make([]int, len(pods))

		// Start with current assignment
		for i, pod := range pods {
			assignment[i] = pod.Node
		}

		// Apply ReplicaSet movements to reconstruct the target assignment
		for _, rsMovement := range solution.ReplicaSetMovements {
			rsKey := fmt.Sprintf("%s/%s", rsMovement.Namespace, rsMovement.ReplicaSetName)

			// Find all pods in this ReplicaSet
			rsPods := make([]int, 0)
			for i, pod := range pods {
				if fmt.Sprintf("%s/%s", pod.Namespace, pod.ReplicaSetName) == rsKey {
					rsPods = append(rsPods, i)
				}
			}

			// Distribute pods according to target distribution
			podIndex := 0
			for nodeName, replicaCount := range rsMovement.TargetDistribution {
				nodeIdx, nodeExists := nodeNameToIdx[nodeName]
				if !nodeExists {
					m.logger.V(3).Info("Skipping ReplicaSet movement for unknown node",
						"replicaSet", rsKey, "node", nodeName)
					continue
				}

				// Assign the specified number of replicas to this node
				for i := 0; i < replicaCount && podIndex < len(rsPods); i++ {
					assignment[rsPods[podIndex]] = nodeIdx
					podIndex++
				}
			}
		}

		solutionResult := solutionResult{
			assignment:      assignment,
			objectives:      []float64{solution.Objectives.Cost, solution.Objectives.Disruption, solution.Objectives.Balance},
			normalizedScore: solution.WeightedScore,
			movementCount:   solution.MovementCount,
		}

		solutions = append(solutions, solutionResult)
	}

	m.logger.Info("Converted SchedulingHint to solutions for seeding",
		"originalSolutions", len(hint.Spec.Solutions),
		"convertedSolutions", len(solutions))

	return solutions
}

// createSeededProblem wraps a problem to provide seeded initialization with existing solutions
func (m *MultiObjective) createSeededProblem(originalProblem *KubernetesProblem, existingSolutions []solutionResult) framework.Problem {
	return &SeededKubernetesProblem{
		KubernetesProblem: originalProblem,
		existingSolutions: existingSolutions,
		logger:            m.logger,
	}
}

// SeededKubernetesProblem wraps KubernetesProblem to provide seeded initialization
type SeededKubernetesProblem struct {
	*KubernetesProblem
	existingSolutions []solutionResult
	logger            klog.Logger
}

// Initialize seeds the population with existing solutions (70%) + random solutions (30%)
func (skp *SeededKubernetesProblem) Initialize(popSize int) []framework.Solution {
	solutions := make([]framework.Solution, 0, popSize)

	// Seed with existing solutions (up to 70% of population)
	seedCount := len(skp.existingSolutions)
	maxSeeds := popSize * 7 / 10
	if seedCount > maxSeeds {
		seedCount = maxSeeds
	}

	if seedCount > 0 {
		skp.logger.Info("Seeding population",
			"totalPopulation", popSize,
			"existingSolutions", len(skp.existingSolutions),
			"seedCount", seedCount,
			"seedPercentage", float64(seedCount)/float64(popSize)*100)

		for i := 0; i < seedCount; i++ {
			existingSol := skp.existingSolutions[i]

			// Convert to framework solution
			bounds := make([]framework.IntBounds, len(existingSol.assignment))
			for j := range bounds {
				bounds[j] = framework.IntBounds{L: 0, H: len(skp.nodes) - 1}
			}

			sol := &framework.IntegerSolution{
				Variables: make([]int, len(existingSol.assignment)),
				Bounds:    bounds,
			}
			copy(sol.Variables, existingSol.assignment)

			solutions = append(solutions, sol)
		}
	}

	// Fill remaining with random solutions for exploration
	remainingCount := popSize - len(solutions)
	if remainingCount > 0 {
		skp.logger.V(2).Info("Adding random solutions for exploration", "count", remainingCount)
		randomSolutions := skp.KubernetesProblem.Initialize(remainingCount)
		solutions = append(solutions, randomSolutions...)
	}

	return solutions
}
