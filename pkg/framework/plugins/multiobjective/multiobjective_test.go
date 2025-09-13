package multiobjective_test

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/algorithms"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/constraints"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/objectives/balance"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/objectives/cost"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/objectives/disruption"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/objectives/resourcecost"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/warmstart"
)

// JSON output structures for visualization
type OptimizationOutput struct {
	Timestamp         string              `json:"timestamp"`
	TestCase          TestCaseConfig      `json:"testCase"`
	Algorithm         AlgorithmConfig     `json:"algorithm"`
	Rounds            []OptimizationRound `json:"rounds"`
	BaselineResults   []BaselineResult    `json:"baselineResults"`
	FinalResults      FinalAnalysis       `json:"finalResults"`
	ComparisonMetrics ComparisonMetrics   `json:"comparisonMetrics"`
}

type TestCaseConfig struct {
	Name             string        `json:"name"`
	Nodes            []NodeConfig  `json:"nodes"`
	Pods             []PodConfig   `json:"pods"`
	WeightProfile    WeightProfile `json:"weightProfile"`
	ExpectedBehavior string        `json:"expectedBehavior"`
}

type AlgorithmConfig struct {
	PopulationSize       int     `json:"populationSize"`
	MaxGenerations       int     `json:"maxGenerations"`
	CrossoverProbability float64 `json:"crossoverProbability"`
	MutationProbability  float64 `json:"mutationProbability"`
	TournamentSize       int     `json:"tournamentSize"`
	ParallelExecution    bool    `json:"parallelExecution"`
}

type OptimizationRound struct {
	Round             int                   `json:"round"`
	ParetoFront       []Solution3D          `json:"paretoFront"`
	BestSolution      Solution3D            `json:"bestSolution"`
	InitialState      ClusterState          `json:"initialState"`
	IntermediateState ClusterState          `json:"intermediateState"` // After feasible moves
	FinalState        ClusterState          `json:"finalState"`        // Target state (if all moves were possible)
	Improvements      Improvements          `json:"improvements"`
	MovementAnalysis  MovementAnalysis      `json:"movementAnalysis"`
	FeasibleMoves     FeasibleMovesAnalysis `json:"feasibleMoves"` // What actually happened
}

type Solution3D struct {
	ID            int     `json:"id"`
	Assignment    []int   `json:"assignment"`
	Cost          float64 `json:"cost"`
	Disruption    float64 `json:"disruption"`
	Balance       float64 `json:"balance"`
	WeightedScore float64 `json:"weightedScore"`
	Movements     int     `json:"movements"`
	RawCost       float64 `json:"rawCost"`       // Actual dollar cost
	RawDisruption float64 `json:"rawDisruption"` // Actual disruption units
	RawBalance    float64 `json:"rawBalance"`    // Actual balance percentage
}

type ClusterState struct {
	TotalCost        float64 `json:"totalCost"`
	BalancePercent   float64 `json:"balancePercent"`
	NodeUtilizations []struct {
		NodeName   string  `json:"nodeName"`
		CPUPercent float64 `json:"cpuPercent"`
		MemPercent float64 `json:"memPercent"`
		PodCount   int     `json:"podCount"`
		HourlyCost float64 `json:"hourlyCost"`
	} `json:"nodeUtilizations"`
}

type Improvements struct {
	CostSavings        float64 `json:"costSavings"`
	CostSavingsPercent float64 `json:"costSavingsPercent"`
	BalanceImprovement float64 `json:"balanceImprovement"`
	AnnualSavings      float64 `json:"annualSavings"`
}

type MovementAnalysis struct {
	TotalMoves     int            `json:"totalMoves"`
	MovesByType    map[string]int `json:"movesByType"`
	MovesByRS      map[string]int `json:"movesByRS"`
	CostOptimal    int            `json:"costOptimal"`    // Moves to cheaper nodes
	BalanceOptimal int            `json:"balanceOptimal"` // Moves for better balance
}

type FeasibleMovesAnalysis struct {
	TotalTargetMoves   int              `json:"totalTargetMoves"`   // Total moves in best solution
	FeasibleMoves      int              `json:"feasibleMoves"`      // Moves that respect PDB constraints
	BlockedByPDB       int              `json:"blockedByPDB"`       // Moves blocked by PDB constraints
	FeasibilityPercent float64          `json:"feasibilityPercent"` // % of moves that are feasible
	MovedPods          []PodMovement    `json:"movedPods"`          // Details of each moved pod
	ObjectiveChanges   ObjectiveChanges `json:"objectiveChanges"`   // How objectives change with intermediate state
}

type PodMovement struct {
	PodName    string  `json:"podName"`
	ReplicaSet string  `json:"replicaSet"`
	FromNode   string  `json:"fromNode"`
	ToNode     string  `json:"toNode"`
	MoveType   string  `json:"moveType"`   // "cost-saving", "balance-improving", etc.
	CostImpact float64 `json:"costImpact"` // Cost change from this move
}

type ObjectiveChanges struct {
	InitialObjectives      ObjectiveValues `json:"initialObjectives"`      // Before any moves
	IntermediateObjectives ObjectiveValues `json:"intermediateObjectives"` // After feasible moves only
	TargetObjectives       ObjectiveValues `json:"targetObjectives"`       // If all moves were applied
}

type ObjectiveValues struct {
	Cost          float64 `json:"cost"`          // Normalized cost objective
	Disruption    float64 `json:"disruption"`    // Normalized disruption objective
	Balance       float64 `json:"balance"`       // Normalized balance objective
	RawCost       float64 `json:"rawCost"`       // Actual dollar cost per hour
	RawDisruption float64 `json:"rawDisruption"` // Actual disruption value
	RawBalance    float64 `json:"rawBalance"`    // Actual balance percentage
}

type FinalAnalysis struct {
	TotalRounds       int        `json:"totalRounds"`
	ConvergedAtRound  int        `json:"convergedAtRound"`
	ConvergenceReason string     `json:"convergenceReason"`
	TotalCostSavings  float64    `json:"totalCostSavings"`
	TotalBalanceGain  float64    `json:"totalBalanceGain"`
	FinalParetoSize   int        `json:"finalParetoSize"`
	OptimalSolution   Solution3D `json:"optimalSolution"`
}

type ComparisonMetrics struct {
	NSGAIIBest            Solution3D            `json:"nsgaiiBest"`
	BaselineBest          BaselineResult        `json:"baselineBest"`
	ImprovementRatio      float64               `json:"improvementRatio"`
	CostImprovement       float64               `json:"costImprovement"`
	BalanceImprovement    float64               `json:"balanceImprovement"`
	PerformanceComparison PerformanceComparison `json:"performanceComparison"`
}

type PerformanceComparison struct {
	NSGAIIExecutionTime float64 `json:"nsgaiiExecutionTimeMs"`
	FastestBaseline     string  `json:"fastestBaseline"`
	FastestBaselineTime float64 `json:"fastestBaselineTimeMs"`
	SpeedupRatio        float64 `json:"speedupRatio"`
}

// Helper types for test configuration
// type NodeConfig struct {
// 	Name      string
// 	CPU       float64
// 	Mem       float64
// 	Type      string // e.g., "m5.large", "t3.small"
// 	Region    string // e.g., "us-east-1", "eu-west-1"
// 	Lifecycle string // "on-demand" or "spot"
// }

// type PodConfig struct {
// 	Name       string
// 	CPU        float64
// 	Mem        float64
// 	Node       int
// 	RS         string
// 	MaxUnavail int
// }

// type WeightProfile struct {
// 	Cost       float64
// 	Disruption float64
// 	Balance    float64
// }

// type Analysis struct {
// 	Assignment    []int
// 	Cost          float64
// 	Disruption    float64
// 	Balance       float64
// 	WeightedTotal float64
// 	Movements     int
// 	// Raw (unnormalized) values
// 	RawCost       float64
// 	RawDisruption float64
// 	RawBalance    float64
// }

var testCasePods = []PodConfig{
	// App-A workloads: ALL on BAD nodes initially (24 pods)
	{Name: "app-a-1", CPU: 1000, Mem: 2e9, Node: 0, RS: "app-a", MaxUnavail: 4}, {Name: "app-a-2", CPU: 1000, Mem: 2e9, Node: 0, RS: "app-a", MaxUnavail: 4},
	{Name: "app-a-3", CPU: 1000, Mem: 2e9, Node: 1, RS: "app-a", MaxUnavail: 4}, {Name: "app-a-4", CPU: 1000, Mem: 2e9, Node: 1, RS: "app-a", MaxUnavail: 4},
	{Name: "app-a-5", CPU: 1000, Mem: 2e9, Node: 2, RS: "app-a", MaxUnavail: 4}, {Name: "app-a-6", CPU: 1000, Mem: 2e9, Node: 2, RS: "app-a", MaxUnavail: 4},
	{Name: "app-a-7", CPU: 1000, Mem: 2e9, Node: 3, RS: "app-a", MaxUnavail: 4}, {Name: "app-a-8", CPU: 1000, Mem: 2e9, Node: 3, RS: "app-a", MaxUnavail: 4},
	{Name: "app-a-9", CPU: 1000, Mem: 2e9, Node: 4, RS: "app-a", MaxUnavail: 4}, {Name: "app-a-10", CPU: 1000, Mem: 2e9, Node: 4, RS: "app-a", MaxUnavail: 4},
	{Name: "app-a-11", CPU: 1000, Mem: 2e9, Node: 5, RS: "app-a", MaxUnavail: 4}, {Name: "app-a-12", CPU: 1000, Mem: 2e9, Node: 5, RS: "app-a", MaxUnavail: 4},
	{Name: "app-a-13", CPU: 1000, Mem: 2e9, Node: 6, RS: "app-a", MaxUnavail: 4}, {Name: "app-a-14", CPU: 1000, Mem: 2e9, Node: 6, RS: "app-a", MaxUnavail: 4},
	{Name: "app-a-15", CPU: 1000, Mem: 2e9, Node: 7, RS: "app-a", MaxUnavail: 4}, {Name: "app-a-16", CPU: 1000, Mem: 2e9, Node: 7, RS: "app-a", MaxUnavail: 4},
	{Name: "app-a-17", CPU: 1000, Mem: 2e9, Node: 8, RS: "app-a", MaxUnavail: 4}, {Name: "app-a-18", CPU: 1000, Mem: 2e9, Node: 8, RS: "app-a", MaxUnavail: 4},
	{Name: "app-a-19", CPU: 1000, Mem: 2e9, Node: 9, RS: "app-a", MaxUnavail: 4}, {Name: "app-a-20", CPU: 1000, Mem: 2e9, Node: 9, RS: "app-a", MaxUnavail: 4},
	{Name: "app-a-21", CPU: 1000, Mem: 2e9, Node: 10, RS: "app-a", MaxUnavail: 4}, {Name: "app-a-22", CPU: 1000, Mem: 2e9, Node: 10, RS: "app-a", MaxUnavail: 4},
	{Name: "app-a-23", CPU: 1000, Mem: 2e9, Node: 11, RS: "app-a", MaxUnavail: 4}, {Name: "app-a-24", CPU: 1000, Mem: 2e9, Node: 11, RS: "app-a", MaxUnavail: 4},
	// Web-A workloads: ALL on BAD nodes initially (24 pods)
	{Name: "web-a-1", CPU: 500, Mem: 1e9, Node: 0, RS: "web-a", MaxUnavail: 3}, {Name: "web-a-2", CPU: 500, Mem: 1e9, Node: 0, RS: "web-a", MaxUnavail: 3},
	{Name: "web-a-3", CPU: 500, Mem: 1e9, Node: 1, RS: "web-a", MaxUnavail: 3}, {Name: "web-a-4", CPU: 500, Mem: 1e9, Node: 1, RS: "web-a", MaxUnavail: 3},
	{Name: "web-a-5", CPU: 500, Mem: 1e9, Node: 2, RS: "web-a", MaxUnavail: 3}, {Name: "web-a-6", CPU: 500, Mem: 1e9, Node: 2, RS: "web-a", MaxUnavail: 3},
	{Name: "web-a-7", CPU: 500, Mem: 1e9, Node: 3, RS: "web-a", MaxUnavail: 3}, {Name: "web-a-8", CPU: 500, Mem: 1e9, Node: 3, RS: "web-a", MaxUnavail: 3},
	{Name: "web-a-9", CPU: 500, Mem: 1e9, Node: 4, RS: "web-a", MaxUnavail: 3}, {Name: "web-a-10", CPU: 500, Mem: 1e9, Node: 4, RS: "web-a", MaxUnavail: 3},
	{Name: "web-a-11", CPU: 500, Mem: 1e9, Node: 5, RS: "web-a", MaxUnavail: 3}, {Name: "web-a-12", CPU: 500, Mem: 1e9, Node: 5, RS: "web-a", MaxUnavail: 3},
	{Name: "web-a-13", CPU: 500, Mem: 1e9, Node: 6, RS: "web-a", MaxUnavail: 3}, {Name: "web-a-14", CPU: 500, Mem: 1e9, Node: 6, RS: "web-a", MaxUnavail: 3},
	{Name: "web-a-15", CPU: 500, Mem: 1e9, Node: 7, RS: "web-a", MaxUnavail: 3}, {Name: "web-a-16", CPU: 500, Mem: 1e9, Node: 7, RS: "web-a", MaxUnavail: 3},
	{Name: "web-a-17", CPU: 500, Mem: 1e9, Node: 8, RS: "web-a", MaxUnavail: 3}, {Name: "web-a-18", CPU: 500, Mem: 1e9, Node: 8, RS: "web-a", MaxUnavail: 3},
	{Name: "web-a-19", CPU: 500, Mem: 1e9, Node: 9, RS: "web-a", MaxUnavail: 3}, {Name: "web-a-20", CPU: 500, Mem: 1e9, Node: 9, RS: "web-a", MaxUnavail: 3},
	{Name: "web-a-21", CPU: 500, Mem: 1e9, Node: 10, RS: "web-a", MaxUnavail: 3}, {Name: "web-a-22", CPU: 500, Mem: 1e9, Node: 10, RS: "web-a", MaxUnavail: 3},
	{Name: "web-a-23", CPU: 500, Mem: 1e9, Node: 11, RS: "web-a", MaxUnavail: 3}, {Name: "web-a-24", CPU: 500, Mem: 1e9, Node: 11, RS: "web-a", MaxUnavail: 3},
	// GOOD nodes start EMPTY (0 pods) - algorithm should discover they're better
}

