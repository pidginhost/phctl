# AGENTS.md

Conventions for working in this repository. Where this file and a general habit
disagree, this file wins.

## What this is

`phctl` is the PidginHost CLI. It is a thin layer over `github.com/pidginhost/sdk-go`,
which is generated from the PidginHost OpenAPI schema. Almost every command is a
`cobra.Command` whose `RunE` closure calls one SDK operation and renders the result.

## Build and test

```bash
go build ./... && go vet ./... && go test $(go list ./... | grep -v /e2e)
golangci-lint run ./...
```

`./e2e` is excluded on purpose. Those tests talk to the **live API with a real
account**, and `TestE2E_ComputeServerLifecycle` provisions a billable VM. They
skip without `PIDGINHOST_API_TOKEN`, and the lifecycle test additionally needs
`PHCTL_E2E_LIFECYCLE`. Run them deliberately, never as part of a general
`go test ./...`.

`scripts/pre-commit` runs fmt, vet, lint, build and tests. Install it with
`scripts/install-hooks.sh`.

## Test-first

Write the failing test, watch it fail for the reason you expect, then implement.
A bug gets a test that reproduces it before it gets a fix.

Prove a new guard is non-vacuous. Both contract tests below carry a fixture test
that feeds them known-bad input, because an AST analyzer that silently stops
matching anything passes forever.

## Two contracts enforced by tests

These exist because both problems recurred across many files, and a reviewer
cannot hold ~120 call sites in their head.

**`cmd/output_contract_test.go` — the `-o/--output` contract.** A command that
reports a resource returned by the API renders it through `internal/output`
(`output.Print` / `output.Result`), never `cmd.Print*`. Text written with
`cmd.Print*` is prose, so under `-o json` the caller gets a sentence where it
asked for a document. Acknowledgements ("Server 42 deleted."), prompts and
progress narration are exempt — there is no payload to encode.

**`cmd/apierror_contract_test.go` — the API error contract.** An error from the
API is wrapped with `cmdutil.APIError`, or propagated unchanged so an outer call
can wrap it. `fmt.Errorf("...: %w", err)` looks like it adds context but discards
the response body, which is the only part that says what was wrong:

```
creating ticket: 400 Bad Request                                    <- fmt.Errorf
creating ticket: 400 Bad Request: department=This field is required. <- APIError
```

Errors that are not from the API (a file open, a `strconv`) take `fmt.Errorf` as
normal; the analyzer tracks rebinding and will not flag them.

One sanctioned exception: `cmdutil.APIErrorRedacted` wraps the same way but
never lets the body through. Use it *only* where the body is itself a secret —
today that is the two bucket credential routes, which answer `200` with an
access key and secret, so a body that fails to decode would put live
credentials into an error string, a log, and any bug report it is pasted into.
Everywhere else the body is the whole point, so reach for `APIError`.

## Command conventions

- Resolve global flags through `cmdutil`: `cmdutil.OutputFormat(cmd)`,
  `cmdutil.Force(cmd)`. Both look flags up in a way that works before Cobra has
  merged the root's persistent flags, which is how every `RunE`-level test runs.
- Check the operation actually happened. Several endpoints answer `200` with
  `{"attached": false}` or `{"upgrading": false}`; reporting success there tells
  an operator something is true when it is not. Return an error instead.
- Destructive, billable or restarting operations confirm first:
  `if !cmdutil.Force(cmd) && !confirm.Action(...) { return nil }`.
- Paginated list endpoints go through `cmdutil.FetchAll`.
- Endpoints whose response carries a decimal still go through `internal/client`'s
  `Raw*` types and `RawFetchAll`. That is now legacy, not a rule for new code —
  see below.

## The decimal workaround (now removable)

`internal/client/rawtypes.go` exists because `sdk-go` up to and including
v0.11.0 mapped `type: string, format: decimal` to `float64`, which cannot decode
the `"12.50"` the API actually sends, so any endpoint returning a price, balance
or total had to bypass the SDK model.

**That is fixed.** phctl is on `sdk-go` v0.12.2, where those fields generate as
`string`. New commands should use the generated models even when the response
carries a decimal — render the value with `%s` and never a float verb; `%.2f`
against a string silently truncates it to two characters. The `Raw*` types and
their remaining call sites are now dead weight to be deleted, not extended: do
not add another one.

## SDK version bumps

`sdk-go` is generated from **production**'s schema, so bumping it is gated on a
production `phclient` deploy, not just a merge — see the project memory note
`sdk-release-chain`. Never hand-edit generated SDK code; fix the schema or the
generator config in the `sdk` repo and regenerate.

## Commits

Conventional Commits, subject **50 characters or fewer**, body only when the
"why" is not obvious from the diff. No `Co-Authored-By` or any other attribution
trailer.

## Releases

1. Rename `## Unreleased` in `CHANGELOG.md` to `## vX.Y.Z`.
2. Commit as `chore(release): vX.Y.Z`, then tag `vX.Y.Z` and push both.

CI injects the version with `-ldflags "-X main.version=${CI_COMMIT_TAG}"`, so
there is no version constant to edit. The release jobs extract the notes with

```
awk -v ver="vX.Y.Z" '/^## / { if (found) exit; if ($2 == ver) { found=1; next } } found { print }' CHANGELOG.md
```

so the heading must be exactly `## vX.Y.Z` — check the extraction before tagging.

## CI notes

- Coverage is measured by `scripts/coverage-total.sh`, from the profile.
  `go tool cover -func` reports named functions only and cannot see a `RunE`
  closure, which is where nearly all of this CLI lives. `THRESHOLD` is a
  ratchet: raise it as coverage improves, never lower it.
- Any job touching a shared external resource carries a `resource_group`. A tag
  push and a branch push on the same commit produce two independent pipelines,
  so `e2e` (one live account, one VM quota) and `deploy:github` (a force-pushed
  mirror) would otherwise race.
