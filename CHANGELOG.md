# Changelog

## v0.13.0

### Added

- **`compute floating-ip reverse-dns <id>` (alias `rdns`)** reads the current PTR record for a floating IP. Pass `--hostname <fqdn>` to set it. Combine with `--ipv6` to target a floating IPv6. Backed by the new `/cloud/floating-ipv{4,6}/{id}/rdns/` endpoints — until now, floating IPs had no API path for rDNS because the regular `ipv4`/`ipv6` viewsets explicitly excluded floating-backed IPs.

### Changed

- **Dependency updates**: sdk-go v0.9.0 → v0.10.1 (new `CloudFloatingIpv4Rdns{Retrieve,Create}` and `CloudFloatingIpv6Rdns{Retrieve,Create}` methods; restored typing on `CloudFloatingIpv{4,6}AuthorizationsList` after a brief v0.10.0 regression that named it `Retrieve` and typed the response as a single floating IP).

## v0.12.2

### Fixed

- **`cmdutil.APIError` now actually extracts SDK response bodies**. v0.12.1 declared the target as `var apiErr GenericOpenAPIError` (value), but the SDK returns the error as `*GenericOpenAPIError`, so `errors.As` never matched and the body branch was unreachable — the release was effectively a no-op for SDK errors. Target is now `*GenericOpenAPIError`, and an httptest-backed integration test pins the behaviour so this can't silently regress.
- **`compute floating-ip authorize`/`unauthorize` errors** also surface the backend body now (previously missed because their `fmt.Errorf` used `%d, %d, %w` not the `: %w` pattern the bulk-rewrite matched).

## v0.12.1

### Changed

- **`compute` commands now surface the backend error body** on 4xx/5xx responses. The previous wrapping dropped the JSON payload, leaving cryptic messages like `attaching IPv4: 400 Bad Request` with no clue why. The new `cmdutil.APIError` helper extracts the body and flattens the validation errors (e.g. `attaching IPv4: 400 Bad Request: ipv4=Invalid pk "1"`). Wrapped errors still chain so `errors.As(err, &pidginhost.GenericOpenAPIError{})` keeps working.

## v0.12.0

### Added

- **`compute server detach-ipv4 <id>`**: detach an IPv4 from a VM. Without `--ipv4`, the primary NIC's IPv4 is detached. Pass `--ipv4 <id|slug>` to target a specific attached address (required when the server has more than one IPv4 after the backend's multi-NIC support landed).
- **Both `compute server attach-ipv4` and `detach-ipv4` now read the backend's typed response** (`Attached`/`Detached` boolean) and fail loudly when the backend says the operation was a no-op — closes the "phctl can't tell success from no-op" gap from v0.11.0.

### Changed

- **`compute server attach-ipv4` help text rewritten**: the "one IPv4 per server" caveat no longer applies — the backend now creates a new secondary NIC on the second and subsequent attaches.
- **Dependency updates**: sdk-go v0.8.0 → v0.9.0 (typed `AttachIPv4Response`/`DetachIPv4Response` models, `Ipv4()` builder on `ApiCloudServersDetachIpv4CreateRequest` for the new `?ipv4=` query param).

## v0.11.0

### Added

- **`compute server create --generation <slug>`**: pick a hardware generation at provision time (e.g. `compute-optimized`). Omit to use the backend's default generation. SDK already shipped `ServerAdd.Generation` in v0.8.0; this exposes it on the CLI.

### Documented

- **`compute server attach-ipv4` 1-IPv4-per-server limit**: the backend caps attachments at one IPv4 per server via this endpoint, and a second call returns success but is a no-op. phctl cannot currently distinguish the two cases — the backend's success and "already attached" responses use the same OpenAPI schema (`inline_serializer("AttachIPv4", ...)`) which collides with the request body type, so the SDK deserialises both into an empty `AttachIPv4{}`. The `Long:` help on the command now warns about the limit and points at `compute floating-ip authorize` for multi-IP-per-VM scenarios.

## v0.10.2

### Fixed

- **`compute ipv4 reverse-dns`** now validates `--hostname ""` before initialising the API client, so the error surfaces even when the config or auth env is missing. Includes a regression test.

## v0.10.1

### Fixed

- **`compute package list` traffic column now labelled `TRAFFIC_GB`** to match the actual unit (included traffic is stored in GB, not unitless or TB as the column previously implied).
- **`compute ipv4 reverse-dns --hostname ""`** now rejects an empty FQDN client-side with a clear error instead of round-tripping to the API and surfacing the validator's 400.

## v0.10.0

### Added