var testCaseNodes = []NodeConfig{
	// BAD pool - expensive on-demand instances (12 nodes)
	{Name: "bad-1", CPU: 4000, Mem: 8e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
	{Name: "bad-2", CPU: 4000, Mem: 8e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
	{Name: "bad-3", CPU: 4000, Mem: 8e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
	{Name: "bad-4", CPU: 4000, Mem: 8e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
	{Name: "bad-5", CPU: 4000, Mem: 8e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
	{Name: "bad-6", CPU: 4000, Mem: 8e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
	{Name: "bad-7", CPU: 4000, Mem: 8e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
	{Name: "bad-8", CPU: 4000, Mem: 8e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
	{Name: "bad-9", CPU: 4000, Mem: 8e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
	{Name: "bad-10", CPU: 4000, Mem: 8e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
	{Name: "bad-11", CPU: 4000, Mem: 8e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
	{Name: "bad-12", CPU: 4000, Mem: 8e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
	// GOOD pool - cheap spot instances (12 nodes)
	{Name: "good-1", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "good-2", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "good-3", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "good-4", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "good-5", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "good-6", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "good-7", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "good-8", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "good-9", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "good-10", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "good-11", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "good-12", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
}

var massivePods = []PodConfig{
	// LARGE APPS: Initially on expensive prod nodes (32 pods, 2 CPU each)
	{Name: "large-app-1", CPU: 2000, Mem: 4e9, Node: 0, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-2", CPU: 2000, Mem: 4e9, Node: 0, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-3", CPU: 2000, Mem: 4e9, Node: 0, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-4", CPU: 2000, Mem: 4e9, Node: 0, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-5", CPU: 2000, Mem: 4e9, Node: 1, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-6", CPU: 2000, Mem: 4e9, Node: 1, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-7", CPU: 2000, Mem: 4e9, Node: 1, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-8", CPU: 2000, Mem: 4e9, Node: 1, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-9", CPU: 2000, Mem: 4e9, Node: 2, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-10", CPU: 2000, Mem: 4e9, Node: 2, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-11", CPU: 2000, Mem: 4e9, Node: 2, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-12", CPU: 2000, Mem: 4e9, Node: 2, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-13", CPU: 2000, Mem: 4e9, Node: 3, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-14", CPU: 2000, Mem: 4e9, Node: 3, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-15", CPU: 2000, Mem: 4e9, Node: 3, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-16", CPU: 2000, Mem: 4e9, Node: 3, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-17", CPU: 2000, Mem: 4e9, Node: 4, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-18", CPU: 2000, Mem: 4e9, Node: 4, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-19", CPU: 2000, Mem: 4e9, Node: 4, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-20", CPU: 2000, Mem: 4e9, Node: 4, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-21", CPU: 2000, Mem: 4e9, Node: 5, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-22", CPU: 2000, Mem: 4e9, Node: 5, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-23", CPU: 2000, Mem: 4e9, Node: 5, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-24", CPU: 2000, Mem: 4e9, Node: 5, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-25", CPU: 2000, Mem: 4e9, Node: 6, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-26", CPU: 2000, Mem: 4e9, Node: 6, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-27", CPU: 2000, Mem: 4e9, Node: 6, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-28", CPU: 2000, Mem: 4e9, Node: 6, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-29", CPU: 2000, Mem: 4e9, Node: 7, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-30", CPU: 2000, Mem: 4e9, Node: 7, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-31", CPU: 2000, Mem: 4e9, Node: 7, RS: "large-app", MaxUnavail: 6},
	{Name: "large-app-32", CPU: 2000, Mem: 4e9, Node: 7, RS: "large-app", MaxUnavail: 6},
	// MEDIUM APPS: Initially on mixed tier nodes (40 pods, 1 CPU each)
	{Name: "api-1", CPU: 1000, Mem: 2e9, Node: 8, RS: "api-service", MaxUnavail: 8},
	{Name: "api-2", CPU: 1000, Mem: 2e9, Node: 8, RS: "api-service", MaxUnavail: 8},
	{Name: "api-3", CPU: 1000, Mem: 2e9, Node: 8, RS: "api-service", MaxUnavail: 8},
	{Name: "api-4", CPU: 1000, Mem: 2e9, Node: 8, RS: "api-service", MaxUnavail: 8},
	{Name: "api-5", CPU: 1000, Mem: 2e9, Node: 8, RS: "api-service", MaxUnavail: 8},
	{Name: "api-6", CPU: 1000, Mem: 2e9, Node: 9, RS: "api-service", MaxUnavail: 8},
	{Name: "api-7", CPU: 1000, Mem: 2e9, Node: 9, RS: "api-service", MaxUnavail: 8},
	{Name: "api-8", CPU: 1000, Mem: 2e9, Node: 9, RS: "api-service", MaxUnavail: 8},
	{Name: "api-9", CPU: 1000, Mem: 2e9, Node: 9, RS: "api-service", MaxUnavail: 8},
	{Name: "api-10", CPU: 1000, Mem: 2e9, Node: 9, RS: "api-service", MaxUnavail: 8},
	{Name: "api-11", CPU: 1000, Mem: 2e9, Node: 10, RS: "api-service", MaxUnavail: 8},
	{Name: "api-12", CPU: 1000, Mem: 2e9, Node: 10, RS: "api-service", MaxUnavail: 8},
	{Name: "api-13", CPU: 1000, Mem: 2e9, Node: 10, RS: "api-service", MaxUnavail: 8},
	{Name: "api-14", CPU: 1000, Mem: 2e9, Node: 10, RS: "api-service", MaxUnavail: 8},
	{Name: "api-15", CPU: 1000, Mem: 2e9, Node: 10, RS: "api-service", MaxUnavail: 8},
	{Name: "api-16", CPU: 1000, Mem: 2e9, Node: 11, RS: "api-service", MaxUnavail: 8},
	{Name: "api-17", CPU: 1000, Mem: 2e9, Node: 11, RS: "api-service", MaxUnavail: 8},
	{Name: "api-18", CPU: 1000, Mem: 2e9, Node: 11, RS: "api-service", MaxUnavail: 8},
	{Name: "api-19", CPU: 1000, Mem: 2e9, Node: 11, RS: "api-service", MaxUnavail: 8},
	{Name: "api-20", CPU: 1000, Mem: 2e9, Node: 11, RS: "api-service", MaxUnavail: 8},
	{Name: "api-21", CPU: 1000, Mem: 2e9, Node: 12, RS: "api-service", MaxUnavail: 8},
	{Name: "api-22", CPU: 1000, Mem: 2e9, Node: 12, RS: "api-service", MaxUnavail: 8},
	{Name: "api-23", CPU: 1000, Mem: 2e9, Node: 12, RS: "api-service", MaxUnavail: 8},
	{Name: "api-24", CPU: 1000, Mem: 2e9, Node: 12, RS: "api-service", MaxUnavail: 8},
	{Name: "api-25", CPU: 1000, Mem: 2e9, Node: 12, RS: "api-service", MaxUnavail: 8},
	{Name: "api-26", CPU: 1000, Mem: 2e9, Node: 13, RS: "api-service", MaxUnavail: 8},
	{Name: "api-27", CPU: 1000, Mem: 2e9, Node: 13, RS: "api-service", MaxUnavail: 8},
	{Name: "api-28", CPU: 1000, Mem: 2e9, Node: 13, RS: "api-service", MaxUnavail: 8},
	{Name: "api-29", CPU: 1000, Mem: 2e9, Node: 13, RS: "api-service", MaxUnavail: 8},
	{Name: "api-30", CPU: 1000, Mem: 2e9, Node: 13, RS: "api-service", MaxUnavail: 8},
	{Name: "api-31", CPU: 1000, Mem: 2e9, Node: 14, RS: "api-service", MaxUnavail: 8},
	{Name: "api-32", CPU: 1000, Mem: 2e9, Node: 14, RS: "api-service", MaxUnavail: 8},
	{Name: "api-33", CPU: 1000, Mem: 2e9, Node: 14, RS: "api-service", MaxUnavail: 8},
	{Name: "api-34", CPU: 1000, Mem: 2e9, Node: 14, RS: "api-service", MaxUnavail: 8},
	{Name: "api-35", CPU: 1000, Mem: 2e9, Node: 14, RS: "api-service", MaxUnavail: 8},
	{Name: "api-36", CPU: 1000, Mem: 2e9, Node: 15, RS: "api-service", MaxUnavail: 8},
	{Name: "api-37", CPU: 1000, Mem: 2e9, Node: 15, RS: "api-service", MaxUnavail: 8},
	{Name: "api-38", CPU: 1000, Mem: 2e9, Node: 15, RS: "api-service", MaxUnavail: 8},
	{Name: "api-39", CPU: 1000, Mem: 2e9, Node: 15, RS: "api-service", MaxUnavail: 8},
	{Name: "api-40", CPU: 1000, Mem: 2e9, Node: 15, RS: "api-service", MaxUnavail: 8},
	// SMALL SERVICES: Initially on medium tier nodes (60 pods, 0.5 CPU each)
	{Name: "web-1", CPU: 500, Mem: 1e9, Node: 16, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-2", CPU: 500, Mem: 1e9, Node: 16, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-3", CPU: 500, Mem: 1e9, Node: 16, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-4", CPU: 500, Mem: 1e9, Node: 17, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-5", CPU: 500, Mem: 1e9, Node: 17, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-6", CPU: 500, Mem: 1e9, Node: 17, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-7", CPU: 500, Mem: 1e9, Node: 18, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-8", CPU: 500, Mem: 1e9, Node: 18, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-9", CPU: 500, Mem: 1e9, Node: 18, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-10", CPU: 500, Mem: 1e9, Node: 19, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-11", CPU: 500, Mem: 1e9, Node: 19, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-12", CPU: 500, Mem: 1e9, Node: 19, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-13", CPU: 500, Mem: 1e9, Node: 20, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-14", CPU: 500, Mem: 1e9, Node: 20, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-15", CPU: 500, Mem: 1e9, Node: 20, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-16", CPU: 500, Mem: 1e9, Node: 21, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-17", CPU: 500, Mem: 1e9, Node: 21, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-18", CPU: 500, Mem: 1e9, Node: 21, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-19", CPU: 500, Mem: 1e9, Node: 22, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-20", CPU: 500, Mem: 1e9, Node: 22, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-21", CPU: 500, Mem: 1e9, Node: 22, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-22", CPU: 500, Mem: 1e9, Node: 23, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-23", CPU: 500, Mem: 1e9, Node: 23, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-24", CPU: 500, Mem: 1e9, Node: 23, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-25", CPU: 500, Mem: 1e9, Node: 24, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-26", CPU: 500, Mem: 1e9, Node: 24, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-27", CPU: 500, Mem: 1e9, Node: 24, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-28", CPU: 500, Mem: 1e9, Node: 25, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-29", CPU: 500, Mem: 1e9, Node: 25, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-30", CPU: 500, Mem: 1e9, Node: 25, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-31", CPU: 500, Mem: 1e9, Node: 26, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-32", CPU: 500, Mem: 1e9, Node: 26, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-33", CPU: 500, Mem: 1e9, Node: 26, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-34", CPU: 500, Mem: 1e9, Node: 27, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-35", CPU: 500, Mem: 1e9, Node: 27, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-36", CPU: 500, Mem: 1e9, Node: 27, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-37", CPU: 500, Mem: 1e9, Node: 16, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-38", CPU: 500, Mem: 1e9, Node: 17, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-39", CPU: 500, Mem: 1e9, Node: 18, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-40", CPU: 500, Mem: 1e9, Node: 19, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-41", CPU: 500, Mem: 1e9, Node: 20, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-42", CPU: 500, Mem: 1e9, Node: 21, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-43", CPU: 500, Mem: 1e9, Node: 22, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-44", CPU: 500, Mem: 1e9, Node: 23, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-45", CPU: 500, Mem: 1e9, Node: 24, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-46", CPU: 500, Mem: 1e9, Node: 25, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-47", CPU: 500, Mem: 1e9, Node: 26, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-48", CPU: 500, Mem: 1e9, Node: 27, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-49", CPU: 500, Mem: 1e9, Node: 16, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-50", CPU: 500, Mem: 1e9, Node: 17, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-51", CPU: 500, Mem: 1e9, Node: 18, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-52", CPU: 500, Mem: 1e9, Node: 19, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-53", CPU: 500, Mem: 1e9, Node: 20, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-54", CPU: 500, Mem: 1e9, Node: 21, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-55", CPU: 500, Mem: 1e9, Node: 22, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-56", CPU: 500, Mem: 1e9, Node: 23, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-57", CPU: 500, Mem: 1e9, Node: 24, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-58", CPU: 500, Mem: 1e9, Node: 25, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-59", CPU: 500, Mem: 1e9, Node: 26, RS: "web-frontend", MaxUnavail: 12},
	{Name: "web-60", CPU: 500, Mem: 1e9, Node: 27, RS: "web-frontend", MaxUnavail: 12},
	// MICROSERVICES: Initially scattered on medium tier (48 pods, 0.25 CPU each)
	{Name: "micro-1", CPU: 250, Mem: 512e6, Node: 16, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-2", CPU: 250, Mem: 512e6, Node: 16, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-3", CPU: 250, Mem: 512e6, Node: 17, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-4", CPU: 250, Mem: 512e6, Node: 17, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-5", CPU: 250, Mem: 512e6, Node: 18, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-6", CPU: 250, Mem: 512e6, Node: 18, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-7", CPU: 250, Mem: 512e6, Node: 19, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-8", CPU: 250, Mem: 512e6, Node: 19, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-9", CPU: 250, Mem: 512e6, Node: 20, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-10", CPU: 250, Mem: 512e6, Node: 20, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-11", CPU: 250, Mem: 512e6, Node: 21, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-12", CPU: 250, Mem: 512e6, Node: 21, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-13", CPU: 250, Mem: 512e6, Node: 22, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-14", CPU: 250, Mem: 512e6, Node: 22, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-15", CPU: 250, Mem: 512e6, Node: 23, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-16", CPU: 250, Mem: 512e6, Node: 23, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-17", CPU: 250, Mem: 512e6, Node: 24, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-18", CPU: 250, Mem: 512e6, Node: 24, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-19", CPU: 250, Mem: 512e6, Node: 25, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-20", CPU: 250, Mem: 512e6, Node: 25, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-21", CPU: 250, Mem: 512e6, Node: 26, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-22", CPU: 250, Mem: 512e6, Node: 26, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-23", CPU: 250, Mem: 512e6, Node: 27, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-24", CPU: 250, Mem: 512e6, Node: 27, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-25", CPU: 250, Mem: 512e6, Node: 16, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-26", CPU: 250, Mem: 512e6, Node: 17, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-27", CPU: 250, Mem: 512e6, Node: 18, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-28", CPU: 250, Mem: 512e6, Node: 19, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-29", CPU: 250, Mem: 512e6, Node: 20, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-30", CPU: 250, Mem: 512e6, Node: 21, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-31", CPU: 250, Mem: 512e6, Node: 22, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-32", CPU: 250, Mem: 512e6, Node: 23, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-33", CPU: 250, Mem: 512e6, Node: 24, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-34", CPU: 250, Mem: 512e6, Node: 25, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-35", CPU: 250, Mem: 512e6, Node: 26, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-36", CPU: 250, Mem: 512e6, Node: 27, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-37", CPU: 250, Mem: 512e6, Node: 16, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-38", CPU: 250, Mem: 512e6, Node: 17, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-39", CPU: 250, Mem: 512e6, Node: 18, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-40", CPU: 250, Mem: 512e6, Node: 19, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-41", CPU: 250, Mem: 512e6, Node: 20, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-42", CPU: 250, Mem: 512e6, Node: 21, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-43", CPU: 250, Mem: 512e6, Node: 22, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-44", CPU: 250, Mem: 512e6, Node: 23, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-45", CPU: 250, Mem: 512e6, Node: 24, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-46", CPU: 250, Mem: 512e6, Node: 25, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-47", CPU: 250, Mem: 512e6, Node: 26, RS: "microservice", MaxUnavail: 12},
	{Name: "micro-48", CPU: 250, Mem: 512e6, Node: 27, RS: "microservice", MaxUnavail: 12},
	// Compute and cheap nodes start EMPTY - algorithm should discover cost optimization opportunities
}

var massiveNodes = []NodeConfig{
	// TIER 1: Very expensive production nodes (8 nodes) - m5.4xlarge on-demand
	{Name: "prod-1", CPU: 16000, Mem: 64e9, Type: "m5.4xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
	{Name: "prod-2", CPU: 16000, Mem: 64e9, Type: "m5.4xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
	{Name: "prod-3", CPU: 16000, Mem: 64e9, Type: "m5.4xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
	{Name: "prod-4", CPU: 16000, Mem: 64e9, Type: "m5.4xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
	{Name: "prod-5", CPU: 16000, Mem: 64e9, Type: "m5.4xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
	{Name: "prod-6", CPU: 16000, Mem: 64e9, Type: "m5.4xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
	{Name: "prod-7", CPU: 16000, Mem: 64e9, Type: "m5.4xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
	{Name: "prod-8", CPU: 16000, Mem: 64e9, Type: "m5.4xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
	// TIER 2: Expensive mixed nodes (8 nodes) - m5.2xlarge mixed lifecycle
	{Name: "mixed-1", CPU: 8000, Mem: 32e9, Type: "m5.2xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
	{Name: "mixed-2", CPU: 8000, Mem: 32e9, Type: "m5.2xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
	{Name: "mixed-3", CPU: 8000, Mem: 32e9, Type: "m5.2xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
	{Name: "mixed-4", CPU: 8000, Mem: 32e9, Type: "m5.2xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
	{Name: "mixed-5", CPU: 8000, Mem: 32e9, Type: "m5.2xlarge", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "mixed-6", CPU: 8000, Mem: 32e9, Type: "m5.2xlarge", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "mixed-7", CPU: 8000, Mem: 32e9, Type: "m5.2xlarge", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "mixed-8", CPU: 8000, Mem: 32e9, Type: "m5.2xlarge", Region: "us-east-1", Lifecycle: "spot"},
	// TIER 3: Medium cost nodes (12 nodes) - m5.large mixed lifecycle
	{Name: "medium-1", CPU: 4000, Mem: 16e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "on-demand"},
	{Name: "medium-2", CPU: 4000, Mem: 16e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "on-demand"},
	{Name: "medium-3", CPU: 4000, Mem: 16e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "on-demand"},
	{Name: "medium-4", CPU: 4000, Mem: 16e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "on-demand"},
	{Name: "medium-5", CPU: 4000, Mem: 16e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "medium-6", CPU: 4000, Mem: 16e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "medium-7", CPU: 4000, Mem: 16e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "medium-8", CPU: 4000, Mem: 16e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "medium-9", CPU: 4000, Mem: 16e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "medium-10", CPU: 4000, Mem: 16e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "medium-11", CPU: 4000, Mem: 16e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "medium-12", CPU: 4000, Mem: 16e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
	// TIER 4: Cheap compute nodes (16 nodes) - c5.xlarge spot
	{Name: "compute-1", CPU: 4000, Mem: 8e9, Type: "c5.xlarge", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "compute-2", CPU: 4000, Mem: 8e9, Type: "c5.xlarge", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "compute-3", CPU: 4000, Mem: 8e9, Type: "c5.xlarge", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "compute-4", CPU: 4000, Mem: 8e9, Type: "c5.xlarge", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "compute-5", CPU: 4000, Mem: 8e9, Type: "c5.xlarge", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "compute-6", CPU: 4000, Mem: 8e9, Type: "c5.xlarge", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "compute-7", CPU: 4000, Mem: 8e9, Type: "c5.xlarge", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "compute-8", CPU: 4000, Mem: 8e9, Type: "c5.xlarge", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "compute-9", CPU: 4000, Mem: 8e9, Type: "c5.xlarge", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "compute-10", CPU: 4000, Mem: 8e9, Type: "c5.xlarge", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "compute-11", CPU: 4000, Mem: 8e9, Type: "c5.xlarge", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "compute-12", CPU: 4000, Mem: 8e9, Type: "c5.xlarge", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "compute-13", CPU: 4000, Mem: 8e9, Type: "c5.xlarge", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "compute-14", CPU: 4000, Mem: 8e9, Type: "c5.xlarge", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "compute-15", CPU: 4000, Mem: 8e9, Type: "c5.xlarge", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "compute-16", CPU: 4000, Mem: 8e9, Type: "c5.xlarge", Region: "us-east-1", Lifecycle: "spot"},
	// TIER 5: Very cheap small nodes (16 nodes) - t3.large spot
	{Name: "cheap-1", CPU: 2000, Mem: 8e9, Type: "t3.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "cheap-2", CPU: 2000, Mem: 8e9, Type: "t3.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "cheap-3", CPU: 2000, Mem: 8e9, Type: "t3.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "cheap-4", CPU: 2000, Mem: 8e9, Type: "t3.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "cheap-5", CPU: 2000, Mem: 8e9, Type: "t3.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "cheap-6", CPU: 2000, Mem: 8e9, Type: "t3.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "cheap-7", CPU: 2000, Mem: 8e9, Type: "t3.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "cheap-8", CPU: 2000, Mem: 8e9, Type: "t3.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "cheap-9", CPU: 2000, Mem: 8e9, Type: "t3.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "cheap-10", CPU: 2000, Mem: 8e9, Type: "t3.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "cheap-11", CPU: 2000, Mem: 8e9, Type: "t3.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "cheap-12", CPU: 2000, Mem: 8e9, Type: "t3.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "cheap-13", CPU: 2000, Mem: 8e9, Type: "t3.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "cheap-14", CPU: 2000, Mem: 8e9, Type: "t3.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "cheap-15", CPU: 2000, Mem: 8e9, Type: "t3.large", Region: "us-east-1", Lifecycle: "spot"},
	{Name: "cheap-16", CPU: 2000, Mem: 8e9, Type: "t3.large", Region: "us-east-1", Lifecycle: "spot"},
}

var mixedNodes = []NodeConfig{
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
}

var mixedPods = []PodConfig{
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
}

// TestMultiObjectiveOptimization runs sequential optimization scenarios with effective cost objective
func TestMultiObjectiveOptimization(t *testing.T) {
	testCases := []struct {
		name             string
		nodes            []NodeConfig
		pods             []PodConfig
		weightProfile    WeightProfile
		populationSize   int
		maxGenerations   int
		expectedBehavior string
		useGCSH          bool // Toggle: true = GCSH warm start, false = random initialization
	}{
		// {
		// 	name:             "LargeCluster_MixedWorkloads_Cost_Focused",
		// 	nodes:            mixedNodes,
		// 	pods:             mixedPods,
		// 	weightProfile:    WeightProfile{Cost: 0.80, Disruption: 0.10, Balance: 0.10},
		// 	populationSize:   200,
		// 	maxGenerations:   200,
		// 	expectedBehavior: "Cost-focused optimization - should aggressively consolidate workloads to cheapest spot nodes",
		// 	// useGCSH:          true,
		// },
		// {
		// 	name:             "LargeCluster_MixedWorkloads_Cost_Balance_Mixed",
		// 	nodes:            mixedNodes,
		// 	pods:             mixedPods,
		// 	weightProfile:    WeightProfile{Cost: 0.45, Disruption: 0.10, Balance: 0.45},
		// 	populationSize:   200,
		// 	maxGenerations:   200,
		// 	expectedBehavior: "Cost-balance optimization - should balance cost savings with load distribution across nodes",
		// 	// useGCSH:          true,
		// },
		// {
		// 	name:             "LargeCluster_MixedWorkloads_Balance_Focused",
		// 	nodes:            mixedNodes,
		// 	pods:             mixedPods,
		// 	weightProfile:    WeightProfile{Cost: 0.10, Disruption: 0.10, Balance: 0.80},
		// 	populationSize:   200,
		// 	maxGenerations:   200,
		// 	expectedBehavior: "Balance-focused optimization - should prioritize even distribution across all nodes",
		// 	// useGCSH:          true,
		// },
		// {
		// 	name:             "LargeCluster_MixedWorkloads_Cost_Disruption_Balance",
		// 	nodes:            mixedNodes,
		// 	pods:             mixedPods,
		// 	weightProfile:    WeightProfile{Cost: 0.40, Disruption: 0.20, Balance: 0.40},
		// 	populationSize:   200,
		// 	maxGenerations:   200,
		// 	expectedBehavior: "Balanced optimization with moderate disruption tolerance",
		// 	// useGCSH:          true,
		// },
		// {
		// 	name:             "LargeCluster_MixedWorkloads_Equal_Weights",
		// 	nodes:            mixedNodes,
		// 	pods:             mixedPods,
		// 	weightProfile:    WeightProfile{Cost: 0.33, Disruption: 0.33, Balance: 0.34},
		// 	populationSize:   200,
		// 	maxGenerations:   200,
		// 	expectedBehavior: "Equal-weight optimization - should find balanced trade-offs across all objectives",
		// 	// useGCSH:          true,
		// },
		// {
		// 	name:             "LargeCluster_MixedWorkloads_Disruption_Moderate",
		// 	nodes:            mixedNodes,
		// 	pods:             mixedPods,
		// 	weightProfile:    WeightProfile{Cost: 0.30, Disruption: 0.40, Balance: 0.30},
		// 	populationSize:   200,
		// 	maxGenerations:   200,
		// 	expectedBehavior: "Moderate disruption focus - should be more conservative with pod movements",
		// 	// useGCSH:          true,
		// },
		// {
		// 	name:             "LargeCluster_MixedWorkloads_Disruption_High",
		// 	nodes:            mixedNodes,
		// 	pods:             mixedPods,
		// 	weightProfile:    WeightProfile{Cost: 0.20, Disruption: 0.60, Balance: 0.20},
		// 	populationSize:   200,
		// 	maxGenerations:   200,
		// 	expectedBehavior: "High disruption focus - should minimize pod movements, make small incremental improvements",
		// 	// useGCSH:          true,
		// },
		// {
		// 	name:             "LargeCluster_MixedWorkloads_Disruption_Extreme",
		// 	nodes:            mixedNodes,
		// 	pods:             mixedPods,
		// 	weightProfile:    WeightProfile{Cost: 0.15, Disruption: 0.70, Balance: 0.15},
		// 	populationSize:   200,
		// 	maxGenerations:   200,
		// 	expectedBehavior: "Extreme disruption avoidance - should make minimal movements, prioritize stability",
		// 	// useGCSH:          true,
		// },
		// {
		// 	name:             "LargeCluster_MixedWorkloads_Cost_Disruption_Focus",
		// 	nodes:            mixedNodes,
		// 	pods:             mixedPods,
		// 	weightProfile:    WeightProfile{Cost: 0.50, Disruption: 0.40, Balance: 0.10},
		// 	populationSize:   200,
		// 	maxGenerations:   200,
		// 	expectedBehavior: "Cost-disruption focus with minimal balance concern",
		// 	// useGCSH:          true,
		// },
		// {
		// 	name:             "LargeCluster_MixedWorkloads_Disruption_Cost_Focus",
		// 	nodes:            mixedNodes,
		// 	pods:             mixedPods,
		// 	weightProfile:    WeightProfile{Cost: 0.20, Disruption: 0.70, Balance: 0.10},
		// 	populationSize:   200,
		// 	maxGenerations:   200,
		// 	expectedBehavior: "Disruption-cost focus with minimal balance concern",
		// 	// useGCSH:          true,
		// },
		// {
		// 	name:             "LargeCluster_MixedWorkloads_Disruption_Maximum",
		// 	nodes:            mixedNodes,
		// 	pods:             mixedPods,
		// 	weightProfile:    WeightProfile{Cost: 0.10, Disruption: 0.80, Balance: 0.10},
		// 	populationSize:   200,
		// 	maxGenerations:   200,
		// 	expectedBehavior: "Maximum disruption avoidance - should make extremely minimal movements",
		// 	// useGCSH:          true,
		// },
		// {
		// 	name: "BadToGood_ResourceCostMigration",
		// 	nodes: []NodeConfig{
		// 		// BAD $/resource pool - expensive on-demand instances with poor resource ratios
		// 		{Name: "bad-1", CPU: 4000, Mem: 8e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"}, // $0.048/vCPU, $0.012/GiB
		// 		{Name: "bad-2", CPU: 4000, Mem: 8e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
		// 		{Name: "bad-3", CPU: 4000, Mem: 8e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
		// 		{Name: "bad-4", CPU: 4000, Mem: 8e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
		// 		// GOOD $/resource pool - cheap spot instances with better resource ratios
		// 		{Name: "good-1", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"}, // $0.018/vCPU, $0.0045/GiB
		// 		{Name: "good-2", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
		// 		{Name: "good-3", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
		// 		{Name: "good-4", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
		// 	},
		// 	pods: []PodConfig{
		// 		// Group A: Similar pods on BAD nodes (intentionally inefficient placement)
		// 		{Name: "app-a-1", CPU: 1000, Mem: 2e9, Node: 0, RS: "app-a", MaxUnavail: 2}, // bad-1
		// 		{Name: "app-a-2", CPU: 1000, Mem: 2e9, Node: 0, RS: "app-a", MaxUnavail: 2}, // bad-1
		// 		{Name: "app-a-3", CPU: 1000, Mem: 2e9, Node: 1, RS: "app-a", MaxUnavail: 2}, // bad-2
		// 		{Name: "app-a-4", CPU: 1000, Mem: 2e9, Node: 1, RS: "app-a", MaxUnavail: 2}, // bad-2
		// 		{Name: "web-a-1", CPU: 500, Mem: 1e9, Node: 2, RS: "web-a", MaxUnavail: 1},  // bad-3
		// 		{Name: "web-a-2", CPU: 500, Mem: 1e9, Node: 2, RS: "web-a", MaxUnavail: 1},  // bad-3
		// 		{Name: "web-a-3", CPU: 500, Mem: 1e9, Node: 3, RS: "web-a", MaxUnavail: 1},  // bad-4
		// 		{Name: "web-a-4", CPU: 500, Mem: 1e9, Node: 3, RS: "web-a", MaxUnavail: 1},  // bad-4
		// 		// Group B: Similar pods on GOOD nodes (already efficient placement)
		// 		{Name: "app-b-1", CPU: 1000, Mem: 2e9, Node: 4, RS: "app-b", MaxUnavail: 2}, // good-1
		// 		{Name: "app-b-2", CPU: 1000, Mem: 2e9, Node: 4, RS: "app-b", MaxUnavail: 2}, // good-1
		// 		{Name: "app-b-3", CPU: 1000, Mem: 2e9, Node: 5, RS: "app-b", MaxUnavail: 2}, // good-2
		// 		{Name: "app-b-4", CPU: 1000, Mem: 2e9, Node: 5, RS: "app-b", MaxUnavail: 2}, // good-2
		// 		{Name: "web-b-1", CPU: 500, Mem: 1e9, Node: 6, RS: "web-b", MaxUnavail: 1},  // good-3
		// 		{Name: "web-b-2", CPU: 500, Mem: 1e9, Node: 6, RS: "web-b", MaxUnavail: 1},  // good-3
		// 		{Name: "web-b-3", CPU: 500, Mem: 1e9, Node: 7, RS: "web-b", MaxUnavail: 1},  // good-4
		// 		{Name: "web-b-4", CPU: 500, Mem: 1e9, Node: 7, RS: "web-b", MaxUnavail: 1},  // good-4
		// 	},
		// 	weightProfile:    WeightProfile{Cost: 0.90, Disruption: 0.10, Balance: 0.00},
		// 	populationSize:   200,
		// 	maxGenerations:   500,
		// 	expectedBehavior: "Should migrate pods from expensive on-demand nodes to cheap spot nodes with better $/resource ratios",
		// },
		// Scalability test series with precise resource allocation for academic comparison
		createPreciseTestCase("Cluster_20Nodes_Precise_Bal", 20, 0.35, WeightProfile{Cost: 0.33, Disruption: 0.33, Balance: 0.34}, 500, 500),
		createPreciseTestCase("Cluster_40Nodes_Precise_Bal", 40, 0.35, WeightProfile{Cost: 0.33, Disruption: 0.33, Balance: 0.34}, 200, 200),
		createPreciseTestCase("Cluster_60Nodes_Precise_Bal", 60, 0.35, WeightProfile{Cost: 0.33, Disruption: 0.33, Balance: 0.34}, 200, 200),
		createPreciseTestCase("Cluster_80Nodes_Precise_Bal", 80, 0.35, WeightProfile{Cost: 0.33, Disruption: 0.33, Balance: 0.34}, 200, 200),
		// createPreciseTestCase("Cluster_100Nodes_Precise_Bal", 100, 0.35, WeightProfile{Cost: 0.33, Disruption: 0.33, Balance: 0.34}, 200, 200),
		// {
		// 	name:             "LargeBadToGood_SlowerConvergence",
		// 	nodes:            testCaseNodes,
		// 	pods:             testCasePods,
		// 	weightProfile:    WeightProfile{Cost: 0.45, Disruption: 0.10, Balance: 0.45},
		// 	populationSize:   300,
		// 	maxGenerations:   300,
		// 	expectedBehavior: "Large-scale bad-to-good migration with slower convergence - should gradually discover that good nodes are cheaper and migrate workloads",
		// 	useGCSH:          false,
		// },
		// {
		// 	name:             "LargeBadToGood_SlowerConvergence",
		// 	nodes:            testCaseNodes,
		// 	pods:             testCasePods,
		// 	weightProfile:    WeightProfile{Cost: 0.10, Disruption: 0.10, Balance: 0.80},
		// 	populationSize:   300,
		// 	maxGenerations:   300,
		// 	expectedBehavior: "Large-scale bad-to-good migration with slower convergence - should gradually discover that good nodes are cheaper and migrate workloads",
		// 	useGCSH:          false,
		// },
		// {
		// 	name:             "LargeBadToGood_SlowerConvergence",
		// 	nodes:            testCaseNodes,
		// 	pods:             testCasePods,
		// 	weightProfile:    WeightProfile{Cost: 0.40, Disruption: 0.20, Balance: 0.40},
		// 	populationSize:   300,
		// 	maxGenerations:   300,
		// 	expectedBehavior: "Large-scale bad-to-good migration with slower convergence - should gradually discover that good nodes are cheaper and migrate workloads",
		// 	useGCSH:          false,
		// },
		// {
		// 	name:             "LargeBadToGood_SlowerConvergence",
		// 	nodes:            testCaseNodes,
		// 	pods:             testCasePods,
		// 	weightProfile:    WeightProfile{Cost: 0.33, Disruption: 0.33, Balance: 0.34},
		// 	populationSize:   300,
		// 	maxGenerations:   300,
		// 	expectedBehavior: "Large-scale bad-to-good migration with slower convergence - should gradually discover that good nodes are cheaper and migrate workloads",
		// 	useGCSH:          false,
		// },
		// {
		// 	name:             "LargeBadToGood_SlowerConvergence",
		// 	nodes:            testCaseNodes,
		// 	pods:             testCasePods,
		// 	weightProfile:    WeightProfile{Cost: 0.30, Disruption: 0.40, Balance: 0.30},
		// 	populationSize:   300,
		// 	maxGenerations:   300,
		// 	expectedBehavior: "Large-scale bad-to-good migration with slower convergence - should gradually discover that good nodes are cheaper and migrate workloads",
		// 	useGCSH:          false,
		// },
		// {
		// 	name:             "LargeBadToGood_SlowerConvergence",
		// 	nodes:            testCaseNodes,
		// 	pods:             testCasePods,
		// 	weightProfile:    WeightProfile{Cost: 0.20, Disruption: 0.60, Balance: 0.20},
		// 	populationSize:   300,
		// 	maxGenerations:   300,
		// 	expectedBehavior: "Large-scale bad-to-good migration with slower convergence - should gradually discover that good nodes are cheaper and migrate workloads",
		// 	useGCSH:          false,
		// },
		// {
		// 	name:             "LargeBadToGood_SlowerConvergence",
		// 	nodes:            testCaseNodes,
		// 	pods:             testCasePods,
		// 	weightProfile:    WeightProfile{Cost: 0.15, Disruption: 0.70, Balance: 0.15},
		// 	populationSize:   300,
		// 	maxGenerations:   300,
		// 	expectedBehavior: "Large-scale bad-to-good migration with slower convergence - should gradually discover that good nodes are cheaper and migrate workloads",
		// 	useGCSH:          false,
		// },
		// {
		// 	name:             "LargeBadToGood_SlowerConvergence",
		// 	nodes:            testCaseNodes,
		// 	pods:             testCasePods,
		// 	weightProfile:    WeightProfile{Cost: 0.50, Disruption: 0.40, Balance: 0.10},
		// 	populationSize:   300,
		// 	maxGenerations:   300,
		// 	expectedBehavior: "Large-scale bad-to-good migration with slower convergence - should gradually discover that good nodes are cheaper and migrate workloads",
		// 	useGCSH:          false,
		// },
		// {
		// 	name:             "LargeBadToGood_SlowerConvergence",
		// 	nodes:            testCaseNodes,
		// 	pods:             testCasePods,
		// 	weightProfile:    WeightProfile{Cost: 0.20, Disruption: 0.70, Balance: 0.10},
		// 	populationSize:   300,
		// 	maxGenerations:   300,
		// 	expectedBehavior: "Large-scale bad-to-good migration with slower convergence - should gradually discover that good nodes are cheaper and migrate workloads",
		// 	useGCSH:          false,
		// },
		// {
		// 	name:             "LargeBadToGood_SlowerConvergence",
		// 	nodes:            testCaseNodes,
		// 	pods:             testCasePods,
		// 	weightProfile:    WeightProfile{Cost: 0.10, Disruption: 0.80, Balance: 0.10},
		// 	populationSize:   300,
		// 	maxGenerations:   300,
		// 	expectedBehavior: "Large-scale bad-to-good migration with slower convergence - should gradually discover that good nodes are cheaper and migrate workloads",
		// 	useGCSH:          false,
		// },
		// WeightProfile{Cost: 0.80, Disruption: 0.10, Balance: 0.10}
		// WeightProfile{Cost: 0.45, Disruption: 0.10, Balance: 0.45}
		// WeightProfile{Cost: 0.10, Disruption: 0.10, Balance: 0.80}
		// WeightProfile{Cost: 0.40, Disruption: 0.20, Balance: 0.40}
		// WeightProfile{Cost: 0.33, Disruption: 0.33, Balance: 0.34}
		// WeightProfile{Cost: 0.30, Disruption: 0.40, Balance: 0.30}
		// WeightProfile{Cost: 0.20, Disruption: 0.60, Balance: 0.20}
		// WeightProfile{Cost: 0.15, Disruption: 0.70, Balance: 0.15}
		// WeightProfile{Cost: 0.50, Disruption: 0.40, Balance: 0.10}
		// WeightProfile{Cost: 0.20, Disruption: 0.70, Balance: 0.10}
		// WeightProfile{Cost: 0.10, Disruption: 0.80, Balance: 0.10}
		// {
		// 	name:             "MassiveCluster_CostOptimization_2_Random",
		// 	nodes:            massiveNodes,
		// 	pods:             massivePods,
		// 	weightProfile:    WeightProfile{Cost: 0.80, Disruption: 0.10, Balance: 0.10},
		// 	populationSize:   300,
		// 	maxGenerations:   300,
		// 	expectedBehavior: "Massive cluster cost optimization - should gradually migrate workloads from expensive to cheap nodes across multiple tiers",
		// 	useGCSH:          false, // Set to false to test random initialization
		// },
		// {
		// 	name: "InitializationComparison_RandomVsGCSH",
		// 	nodes: []NodeConfig{
		// 		// Simple bad-to-good scenario for clear comparison
		// 		{Name: "expensive-1", CPU: 4000, Mem: 8e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
		// 		{Name: "expensive-2", CPU: 4000, Mem: 8e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
		// 		{Name: "expensive-3", CPU: 4000, Mem: 8e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
		// 		{Name: "expensive-4", CPU: 4000, Mem: 8e9, Type: "m5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
		// 		{Name: "cheap-1", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
		// 		{Name: "cheap-2", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
		// 		{Name: "cheap-3", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
		// 		{Name: "cheap-4", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
		// 	},
		// 	pods: []PodConfig{
		// 		// All pods start on expensive nodes - clear optimization opportunity
		// 		{Name: "app-1", CPU: 1000, Mem: 2e9, Node: 0, RS: "app", MaxUnavail: 3},
		// 		{Name: "app-2", CPU: 1000, Mem: 2e9, Node: 0, RS: "app", MaxUnavail: 3},
		// 		{Name: "app-3", CPU: 1000, Mem: 2e9, Node: 1, RS: "app", MaxUnavail: 3},
		// 		{Name: "app-4", CPU: 1000, Mem: 2e9, Node: 1, RS: "app", MaxUnavail: 3},
		// 		{Name: "web-1", CPU: 500, Mem: 1e9, Node: 2, RS: "web", MaxUnavail: 2},
		// 		{Name: "web-2", CPU: 500, Mem: 1e9, Node: 2, RS: "web", MaxUnavail: 2},
		// 		{Name: "web-3", CPU: 500, Mem: 1e9, Node: 3, RS: "web", MaxUnavail: 2},
		// 		{Name: "web-4", CPU: 500, Mem: 1e9, Node: 3, RS: "web", MaxUnavail: 2},
		// 	},
		// 	weightProfile:    WeightProfile{Cost: 0.80, Disruption: 0.15, Balance: 0.05},
		// 	populationSize:   100,
		// 	maxGenerations:   200,
		// 	expectedBehavior: "Compare random vs GCSH initialization - GCSH should converge faster and find better solutions",
		// 	useGCSH:          true, // This test case will be handled specially
		// },
		// {
		// 	name: "ExtremeResourceCostDifference",
		// 	nodes: []NodeConfig{
		// 		// TERRIBLE $/resource pool - very expensive memory-optimized on-demand
		// 		{Name: "terrible-1", CPU: 4000, Mem: 32e9, Type: "r5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"}, // $0.063/vCPU, $0.0079/GiB
		// 		{Name: "terrible-2", CPU: 4000, Mem: 32e9, Type: "r5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
		// 		{Name: "terrible-3", CPU: 4000, Mem: 32e9, Type: "r5.xlarge", Region: "us-east-1", Lifecycle: "on-demand"},
		// 		// EXCELLENT $/resource pool - very cheap burstable spot instances
		// 		{Name: "excellent-1", CPU: 4000, Mem: 8e9, Type: "t3.xlarge", Region: "us-east-1", Lifecycle: "spot"}, // $0.0147/vCPU, $0.0037/GiB
		// 		{Name: "excellent-2", CPU: 4000, Mem: 8e9, Type: "t3.xlarge", Region: "us-east-1", Lifecycle: "spot"},
		// 		{Name: "excellent-3", CPU: 4000, Mem: 8e9, Type: "t3.xlarge", Region: "us-east-1", Lifecycle: "spot"},
		// 	},
		// 	pods: []PodConfig{
		// 		// CPU-intensive workloads on TERRIBLE nodes (massive waste of memory resources)
		// 		{Name: "cpu-heavy-1", CPU: 2000, Mem: 1e9, Node: 0, RS: "cpu-heavy", MaxUnavail: 1}, // terrible-1: using 2 cores, 1GB of 4 cores, 32GB
		// 		{Name: "cpu-heavy-2", CPU: 2000, Mem: 1e9, Node: 1, RS: "cpu-heavy", MaxUnavail: 1}, // terrible-2: using 2 cores, 1GB of 4 cores, 32GB
		// 		{Name: "cpu-heavy-3", CPU: 2000, Mem: 1e9, Node: 2, RS: "cpu-heavy", MaxUnavail: 1}, // terrible-3: using 2 cores, 1GB of 4 cores, 32GB
		// 		// Balanced workloads on EXCELLENT nodes (good fit)
		// 		{Name: "balanced-1", CPU: 1500, Mem: 3e9, Node: 3, RS: "balanced", MaxUnavail: 2}, // excellent-1: using 1.5 cores, 3GB of 4 cores, 8GB
		// 		{Name: "balanced-2", CPU: 1500, Mem: 3e9, Node: 4, RS: "balanced", MaxUnavail: 2}, // excellent-2: using 1.5 cores, 3GB of 4 cores, 8GB
		// 		{Name: "balanced-3", CPU: 1500, Mem: 3e9, Node: 5, RS: "balanced", MaxUnavail: 2}, // excellent-3: using 1.5 cores, 3GB of 4 cores, 8GB
		// 	},
		// 	weightProfile:    WeightProfile{Cost: 0.95, Disruption: 0.05, Balance: 0.00},
		// 	populationSize:   150,
		// 	maxGenerations:   300,
		// 	expectedBehavior: "Should aggressively migrate CPU-heavy workloads from expensive memory-optimized nodes to cheap compute nodes",
		// },
		// {
		// 	name: "SeverelyUnbalanced_CostVsBalance",
		// 	nodes: []NodeConfig{
		// 		// All nodes same type for pure balance testing
		// 		{Name: "node-1", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
		// 		{Name: "node-2", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
		// 		{Name: "node-3", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
		// 		{Name: "node-4", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
		// 	},
		// 	pods: []PodConfig{
		// 		// ALL pods crammed on node-1 (severely unbalanced)
		// 		{Name: "overload-1", CPU: 800, Mem: 1.5e9, Node: 0, RS: "overload", MaxUnavail: 3},
		// 		{Name: "overload-2", CPU: 800, Mem: 1.5e9, Node: 0, RS: "overload", MaxUnavail: 3},
		// 		{Name: "overload-3", CPU: 800, Mem: 1.5e9, Node: 0, RS: "overload", MaxUnavail: 3},
		// 		{Name: "overload-4", CPU: 800, Mem: 1.5e9, Node: 0, RS: "overload", MaxUnavail: 3},
		// 		{Name: "overload-5", CPU: 800, Mem: 1.5e9, Node: 0, RS: "overload", MaxUnavail: 3},
		// 		// Nodes 2, 3, 4 are completely empty (maximum imbalance)
		// 	},
		// 	weightProfile:    WeightProfile{Cost: 0.10, Disruption: 0.10, Balance: 0.80},
		// 	populationSize:   150,
		// 	maxGenerations:   400,
		// 	expectedBehavior: "Should spread pods from overloaded node-1 to empty nodes for better balance",
		// },
		// {
		// 	name: "MixedUnbalance_CostVsBalanceTradeoff",
		// 	nodes: []NodeConfig{
		// 		// Mix of cheap and expensive nodes to test cost vs balance trade-offs
		// 		{Name: "cheap-overloaded", CPU: 4000, Mem: 8e9, Type: "t3.large", Region: "us-east-1", Lifecycle: "spot"},     // $0.0335/hr - cheap but overloaded
		// 		{Name: "expensive-empty", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "on-demand"}, // $0.096/hr - expensive but empty
		// 		{Name: "medium-empty", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},         // $0.036/hr - medium cost, empty
		// 		{Name: "cheap-empty", CPU: 4000, Mem: 8e9, Type: "t3.large", Region: "us-east-1", Lifecycle: "spot"},          // $0.0335/hr - cheap and empty
		// 	},
		// 	pods: []PodConfig{
		// 		// ALL pods on the cheap node (cost-optimal but severely unbalanced)
		// 		{Name: "packed-1", CPU: 700, Mem: 1.2e9, Node: 0, RS: "packed", MaxUnavail: 2},
		// 		{Name: "packed-2", CPU: 700, Mem: 1.2e9, Node: 0, RS: "packed", MaxUnavail: 2},
		// 		{Name: "packed-3", CPU: 700, Mem: 1.2e9, Node: 0, RS: "packed", MaxUnavail: 2},
		// 		{Name: "packed-4", CPU: 700, Mem: 1.2e9, Node: 0, RS: "packed", MaxUnavail: 2},
		// 		{Name: "packed-5", CPU: 700, Mem: 1.2e9, Node: 0, RS: "packed", MaxUnavail: 2},
		// 		{Name: "packed-6", CPU: 500, Mem: 1e9, Node: 0, RS: "packed-small", MaxUnavail: 1},
		// 		// Nodes 1, 2, 3 are empty but have different costs
		// 	},
		// 	weightProfile:    WeightProfile{Cost: 0.40, Disruption: 0.20, Balance: 0.40},
		// 	populationSize:   200,
		// 	maxGenerations:   500,
		// 	expectedBehavior: "Should balance cost savings vs load distribution - prefer cheaper nodes but spread for balance",
		// },
		// {
		// 	name: "BalanceFocused_MinimalCost",
		// 	nodes: []NodeConfig{
		// 		// All same cost to focus purely on balance
		// 		{Name: "balance-1", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
		// 		{Name: "balance-2", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
		// 		{Name: "balance-3", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
		// 		{Name: "balance-4", CPU: 4000, Mem: 8e9, Type: "m5.large", Region: "us-east-1", Lifecycle: "spot"},
		// 	},
		// 	pods: []PodConfig{
		// 		// Extremely unbalanced: all on first two nodes
		// 		{Name: "unbal-1", CPU: 1000, Mem: 2e9, Node: 0, RS: "unbal-a", MaxUnavail: 2},
		// 		{Name: "unbal-2", CPU: 1000, Mem: 2e9, Node: 0, RS: "unbal-a", MaxUnavail: 2},
		// 		{Name: "unbal-3", CPU: 1000, Mem: 2e9, Node: 0, RS: "unbal-a", MaxUnavail: 2},
		// 		{Name: "unbal-4", CPU: 1000, Mem: 2e9, Node: 1, RS: "unbal-b", MaxUnavail: 2},
		// 		{Name: "unbal-5", CPU: 1000, Mem: 2e9, Node: 1, RS: "unbal-b", MaxUnavail: 2},
		// 		{Name: "unbal-6", CPU: 1000, Mem: 2e9, Node: 1, RS: "unbal-b", MaxUnavail: 2},
		// 		// Nodes 3 and 4 are completely empty
		// 	},
		// 	weightProfile:    WeightProfile{Cost: 0.10, Disruption: 0.20, Balance: 0.70},
		// 	populationSize:   150,
		// 	maxGenerations:   300,
		// 	expectedBehavior: "Should prioritize balance over cost - spread pods evenly across all nodes",
		// },
		// {
		// 	name:             "MassiveCluster_Cost_Focused",
		// 	nodes:            nil,
		// 	pods:             nil,
		// 	weightProfile:    WeightProfile{Cost: 0.80, Disruption: 0.10, Balance: 0.10},
		// 	populationSize:   200,
		// 	maxGenerations:   200,
		// 	expectedBehavior: "Cost-focused optimization - should aggressively move pods to cheaper nodes",
		// 	useGCSH:          false,
		// },
		// {
		// 	name:             "MassiveCluster_Cost_Balance_Mixed",
		// 	nodes:            nil,
		// 	pods:             nil,
		// 	weightProfile:    WeightProfile{Cost: 0.45, Disruption: 0.10, Balance: 0.45},
		// 	populationSize:   200,
		// 	maxGenerations:   200,
		// 	expectedBehavior: "Cost-balance optimization - should balance cost savings with load distribution",
		// 	useGCSH:          false,
		// },
		// {
		// 	name:             "MassiveCluster_Balance_Focused",
		// 	nodes:            nil,
		// 	pods:             nil,
		// 	weightProfile:    WeightProfile{Cost: 0.10, Disruption: 0.10, Balance: 0.80},
		// 	populationSize:   200,
		// 	maxGenerations:   200,
		// 	expectedBehavior: "Balance-focused optimization - should prioritize even load distribution across nodes",
		// 	useGCSH:          false,
		// },
		// {
		// 	name:             "MassiveCluster_Cost_Disruption_Balance",
		// 	nodes:            nil,
		// 	pods:             nil,
		// 	weightProfile:    WeightProfile{Cost: 0.40, Disruption: 0.20, Balance: 0.40},
		// 	populationSize:   200,
		// 	maxGenerations:   200,
		// 	expectedBehavior: "Balanced cost-disruption-balance optimization with moderate disruption tolerance",
		// 	useGCSH:          false,
		// },
		// {
		// 	name:             "MassiveCluster_Equal_Weights",
		// 	nodes:            nil,
		// 	pods:             nil,
		// 	weightProfile:    WeightProfile{Cost: 0.33, Disruption: 0.33, Balance: 0.34},
		// 	populationSize:   200,
		// 	maxGenerations:   200,
		// 	expectedBehavior: "Equal-weight optimization - should find balanced trade-offs across all objectives",
		// 	useGCSH:          false,
		// },
		// {
		// 	name:             "MassiveCluster_Disruption_Moderate",
		// 	nodes:            nil,
		// 	pods:             nil,
		// 	weightProfile:    WeightProfile{Cost: 0.30, Disruption: 0.40, Balance: 0.30},
		// 	populationSize:   200,
		// 	maxGenerations:   200,
		// 	expectedBehavior: "Moderate disruption focus - should be more conservative with pod movements",
		// 	useGCSH:          false,
		// },
		// {
		// 	name:             "MassiveCluster_Disruption_High",
		// 	nodes:            nil,
		// 	pods:             nil,
		// 	weightProfile:    WeightProfile{Cost: 0.20, Disruption: 0.60, Balance: 0.20},
		// 	populationSize:   200,
		// 	maxGenerations:   200,
		// 	expectedBehavior: "High disruption focus - should minimize pod movements, small incremental improvements",
		// 	useGCSH:          false,
		// },
		// {
		// 	name:             "MassiveCluster_Disruption_Extreme",
		// 	nodes:            nil,
		// 	pods:             nil,
		// 	weightProfile:    WeightProfile{Cost: 0.15, Disruption: 0.70, Balance: 0.15},
		// 	populationSize:   200,
		// 	maxGenerations:   200,
		// 	expectedBehavior: "Extreme disruption avoidance - should make minimal movements, prioritize stability",
		// 	useGCSH:          false,
		// },
	}

	realisticNodes := generateMassiveRealisticNodes()
	realisticPods := generateMassiveRealisticPods()

	for _, tc := range testCases {
		if tc.nodes == nil {
			tc.nodes = realisticNodes
		}
		if tc.pods == nil {
			tc.pods = realisticPods
		}
		t.Run(tc.name, func(t *testing.T) {
			runSequentialOptimizationWithOutput(t, tc, 50) // Run 50 sequential optimization rounds with JSON output
		})
	}
}

func runSingleOptimizationRound(t *testing.T, tc struct {
	name             string
	nodes            []NodeConfig
	pods             []PodConfig
	weightProfile    WeightProfile
	populationSize   int
	maxGenerations   int
	expectedBehavior string
	useGCSH          bool
}, nodes []framework.NodeInfo, pods []framework.PodInfo, round int, previousSolutions []Analysis) (Analysis, []Analysis) {
	// Create problem for this round with initialization toggle
	problem := createKubernetesProblem2(nodes, pods, tc.weightProfile)
	problem.useGCSH = tc.useGCSH

	if round == 1 {
		if tc.useGCSH {
			t.Logf("Using GCSH warm start initialization")
		} else {
			t.Logf("Using random initialization (GCSH disabled)")
		}
	}

	// Configure and run NSGA-II
	config := algorithms.NSGA2Config{
		PopulationSize:       tc.populationSize,
		MaxGenerations:       tc.maxGenerations,
		CrossoverProbability: 0.9,
		MutationProbability:  0.3,
		TournamentSize:       3,
		ParallelExecution:    true,
	}

	// Run NSGA-II with diversity seeding
	nsga2 := algorithms.NewNSGAII(config, problem)

	// Create problem with custom seeding for diversity
	var population []*algorithms.NSGAIISolution
	if len(previousSolutions) > 0 && round > 1 {
		t.Logf("Seeding with ALL %d solutions from previous rounds (no filtering)", len(previousSolutions))

		// Create a custom problem that seeds with ALL previous solutions
		seededProblem := createSeededProblemAllSolutions(problem, previousSolutions, tc.populationSize)
		nsga2Seeded := algorithms.NewNSGAII(config, seededProblem)
		population = nsga2Seeded.Run()
	} else {
		// First round - use standard initialization
		t.Logf("First round: using standard random initialization")
		population = nsga2.Run()
	}

	// Get Pareto front
	fronts := algorithms.NonDominatedSort(population)
	if len(fronts) == 0 || len(fronts[0]) == 0 {
		t.Fatal("No solutions found in Pareto front")
	}

	paretoFront := fronts[0]
	t.Logf("Round %d: Found %d Pareto-optimal solutions", round, len(paretoFront))

	// Analyze solutions
	analyses := make([]Analysis, len(paretoFront))
	objFuncs := problem.ObjectiveFuncs()
	for i, sol := range paretoFront {
		intSol := sol.Solution.(*framework.IntegerSolution)
		// Evaluate objectives manually
		obj := make([]float64, len(objFuncs))
		for j, objFunc := range objFuncs {
			obj[j] = objFunc(sol.Solution)
		}

		movements := 0
		for j, node := range intSol.Variables {
			if node != pods[j].Node {
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

	// Sort by weighted total to find best
	for i := 0; i < len(analyses)-1; i++ {
		for j := i + 1; j < len(analyses); j++ {
			if analyses[i].WeightedTotal > analyses[j].WeightedTotal {
				analyses[i], analyses[j] = analyses[j], analyses[i]
			}
		}
	}

	// Deduplicate solutions
	uniqueAnalyses := []Analysis{}
	seenAssignments := make(map[string]bool)

	for _, a := range analyses {
		key := fmt.Sprintf("%v", a.Assignment)
		if !seenAssignments[key] {
			seenAssignments[key] = true
			uniqueAnalyses = append(uniqueAnalyses, a)
		}
	}

	// Show top 5 for subsequent rounds, all for first round
	maxToShow := 5

	t.Logf("\nTop %d solutions for round %d:", maxToShow, round)
	for i := 0; i < len(uniqueAnalyses) && i < maxToShow; i++ {
		a := uniqueAnalyses[i]
		isInitial := a.Movements == 0
		marker := ""
		if isInitial {
			marker = " [CURRENT STATE]"
		}
		if i == 0 {
			marker += " [BEST]"
		}

		// Add movement category for clarity
		var category string
		switch {
		case a.Movements == 0:
			category = "No change"
		case a.Movements <= 2:
			category = "Minimal change"
		case a.Movements <= 5:
			category = "Small change"
		case a.Movements <= 10:
			category = "Medium change"
		default:
			category = "Large change"
		}

		t.Logf("\n%d. [%s] Movements: %d%s", i+1, category, a.Movements, marker)
		t.Logf("   Objectives: Cost=%.4f, Disruption=%.4f, Balance=%.4f, Weighted=%.4f",
			a.Cost, a.Disruption, a.Balance, a.WeightedTotal)
		t.Logf("   Assignment: %v", a.Assignment)

		// Show specific movements for all solutions in top 5
		if a.Movements > 0 {
			showDetailedMovements(t, a.Assignment, pods, tc.nodes, "   ")
		} else {
			t.Logf("   📋 No pod movements (current state)")
		}
	}

	return uniqueAnalyses[0], uniqueAnalyses // Return best solution and all solutions
}

func showDetailedMovements(t *testing.T, assignment []int, pods []framework.PodInfo, nodes []NodeConfig, prefix string) {
	t.Logf("%sSpecific pod movements:", prefix)
	movementsByType := make(map[string][]string)

	for j, targetNode := range assignment {
		if targetNode != pods[j].Node {
			currentNodeName := nodes[pods[j].Node].Name
			targetNodeName := nodes[targetNode].Name
			currentType := nodes[pods[j].Node].Type
			targetType := nodes[targetNode].Type
			currentLifecycle := nodes[pods[j].Node].Lifecycle
			targetLifecycle := nodes[targetNode].Lifecycle

			// Categorize movement type
			var movementType string
			if currentLifecycle == "on-demand" && targetLifecycle == "spot" {
				movementType = "💰 On-demand → Spot (cost saving)"
			} else if currentLifecycle == "spot" && targetLifecycle == "on-demand" {
				movementType = "🛡️ Spot → On-demand (reliability)"
			} else if currentType != targetType {
				movementType = fmt.Sprintf("🔄 Instance type change (%s → %s)", currentType, targetType)
			} else {
				movementType = "🔀 Same type migration"
			}

			podInfo := fmt.Sprintf("%s: %s → %s [%.1f cores, %.1f GiB]",
				pods[j].Name, currentNodeName, targetNodeName,
				pods[j].CPURequest/1000.0, pods[j].MemRequest/1e9)

			if movementsByType[movementType] == nil {
				movementsByType[movementType] = []string{}
			}
			movementsByType[movementType] = append(movementsByType[movementType], podInfo)
		}
	}

	// Display movements grouped by type
	for movementType, movements := range movementsByType {
		t.Logf("%s  %s:", prefix, movementType)
		for _, movement := range movements {
			t.Logf("%s    - %s", prefix, movement)
		}
	}
}

// runSequentialOptimizationWithOutput runs optimization with JSON output for visualization
func runSequentialOptimizationWithOutput(t *testing.T, tc struct {
	name             string
	nodes            []NodeConfig
	pods             []PodConfig
	weightProfile    WeightProfile
	populationSize   int
	maxGenerations   int
	expectedBehavior string
	useGCSH          bool
}, numRounds int) {
	// Create unique timestamp with microseconds to avoid parallel test collisions
	timestamp := time.Now().Format("2006-01-02_15-04-05.000000")

	// Run baseline algorithms for comparison
	t.Logf("🏁 Running baseline algorithms for comparison...")
	baselineResults := runAllBaselineAlgorithms(tc.nodes, tc.pods, tc.weightProfile)

	// Log baseline results
	for _, result := range baselineResults {
		t.Logf("📊 %s: Cost=%.2f, Balance=%.2f, Movements=%d, Time=%.2fms, Feasible=%t",
			result.Algorithm, result.RawCost, result.RawBalance, result.Movements, result.ExecutionTime, result.Feasible)
	}

	// Initialize output structure
	output := OptimizationOutput{
		Timestamp: timestamp,
		TestCase: TestCaseConfig{
			Name:             tc.name,
			Nodes:            tc.nodes,
			Pods:             tc.pods,
			WeightProfile:    tc.weightProfile,
			ExpectedBehavior: tc.expectedBehavior,
		},
		BaselineResults: baselineResults,
		Algorithm: AlgorithmConfig{
			PopulationSize:       tc.populationSize,
			MaxGenerations:       tc.maxGenerations,
			CrossoverProbability: 0.7,
			MutationProbability:  0.3,
			TournamentSize:       3,
			ParallelExecution:    true,
		},
		Rounds: make([]OptimizationRound, 0, numRounds),
	}

	t.Logf("\n🚀 SEQUENTIAL OPTIMIZATION WITH JSON OUTPUT: %s", tc.name)
	t.Logf("Expected behavior: %s", tc.expectedBehavior)
	t.Logf("Weights: Cost=%.2f, Disruption=%.2f, Balance=%.2f",
		tc.weightProfile.Cost, tc.weightProfile.Disruption, tc.weightProfile.Balance)
	t.Logf("Running %d sequential optimization rounds...\n", numRounds)

	// Convert to framework types once
	nodes := make([]framework.NodeInfo, len(tc.nodes))
	for i, n := range tc.nodes {
		c, err := cost.GetInstanceCost(n.Region, n.Type, n.Lifecycle)
		if err != nil {
			t.Errorf("Failed to get cost for node %s: %v", n.Name, err)
		}
		nodes[i] = framework.NodeInfo{
			Idx:               i,
			Name:              n.Name,
			CPUCapacity:       n.CPU,
			MemCapacity:       n.Mem,
			InstanceType:      n.Type,
			InstanceLifecycle: n.Lifecycle,
			Region:            n.Region,
			HourlyCost:        c,
		}
	}

	// Store the initial cluster state
	initialClusterState := make([]int, len(tc.pods))
	for i, pod := range tc.pods {
		initialClusterState[i] = pod.Node
	}

	// Debug: Check initial cluster state consistency
	trueInitialCost, _ := calculateActualMetricsEffective(initialClusterState, tc.nodes, tc.pods)
	t.Logf("🔍 DEBUG: True initial cluster cost for %s: $%.6f/hour", tc.name, trueInitialCost)

	// Current pod configuration (will be updated each round)
	currentPods := make([]PodConfig, len(tc.pods))
	copy(currentPods, tc.pods)

	// Keep solutions from the immediately previous round for seeding
	var previousRoundSolutions []Analysis

	// Dynamic stopping criteria based on PDB-blocked rounds
	maxConsecutiveZeroMoves := 5 // Stop after 5 consecutive rounds with 0 movements
	consecutiveZeroMoves := 0
	round := 0

	for round < numRounds {
		t.Logf("\n%s", strings.Repeat("=", 100))
		t.Logf("🔄 OPTIMIZATION ROUND %d (Max: %d, Zero-move streak: %d/%d)", round+1, numRounds, consecutiveZeroMoves, maxConsecutiveZeroMoves)
		t.Logf("%s", strings.Repeat("=", 100))

		// Convert current pods to framework types
		pods := make([]framework.PodInfo, len(currentPods))
		for i, p := range currentPods {
			pods[i] = framework.PodInfo{
				Idx:                    i,
				Name:                   p.Name,
				CPURequest:             p.CPU,
				MemRequest:             p.Mem,
				Node:                   p.Node,
				ReplicaSetName:         p.RS,
				MaxUnavailableReplicas: p.MaxUnavail,
			}
		}

		// Calculate initial state for this round
		currentAssignment := make([]int, len(currentPods))
		for i, pod := range currentPods {
			currentAssignment[i] = pod.Node
		}
		initialCost, initialBalance := calculateActualMetricsEffective(currentAssignment, tc.nodes, currentPods)
		initialState := createClusterState(currentAssignment, tc.nodes, currentPods, nodes)

		t.Logf("💰 Current cluster cost: $%.2f/hour, Load balance: %.1f%%", initialCost, initialBalance)

		// Run optimization with seeding from previous round only
		bestSol, allSols := runSingleOptimizationRound(t, tc, nodes, pods, round+1, previousRoundSolutions)

		// Convert all solutions to 3D format for JSON
		paretoFront := make([]Solution3D, len(allSols))
		for i, sol := range allSols {
			rawCost, rawBalance := calculateActualMetricsEffective(sol.Assignment, tc.nodes, currentPods)
			paretoFront[i] = Solution3D{
				ID:            i,
				Assignment:    sol.Assignment,
				Cost:          sol.Cost,
				Disruption:    sol.Disruption,
				Balance:       sol.Balance,
				WeightedScore: sol.WeightedTotal,
				Movements:     sol.Movements,
				RawCost:       rawCost,
				RawDisruption: sol.RawDisruption,
				RawBalance:    rawBalance,
			}
		}

		// Calculate final state and improvements
		finalCost, finalBalance := calculateActualMetricsEffective(bestSol.Assignment, tc.nodes, currentPods)
		finalState := createClusterState(bestSol.Assignment, tc.nodes, currentPods, nodes)

		// Calculate improvements with NaN protection
		costSavings := initialCost - finalCost
		costSavingsPercent := 0.0
		if initialCost > 0 {
			costSavingsPercent = ((initialCost - finalCost) / initialCost) * 100
		}
		balanceImprovement := initialBalance - finalBalance
		annualSavings := costSavings * 24 * 365

		// Protect against NaN values
		if math.IsNaN(costSavings) || math.IsInf(costSavings, 0) {
			costSavings = 0.0
		}
		if math.IsNaN(costSavingsPercent) || math.IsInf(costSavingsPercent, 0) {
			costSavingsPercent = 0.0
		}
		if math.IsNaN(balanceImprovement) || math.IsInf(balanceImprovement, 0) {
			balanceImprovement = 0.0
		}
		if math.IsNaN(annualSavings) || math.IsInf(annualSavings, 0) {
			annualSavings = 0.0
		}

		improvements := Improvements{
			CostSavings:        costSavings,
			CostSavingsPercent: costSavingsPercent,
			BalanceImprovement: balanceImprovement,
			AnnualSavings:      annualSavings,
		}

		// Calculate feasible moves for this round (what can actually be implemented)
		feasibleMoveIndices := calculateFeasibleMovements(bestSol.Assignment, pods)

		// Create intermediate assignment (after applying only feasible moves)
		intermediateAssignment := make([]int, len(currentAssignment))
		copy(intermediateAssignment, currentAssignment)
		for _, podIdx := range feasibleMoveIndices {
			intermediateAssignment[podIdx] = bestSol.Assignment[podIdx]
		}

		// Calculate intermediate state metrics
		intermediateCost, _ := calculateActualMetricsEffective(intermediateAssignment, tc.nodes, currentPods)
		intermediateState := createClusterState(intermediateAssignment, tc.nodes, currentPods, nodes)

		// Create detailed pod movement analysis
		movedPods := []PodMovement{}
		for _, podIdx := range feasibleMoveIndices {
			fromNode := tc.nodes[currentAssignment[podIdx]]
			toNode := tc.nodes[bestSol.Assignment[podIdx]]

			moveType := categorizeMove(fromNode, toNode)
			costImpact := fromNode.CPU*0.001 - toNode.CPU*0.001 // Simplified cost impact

			// Protect against NaN
			if math.IsNaN(costImpact) || math.IsInf(costImpact, 0) {
				costImpact = 0.0
			}

			movedPods = append(movedPods, PodMovement{
				PodName:    tc.pods[podIdx].Name,
				ReplicaSet: tc.pods[podIdx].RS,
				FromNode:   fromNode.Name,
				ToNode:     toNode.Name,
				MoveType:   moveType,
				CostImpact: costImpact,
			})
		}

		// Calculate objective changes
		initialObjectives := calculateObjectiveValues(currentAssignment, tc.nodes, currentPods, nodes, pods, tc.weightProfile)
		intermediateObjectives := calculateObjectiveValues(intermediateAssignment, tc.nodes, currentPods, nodes, pods, tc.weightProfile)
		targetObjectives := calculateObjectiveValues(bestSol.Assignment, tc.nodes, currentPods, nodes, pods, tc.weightProfile)

		// Calculate feasibility percentage with NaN protection
		feasibilityPercent := 0.0
		if bestSol.Movements > 0 {
			feasibilityPercent = (float64(len(feasibleMoveIndices)) / float64(bestSol.Movements)) * 100
		}
		if math.IsNaN(feasibilityPercent) || math.IsInf(feasibilityPercent, 0) {
			feasibilityPercent = 0.0
		}

		feasibleMovesAnalysis := FeasibleMovesAnalysis{
			TotalTargetMoves:   bestSol.Movements,
			FeasibleMoves:      len(feasibleMoveIndices),
			BlockedByPDB:       bestSol.Movements - len(feasibleMoveIndices),
			FeasibilityPercent: feasibilityPercent,
			MovedPods:          movedPods,
			ObjectiveChanges: ObjectiveChanges{
				InitialObjectives:      initialObjectives,
				IntermediateObjectives: intermediateObjectives,
				TargetObjectives:       targetObjectives,
			},
		}

		t.Logf("📊 Feasible moves analysis: %d/%d moves feasible (%.1f%%), blocked by PDB: %d",
			len(feasibleMoveIndices), bestSol.Movements, feasibleMovesAnalysis.FeasibilityPercent,
			feasibleMovesAnalysis.BlockedByPDB)
		t.Logf("💰 Intermediate cost impact: $%.2f → $%.2f → $%.2f (current → feasible → target)",
			initialCost, intermediateCost, finalCost)

		// Analyze movements
		movementAnalysis := analyzeMovements(currentAssignment, bestSol.Assignment, tc.nodes, tc.pods)

		// Create best solution in 3D format
		bestSolution3D := Solution3D{
			ID:            0,
			Assignment:    bestSol.Assignment,
			Cost:          bestSol.Cost,
			Disruption:    bestSol.Disruption,
			Balance:       bestSol.Balance,
			WeightedScore: bestSol.WeightedTotal,
			Movements:     bestSol.Movements,
			RawCost:       finalCost,
			RawDisruption: bestSol.RawDisruption,
			RawBalance:    finalBalance,
		}

		// Add round data to output
		roundData := OptimizationRound{
			Round:             round + 1,
			ParetoFront:       paretoFront,
			BestSolution:      bestSolution3D,
			InitialState:      initialState,
			IntermediateState: intermediateState,
			FinalState:        finalState,
			Improvements:      improvements,
			MovementAnalysis:  movementAnalysis,
			FeasibleMoves:     feasibleMovesAnalysis,
		}
		output.Rounds = append(output.Rounds, roundData)

		// Update current pod configuration for next round using ONLY feasible moves
		// This simulates the real descheduler behavior where only feasible moves are applied
		for i, newNode := range intermediateAssignment {
			currentPods[i].Node = newNode
		}

		// Replace with solutions from current round for next round seeding
		previousRoundSolutions = allSols

		t.Logf("🎯 Best solution impact: Cost $%.2f→$%.2f (Δ$%.2f), Balance %.1f%%→%.1f%% (Δ%.1f%%)",
			initialCost, finalCost, improvements.CostSavings, initialBalance, finalBalance, improvements.BalanceImprovement)
		t.Logf("🌱 Next round will seed with %d solutions from current round", len(previousRoundSolutions))

		// Check for stopping condition based on feasible movements
		actualMovements := len(feasibleMoveIndices)
		if actualMovements == 0 {
			consecutiveZeroMoves++
			t.Logf("⚠️  No feasible movements in this round (streak: %d/%d)", consecutiveZeroMoves, maxConsecutiveZeroMoves)

			if consecutiveZeroMoves >= maxConsecutiveZeroMoves {
				t.Logf("🛑 EARLY TERMINATION: %d consecutive rounds with 0 movements (PDB-blocked scenario)", maxConsecutiveZeroMoves)
				t.Logf("🛑 This simulates realistic descheduler behavior where PDB constraints prevent further optimization")
				round++ // Increment for final count
				break
			}
		} else {
			consecutiveZeroMoves = 0 // Reset counter on successful movements
			t.Logf("✅ %d feasible movements applied, continuing optimization", actualMovements)
		}

		round++ // Increment round counter
	}

	// Calculate final analysis and comparison metrics
	if len(output.Rounds) > 0 {
		finalRound := output.Rounds[len(output.Rounds)-1]
		trueInitialCost, trueInitialBalance := calculateActualMetricsEffective(initialClusterState, tc.nodes, tc.pods)

		// Determine convergence reason
		convergedReason := "Completed all planned rounds"
		if consecutiveZeroMoves >= maxConsecutiveZeroMoves {
			convergedReason = fmt.Sprintf("PDB-blocked: %d consecutive rounds with 0 movements", maxConsecutiveZeroMoves)
		}

		output.FinalResults = FinalAnalysis{
			TotalRounds:       len(output.Rounds),
			ConvergedAtRound:  len(output.Rounds), // Actual rounds completed
			ConvergenceReason: convergedReason,
			TotalCostSavings:  trueInitialCost - finalRound.FinalState.TotalCost,
			TotalBalanceGain:  trueInitialBalance - finalRound.FinalState.BalancePercent,
			FinalParetoSize:   len(finalRound.ParetoFront),
			OptimalSolution:   finalRound.BestSolution,
		}

		t.Logf("📊 OPTIMIZATION COMPLETED: %s", convergedReason)
		t.Logf("📊 Completed %d/%d planned rounds", len(output.Rounds), numRounds)

		// Calculate comparison metrics between NSGA-II and baselines
		output.ComparisonMetrics = calculateComparisonMetrics(finalRound.BestSolution, baselineResults, float64(tc.maxGenerations*tc.populationSize))

		// Log comparison summary
		t.Logf("\n🏆 ALGORITHM COMPARISON SUMMARY:")
		t.Logf("NSGA-II Best: Cost=$%.2f, Balance=%.1f%%, Score=%.4f",
			output.ComparisonMetrics.NSGAIIBest.RawCost,
			output.ComparisonMetrics.NSGAIIBest.RawBalance,
			output.ComparisonMetrics.NSGAIIBest.WeightedScore)
		t.Logf("Baseline Best (%s): Cost=$%.2f, Balance=%.1f%%, Score=%.4f",
			output.ComparisonMetrics.BaselineBest.Algorithm,
			output.ComparisonMetrics.BaselineBest.RawCost,
			output.ComparisonMetrics.BaselineBest.RawBalance,
			output.ComparisonMetrics.BaselineBest.WeightedScore)
		t.Logf("NSGA-II Improvement: %.2fx better weighted score, $%.2f cost improvement, %.1f%% balance improvement",
			output.ComparisonMetrics.ImprovementRatio,
			output.ComparisonMetrics.CostImprovement,
			output.ComparisonMetrics.BalanceImprovement)
	}

	// Create descriptive filename with weight profile and initialization method
	initMethod := "GCSH"
	if !tc.useGCSH {
		initMethod = "Random"
	}

	filename := fmt.Sprintf("optimization_results_%s_Cost%.1f_Disruption%.1f_Balance%.1f_%s_%s.json",
		strings.ReplaceAll(tc.name, " ", "_"),
		tc.weightProfile.Cost,
		tc.weightProfile.Disruption,
		tc.weightProfile.Balance,
		initMethod,
		timestamp)

	if err := writeJSONOutput(output, filename); err != nil {
		t.Errorf("Failed to write JSON output: %v", err)
	} else {
		t.Logf("\n📊 JSON output written to: %s", filename)
		t.Logf("🐍 Ready for Python visualization!")
	}

	// Enhanced function provides complete output - no need for original function
}

// Helper functions for JSON output
func createClusterState(assignment []int, nodeConfigs []NodeConfig, podConfigs []PodConfig, nodes []framework.NodeInfo) ClusterState {
	// Calculate node utilizations
	nodeUtils := make([]struct {
		NodeName   string  `json:"nodeName"`
		CPUPercent float64 `json:"cpuPercent"`
		MemPercent float64 `json:"memPercent"`
		PodCount   int     `json:"podCount"`
		HourlyCost float64 `json:"hourlyCost"`
	}, len(nodeConfigs))

	for i, nodeConfig := range nodeConfigs {
		cpuUsed := 0.0
		memUsed := 0.0
		podCount := 0

		for j, podNode := range assignment {
			if podNode == i {
				cpuUsed += podConfigs[j].CPU
				memUsed += podConfigs[j].Mem
				podCount++
			}
		}

		nodeUtils[i] = struct {
			NodeName   string  `json:"nodeName"`
			CPUPercent float64 `json:"cpuPercent"`
			MemPercent float64 `json:"memPercent"`
			PodCount   int     `json:"podCount"`
			HourlyCost float64 `json:"hourlyCost"`
		}{
			NodeName:   nodeConfig.Name,
			CPUPercent: (cpuUsed / nodeConfig.CPU) * 100,
			MemPercent: (memUsed / nodeConfig.Mem) * 100,
			PodCount:   podCount,
			HourlyCost: nodes[i].HourlyCost,
		}
	}

	totalCost, balancePercent := calculateActualMetricsEffective(assignment, nodeConfigs, podConfigs)

	return ClusterState{
		TotalCost:        totalCost,
		BalancePercent:   balancePercent,
		NodeUtilizations: nodeUtils,
	}
}

func analyzeMovements(initialAssignment, finalAssignment []int, nodeConfigs []NodeConfig, podConfigs []PodConfig) MovementAnalysis {
	movesByType := make(map[string]int)
	movesByRS := make(map[string]int)
	costOptimal := 0
	balanceOptimal := 0
	totalMoves := 0

	for i, finalNode := range finalAssignment {
		if finalNode != initialAssignment[i] {
			totalMoves++

			// Categorize by ReplicaSet
			rs := podConfigs[i].RS
			movesByRS[rs]++

			// Categorize by move type
			initialNodeConfig := nodeConfigs[initialAssignment[i]]
			finalNodeConfig := nodeConfigs[finalNode]

			moveType := categorizeMove(initialNodeConfig, finalNodeConfig)
			movesByType[moveType]++

			// Check if it's cost optimal (moving to cheaper node)
			if finalNodeConfig.Type != initialNodeConfig.Type || finalNodeConfig.Lifecycle != initialNodeConfig.Lifecycle {
				// Simplified cost check - could be more sophisticated
				if finalNodeConfig.Lifecycle == "spot" && initialNodeConfig.Lifecycle == "on-demand" {
					costOptimal++
				}
			}

			// Balance optimal moves are harder to detect without full context
			balanceOptimal++ // Simplified
		}
	}

	return MovementAnalysis{
		TotalMoves:     totalMoves,
		MovesByType:    movesByType,
		MovesByRS:      movesByRS,
		CostOptimal:    costOptimal,
		BalanceOptimal: balanceOptimal,
	}
}

func categorizeMove(from, to NodeConfig) string {
	if from.Lifecycle == "on-demand" && to.Lifecycle == "spot" {
		return "On-demand → Spot (cost saving)"
	} else if from.Lifecycle == "spot" && to.Lifecycle == "on-demand" {
		return "Spot → On-demand (reliability)"
	} else if from.Type != to.Type {
		return fmt.Sprintf("Instance type change (%s → %s)", from.Type, to.Type)
	} else {
		return "Same type migration"
	}
}

func writeJSONOutput(output OptimizationOutput, filename string) error {
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

// calculateFeasibleMovements determines which pods can actually be moved in the first iteration
// while respecting PDB constraints (maxUnavailable) - copied from production code
func calculateFeasibleMovements(targetAssignment []int, pods []framework.PodInfo) []int {
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

// calculateObjectiveValues calculates the actual objective values for a given assignment
func calculateObjectiveValues(assignment []int, nodeConfigs []NodeConfig, podConfigs []PodConfig, nodes []framework.NodeInfo, pods []framework.PodInfo, weights WeightProfile) ObjectiveValues {
	// Convert assignment to PodConfig for cost calculation
	tempPods := make([]PodConfig, len(podConfigs))
	copy(tempPods, podConfigs)
	for i, newNode := range assignment {
		tempPods[i].Node = newNode
	}

	// Calculate raw metrics with NaN protection
	rawCost, rawBalance := calculateActualMetricsEffective(assignment, nodeConfigs, tempPods)

	// Calculate raw disruption by counting movements and applying disruption logic
	rawDisruption := calculateRawDisruption(assignment, pods, podConfigs)

	// Protect against NaN values
	if math.IsNaN(rawCost) || math.IsInf(rawCost, 0) {
		rawCost = 0.0
	}
	if math.IsNaN(rawBalance) || math.IsInf(rawBalance, 0) {
		rawBalance = 0.0
	}
	if math.IsNaN(rawDisruption) || math.IsInf(rawDisruption, 0) {
		rawDisruption = 0.0
	}

	// Calculate normalized objectives with NaN protection
	normalizedCost := 0.0
	normalizedDisruption := 0.0
	normalizedBalance := rawBalance / 100.0 // Simplified normalization

	if math.IsNaN(normalizedBalance) || math.IsInf(normalizedBalance, 0) {
		normalizedBalance = 0.0
	}

	return ObjectiveValues{
		Cost:          normalizedCost,
		Disruption:    normalizedDisruption,
		Balance:       normalizedBalance,
		RawCost:       rawCost,
		RawDisruption: rawDisruption,
		RawBalance:    rawBalance,
	}
}

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

// KubernetesProblem implements framework.Problem for Kubernetes pod scheduling
type KubernetesProblem struct {
	nodes               []framework.NodeInfo
	pods                []framework.PodInfo
	costObjective       framework.ObjectiveFunc
	disruptionObjective framework.ObjectiveFunc
	balanceObjective    framework.ObjectiveFunc
	constraint          framework.Constraint
	maxPossibleCost     float64
	useGCSH             bool // Toggle between GCSH and random initialization
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
		variables[i] = pod.Node
	}

	return &framework.IntegerSolution{
		Variables: variables,
		Bounds:    bounds,
	}
}

func (kp *KubernetesProblem) Constraints() []framework.Constraint {
	return []framework.Constraint{kp.constraint}
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

// Initialize creates initial population based on useGCSH flag
func (kp *KubernetesProblem) Initialize(popSize int) []framework.Solution {
	if kp.useGCSH {
		// Use GCSH warm start initialization
		return kp.initializeWithGCSH(popSize)
	} else {
		// Use constraint-aware random initialization
		return kp.initializeRandom(popSize)
	}
}

// initializeWithGCSH uses the GCSH warm start heuristic
func (kp *KubernetesProblem) initializeWithGCSH(popSize int) []framework.Solution {
	objectives := kp.ObjectiveFuncs()
	constructionObjectives := []framework.ObjectiveFunc{
		objectives[0], // cost
		objectives[2], // balance
	}

	// Create GCSH configuration
	gcshConfig := warmstart.GCSHConfig{
		Pods:                kp.pods,
		Nodes:               kp.nodes,
		Objectives:          constructionObjectives,
		Constraints:         kp.Constraints(),
		IncludeCurrentState: true,
	}

	gcsh := warmstart.NewGCSH(gcshConfig)
	return gcsh.GenerateInitialPopulation(popSize)
}

// initializeRandom creates constraint-aware random solutions
func (kp *KubernetesProblem) initializeRandom(popSize int) []framework.Solution {
	fmt.Printf("\n🎯 RANDOM INITIALIZATION: Generating %d random solutions\n", popSize)
	solutions := make([]framework.Solution, popSize)

	// Include current state as first solution
	if popSize > 0 {
		vars := make([]int, len(kp.pods))
		bounds := make([]framework.IntBounds, len(kp.pods))
		for i, pod := range kp.pods {
			vars[i] = pod.Node
			bounds[i] = framework.IntBounds{L: 0, H: len(kp.nodes) - 1}
		}
		solutions[0] = framework.NewIntegerSolution(vars, bounds)
		fmt.Printf("Solution 0: Current state [%d, %d, %d, ...]\n",
			vars[0], min(len(vars)-1, vars[1]), min(len(vars)-1, vars[2]))
	}

	// Generate random constraint-aware solutions for the rest
	constraints := kp.Constraints()
	for i := 1; i < popSize; i++ {
		fmt.Printf("\nGenerating solution %d/%d:\n", i, popSize-1)
		solutions[i] = kp.generateRandomConstraintAwareSolution(constraints)
	}

	// Analyze uniqueness
	uniqueCount := kp.analyzeUniqueness(solutions)
	fmt.Printf("\n📊 DIVERSITY ANALYSIS: %d/%d solutions are unique (%.1f%%)\n",
		uniqueCount, popSize, float64(uniqueCount)/float64(popSize)*100)

	return solutions
}

// analyzeUniqueness counts how many unique solutions we have
func (kp *KubernetesProblem) analyzeUniqueness(solutions []framework.Solution) int {
	uniqueSolutions := make(map[string]bool)

	for i, sol := range solutions {
		intSol := sol.(*framework.IntegerSolution)
		// Create a string representation of the assignment
		key := fmt.Sprintf("%v", intSol.Variables)

		if uniqueSolutions[key] {
			fmt.Printf("   🔄 Solution %d is DUPLICATE of previous solution\n", i)
		} else {
			uniqueSolutions[key] = true
		}
	}

	return len(uniqueSolutions)
}

// generateRandomConstraintAwareSolution creates a single random solution that respects constraints
func (kp *KubernetesProblem) generateRandomConstraintAwareSolution(constraints []framework.Constraint) framework.Solution {
	vars := make([]int, len(kp.pods))
	bounds := make([]framework.IntBounds, len(kp.pods))

	for i := range vars {
		bounds[i] = framework.IntBounds{L: 0, H: len(kp.nodes) - 1}
	}

	maxAttempts := 100 // Maximum attempts to find a valid solution
	attemptCount := 0

	for attempt := 0; attempt < maxAttempts; attempt++ {
		attemptCount++

		// Generate constraint-aware random assignment using resource tracking
		solutionValid := true

		// Track node resource usage
		nodeUsage := make([]struct {
			cpuUsed float64
			memUsed float64
		}, len(kp.nodes))

		for i := range vars {
			podPlaced := false
			maxPodAttempts := len(kp.nodes) * 2 // Try multiple random nodes per pod
			pod := kp.pods[i]

			for podAttempt := 0; podAttempt < maxPodAttempts; podAttempt++ {
				nodeIdx := rand.Intn(len(kp.nodes))
				node := kp.nodes[nodeIdx]

				// Check if pod fits on this node
				if (nodeUsage[nodeIdx].cpuUsed+pod.CPURequest) <= node.CPUCapacity &&
					(nodeUsage[nodeIdx].memUsed+pod.MemRequest) <= node.MemCapacity {

					// Pod fits! Place it and update usage
					vars[i] = nodeIdx
					nodeUsage[nodeIdx].cpuUsed += pod.CPURequest
					nodeUsage[nodeIdx].memUsed += pod.MemRequest
					podPlaced = true

					break
				} else if attempt < 3 && i == 0 && podAttempt < 5 {
					fmt.Printf("       ❌ Node %d: Not enough resources (would need CPU=%.0f, Mem=%.0fMB)\n",
						nodeIdx, nodeUsage[nodeIdx].cpuUsed+pod.CPURequest,
						(nodeUsage[nodeIdx].memUsed+pod.MemRequest)/1e6)
				}
			}

			if !podPlaced {
				// Couldn't place this pod anywhere, this attempt fails
				solutionValid = false
				if attempt < 5 {
					fmt.Printf("   Attempt %d: Pod %d couldn't be placed after %d tries\n",
						attempt+1, i, maxPodAttempts)
				}
				break
			}
		}

		if solutionValid {
			// All pods placed successfully
			candidate := framework.NewIntegerSolution(vars, bounds)
			return candidate
		}
	}

	// Fallback: if no valid random solution found, use round-robin distribution
	fmt.Printf("   ⚠️ No valid random solution found after %d attempts, using fallback\n", maxAttempts)
	for i := range vars {
		vars[i] = i % len(kp.nodes)
	}

	fallbackSolution := framework.NewIntegerSolution(vars, bounds)

	// Check if fallback is valid
	fallbackValid := true
	for _, constraint := range constraints {
		if !constraint(fallbackSolution) {
			fallbackValid = false
			break
		}
	}

	fmt.Printf("   Fallback solution valid: %t\n", fallbackValid)
	fmt.Printf("   Fallback assignment sample: [%d, %d, %d, ...] (first 3 pods)\n",
		vars[0], min(len(vars)-1, vars[1]), min(len(vars)-1, vars[2]))

	return fallbackSolution
}

// createKubernetesProblem2 creates a MOO problem for pod scheduling with effective cost objective
func createKubernetesProblem2(nodes []framework.NodeInfo, pods []framework.PodInfo, weights WeightProfile) *KubernetesProblem {
	// Create objectives - using new effective cost objective
	effectiveCostObj := resourcecost.EffectiveCostObjectiveFunc(nodes, pods)

	// Convert for disruption objective
	disruptionPods := make([]framework.PodInfo, len(pods))
	for i, p := range pods {
		disruptionPods[i] = framework.PodInfo{
			Name:                   p.Name,
			Node:                   p.Node,
			ColdStartTime:          0.0, // Default 10s cold start
			ReplicaSetName:         p.ReplicaSetName,
			MaxUnavailableReplicas: p.MaxUnavailableReplicas,
		}
	}
	currentState := make([]int, len(pods))
	for i, p := range pods {
		currentState[i] = p.Node
	}
	disruptionConfig := disruption.NewDisruptionConfig(disruptionPods)
	disruptionObj := disruption.DisruptionObjective(currentState, disruptionPods, disruptionConfig)

	balanceConfig := balance.DefaultBalanceConfig()
	balanceObj := balance.BalanceObjectiveFunc(pods, nodes, balanceConfig)

	// Create constraints
	resourceConstraint := constraints.ResourceConstraint(pods, nodes)

	// Calculate max possible cost
	maxPossibleCost := 0.0
	for _, node := range nodes {
		maxPossibleCost += node.HourlyCost
	}

	return &KubernetesProblem{
		nodes:               nodes,
		pods:                pods,
		costObjective:       effectiveCostObj,
		disruptionObjective: disruptionObj,
		balanceObjective:    balanceObj,
		constraint:          resourceConstraint,
		maxPossibleCost:     maxPossibleCost,
		useGCSH:             true, // Default to GCSH, will be overridden by caller
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SeededProblem wraps a problem to provide seeded initialization with previous solutions
type SeededProblem struct {
	originalProblem   framework.Problem
	previousSolutions []Analysis
	populationSize    int
}

func createSeededProblemAllSolutions(original framework.Problem, previousSolutions []Analysis, populationSize int) *SeededProblem {
	return &SeededProblem{
		originalProblem:   original,
		previousSolutions: previousSolutions,
		populationSize:    populationSize,
	}
}

func (sp *SeededProblem) Name() string {
	return sp.originalProblem.Name() + "_Seeded"
}

func (sp *SeededProblem) ObjectiveFuncs() []framework.ObjectiveFunc {
	return sp.originalProblem.ObjectiveFuncs()
}

func (sp *SeededProblem) Constraints() []framework.Constraint {
	return sp.originalProblem.Constraints()
}

func (sp *SeededProblem) Bounds() []framework.Bounds {
	return sp.originalProblem.Bounds()
}

func (sp *SeededProblem) TrueParetoFront(size int) []framework.ObjectiveSpacePoint {
	return sp.originalProblem.TrueParetoFront(size)
}

func (sp *SeededProblem) Initialize(size int) []framework.Solution {
	solutions := make([]framework.Solution, 0, size)

	// Seed with ALL previous solutions (up to 70% of population for maximum diversity)
	seedCount := min(len(sp.previousSolutions), size*7/10)
	if seedCount > 0 {
		// Use ALL previous solutions up to the limit - no filtering!
		for i := 0; i < seedCount; i++ {
			// Convert Analysis to Solution
			sol := &framework.IntegerSolution{
				Variables: make([]int, len(sp.previousSolutions[i].Assignment)),
				Bounds:    make([]framework.IntBounds, len(sp.previousSolutions[i].Assignment)),
			}
			copy(sol.Variables, sp.previousSolutions[i].Assignment)

			// Get bounds from original problem
			originalBounds := sp.originalProblem.Bounds()
			for j := range sol.Bounds {
				if j < len(originalBounds) {
					sol.Bounds[j] = framework.IntBounds{
						L: int(originalBounds[j].L),
						H: int(originalBounds[j].H),
					}
				}
			}
			solutions = append(solutions, sol)
		}
	}

	// Fill remaining with random solutions for exploration (30%)
	remainingCount := size - len(solutions)
	if remainingCount > 0 {
		randomSolutions := sp.originalProblem.Initialize(remainingCount)
		solutions = append(solutions, randomSolutions...)
	}

	return solutions
}

// calculateActualMetricsEffective calculates actual dollar cost and balance percentage for a solution
func calculateActualMetricsEffective(assignment []int, nodes []NodeConfig, pods []PodConfig) (float64, float64) {
	// Calculate actual dollar cost
	activeNodes := make(map[int]bool)
	for _, nodeIdx := range assignment {
		if nodeIdx >= 0 && nodeIdx < len(nodes) {
			activeNodes[nodeIdx] = true
		}
	}

	totalCost := 0.0
	for nodeIdx := range activeNodes {
		// Get the hourly cost for this node
		c, err := cost.GetInstanceCost(nodes[nodeIdx].Region, nodes[nodeIdx].Type, nodes[nodeIdx].Lifecycle)
		if err == nil {
			totalCost += c
		}
	}

	// Calculate balance percentage using RESOURCE UTILIZATION (matching balance.go objective)
	// Calculate resource utilizations per node
	nodeResourceUtils := make([]struct {
		cpuUtil float64
		memUtil float64
	}, len(nodes))

	// Track resource usage per node
	nodeResourceUsage := make([]struct {
		cpuUsed float64
		memUsed float64
	}, len(nodes))

	// Calculate total resource usage per node
	for i, pod := range pods {
		nodeIdx := assignment[i]
		if nodeIdx >= 0 && nodeIdx < len(nodes) {
			nodeResourceUsage[nodeIdx].cpuUsed += pod.CPU
			nodeResourceUsage[nodeIdx].memUsed += pod.Mem
		}
	}

	// Calculate utilization percentages
	cpuUtilizations := make([]float64, 0, len(activeNodes))
	memUtilizations := make([]float64, 0, len(activeNodes))

	for nodeIdx := range activeNodes {
		if nodeIdx < len(nodes) {
			node := nodes[nodeIdx]
			cpuUtil := 0.0
			if node.CPU > 0 {
				cpuUtil = (nodeResourceUsage[nodeIdx].cpuUsed / node.CPU) * 100
			}
			memUtil := 0.0
			if node.Mem > 0 {
				memUtil = (nodeResourceUsage[nodeIdx].memUsed / node.Mem) * 100
			}

			nodeResourceUtils[nodeIdx].cpuUtil = cpuUtil
			nodeResourceUtils[nodeIdx].memUtil = memUtil
			cpuUtilizations = append(cpuUtilizations, cpuUtil)
			memUtilizations = append(memUtilizations, memUtil)
		}
	}

	// Calculate standard deviations for CPU and memory utilizations (matching balance.go)
	cpuStdDev := calculateStandardDeviation(cpuUtilizations)
	memStdDev := calculateStandardDeviation(memUtilizations)

	// Use weighted average with equal weights (matching DefaultBalanceConfig)
	balancePercent := (cpuStdDev + memStdDev) / 2.0

	return totalCost, balancePercent
}

// calculateStandardDeviation calculates the standard deviation of a slice of values (matching balance.go)
func calculateStandardDeviation(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Calculate mean
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	// Calculate variance
	variance := 0.0
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(values))

	// Return standard deviation
	return math.Sqrt(variance)
}

// BaselineAlgorithm represents different baseline scheduling algorithms for comparison
type BaselineAlgorithm int

const (
	BestFitDecreasing BaselineAlgorithm = iota
	FirstFitDecreasing
	RandomPlacement
	GreedyScheduler
)

// BaselineResult contains the result of a baseline algorithm run
type BaselineResult struct {
	Algorithm     string  `json:"algorithm"`
	Assignment    []int   `json:"assignment"`
	Cost          float64 `json:"cost"`
	Balance       float64 `json:"balance"`
	Disruption    float64 `json:"disruption"`
	WeightedScore float64 `json:"weightedScore"`
	Movements     int     `json:"movements"`
	ExecutionTime float64 `json:"executionTimeMs"`
	Feasible      bool    `json:"feasible"`
	RawCost       float64 `json:"rawCost"`
	RawBalance    float64 `json:"rawBalance"`
}

// runBaselineAlgorithm runs a specific baseline algorithm and returns the result
func runBaselineAlgorithm(algorithm BaselineAlgorithm, nodes []NodeConfig, pods []PodConfig, weights WeightProfile) BaselineResult {
	start := time.Now()

	var assignment []int
	var algorithmName string

	switch algorithm {
	case BestFitDecreasing:
		algorithmName = "Best Fit Decreasing"
		assignment = bestFitDecreasingScheduler(nodes, pods)
	case FirstFitDecreasing:
		algorithmName = "First Fit Decreasing"
		assignment = firstFitDecreasingScheduler(nodes, pods)
	case RandomPlacement:
		algorithmName = "Random Placement"
		assignment = randomPlacementScheduler(nodes, pods)
	case GreedyScheduler:
		algorithmName = "Greedy Cost Scheduler"
		assignment = greedyCostScheduler(nodes, pods)
	default:
		algorithmName = "Unknown"
		assignment = make([]int, len(pods))
	}

	executionTime := float64(time.Since(start).Nanoseconds()) / 1e6 // Convert to milliseconds

	// Check feasibility (all pods assigned to valid nodes with enough resources)
	feasible := checkAssignmentFeasibility(assignment, nodes, pods)

	// Calculate metrics
	rawCost, rawBalance := calculateActualMetricsEffective(assignment, nodes, pods)

	// Count movements from initial state
	movements := 0
	for i, newNode := range assignment {
		if newNode != pods[i].Node {
			movements++
		}
	}

	// Calculate normalized objectives (simplified - using raw values scaled)
	normalizedCost := rawCost / 1000.0                              // Scale cost for comparison
	normalizedBalance := rawBalance / 100.0                         // Scale balance percentage
	normalizedDisruption := float64(movements) / float64(len(pods)) // Movement ratio

	// Calculate weighted score
	weightedScore := weights.Cost*normalizedCost +
		weights.Disruption*normalizedDisruption +
		weights.Balance*normalizedBalance

	return BaselineResult{
		Algorithm:     algorithmName,
		Assignment:    assignment,
		Cost:          normalizedCost,
		Balance:       normalizedBalance,
		Disruption:    normalizedDisruption,
		WeightedScore: weightedScore,
		Movements:     movements,
		ExecutionTime: executionTime,
		Feasible:      feasible,
		RawCost:       rawCost,
		RawBalance:    rawBalance,
	}
}

// bestFitDecreasingScheduler implements Best Fit Decreasing algorithm
func bestFitDecreasingScheduler(nodes []NodeConfig, pods []PodConfig) []int {
	assignment := make([]int, len(pods))

	// Track resource usage per node
	nodeUsage := make([]struct {
		cpuUsed float64
		memUsed float64
	}, len(nodes))

	// Sort pods by resource requirements (decreasing order by CPU + Memory)
	podIndices := make([]int, len(pods))
	for i := range podIndices {
		podIndices[i] = i
	}

	sort.Slice(podIndices, func(i, j int) bool {
		podI := pods[podIndices[i]]
		podJ := pods[podIndices[j]]
		resourceI := podI.CPU + podI.Mem/1e6 // Convert memory to MB for comparison
		resourceJ := podJ.CPU + podJ.Mem/1e6
		return resourceI > resourceJ
	})

	// For each pod (in decreasing resource order), find the best fitting node
	for _, podIdx := range podIndices {
		pod := pods[podIdx]
		bestNode := -1
		bestWaste := math.Inf(1)

		// Try each node and pick the one with minimum waste (best fit)
		for nodeIdx, node := range nodes {
			// Check if pod fits
			if nodeUsage[nodeIdx].cpuUsed+pod.CPU <= node.CPU &&
				nodeUsage[nodeIdx].memUsed+pod.Mem <= node.Mem {

				// Calculate waste (remaining resources after placement)
				cpuWaste := node.CPU - (nodeUsage[nodeIdx].cpuUsed + pod.CPU)
				memWaste := node.Mem - (nodeUsage[nodeIdx].memUsed + pod.Mem)
				totalWaste := cpuWaste + memWaste/1e6 // Normalize memory

				if totalWaste < bestWaste {
					bestWaste = totalWaste
					bestNode = nodeIdx
				}
			}
		}

		// Assign to best node or fallback to first available
		if bestNode != -1 {
			assignment[podIdx] = bestNode
			nodeUsage[bestNode].cpuUsed += pod.CPU
			nodeUsage[bestNode].memUsed += pod.Mem
		} else {
			// Fallback: assign to first node (may exceed capacity)
			assignment[podIdx] = 0
		}
	}

	return assignment
}

// firstFitDecreasingScheduler implements First Fit Decreasing algorithm
func firstFitDecreasingScheduler(nodes []NodeConfig, pods []PodConfig) []int {
	assignment := make([]int, len(pods))

	// Track resource usage per node
	nodeUsage := make([]struct {
		cpuUsed float64
		memUsed float64
	}, len(nodes))

	// Sort pods by resource requirements (decreasing order)
	podIndices := make([]int, len(pods))
	for i := range podIndices {
		podIndices[i] = i
	}

	sort.Slice(podIndices, func(i, j int) bool {
		podI := pods[podIndices[i]]
		podJ := pods[podIndices[j]]
		resourceI := podI.CPU + podI.Mem/1e6
		resourceJ := podJ.CPU + podJ.Mem/1e6
		return resourceI > resourceJ
	})

	// For each pod, find the first node that fits
	for _, podIdx := range podIndices {
		pod := pods[podIdx]
		assigned := false

		// Try nodes in order until we find one that fits
		for nodeIdx, node := range nodes {
			if nodeUsage[nodeIdx].cpuUsed+pod.CPU <= node.CPU &&
				nodeUsage[nodeIdx].memUsed+pod.Mem <= node.Mem {

				assignment[podIdx] = nodeIdx
				nodeUsage[nodeIdx].cpuUsed += pod.CPU
				nodeUsage[nodeIdx].memUsed += pod.Mem
				assigned = true
				break
			}
		}

		// Fallback if no node fits
		if !assigned {
			assignment[podIdx] = 0
		}
	}

	return assignment
}

// randomPlacementScheduler implements random placement
func randomPlacementScheduler(nodes []NodeConfig, pods []PodConfig) []int {
	assignment := make([]int, len(pods))
	rand.Seed(42) // Fixed seed for reproducibility

	for i := range pods {
		assignment[i] = rand.Intn(len(nodes))
	}

	return assignment
}

// greedyCostScheduler implements greedy cost-based scheduling
func greedyCostScheduler(nodes []NodeConfig, pods []PodConfig) []int {
	assignment := make([]int, len(pods))

	// Track which nodes are used (for cost calculation)
	nodeUsed := make([]bool, len(nodes))

	// Sort nodes by cost (ascending - prefer cheaper nodes)
	nodeIndices := make([]int, len(nodes))
	for i := range nodeIndices {
		nodeIndices[i] = i
	}

	sort.Slice(nodeIndices, func(i, j int) bool {
		costI, _ := cost.GetInstanceCost(nodes[nodeIndices[i]].Region, nodes[nodeIndices[i]].Type, nodes[nodeIndices[i]].Lifecycle)
		costJ, _ := cost.GetInstanceCost(nodes[nodeIndices[j]].Region, nodes[nodeIndices[j]].Type, nodes[nodeIndices[j]].Lifecycle)
		return costI < costJ
	})

	// Track resource usage per node
	nodeUsage := make([]struct {
		cpuUsed float64
		memUsed float64
	}, len(nodes))

	// For each pod, try to place on the cheapest available node
	for podIdx, pod := range pods {
		assigned := false

		// Try nodes in cost order (cheapest first)
		for _, nodeIdx := range nodeIndices {
			node := nodes[nodeIdx]
			if nodeUsage[nodeIdx].cpuUsed+pod.CPU <= node.CPU &&
				nodeUsage[nodeIdx].memUsed+pod.Mem <= node.Mem {

				assignment[podIdx] = nodeIdx
				nodeUsage[nodeIdx].cpuUsed += pod.CPU
				nodeUsage[nodeIdx].memUsed += pod.Mem
				nodeUsed[nodeIdx] = true
				assigned = true
				break
			}
		}

		// Fallback if no node fits
		if !assigned {
			assignment[podIdx] = nodeIndices[0] // Use cheapest node
		}
	}

	return assignment
}

// checkAssignmentFeasibility checks if an assignment respects resource constraints
func checkAssignmentFeasibility(assignment []int, nodes []NodeConfig, pods []PodConfig) bool {
	// Track resource usage per node
	nodeUsage := make([]struct {
		cpuUsed float64
		memUsed float64
	}, len(nodes))

	// Calculate resource usage
	for i, nodeIdx := range assignment {
		if nodeIdx < 0 || nodeIdx >= len(nodes) {
			return false // Invalid node assignment
		}
		nodeUsage[nodeIdx].cpuUsed += pods[i].CPU
		nodeUsage[nodeIdx].memUsed += pods[i].Mem
	}

	// Check if any node exceeds capacity
	for i, node := range nodes {
		if nodeUsage[i].cpuUsed > node.CPU || nodeUsage[i].memUsed > node.Mem {
			return false
		}
	}

	return true
}

// runAllBaselineAlgorithms runs all baseline algorithms and returns their results
func runAllBaselineAlgorithms(nodes []NodeConfig, pods []PodConfig, weights WeightProfile) []BaselineResult {
	algorithms := []BaselineAlgorithm{
		BestFitDecreasing,
		FirstFitDecreasing,
		RandomPlacement,
		GreedyScheduler,
	}

	results := make([]BaselineResult, len(algorithms))
	for i, alg := range algorithms {
		results[i] = runBaselineAlgorithm(alg, nodes, pods, weights)
	}

	return results
}

// calculateComparisonMetrics calculates comparison metrics between NSGA-II and baseline algorithms
func calculateComparisonMetrics(nsgaiiBest Solution3D, baselineResults []BaselineResult, nsgaiiExecutionTimeMs float64) ComparisonMetrics {
	// Find the best baseline result by weighted score
	var bestBaseline BaselineResult
	bestWeightedScore := math.Inf(1)

	for _, result := range baselineResults {
		if result.WeightedScore < bestWeightedScore {
			bestWeightedScore = result.WeightedScore
			bestBaseline = result
		}
	}

	// Calculate improvement ratio (lower is better for weighted score)
	improvementRatio := 1.0
	if bestBaseline.WeightedScore > 0 {
		improvementRatio = bestBaseline.WeightedScore / nsgaiiBest.WeightedScore
	}

	// Calculate cost and balance improvements
	costImprovement := bestBaseline.RawCost - nsgaiiBest.RawCost
	balanceImprovement := bestBaseline.RawBalance - nsgaiiBest.RawBalance

	// Find fastest baseline algorithm
	fastestBaseline := ""
	fastestTime := math.Inf(1)
	for _, result := range baselineResults {
		if result.ExecutionTime < fastestTime {
			fastestTime = result.ExecutionTime
			fastestBaseline = result.Algorithm
		}
	}

	// Calculate speedup ratio (baseline is faster, so this will be > 1)
	speedupRatio := nsgaiiExecutionTimeMs / fastestTime

	return ComparisonMetrics{
		NSGAIIBest:         nsgaiiBest,
		BaselineBest:       bestBaseline,
		ImprovementRatio:   improvementRatio,
		CostImprovement:    costImprovement,
		BalanceImprovement: balanceImprovement,
		PerformanceComparison: PerformanceComparison{
			NSGAIIExecutionTime: nsgaiiExecutionTimeMs,
			FastestBaseline:     fastestBaseline,
			FastestBaselineTime: fastestTime,
			SpeedupRatio:        speedupRatio,
		},
	}
}

// NodeTemplate defines a node type with its characteristics
type NodeTemplate struct {
	Name         string
	CPU          float64 // millicores
	Mem          float64 // bytes
	InstanceType string
	Lifecycle    string
	Region       string
	CostPerHour  float64 // USD per hour
	Distribution float64 // Percentage of cluster (0.0-1.0)
}

// PodTemplate defines a workload type with its resource requirements
type PodTemplate struct {
	Name       string
	CPU        float64 // millicores
	Mem        float64 // bytes
	Count      int     // Number of pods of this type
	MaxUnavail int     // Max unavailable for PDB
}

// ClusterCapacity represents total cluster resource capacity
type ClusterCapacity struct {
	TotalCPU  float64
	TotalMem  float64
	NodeCount int
}

// generatePreciseNodes creates exactly nodeCount nodes with controlled heterogeneous distribution
func generatePreciseNodes(nodeCount int) ([]NodeConfig, ClusterCapacity) {
	// Define node templates with precise resource specifications
	templates := []NodeTemplate{
		// Small nodes (25% of cluster) - cost-effective for small workloads
		{"small-spot", 2000, 4e9, "t3.small", "spot", "us-east-1", 0.012, 0.125},
		{"small-od", 2000, 4e9, "t3.small", "on-demand", "us-east-1", 0.0208, 0.125},

		// Medium nodes (40% of cluster) - good general purpose
		{"medium-spot", 4000, 8e9, "m5.large", "spot", "us-east-1", 0.0288, 0.20},
		{"medium-od", 4000, 8e9, "m5.large", "on-demand", "us-east-1", 0.096, 0.20},

		// Large nodes (25% of cluster) - high capacity
		{"large-spot", 8000, 16e9, "m5.2xlarge", "spot", "us-east-1", 0.0576, 0.125},
		{"large-od", 8000, 16e9, "m5.2xlarge", "on-demand", "us-east-1", 0.192, 0.125},

		// XL nodes (10% of cluster) - very high capacity
		{"xl-spot", 16000, 32e9, "m5.4xlarge", "spot", "us-east-1", 0.1152, 0.05},
		{"xl-od", 16000, 32e9, "m5.4xlarge", "on-demand", "us-east-1", 0.384, 0.05},
	}

	nodes := make([]NodeConfig, 0, nodeCount)
	totalCPU := 0.0
	totalMem := 0.0

	nodeIdx := 0
	for _, template := range templates {
		count := int(math.Round(float64(nodeCount) * template.Distribution))
		// Ensure we don't exceed nodeCount and have at least 1 of each type for larger clusters
		if nodeCount >= 8 && count == 0 {
			count = 1
		}

		for i := 0; i < count && nodeIdx < nodeCount; i++ {
			node := NodeConfig{
				Name:      fmt.Sprintf("%s-%d", template.Name, nodeIdx+1),
				CPU:       template.CPU,
				Mem:       template.Mem,
				Type:      template.InstanceType,
				Lifecycle: template.Lifecycle,
				Region:    template.Region,
			}
			nodes = append(nodes, node)
			totalCPU += template.CPU
			totalMem += template.Mem
			nodeIdx++
		}
	}

	// Fill remaining slots with medium nodes if we're under nodeCount
	mediumTemplate := templates[2] // medium-spot
	for nodeIdx < nodeCount {
		node := NodeConfig{
			Name:      fmt.Sprintf("%s-%d", mediumTemplate.Name, nodeIdx+1),
			CPU:       mediumTemplate.CPU,
			Mem:       mediumTemplate.Mem,
			Type:      mediumTemplate.InstanceType,
			Lifecycle: mediumTemplate.Lifecycle,
			Region:    mediumTemplate.Region,
		}
		nodes = append(nodes, node)
		totalCPU += mediumTemplate.CPU
		totalMem += mediumTemplate.Mem
		nodeIdx++
	}

	capacity := ClusterCapacity{
		TotalCPU:  totalCPU,
		TotalMem:  totalMem,
		NodeCount: nodeCount,
	}

	return nodes, capacity
}

// generatePrecisePods creates pods that are mathematically guaranteed to fit within cluster capacity
func generatePrecisePods(capacity ClusterCapacity, targetUtilization float64) []PodConfig {
	// Calculate available resources based on target utilization
	availableCPU := capacity.TotalCPU * targetUtilization
	availableMem := capacity.TotalMem * targetUtilization

	// Define workload templates with realistic resource patterns
	workloadTemplates := []PodTemplate{
		// Web tier - many small pods (40% of CPU, 35% of Memory)
		{"web-frontend", 100, 256e6, 0, 1}, // Small web servers
		{"web-api", 200, 512e6, 0, 1},      // API services
		{"web-cache", 150, 1e9, 0, 1},      // Cache services

		// Application tier - medium pods (35% of CPU, 40% of Memory)
		{"app-service", 300, 1.5e9, 0, 2}, // Business logic
		{"app-worker", 250, 1e9, 0, 2},    // Background workers
		{"app-queue", 200, 800e6, 0, 1},   // Queue processors

		// Database tier - larger pods (20% of CPU, 20% of Memory)
		{"db-primary", 500, 4e9, 0, 1}, // Primary databases
		{"db-replica", 400, 3e9, 0, 1}, // Read replicas

		// System tier - small but essential (5% of CPU, 5% of Memory)
		{"monitoring", 50, 200e6, 0, 3}, // Monitoring services
		{"logging", 75, 300e6, 0, 2},    // Logging services
	}

	// Calculate resource allocation per tier
	tiers := []struct {
		name      string
		cpuPct    float64
		memPct    float64
		templates []int // indices into workloadTemplates
	}{
		{"web", 0.40, 0.35, []int{0, 1, 2}},
		{"app", 0.35, 0.40, []int{3, 4, 5}},
		{"db", 0.20, 0.20, []int{6, 7}},
		{"sys", 0.05, 0.05, []int{8, 9}},
	}

	pods := make([]PodConfig, 0)
	podIdx := 0

	rand.Seed(42) // For reproducible generation

	for _, tier := range tiers {
		tierCPU := availableCPU * tier.cpuPct
		tierMem := availableMem * tier.memPct

		// Distribute resources among templates in this tier
		templateCount := len(tier.templates)
		for _, templateIdx := range tier.templates {
			template := workloadTemplates[templateIdx]

			// Calculate how much resource this template gets (roughly equal split)
			templateCPU := tierCPU / float64(templateCount)
			templateMem := tierMem / float64(templateCount)

			// Calculate how many pods we can fit
			maxPodsByCPU := int(templateCPU / template.CPU)
			maxPodsByMem := int(templateMem / template.Mem)
			maxPods := int(math.Min(float64(maxPodsByCPU), float64(maxPodsByMem)))

			// Ensure we have at least 1 pod of each type, but not too many
			if maxPods < 1 {
				maxPods = 1
			}
			if maxPods > capacity.NodeCount*2 { // Don't exceed 2 pods per node of any type
				maxPods = capacity.NodeCount * 2
			}

			// Create pods of this type
			for j := 0; j < maxPods; j++ {
				// Assign to random node initially (will be optimized)
				nodeIdx := rand.Intn(capacity.NodeCount)

				pods = append(pods, PodConfig{
					Name:       fmt.Sprintf("%s-%d", template.Name, j+1),
					CPU:        template.CPU,
					Mem:        template.Mem,
					Node:       nodeIdx,
					RS:         template.Name,
					MaxUnavail: template.MaxUnavail,
				})
				podIdx++
			}
		}
	}

	return pods
}

// createPreciseTestCase creates a test case with mathematically controlled resource allocation
func createPreciseTestCase(name string, nodeCount int, targetUtilization float64, weights WeightProfile, popSize, generations int) struct {
	name             string
	nodes            []NodeConfig
	pods             []PodConfig
	weightProfile    WeightProfile
	populationSize   int
	maxGenerations   int
	expectedBehavior string
	useGCSH          bool
} {
	nodes, capacity := generatePreciseNodes(nodeCount)
	pods := generatePrecisePods(capacity, targetUtilization)

	return struct {
		name             string
		nodes            []NodeConfig
		pods             []PodConfig
		weightProfile    WeightProfile
		populationSize   int
		maxGenerations   int
		expectedBehavior string
		useGCSH          bool
	}{
		name:             name,
		nodes:            nodes,
		pods:             pods,
		weightProfile:    weights,
		populationSize:   popSize,
		maxGenerations:   generations,
		expectedBehavior: fmt.Sprintf("Precise cluster - %d nodes, %.0f%% utilization, guaranteed resource fit", nodeCount, targetUtilization*100),
		useGCSH:          true,
	}
}

// generateMassiveRealisticNodes creates 500 nodes with diverse instance types and costs
func generateMassiveRealisticNodes() []NodeConfig {
	return generateScalableRealisticNodes(500) // Default to 500 for backward compatibility
}

// generateScalableRealisticNodes creates a realistic heterogeneous cluster with the specified number of nodes
func generateScalableRealisticNodes(nodeCount int) []NodeConfig {
	nodes := make([]NodeConfig, 0, nodeCount)

	// Node type distributions for realistic cloud environments (scalable by nodeCount)
	nodeTypes := []struct {
		name         string
		cpu          float64
		mem          float64
		instanceType string
		lifecycle    string
		region       string
		percentage   float64 // Percentage of total cluster
	}{
		// Small instances (30% of cluster) - cheap but limited capacity
		{"small", 2000, 4e9, "t3.small", "spot", "us-east-1", 0.15},
		{"small", 2000, 4e9, "t3.small", "on-demand", "us-east-1", 0.15},

		// Medium instances (40% of cluster) - good balance
		{"medium", 4000, 8e9, "m5.large", "spot", "us-east-1", 0.16},
		{"medium", 4000, 8e9, "m5.large", "on-demand", "us-east-1", 0.16},
		{"medium", 4000, 16e9, "m5.xlarge", "spot", "us-east-1", 0.08},

		// Large instances (20% of cluster) - expensive but high capacity
		{"large", 8000, 16e9, "m5.2xlarge", "spot", "us-east-1", 0.06},
		{"large", 8000, 16e9, "m5.2xlarge", "on-demand", "us-east-1", 0.06},
		{"large", 8000, 32e9, "m5.4xlarge", "spot", "us-east-1", 0.04},
		{"large", 8000, 32e9, "m5.4xlarge", "on-demand", "us-east-1", 0.04},

		// Compute optimized (5% of cluster) - high CPU, expensive
		{"compute", 16000, 16e9, "c5.4xlarge", "spot", "us-east-1", 0.03},
		{"compute", 16000, 16e9, "c5.4xlarge", "on-demand", "us-east-1", 0.02},

		// Memory optimized (5% of cluster) - high memory, very expensive
		{"memory", 4000, 64e9, "r5.2xlarge", "spot", "us-east-1", 0.03},
		{"memory", 4000, 64e9, "r5.2xlarge", "on-demand", "us-east-1", 0.02},
	}

	nodeIdx := 0
	for _, nodeType := range nodeTypes {
		count := int(math.Ceil(float64(nodeCount) * nodeType.percentage))
		if count == 0 && nodeType.percentage > 0 {
			count = 1 // Ensure at least one node of each type for small clusters
		}
		for i := 0; i < count; i++ {
			nodes = append(nodes, NodeConfig{
				Name:      fmt.Sprintf("%s-%d", nodeType.name, nodeIdx+1),
				CPU:       nodeType.cpu,
				Mem:       nodeType.mem,
				Type:      nodeType.instanceType,
				Region:    nodeType.region,
				Lifecycle: nodeType.lifecycle,
			})
			nodeIdx++
		}
	}

	return nodes
}

// generateMassiveRealisticPods creates pods with 50-60% cluster utilization and realistic replica sets
func generateMassiveRealisticPods() []PodConfig {
	return generateScalableRealisticPods(500, 0.35) // Default: 500 nodes at 35% target utilization
}

// generateScalableRealisticPods creates realistic workloads sized for the given cluster
// nodeCount: number of nodes in the cluster
// targetUtilization: target cluster utilization (0.35 = 35% to avoid over-packing)
func generateScalableRealisticPods(nodeCount int, targetUtilization float64) []PodConfig {
	pods := make([]PodConfig, 0, nodeCount*50) // Estimate ~50 pods per node max

	// Calculate resource capacity for scaling
	// Approximate average node capacity (weighted average of different node types)
	// avgNodeCPU := 6000.0 // ~6 cores average (mix of 2, 4, 8, 16, 32 core nodes)
	// avgNodeMem := 12e9   // ~12GB average (mix of 4GB to 128GB nodes)

	// Calculate total available resources for capacity planning
	// totalClusterCPU := float64(nodeCount) * avgNodeCPU * targetUtilization
	// totalClusterMem := float64(nodeCount) * avgNodeMem * targetUtilization

	// Realistic workload patterns (scaled by cluster size)
	workloads := []struct {
		name            string
		cpuRequest      float64
		memRequest      float64
		replicasPerNode float64 // Replicas per node (will be scaled)
		maxUnavailPct   float64 // Max unavailable as percentage
		distribution    string  // How to distribute across nodes
	}{
		// Web tier workloads (many small replicas, ~1-2 per node)
		{"web-frontend", 200, 512e6, 1.0, 0.20, "spread"},
		{"web-api", 300, 1e9, 0.75, 0.20, "spread"},
		{"web-auth", 250, 768e6, 0.6, 0.25, "spread"},
		{"web-gateway", 400, 1.5e9, 0.4, 0.25, "spread"},
		{"web-cdn", 150, 256e6, 0.9, 0.22, "spread"},
		{"web-proxy", 180, 384e6, 0.8, 0.25, "spread"},

		// Application tier (medium sized workloads, ~0.5-1 per node)
		{"app-users", 500, 2e9, 0.9, 0.22, "mixed"},
		{"app-orders", 600, 2.5e9, 0.75, 0.20, "mixed"},
		{"app-inventory", 400, 1.8e9, 0.6, 0.25, "mixed"},
		{"app-payments", 800, 3e9, 0.5, 0.20, "mixed"},
		{"app-notifications", 300, 1e9, 1.0, 0.25, "mixed"},
		{"app-search", 700, 2.2e9, 0.7, 0.21, "mixed"},
		{"app-recommendations", 900, 3.5e9, 0.4, 0.25, "mixed"},
		{"app-analytics", 1200, 4e9, 0.3, 0.17, "mixed"},

		// Database tier (fewer, larger workloads, ~0.1-0.3 per node)
		{"db-primary", 1500, 8e9, 0.3, 0.17, "concentrated"},
		{"db-replica", 1200, 6e9, 0.6, 0.17, "concentrated"},
		{"db-cache", 800, 4e9, 0.75, 0.20, "concentrated"},
		{"db-analytics", 2000, 12e9, 0.2, 0.25, "concentrated"},
		{"db-timeseries", 1000, 6e9, 0.4, 0.13, "concentrated"},

		// Background processing (medium utilization)
		{"worker-queue", 600, 2e9, 0.8, 0.25, "mixed"},
		{"worker-batch", 1000, 4e9, 0.4, 0.25, "mixed"},
		{"worker-ml", 2500, 8e9, 0.3, 0.17, "concentrated"},
		{"worker-etl", 800, 3e9, 0.5, 0.20, "mixed"},

		// Monitoring and system services (spread across cluster)
		{"monitor-metrics", 300, 1.5e9, 0.5, 0.20, "spread"},
		{"monitor-logs", 400, 2e9, 0.4, 0.25, "spread"},
		{"monitor-traces", 350, 1.8e9, 0.3, 0.17, "spread"},
		{"monitor-alerts", 200, 1e9, 0.6, 0.25, "spread"},

		// Microservices (many small services)
		{"micro-auth", 150, 512e6, 0.6, 0.25, "spread"},
		{"micro-config", 100, 256e6, 0.4, 0.25, "spread"},
		{"micro-events", 200, 768e6, 0.5, 0.20, "spread"},
		{"micro-files", 250, 1e9, 0.3, 0.17, "spread"},
		{"micro-chat", 180, 640e6, 0.7, 0.21, "spread"},
		{"micro-email", 220, 896e6, 0.5, 0.20, "spread"},

		// Development and testing workloads (lower priority)
		{"dev-webapp", 300, 1e9, 0.4, 0.38, "mixed"},
		{"test-runner", 500, 2e9, 0.2, 0.50, "mixed"},
		{"staging-api", 400, 1.5e9, 0.3, 0.33, "mixed"},
		{"dev-db", 600, 3e9, 0.2, 0.25, "mixed"},
	}

	rand.Seed(42) // For reproducible generation
	podIdx := 0

	for workloadIdx, workload := range workloads {
		replicas := int(math.Ceil(float64(nodeCount) * workload.replicasPerNode))
		if replicas == 0 && workload.replicasPerNode > 0 {
			replicas = 1 // Ensure at least one replica for small clusters
		}
		maxUnavail := int(math.Ceil(float64(replicas) * workload.maxUnavailPct))
		if maxUnavail == 0 && replicas > 1 {
			maxUnavail = 1
		}

		for replica := 0; replica < replicas; replica++ {
			// Determine initial placement based on distribution strategy
			var nodeIdx int

			switch workload.distribution {
			case "spread":
				// Spread evenly across all nodes
				nodeIdx = (podIdx * 7) % nodeCount // Prime multiplier for better spread
			case "mixed":
				// Mix of spread and some clustering
				if replica < replicas/2 {
					nodeIdx = (podIdx * 11) % nodeCount
				} else {
					// Cluster some replicas together for balance optimization opportunities
					clusterNodes := nodeCount - 10
					if clusterNodes < 1 {
						clusterNodes = nodeCount
					}
					baseNode := (workloadIdx * 23) % clusterNodes
					nodeIdx = baseNode + (replica % 10)
				}
			case "concentrated":
				// Concentrate on fewer, larger nodes (creates balance optimization opportunities)
				baseNode := (workloadIdx * 37) % (nodeCount / 4) // Use only 25% of nodes
				if baseNode == 0 && nodeCount >= 4 {
					baseNode = 1
				}
				nodeIdx = baseNode + (replica % 5)

				// Sometimes place on expensive nodes for cost optimization opportunities
				if replica%3 == 0 && nodeCount > 20 {
					// Place on last 20% of nodes (typically the expensive ones)
					expensiveStart := int(float64(nodeCount) * 0.8)
					nodeIdx = expensiveStart + rand.Intn(nodeCount-expensiveStart)
				}
			}

			// Ensure node index is valid
			nodeIdx = nodeIdx % nodeCount

			pods = append(pods, PodConfig{
				Name:       fmt.Sprintf("%s-%d", workload.name, replica+1),
				CPU:        workload.cpuRequest,
				Mem:        workload.memRequest,
				Node:       nodeIdx,
				RS:         workload.name,
				MaxUnavail: maxUnavail,
			})
			podIdx++
		}
	}

	// Add some additional random workloads to reach target utilization
	additionalWorkloads := []struct {
		prefix     string
		cpuRange   [2]float64
		memRange   [2]float64
		maxUnavail int
		count      int
	}{
		{"service", [2]float64{150, 500}, [2]float64{512e6, 2e9}, 3, 200},
		{"job", [2]float64{300, 800}, [2]float64{1e9, 3e9}, 2, 150},
		{"daemon", [2]float64{100, 300}, [2]float64{256e6, 1e9}, 5, 100},
		{"cron", [2]float64{200, 600}, [2]float64{768e6, 2.5e9}, 4, 80},
	}

	for _, additional := range additionalWorkloads {
		replicaSetSize := 5 + rand.Intn(15) // 5-20 replicas per RS
		numReplicaSets := additional.count / replicaSetSize

		for rs := 0; rs < numReplicaSets; rs++ {
			rsName := fmt.Sprintf("%s-rs-%d", additional.prefix, rs+1)

			for replica := 0; replica < replicaSetSize; replica++ {
				cpuRequest := additional.cpuRange[0] + rand.Float64()*(additional.cpuRange[1]-additional.cpuRange[0])
				memRequest := additional.memRange[0] + rand.Float64()*(additional.memRange[1]-additional.memRange[0])

				// Strategic placement to create optimization opportunities
				var nodeIdx int
				if replica < replicaSetSize/3 {
					// 1/3 on expensive nodes (cost optimization opportunity)
					// 1/3 on expensive nodes for cost optimization (last 20% of nodes)
					if nodeCount > 20 {
						expensiveStart := int(float64(nodeCount) * 0.8)
						nodeIdx = expensiveStart + rand.Intn(nodeCount-expensiveStart)
					} else {
						nodeIdx = rand.Intn(nodeCount)
					}
				} else if replica < 2*replicaSetSize/3 {
					// 1/3 clustered together (balance optimization opportunity)
					maxBase := nodeCount / 4
					if maxBase < 1 {
						maxBase = 1
					}
					baseNode := rand.Intn(maxBase)
					nodeIdx = baseNode + (replica % 5)
				} else {
					// 1/3 spread randomly (mixed)
					nodeIdx = rand.Intn(nodeCount)
				}

				nodeIdx = nodeIdx % nodeCount

				pods = append(pods, PodConfig{
					Name:       fmt.Sprintf("%s-%d", rsName, replica+1),
					CPU:        cpuRequest,
					Mem:        memRequest,
					Node:       nodeIdx,
					RS:         rsName,
					MaxUnavail: additional.maxUnavail,
				})
				podIdx++
			}
		}
	}

	return pods
}

// calculateRawDisruption calculates the actual disruption value (not normalized)
// This should match the actual disruption objective calculation
func calculateRawDisruption(assignment []int, pods []framework.PodInfo, podConfigs []PodConfig) float64 {
	// Count total movements
	totalMovements := 0
	for i, newNode := range assignment {
		if newNode != pods[i].Node {
			totalMovements++
		}
	}

	// If no movements, no disruption
	if totalMovements == 0 {
		return 0.0
	}

	// Calculate ReplicaSet-weighted disruption (matching the production algorithm)
	replicaSets := make(map[string]*struct {
		totalPods int
		movedPods int
	})

	// Group pods by ReplicaSet and count movements
	for i, pod := range pods {
		rsName := pod.ReplicaSetName
		if replicaSets[rsName] == nil {
			replicaSets[rsName] = &struct {
				totalPods int
				movedPods int
			}{}
		}
		rs := replicaSets[rsName]
		rs.totalPods++

		if assignment[i] != pod.Node {
			rs.movedPods++
		}
	}

	// Calculate weighted average disruption across ReplicaSets
	totalWeight := 0.0
	weightedDisruption := 0.0

	for _, rs := range replicaSets {
		if rs.totalPods == 0 {
			continue
		}

		// Movement ratio for this ReplicaSet
		rsMovementRatio := float64(rs.movedPods) / float64(rs.totalPods)

		// Weight by ReplicaSet size
		weight := float64(rs.totalPods)
		weightedDisruption += rsMovementRatio * weight
		totalWeight += weight
	}

	if totalWeight > 0 {
		return weightedDisruption / totalWeight
	}

	return 0.0
}
