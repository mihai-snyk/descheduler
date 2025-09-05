#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

echo "Generating typed clients for SchedulingHint CRD..."

# Build the client-gen tool
go build -o "${OS_OUTPUT_BINPATH}/client-gen" "k8s.io/code-generator/cmd/client-gen"

# Generate the typed client
${OS_OUTPUT_BINPATH}/client-gen \
    --go-header-file "hack/boilerplate/boilerplate.go.txt" \
    --clientset-name "versioned" \
    --input-base "sigs.k8s.io/descheduler/pkg/api" \
    --input "v1alpha1" \
    --output-pkg "sigs.k8s.io/descheduler/pkg/generated/clientset" \
    --output-dir "." \
    -v 2

echo "Client generation complete!"
