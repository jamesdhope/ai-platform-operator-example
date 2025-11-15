````markdown
# AI Platform Operator - KServe Deployment Controller

A Kubernetes operator that deploys and manages KServe in RawDeployment mode with Ollama-based inference services.

## Overview

This operator manages a complete AI inference platform on Kubernetes:
- **KServe v0.11.0** in RawDeployment mode (no Knative/Istio required)
- **cert-manager** for TLS certificate management
- **Ollama + Gemma 2 2B** model for text generation inference
- Automatic configuration and deployment

## Architecture

- **KServe Controller**: Manages InferenceService resources
- **RawDeployment Mode**: Uses standard Kubernetes Deployments and Services
- **Ollama Runtime**: Serves Gemma 2 2B model via OpenAI-compatible API
- **Port 11434**: Inference API endpoint (Ollama standard)

## Quick Start

### Prerequisites

- Kubernetes cluster (Kind, Minikube, or cloud provider)
- kubectl configured
- Go 1.21+

### 1. Create Kind Cluster

```bash
kind create cluster --config kind-config.yaml --name ai-platform
```

### 2. Deploy the Operator

```bash
# Install CRD
kubectl apply -f config/crd/kservedeployment-crd.yaml

# Run operator locally
go run main.go
```

### 3. Deploy KServe Platform

```bash
# Deploy KServe with inference service
kubectl apply -f config/samples/kserve-minimal.yaml
```

The operator will:
1. Install KServe v0.11.0
2. Configure RawDeployment mode
3. Deploy Gemma 2 2B inference service
4. Wait for model download (~2-3 minutes)

### 4. Access the Inference API

```bash
# Port-forward to localhost
kubectl port-forward -n default svc/gemma2-2b-it-predictor 11434:80 &

# Test inference
curl -s http://localhost:11434/api/generate \
  -d '{"model":"gemma2:2b","prompt":"What is Kubernetes?","stream":false}' \
  | python3 -c "import sys, json; data=json.load(sys.stdin); print(data['response'])"
```

## Configuration

### KServeDeployment Resource

```yaml
apiVersion: platform.ai-platform.io/v1alpha1
kind: KServeDeployment
metadata:
  name: kserve-minimal
  namespace: default
spec:
  version: "v0.11.0"
  components:
    - kserve
```

### InferenceService (Gemma 2)

The operator automatically deploys:

```yaml
apiVersion: serving.kserve.io/v1beta1
kind: InferenceService
metadata:
  name: gemma2-2b-it
  annotations:
    serving.kserve.io/deploymentMode: RawDeployment
spec:
  predictor:
    containers:
      - name: kserve-container
        image: ollama/ollama:latest
        command: ["/bin/sh", "-c"]
        args:
          - "ollama serve &\nsleep 5\nollama pull gemma2:2b\nwait"
        resources:
          requests:
            cpu: "2"
            memory: "4Gi"
          limits:
            cpu: "3"
            memory: "6Gi"
```

## API Usage

### Generate Text (Non-Streaming)

```bash
curl -s http://localhost:11434/api/generate \
  -d '{
    "model": "gemma2:2b",
    "prompt": "Explain machine learning",
    "stream": false
  }' | jq -r '.response'
```

### Generate Text (Streaming)

```bash
curl -s http://localhost:11434/api/generate \
  -d '{
    "model": "gemma2:2b",
    "prompt": "Hello",
    "stream": true
  }' | jq -r '.response' | tr -d '\n'
```

## Project Structure

```
.
├── api/v1alpha1/           # CRD definitions
│   ├── kservedeployment_types.go
│   └── groupversion_info.go
├── controllers/            # Reconciliation logic
│   └── kservedeployment_controller.go
├── config/
│   ├── crd/               # CRD manifests
│   ├── samples/           # Example resources
│   │   ├── kserve-minimal.yaml
│   │   └── gemma2-inferenceservice.yaml
│   └── kserve-rawdeployment-patch.yaml
├── main.go                # Operator entry point
└── kind-config.yaml       # Local cluster config
```

## Operator Features

- **Declarative Deployment**: Apply KServeDeployment CR to install everything
- **RawDeployment Auto-Configuration**: Patches ConfigMap automatically
- **ConfigMap Protection**: Skips updating ConfigMaps on reconciliation to preserve settings
- **Inference Service Management**: Deploys model serving workloads
- **Version Control**: Pin KServe versions via spec.version

## Development

### Run Operator Locally

```bash
go run main.go > /tmp/operator.log 2>&1 &
```

### Check Logs

```bash
tail -f /tmp/operator.log
```

### Verify Deployment

```bash
# Check operator reconciliation
kubectl get kservedeployments -A

# Check KServe controller
kubectl get pods -n kserve

# Check inference service
kubectl get inferenceservices -n default
kubectl get pods -n default -l serving.kserve.io/inferenceservice=gemma2-2b-it
```

## Troubleshooting

### Model Download Taking Too Long

Check pod logs:
```bash
kubectl logs -n default -l serving.kserve.io/inferenceservice=gemma2-2b-it
```

### ConfigMap Reverted to Serverless

The operator now prevents this by skipping ConfigMap updates on reconciliation.

### Port-Forward Disconnected

Restart port-forward:
```bash
pkill -f "port-forward.*gemma2"
kubectl port-forward -n default svc/gemma2-2b-it-predictor 11434:80 &
```

### Inference Slow

The Gemma 2 2B model runs on CPU. Performance depends on:
- CPU allocation (currently 2 cores)
- Prompt complexity
- Expected: 10-30 seconds per response

## Cleanup

```bash
# Delete inference service
kubectl delete inferenceservice gemma2-2b-it

# Delete KServe deployment
kubectl delete kservedeployment kserve-minimal

# Delete cluster
kind delete cluster --name ai-platform
```

## What's Next

- Add support for additional models (Llama, Mistral, etc.)
- Implement GPU scheduling
- Add metrics and observability
- Multi-tenant inference services
- Model versioning and A/B testing

## License

MIT
````
# ai-platform-operator-example
