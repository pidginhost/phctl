# Changelog

## v0.2.2

### Changed

- Upgraded SDK to `github.com/pidginhost/sdk-go` v0.3.0
  - Datetime fields changed from `time.Time` to `string` (API uses naive datetimes)
  - Server `cpus`/`memory`/`disk_size` changed from `string` to `int32`
  - `CloudServersRetrieve` now returns `*ServerDetail` directly

### Fixed

- **Decimal field deserialization**: API returns decimal strings (`"123.45"`) but SDK types them as `float64`. Added raw HTTP bypass (`RawGet`/`RawFetchAll`) for all affected endpoints:
  - Account: profile (funds)
  - Billing: funds balance, deposits, invoices, services, subscriptions
  - Domain: list, get, TLD list
  - Dedicated: server list, get
  - Hosting: service list, get
  - Kubernetes: cluster list, get
- **Billing funds balance**: API returns single object, not array — bypassed SDK array expectation
- **Route/token constructors**: updated for SDK v0.3.0 signature changes

### Added

- E2E test suite (`e2e/e2e_test.go`) covering all read-only list endpoints (30 tests)
- `RawGet` and `RawFetchAll` helpers in `internal/client` for bypassing SDK type mismatches
- Changelog-driven release notes for both GitLab and GitHub releases

## v0.2.1

### Fixed

- Auth login: poll response field name (`token_key` instead of `token`)

### Added

- GitHub sync CI stage and manual GitHub release workflow

## v0.2.0

### Added

**Browser-based login**
- `auth login` — opens browser for approval, polls for token
- `auth login --token <token>` — direct token for CI/CD pipelines

**Billing** (`billing` / `bill`)
- Funds: balance, activity log
- Deposits: list, get, create
- Invoices: list, get, pay with funds
- Services: list, get, cancel, toggle auto-pay
- Subscriptions: list, get

**Dedicated servers** (`dedicated` / `ded`)
- Server management: list, get, power control, OS reinstall, reverse DNS

**FreeDNS** (`freedns` / `fdns`)
- Domain management: list, activate, deactivate
- DNS records: list, create, delete

**Web hosting** (`hosting` / `host`)
- Hosting services: list, get, change cPanel password

**Support tickets** (`support` / `ticket`)
- Departments: list
- Tickets: list, get, create, reply, close, reopen

**Account extras**
- API token management: list, create, delete
- Email history: list

**Domain extras**
- `domain cancel` — cancel a domain
- `domain transfer` — transfer with auth code
- `domain nameservers` — update nameservers

### Changed

- Upgraded SDK to `github.com/pidginhost/sdk-go` v0.2.0

### Fixed

- Firewall rule direction enum type

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
