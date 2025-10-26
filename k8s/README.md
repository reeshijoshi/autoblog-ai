# Kubernetes Deployment

This directory contains Kubernetes manifests for deploying AutoBlog AI.

## Important Note

**Topics and templates are baked into the Docker image** (not mounted from ConfigMaps).

This means:
- `topics.csv` and `templates/*.md` are included in the image at build time
- To change topics or prompts, edit the files, rebuild the Docker image, and redeploy
- Only `config.yaml` is mounted from ConfigMap for runtime configuration changes

## Prerequisites

- Kubernetes cluster (v1.24+)
- `kubectl` configured
- Container image pushed to registry

## Quick Start

1. **Update the Secret** with your API keys:

```bash
# Create secret from command line (recommended)
kubectl create secret generic autoblog-ai-secrets \
  --from-literal=ANTHROPIC_API_KEY=your-anthropic-key \
  --from-literal=MEDIUM_TOKEN=your-medium-token \
  --namespace autoblog-ai
```

2. **Update the image** in `cronjob.yaml` and `deployment.yaml`:

Replace `ghcr.io/yourusername/autoblog-ai:latest` with your actual image.

3. **Deploy**:

```bash
# From project root
make k8s-deploy

# Or manually
kubectl apply -f k8s/
```

## Files

- `namespace.yaml` - Creates the autoblog-ai namespace
- `secret.yaml` - API keys (template - use kubectl create secret instead)
- `configmap.yaml` - Application configuration (AI settings, style, etc.)
- `cronjob.yaml` - Weekly scheduled job (main deployment method)
- `deployment.yaml` - Optional deployment for manual runs
- `README.md` - This file

**Note**: topics.csv and templates/ are NOT in ConfigMaps - they're baked into the Docker image.

## Configuration

### CronJob Schedule

Edit `cronjob.yaml` to change the schedule:

```yaml
schedule: "0 10 * * 0"  # Every Sunday at 10:00 AM UTC
```

Common schedules:
- Daily: `"0 10 * * *"`
- Weekly: `"0 10 * * 0"` (Sunday)
- Monthly: `"0 10 1 * *"` (1st of month)

### Resource Limits

Adjust in `cronjob.yaml` or `deployment.yaml`:

```yaml
resources:
  requests:
    memory: "256Mi"
    cpu: "250m"
  limits:
    memory: "512Mi"
    cpu: "500m"
```

## Usage

### Check CronJob Status

```bash
make k8s-status

# Or
kubectl get cronjobs,jobs,pods -n autoblog-ai
```

### View Logs

```bash
make k8s-logs

# Or
kubectl logs -l app=autoblog-ai -n autoblog-ai --tail=100 -f
```

### Manual Trigger

Create a one-off job from the CronJob:

```bash
kubectl create job --from=cronjob/autoblog-ai manual-run-1 -n autoblog-ai
```

### Update Configuration

Edit the ConfigMap and restart:

```bash
kubectl edit configmap autoblog-ai-config -n autoblog-ai
kubectl rollout restart deployment/autoblog-ai -n autoblog-ai
```

## Security Best Practices

1. **Never commit secrets** - Use `kubectl create secret` or a secrets management solution

2. **Use Sealed Secrets** for GitOps:
   ```bash
   # Install sealed-secrets controller
   kubectl apply -f https://github.com/bitnami-labs/sealed-secrets/releases/download/v0.24.0/controller.yaml

   # Seal your secret
   kubeseal --format yaml < secret.yaml > sealed-secret.yaml
   ```

3. **Use External Secrets Operator** for cloud integration:
   - AWS Secrets Manager
   - GCP Secret Manager
   - Azure Key Vault

4. **Use RBAC** - Create ServiceAccount with minimal permissions

## Troubleshooting

### CronJob not running

```bash
# Check CronJob
kubectl describe cronjob autoblog-ai -n autoblog-ai

# Check recent jobs
kubectl get jobs -n autoblog-ai

# Check pod events
kubectl get events -n autoblog-ai --sort-by='.lastTimestamp'
```

### Pod crashes

```bash
# Check pod logs
kubectl logs -l app=autoblog-ai -n autoblog-ai --previous

# Describe pod for events
kubectl describe pod -l app=autoblog-ai -n autoblog-ai
```

### Secret not found

```bash
# Verify secret exists
kubectl get secret autoblog-ai-secrets -n autoblog-ai

# Check secret contents (base64 encoded)
kubectl get secret autoblog-ai-secrets -n autoblog-ai -o yaml
```

## Cleanup

```bash
make k8s-delete

# Or
kubectl delete -f k8s/
```
