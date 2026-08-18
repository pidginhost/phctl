# phctl

Command-line interface for managing [PidginHost](https://pidginhost.com) cloud resources.

## Installation

### Quick install (Linux & macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/pidginhost/phctl/main/scripts/install.sh | sh
```

To install a specific version:

```bash
curl -fsSL https://raw.githubusercontent.com/pidginhost/phctl/main/scripts/install.sh | VERSION=v0.4.1 sh
```

### From binary releases

Download the latest release for your platform from the [Releases](https://github.com/pidginhost/phctl/releases) page.

### From source

```bash
go install github.com/pidginhost/phctl@latest
```

## Authentication

```bash
# Browser-based login (interactive)
phctl auth login

# Direct token (CI/CD pipelines, scripts)
phctl auth login --token <token>

# Or use environment variables (best for CI)
export PIDGINHOST_API_TOKEN=your-token
```

Other auth commands:

```bash
phctl auth init       # interactive token prompt
phctl auth set <tok>  # save token directly
phctl auth status     # show current auth
```

Config is stored at `~/.config/phctl/config.yaml`. Environment variables take precedence.

## Usage

```
phctl <resource> <command> [flags]
```

### Global flags

| Flag | Description |
|------|-------------|
| `-o, --output` | Output format: `table` (default), `json`, `yaml` |
| `-f, --force` | Skip confirmation prompts |

`-o json` / `-o yaml` applies to every command that reports a resource — reads
(`list`, `get`) and writes alike, so `compute server create -o json` emits the
created object rather than a sentence. Commands whose only result is a side
effect (`delete`, `power`, snapshot queueing), interactive prompts, and progress
narration stay human-readable; progress goes to stderr, so stdout under
`-o json` is safe to pipe.

### Resources

| Command | Alias | Description |
|---------|-------|-------------|
| `phctl auth` | | Authentication |
| `phctl account` | | Profile, SSH keys, companies, API tokens, email history |
| `phctl compute` | `c` | Servers, volumes, firewalls, IPs, networks, snapshots |
| `phctl domain` | `dns` | Domains, TLDs, registrants, nameservers, transfers |
| `phctl kubernetes` | `k8s` | Clusters, pools, nodes, LB firewall, port forwards, HTTP/TCP/UDP routes |
| `phctl storage` | | S3 buckets: quota, visibility, credentials |
| `phctl billing` | `bill` | Funds, deposits, invoices, services, subscriptions |
| `phctl dedicated` | `ded` | Dedicated servers |
| `phctl freedns` | `fdns` | FreeDNS domains and records |
| `phctl hosting` | `host` | Web hosting services |
| `phctl support` | | Support tickets and departments |
| `phctl update` | | Self-update to latest version |

### Examples

```bash
# List servers
phctl compute server list

# Create a server
phctl compute server create --image ubuntu-22 --package starter

# Create a server with a brand-new public IPv4, or with one you already own
phctl compute server create --image ubuntu-22 --package starter --new-ipv4
phctl compute server create --image ubuntu-22 --package starter --public-ip 203.0.113.7

# Same for IPv6 (--new-ipv6 allocates, --public-ipv6 attaches one you own)
phctl compute server create --image ubuntu-22 --package starter --new-ipv6

# Attach an IPv4 you own to a running server, restarting it so the guest sees it
phctl compute server attach-ipv4 123 --ipv4 203.0.113.7 --reboot

# Detach addresses again
phctl compute server detach-ipv4 123 --ipv4 203.0.113.7
phctl compute server detach-ipv6 123

# Change a server's package (restarts the server; asks first unless -f)
phctl compute server resize 123 --package cloudv-2

# Get server details as JSON
phctl compute server get 123 -o json

# Machine-readable create
phctl compute server create --image ubuntu-22 --package starter -o json | jq .id

# Manage Kubernetes clusters
phctl k8s cluster list
phctl k8s cluster kubeconfig my-cluster
phctl k8s cluster kubeconfig 42 --regenerate   # invalidates every existing copy

# Cluster settings: rename, delete protection, feature set
phctl k8s cluster update 42 --name prod
phctl k8s cluster update 42 --protected
phctl k8s cluster update 42 --features cert-manager,metrics-server   # replaces the set

# Cluster features and cloud VM access
phctl k8s cluster upgrade-feature 42 --feature cert-manager
phctl k8s cluster toggle-vm-access 42          # flips the setting; run twice to undo
phctl k8s cluster eligible-vms 42              # empty while VM access is off

# Resource pools and their nodes
phctl k8s pool get 42 3
phctl k8s pool resize 42 3 --size 4            # provisions or destroys billable VMs
phctl k8s node get 42 3 11
phctl k8s node metrics 42 3 11
phctl k8s node rrd 42 3 11 --timeframe week

# Load balancer port forwards (lb-firewall governs who may reach them)
phctl k8s port-forward list 42
phctl k8s port-forward create 42 --internal-ip 10.0.0.50 --port 8080 --protocol tcp
phctl k8s port-forward update 42 9 --port 9090   # sends only the flags you pass
phctl k8s port-forward delete 42 9

# Load balancer firewall
phctl k8s lb-firewall list 42
phctl k8s lb-firewall create 42 --direction in --action ACCEPT --protocol tcp --dport 443
phctl k8s lb-firewall update 42 5 --dport 8443 # sends only the flags you pass
phctl k8s lb-firewall delete 42 5

# Gateway routes
phctl k8s http-route get 42 4
phctl k8s http-route update 42 4 --path-prefix /api   # partial: omitted fields keep their value
phctl k8s tcp-route create 42 --name pg --port 5432 \
  --backend pg-svc --backend-port 5432 --backend-namespace database

# Domain management
phctl domain create example.ro --years 1
phctl domain check example.ro

# Server operations
phctl compute server usage 42
phctl compute server activity 42
phctl compute server boot-isos 42            # per-server: compatibility depends on its disk/RAM
phctl compute server rescue enter 42          # default rescue image; --iso <slug> for another
phctl compute server rescue exit 42
phctl compute server retry-provision 42

# A server's public interface (addresses are platform-assigned; firewall is not)
phctl compute server public-interface get 42
phctl compute server public-interface set 42 --firewall web --policy-in DROP
phctl compute server public-interface delete 42

# Reverse DNS on a standalone IP
phctl compute ipv6 reverse-dns 9 --hostname host.example.com

# Object storage
phctl storage bucket list
phctl storage bucket create --name assets --quota 50
phctl storage bucket resize 7 --quota 100
phctl storage bucket visibility 7 --public     # asks first; --private needs no prompt
phctl storage bucket delete 7

# Bucket credentials -- these grant full read/write access to the bucket.
# reveal asks before printing; rotate kills the current pair immediately.
phctl storage bucket credentials reveal 7
phctl storage bucket credentials rotate 7

# Billing
phctl billing funds balance
phctl billing invoice list

# Support tickets
phctl support ticket create --subject "Help" --department 1 --message "Issue..."

# Delete with confirmation skip
phctl compute server delete 123 -f
```

### Command aliases

- `kubernetes` → `k8s`, `compute` → `c`, `domain` → `dns`
- `billing` → `bill`, `dedicated` → `ded`, `freedns` → `fdns`
- `hosting` → `host`
- `delete` → `rm` or `destroy`

## Building

```bash
go build -o phctl .
```

## License

See [LICENSE](LICENSE) for details.
