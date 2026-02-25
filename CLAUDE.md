# CLAUDE.md — EVE OS

EVE (Edge Virtualization Engine) is an open-source OS for managing edge compute nodes. It runs VMs and containers on bare-metal hardware, managed by a cloud controller. Part of the LF Edge project.

## Build System

All builds run inside Docker containers. You need Docker with BuildKit support.

```bash
make pkgs                     # Build all packages
make pkg/pillar               # Build just the pillar package
make rootfs                   # Build rootfs image
make live                     # Build full QCOW2 disk image
make live ZARCH=arm64         # Cross-compile for ARM64
make config                   # Build initial config bundle
make shell                    # Drop into the Go build container
```

Key variables: `HV=kvm|xen` (hypervisor), `DEV=y` (dev build), `ZARCH=amd64|arm64|riscv64`, `PLATFORM=generic|nvidia|imx8`.

## Testing

Tests run inside Docker containers:

```bash
make test                     # Run all tests (pillar, bpftrace, dnsmasq, debug, newlog)
make pillar-test              # Run only pillar Go tests
make test-profiling           # Run pillar tests with memory profiler
```

Go tests use standard `testing` package with `testify/assert` for assertions. Table-driven tests are the norm.

## Linting and Formatting

```bash
make pillar-fmt               # Format code with gofmt
make pillar-fmt-check         # Check formatting (CI)
make pillar-vet               # Run go vet
make yetus                    # Full Apache Yetus code quality checks
make mini-yetus               # Yetus on changed files only (faster)
```

Linter config: `pkg/pillar/.golangci.yml` (golangci-lint), `.revive.toml` (revive).

## Running

```bash
make run                      # Run EVE on QEMU
make run-live-gui             # Run with graphics
```

## Ubuntu Docker Build (Pillar on Ubuntu)

The pillar middleware can also run as a privileged Docker container on stock Ubuntu, using the host's containerd for workloads. This uses the `ubuntu` Go build tag to stub out EVE-OS-specific agents (ZFS, TPM, baseos, LED, USB).

```bash
cd pkg/pillar
make build-docker-ubuntu      # Build Ubuntu Docker image
make run-ubuntu               # Start via docker-compose (privileged, host network)
make stop-ubuntu              # Stop the container
```

Prerequisites: Docker, containerd on host, and a `pkg/pillar/config/` directory with device credentials. See `pkg/pillar/UBUNTU.md` for full setup instructions.

## Project Structure

```
pkg/pillar/           Core control plane (Go)
  cmd/                38+ microservice agents (zedagent, domainmgr, zedrouter, etc.)
  zedbox/             Monolith binary — all agents compiled into one (BusyBox pattern)
  pubsub/             Publish-subscribe IPC system
    socketdriver/     Unix socket-based IPC driver
  types/              Shared type definitions (~68 modules)
  agentbase/          Agent initialization framework
  hypervisor/         Hypervisor abstraction (KVM, Xen)
  containerd/         Containerd integration
  vendor/             Vendored Go dependencies
  go.mod              Go 1.24.0, module: github.com/lf-edge/eve/pkg/pillar
docs/                 Documentation
tools/                Build and utility scripts
```

## Architecture

### Zedbox Monolith

All agents compile into a single `zedbox` binary (like BusyBox). Agents are invoked via symlinks — `domainmgr` symlinked to `zedbox` causes zedbox to run the domainmgr agent. See `pkg/pillar/zedbox/zedbox.go` for the entrypoint map.

### PubSub IPC

Agents communicate via publish-subscribe, not direct RPC. Each agent publishes typed status objects and subscribes to others' publications.

- **Publications**: key-value stores scoped by agent name and topic type
- **Subscriptions**: event handlers — `CreateHandler`, `ModifyHandler`, `DeleteHandler`, `RestartHandler`
- **Transport**: Unix domain sockets (one per table), with JSON-serialized data
- **Persistence**: optional file-based (`/persist/` for persistent, `/run/` for ephemeral)

### Key Agents

| Agent | Purpose |
|-------|---------|
| `zedagent` | Cloud controller communication, config/status sync |
| `domainmgr` | VM and container lifecycle (hypervisor interface) |
| `zedrouter` | Network connectivity for app instances |
| `zedmanager` | Application instance orchestration |
| `nim` | Network interface management |
| `volumemgr` | Volume/disk lifecycle |
| `baseosmgr` | Base OS update with dual partitions |
| `nodeagent` | Device health, watchdog, reboot |
| `downloader` | Image downloads |
| `verifier` | Cryptographic verification |

## Conventions

### Agent Pattern

Agents implement `types.AgentRunner`:
```go
func Run(ps *pubsub.PubSub, logger *logrus.Logger, log *base.LogObject, arguments []string, baseDir string) int
```

They embed `agentbase.AgentBase`, set up subscriptions/publications in a context struct, and run an event loop processing pubsub messages. Business logic should be separated from pubsub wiring.

### Dependencies

Dependencies are **vendored** (`pkg/pillar/vendor/`). Build uses `GO111MODULE=on GOFLAGS=-mod=vendor`.

### Commits

- Sign off all commits: `git commit -s` (DCO required)
- Reference issues: `Closes #XXXX` or `Fixes #XXXX`
- One logical change per commit; squash before PR
- PRs need a "How to test and validate" section

### Logging

Uses `github.com/sirupsen/logrus`. Agents receive a logger and `base.LogObject` via their Run function.

### License

Apache 2.0. All files need SPDX license headers.
