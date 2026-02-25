//go:build ubuntu

// Copyright (c) 2024 Zededa, Inc.
// SPDX-License-Identifier: Apache-2.0

package vault

import (
	"github.com/lf-edge/eve/pkg/pillar/base"
	"github.com/lf-edge/eve/pkg/pillar/types"
	"github.com/lf-edge/eve/pkg/pillar/utils/persist"
)

// GetHandler returns Handler implementation for the current persist type.
// On Ubuntu, ZFS is not supported so only Ext4 and Unsupported handlers are available.
func GetHandler(log *base.LogObject) Handler {
	persistFsType := persist.ReadPersistType()
	switch persistFsType {
	case types.PersistExt4:
		return &Ext4Handler{log: log}
	default:
		log.Warnf("unsupported persist type: %s", persistFsType)
		return &UnsupportedHandler{log: log}
	}
}
