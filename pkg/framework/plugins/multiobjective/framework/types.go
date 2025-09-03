package framework

// Default constants for the plugin
const (
	// Algorithm defaults
	DefaultPopulationSize       = 400
	DefaultMaxGenerations       = 1000
	DefaultCrossoverProbability = 0.90
	DefaultMutationProbability  = 0.30

	// Weight defaults
	DefaultWeightCost       = 0.33
	DefaultWeightDisruption = 0.33
	DefaultWeightBalance    = 0.34
	DefaultTournamentSize   = 3
)

// NodeInfo contains node information for optimization
type NodeInfo struct {
	Idx               int
	Name              string
	CPUCapacity       float64 // in millicores
	MemCapacity       float64 // in bytes
	CPUUsed           float64 // for statistics only
	MemUsed           float64 // for statistics only
	InstanceType      string
	InstanceLifecycle string
	Region            string
	HourlyCost        float64
	CostPerHour       float64
}

// PodInfo contains pod information for optimization
type PodInfo struct {
	Idx                    int
	Name                   string
	Namespace              string
	Node                   int
	CPURequest             float64
	MemRequest             float64
	ReplicaSetName         string
	ColdStartTime          float64 // Startup probe time for disruption calculation
	MaxUnavailableReplicas int     // From PDB for disruption calculation
}
