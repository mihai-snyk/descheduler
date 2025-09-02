package benchmarks

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"sigs.k8s.io/descheduler/pkg/framework/plugins/multiobjective/algorithms"
)

// TestVisualizeCrossovers generates inheritance pattern visualizations
func TestVisualizeCrossovers(t *testing.T) {
	numVars := 50   // 50 pods
	numTrials := 30 // 30 different offspring

	tests := []struct {
		name      string
		crossover algorithms.CrossoverFunc
	}{
		{"OnePoint", algorithms.OnePointCrossover},
		{"TwoPoint", algorithms.TwoPointCrossover},
		{"Uniform", algorithms.UniformCrossover},
		{"NodeAware", algorithms.NodeAwareCrossover},
		{"4Point", func(p1, p2 []int) ([]int, []int) {
			return algorithms.KPointCrossover(p1, p2, 4)
		}},
	}

	// Create parents with skewed distribution for all crossovers
	numNodes := 10
	parent1 := make([]int, numVars)
	parent2 := make([]int, numVars)

	// Parent 1: first half on node 0, second half on node 1
	for i := 0; i < numVars/2; i++ {
		parent1[i] = 0
	}
	for i := numVars / 2; i < numVars; i++ {
		parent1[i] = 1
	}

	// Parent 2: similar but slightly different distribution
	// 40% on node 0, 40% on node 1, 20% on node 2
	for i := 0; i < int(0.4*float64(numVars)); i++ {
		parent2[i] = 0
	}
	for i := int(0.4 * float64(numVars)); i < int(0.8*float64(numVars)); i++ {
		parent2[i] = 1
	}
	for i := int(0.8 * float64(numVars)); i < numVars; i++ {
		parent2[i] = 2
	}

	t.Logf("Using skewed parent distribution for all crossovers:")
	t.Logf("Parent1: 50%% on node 0, 50%% on node 1")
	t.Logf("Parent2: 40%% on node 0, 40%% on node 1, 20%% on node 2")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputFile := fmt.Sprintf("crossover_%s.png", tt.name)

			// Use same skewed parents for all crossovers
			err := VisualizeSpecificCrossover(tt.crossover, parent1, parent2, numNodes, numTrials, outputFile)
			if err != nil {
				t.Errorf("Failed to create visualization: %v", err)
			} else {
				t.Logf("Created visualization: %s", outputFile)
			}
		})
	}
}

// VisualizeCrossover creates a visualization of crossover inheritance patterns
// Similar to pymoo's visualization: black = inherited from parent B, white = inherited from parent A
func VisualizeCrossover(crossoverFunc func([]int, []int) ([]int, []int),
	numVars int, numTrials int, outputFile string) error {

	// Create image with better scaling
	pixelsPerVar := 8
	pixelsPerTrial := 8
	width := numVars * pixelsPerVar
	height := numTrials * pixelsPerTrial
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Colors (matching pymoo: black = different from parent A)
	black := color.RGBA{0, 0, 0, 255}
	white := color.RGBA{255, 255, 255, 255}
	gray := color.RGBA{128, 128, 128, 255} // For grid lines

	// Generate distinct parent chromosomes
	parentA := make([]int, numVars)
	parentB := make([]int, numVars)
	for i := range parentA {
		parentA[i] = i + 1    // [1, 2, 3, ..., numVars]
		parentB[i] = -(i + 1) // [-1, -2, -3, ..., -numVars]
	}

	// Run multiple crossovers
	for trial := 0; trial < numTrials; trial++ {
		// Always use same parents for consistency
		child, _ := crossoverFunc(parentA, parentB)

		// Plot inheritance pattern (black where child != parentA)
		for varIdx := 0; varIdx < numVars; varIdx++ {
			var col color.RGBA
			if child[varIdx] != parentA[varIdx] {
				col = black // Inherited from parent B
			} else {
				col = white // Inherited from parent A
			}

			// Draw rectangle for this gene
			for x := varIdx * pixelsPerVar; x < (varIdx+1)*pixelsPerVar-1; x++ {
				for y := trial * pixelsPerTrial; y < (trial+1)*pixelsPerTrial-1; y++ {
					img.Set(x, y, col)
				}
			}

			// Add subtle grid lines
			for y := trial * pixelsPerTrial; y < (trial+1)*pixelsPerTrial; y++ {
				img.Set((varIdx+1)*pixelsPerVar-1, y, gray)
			}
		}

		// Horizontal grid line
		for x := 0; x < width; x++ {
			img.Set(x, (trial+1)*pixelsPerTrial-1, gray)
		}
	}

	// Save image
	f, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer f.Close()

	return png.Encode(f, img)
}

// VisualizeSpecificCrossover creates a visualization using specific parent configurations
func VisualizeSpecificCrossover(crossoverFunc func([]int, []int) ([]int, []int),
	parentA, parentB []int, numNodes int, numTrials int, outputFile string) error {

	numVars := len(parentA)

	// Create image with better scaling
	pixelsPerVar := 8
	pixelsPerTrial := 8
	width := numVars * pixelsPerVar
	height := numTrials * pixelsPerTrial
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Colors (matching pymoo: black = different from parent A)
	black := color.RGBA{0, 0, 0, 255}
	white := color.RGBA{255, 255, 255, 255}
	gray := color.RGBA{128, 128, 128, 255} // For grid lines

	// Run multiple crossovers
	for trial := 0; trial < numTrials; trial++ {
		// Always use same parents for consistency
		child, _ := crossoverFunc(parentA, parentB)

		// Plot inheritance pattern (black where child != parentA)
		for varIdx := 0; varIdx < numVars; varIdx++ {
			var col color.RGBA
			if child[varIdx] != parentA[varIdx] {
				col = black // Inherited from parent B
			} else {
				col = white // Inherited from parent A
			}

			// Draw rectangle for this gene
			for x := varIdx * pixelsPerVar; x < (varIdx+1)*pixelsPerVar-1; x++ {
				for y := trial * pixelsPerTrial; y < (trial+1)*pixelsPerTrial-1; y++ {
					img.Set(x, y, col)
				}
			}

			// Add subtle grid lines
			for y := trial * pixelsPerTrial; y < (trial+1)*pixelsPerTrial; y++ {
				img.Set((varIdx+1)*pixelsPerVar-1, y, gray)
			}
		}

		// Horizontal grid line
		for x := 0; x < width; x++ {
			img.Set(x, (trial+1)*pixelsPerTrial-1, gray)
		}
	}

	// Save image
	f, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer f.Close()

	return png.Encode(f, img)
}
