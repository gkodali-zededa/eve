# Running EVE Pillar on Ubuntu

This document describes how to build and run the EVE pillar middleware as a Docker container on a stock Ubuntu host. The container manages containerd-based application workloads while remaining API-compatible with the EVE cloud controller.

## Prerequisites

- **Ubuntu 22.04+** host (amd64 or arm64)
- **Docker** with Compose v2 (`docker compose`)
- **containerd** running on the host (`systemctl status containerd`)

## Quick Start

```bash
cd pkg/pillar

# 1. Prepare device credentials (see "Device Provisioning" below)
mkdir -p config
# ... populate config/ with your device certs ...

# 2. Build the Docker image
make build-docker-ubuntu

# 3. Start
make run-ubuntu

# 4. Check logs
docker logs -f eve-pillar

# 5. Stop
make stop-ubuntu
```

## Build Targets

All targets run from `pkg/pillar/`:

| Target | Description |
|--------|-------------|
| `make build-docker-ubuntu` | Build the Ubuntu Docker image (`lfedge/eve-pillar-ubuntu:local-<arch>`) |
| `make build-ubuntu` | Build just the `zedbox-ubuntu` binary locally (requires Go 1.24+ and CGO) |
| `make run-ubuntu` | Start the container via `docker-compose.ubuntu.yml` |
| `make stop-ubuntu` | Stop and remove the container |

Cross-architecture build:
```bash
make build-docker-ubuntu ZARCH=arm64
```

## Device Provisioning

Before first start, the `pkg/pillar/config/` directory must contain device identity files. This directory is mounted read-only into the container at `/config`.

### Required files

| File | Description |
|------|-------------|
| `server` | Controller hostname and port (e.g., `zedcloud.alpha.zededa.net:443`) |
| `root-certificate.pem` | Root CA certificate for validating the controller |
| `onboard.cert.pem` | Device onboarding certificate |
| `onboard.key.pem` | Device onboarding private key |

### Optional files (created during onboarding or pre-provisioned)

| File | Description |
|------|-------------|
| `device.cert.pem` | Device identity certificate |
| `device.key.pem` | Device private key (file-based since there is no TPM) |
| `v2tlsbaseroot-certificates.pem` | Additional root CAs for V2 API |

### Example setup

```bash
mkdir -p pkg/pillar/config
echo "zedcloud.alpha.zededa.net:443" > pkg/pillar/config/server
cp /path/to/root-certificate.pem pkg/pillar/config/
cp /path/to/onboard.cert.pem pkg/pillar/config/
cp /path/to/onboard.key.pem pkg/pillar/config/
```

## How It Works

### Build tag: `ubuntu`

The Docker image is built with `go build -tags ubuntu`, which:

- **Stubs out** 6 EVE-OS-specific agents: `baseosmgr`, `ledmanager`, `tpmmgr`, `vaultmgr`, `zfsmanager`, `usbmanager`. Each stub publishes healthy status to the controller but performs no real work.
- **Eliminates the ZFS CGO dependency** (`go-libzfs`) entirely. The `zfs` package is replaced with no-op stubs that return errors, and the runtime persist type is set to `ext4`.
- **Keeps all workload agents functional**: `zedagent`, `zedmanager`, `domainmgr`, `zedrouter`, `volumemgr`, `downloader`, `verifier`, `nim`, `nodeagent`, and others run unchanged.

The existing EVE build (without the `ubuntu` tag) is completely unaffected.

### Container configuration

The container runs with:

- **`--privileged`** and **`--network=host`** — required for `zedrouter` to create bridges, iptables rules, and DHCP/DNS services for application workloads.
- **tmpfs at `/run`** — ephemeral storage for pubsub Unix domain sockets and agent state.
- **Host containerd socket** mounted at `/run/containerd/containerd.sock` — pillar manages application containers in the `eve-user-apps` namespace on the host's containerd.
- **Docker volume `eve-persist`** at `/persist` — persistent state (certs, downloaded images, agent status) survives container restarts.
- **Read-only bind mounts** for `/sys` and `/proc` at `/hostfs/sys` and `/hostfs/proc` for cgroup and kernel parameter access.

### Startup sequence

The entrypoint script (`scripts/ubuntu-start.sh`) performs a simplified version of the EVE boot sequence:

1. Creates all required directories under `/run` and `/persist`
2. Sets persist type to `ext4` (no ZFS)
3. Starts the `zedbox` monolith process
4. Starts all agents in order (vaultmgr early, since other agents block on `WaitForVault()`)
5. Waits for each agent to signal readiness via touch files

### Stubbed agents

| Agent | What the stub does |
|-------|--------------------|
| `vaultmgr` | Publishes `VaultStatus{ConversionComplete: true, Status: DISABLED}` — unblocks all agents that call `WaitForVault()` |
| `baseosmgr` | Publishes `BaseOSMgrStatus{CurrentRetryUpdateCounter: 0}`, signals restarted |
| `tpmmgr` | Publishes empty `EdgeNodeCert{}`, no TPM operations |
| `ledmanager` | No-op idle loop (no LED hardware) |
| `zfsmanager` | No-op idle loop (no ZFS pools) |
| `usbmanager` | No-op idle loop (no USB passthrough) |

## Verification

### Check all agents started

```bash
docker logs eve-pillar 2>&1 | grep "All agents started"
```

### Monitor pubsub IPC

```bash
docker exec eve-pillar /opt/zededa/bin/ipcmonitor
```

### Check controller connectivity

```bash
docker exec eve-pillar cat /run/global/DeviceNetworkStatus/global.json | jq .
```

### List managed containers on host

```bash
sudo ctr -n eve-user-apps containers list
```

## Troubleshooting

### Container exits immediately

Check logs for missing config files:
```bash
docker logs eve-pillar 2>&1 | head -50
```
Most common cause: missing `config/server` file.

### Agent timeout warnings

Some agents may log `WARNING: Timed out waiting for <agent>`. This is usually transient during first boot while the device onboards with the controller.

### Containerd socket not found

Ensure containerd is running on the host:
```bash
systemctl status containerd
ls -la /run/containerd/containerd.sock
```

### Network conflicts

The `zedrouter` agent creates bridges and iptables rules on the host network. If you have conflicting bridge names or iptables rules, stop other container orchestrators (e.g., Docker's bridge network) before starting.

## File Layout

```
pkg/pillar/
  Dockerfile.ubuntu              # Multi-stage Ubuntu build
  docker-compose.ubuntu.yml      # Deployment configuration
  scripts/ubuntu-start.sh        # Container entrypoint
  config/                        # Device credentials (mount into container)
  cmd/baseosmgr/baseosmgr_ubuntu.go    # Stub agents (//go:build ubuntu)
  cmd/ledmanager/ledmanager_ubuntu.go
  cmd/tpmmgr/tpmmgr_ubuntu.go
  cmd/vaultmgr/vaultmgr_ubuntu.go
  cmd/zfsmanager/zfsmanager_ubuntu.go
  cmd/usbmanager/usbmanager_ubuntu.go
  zfs/zfs_ubuntu.go              # No-op ZFS stubs
  vault/handler_gethandler_ubuntu.go   # Vault handler without ZFS
```