- **`compute package list` shows full specs and accepts a generation filter**. The table gains `CPUS`, `MEMORY_GB`, `DISK_GB`, `TRAFFIC` columns sourced from the package config (no more probing an existing server to learn what a SKU contains). `--generation <slug>` narrows the list to packages allowed on that hardware generation; the backend hides free-tier-only packages on generations not flagged free-tier eligible.
- **`compute ipv4 reverse-dns <id>` (alias `rdns`)** reads the current PTR record for an IPv4 address. Pass `--hostname <fqdn>` to set it via `POST /cloud/ipv4/<id>/rdns/`.

### Changed

- **Dependency updates**: sdk-go v0.7.0 → v0.8.0 (typed `ServerProduct.Cpus/Memory/DiskSize/Traffic` fields and the `ApiCloudServerPackagesListRequest.Generation()` builder).

## v0.9.1

### Changed

- **Floating-IP commands switch to typed sdk-go v0.7.0 methods**. `compute floating-ip {list,create,delete,authorize,unauthorize,authorizations}` now uses the generated `CloudFloatingIpv4*`/`CloudFloatingIpv6*` API surface and model types instead of the raw HTTP helpers in `internal/client`. Behaviour, flags, and output are unchanged; pagination on `list` and `authorizations` now flows through `cmdutil.FetchAll` like the rest of the compute commands.
- **Dependency updates**: sdk-go v0.6.0 → v0.7.0 (adds floating-IP API surface).

## v0.9.0

### Added

- **`compute floating-ip` commands**: `list`, `create`, `delete`, `authorize`, `unauthorize`, `authorizations` for managing floating IPs that can be authorized on multiple servers simultaneously (multi-VM HA via keepalived/VRRP inside the guest). Use `--ipv6` to target IPv6 floating addresses; otherwise the IPv4 endpoints are hit.

### Changed

- **Floating-IP commands use raw HTTP via the `internal/client` helpers** instead of generated SDK methods, so phctl ships floating-IP support without waiting for an sdk-go release. New helpers `RawGet`, `RawPost`, `RawDelete`, `RawFetchAll` cover the basic CRUD + paginated-list shape.

## v0.8.0

### Added

- **`server create --user-data` and `--user-data-file`**: pass a cloud-init startup script (bash with shebang or `#cloud-config` YAML) at provision time. Linux images only. Use `--user-data-file -` to read from stdin. Capped at 64 KiB to match the API.

### Changed

- **Dependency updates**: sdk-go v0.5.0 -> v0.6.0 (adds `ServerAdd.UserData` field).

## v0.7.0

### Security

- **Self-update integrity verification**: `phctl update` now downloads the release's `checksums.txt` asset and verifies the SHA-256 of the downloaded binary before applying it. Releases without `checksums.txt` are rejected so a tampered or partial download cannot replace the running binary
- **`auth set` no longer takes the token as a positional argument** (BREAKING): the token is read from stdin (`echo "$TOKEN" | phctl auth set`) so it cannot leak via `ps`, `/proc/<pid>/cmdline`, or shell history

### Added

- **Atomic writes for `config.yaml` and merged kubeconfig**: both files are now written via temp-file + rename via the new `cmdutil.WriteAtomic` helper, so a crash or a concurrent save can't leave a half-written file at the destination

### Changed

- **Go version**: bumped minimum from 1.25 to 1.26; CI images updated accordingly
- **Dependency updates**: sdk-go v0.4.1 → v0.5.0, x/sys v0.42.0 → v0.43.0, x/term v0.41.0 → v0.42.0
- **GitHub API requests**: `phctl update` now sends a `phctl/<version>` `User-Agent` header on both the release-metadata and asset-download calls

### Fixed

- **`update` claimed "Already up to date" for development builds**: when `version` is unparseable (empty, "dev", commit SHA, etc.), the command now prints "Running a development build" and refuses to overwrite the binary
- **`auth login` aborted on a single transient 5xx**: the poll now treats 5xx as retryable up to the existing 3-error budget; 4xx still fails fast
- **Unbounded pagination loops**: `cmdutil.FetchAll` and `internal/client.RawFetchAll` now error out after 10000 pages instead of looping forever if the API never signals the last page

## v0.6.2

### Fixed

- **Silent API failures**: updated sdk-go from v0.4.0 to v0.4.1 — removes `DisallowUnknownFields` from all model deserialization, fixing silent failures when the API returns new fields

## v0.6.1

### Fixed

- **`server list` / `server get` crash**: updated sdk-go from v0.3.0 to v0.4.0 to include the `Server.generation` field now returned by the API

## v0.6.0

### Added

