#!/bin/bash
# Copyright (c) 2024 Zededa, Inc.
# SPDX-License-Identifier: Apache-2.0

# Simplified startup script for running EVE pillar as a Docker container on Ubuntu.
# Replaces the init.sh -> device-steps.sh -> onboot.sh chain used in EVE OS.

set -e

BINDIR=/opt/zededa/bin
PERSISTDIR=/persist
CONFIGDIR=/config
ZTMPDIR=/run/global
DPCDIR=$ZTMPDIR/DevicePortConfig
WATCHDOG_PID=/run/watchdog/pid
WATCHDOG_FILE=/run/watchdog/file

echo "$(date -Ins -u) Starting EVE pillar on Ubuntu"

# ---- DNS setup ----
if [ ! -f /etc/resolv.conf ] || [ ! -s /etc/resolv.conf ]; then
    echo 'nameserver 8.8.8.8' > /etc/resolv.conf
fi

# ---- Create required directories ----
mkdir -p \
    "$ZTMPDIR" \
    "$DPCDIR" \
    "$PERSISTDIR/tmp" \
    "$PERSISTDIR/certs" \
    "$PERSISTDIR/status" \
    "$PERSISTDIR/status/zedclient/OnboardingStatus" \
    "$PERSISTDIR/agentdebug" \
    "$PERSISTDIR/log" \
    "$PERSISTDIR/checkpoint" \
    "$PERSISTDIR/newlog/collect" \
    "$PERSISTDIR/newlog/devUpload" \
    "$PERSISTDIR/newlog/appUpload" \
    "$PERSISTDIR/newlog/keepSentQueue" \
    "$PERSISTDIR/vault/volumes" \
    "$PERSISTDIR/vault/downloader" \
    "$PERSISTDIR/vault/verifier" \
    "$PERSISTDIR/vault/containerd" \
    "$PERSISTDIR/clear/volumes" \
    "$PERSISTDIR/config" \
    "$PERSISTDIR/ingested" \
    "$PERSISTDIR/memory-monitor/output" \
    "$WATCHDOG_PID" \
    "$WATCHDOG_FILE"

# ---- Set persist type to ext4 (no ZFS on Ubuntu) ----
echo "ext4" > /run/eve.persist_type

# ---- Write EVE version ----
echo "ubuntu-pillar-dev" > /run/eve-release

# ---- Initialize LedBlinkCounter ----
mkdir -p "$ZTMPDIR/LedBlinkCounter"
echo '{"BlinkCounter": 1}' > "$ZTMPDIR/LedBlinkCounter/ledconfig.json"

# ---- Verify device config exists ----
if [ ! -f "$CONFIGDIR/server" ]; then
    echo "WARNING: $CONFIGDIR/server not found. Device will not be able to connect to controller."
    echo "Please mount a /config volume with: server, root-certificate.pem, onboard.cert.pem, onboard.key.pem"
fi

# ---- Helper: wait for a touch file ----
wait_for_touch() {
    local f="/run/${1}.touch"
    local waited=0
    while [ ! -f "$f" ] && [ "$waited" -lt 120 ]; do
        sleep 2
        waited=$((waited + 2))
    done
    if [ ! -f "$f" ]; then
        echo "WARNING: Timed out waiting for $1"
    fi
}

# ---- Start zedbox (main process) ----
echo "$(date -Ins -u) Starting zedbox"
"$BINDIR/zedbox" &

wait_for_touch zedbox

# ---- Start all agents ----
# Order matters: vaultmgr must start early since other agents wait for vault status.
AGENTS="ledmanager tpmmgr vaultmgr evalmgr diag nim nodeagent \
domainmgr loguploader zedagent zedmanager zedrouter downloader verifier \
baseosmgr wstunnelclient volumemgr watcher zfsmanager usbmanager \
monitor collectinfo vcomlink"

for AGENT in $AGENTS; do
    echo "$(date -Ins -u) Starting $AGENT"
    "$BINDIR/$AGENT" &
done

# ---- Wait for critical agents ----
for AGENT in $AGENTS; do
    if [ "$AGENT" = "diag" ]; then continue; fi
    wait_for_touch "$AGENT"
    touch "$WATCHDOG_FILE/$AGENT.touch" 2>/dev/null || true
done

echo "$(date -Ins -u) All agents started successfully"

# Keep container alive
wait
