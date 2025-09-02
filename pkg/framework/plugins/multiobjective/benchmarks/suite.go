package benchmarks

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/algorithms"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/util"
)

// TestSuite runs a set of benchmark problems
type TestSuite struct {
	problems  []framework.Problem
	algorithm framework.Algorithm
	config    algorithms.NSGA2Config
}

// NewTestSuite creates a new benchmark test suite
func NewTestSuite(config algorithms.NSGA2Config) *TestSuite {
	return &TestSuite{
		config: config,
	}
}

// AddProblem adds a problem to the test suite
func (ts *TestSuite) AddProblem(p framework.Problem) {
	ts.problems = append(ts.problems, p)
}

// AddStandardProblems adds common benchmark problems
func (ts *TestSuite) AddStandardProblems() {
	// ZDT problems with 30 variables (standard)
	ts.AddProblem(NewZDT1(30))
	ts.AddProblem(NewZDT2(30))
	ts.AddProblem(NewZDT3(30))

	// DTLZ problems
	// 2 objectives, 7 variables (M + k - 1, where k=5 for DTLZ1)
	ts.AddProblem(NewDTLZ1(7, 2))
	// 2 objectives, 12 variables (M + k - 1, where k=10 for DTLZ2)
	ts.AddProblem(NewDTLZ2(12, 2))

	// 3 objectives versions
	ts.AddProblem(NewDTLZ1(8, 3))
	ts.AddProblem(NewDTLZ2(13, 3))
}

// Run executes the test suite
func (ts *TestSuite) Run(outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	for _, problem := range ts.problems {
		log.Printf("Running NSGA-II on %s...", problem.Name())

		// Run NSGA-II
		nsga2 := algorithms.NewNSGAII(ts.config, problem)
		finalPop := nsga2.Run()

		// Extract Pareto front
		paretoFront := algorithms.GetParetoFront(finalPop, problem)

		// Generate output files
		outputFile := filepath.Join(outputDir, fmt.Sprintf("%s_NSGA-II_results", problem.Name()))

		// For 2D problems, create plots
		if len(problem.ObjectiveFuncs()) == 2 {
			plotFile := outputFile + ".html"
			err := util.PlotResults(paretoFront, problem, "NSGA-II", plotFile)
			if err != nil {
				log.Printf("Failed to plot results for %s: %v", problem.Name(), err)
			}
		}

		// Calculate metrics if true front is available
		trueFront := problem.TrueParetoFront(500)
		if trueFront != nil {
			metrics := ts.calculateMetrics(paretoFront, trueFront)
			log.Printf("%s - Hypervolume: %.4f, IGD: %.4f",
				problem.Name(), metrics.hypervolume, metrics.igd)
		}
	}

	return nil
}

type metrics struct {
	hypervolume float64
	igd         float64 // Inverted Generational Distance
}

func (ts *TestSuite) calculateMetrics(obtained, trueFront []framework.ObjectiveSpacePoint) metrics {
	// Simple IGD calculation (average distance from true front to obtained front)
	igd := 0.0
	for _, truePoint := range trueFront {
		minDist := 1e10
		for _, obtPoint := range obtained {
			dist := euclideanDistance(truePoint, obtPoint)
			if dist < minDist {
				minDist = dist
			}
		}
		igd += minDist
	}
	igd /= float64(len(trueFront))

	// Hypervolume calculation would go here (complex for many objectives)
	// For now, return a placeholder
	return metrics{
		hypervolume: -1, // Not implemented
		igd:         igd,
	}
}

func euclideanDistance(a, b framework.ObjectiveSpacePoint) float64 {
	sum := 0.0
	for i := range a {
		diff := a[i] - b[i]
		sum += diff * diff
	}
	return sum // Return squared distance to save computation
}