- **`--wait` / `--wait-timeout` for async Kubernetes operations**: `cluster create`, `cluster upgrade-kube`, `cluster upgrade-talos`, and `pool create` now support `--wait` to block until the cluster reaches active state, with a configurable `--wait-timeout` (default 10m). Ideal for CI/CD pipelines
- **`--merge` flag for kubeconfig**: `cluster kubeconfig --merge` upserts cluster, context, and user entries into the existing kubeconfig file (`~/.kube/config` or `$KUBECONFIG`), sets the new context as current, and preserves all existing top-level fields including extensions
- **Decimal type**: replaced raw `json.Number` fields with a purpose-built `Decimal` type that accepts both JSON strings and numbers on unmarshal, emits bare JSON numbers on marshal, and preserves exact precision in YAML output (e.g. `42.50` stays `42.50`)
- **Kubernetes deployment guide**: `docs/kubernetes-guide.md` with end-to-end walkthrough covering cluster creation, kubeconfig, app deployment, HTTP/TCP/UDP routes, resource pools, VM connectivity, upgrades, teardown, and a CI/CD script example

### Fixed

- **Inconsistent ID validation**: `cluster delete`, `pool delete`, `node delete`, and all route delete commands now validate numeric IDs client-side before making API calls, matching the pattern already used by list and create commands
- **Opaque API error messages**: `RawGet` now reads up to 512 bytes of the HTTP error response body, surfacing server error details instead of just the status code
- **Missing route test coverage**: added tests for `http-route`, `tcp-route`, and `udp-route` subcommand registration and flag presence

### Changed

- **golangci-lint**: CI updated from v2.9.0 to v2.11.4

## v0.5.0

### Added

- **One-line installer**: `curl -fsSL .../install.sh | sh` for quick setup on Linux and macOS, with OS/arch auto-detection and `VERSION`/`INSTALL_DIR` overrides

### Changed

- **Dependency updates**: cobra v1.8.1 → v1.10.2, pflag v1.0.5 → v1.0.10, go-md2man v2.0.4 → v2.0.7, x/sys v0.41.0 → v0.42.0, x/term v0.40.0 → v0.41.0
- **Go version**: bumped minimum from 1.24 to 1.25; CI images updated accordingly

## v0.4.1

### Fixed

- **RawGet auth header**: raw HTTP endpoints (billing, domains, dedicated, hosting, kubernetes) were sending `Bearer` instead of `Token` authorization, causing 403 errors despite valid credentials
- **Decimal field deserialization**: API fields like `balance`, `price`, and `amount` now accept both quoted strings and bare numbers, preventing unmarshalling errors

## v0.4.0

### Improved

- **Graceful cancellation**: Ctrl+C (SIGINT/SIGTERM) now cleanly cancels in-flight API requests instead of leaving orphaned connections
- **Browser login reliability**: polling now fails fast after 3 consecutive errors and reports HTTP error details instead of looping silently for 10 minutes
- **Error reporting**: configuration file corruption and update-check write failures are now surfaced instead of silently ignored

### Changed

- **CI pipeline**: test runs now produce JUnit XML reports, enforce a minimum coverage threshold, and run E2E tests automatically when `PIDGINHOST_API_TOKEN` is configured

## v0.3.1

### Fixed

- **Self-update asset matching**: updater now looks for the published hyphenated release binaries, so `phctl update` can download real artifacts again
- **Automatic update checks**: notices now run in the background and only consume the 24-hour throttle window after a successful release lookup
- **Windows self-update handling**: `phctl update` now returns a clear unsupported error instead of attempting an in-place executable replacement
- **Flag-only command validation**: all CLI commands now reject unexpected positional arguments consistently instead of silently accepting extras
- **Support ticket routing**: restored `phctl ticket ...` as a working root command without breaking `phctl support ticket ...`
- **Output format validation**: invalid `--output` values now fail fast instead of silently falling back to table output
- **Browser login timeout**: CLI session creation now uses a bounded HTTP client, preventing indefinite hangs before polling starts

## v0.3.0

### Added

- `phctl update` command for self-updating the CLI
- Automatic update availability checks after command execution
- Release asset downloads from the latest published `phctl` release

## v0.2.4

### Fixed

- **RawGet auth guard**: added missing empty-token check (matching SDK client behavior)
- **Grouping commands accept stray args**: added `Args: cobra.NoArgs` to all 37 parent commands so typos like `phctl compute foo` return an error instead of silently showing help
- **FreeDNS E2E test**: corrected command path from `freedns list` to `freedns domain list`

## v0.2.3

### Fixed

- CI: changelog extraction for release notes (awk range pattern fix)

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
