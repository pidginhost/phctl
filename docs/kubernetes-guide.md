# Kubernetes Deployment Guide

End-to-end guide for deploying applications on PidginHost Kubernetes using `phctl`.

## Prerequisites

```bash
# Authenticate (pick one)
phctl auth login                              # Interactive browser login
phctl auth login --token YOUR_API_TOKEN       # Direct token
export PIDGINHOST_API_TOKEN=YOUR_API_TOKEN    # Environment variable

# Verify authentication
phctl account profile
```

You also need `kubectl` installed: https://kubernetes.io/docs/tasks/tools/

## Step 1: Discover Available Options

```bash
# List cluster types and their constraints
phctl k8s types

# Output example:
# TYPE        MIN WORKERS  MAX WORKERS  PACKAGES
# standard    1            10           3
# high-mem    1            5            2
```

Note the TYPE and PACKAGES values — you'll need them for cluster creation.

## Step 2: Create a Cluster

```bash
# Basic creation (returns immediately)
phctl k8s cluster create \
  --name my-cluster \
  --type standard \
  --package "ph-k8s-std-2" \
  --pool-size 2 \
  --kube-version "1.32"

# With --wait: blocks until cluster is active (ideal for CI/CD)
phctl k8s cluster create \
  --name my-cluster \
  --type standard \
  --package "ph-k8s-std-2" \
  --pool-size 2 \
  --wait \
  --wait-timeout 15m
```

Save the cluster ID from the output — you'll use it in every subsequent command.

```bash
CLUSTER_ID=42
```

If you didn't use `--wait`, poll manually:

```bash
phctl k8s cluster get $CLUSTER_ID
# Repeat until Status: active
```

## Step 3: Get the Kubeconfig

**Option A** — Save to a separate file:
```bash
phctl k8s cluster kubeconfig $CLUSTER_ID > ~/.kube/pidginhost.yaml
export KUBECONFIG=~/.kube/pidginhost.yaml
```

**Option B** — Merge into your existing kubeconfig (recommended):
```bash
phctl k8s cluster kubeconfig $CLUSTER_ID --merge
# Output: Kubeconfig merged into /home/user/.kube/config and context set.
```

This upserts the cluster, user, and context entries into `~/.kube/config`
(or `$KUBECONFIG`) and switches to the new context automatically.

Verify connectivity:

```bash
kubectl get nodes
kubectl get namespaces
```

## Step 4: Deploy an Application

```bash
# Create a namespace
kubectl create namespace demo

# Deploy nginx with 2 replicas
kubectl create deployment nginx \
  --image=nginx:latest \
  --replicas=2 \
  -n demo

# Expose it as a ClusterIP service
kubectl expose deployment nginx \
  --port=80 \
  --target-port=80 \
  --type=ClusterIP \
  -n demo

# Verify pods are running
kubectl get pods -n demo
kubectl get svc -n demo
```

## Step 5: Expose via HTTP Route (with TLS)

```bash
# Create an HTTP route with automatic TLS certificate
phctl k8s http-route create $CLUSTER_ID \
  --name nginx-route \
  --hostname app.example.com \
  --backend nginx \
  --port 80 \
  --namespace demo \
  --tls

# Verify the route
phctl k8s http-route list $CLUSTER_ID
```

Make sure your DNS A record for `app.example.com` points to the cluster's
IPv4 address (visible in `phctl k8s cluster get $CLUSTER_ID`).

### TCP/UDP routes

```bash
# Expose a TCP service (e.g., PostgreSQL)
phctl k8s tcp-route create $CLUSTER_ID \
  --name pg-route \
  --port 5432 \
  --backend my-postgres \
  --backend-port 5432 \
  --namespace demo

# Expose a UDP service (e.g., DNS)
phctl k8s udp-route create $CLUSTER_ID \
  --name dns-route \
  --port 53 \
  --backend coredns \
  --backend-port 53 \
  --namespace kube-system
```

