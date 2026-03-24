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

# Output:
# TYPE  MIN WORKERS  MAX WORKERS  PACKAGES
# dev   1            3            8
# prod  2            20           8
```

To see the available packages (worker node sizes) in detail:

```bash
phctl k8s types -o json
```

Common packages: `cloudv-1` (smallest) through `cloudv-8` (largest).

## Step 2: Create a Cluster

> **Note:** `phctl k8s cluster create` currently has a known SDK issue where
> the `--package` flag maps to the wrong API field. Use the workaround below
> until the SDK is updated. All other commands (get, list, delete, kubeconfig,
> routes, pools) work correctly.

**Workaround using curl:**

```bash
# Get your token
TOKEN=$(grep auth_token ~/.config/phctl/config.yaml | awk '{print $2}')

# Create a dev cluster with 1 worker (cloudv-3 package)
curl -s -X POST "https://www.pidginhost.com/api/kubernetes/clusters/" \
  -H "Authorization: Token $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "cluster_type": "dev",
    "resource_pool_package": "cloudv-3",
    "name": "my-cluster",
    "resource_pool_size": 1
  }'
# Output: {"id": 42}
```

Save the cluster ID from the output:

```bash
CLUSTER_ID=42
```

Poll until active (~5-6 minutes for a dev cluster):

```bash
phctl k8s cluster get $CLUSTER_ID
# Repeat until Status: active
```

Or use `--wait` on a loop:

```bash
while [ "$(phctl k8s cluster get $CLUSTER_ID -o json | python3 -c 'import sys,json; print(json.load(sys.stdin)["status"])')" != "active" ]; do
  sleep 15
done
echo "Cluster is active!"
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
  --package "cloudv-5" \
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

If you used `--merge` to add the kubeconfig, clean up afterwards:

```bash
# Replace CONTEXT, CLUSTER, USER with the names from your kubeconfig
kubectl config delete-context <context-name>
kubectl config delete-cluster <cluster-name>
kubectl config delete-user <user-name>
kubectl config use-context <your-default-context>
```

## CI/CD Example

Fully scripted cluster lifecycle for automation:

```bash
#!/usr/bin/env bash
set -euo pipefail

TOKEN="${PIDGINHOST_API_TOKEN}"

# Create cluster via API (SDK workaround)
CLUSTER_ID=$(curl -sf -X POST "https://www.pidginhost.com/api/kubernetes/clusters/" \
  -H "Authorization: Token $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"cluster_type\": \"dev\",
    \"resource_pool_package\": \"cloudv-3\",
    \"name\": \"ci-${CI_COMMIT_SHORT_SHA}\",
    \"resource_pool_size\": 1
  }" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")

echo "Cluster created: $CLUSTER_ID"

# Wait for active (~5-6 minutes)
while true; do
  STATUS=$(phctl k8s cluster get "$CLUSTER_ID" -o json | \
    python3 -c "import sys,json; print(json.load(sys.stdin)['status'])")
  echo "Status: $STATUS"
  [ "$STATUS" = "active" ] && break
  [ "$STATUS" = "failed" ] && echo "Cluster failed!" && exit 1
  sleep 15
done

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
| Get cluster details | `phctl k8s cluster get ID` |
| Get kubeconfig | `phctl k8s cluster kubeconfig ID [--merge]` |
| Delete cluster | `phctl k8s cluster delete ID [-f]` |
| Upgrade k8s | `phctl k8s cluster upgrade-kube ID [--wait]` |
| Upgrade Talos | `phctl k8s cluster upgrade-talos ID [--wait]` |
| List types | `phctl k8s types` |
| List pools | `phctl k8s pool list CLUSTER_ID` |
| Add pool | `phctl k8s pool create CLUSTER_ID --package P --size N [--wait]` |
| List nodes | `phctl k8s node list CLUSTER_ID POOL_ID` |
| HTTP route | `phctl k8s http-route create ID --name N --hostname H --backend B --port P [--tls]` |
| TCP route | `phctl k8s tcp-route create ID --name N --port P --backend B --backend-port BP` |
| UDP route | `phctl k8s udp-route create ID --name N --port P --backend B --backend-port BP` |
| Connect VM | `phctl k8s cluster connect-vm ID --server S` |
| List connected VMs | `phctl k8s cluster connected-vms ID` |
