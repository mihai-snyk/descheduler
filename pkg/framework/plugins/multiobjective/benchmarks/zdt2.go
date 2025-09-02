package benchmarks

import (
	"math"
	"math/rand/v2"

	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
)

// ZDT2 has a non-convex Pareto front
type ZDT2 struct {
	numVars int
}

func NewZDT2(numVars int) *ZDT2 {
	return &ZDT2{numVars: numVars}
}

func (p *ZDT2) Name() string {
	return "ZDT2"
}

func (p *ZDT2) ObjectiveFuncs() []framework.ObjectiveFunc {
	return []framework.ObjectiveFunc{p.f1, p.f2}
}

func (p *ZDT2) f1(x framework.Solution) float64 {
	xx := x.(*framework.RealSolution)
	return xx.Variables[0]
}

func (p *ZDT2) f2(x framework.Solution) float64 {
	xx := x.(*framework.RealSolution).Variables
	g := 1.0
	for i := 1; i < len(xx); i++ {
		g += 9.0 * xx[i] / float64(len(xx)-1)
	}
	// Note: ZDT2 uses (1 - (x1/g)^2) instead of sqrt
	return g * (1.0 - math.Pow(xx[0]/g, 2))
}

func (p *ZDT2) Constraints() []framework.Constraint {
	return nil
}

func (p *ZDT2) Bounds() []framework.Bounds {
	b := make([]framework.Bounds, p.numVars)
	for i := range p.numVars {
		b[i] = framework.Bounds{L: 0.0, H: 1.0}
	}
	return b
}

func (p *ZDT2) Initialize(popSize int) []framework.Solution {
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

func (p *ZDT2) TrueParetoFront(numPoints int) []framework.ObjectiveSpacePoint {
	points := make([]framework.ObjectiveSpacePoint, numPoints)
	for i := 0; i < numPoints; i++ {
		x := float64(i) / float64(numPoints-1)
		points[i] = framework.ObjectiveSpacePoint{
			x, 1.0 - x*x,
		}
	}
	return points
}
