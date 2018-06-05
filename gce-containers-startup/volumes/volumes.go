// Copyright 2017 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package volumes

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/GoogleCloudPlatform/konlet/gce-containers-startup/metadata"
	api "github.com/GoogleCloudPlatform/konlet/gce-containers-startup/types"
)

const (
	ext4FsType string = "ext4"
)

var (
	mountedVolumesPathPrefixFlag = flag.String("mounted-volumes-path-prefix", "/mnt/disks/gce-containers-mounts", "Path prefix under which mount volumes.")
	hostProcPathFlag             = flag.String("host-proc-path", "/host_proc", "Use nsenter to enter host's mount namespace specified under this path. If left empty, no namespace switch is performed (implying running outside of container.")
)

// Environment struct for dependency injection.
type Env struct {
	OsCommandRunner  OsCommandRunner
	MetadataProvider metadata.Provider
}

type OsCommandRunner interface {
	Run(...string) (string, error)
	MkdirAll(path string, perm os.FileMode) error
	Stat(name string) (os.FileInfo, error)
}

type VolumeHostPathAndMode struct {
	// nil hostPath means no backing directory, implying tmpfs mount.
	hostPath string
	readOnly bool
}

type HostPathBindConfiguration struct {
	HostPath      string
	ContainerPath string
	ReadOnly      bool
}

// PrepareVolumesAndGetBindings does 3 things:
// - Verifies if the container specification passed to it is correct in terms of
//   volumes it references.
// - Creates/mounts/formats all the necessary volumes.
// - Outputs the binding map, which will be used by the container runtime to
//   bind the volumes containers.
//
// The function takes a container specification struct. It operates in context
// of its environment (the receiver), which consist of two parts:
// - OsCommandRunner: it executes the commands issued during execution of the
//   function.
// - MetadataProvider: it is the source of additional information coming from
//   the metadata, used in processing persistent disks.
//
// The function returns a map, its keys are container names (currently this
// should be a single name) and values are slices of hostPath binds. These
// corresponds to files and directories that are mounted in a container.
// Currently all supported types of volumes (EmptyDir, HostPath and
// PersistentDisk) are ultimately handled using a Docker's hostPath bind.
//
// The caller should not expect the function to be idempotent. Errors are to be
// considered non-retryable.
func (env Env) PrepareVolumesAndGetBindings(spec api.ContainerSpecStruct) (map[string][]HostPathBindConfiguration, error) {
	// First, build maps that will allow to verify logical consistency:
	//  - All volumes must be referenced at least once.
	//  - All volume mounts must refer an existing volume.
	volumesReferencesCount := map[string]int{}
	volumeMountWantsReadWriteMap := map[string]bool{}
	for _, apiVolume := range spec.Volumes {
		volumesReferencesCount[apiVolume.Name] = 0
		volumeMountWantsReadWriteMap[apiVolume.Name] = false
	}

	for containerIndex, container := range spec.Containers {
		log.Printf("Found %d volume mounts in container %s declaration.", len(container.VolumeMounts), container.Name)
		for _, volumeMount := range container.VolumeMounts {
			if _, present := volumesReferencesCount[volumeMount.Name]; !present {
				return nil, fmt.Errorf("Invalid container declaration: Volume %s referenced in container %s (index: %d) not found in volume definitions.", volumeMount.Name, container.Name, containerIndex)
			} else {
				volumesReferencesCount[volumeMount.Name] += 1
				volumeMountWantsReadWriteMap[volumeMount.Name] = volumeMountWantsReadWriteMap[volumeMount.Name] || !volumeMount.ReadOnly
			}
		}
	}
	for volumeName, referenceCount := range volumesReferencesCount {
		if referenceCount == 0 {
			return nil, fmt.Errorf("Invalid container declaration: Volume %s not referenced by any container.", volumeName)
		}
	}

	volumeNameToHostPathMap, volumeNameMapBuildingError := env.buildVolumeNameToHostPathMap(spec.Volumes, volumeMountWantsReadWriteMap)
	if volumeNameMapBuildingError != nil {
		return nil, volumeNameMapBuildingError
	}

	containerBindingConfigurationMap := map[string][]HostPathBindConfiguration{}
	for _, container := range spec.Containers {
		var hostPathBinds []HostPathBindConfiguration
		for _, volumeMount := range container.VolumeMounts {
			// It has already been checked that the volume is present.
			volumeHostPathAndMode, _ := volumeNameToHostPathMap[volumeMount.Name]

			if volumeHostPathAndMode.readOnly && !volumeMount.ReadOnly {
				return nil, fmt.Errorf("Container %s: volumeMount %s specifies read-write access, but underlying volume is read-only.", container.Name, volumeMount.Name)
			}
			hostPathBinds = append(hostPathBinds, HostPathBindConfiguration{HostPath: volumeHostPathAndMode.hostPath, ContainerPath: volumeMount.MountPath, ReadOnly: volumeMount.ReadOnly})
		}
		containerBindingConfigurationMap[container.Name] = hostPathBinds
	}

	return containerBindingConfigurationMap, nil
}

