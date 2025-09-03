#!/bin/bash

echo "Setting up Large Cluster Mixed Workloads scenario..."

# Clean up any existing resources
echo "Cleaning up existing resources..."
kubectl delete nodes prod-1 prod-2 prod-3 prod-4 dev-1 dev-2 dev-3 worker-1 worker-2 mem-1 mem-2 2>/dev/null || true
kubectl delete deployment frontend api cache worker test-runner 2>/dev/null || true
kubectl delete pdb frontend-pdb api-pdb cache-pdb worker-pdb test-runner-pdb 2>/dev/null || true

# Apply nodes
echo "Creating kwok nodes..."
kubectl apply -f large-cluster-kwok-nodes.yaml

# Wait for nodes to be ready
echo "Waiting for nodes to be ready..."
sleep 5

# Apply deployments and PDBs
echo "Creating deployments and PDBs..."
kubectl apply -f large-cluster-deployments.yaml

# Wait for deployments to be ready
echo "Waiting for deployments to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/frontend deployment/api deployment/cache deployment/worker deployment/test-runner

# Show cluster state
echo -e "\nCluster setup complete! Current state:"
echo -e "\nNodes:"
kubectl get nodes -l type=kwok -o wide

echo -e "\nDeployments:"
kubectl get deployments

echo -e "\nPods distribution:"
for node in prod-1 prod-2 prod-3 prod-4 dev-1 dev-2 dev-3 worker-1 worker-2 mem-1 mem-2; do
  pod_count=$(kubectl get pods --field-selector spec.nodeName=$node --no-headers 2>/dev/null | wc -l)
  echo "$node: $pod_count pods"
done

echo -e "\nPodDisruptionBudgets:"
kubectl get pdb

echo -e "\nTotal resource usage:"
kubectl describe nodes prod-1 prod-2 prod-3 prod-4 dev-1 dev-2 dev-3 worker-1 worker-2 mem-1 mem-2 | grep -A 5 "Allocated resources:" | grep -E "(Name:|cpu|memory)"
