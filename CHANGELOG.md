# Changelog

## v0.1.0

Initial release of `phctl`, the PidginHost CLI.

### Features

**Authentication**
- `auth init` — interactive API token setup
- `auth set` — set token directly
- `auth status` — show current auth status
- Config file at `~/.config/phctl/config.yaml`
- Environment variable overrides (`PIDGINHOST_API_TOKEN`, `PIDGINHOST_API_URL`)

**Compute**
- Server management: list, get, create, delete, power actions, console access
- Server snapshots: list, create, delete, rollback
- IP attachment: attach/detach IPv4 and IPv6 to servers
- Volume management: list, get, delete, attach, detach
- Firewall management: list, get, create, delete
- Firewall rules: list, create, delete
- IPv4 addresses: list, create, delete, detach
- IPv6 addresses: list, create, delete, detach
- Private networks: list, get, create, delete, add/remove servers
- Server packages: list
- OS images: list

**Domains**
- Domain management: list, get, create (register), check availability, renew
- TLD listing
- Registrant management: list, get, create, delete

**Kubernetes**
- Cluster management: list, get, create, delete
- Cluster operations: kubeconfig, upgrade Kubernetes version, upgrade Talos version
- VM connectivity: connect, disconnect, list connected VMs
- Cluster types listing
- Resource pool management: list, create, delete
- Node management: list, delete
- HTTP/TCP/UDP route management: list, create, delete

**Account**
- Profile viewing
- SSH key management: list, create, delete
- Company listing

**Output & UX**
- Multiple output formats: table (default), JSON, YAML
- Confirmation prompts on destructive operations (skip with `--force`)
- Command aliases for common operations

**CI/CD**
- GitLab CI pipeline for multi-platform builds (linux/darwin/windows, amd64/arm64)
- Automated release creation with binary artifacts
