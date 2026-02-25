//go:build ubuntu

// Copyright (c) 2024 Zededa, Inc.
// SPDX-License-Identifier: Apache-2.0

// Ubuntu stub for baseosmgr. Publishes minimal status so zedagent
// and nodeagent can proceed without A/B partition support.

package baseosmgr

import (
	"time"

	"github.com/lf-edge/eve/pkg/pillar/agentbase"
	"github.com/lf-edge/eve/pkg/pillar/base"
	"github.com/lf-edge/eve/pkg/pillar/pubsub"
	"github.com/lf-edge/eve/pkg/pillar/types"
	"github.com/sirupsen/logrus"
)

const agentName = "baseosmgr"

// Run is the ubuntu stub entry point for baseosmgr.
func Run(ps *pubsub.PubSub, loggerArg *logrus.Logger, logArg *base.LogObject, arguments []string, baseDir string) int {
	logger := loggerArg
	log := logArg

	state := &agentbase.AgentBase{}
	agentbase.Init(state, logger, log, agentName,
		agentbase.WithPidFile(),
		agentbase.WithBaseDir(baseDir),
		agentbase.WithArguments(arguments))

	pubBaseOSMgrStatus, err := ps.NewPublication(pubsub.PublicationOptions{
		AgentName: agentName,
		TopicType: types.BaseOSMgrStatus{},
	})
	if err != nil {
		log.Fatal(err)
	}

	if err := pubBaseOSMgrStatus.Publish("global", types.BaseOSMgrStatus{
		CurrentRetryUpdateCounter: 0,
	}); err != nil {
		log.Fatal(err)
	}
	pubBaseOSMgrStatus.SignalRestarted()

	log.Noticef("%s(ubuntu stub): published BaseOSMgrStatus, entering idle loop", agentName)

	stillRunning := time.NewTicker(25 * time.Second)
	for {
		<-stillRunning.C
		ps.StillRunning(agentName, 40*time.Second, 3*time.Minute)
	}
}
