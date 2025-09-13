package algorithms

import (
	"fmt"
	"log"
	"math"
	"runtime"
	"sort"
	"sync"
	"time"

	"golang.org/x/exp/rand"
	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/framework"
)

const (
	Name = "NSGA-II"
)

// NSGAIISolution wraps a solution in the population
// with Rank and Distance fields. Value stores the value in
// the objective space for the solution (this is used when comparing
// solutions).
type NSGAIISolution struct {
	Solution framework.Solution
	Value    framework.ObjectiveSpacePoint

	Rank     int
	Distance float64
}

func NewNSGAIISolution(sol framework.Solution, val framework.ObjectiveSpacePoint) *NSGAIISolution {
	return &NSGAIISolution{
		Solution: sol,
		Value:    val,
	}
}

// Normalizer handles objective value normalization
type Normalizer struct {
	min []float64
	max []float64
}

// NewNormalizer creates a normalizer for the given number of objectives
func NewNormalizer(min []float64, max []float64) *Normalizer {
	return &Normalizer{
		min: min,
		max: max,
	}
}

// Normalize returns normalized objective values in [0,1]
func (n *Normalizer) Normalize(values []float64) []float64 {
	normalized := make([]float64, len(values))
	for i, val := range values {
		// Avoid division by zero
		if n.max[i] == n.min[i] {
			normalized[i] = 0
		} else {
			normalized[i] = (val - n.min[i]) / (n.max[i] - n.min[i])
		}
	}
	return normalized
}

// NonDominatedSort performs non-dominated sorting on the population
func NonDominatedSort(population []*NSGAIISolution) [][]*NSGAIISolution {
	var fronts [][]*NSGAIISolution
	dominated := make(map[int][]int)
	domCount := make([]int, len(population))

	// Calculate domination for each individual
	for i := 0; i < len(population); i++ {
		dominated[i] = []int{}
		for j := 0; j < len(population); j++ {
			if i != j {
				if Dominates(population[i], population[j]) {
					dominated[i] = append(dominated[i], j)
				} else if Dominates(population[j], population[i]) {
					domCount[i]++
				}
			}
		}
	}

	// Find first front
	currentFront := []*NSGAIISolution{}
	currentFrontIndices := []int{}
	for i := 0; i < len(population); i++ {
		if domCount[i] == 0 {
			population[i].Rank = 0
			currentFront = append(currentFront, population[i])
			currentFrontIndices = append(currentFrontIndices, i)
		}
	}
	fronts = append(fronts, currentFront)

	// Find subsequent fronts
	frontIndex := 0
	for len(currentFront) > 0 {
		nextFront := []*NSGAIISolution{}
		nextFrontIndices := []int{}
		for _, idx := range currentFrontIndices {
			for _, dominatedIdx := range dominated[idx] {
				domCount[dominatedIdx]--
				if domCount[dominatedIdx] == 0 {
					population[dominatedIdx].Rank = frontIndex + 1
					nextFront = append(nextFront, population[dominatedIdx])
					nextFrontIndices = append(nextFrontIndices, dominatedIdx)
				}
			}
		}
		frontIndex++
		if len(nextFront) > 0 {
			fronts = append(fronts, nextFront)
		}
		currentFront = nextFront
		currentFrontIndices = nextFrontIndices
	}

	return fronts
}

// Dominates checks if individual a dominates individual b
func Dominates(a, b *NSGAIISolution) bool {
	better := false
	for i := 0; i < len(a.Value); i++ {
		if a.Value[i] > b.Value[i] {
			return false
		}
		if a.Value[i] < b.Value[i] {
			better = true
		}
	}
	return better
}

// CrowdingDistance calculates crowding distance for individuals in a front
func CrowdingDistance(front []*NSGAIISolution) {
	if len(front) <= 2 {
		for i := range front {
			front[i].Distance = math.Inf(1)
		}
		return
	}

	numObjectives := len(front[0].Value)
	for i := range front {
		front[i].Distance = 0
	}

	for m := 0; m < numObjectives; m++ {
		// Sort by each objective
		sort.Slice(front, func(i, j int) bool {
			return front[i].Value[m] < front[j].Value[m]
		})

		// Set boundary points to infinity
		front[0].Distance = math.Inf(1)
		front[len(front)-1].Distance = math.Inf(1)

		objectiveRange := front[len(front)-1].Value[m] - front[0].Value[m]
		if objectiveRange == 0 {
			continue
		}

		// Calculate distance for intermediate points
		for i := 1; i < len(front)-1; i++ {
			front[i].Distance += (front[i+1].Value[m] - front[i-1].Value[m]) / objectiveRange
		}
	}
}

