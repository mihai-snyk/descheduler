package multiobjective_test

import (
	"fmt"
	"strings"
	"testing"

	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/algorithms"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/constraints"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/objectives/balance"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/objectives/cost"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/objectives/disruption"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/warmstart"
)

// TestMultiObjectiveOptimization runs all multi-objective optimization scenarios
func TestMultiObjectiveOptimization(t *testing.T) {
	// Define test scenarios with all configuration in one place
	testCases := []struct {
		name             string
		nodes            []NodeConfig
		pods             []PodConfig
		weightProfile    WeightProfile
		populationSize   int
		maxGenerations   int
		expectedBehavior string
	}{
		// {
		// 	name: "SmallBalanced",
		// 	nodes: []NodeConfig{
		// 		{Name: "node-1", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "on-demand"},
		// 		{Name: "node-2", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "on-demand"},
		// 		{Name: "node-3", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "on-demand"},
		// 	},
		// 	pods: []PodConfig{
		// 		{Name: "app-0", CPU: 1000, Mem: 2e9, Node: 0, RS: "app", MaxUnavail: 1},
		// 		{Name: "app-1", CPU: 1000, Mem: 2e9, Node: 1, RS: "app", MaxUnavail: 1},
		// 		{Name: "app-2", CPU: 1000, Mem: 2e9, Node: 2, RS: "app", MaxUnavail: 1},
		// 		{Name: "web-0", CPU: 500, Mem: 1e9, Node: 0, RS: "web", MaxUnavail: 1},
		// 		{Name: "web-1", CPU: 500, Mem: 1e9, Node: 1, RS: "web", MaxUnavail: 1},
		// 		{Name: "web-2", CPU: 500, Mem: 1e9, Node: 2, RS: "web", MaxUnavail: 1},
		// 	},
		// 	weightProfile:    WeightProfile{Cost: 0.33, Disruption: 0.33, Balance: 0.34},
		// 	populationSize:   50,
		// 	maxGenerations:   100,
		// 	expectedBehavior: "Already well-balanced, should maintain with minimal changes",
		// },
		// {
		// 	name: "Imbalanced_IdenticalNodes",
		// 	nodes: []NodeConfig{
		// 		{Name: "node-1", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "on-demand"},
		// 		{Name: "node-2", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "on-demand"},
		// 		{Name: "node-3", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "on-demand"},
		// 	},
		// 	pods: []PodConfig{
		// 		{Name: "app-0", CPU: 1000, Mem: 2e9, Node: 0, RS: "app", MaxUnavail: 1},
		// 		{Name: "app-1", CPU: 1000, Mem: 2e9, Node: 0, RS: "app", MaxUnavail: 1},
		// 		{Name: "web-0", CPU: 500, Mem: 1e9, Node: 0, RS: "web", MaxUnavail: 1},
		// 		{Name: "web-1", CPU: 500, Mem: 1e9, Node: 0, RS: "web", MaxUnavail: 1},
		// 		{Name: "db-0", CPU: 500, Mem: 1e9, Node: 0, RS: "db", MaxUnavail: 0},
		// 	},
		// 	weightProfile:    WeightProfile{Cost: 0.33, Disruption: 0.33, Balance: 0.34},
		// 	populationSize:   100,
		// 	maxGenerations:   50,
		// 	expectedBehavior: "Limited optimization: already optimal for cost/disruption with identical nodes",
		// },
		// {
		// 	name: "MixedNodeTypes_Imbalanced",
		// 	nodes: []NodeConfig{
		// 		{Name: "node-1", CPU: 2000, Mem: 4e9, Type: "t3.small", Region: "us-east-1", Lifecycle: "on-demand"},
		// 		{Name: "node-2", CPU: 2000, Mem: 4e9, Type: "t3.small", Region: "us-east-1", Lifecycle: "on-demand"},
		// 		{Name: "node-3", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "on-demand"},
		// 		{Name: "node-4", CPU: 8000, Mem: 16e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
		// 	},
		// 	pods: []PodConfig{
		// 		{Name: "small-1", CPU: 500, Mem: 1e9, Node: 3, RS: "small", MaxUnavail: 2},
		// 		{Name: "small-2", CPU: 500, Mem: 1e9, Node: 3, RS: "small", MaxUnavail: 2},
		// 		{Name: "small-3", CPU: 500, Mem: 1e9, Node: 3, RS: "small", MaxUnavail: 2},
		// 		{Name: "medium-1", CPU: 1000, Mem: 2e9, Node: 3, RS: "medium", MaxUnavail: 1},
		// 		{Name: "medium-2", CPU: 1000, Mem: 2e9, Node: 3, RS: "medium", MaxUnavail: 1},
		// 		{Name: "large-1", CPU: 2000, Mem: 4e9, Node: 3, RS: "large", MaxUnavail: 1},
		// 	},
		// 	weightProfile:    WeightProfile{Cost: 0.45, Disruption: 0.45, Balance: 0.10},
		// 	populationSize:   100,
		// 	maxGenerations:   50,
		// 	expectedBehavior: "Should show clear trade-offs: cheaper nodes vs balance vs disruption",
		// },
		// {
		// 	name: "MixedNodeTypes_CostFocused",
		// 	nodes: []NodeConfig{
		// 		{Name: "node-1", CPU: 2000, Mem: 4e9, Type: "t3.small", Region: "us-east-1", Lifecycle: "spot"}, // Using spot for extra savings
		// 		{Name: "node-2", CPU: 2000, Mem: 4e9, Type: "t3.small", Region: "us-east-1", Lifecycle: "spot"}, // Using spot for extra savings
		// 		{Name: "node-3", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "on-demand"},
		// 		{Name: "node-4", CPU: 8000, Mem: 16e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
		// 	},
		// 	pods: []PodConfig{
		// 		{Name: "small-1", CPU: 500, Mem: 1e9, Node: 3, RS: "small", MaxUnavail: 1},
		// 		{Name: "small-2", CPU: 500, Mem: 1e9, Node: 3, RS: "small", MaxUnavail: 1},
		// 		{Name: "small-3", CPU: 500, Mem: 1e9, Node: 3, RS: "small", MaxUnavail: 1},
		// 		{Name: "medium-1", CPU: 1000, Mem: 2e9, Node: 3, RS: "medium", MaxUnavail: 1},
		// 		{Name: "medium-2", CPU: 1000, Mem: 2e9, Node: 3, RS: "medium", MaxUnavail: 1},
		// 		{Name: "large-1", CPU: 2000, Mem: 4e9, Node: 3, RS: "large", MaxUnavail: 1},
		// 	},
		// 	weightProfile:    WeightProfile{Cost: 0.33, Disruption: 0.34, Balance: 0.33},
		// 	populationSize:   1000,
		// 	maxGenerations:   200,
		// 	expectedBehavior: "Should strongly prefer cheaper nodes despite disruption",
		// },
		// {
		// 	name: "MultiRegion_MixedLifecycle",
		// 	nodes: []NodeConfig{
		// 		{Name: "node-1", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "on-demand"},
		// 		{Name: "node-2", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},      // Same region, spot
		// 		{Name: "node-3", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "eu-west-3", Lifecycle: "on-demand"}, // EU region
		// 		{Name: "node-4", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "ap-northeast-1", Lifecycle: "spot"}, // Asia region, spot
		// 	},
		// 	pods: []PodConfig{
		// 		{Name: "app-0", CPU: 1500, Mem: 3e9, Node: 0, RS: "app", MaxUnavail: 1},
		// 		{Name: "app-1", CPU: 1500, Mem: 3e9, Node: 0, RS: "app", MaxUnavail: 1},
		// 		{Name: "app-2", CPU: 1500, Mem: 3e9, Node: 0, RS: "app", MaxUnavail: 1},
		// 		{Name: "app-3", CPU: 1500, Mem: 3e9, Node: 0, RS: "app", MaxUnavail: 1},
		// 	},
		// 	weightProfile:    WeightProfile{Cost: 0.50, Disruption: 0.30, Balance: 0.20},
		// 	populationSize:   80,
		// 	maxGenerations:   50,
		// 	expectedBehavior: "Should prefer spot instances in cheaper regions while maintaining some balance",
		// },
		{
			name: "LargeCluster_MixedWorkloads",
			nodes: []NodeConfig{
				// Production nodes - stable, on-demand
				{Name: "prod-1", CPU: 8000, Mem: 16e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
				{Name: "prod-2", CPU: 8000, Mem: 16e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
				{Name: "prod-3", CPU: 8000, Mem: 16e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
				{Name: "prod-4", CPU: 8000, Mem: 16e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
				// Development nodes - mix of spot and on-demand
				{Name: "dev-1", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
				{Name: "dev-2", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
				{Name: "dev-3", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "on-demand"},
				// Worker nodes - compute optimized
				{Name: "worker-1", CPU: 16000, Mem: 16e9, Type: "c5.2xlarge", Region: "us-east-1", Lifecycle: "spot"},
				{Name: "worker-2", CPU: 16000, Mem: 16e9, Type: "c5.2xlarge", Region: "us-east-1", Lifecycle: "spot"},
				// Memory optimized nodes
				{Name: "mem-1", CPU: 4000, Mem: 32e9, Type: "r5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
				{Name: "mem-2", CPU: 4000, Mem: 32e9, Type: "r5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
			},
			pods: []PodConfig{
				// Frontend pods (12 replicas)
				{Name: "frontend-1", CPU: 500, Mem: 1e9, Node: 0, RS: "frontend", MaxUnavail: 2},
				{Name: "frontend-2", CPU: 500, Mem: 1e9, Node: 0, RS: "frontend", MaxUnavail: 2},
				{Name: "frontend-3", CPU: 500, Mem: 1e9, Node: 1, RS: "frontend", MaxUnavail: 2},
				{Name: "frontend-4", CPU: 500, Mem: 1e9, Node: 1, RS: "frontend", MaxUnavail: 2},
				{Name: "frontend-5", CPU: 500, Mem: 1e9, Node: 2, RS: "frontend", MaxUnavail: 2},
				{Name: "frontend-6", CPU: 500, Mem: 1e9, Node: 2, RS: "frontend", MaxUnavail: 2},
				{Name: "frontend-7", CPU: 500, Mem: 1e9, Node: 3, RS: "frontend", MaxUnavail: 2},
				{Name: "frontend-8", CPU: 500, Mem: 1e9, Node: 3, RS: "frontend", MaxUnavail: 2},
				{Name: "frontend-9", CPU: 500, Mem: 1e9, Node: 4, RS: "frontend", MaxUnavail: 2},
				{Name: "frontend-10", CPU: 500, Mem: 1e9, Node: 4, RS: "frontend", MaxUnavail: 2},
				{Name: "frontend-11", CPU: 500, Mem: 1e9, Node: 5, RS: "frontend", MaxUnavail: 2},
				{Name: "frontend-12", CPU: 500, Mem: 1e9, Node: 5, RS: "frontend", MaxUnavail: 2},
				// API pods (8 replicas)
				{Name: "api-1", CPU: 1000, Mem: 2e9, Node: 0, RS: "api", MaxUnavail: 1},
				{Name: "api-2", CPU: 1000, Mem: 2e9, Node: 1, RS: "api", MaxUnavail: 1},
				{Name: "api-3", CPU: 1000, Mem: 2e9, Node: 2, RS: "api", MaxUnavail: 1},
				{Name: "api-4", CPU: 1000, Mem: 2e9, Node: 3, RS: "api", MaxUnavail: 1},
				{Name: "api-5", CPU: 1000, Mem: 2e9, Node: 0, RS: "api", MaxUnavail: 1},
				{Name: "api-6", CPU: 1000, Mem: 2e9, Node: 1, RS: "api", MaxUnavail: 1},
				{Name: "api-7", CPU: 1000, Mem: 2e9, Node: 2, RS: "api", MaxUnavail: 1},
				{Name: "api-8", CPU: 1000, Mem: 2e9, Node: 3, RS: "api", MaxUnavail: 1},
				// Cache pods (6 replicas) - need 6GB each, so careful with placement
				{Name: "cache-1", CPU: 1000, Mem: 6e9, Node: 9, RS: "cache", MaxUnavail: 2},  // mem-1 (32GB)
				{Name: "cache-2", CPU: 1000, Mem: 6e9, Node: 10, RS: "cache", MaxUnavail: 2}, // mem-2 (32GB)
				{Name: "cache-3", CPU: 1000, Mem: 6e9, Node: 9, RS: "cache", MaxUnavail: 2},  // mem-1 (32GB)
				{Name: "cache-4", CPU: 1000, Mem: 6e9, Node: 10, RS: "cache", MaxUnavail: 2}, // mem-2 (32GB)
				{Name: "cache-5", CPU: 1000, Mem: 6e9, Node: 0, RS: "cache", MaxUnavail: 2},  // prod-1 (16GB)
				{Name: "cache-6", CPU: 1000, Mem: 6e9, Node: 1, RS: "cache", MaxUnavail: 2},  // prod-2 (16GB)
				// Worker pods (4 replicas)
				{Name: "worker-job-1", CPU: 4000, Mem: 4e9, Node: 7, RS: "worker", MaxUnavail: 3},
				{Name: "worker-job-2", CPU: 4000, Mem: 4e9, Node: 7, RS: "worker", MaxUnavail: 3},
				{Name: "worker-job-3", CPU: 4000, Mem: 4e9, Node: 8, RS: "worker", MaxUnavail: 3},
				{Name: "worker-job-4", CPU: 4000, Mem: 4e9, Node: 8, RS: "worker", MaxUnavail: 3},
				{Name: "test-runner-1", CPU: 1500, Mem: 3e9, Node: 4, RS: "test", MaxUnavail: 2},
				{Name: "test-runner-2", CPU: 1500, Mem: 3e9, Node: 5, RS: "test", MaxUnavail: 2},
			},
			weightProfile:    WeightProfile{Cost: 0.30, Disruption: 0.50, Balance: 0.20},
			populationSize:   400,
			maxGenerations:   1000,
			expectedBehavior: "Should consolidate non-critical workloads to spot nodes while respecting PDBs",
		},
		// {
		// 	name: "RealisticProduction_100Pods",
		// 	nodes: []NodeConfig{
		// 		// General purpose nodes (12x m5.xlarge)
		// 		{Name: "gp-node-1", CPU: 8000, Mem: 16e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
		// 		{Name: "gp-node-2", CPU: 8000, Mem: 16e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
		// 		{Name: "gp-node-3", CPU: 8000, Mem: 16e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
		// 		{Name: "gp-node-4", CPU: 8000, Mem: 16e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
		// 		{Name: "gp-node-5", CPU: 8000, Mem: 16e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "spot"},
		// 		{Name: "gp-node-6", CPU: 8000, Mem: 16e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "spot"},
		// 		{Name: "gp-node-7", CPU: 8000, Mem: 16e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "spot"},
		// 		{Name: "gp-node-8", CPU: 8000, Mem: 16e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "spot"},
		// 		// Smaller nodes for cost optimization (8x m5.large)
		// 		{Name: "sm-node-1", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
		// 		{Name: "sm-node-2", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
		// 		{Name: "sm-node-3", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
		// 		{Name: "sm-node-4", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
		// 	},
		// 	pods:             generateRealisticPods(100),
		// 	weightProfile:    WeightProfile{Cost: 0.50, Disruption: 0.25, Balance: 0.25},
		// 	populationSize:   2000,
		// 	maxGenerations:   400,
		// 	expectedBehavior: "Should find good balance between cost savings and operational stability",
		// },
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runOptimizationTest(t, tc)
		})
	}
}

// Helper types for cleaner test definitions
type NodeConfig struct {
	Name      string
	CPU       float64
	Mem       float64
	Type      string // e.g., "m5.large", "t3.small"
	Region    string // e.g., "us-east-1", "eu-west-1"
	Lifecycle string // "on-demand" or "spot"
}

type PodConfig struct {
	Name       string
	CPU        float64
	Mem        float64
	Node       int
	RS         string
	MaxUnavail int
}

type WeightProfile struct {
	Cost       float64
	Disruption float64
	Balance    float64
}

type Analysis struct {
	Assignment    []int
	Cost          float64
	Disruption    float64
	Balance       float64
	WeightedTotal float64
	Movements     int
	// Raw (unnormalized) values
	RawCost       float64
	RawDisruption float64
	RawBalance    float64
}

// generateRealisticPods generates a realistic distribution of pods for a production cluster
func generateRealisticPods(count int) []PodConfig {
	pods := make([]PodConfig, 0, count)

	// Distribution:
	// - 40% small pods (microservices, sidecars)
	// - 30% medium pods (APIs, web services)
	// - 20% large pods (databases, processing)
	// - 10% extra large pods (data processing, ML)

	currentNode := 0
	nodeCount := 12 // Assuming 12 nodes for distribution

	// Small pods (40%)
	smallCount := int(float64(count) * 0.4)
	for i := 0; i < smallCount; i++ {
		pods = append(pods, PodConfig{
			Name:       fmt.Sprintf("small-pod-%d", i),
			CPU:        250 + float64(i%3)*100,     // 250-450 CPU
			Mem:        0.5e9 + float64(i%3)*0.2e9, // 0.5-0.9 GB
			Node:       currentNode,
			RS:         fmt.Sprintf("small-svc-%d", i/5), // Group by 5
			MaxUnavail: 2,
		})
		currentNode = (currentNode + 1) % nodeCount
	}

	// Medium pods (30%)
	mediumCount := int(float64(count) * 0.3)
	for i := 0; i < mediumCount; i++ {
		pods = append(pods, PodConfig{
			Name:       fmt.Sprintf("medium-pod-%d", i),
			CPU:        1000 + float64(i%4)*250,  // 1000-1750 CPU
			Mem:        2e9 + float64(i%4)*0.5e9, // 2-3.5 GB
			Node:       currentNode,
			RS:         fmt.Sprintf("medium-svc-%d", i/3), // Group by 3
			MaxUnavail: 1,
		})
		currentNode = (currentNode + 1) % nodeCount
	}

	// Large pods (20%)
	largeCount := int(float64(count) * 0.2)
	for i := 0; i < largeCount; i++ {
		pods = append(pods, PodConfig{
			Name:       fmt.Sprintf("large-pod-%d", i),
			CPU:        2000 + float64(i%3)*500, // 2000-3000 CPU
			Mem:        4e9 + float64(i%3)*2e9,  // 4-8 GB
			Node:       currentNode,
			RS:         fmt.Sprintf("large-svc-%d", i/2), // Group by 2
			MaxUnavail: 0,                                // Critical services
		})
		currentNode = (currentNode + 1) % nodeCount
	}

	// Extra large pods (10%)
	xlCount := count - smallCount - mediumCount - largeCount
	for i := 0; i < xlCount; i++ {
		pods = append(pods, PodConfig{
			Name:       fmt.Sprintf("xl-pod-%d", i),
			CPU:        4000 + float64(i%2)*1000, // 4000-5000 CPU
			Mem:        8e9 + float64(i%2)*4e9,   // 8-12 GB
			Node:       currentNode,
			RS:         fmt.Sprintf("xl-svc-%d", i), // Each is unique
			MaxUnavail: 1,
		})
		currentNode = (currentNode + 1) % nodeCount
	}

	return pods
}

func runOptimizationTest(t *testing.T, tc struct {
	name             string
	nodes            []NodeConfig
	pods             []PodConfig
	weightProfile    WeightProfile
	populationSize   int
	maxGenerations   int
	expectedBehavior string
}) {
	t.Logf("\n=== Scenario: %s ===", tc.name)
	t.Logf("Expected behavior: %s", tc.expectedBehavior)
	t.Logf("Weights: Cost=%.2f, Disruption=%.2f, Balance=%.2f",
		tc.weightProfile.Cost, tc.weightProfile.Disruption, tc.weightProfile.Balance)

	// Convert to framework types
	nodes := make([]constraints.NodeInfo, len(tc.nodes))
	costNodes := make([]cost.NodeInfo, len(tc.nodes))
	for i, n := range tc.nodes {
		nodes[i] = constraints.NodeInfo{
			Name:        n.Name,
			CPUCapacity: n.CPU,
			MemCapacity: n.Mem,
		}
		costNodes[i] = cost.NewNodeInfoWithCapacity(n.Name, n.Region, n.Type, n.Lifecycle, n.CPU, n.Mem)
	}

	pods := make([]constraints.PodInfo, len(tc.pods))
	for i, p := range tc.pods {
		pods[i] = constraints.PodInfo{
			Name:                   p.Name,
			CPURequest:             p.CPU,
			MemRequest:             p.Mem,
			CurrentNode:            p.Node,
			ReplicaSetName:         p.RS,
			MaxUnavailableReplicas: p.MaxUnavail,
		}
	}

	// Log node costs
	t.Log("\nNodes:")
	totalMaxCost := 0.0
	for i, n := range tc.nodes {
		t.Logf("  %s: %s %s in %s (CPU: %.0f, Mem: %.1fGB, Cost: $%.4f/hr)",
			n.Name, n.Type, n.Lifecycle, n.Region, n.CPU, n.Mem/1e9, costNodes[i].CostPerHour)
		totalMaxCost += costNodes[i].CostPerHour
	}
	t.Logf("  Total max cost: $%.4f/hr", totalMaxCost)

	// Validate initial allocation
	t.Log("\nValidating initial pod allocation:")
	nodeUsage := make([]struct {
		cpu, mem float64
		name     string
	}, len(tc.nodes))
	for i, n := range tc.nodes {
		nodeUsage[i].name = n.Name
	}

	for _, p := range tc.pods {
		if p.Node >= 0 && p.Node < len(tc.nodes) {
			nodeUsage[p.Node].cpu += p.CPU
			nodeUsage[p.Node].mem += p.Mem
		}
	}

	allValid := true
	for i, usage := range nodeUsage {
		cpuOK := usage.cpu <= tc.nodes[i].CPU
		memOK := usage.mem <= tc.nodes[i].Mem
		status := "✓"
		if !cpuOK || !memOK {
			status = "✗"
			allValid = false
		}
		t.Logf("  %s %s: CPU %.0f/%.0f (%.1f%%), Mem %.1fG/%.1fG (%.1f%%)",
			status, usage.name,
			usage.cpu, tc.nodes[i].CPU, (usage.cpu/tc.nodes[i].CPU)*100,
			usage.mem/1e9, tc.nodes[i].Mem/1e9, (usage.mem/tc.nodes[i].Mem)*100)
		if !cpuOK {
			t.Logf("    ❌ CPU OVERCOMMIT by %.0f", usage.cpu-tc.nodes[i].CPU)
		}
		if !memOK {
			t.Logf("    ❌ MEM OVERCOMMIT by %.1fG", (usage.mem-tc.nodes[i].Mem)/1e9)
		}
	}

	if !allValid {
		t.Fatal("Initial allocation has overcommitted nodes!")
	}

	// Create problem
	problem := createKubernetesProblem(nodes, pods, costNodes, tc.weightProfile)

	// Evaluate current state
	currentSol := problem.CreateSolution()
	currentObj := problem.Evaluate(currentSol)

	// Get detailed breakdown of current state
	currentIntSol := currentSol.(*framework.IntegerSolution)

	// Count active nodes for cost
	activeNodes := make(map[int]bool)
	for _, nodeIdx := range currentIntSol.Variables {
		activeNodes[nodeIdx] = true
	}
	rawCost := 0.0
	for nodeIdx := range activeNodes {
		rawCost += costNodes[nodeIdx].CostPerHour
	}

	// Get distribution for balance
	distribution := getDistribution(currentIntSol.Variables, tc.nodes)

	t.Logf("\nInitial state (current cluster configuration):")
	t.Logf("  Assignment: %v", currentIntSol.Variables)
	t.Logf("  Distribution: %s", distribution)
	t.Logf("  Active nodes: %d (raw cost: $%.4f/hr)", len(activeNodes), rawCost)
	t.Logf("  Normalized objectives: Cost=%.4f, Disruption=%.4f, Balance=%.4f",
		currentObj[0], currentObj[1], currentObj[2])
	t.Logf("  Weighted total: %.4f",
		currentObj[0]*tc.weightProfile.Cost+
			currentObj[1]*tc.weightProfile.Disruption+
			currentObj[2]*tc.weightProfile.Balance)

	// Configure and run NSGA-II
	config := algorithms.NSGA2Config{
		PopulationSize:       tc.populationSize,
		MaxGenerations:       tc.maxGenerations,
		CrossoverProbability: 0.9,
		MutationProbability:  0.3,
		TournamentSize:       3,
		ParallelExecution:    true,
	}

	// Run NSGA-II
	nsga2 := algorithms.NewNSGAII(config, problem)
	population := nsga2.Run()

	// Get Pareto front
	fronts := algorithms.NonDominatedSort(population)
	if len(fronts) == 0 || len(fronts[0]) == 0 {
		t.Fatal("No solutions found in Pareto front")
	}

	paretoFront := fronts[0]
	t.Logf("\nFound %d Pareto-optimal solutions", len(paretoFront))

	// Analyze solutions
	analyses := make([]Analysis, len(paretoFront))
	for i, sol := range paretoFront {
		intSol := sol.Solution.(*framework.IntegerSolution)
		obj := problem.Evaluate(sol.Solution)

		movements := 0
		for j, node := range intSol.Variables {
			if node != pods[j].CurrentNode {
				movements++
			}
		}

		analyses[i] = Analysis{
			Assignment:    intSol.Variables,
			Cost:          obj[0],
			Disruption:    obj[1],
			Balance:       obj[2],
			WeightedTotal: obj[0]*tc.weightProfile.Cost + obj[1]*tc.weightProfile.Disruption + obj[2]*tc.weightProfile.Balance,
			Movements:     movements,
		}
	}

	// Sort by weighted total
	for i := 0; i < len(analyses)-1; i++ {
		for j := i + 1; j < len(analyses); j++ {
			if analyses[i].WeightedTotal > analyses[j].WeightedTotal {
				analyses[i], analyses[j] = analyses[j], analyses[i]
			}
		}
	}

	// Deduplicate solutions
	uniqueAnalyses := []Analysis{}

	// Show all unique solutions for MixedNodeTypes scenarios or top 5 for others
	showAll := strings.Contains(tc.name, "MixedNodeTypes")
	maxToShow := 5
	if showAll {
		maxToShow = len(uniqueAnalyses)
		t.Logf("\nAll %d unique solutions (out of %d total):", len(uniqueAnalyses), len(analyses))
	} else {
		t.Logf("\nTop unique solutions by weighted total (%d unique out of %d total):", len(uniqueAnalyses), len(analyses))
	}

	for i := 0; i < len(uniqueAnalyses) && i < maxToShow; i++ {
		a := uniqueAnalyses[i]
		isInitial := a.Movements == 0
		marker := ""
		if isInitial {
			marker = " [INITIAL STATE]"
		}
		t.Logf("\n%d. Normalized: Cost=%.4f, Disruption=%.4f, Balance=%.4f, Weighted=%.4f%s",
			i+1, a.Cost, a.Disruption, a.Balance, a.WeightedTotal, marker)
		t.Logf("   Movements: %d, Assignment: %v", a.Movements, a.Assignment)
		t.Logf("   Distribution: %s", getDistribution(a.Assignment, tc.nodes))

		// Show feasible movements respecting PDBs for execution planning
		feasibleMoves := calculateFeasibleMovements(a.Assignment, tc.pods, tc.nodes)
		if len(feasibleMoves) > 0 {
			t.Logf("   Execution plan (respecting PDBs): %d pods can move in first iteration", len(feasibleMoves))

			// Group by replica set for clarity
			movesByRS := groupMovesByReplicaSet(feasibleMoves, tc.pods)
			for rs, moves := range movesByRS {
				t.Logf("     - %s: %v", rs, moves)
			}

			// Calculate intermediate state objectives
			intermediateSol := createIntermediateSolution(tc.pods, a.Assignment, feasibleMoves)
			intermediateObj := problem.Evaluate(intermediateSol)
			t.Logf("   Intermediate state objectives: Cost=%.4f, Disruption=%.4f, Balance=%.4f",
				intermediateObj[0], intermediateObj[1], intermediateObj[2])
		}
	}

	// Find and show extremes from unique solutions
	var bestCost, bestBalance, bestDisruption *Analysis
	for i := range uniqueAnalyses {
		if bestCost == nil || uniqueAnalyses[i].Cost < bestCost.Cost {
			bestCost = &uniqueAnalyses[i]
		}
		if bestBalance == nil || uniqueAnalyses[i].Balance < bestBalance.Balance {
			bestBalance = &uniqueAnalyses[i]
		}
		if bestDisruption == nil || uniqueAnalyses[i].Disruption < bestDisruption.Disruption {
			bestDisruption = &uniqueAnalyses[i]
		}
	}

	t.Log("\nExtreme solutions:")
	if bestCost != nil && !isInTop5(bestCost, uniqueAnalyses) {
		t.Logf("\nBest Cost: %s", getDistribution(bestCost.Assignment, tc.nodes))
		t.Logf("  Normalized: Cost=%.4f, Disruption=%.4f, Balance=%.4f",
			bestCost.Cost, bestCost.Disruption, bestCost.Balance)
	}
	if bestBalance != nil && bestBalance != bestCost && !isInTop5(bestBalance, uniqueAnalyses) {
		t.Logf("\nBest Balance: %s", getDistribution(bestBalance.Assignment, tc.nodes))
		t.Logf("  Normalized: Cost=%.4f, Disruption=%.4f, Balance=%.4f",
			bestBalance.Cost, bestBalance.Disruption, bestBalance.Balance)
	}
	if bestDisruption != nil && bestDisruption != bestCost && bestDisruption != bestBalance && !isInTop5(bestDisruption, uniqueAnalyses) {
		t.Logf("\nBest Disruption: %s", getDistribution(bestDisruption.Assignment, tc.nodes))
		t.Logf("  Normalized: Cost=%.4f, Disruption=%.4f, Balance=%.4f",
			bestDisruption.Cost, bestDisruption.Disruption, bestDisruption.Balance)
	}
}

func getDistribution(assignment []int, nodes []NodeConfig) string {
	counts := make([]int, len(nodes))
	for _, node := range assignment {
		counts[node]++
	}

	result := ""
	for i, count := range counts {
		if count > 0 {
			if result != "" {
				result += ", "
			}
			result += fmt.Sprintf("%s:%d", nodes[i].Name, count)
		}
	}
	return result
}

func isInTop5(target *Analysis, analyses []Analysis) bool {
	for i := 0; i < 5 && i < len(analyses); i++ {
		if &analyses[i] == target {
			return true
		}
	}
	return false
}

// calculateFeasibleMovements determines which pods can actually be moved in the first iteration
// while respecting PDB constraints (maxUnavailable)
// This is used for execution planning, not optimization. The optimizer finds the best end state,
// then this function helps determine how to get there incrementally.
func calculateFeasibleMovements(targetAssignment []int, pods []PodConfig, nodes []NodeConfig) []string {
	feasibleMoves := []string{}

	// Group pods by replica set
	replicaSets := make(map[string][]int) // RS name -> pod indices
	for i, pod := range pods {
		if replicaSets[pod.RS] == nil {
			replicaSets[pod.RS] = []int{}
		}
		replicaSets[pod.RS] = append(replicaSets[pod.RS], i)
	}

	// For each replica set, determine how many pods we can move
	for _, podIndices := range replicaSets {
		// Find maxUnavailable for this RS
		maxUnavailable := 1
		if len(podIndices) > 0 {
			maxUnavailable = pods[podIndices[0]].MaxUnavail
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
			feasibleMoves = append(feasibleMoves, pods[podsToMove[i]].Name)
		}
	}

	return feasibleMoves
}

// groupMovesByReplicaSet groups pod movements by their replica set
func groupMovesByReplicaSet(podNames []string, pods []PodConfig) map[string][]string {
	groups := make(map[string][]string)

	for _, podName := range podNames {
		// Find the pod
		for _, pod := range pods {
			if pod.Name == podName {
				if groups[pod.RS] == nil {
					groups[pod.RS] = []string{}
				}
				groups[pod.RS] = append(groups[pod.RS], podName)
				break
			}
		}
	}

	return groups
}

// createIntermediateSolution creates a solution representing the state after feasible moves
func createIntermediateSolution(pods []PodConfig, targetAssignment []int, feasibleMoves []string) framework.Solution {
	// Start with current state
	intermediateAssignment := make([]int, len(pods))
	for i, pod := range pods {
		intermediateAssignment[i] = pod.Node
	}

	// Apply only the feasible moves
	feasibleSet := make(map[string]bool)
	for _, name := range feasibleMoves {
		feasibleSet[name] = true
	}

	for i, pod := range pods {
		if feasibleSet[pod.Name] {
			intermediateAssignment[i] = targetAssignment[i]
		}
	}

	return &framework.IntegerSolution{Variables: intermediateAssignment}
}

// KubernetesProblem implements framework.Problem
type KubernetesProblem struct {
	nodes               []constraints.NodeInfo
	pods                []constraints.PodInfo
	costNodes           []cost.NodeInfo
	costObjective       framework.ObjectiveFunc
	disruptionObjective framework.ObjectiveFunc
	balanceObjective    framework.ObjectiveFunc
	constraint          framework.Constraint
	maxPossibleCost     float64
}

// createKubernetesProblem creates a MOO problem for pod scheduling
// IMPORTANT: PDB constraints are NOT applied during optimization as they are execution-time constraints.
// The descheduler finds the optimal end state, then executes it incrementally respecting PDBs.
func createKubernetesProblem(nodes []constraints.NodeInfo, pods []constraints.PodInfo, costNodes []cost.NodeInfo, weights WeightProfile) *KubernetesProblem {
	// Create objectives
	costObj := cost.CostObjective(pods, costNodes)

	// Convert for disruption objective
	disruptionPods := make([]disruption.PodInfo, len(pods))
	for i, p := range pods {
		disruptionPods[i] = disruption.PodInfo{
			Name:                   p.Name,
			CurrentNode:            p.CurrentNode,
			ColdStartTime:          0.0, // Default 10s cold start
			ReplicaSetName:         p.ReplicaSetName,
			MaxUnavailableReplicas: p.MaxUnavailableReplicas,
		}
	}
	currentState := make([]int, len(pods))
	for i, p := range pods {
		currentState[i] = p.CurrentNode
	}
	disruptionConfig := disruption.NewDisruptionConfig(disruptionPods)
	disruptionObj := disruption.DisruptionObjective(currentState, disruptionPods, disruptionConfig)

	// Convert for balance objective
	balancePods := make([]balance.PodResources, len(pods))
	for i, p := range pods {
		balancePods[i] = balance.PodResources{
			CPURequest: p.CPURequest,
			MemRequest: p.MemRequest,
		}
	}
	balanceNodes := make([]balance.ResourceInfo, len(nodes))
	for i, n := range nodes {
		balanceNodes[i] = balance.ResourceInfo{
			CPUCapacity:  n.CPUCapacity,
			MemCapacity:  n.MemCapacity,
			CPUAllocated: 0,
			MemAllocated: 0,
		}
	}
	balanceConfig := balance.DefaultBalanceConfig()
	balanceObj := balance.BalanceObjectiveFunc(balancePods, balanceNodes, balanceConfig)

	// Create constraints
	// Only use resource constraints for optimization - PDB constraints are for execution time
	resourceConstraint := constraints.ResourceConstraint(pods, nodes)

	// Calculate max possible cost
	maxPossibleCost := 0.0
	for _, node := range costNodes {
		maxPossibleCost += node.CostPerHour
	}

	return &KubernetesProblem{
		nodes:               nodes,
		pods:                pods,
		costNodes:           costNodes,
		costObjective:       costObj,
		disruptionObjective: disruptionObj,
		balanceObjective:    balanceObj,
		constraint:          resourceConstraint,
		maxPossibleCost:     maxPossibleCost,
	}
}

func (kp *KubernetesProblem) Name() string {
	return "KubernetesPodScheduling"
}

func (kp *KubernetesProblem) Objectives() int {
	return 3
}

func (kp *KubernetesProblem) Variables() int {
	return len(kp.pods)
}

func (kp *KubernetesProblem) ObjectiveFuncs() []framework.ObjectiveFunc {
	return []framework.ObjectiveFunc{
		kp.costObjective,
		kp.disruptionObjective,
		kp.balanceObjective,
	}
}

func (kp *KubernetesProblem) Evaluate(solution framework.Solution) []float64 {
	return []float64{
		kp.costObjective(solution),
		kp.disruptionObjective(solution),
		kp.balanceObjective(solution),
	}
}

func (kp *KubernetesProblem) CreateSolution() framework.Solution {
	bounds := make([]framework.IntBounds, len(kp.pods))
	variables := make([]int, len(kp.pods))

	for i, pod := range kp.pods {
		bounds[i] = framework.IntBounds{L: 0, H: len(kp.nodes) - 1}
		variables[i] = pod.CurrentNode
	}

	return &framework.IntegerSolution{
		Variables: variables,
		Bounds:    bounds,
	}
}

func (kp *KubernetesProblem) Constraints() []framework.Constraint {
	return []framework.Constraint{kp.constraint}
}

func (kp *KubernetesProblem) Initialize(size int) []framework.Solution {
	// Use GCSH warm start for better initial population
	// Only pass cost and balance objectives (not disruption which is at index 0)
	objectives := kp.ObjectiveFuncs()
	constructionObjectives := []framework.ObjectiveFunc{
		objectives[0], // cost
		objectives[2], // balance
	}

	gcshConfig := warmstart.GCSHConfig{
		Pods:                kp.pods,
		Nodes:               kp.nodes,
		Objectives:          constructionObjectives,
		Constraints:         []framework.Constraint{kp.constraint},
		IncludeCurrentState: true,
	}

	gcsh := warmstart.NewGCSH(gcshConfig)
	return gcsh.GenerateInitialPopulation(size)
}

func (kp *KubernetesProblem) Bounds() []framework.Bounds {
	bounds := make([]framework.Bounds, len(kp.pods))
	for i := range bounds {
		bounds[i] = framework.Bounds{
			L: 0,
			H: float64(len(kp.nodes) - 1),
		}
	}
	return bounds
}

func (kp *KubernetesProblem) TrueParetoFront(size int) []framework.ObjectiveSpacePoint {
	return nil // Unknown for this problem
}
