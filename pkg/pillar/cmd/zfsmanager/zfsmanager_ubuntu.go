//go:build ubuntu

// Copyright (c) 2024 Zededa, Inc.
// SPDX-License-Identifier: Apache-2.0

// Ubuntu stub for zfsmanager. No ZFS pool management on Ubuntu.

package zfsmanager

import (
	"time"

	"github.com/lf-edge/eve/pkg/pillar/agentbase"
	"github.com/lf-edge/eve/pkg/pillar/base"
	"github.com/lf-edge/eve/pkg/pillar/pubsub"
	"github.com/sirupsen/logrus"
)

const agentName = "zfsmanager"

// Run is the ubuntu stub entry point for zfsmanager.
func Run(ps *pubsub.PubSub, loggerArg *logrus.Logger, logArg *base.LogObject, arguments []string, baseDir string) int {
	logger := loggerArg
	log := logArg

	state := &agentbase.AgentBase{}
	agentbase.Init(state, logger, log, agentName,
		agentbase.WithPidFile(),
		agentbase.WithBaseDir(baseDir),
		agentbase.WithArguments(arguments))

	log.Noticef("%s(ubuntu stub): no ZFS, entering idle loop", agentName)

	stillRunning := time.NewTicker(25 * time.Second)
	for {
		<-stillRunning.C
		ps.StillRunning(agentName, 40*time.Second, 3*time.Minute)
	}
}
