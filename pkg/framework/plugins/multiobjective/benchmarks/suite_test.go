package benchmarks

import (
	"testing"

	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/algorithms"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
)

func TestBenchmarkSuite(t *testing.T) {
	config := algorithms.NSGA2Config{
		PopulationSize:       200,
		MaxGenerations:       500,
		CrossoverProbability: 0.9,
		MutationProbability:  1.0 / 30.0, // 1/n for n variables
		TournamentSize:       2,
	}

	suite := NewTestSuite(config)
	suite.AddStandardProblems()

	err := suite.Run("./results")
	if err != nil {
		t.Fatalf("Failed to run benchmark suite: %v", err)
	}
}

func TestIndividualBenchmarks(t *testing.T) {
	tests := []struct {
		name    string
		problem framework.Problem
		config  algorithms.NSGA2Config
	}{
		{
			name:    "ZDT1",
			problem: NewZDT1(30),
			config: algorithms.NSGA2Config{
				PopulationSize:       100,
				MaxGenerations:       250,
				CrossoverProbability: 0.9,
				MutationProbability:  1.0 / 30.0,
				TournamentSize:       2,
			},
		},
		{
			name:    "ZDT2",
			problem: NewZDT2(30),
			config: algorithms.NSGA2Config{
				PopulationSize:       100,
				MaxGenerations:       250,
				CrossoverProbability: 0.9,
				MutationProbability:  1.0 / 30.0,
				TournamentSize:       2,
			},
		},
		{
			name:    "DTLZ2_3obj",
			problem: NewDTLZ2(13, 3),
			config: algorithms.NSGA2Config{
				PopulationSize:       200,
				MaxGenerations:       300,
				CrossoverProbability: 0.9,
				MutationProbability:  1.0 / 13.0,
				TournamentSize:       2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nsga2 := algorithms.NewNSGAII(tt.config, tt.problem)
			finalPop := nsga2.Run()

			if len(finalPop) == 0 {
				t.Errorf("Final population is empty")
			}

			// Extract and verify Pareto front
			paretoFront := algorithms.GetParetoFront(finalPop, tt.problem)
			t.Logf("%s: Found %d solutions in Pareto front",
				tt.name, len(paretoFront))

			// For problems with known true fronts, calculate IGD
			trueFront := tt.problem.TrueParetoFront(500)
			if trueFront != nil {
				igd := calculateIGD(paretoFront, trueFront)
				t.Logf("%s: IGD = %.6f", tt.name, igd)

				// Check if IGD is reasonable (problem-specific thresholds)
				maxIGD := 0.1 // Adjust based on problem
				if igd > maxIGD {
					t.Errorf("%s: IGD %.6f exceeds threshold %.6f",
						tt.name, igd, maxIGD)
				}
			}
		})
	}
}

func calculateIGD(obtained, trueFront []framework.ObjectiveSpacePoint) float64 {
	igd := 0.0
	for _, truePoint := range trueFront {
		minDist := 1e10
		for _, obtPoint := range obtained {
			dist := 0.0
			for i := range truePoint {
				diff := truePoint[i] - obtPoint[i]
				dist += diff * diff
			}
			if dist < minDist {
				minDist = dist
			}
		}
		igd += minDist
	}
	return igd / float64(len(trueFront))
}