## Step 6: Scale with Resource Pools

```bash
# Add a larger pool for memory-intensive workloads
phctl k8s pool create $CLUSTER_ID \
  --package "ph-k8s-std-4" \
  --size 1 \
  --wait

# List pools and their nodes
phctl k8s pool list $CLUSTER_ID
POOL_ID=5
phctl k8s node list $CLUSTER_ID $POOL_ID
```

## Step 7: Connect a Cloud VM (Optional)

Bridge a PidginHost cloud server into the cluster's private network:

```bash
# Get your server ID
phctl compute server list

# Connect it to the cluster
phctl k8s cluster connect-vm $CLUSTER_ID --server 123

# Verify
phctl k8s cluster connected-vms $CLUSTER_ID
```

## Step 8: Upgrade Kubernetes / Talos

```bash
# Upgrade Kubernetes version (blocks until complete with --wait)
phctl k8s cluster upgrade-kube $CLUSTER_ID --wait

# Upgrade Talos OS version
phctl k8s cluster upgrade-talos $CLUSTER_ID --wait
```

## Teardown

```bash
# Remove routes
phctl k8s http-route delete $CLUSTER_ID <route-id> -f

# Remove extra pools
phctl k8s pool delete $CLUSTER_ID <pool-id> -f

# Destroy the cluster
phctl k8s cluster delete $CLUSTER_ID -f
```

## CI/CD Example

Fully scripted cluster lifecycle for automation:

```bash
#!/usr/bin/env bash
set -euo pipefail

# Create and wait (outputs human-readable text, e.g. "Cluster created (ID: 42)")
CLUSTER_OUTPUT=$(phctl k8s cluster create \
  --name "ci-${CI_COMMIT_SHORT_SHA}" \
  --type standard \
  --package "ph-k8s-std-2" \
  --pool-size 1 \
  --wait \
  --wait-timeout 15m)

CLUSTER_ID=$(echo "$CLUSTER_OUTPUT" | grep -oP 'ID: \K\d+')

# Merge kubeconfig for kubectl access
phctl k8s cluster kubeconfig "$CLUSTER_ID" --merge

# Deploy
kubectl apply -f k8s/

# Expose
phctl k8s http-route create "$CLUSTER_ID" \
  --name app \
  --hostname "ci-${CI_COMMIT_SHORT_SHA}.example.com" \
  --backend myapp \
  --port 8080 \
  --namespace default \
  --tls

# Run tests against the deployment...
# kubectl run tests ...

# Cleanup
phctl k8s cluster delete "$CLUSTER_ID" -f
```

## JSON/YAML Output

Read commands (list, get, kubeconfig) support `--output json` or `--output yaml`.
Mutating commands (create, delete, upgrade) print human-readable status text only.

```bash
phctl k8s cluster list -o json
phctl k8s cluster get $CLUSTER_ID -o yaml
phctl k8s http-route list $CLUSTER_ID -o json
phctl k8s cluster kubeconfig $CLUSTER_ID -o json
```

## Quick Reference

| Task | Command |
|------|---------|
| List clusters | `phctl k8s cluster list` |
| Create cluster | `phctl k8s cluster create --type T --package P [--wait]` |
| Get kubeconfig | `phctl k8s cluster kubeconfig ID [--merge]` |
| Delete cluster | `phctl k8s cluster delete ID [-f]` |
| Upgrade k8s | `phctl k8s cluster upgrade-kube ID [--wait]` |
| List pools | `phctl k8s pool list CLUSTER_ID` |
| Add pool | `phctl k8s pool create CLUSTER_ID --package P --size N` |
| List nodes | `phctl k8s node list CLUSTER_ID POOL_ID` |
| HTTP route | `phctl k8s http-route create ID --name N --hostname H --backend B --port P [--tls]` |
| TCP route | `phctl k8s tcp-route create ID --name N --port P --backend B --backend-port BP` |
| Connect VM | `phctl k8s cluster connect-vm ID --server S` |