func (env Env) buildVolumeNameToHostPathMap(apiVolumes []api.Volume, volumeMountWantsReadWriteMap map[string]bool) (map[string]VolumeHostPathAndMode, error) {
	// For each volume, use the proper handler function to build the volume name -> hostpath+mode map.
	volumeNameToHostPathMap := map[string]VolumeHostPathAndMode{}

	diskMetadataReadOnlyMap, err := env.buildDiskMetadataReadOnlyMap()
	if err != nil {
		return nil, fmt.Errorf("Failed to build disk read only map from metadata: %s", err)
	}

	for _, apiVolume := range apiVolumes {
		volumeMountWantsReadWrite, found := volumeMountWantsReadWriteMap[apiVolume.Name]
		if !found {
			return nil, fmt.Errorf("apiVolume %s not found in the volumeMount RW map. This should not happen.", apiVolume.Name)
		}
		// Enforce exactly one volume definition.
		definitions := 0
		var volumeHostPathAndMode VolumeHostPathAndMode
		var processError error
		if apiVolume.HostPath != nil {
			definitions++
			volumeHostPathAndMode, processError = env.processHostPathVolume(apiVolume.HostPath)
		}
		if apiVolume.EmptyDir != nil {
			definitions++
			volumeHostPathAndMode, processError = env.processEmptyDirVolume(apiVolume.EmptyDir, apiVolume.Name)
		}
		if apiVolume.GcePersistentDisk != nil {
			definitions++
			volumeHostPathAndMode, processError = env.processGcePersistentDiskVolume(apiVolume.GcePersistentDisk, volumeMountWantsReadWrite, diskMetadataReadOnlyMap)
		}
		if definitions != 1 {
			return nil, fmt.Errorf("Invalid container declaration: Exactly one volume specification required for volume %s, %d found.", apiVolume.Name, definitions)
		}

		if processError != nil {
			return nil, fmt.Errorf("Volume %s: %s", apiVolume.Name, processError)
		} else {
			volumeNameToHostPathMap[apiVolume.Name] = volumeHostPathAndMode
		}
	}
	return volumeNameToHostPathMap, nil
}

func (env Env) processEmptyDirVolume(volume *api.EmptyDirVolume, volumeName string) (VolumeHostPathAndMode, error) {
	if volume.Medium != "Memory" {
		return VolumeHostPathAndMode{}, fmt.Errorf("Unsupported emptyDir volume medium: %s", volume.Medium)
	}
	return env.processMemoryBackedEmptyDirVolume(volume, volumeName)
}

func (env Env) processMemoryBackedEmptyDirVolume(volume *api.EmptyDirVolume, volumeName string) (VolumeHostPathAndMode, error) {
	tmpfsMountPoint, err := env.createNewMountPath("tmpfs", volumeName)
	if err != nil {
		return VolumeHostPathAndMode{}, err
	}
	if err := env.mountDevice("tmpfs", tmpfsMountPoint, "tmpfs", false); err != nil {
		return VolumeHostPathAndMode{}, err
	}
	return VolumeHostPathAndMode{hostPath: tmpfsMountPoint, readOnly: false}, nil
}

func (env Env) processHostPathVolume(volume *api.HostPathVolume) (VolumeHostPathAndMode, error) {
	// No checks are done on this level. It is expected that underlying docker
	// will report errors (if any), at the same time it will take care of
	// creating missing directores etc. Note that it might still fail due to
	// large parts of the COS system being read-only.
	return VolumeHostPathAndMode{hostPath: volume.Path, readOnly: false}, nil
}

