package benchmarks

import (
	"math"
	"math/rand/v2"

	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
)

// DTLZ2 has a spherical Pareto front
// It's easier than DTLZ1 as it has no local fronts
type DTLZ2 struct {
	numVars       int
	numObjectives int
}

func NewDTLZ2(numVars, numObjectives int) *DTLZ2 {
	// Recommended: numVars = numObjectives + k - 1, where k = 10 for DTLZ2
	return &DTLZ2{
		numVars:       numVars,
		numObjectives: numObjectives,
	}
}

func (p *DTLZ2) Name() string {
	return "DTLZ2"
}

func (p *DTLZ2) ObjectiveFuncs() []framework.ObjectiveFunc {
	funcs := make([]framework.ObjectiveFunc, p.numObjectives)
	for i := 0; i < p.numObjectives; i++ {
		idx := i // Capture loop variable
		funcs[i] = func(x framework.Solution) float64 {
			return p.objective(x, idx)
		}
	}
	return funcs
}

func (p *DTLZ2) g(x []float64) float64 {
	sum := 0.0
	for i := p.numObjectives - 1; i < p.numVars; i++ {
		sum += math.Pow(x[i]-0.5, 2)
	}
	return sum
}

func (p *DTLZ2) objective(sol framework.Solution, objIdx int) float64 {
	x := sol.(*framework.RealSolution).Variables
	g := p.g(x)

	f := 1 + g

	// Product of cos terms
	for i := 0; i < p.numObjectives-objIdx-1; i++ {
		f *= math.Cos(x[i] * math.Pi / 2)
	}

	// Last term is sin for all objectives except the last
	if objIdx > 0 {
		f *= math.Sin(x[p.numObjectives-objIdx-1] * math.Pi / 2)
	}

	return f
}

func (p *DTLZ2) Constraints() []framework.Constraint {
	return nil
}

func (p *DTLZ2) Bounds() []framework.Bounds {
	b := make([]framework.Bounds, p.numVars)
	for i := range p.numVars {
		b[i] = framework.Bounds{L: 0.0, H: 1.0}
	}
	return b
}

func (p *DTLZ2) Initialize(popSize int) []framework.Solution {
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

func (p *DTLZ2) TrueParetoFront(numPoints int) []framework.ObjectiveSpacePoint {
	// For DTLZ2, the true Pareto front is on a unit sphere: sum(f_i^2) = 1
	// For 2 objectives, it's a quarter circle
	if p.numObjectives == 2 {
		points := make([]framework.ObjectiveSpacePoint, numPoints)
		for i := 0; i < numPoints; i++ {
			theta := (math.Pi / 2) * float64(i) / float64(numPoints-1)
			points[i] = framework.ObjectiveSpacePoint{
				math.Cos(theta),
				math.Sin(theta),
			}
		}
		return points
	}
	// For 3 objectives, generate points on a unit sphere
	if p.numObjectives == 3 {
		// Simple uniform distribution on sphere surface
		sqrtN := int(math.Sqrt(float64(numPoints)))
		points := make([]framework.ObjectiveSpacePoint, 0, sqrtN*sqrtN)

		for i := 0; i < sqrtN; i++ {
			theta := (math.Pi / 2) * float64(i) / float64(sqrtN-1)
			for j := 0; j < sqrtN; j++ {
				phi := (math.Pi / 2) * float64(j) / float64(sqrtN-1)
				points = append(points, framework.ObjectiveSpacePoint{
					math.Cos(theta) * math.Cos(phi),
					math.Sin(theta) * math.Cos(phi),
					math.Sin(phi),
				})
			}
		}
		return points
	}
	return nil
}
