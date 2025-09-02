package framework

import (
	"math"
	"math/rand/v2"
)

// Problem describes the contract a specific multi-objective problem needs to implement.
type Problem interface {
	Name() string

	ObjectiveFuncs() []ObjectiveFunc
	Constraints() []Constraint
	Bounds() []Bounds
	Initialize(int) []Solution

	// TrueParetoFront is optional due to the difficulty of finding the true front
	// in some types of problems. When there isn't a way to find the true front,
	// just return nil.
	TrueParetoFront(int) []ObjectiveSpacePoint
}

type Solution interface {
	Clone() Solution
	Crossover(Solution, float64) (Solution, Solution)
	Mutate(float64)
}

// Algorithm describes the contract that a MOO algorithm needs to implement.
// TODO: Improve the abstraction by adding more methods
type Algorithm interface {
	Name() string
}

// ObjectiveFunc defines the interface for objective functions
type ObjectiveFunc func(Solution) float64

// ObjectiveSpacePoint represents an N-dimensional point in the objective space.
// As an example, for a problem with 2 objective functions f1 and f2, a point
// in the objective space could be [f1(x'), f2(x')], for the input of x'.
type ObjectiveSpacePoint []float64

// Constraint returns true if the constraint is satisfied and false otherwise.
type Constraint func(Solution) bool

// BinarySolution uses a binary encoding scheme, where each bit
// or group of bits can have a meaning in the context of the problem.
type BinarySolution struct {
	Bits []bool
}

func NewBinarySolution(bits []bool) *BinarySolution {
	return &BinarySolution{
		Bits: bits,
	}
}

func (sol *BinarySolution) Clone() Solution {
	newBits := make([]bool, len(sol.Bits))
	copy(newBits, sol.Bits)
	return &BinarySolution{
		Bits: newBits,
	}
}

// Crossover implements Solution interface using single-point crossover
func (s *BinarySolution) Crossover(other Solution, crossoverRate float64) (Solution, Solution) {
	o := other.(*BinarySolution)
	child1 := s.Clone().(*BinarySolution)
	child2 := o.Clone().(*BinarySolution)

	if rand.Float64() < crossoverRate { // crossover probability
		// Single point crossover
		point := rand.IntN(len(s.Bits))
		for i := point; i < len(s.Bits); i++ {
			child1.Bits[i], child2.Bits[i] = child2.Bits[i], child1.Bits[i]
		}
	}

	return child1, child2
}

// Mutate implements Solution interface using bit-flip mutation
func (s *BinarySolution) Mutate(mutationRate float64) {
	for i := range s.Bits {
		if rand.Float64() < mutationRate {
			s.Bits[i] = !s.Bits[i]
		}
	}
}

// RealSolution represents a solution with real-valued variables.
type RealSolution struct {
	Variables []float64
	Bounds    []Bounds
}

type Bounds struct {
	L float64
	H float64
}

func NewRealSolution(vars []float64, b []Bounds) *RealSolution {
	return &RealSolution{
		Variables: vars,
		Bounds:    b,
	}
}

func (sol *RealSolution) Clone() Solution {
	return &RealSolution{
		Variables: make([]float64, len(sol.Variables)),
		Bounds:    sol.Bounds,
	}
}

// Crossover performs SBX (Simulated Binary Crossover)
func (sol *RealSolution) Crossover(other Solution, crossoverRate float64) (Solution, Solution) {
	o := other.(*RealSolution)
	child1 := sol.Clone().(*RealSolution)
	child2 := other.Clone().(*RealSolution)

	if rand.Float64() < crossoverRate {
		for i := range sol.Variables {
			beta := 0.0
			if rand.Float64() <= 0.5 {
				beta = math.Pow(2*rand.Float64(), 1.0/3.0)
			} else {
				beta = math.Pow(1.0/(2*(1.0-rand.Float64())), 1.0/3.0)
			}

			child1.Variables[i] = 0.5 * ((1+beta)*sol.Variables[i] + (1-beta)*o.Variables[i])
			child2.Variables[i] = 0.5 * ((1-beta)*sol.Variables[i] + (1+beta)*o.Variables[i])

			// Bound checking
			child1.Variables[i] = math.Max(sol.Bounds[i].L, math.Min(sol.Bounds[i].H, child1.Variables[i]))
			child2.Variables[i] = math.Max(sol.Bounds[i].L, math.Min(sol.Bounds[i].H, child2.Variables[i]))
		}
	} else {
		copy(child1.Variables, sol.Variables)
		copy(child2.Variables, o.Variables)
	}

	return child1, child2
}

// Mutation performs polynomial mutation
func (sol *RealSolution) Mutate(mutationRate float64) {
	for i := range sol.Variables {
		if rand.Float64() < mutationRate {
			delta := 0.0
			if rand.Float64() <= 0.5 {
				delta = math.Pow(2*rand.Float64(), 1.0/3.0) - 1
			} else {
				delta = 1 - math.Pow(2*(1-rand.Float64()), 1.0/3.0)
			}

			sol.Variables[i] += delta * (sol.Bounds[i].H - sol.Bounds[i].L)
			sol.Variables[i] = math.Max(sol.Bounds[i].L, math.Min(sol.Bounds[i].H, sol.Variables[i]))
		}
	}
}

// IntegerSolution represents a solution with integer-valued variables
type IntegerSolution struct {
	Variables []int
	Bounds    []IntBounds
}

type IntBounds struct {
	L int // Lower bound
	H int // Upper bound
}

func NewIntegerSolution(vars []int, bounds []IntBounds) *IntegerSolution {
	return &IntegerSolution{
		Variables: vars,
		Bounds:    bounds,
	}
}

func (sol *IntegerSolution) Clone() Solution {
	newVars := make([]int, len(sol.Variables))
	copy(newVars, sol.Variables)
	return &IntegerSolution{
		Variables: newVars,
		Bounds:    sol.Bounds,
	}
}

// Crossover performs uniform crossover by default
func (sol *IntegerSolution) Crossover(other Solution, crossoverRate float64) (Solution, Solution) {
	o := other.(*IntegerSolution)
	child1 := sol.Clone().(*IntegerSolution)
	child2 := o.Clone().(*IntegerSolution)

	if rand.Float64() < crossoverRate {
		// Uniform crossover
		for i := range sol.Variables {
			if rand.Float64() < 0.5 {
				child1.Variables[i], child2.Variables[i] = child2.Variables[i], child1.Variables[i]
			}
		}
	}

	return child1, child2
}

// Mutate performs random reset mutation
func (sol *IntegerSolution) Mutate(mutationRate float64) {
	// mutated := false
	for i := range sol.Variables {
		if rand.Float64() < mutationRate {
			// Random reset within bounds
			if sol.Bounds != nil && i < len(sol.Bounds) {
				oldVal := sol.Variables[i]
				sol.Variables[i] = sol.Bounds[i].L + rand.IntN(sol.Bounds[i].H-sol.Bounds[i].L+1)
				if oldVal != sol.Variables[i] {
					// mutated = true
				}
			}
		}
	}
	// // Uncomment for debugging:
	// if mutated {
	// 	log.Printf("Mutation occurred: %v", sol.Variables)
	// }
}