func (env Env) processGcePersistentDiskVolume(volume *api.GcePersistentDiskVolume, volumeMountWantsReadWrite bool, diskMetadataReadOnlyMap map[string]bool) (VolumeHostPathAndMode, error) {
	if volume.FsType != "" && volume.FsType != ext4FsType {
		return VolumeHostPathAndMode{}, fmt.Errorf("Unsupported filesystem type: %s", volume.FsType)
	}
	chosenFsType := ext4FsType
	if volume.PdName == "" {
		return VolumeHostPathAndMode{}, fmt.Errorf("Empty GCE Persistent Disk name!")
	}
	attachedReadOnly, diskMetadataFound := diskMetadataReadOnlyMap[volume.PdName]
	if !diskMetadataFound {
		return VolumeHostPathAndMode{}, fmt.Errorf("Could not determine if the GCE Persistent Disk %s is attached read-only or read-write.", volume.PdName)
	}
	if attachedReadOnly && volumeMountWantsReadWrite {
		return VolumeHostPathAndMode{}, fmt.Errorf("Volume mount requires read-write access, but the GCE Persistent Disk %s is attached read-only.", volume.PdName)
	}
	devicePath, err := resolveGcePersistentDiskDevicePath(volume.PdName)
	if err != nil {
		return VolumeHostPathAndMode{}, fmt.Errorf("Could not resolve GCE Persistent Disk device path: %s", err)
	}
	if volume.Partition > 0 {
		devicePath = fmt.Sprintf("%s-part%d", devicePath, volume.Partition)
	}

	if err := env.checkDeviceReadable(devicePath); err != nil {
		return VolumeHostPathAndMode{}, err
	}
	if err := env.checkDeviceNotMounted(devicePath); err != nil {
		return VolumeHostPathAndMode{}, err
	}
	if !attachedReadOnly {
		if err := env.checkFilesystemAndFormatIfNeeded(devicePath, chosenFsType); err != nil {
			return VolumeHostPathAndMode{}, err
		}
	}

	mountReadOnly := attachedReadOnly || !volumeMountWantsReadWrite
	deviceMountPoint, err := env.createNewMountPath("gce-persistent-disk", volume.PdName)
	if err != nil {
		return VolumeHostPathAndMode{}, err
	}
	if err := env.mountDevice(devicePath, deviceMountPoint, chosenFsType, mountReadOnly); err != nil {
		return VolumeHostPathAndMode{}, err
	}

	// Success!
	return VolumeHostPathAndMode{deviceMountPoint, mountReadOnly}, nil
}

func (env Env) buildDiskMetadataReadOnlyMap() (map[string]bool, error) {
	diskMetadataReadOnlyMap := map[string]bool{}

	diskMetadataJson, err := env.MetadataProvider.RetrieveDisksMetadataAsJson()
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve disk metadata: %s", err)
	}

	var parsedMetadata []struct {
		// Note: there are other fields in the list, but they're irrelevant for our purpose.
		DeviceName string
		Mode       string
	}
	err = json.Unmarshal(diskMetadataJson, &parsedMetadata)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal disk metadata JSON: %s", err)
	}

	for _, entry := range parsedMetadata {
		if entry.DeviceName == "" {
			return nil, fmt.Errorf("Received empty device name in the metadata: %+v", parsedMetadata)
		}
		switch entry.Mode {
		case "READ_WRITE":
			diskMetadataReadOnlyMap[entry.DeviceName] = false
		case "READ_ONLY":
			diskMetadataReadOnlyMap[entry.DeviceName] = true
		default:
			return nil, fmt.Errorf("Received unknown device mode from metadata for device %s: %s", entry.DeviceName, entry.Mode)
		}
	}
	return diskMetadataReadOnlyMap, nil
}

func resolveGcePersistentDiskDevicePath(pdName string) (string, error) {
	// Currently, only static mapping is supported, as metadata about PD name is not available.
	return fmt.Sprintf("/dev/disk/by-id/google-%s", pdName), nil
}

// Generate a name for the new volume mount, based on the volume family (type)
// and volume name.  Create the directory if necessary, return a path to a
// valid directory to mount the volume in, error otherwise.
func (env Env) createNewMountPath(volumeFamily string, volumeName string) (string, error) {
	path := fmt.Sprintf("%s/%ss/%s", *mountedVolumesPathPrefixFlag, volumeFamily, volumeName)
	log.Printf("Creating directory %s as a mount point for volume %s.", path, volumeName)
	if err := env.OsCommandRunner.MkdirAll(path, 0755); err != nil {
		return "", fmt.Errorf("Failed to create directory %s: %s", path, err)
	} else {
		return path, nil
	}
}

func wrapToEnterHostMountNamespace(origCommandline ...string) []string {
	if *hostProcPathFlag == "" {
		return origCommandline
	}
	// Change the mount namespace to the host one. Note that we're
	// not able to access the mounted directory afterwards (without
	// yet another nsenter call).
	nsenterCommandline := []string{"nsenter", fmt.Sprintf("--mount=%s/1/ns/mnt", *hostProcPathFlag), "--"}
	return append(nsenterCommandline, origCommandline...)
}

