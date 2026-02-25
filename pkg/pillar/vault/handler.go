// Copyright (c) 2022 Zededa, Inc.
// SPDX-License-Identifier: Apache-2.0

package vault

import (
	"github.com/lf-edge/eve-api/go/info"
	"github.com/lf-edge/eve/pkg/pillar/types"
)

// HandlerOptions defines options for handler
type HandlerOptions struct {
	// TpmKeyOnlyMode will use only TPM key to generate vault key
	TpmKeyOnlyMode bool
}

// Handler is an interface for handling vault operations
type Handler interface {
	RemoveDefaultVault() error
	UnlockDefaultVault() error
	SetupDeprecatedVaults() error
	SetupDefaultVault() error
	GetVaultStatuses() []*types.VaultStatus
	SetHandlerOptions(HandlerOptions)
	GetOperationalInfo() (info.DataSecAtRestStatus, string)
}
