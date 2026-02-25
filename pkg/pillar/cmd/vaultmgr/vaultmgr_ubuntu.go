//go:build ubuntu

// Copyright (c) 2024 Zededa, Inc.
// SPDX-License-Identifier: Apache-2.0

// Ubuntu stub for vaultmgr. Publishes healthy VaultStatus so that
// other agents waiting on WaitForVault() can proceed.

package vaultmgr

import (
	"time"

	"github.com/lf-edge/eve-api/go/info"
	"github.com/lf-edge/eve/pkg/pillar/agentbase"
	"github.com/lf-edge/eve/pkg/pillar/base"
	"github.com/lf-edge/eve/pkg/pillar/pubsub"
	"github.com/lf-edge/eve/pkg/pillar/types"
	"github.com/sirupsen/logrus"
)

const agentName = "vaultmgr"

// Run is the ubuntu stub entry point for vaultmgr.
func Run(ps *pubsub.PubSub, loggerArg *logrus.Logger, logArg *base.LogObject, arguments []string, baseDir string) int {
	logger := loggerArg
	log := logArg

	// When called with arguments (e.g., "setupDeprecatedVaults"), just succeed.
	if len(arguments) > 0 {
		log.Noticef("%s(ubuntu stub): called with args %v, returning success", agentName, arguments)
		return 0
	}

	state := &agentbase.AgentBase{}
	agentbase.Init(state, logger, log, agentName,
		agentbase.WithPidFile(),
		agentbase.WithBaseDir(baseDir),
		agentbase.WithArguments(arguments))

	pubVaultStatus, err := ps.NewPublication(pubsub.PublicationOptions{
		AgentName: agentName,
		TopicType: types.VaultStatus{},
	})
	if err != nil {
		log.Fatal(err)
	}
	pubVaultStatus.ClearRestarted()

	// Publish healthy vault status so WaitForVault() unblocks.
	if err := pubVaultStatus.Publish(types.DefaultVaultName, types.VaultStatus{
		Name:               types.DefaultVaultName,
		Status:             info.DataSecAtRestStatus_DATASEC_AT_REST_DISABLED,
		ConversionComplete: true,
	}); err != nil {
		log.Fatal(err)
	}

	log.Noticef("%s(ubuntu stub): published healthy VaultStatus, entering idle loop", agentName)

	stillRunning := time.NewTicker(25 * time.Second)
	for {
		<-stillRunning.C
		ps.StillRunning(agentName, 40*time.Second, 3*time.Minute)
	}
}
