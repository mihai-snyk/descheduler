package algorithms

import (
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
)

// GetParetoFront extracts the Pareto front (first non-dominated front) from a population
func GetParetoFront(population []*NSGAIISolution, problem framework.Problem) []framework.ObjectiveSpacePoint {
	if len(population) == 0 {
		return nil
	}

	// Get all fronts
	fronts := NonDominatedSort(population)
	if len(fronts) == 0 || len(fronts[0]) == 0 {
		return nil
	}

	// Extract objective values from first front
	paretoFront := make([]framework.ObjectiveSpacePoint, len(fronts[0]))
	for i, sol := range fronts[0] {
		point := make(framework.ObjectiveSpacePoint, len(problem.ObjectiveFuncs()))
		for j, objFunc := range problem.ObjectiveFuncs() {
			point[j] = objFunc(sol.Solution)
		}
		paretoFront[i] = point
	}

	return paretoFront
}

// ConvertToNSGAIISolutions converts generic Solutions to NSGAIISolutions
func ConvertToNSGAIISolutions(solutions []framework.Solution) []*NSGAIISolution {
	nsga2Sols := make([]*NSGAIISolution, len(solutions))
	for i, sol := range solutions {
		nsga2Sols[i] = &NSGAIISolution{
			Solution: sol,
			Rank:     0,
			Distance: 0,
		}
	}
	return nsga2Sols
}