// Tournament selection
func TournamentSelect(population []*NSGAIISolution, tournamentSize int) *NSGAIISolution {
	if tournamentSize < 2 {
		tournamentSize = 2 // minimum tournament size
	}
	best := population[rand.Intn(len(population))]

	for i := 1; i < tournamentSize; i++ {
		contestant := population[rand.Intn(len(population))]
		if contestant.Rank < best.Rank || (contestant.Rank == best.Rank && contestant.Distance > best.Distance) {
			best = contestant
		}
	}

	return best
}

// NSGA2Config holds configuration parameters for NSGA-II
type NSGA2Config struct {
	PopulationSize       int
	MaxGenerations       int
	CrossoverProbability float64
	MutationProbability  float64
	TournamentSize       int
	ParallelExecution    bool // Enable parallel offspring generation
}

// NSGAII represents the NSGA-II algorithm configuration
type NSGAII struct {
	PopSize           int
	NumGenerations    int
	Problem           framework.Problem
	CrossoverRate     float64
	MutationRate      float64
	TournamentSize    int
	ParallelExecution bool
}

// NewNSGAII creates a new instance of NSGA-II with given parameters
func NewNSGAII(config NSGA2Config, problem framework.Problem) *NSGAII {
	return &NSGAII{
		PopSize:           config.PopulationSize,
		NumGenerations:    config.MaxGenerations,
		Problem:           problem,
		CrossoverRate:     config.CrossoverProbability,
		MutationRate:      config.MutationProbability,
		TournamentSize:    config.TournamentSize,
		ParallelExecution: config.ParallelExecution,
	}
}

// Evaluate evaluates the constraints and calculates objective values for an individual
func (n *NSGAII) Evaluate(individual framework.Solution) (framework.ObjectiveSpacePoint, error) {
	constraints := n.Problem.Constraints()

	// Check if all constraints are satisfied
	allSatisfied := true
	for _, c := range constraints {
		if !c(individual) {
			allSatisfied = false
			break
		}
	}

	objectives := n.Problem.ObjectiveFuncs()
	res := make([]float64, len(objectives))

	if !allSatisfied {
		// Use penalty approach: assign very bad fitness to invalid solutions
		// This encourages the algorithm to evolve away from them
		for i := range objectives {
			res[i] = math.Inf(1) // Worst possible value (we're minimizing)
		}
		return res, nil // Return without error so evolution continues
	}

	// Valid solution - evaluate normally
	for i, objFunc := range objectives {
		res[i] = objFunc(individual)
	}
	return res, nil
}

