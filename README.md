# phctl

Command-line interface for managing [PidginHost](https://pidginhost.com) cloud resources.

## Installation

### From binary releases

Download the latest release for your platform from the [Releases](https://git.pidginhost.net/pidginhost/phctl/-/releases) page.

### From source

```bash
go install github.com/pidginhost/phctl@latest
```

## Authentication

Get your API token from the [PidginHost dashboard](https://dashboard.pidginhost.com).

```bash
# Interactive setup
phctl auth init

# Or set directly
phctl auth set <token>

# Check status
phctl auth status
```

You can also use environment variables:

```bash
export PIDGINHOST_API_TOKEN=your-token
export PIDGINHOST_API_URL=https://api.pidginhost.com   # optional
```

Config is stored at `~/.config/phctl/config.yaml`. Environment variables take precedence over the config file.

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

| Command | Description |
|---------|-------------|
| `phctl auth` | Manage authentication |
| `phctl account` | Profile, SSH keys, companies |
| `phctl compute` | Cloud servers, volumes, firewalls, IPs, networks |
| `phctl domain` | Domains, TLDs, registrants |
| `phctl kubernetes` | Clusters, pools, nodes, routes |

### Examples

```bash
# List servers
phctl compute server list

# Create a server
phctl compute server create --hostname my-server --image ubuntu-22 --package starter

# Get server details (JSON)
phctl compute server get 123 -o json

# Manage firewalls
phctl compute firewall list
phctl compute firewall rule list 1

# List Kubernetes clusters
phctl k8s cluster list

# Get kubeconfig
phctl k8s cluster kubeconfig my-cluster

# Register a domain
phctl domain create example.ro --years 1

# Check domain availability
phctl domain check example.ro

# Allocate an IPv4 address
phctl compute ipv4 create

# Delete with confirmation skip
phctl compute server delete 123 -f
```

### Command aliases

Several commands have shorter aliases:

- `kubernetes` → `k8s`
- `compute` → (subcommands: `server` → `srv`, `ipv4` → `ip`, `network` → `net`, `package` → `pkg`)
- `domain` → `dns`
- `ssh-key` → `ssh`
- `delete` → `rm` or `destroy`

## Building

```bash
go build -o phctl .
```

Cross-compile:

```bash
GOOS=linux GOARCH=amd64 go build -o phctl-linux-amd64 .
GOOS=darwin GOARCH=arm64 go build -o phctl-darwin-arm64 .
GOOS=windows GOARCH=amd64 go build -o phctl-windows-amd64.exe .
```

## License

See [LICENSE](LICENSE) for details.
