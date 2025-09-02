package benchmarks

import (
	"math"
	"math/rand/v2"

	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
)

const (
	Name = "ZDT1"
)

// ZDT1 is a benchmark function used to test the correctness
// of multi-objective algorithms. For more details, check the article below:
// https://datacrayon.com/practical-evolutionary-algorithms/synthetic-objective-functions-and-zdt1/
type ZDT1 struct {
	numVars int
}

func NewZDT1(numVars int) *ZDT1 {
	return &ZDT1{
		numVars,
	}
}

func (p *ZDT1) Name() string {
	return Name
}

func (p *ZDT1) ObjectiveFuncs() []framework.ObjectiveFunc {
	return []framework.ObjectiveFunc{
		p.f1, p.f2,
	}
}

// F1 is the first ZD1 benchmark objective
func (p *ZDT1) f1(x framework.Solution) float64 {
	xx := x.(*framework.RealSolution)
	return xx.Variables[0]
}

// F2 is the first ZD1 benchmark objective
func (p *ZDT1) f2(x framework.Solution) float64 {
	xx := x.(*framework.RealSolution).Variables
	g := 1.0
	for i := 1; i < len(xx); i++ {
		g += 9.0 * xx[i] / float64(len(xx)-1)
	}
	return g * (1.0 - math.Sqrt(xx[0]/g))
}

// This is an unconstrained problem
func (p *ZDT1) Constraints() []framework.Constraint {
	return nil
}

func (p *ZDT1) Bounds() []framework.Bounds {
	b := make([]framework.Bounds, p.numVars)
	for i := range p.numVars {
		b[i] = framework.Bounds{
			L: 0.0,
			H: 1.0,
		}
	}
	return b
}

// Initialize creates an initial random population of individuals
func (p *ZDT1) Initialize(popSize int) []framework.Solution {
	population := make([]framework.Solution, popSize)
	b := p.Bounds()

	for i := 0; i < popSize; i++ {
		vars := make([]float64, p.numVars)
		for j := 0; j < p.numVars; j++ {
			vars[j] = b[j].L + rand.Float64()*(b[j].H-b[j].L)
		}
		sol := framework.NewRealSolution(vars, b)
		population[i] = sol
	}
	return population
}

// TrueParetoFront generates numPoints points on the true Pareto front for ZDT1
func (p *ZDT1) TrueParetoFront(numPoints int) []framework.ObjectiveSpacePoint {
	points := make([]framework.ObjectiveSpacePoint, numPoints)
	for i := 0; i < numPoints; i++ {
		x := float64(i) / float64(numPoints-1)
		points[i] = framework.ObjectiveSpacePoint{
			x, 1.0 - math.Sqrt(x),
		}
	}
	return points
}
