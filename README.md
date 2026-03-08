# phctl

Command-line interface for managing [PidginHost](https://pidginhost.com) cloud resources.

## Installation

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

### Resources

| Command | Alias | Description |
|---------|-------|-------------|
| `phctl auth` | | Authentication |
| `phctl account` | | Profile, SSH keys, companies, API tokens, email history |
| `phctl compute` | `c` | Servers, volumes, firewalls, IPs, networks, snapshots |
| `phctl domain` | `dns` | Domains, TLDs, registrants, nameservers, transfers |
| `phctl kubernetes` | `k8s` | Clusters, pools, nodes, HTTP/TCP/UDP routes |
| `phctl billing` | `bill` | Funds, deposits, invoices, services, subscriptions |
| `phctl dedicated` | `ded` | Dedicated servers |
| `phctl freedns` | `fdns` | FreeDNS domains and records |
| `phctl hosting` | `host` | Web hosting services |
| `phctl support` | `ticket` | Support tickets |

### Examples

```bash
# List servers
phctl compute server list

# Create a server
phctl compute server create --image ubuntu-22 --package starter

# Get server details as JSON
phctl compute server get 123 -o json

# Manage Kubernetes clusters
phctl k8s cluster list
phctl k8s cluster kubeconfig my-cluster

# Domain management
phctl domain create example.ro --years 1
phctl domain check example.ro

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
- `hosting` → `host`, `support` → `ticket`
- `delete` → `rm` or `destroy`

## Building

```bash
go build -o phctl .
```

## License

See [LICENSE](LICENSE) for details.
