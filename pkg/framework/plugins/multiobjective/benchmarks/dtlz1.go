package benchmarks

import (
	"math"
	"math/rand/v2"

	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
)

// DTLZ1 is scalable to any number of objectives
// It has a linear Pareto front and many local fronts
type DTLZ1 struct {
	numVars       int
	numObjectives int
}

func NewDTLZ1(numVars, numObjectives int) *DTLZ1 {
	// Recommended: numVars = numObjectives + k - 1, where k = 5 for DTLZ1
	return &DTLZ1{
		numVars:       numVars,
		numObjectives: numObjectives,
	}
}

func (p *DTLZ1) Name() string {
	return "DTLZ1"
}

func (p *DTLZ1) ObjectiveFuncs() []framework.ObjectiveFunc {
	funcs := make([]framework.ObjectiveFunc, p.numObjectives)
	for i := 0; i < p.numObjectives; i++ {
		idx := i // Capture loop variable
		funcs[i] = func(x framework.Solution) float64 {
			return p.objective(x, idx)
		}
	}
	return funcs
}

func (p *DTLZ1) g(x []float64) float64 {
	k := p.numVars - p.numObjectives + 1
	sum := 0.0
	for i := p.numObjectives - 1; i < p.numVars; i++ {
		sum += math.Pow(x[i]-0.5, 2) - math.Cos(20*math.Pi*(x[i]-0.5))
	}
	return 100 * (float64(k) + sum)
}

func (p *DTLZ1) objective(sol framework.Solution, objIdx int) float64 {
	x := sol.(*framework.RealSolution).Variables
	g := p.g(x)

	f := 0.5 * (1 + g)
	for i := 0; i < p.numObjectives-objIdx-1; i++ {
		f *= x[i]
	}
	if objIdx > 0 {
		f *= (1 - x[p.numObjectives-objIdx-1])
	}

	return f
}

func (p *DTLZ1) Constraints() []framework.Constraint {
	return nil
}

func (p *DTLZ1) Bounds() []framework.Bounds {
	b := make([]framework.Bounds, p.numVars)
	for i := range p.numVars {
		b[i] = framework.Bounds{L: 0.0, H: 1.0}
	}
	return b
}

func (p *DTLZ1) Initialize(popSize int) []framework.Solution {
	population := make([]framework.Solution, popSize)
	b := p.Bounds()
	for i := 0; i < popSize; i++ {
		vars := make([]float64, p.numVars)
		for j := 0; j < p.numVars; j++ {
			vars[j] = b[j].L + rand.Float64()*(b[j].H-b[j].L)
		}
		population[i] = framework.NewRealSolution(vars, b)
	}
	return population
}

func (p *DTLZ1) TrueParetoFront(numPoints int) []framework.ObjectiveSpacePoint {
	// For DTLZ1, the true Pareto front satisfies: sum(f_i) = 0.5
	// This is complex to generate for arbitrary dimensions
	// For 2 objectives, it's a line from (0, 0.5) to (0.5, 0)
	if p.numObjectives == 2 {
		points := make([]framework.ObjectiveSpacePoint, numPoints)
		for i := 0; i < numPoints; i++ {
			t := float64(i) / float64(numPoints-1)
			points[i] = framework.ObjectiveSpacePoint{
				0.5 * t,
				0.5 * (1 - t),
			}
		}
		return points
	}
	// For higher dimensions, return nil as it's complex to generate
	return nil
}
