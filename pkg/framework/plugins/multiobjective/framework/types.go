package framework

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
