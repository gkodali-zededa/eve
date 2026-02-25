//go:build ubuntu

// Copyright (c) 2024 Zededa, Inc.
// SPDX-License-Identifier: Apache-2.0

// Ubuntu stub for tpmmgr. Publishes empty EdgeNodeCert since
// there is no TPM hardware on a generic Ubuntu host.

package tpmmgr

import (
	"time"

	"github.com/lf-edge/eve/pkg/pillar/agentbase"
	"github.com/lf-edge/eve/pkg/pillar/base"
	"github.com/lf-edge/eve/pkg/pillar/pubsub"
	"github.com/lf-edge/eve/pkg/pillar/types"
	"github.com/sirupsen/logrus"
)

const agentName = "tpmmgr"

// Run is the ubuntu stub entry point for tpmmgr.
func Run(ps *pubsub.PubSub, loggerArg *logrus.Logger, logArg *base.LogObject, arguments []string, baseDir string) int {
	logger := loggerArg
	log := logArg

	// When called with arguments (e.g., inline commands), just succeed.
	if len(arguments) > 0 {
		log.Noticef("%s(ubuntu stub): called with args %v, returning success", agentName, arguments)
		return 0
	}

	state := &agentbase.AgentBase{}
	agentbase.Init(state, logger, log, agentName,
		agentbase.WithPidFile(),
		agentbase.WithBaseDir(baseDir),
		agentbase.WithArguments(arguments))

	pubEdgeNodeCert, err := ps.NewPublication(pubsub.PublicationOptions{
		AgentName: agentName,
		TopicType: types.EdgeNodeCert{},
	})
	if err != nil {
		log.Fatal(err)
	}
	pubEdgeNodeCert.ClearRestarted()

	log.Noticef("%s(ubuntu stub): started, no TPM available, entering idle loop", agentName)

	stillRunning := time.NewTicker(25 * time.Second)
	for {
		<-stillRunning.C
		ps.StillRunning(agentName, 40*time.Second, 3*time.Minute)
	}
}
