//go:build !ubuntu

// Copyright (c) 2022 Zededa, Inc.
// SPDX-License-Identifier: Apache-2.0

package vault

import (
	"github.com/lf-edge/eve/pkg/pillar/base"
	"github.com/lf-edge/eve/pkg/pillar/types"
	"github.com/lf-edge/eve/pkg/pillar/utils/persist"
)

// GetHandler returns Handler implementation for the current persist type
func GetHandler(log *base.LogObject) Handler {
	persistFsType := persist.ReadPersistType()
	switch persistFsType {
	case types.PersistZFS:
		return &ZFSHandler{log: log}
	case types.PersistExt4:
		return &Ext4Handler{log: log}
	default:
		log.Warnf("unsupported persist type: %s", persistFsType)
		return &UnsupportedHandler{log: log}
	}
}
