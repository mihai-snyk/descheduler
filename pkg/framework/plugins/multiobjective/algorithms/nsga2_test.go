package algorithms_test

import (
	"testing"

	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/algorithms"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/benchmarks"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/util"
)

// Test problem: ZDT1 benchmark function
func TestNSGAIIWithZDT1(t *testing.T) {
	numVars := 30

	// Create the ZDT1 problem instance
	zdt1 := benchmarks.NewZDT1(numVars)

	// Configure NSGA-II
	config := algorithms.NSGA2Config{
		PopulationSize:       100,
		MaxGenerations:       250,
		CrossoverProbability: 0.9,
		MutationProbability:  1.0 / float64(numVars),
		TournamentSize:       2,
	}

	// Create NSGA-II instance
	nsga := algorithms.NewNSGAII(config, zdt1)

	// Run algorithm
	finalPop := nsga.Run()

	// Basic validation
	if len(finalPop) != config.PopulationSize {
		t.Errorf("Expected population size %d, got %d", config.PopulationSize, len(finalPop))
	}

	// Verify Pareto front characteristics
	fronts := algorithms.NonDominatedSort(finalPop)
	if len(fronts) == 0 {
		t.Error("No fronts found in final population")
	}

	firstFront := fronts[0]
	results := make([]framework.ObjectiveSpacePoint, len(firstFront))
	for i := range len(firstFront) {
		results[i] = firstFront[i].Value
	}
	err := util.PlotResults(results, zdt1, "NSGA-II") // Uses default location
	if err != nil {
		t.Errorf("Plot failed: %v", err)
	}

	// Check if first front is non-dominated
	for i := 0; i < len(firstFront); i++ {
		for j := 0; j < len(firstFront); j++ {
			if i != j && algorithms.Dominates(firstFront[i], firstFront[j]) {
				t.Error("First front contains dominated solutions")
			}
		}
	}
}