// Run executes the NSGA-II algorithm
func (n *NSGAII) Run() []*NSGAIISolution {
	startTime := time.Now()

	initPop := n.Problem.Initialize(n.PopSize)
	if len(initPop) != n.PopSize {
		log.Fatalf("could not initialize population with PopSize %d", n.PopSize)
	}

	// Log initial population diversity
	log.Printf("NSGA-II: Starting evolution")
	log.Printf("  Population size: %d", n.PopSize)
	log.Printf("  Generations: %d", n.NumGenerations)
	log.Printf("  Crossover rate: %.2f", n.CrossoverRate)
	log.Printf("  Mutation rate: %.4f", n.MutationRate)
	log.Printf("  Tournament size: %d", n.TournamentSize)
	if n.ParallelExecution {
		log.Printf("  Execution mode: PARALLEL (%d workers)", runtime.NumCPU())
	} else {
		log.Printf("  Execution mode: SEQUENTIAL")
	}

	// Initial population evaluation
	population := make([]*NSGAIISolution, n.PopSize)

	if n.ParallelExecution {
		// Parallel initial population evaluation
		numWorkers := runtime.NumCPU()
		workChan := make(chan int, n.PopSize)
		wg := &sync.WaitGroup{}

		// Start workers for initial evaluation
		for w := 0; w < numWorkers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range workChan {
					val, _ := n.Evaluate(initPop[i]) // Errors now handled via penalty
					population[i] = NewNSGAIISolution(initPop[i], val)
				}
			}()
		}

		// Send work to workers
		for i := 0; i < n.PopSize; i++ {
			workChan <- i
		}
		close(workChan)
		wg.Wait()
	} else {
		// Sequential initial population evaluation
		for i := 0; i < n.PopSize; i++ {
			val, _ := n.Evaluate(initPop[i]) // Errors now handled via penalty
			population[i] = NewNSGAIISolution(initPop[i], val)
		}
	}

	// Compute statistics after parallel work is done
	uniqueSolutions := make(map[string]bool)
	invalidCount := 0
	for i, sol := range population {
		if intSol, ok := sol.Solution.(*framework.IntegerSolution); ok {
			key := fmt.Sprintf("%v", intSol.Variables)
			uniqueSolutions[key] = true

			hasInf := false
			for _, v := range sol.Value {
				if math.IsInf(v, 1) {
					hasInf = true
					break
				}
			}
			if hasInf {
				invalidCount++
			} else if i < 5 { // Log first few valid solutions
				log.Printf("  Initial solution %d: %v, objectives: %v", i, intSol.Variables, sol.Value)
			}
		}
	}
	log.Printf("Initial population: %d unique solutions, %d invalid", len(uniqueSolutions), invalidCount)

	for gen := 0; gen < n.NumGenerations; gen++ {
		if gen%10 == 0 || gen < 5 { // Log every 10 generations and first 5
			log.Printf("\nGeneration %d/%d", gen+1, n.NumGenerations)
		}
		offspring := make([]*NSGAIISolution, n.PopSize)

		if n.ParallelExecution {
			// Parallel offspring generation
			numWorkers := runtime.NumCPU()
			workChan := make(chan int, n.PopSize/2)
			wg := &sync.WaitGroup{}

			// Start workers
			for w := 0; w < numWorkers; w++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := range workChan {
						// Generate offspring pair
						n.generateOffspringPair(i, population, offspring, gen)
					}
				}()
			}

			// Send work to workers
			for i := 0; i < n.PopSize; i += 2 {
				workChan <- i
			}
			close(workChan)

			// Wait for all workers to finish
			wg.Wait()
		} else {
			// Sequential offspring generation
			for i := 0; i < n.PopSize; i += 2 {
				n.generateOffspringPair(i, population, offspring, gen)
			}
		}

		// Compute statistics after parallel work is done
		offspringUnique := make(map[string]bool)
		offspringInvalid := 0
		for _, sol := range offspring {
			if sol != nil && sol.Solution != nil {
				if intSol, ok := sol.Solution.(*framework.IntegerSolution); ok {
					key := fmt.Sprintf("%v", intSol.Variables)
					offspringUnique[key] = true

					hasInf := false
					for _, v := range sol.Value {
						if math.IsInf(v, 1) {
							hasInf = true
							break
						}
					}
					if hasInf {
						offspringInvalid++
					}
				}
			}
		}

		// log.Printf("  Offspring: %d unique, %d invalid (%.1f%% validity with smart crossover+mutation)",
		// 	len(offspringUnique), offspringInvalid,
		// 	float64(len(offspringUnique)-offspringInvalid)/float64(len(offspringUnique))*100)

		// Combine populations
		combined := append(population, offspring...)

		// Non-dominated sorting
		fronts := NonDominatedSort(combined)

		// Clear population for next generation
		population = make([]*NSGAIISolution, 0, n.PopSize)
		frontIndex := 0

		// Add fronts to new population
		for len(population)+len(fronts[frontIndex]) <= n.PopSize {
			CrowdingDistance(fronts[frontIndex])
			population = append(population, fronts[frontIndex]...)
			frontIndex++
			if frontIndex >= len(fronts) {
				break
			}
		}

		// If needed, add remaining individuals based on crowding distance
		if len(population) < n.PopSize && frontIndex < len(fronts) {
			CrowdingDistance(fronts[frontIndex])
			sort.Slice(fronts[frontIndex], func(i, j int) bool {
				return fronts[frontIndex][i].Distance > fronts[frontIndex][j].Distance
			})
			population = append(population, fronts[frontIndex][:n.PopSize-len(population)]...)
		}
	}

	// Log final population statistics
	log.Printf("\nEvolution complete. Final population:")
	finalUnique := make(map[string]bool)
	finalInvalid := 0
	for i, sol := range population {
		if intSol, ok := sol.Solution.(*framework.IntegerSolution); ok {
			key := fmt.Sprintf("%v", intSol.Variables)
			finalUnique[key] = true

			hasInf := false
			for _, v := range sol.Value {
				if math.IsInf(v, 1) {
					hasInf = true
					break
				}
			}
			if hasInf {
				finalInvalid++
			} else if i < 5 { // Log first few final solutions
				log.Printf("  Final solution %d: %v, objectives: %v", i, intSol.Variables, sol.Value)
			}
		}
	}
	log.Printf("Final population: %d unique solutions, %d invalid", len(finalUnique), finalInvalid)

	// Report execution time
	elapsedTime := time.Since(startTime)
	mode := "SEQUENTIAL"
	if n.ParallelExecution {
		mode = "PARALLEL"
	}
	log.Printf("\nNSGA-II %s execution completed in %v", mode, elapsedTime)
	log.Printf("  Time per generation: %v", elapsedTime/time.Duration(n.NumGenerations))

	return population
}

