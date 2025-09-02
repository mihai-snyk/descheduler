package benchmarks

import (
	"math"
	"math/rand/v2"

	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
)

// ZDT3 has a disconnected Pareto front
type ZDT3 struct {
	numVars int
}

func NewZDT3(numVars int) *ZDT3 {
	return &ZDT3{numVars: numVars}
}

func (p *ZDT3) Name() string {
	return "ZDT3"
}

func (p *ZDT3) ObjectiveFuncs() []framework.ObjectiveFunc {
	return []framework.ObjectiveFunc{p.f1, p.f2}
}

func (p *ZDT3) f1(x framework.Solution) float64 {
	xx := x.(*framework.RealSolution)
	return xx.Variables[0]
}

func (p *ZDT3) f2(x framework.Solution) float64 {
	xx := x.(*framework.RealSolution).Variables
	g := 1.0
	for i := 1; i < len(xx); i++ {
		g += 9.0 * xx[i] / float64(len(xx)-1)
	}
	// ZDT3 has a disconnected front due to the sin term
	h := 1.0 - math.Sqrt(xx[0]/g) - (xx[0]/g)*math.Sin(10*math.Pi*xx[0])
	return g * h
}

func (p *ZDT3) Constraints() []framework.Constraint {
	return nil
}

func (p *ZDT3) Bounds() []framework.Bounds {
	b := make([]framework.Bounds, p.numVars)
	for i := range p.numVars {
		b[i] = framework.Bounds{L: 0.0, H: 1.0}
	}
	return b
}

func (p *ZDT3) Initialize(popSize int) []framework.Solution {
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

func (p *ZDT3) TrueParetoFront(numPoints int) []framework.ObjectiveSpacePoint {
	// ZDT3 has a disconnected Pareto front
	// Generate more points to properly show the disconnected nature
	points := make([]framework.ObjectiveSpacePoint, numPoints)

	for i := 0; i < numPoints; i++ {
		x := float64(i) / float64(numPoints-1)
		f1 := x
		f2 := 1.0 - math.Sqrt(x) - x*math.Sin(10*math.Pi*x)
		points[i] = framework.ObjectiveSpacePoint{f1, f2}
	}

	return points
}
