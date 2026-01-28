#!/bin/bash

# K8Watch Quick Start Script

echo "🚀 K8Watch - Kubernetes Change Tracker"
echo "======================================"
echo ""

# Check prerequisites
echo "Checking prerequisites..."

if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.21 or later."
    exit 1
fi

if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl is not installed. Please install kubectl."
    exit 1
fi

# Test kubectl connection
if ! kubectl cluster-info &> /dev/null; then
    echo "❌ Cannot connect to Kubernetes cluster. Please configure kubectl."
    exit 1
fi

echo "✅ All prerequisites met!"
echo ""

# Install dependencies
echo "Installing Go dependencies..."
go mod download

echo ""
echo "Building K8Watch..."
go build -o k8watch ./cmd/k8watch

if [ $? -eq 0 ]; then
    echo "✅ Build successful!"
    echo ""
    echo "Starting K8Watch..."
    echo "Access the UI at: http://localhost:8080"
    echo ""
    echo "Press Ctrl+C to stop"
    echo ""
    ./k8watch
else
    echo "❌ Build failed. Please check the errors above."
    exit 1
fi
