# Test Infrastructure Overlay

This overlay deploys the complete Milo system for testing environments with all required components.

## Directory Structure

```
test-infra/
├── kustomization.yaml         # Main configuration file
├── components/                # Modular, reusable test components
│   ├── auth/                  # Test authentication and secrets
│   └── certificates/          # TLS certificates via cert-manager
├── patches/                   # Modifications to base resources
└── resources/                 # Additional test-specific resources
```

## Components

### Auth Component
Contains test-specific authentication resources:
- Pre-configured auth tokens for testing
- Service account keys
- Controller manager kubeconfig

### Certificates Component
Manages TLS certificates for the test environment:
- ClusterIssuer for cert-manager
- Certificate resources for API server TLS

## Shared Resources

### Gateway API Configuration
This overlay uses the shared Gateway API configuration from `config/components/gateway-api/` which provides:
- HTTPRoute for routing traffic to the API server
- BackendTLSPolicy for TLS backend connections

The Gateway API resources are shared across environments and can be customized per environment if needed.

## Usage

### Build Manifests
```bash
kubectl kustomize config/overlays/test-infra
```

### Deploy to Cluster
```bash
kubectl apply -k config/overlays/test-infra
```

### Verify Deployment
```bash
kubectl get pods -n milo-system
kubectl get httproute -n milo-system
```

## Customization

To customize for different environments:

1. **Different Gateway**: Update the Gateway API component reference in `kustomization.yaml` (now located at `../../components/gateway-api`)
2. **Custom Certificates**: Modify or replace the certificates component
3. **Additional Auth**: Extend the auth component with new secrets
4. **Environment Patches**: Add new patches in the `patches/` directory

## Dependencies

- Kubernetes cluster with Gateway API CRDs installed
- cert-manager for certificate management
- Envoy Gateway (or compatible Gateway API implementation)

## Testing

After deployment, the API server will be accessible through the configured
Gateway at the HTTPRoute endpoint.
