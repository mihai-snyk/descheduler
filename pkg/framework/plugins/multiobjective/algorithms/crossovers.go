package algorithms

import (
	"math/rand"
)

// CrossoverFunc represents a crossover operation on integer chromosomes
type CrossoverFunc func(parent1, parent2 []int) (child1, child2 []int)

// Standard Crossover Operators

// OnePointCrossover creates offspring by selecting a random cut point
func OnePointCrossover(p1, p2 []int) ([]int, []int) {
	child1 := make([]int, len(p1))
	child2 := make([]int, len(p2))

	point := rand.Intn(len(p1))

	for i := 0; i < point; i++ {
		child1[i] = p1[i]
		child2[i] = p2[i]
	}
	for i := point; i < len(p1); i++ {
		child1[i] = p2[i]
		child2[i] = p1[i]
	}

	return child1, child2
}

// TwoPointCrossover creates offspring using two random cut points
func TwoPointCrossover(p1, p2 []int) ([]int, []int) {
	child1 := make([]int, len(p1))
	child2 := make([]int, len(p2))

	point1 := rand.Intn(len(p1))
	point2 := rand.Intn(len(p1))
	if point1 > point2 {
		point1, point2 = point2, point1
	}

	for i := 0; i < len(p1); i++ {
		if i < point1 || i >= point2 {
			child1[i] = p1[i]
			child2[i] = p2[i]
		} else {
			child1[i] = p2[i]
			child2[i] = p1[i]
		}
	}

	return child1, child2
}

// UniformCrossover creates offspring by randomly selecting from each parent
func UniformCrossover(p1, p2 []int) ([]int, []int) {
	child1 := make([]int, len(p1))
	child2 := make([]int, len(p2))

	for i := range p1 {
		if rand.Float64() < 0.5 {
			child1[i] = p1[i]
			child2[i] = p2[i]
		} else {
			child1[i] = p2[i]
			child2[i] = p1[i]
		}
	}

	return child1, child2
}

// KPointCrossover implements k-point crossover
func KPointCrossover(p1, p2 []int, k int) ([]int, []int) {
	child1 := make([]int, len(p1))
	child2 := make([]int, len(p2))

	// Generate k random cut points
	points := make([]int, k+2)
	points[0] = 0
	points[k+1] = len(p1)

	// Generate k unique random points
	used := make(map[int]bool)
	for i := 1; i <= k; i++ {
		point := 0
		for {
			point = 1 + rand.Intn(len(p1)-1)
			if !used[point] {
				used[point] = true
				break
			}
		}
		points[i] = point
	}

	// Sort points
	for i := 1; i < len(points)-1; i++ {
		for j := i + 1; j < len(points)-1; j++ {
			if points[i] > points[j] {
				points[i], points[j] = points[j], points[i]
			}
		}
	}

	// Apply crossover
	swap := false
	for i := 0; i < k+1; i++ {
		start := points[i]
		end := points[i+1]

		for j := start; j < end; j++ {
			if swap {
				child1[j] = p2[j]
				child2[j] = p1[j]
			} else {
				child1[j] = p1[j]
				child2[j] = p2[j]
			}
		}
		swap = !swap
	}

	return child1, child2
}

// Kubernetes-Specific Crossover Operators
// NodeAwareCrossover preserves pods assigned to the same node
func NodeAwareCrossover(p1, p2 []int) ([]int, []int) {
	child1 := make([]int, len(p1))
	child2 := make([]int, len(p2))

	// Group pods by node
	nodeGroups := make(map[int][]int)
	for pod, node := range p1 {
		nodeGroups[node] = append(nodeGroups[node], pod)
	}

	// Inherit node groups as units
	for _, pods := range nodeGroups {
		if rand.Float64() < 0.5 {
			// Child1 inherits this node's assignments from parent1
			for _, pod := range pods {
				child1[pod] = p1[pod]
				child2[pod] = p2[pod]
			}
		} else {
			// Child1 inherits from parent2
			for _, pod := range pods {
				child1[pod] = p2[pod]
				child2[pod] = p1[pod]
			}
		}
	}

	return child1, child2
}
