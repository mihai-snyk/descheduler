# Large Cluster Mixed Workloads Scenario

This scenario replicates the `LargeCluster_MixedWorkloads` test case from the multi-objective integration test using kwok nodes.

## Cluster Configuration

### Nodes (11 total)
- **Production nodes** (4x m5.xlarge, on-demand): 8 CPU, 16GB RAM each
  - prod-1, prod-2, prod-3, prod-4
- **Development nodes** (3x m5.large): 4 CPU, 8GB RAM each
  - dev-1, dev-2 (spot)
  - dev-3 (on-demand)
- **Worker nodes** (2x c5.2xlarge, spot): 16 CPU, 16GB RAM each
  - worker-1, worker-2
- **Memory-optimized nodes** (2x r5.xlarge, on-demand): 4 CPU, 32GB RAM each
  - mem-1, mem-2

### Workloads (32 pods total)
- **Frontend** (12 replicas): 500m CPU, 1GB RAM, MaxUnavailable=2
- **API** (8 replicas): 1 CPU, 2GB RAM, MaxUnavailable=1
- **Cache** (6 replicas): 1 CPU, 6GB RAM, MaxUnavailable=2
- **Worker** (4 replicas): 4 CPU, 4GB RAM, MaxUnavailable=3
- **Test Runner** (2 replicas): 1.5 CPU, 3GB RAM, MaxUnavailable=2

## Setup Instructions

1. Run the setup script:
   ```bash
   cd manifests
   ./large-cluster-setup.sh
   ```

2. Or manually apply the resources:
   ```bash
   kubectl apply -f large-cluster-kwok-nodes.yaml
   kubectl apply -f large-cluster-deployments.yaml
   ```

## Expected Behavior

According to the test case, with weights:
- Cost: 0.30
- Disruption: 0.50
- Balance: 0.20

The multi-objective optimizer should:
- "Consolidate non-critical workloads to spot nodes while respecting PDBs"
- Find trade-offs between cost savings (using spot instances) and operational stability

## Resource Requirements

Total cluster capacity:
- CPU: 88 cores (8×4 + 4×3 + 16×2 + 4×2)
- Memory: 168GB (16×4 + 8×3 + 16×2 + 32×2)

Total pod requirements:
- CPU: 32 cores (0.5×12 + 1×8 + 1×6 + 4×4 + 1.5×2)
- Memory: 62GB (1×12 + 2×8 + 6×6 + 4×4 + 3×2)

Utilization: ~36% CPU, ~37% Memory

## Cleanup

To remove all resources:
```bash
kubectl delete -f large-cluster-deployments.yaml
kubectl delete -f large-cluster-kwok-nodes.yaml
```
