//go:build ubuntu

// Copyright (c) 2024 Zededa, Inc.
// SPDX-License-Identifier: Apache-2.0

// No-op stubs for the zfs package when building for Ubuntu.
// These allow packages like volumemgr, diskmetrics, and volumehandlers
// to compile without the go-libzfs CGO dependency.

package zfs

import (
	"fmt"

	"github.com/lf-edge/eve/pkg/pillar/base"
	"github.com/lf-edge/eve/pkg/pillar/types"
)

// VolBlockSizeBytes is the default dataset block size in bytes
const VolBlockSizeBytes = uint64(16 * 1024)

var errNotSupported = fmt.Errorf("zfs not supported on ubuntu")

// CreateDatasets stub
func CreateDatasets(log *base.LogObject, datasetName string) error {
	return errNotSupported
}

// CreateDataset stub
func CreateDataset(datasetName string) error {
	return errNotSupported
}

// CreateSnapshot stub
func CreateSnapshot(datasetName string) (string, error) {
	return "", errNotSupported
}

// RollbackToSnapshot stub
func RollbackToSnapshot(datasetName, snapshotName string) error {
	return errNotSupported
}

// IsDatasetTypeZvol stub
func IsDatasetTypeZvol(datasetName string) (bool, error) {
	return false, errNotSupported
}

// CreateVaultDataset stub
func CreateVaultDataset(datasetName, zfsKeyFile string) error {
	return errNotSupported
}

// GetDatasetAvailableBytes stub
func GetDatasetAvailableBytes(datasetName string) (uint64, error) {
	return 0, errNotSupported
}

// GetZvolPath stub
func GetZvolPath(datasetName string) string {
	return types.ZVolDevicePrefix + "/" + datasetName
}

// CreateVaultVolumeDataset stub
func CreateVaultVolumeDataset(log *base.LogObject, datasetName string, zfsKeyFile string, encrypted bool, sizeBytes uint64, compressionType string, blockSizeBytes uint64) error {
	return errNotSupported
}

// MountDataset stub
func MountDataset(datasetName string) error {
	return errNotSupported
}

// UnmountDataset stub
func UnmountDataset(datasetName string) error {
	return errNotSupported
}

// DestroyDataset stub
func DestroyDataset(datasetName string) error {
	return errNotSupported
}

// DatasetExist stub - always returns false on ubuntu
func DatasetExist(log *base.LogObject, datasetPath string) bool {
	return false
}

// SetReserved stub
func SetReserved(datasetName string, percentage uint64) error {
	return errNotSupported
}

// CreateVolumeDataset stub
func CreateVolumeDataset(log *base.LogObject, datasetName string, sizeBytes uint64, compression string, blockSizeBytes uint64) error {
	return errNotSupported
}

// GetVolumesFromDataset stub
func GetVolumesFromDataset(datasetName string) ([]string, error) {
	return nil, errNotSupported
}

// GetDatasetByDevice stub
func GetDatasetByDevice(device string) string {
	return ""
}

// GetZVolDeviceByDataset stub
func GetZVolDeviceByDataset(dataset string) string {
	return ""
}

// GetDatasetKeyStatus stub
func GetDatasetKeyStatus(datasetName string) (string, error) {
	return "", errNotSupported
}

// GetZFSVolumeInfo stub
func GetZFSVolumeInfo(device string) (*types.ImgInfo, error) {
	return nil, errNotSupported
}

// RemoveVDev stub
func RemoveVDev(log *base.LogObject, pool, vdev string) (string, error) {
	return "", errNotSupported
}

// AttachVDev stub
func AttachVDev(log *base.LogObject, pool, vdev, newVdev string) (string, error) {
	return "", errNotSupported
}

// AddVDev stub
func AddVDev(log *base.LogObject, pool, vdev string) (string, error) {
	return "", errNotSupported
}

// ReplaceVDev stub
func ReplaceVDev(log *base.LogObject, pool, oldVdev, newVdev string) (string, error) {
	return "", errNotSupported
}

// GetZfsVersion stub
func GetZfsVersion() (string, error) {
	return "", errNotSupported
}

// GetZfsCompressratio stub
func GetZfsCompressratio(zpoolName string) (float64, error) {
	return 0, errNotSupported
}

// GetZfsCountVolume stub
func GetZfsCountVolume(datasetName string) (uint32, error) {
	return 0, errNotSupported
}

// GetRaidTypeFromStr stub
func GetRaidTypeFromStr(raidName string) types.StorageRaidType {
	return types.StorageRaidTypeNoRAID
}

// GetZfsDeviceStatusFromStr stub
func GetZfsDeviceStatusFromStr(statusStr string) types.StorageStatus {
	return types.StorageStatusUnspecified
}

// GetDatasetUsageStat stub
func GetDatasetUsageStat(datasetName string) (*types.UsageStat, error) {
	return nil, errNotSupported
}

// GetZVolSectorSize stub
func GetZVolSectorSize(zVolName string) (uint64, error) {
	return 0, errNotSupported
}

// GetZvolMetrics stub
func GetZvolMetrics(status types.VolumeStatus, poolName string) (*types.StorageZVolMetrics, error) {
	return nil, errNotSupported
}

// GetZpoolStatusMsgStr stub
func GetZpoolStatusMsgStr(status types.PoolStatus) string {
	return "Unspecified"
}

// GetVDevAuxMsgStr stub
func GetVDevAuxMsgStr(state types.VDevAux) string {
	return "Unspecified"
}

// GetAllZFSVolumeInfo stub
func GetAllZFSVolumeInfo() ([]types.ImgInfo, error) {
	return nil, errNotSupported
}