// Attempt to mount the device under a generated path. Assumes the device
// contains an clean filesystem.
func (env Env) mountDevice(devicePath string, mountPath string, fsType string, readOnly bool) error {
	log.Printf("Attempting to mount device %s at %s.", devicePath, mountPath)

	var mountOpts []string
	if readOnly {
		mountOpts = append(mountOpts, "ro")
	} else {
		mountOpts = append(mountOpts, "rw")
	}
	mountCommandline := []string{"mount"}
	if len(mountOpts) > 0 {
		mountCommandline = append(mountCommandline, "-o", strings.Join(mountOpts, ","))
	}
	mountCommandline = append(mountCommandline, "-t", fsType, devicePath, mountPath)
	_, err := env.OsCommandRunner.Run(wrapToEnterHostMountNamespace(mountCommandline...)...)
	if err != nil {
		return fmt.Errorf("Failed to mount %s at %s: %s", devicePath, mountPath, err)
	} else {
		return nil
	}
}

func (env Env) checkDeviceReadable(devicePath string) error {
	fileInfo, err := env.OsCommandRunner.Stat(devicePath)
	if err != nil {
		return fmt.Errorf("Device %s access error: %s", devicePath, err)
	}
	if fileInfo.Mode()&os.ModeDevice == 0 || fileInfo.Mode()&os.ModeCharDevice != 0 {
		return fmt.Errorf("Path %s is not a block device.", devicePath)
	}
	// TODO: More detailed access checks.
	return nil
}

func (env Env) checkFilesystemAndFormatIfNeeded(devicePath string, configuredFsType string) error {
	// Should be const, but Go can't into map consts.
	filesystemCheckerMap := map[string][]string{
		ext4FsType: []string{"fsck.ext4", "-p"},
	}
	filesystemFormatterMap := map[string][]string{
		ext4FsType: []string{"mkfs.ext4"},
	}
	filesystemChecker := filesystemCheckerMap[configuredFsType]
	filesystemFormatter := filesystemFormatterMap[configuredFsType]
	if filesystemChecker == nil || filesystemFormatter == nil {
		return fmt.Errorf("Could not find checker or formatter for filesystem %s.", configuredFsType)
	}

	const lsblkFsType string = "FSTYPE"
	foundFsType, err := env.getSinglePropertyFromDeviceWithLsblk(devicePath, lsblkFsType)
	if err != nil {
		return err
	}
	// Unfortunately, lsblk(8) doesn't provide a way to tell apart a
	// nonexistent filesystem (e.g. a fresh drive) from device read problem
	// - in both cases not reporting any errors and returning an empty
	// FSTYPE field. Therefore, care must be taken to compensate for this
	// behaviour. The strategy below is deemed safe, because:
	//
	// - If lsblk(8) lacks privileges to read the filesystem and the
	//   decision is put forward to format it, mkfs(8) will fail as well.
	// - If lsblk(8) had privileges and still didn't detect the filesystem,
	//   it's OK to format it.
	if foundFsType == "" {
		// Need to format.
		log.Printf("Formatting device %s with filesystem %s...", devicePath, configuredFsType)
		output, err := env.OsCommandRunner.Run(append(filesystemFormatter, devicePath)...)
		if err != nil {
			return fmt.Errorf("Failed to format filesystem: %s", err)
		} else {
			log.Printf("%s\n", output)
		}
	} else if foundFsType == configuredFsType {
		// Need to fsck.
		log.Printf("Running filesystem checker on device %s...", devicePath)
		output, err := env.OsCommandRunner.Run(append(filesystemChecker, devicePath)...)
		if err != nil {
			return fmt.Errorf("Filesystem check failed: %s", err)
		} else {
			log.Printf("%s\n", output)
		}
	} else {
		return fmt.Errorf("Device %s: found filesystem type %s, expected %s.", devicePath, foundFsType, configuredFsType)
	}
	return nil
}

// Return non-nil error with meaningful message when the device is already mounted.
func (env Env) checkDeviceNotMounted(devicePath string) error {
	const lsblkMountPoint string = "MOUNTPOINT"
	if mountPoint, err := env.getSinglePropertyFromDeviceWithLsblk(devicePath, lsblkMountPoint); err != nil {
		return err
	} else {
		if mountPoint == "" {
			return nil
		} else {
			return fmt.Errorf("Device %s is already mounted at %s", devicePath, mountPoint)
		}
	}
}

// Use lsblk(8) to get the value of a single property for the device.
//
// Empty string is returned if property is not present and/or lsblk has
// no access to the device.
func (env Env) getSinglePropertyFromDeviceWithLsblk(devicePath string, property string) (string, error) {
	output, err := env.OsCommandRunner.Run(wrapToEnterHostMountNamespace("lsblk", "-n", "-o", property, devicePath)...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}