// smartMutate performs constraint-aware mutation
// Only mutates to nodes that have capacity for the pod
func (n *NSGAII) smartMutate(solution *framework.IntegerSolution, problem framework.Problem) {
	// Get constraints from problem
	constraints := problem.Constraints()
	if len(constraints) == 0 {
		// Fallback to regular mutation if no constraints
		solution.Mutate(n.MutationRate)
		return
	}

	// Try each gene for mutation
	for i := range solution.Variables {
		if rand.Float64() < n.MutationRate {
			currentNode := solution.Variables[i]
			originalNode := currentNode
			maxNode := solution.Bounds[i].H
			maxAttempts := maxNode + 1 // Try up to number of nodes

			// Try random nodes until we find a valid one (early exit optimization)
			foundValidNode := false
			for attempt := 0; attempt < maxAttempts; attempt++ {
				// Pick a random node (excluding current)
				var candidateNode int
				for {
					candidateNode = rand.Intn(maxNode + 1)
					if candidateNode != currentNode {
						break
					}
				}

				// Temporarily assign to test validity
				solution.Variables[i] = candidateNode

				// Check if all constraints are satisfied
				valid := true
				for _, constraint := range constraints {
					if !constraint(solution) {
						valid = false
						break
					}
				}

				if valid {
					// Found valid node! Use it and exit early
					foundValidNode = true
					break
				}
			}

			if !foundValidNode {
				// No valid alternative found, restore original assignment
				solution.Variables[i] = originalNode
			}
			// Otherwise keep the valid node we found
		}
	}
}

// smartCrossover performs constraint-aware crossover
// Only swaps genes if both children remain valid
func (n *NSGAII) smartCrossover(parent1, parent2 *framework.IntegerSolution) (framework.Solution, framework.Solution) {
	// Clone parents to create children
	child1 := parent1.Clone().(*framework.IntegerSolution)
	child2 := parent2.Clone().(*framework.IntegerSolution)

	// If crossover should not happen, return clones
	if rand.Float64() >= n.CrossoverRate {
		return child1, child2
	}

	constraints := n.Problem.Constraints()
	if len(constraints) == 0 {
		// No constraints, use regular crossover
		return parent1.Crossover(parent2, n.CrossoverRate)
	}

	// Smart uniform crossover: only swap if both children remain valid
	for i := range child1.Variables {
		if rand.Float64() < 0.5 { // 50% chance to consider swapping each gene
			// Check if swap would be valid
			orig1 := child1.Variables[i]
			orig2 := child2.Variables[i]

			// Skip if already same
			if orig1 == orig2 {
				continue
			}

			// Try the swap
			child1.Variables[i] = orig2
			child2.Variables[i] = orig1

			// Check if both children are still valid
			// Important: evaluate both before breaking to avoid partial checks
			valid1 := true
			valid2 := true
			for _, constraint := range constraints {
				c1Valid := constraint(child1)
				c2Valid := constraint(child2)
				if !c1Valid {
					valid1 = false
				}
				if !c2Valid {
					valid2 = false
				}
			}

			if valid1 && valid2 {
				// Keep the swap
			} else {
				// Revert the swap
				child1.Variables[i] = orig1
				child2.Variables[i] = orig2
			}
		}
	}

	return child1, child2
}

// generateOffspringPair generates a pair of offspring from the population
func (n *NSGAII) generateOffspringPair(i int, population, offspring []*NSGAIISolution, gen int) {

	parent1 := TournamentSelect(population, n.TournamentSize)
	parent2 := TournamentSelect(population, n.TournamentSize)

	// Use smart crossover for IntegerSolutions
	var child1, child2 framework.Solution
	if intParent1, ok1 := parent1.Solution.(*framework.IntegerSolution); ok1 {
		if intParent2, ok2 := parent2.Solution.(*framework.IntegerSolution); ok2 {
			child1, child2 = n.smartCrossover(intParent1, intParent2)
		} else {
			child1, child2 = parent1.Solution.Crossover(parent2.Solution, n.CrossoverRate)
		}
	} else {
		child1, child2 = parent1.Solution.Crossover(parent2.Solution, n.CrossoverRate)
	}

	// Use smart mutation instead of blind mutation
	if intChild1, ok := child1.(*framework.IntegerSolution); ok {
		n.smartMutate(intChild1, n.Problem)
	} else {
		child1.Mutate(n.MutationRate)
	}

	if intChild2, ok := child2.(*framework.IntegerSolution); ok {
		n.smartMutate(intChild2, n.Problem)
	} else {
		child2.Mutate(n.MutationRate)
	}

	val1, _ := n.Evaluate(child1) // Errors now handled via penalty
	offspring[i] = NewNSGAIISolution(child1, val1)

	if i+1 < len(offspring) {
		val2, _ := n.Evaluate(child2) // Errors now handled via penalty
		offspring[i+1] = NewNSGAIISolution(child2, val2)
	}
}
