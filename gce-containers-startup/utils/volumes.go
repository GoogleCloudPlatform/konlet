package utils

import (
	"fmt"
	"log"
	"os"
	"strings"

	api "github.com/konlet/types"
)

const (
	ext4FsType               string = "ext4"
	mountedVolumesPathPrefix string = "/mnt/disks/gce-containers-mounts"
)

type VolumeHostPathAndMode struct {
	// nil hostPath means no backing directory, implying tmpfs mount.
	hostPath string
	readOnly bool
}

type TmpFsConfiguration struct {
	path     string
	readOnly bool
}

type HostPathBindConfiguration struct {
	hostPath      string
	containerPath string
	readOnly      bool
}

// Structure represents data that is passed to docker - host binding paths, tmpfs paths and volumes.
type VolumeBindingConfiguration struct {
	// Maps from container name to relevant bindings.
	hostPathBinds []HostPathBindConfiguration
	tmpFsBinds    []TmpFsConfiguration
}

// This is the main interface to this module.
//
// The function takes the API specification and:
//  - Verifies consistency.
//  - Creates/mounts/formats all the necessary volumes.
//  - Outputs all the binding maps, keyed by container name.
//
// The caller should not expect the function to be idempotent. Errors are to be considered non-retryable.
func prepareVolumesAndGetBindings(spec api.ContainerSpecStruct) (map[string]VolumeBindingConfiguration, error) {
	// First, build maps that will allow to verify logical consistency:
	//  - All volumes must be referenced at least once.
	//  - All volume mounts must refer an existing volume.
	volumesReferencesCount := map[string]int{}
	for _, apiVolume := range spec.Volumes {
		volumesReferencesCount[apiVolume.Name] = 0
	}

	for containerIndex, container := range spec.Containers {
		log.Printf("Found %d volume mounts in container %s declaration.", len(container.VolumeMounts), container.Name)
		for _, volumeMount := range container.VolumeMounts {
			if _, present := volumesReferencesCount[volumeMount.Name]; !present {
				return nil, fmt.Errorf("Invalid container declaration: Volume %s referenced in container %s (index: %d) not found in volume definitions.", volumeMount.Name, container.Name, containerIndex)
			} else {
				volumesReferencesCount[volumeMount.Name] += 1
			}
		}
	}
	for volumeName, referenceCount := range volumesReferencesCount {
		if referenceCount == 0 {
			return nil, fmt.Errorf("Invalid container declaration: Volume %s not referenced by any container.", volumeName)
		}
	}

	volumeNameToHostPathMap, volumeNameMapBuildingError := buildVolumeNameToHostPathMap(spec.Volumes)
	if volumeNameMapBuildingError != nil {
		return nil, volumeNameMapBuildingError
	}

	containerBindingConfigurationMap := map[string]VolumeBindingConfiguration{}
	for _, container := range spec.Containers {
		containerBindingConfiguration := VolumeBindingConfiguration{nil, nil}
		for _, volumeMount := range container.VolumeMounts {
			// It has already been checked that the volume is present.
			volumeHostPathAndMode, _ := volumeNameToHostPathMap[volumeMount.Name]

			if volumeHostPathAndMode.readOnly && !volumeMount.ReadOnly {
				return nil, fmt.Errorf("Container %s: volumeMount %s specifies read-write access, but underlying volume is read-only.", container.Name, volumeMount.Name)
			}
			if volumeHostPathAndMode.hostPath == "" {
				containerBindingConfiguration.tmpFsBinds = append(containerBindingConfiguration.tmpFsBinds, TmpFsConfiguration{path: volumeMount.MountPath, readOnly: volumeMount.ReadOnly})
			} else {
				containerBindingConfiguration.hostPathBinds = append(containerBindingConfiguration.hostPathBinds, HostPathBindConfiguration{hostPath: volumeHostPathAndMode.hostPath, containerPath: volumeMount.MountPath, readOnly: volumeMount.ReadOnly})
			}
		}
		containerBindingConfigurationMap[container.Name] = containerBindingConfiguration
	}

	return containerBindingConfigurationMap, nil
}

func buildVolumeNameToHostPathMap(apiVolumes []api.Volume) (map[string]VolumeHostPathAndMode, error) {
	// For each volume, use the proper handler function to build the volume name -> hostpath+mode map.
	volumeNameToHostPathMap := map[string]VolumeHostPathAndMode{}

	for _, apiVolume := range apiVolumes {
		// Enforce exactly one volume definition.
		definitions := 0
		var volumeHostPathAndMode VolumeHostPathAndMode
		var processError error
		if apiVolume.HostPath != nil {
			definitions++
			volumeHostPathAndMode, processError = processHostPathVolume(apiVolume.HostPath)
		}
		if apiVolume.EmptyDir != nil {
			definitions++
			volumeHostPathAndMode, processError = processEmptyDirVolume(apiVolume.EmptyDir)
		}
		if apiVolume.GcePersistentDisk != nil {
			definitions++
			volumeHostPathAndMode, processError = processGcePersistentDiskVolume(apiVolume.GcePersistentDisk)
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

func processEmptyDirVolume(volume *api.EmptyDirVolume) (VolumeHostPathAndMode, error) {
	if volume.Medium != "Memory" {
		return VolumeHostPathAndMode{}, fmt.Errorf("Unsupported emptyDir volume medium: %s", volume.Medium)
	}
	// TODO: For the purpose of preserving data between config updates and
	// sharing data between multiple containers (if supported), actually
	// create and mount the tmpfs here, thus dropping the empty string
	// special case.
	return VolumeHostPathAndMode{hostPath: "", readOnly: false}, nil
}

func processHostPathVolume(volume *api.HostPathVolume) (VolumeHostPathAndMode, error) {
	if _, statError := os.Stat(volume.Path); statError != nil {
		return VolumeHostPathAndMode{}, fmt.Errorf("HostPath directory error: %s", statError)
	}
	// TODO: Check file/directory permissions.
	return VolumeHostPathAndMode{hostPath: volume.Path, readOnly: false}, nil
}

func processGcePersistentDiskVolume(volume *api.GcePersistentDiskVolume) (VolumeHostPathAndMode, error) {
	if volume.FsType != "" && volume.FsType != ext4FsType {
		return VolumeHostPathAndMode{}, fmt.Errof("Unsupported filesystem type: %s", volume.FsType)
	}
	volume.FsType = ext4FsType
	if volume.PdName == "" {
		return VolumeHostPathAndMode{}, fmt.Errof("Empty PD name!")
	}

	devicePath, err := resolveGcePersistentDiskDevicePath(volume.Name)
	if err != nil {
		return VolumeHostPathAndMode{}, fmt.Errorf("Could not resolve GCE Persistent Disk device path: %s", err)
	}
	if volume.partition > 0 {
		devicePath = fmt.Sprintf("%s-part%d", devicePath, volume.partition)
	}

	if err := checkDeviceReadable(devicePath); err != nil {
		return VolumeHostPathAndMode{}, err
	}
	if err := checkDeviceNotMounted(devicePath); err != nil {
		return VolumeHostPathAndMode{}, err
	}
	if err := checkFilesystemAndFormatIfNeeded(devicePath, volume.FsType); err != nil {
		return VolumeHostPathAndMode{}, err
	}

	deviceMountPoint, err := createNewMountPath("gce_persistent_disk", volume.Name)
	if err != nil {
		return VolumeHostPathAndMode{}, err
	}
	if err := mountDevice(devicePath, volume.ReadOnly); err != nil {
		return VolumeHostPathAndMode{}, err
	}

	// Success!
	return VolumeHostPathAndMode{deviceMountPoint, volume.ReadOnly}, nil
}

func resolveGcePersistentDiskDevicePath(pdName string) (string, error) {
	// Currently, only static mapping is supported, as metadata about PD name is not available.
	return fmt.Sprintf("/dev/disk/by-id/google-%s", pdName), nil
}

// Generate a name for the new volume mount, based on the volume family (type)
// and volume name.  Create the directory if necessary, return a path to a
// valid directory to mount the volume in, error otherwise.
func createNewMountPath(volumeFamily string, volumeName string) (string, error) {
	path = fmt.Sprintf("%s/%s/%s", mountedVolumesPathPrefix, volumeFamily, volumeName)
	log.Printf("Creating directory %s as a mount point for volume %s.", path, volumeName)
	if err := os.MkdirAll(path, 0755); err != nil {
		return "", fmt.Errorf("Failed to create directory %s: %s", path, err)
	} else {
		return path, nil
	}
}

// Attempt to mount the device under a generated path. Assumes the device contains an ext4 filesystem.
func mountDevice(devicePath string, mountPath string, fsType string, readOnly bool) error {
	log.Printf("Attempting to mount device %s at %s.", devicePath, mountPath)

	var mountOpts []string
	if readOnly {
		mountOpts = append(mountOpts, "ro")
	} else {
		mountOpts = append(mountOpts, "rw")
	}
	output, err := execCommandWithErrorOutputCommand("mount", "-o", strings.Join(mountOpts, ","), "-t", fsType, devicePath, mountPath)
	if err != nil {
		return fmt.Errorf("Failed to mount %s at %s: %s", devicePath, mountPath, err)
	} else {
		return nil
	}
}

func checkDeviceReadable(devicePath string) error {
	fileInfo, err := os.Stat(devicePath)
	if err != nil {
		return fmt.Errorf("Device %s access error: %s", devicePath, err)
	}
	if !(fileInfo.Mode() & os.ModeDevice) || (fileInfo.Mode() & os.ModeCharDevice) {
		return fmt.Errorf("Device %s is not a block device.", devicePath)
	}
	// TODO: More detailed access checks.
	return nil
}

func checkFilesystemAndFormatIfNeeded(devicePath string, configuredFsType string) error {
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
	foundFsType, err := getSinglePropertyFromDeviceWithLsblk(devicePath, lsblkFsType)
	if err != nil {
		return err
	}
	// Unfortunately, lsblk(8) doesn't provide a way to tell apart a
	// nonexistent filesystem (e.g. a fresh drive) from device read problem
	// - in both cases not reporting any errors and returning an empty
	// FSTYPE field. Therefore, care must be taken to compensate for this behaviour. The strategy below is deemed safe, because:
	//
	// - If lsblk(8) lacks privileges to read the filesystem and the
	//   decision is put forward to format it, mkfs(8) will fail as well.
	// - If lsblk(8) had privileges and still didn't detect the filesystem,
	//   it's OK to format it.
	if foundFsType == "" {
		// Need to format.
		log.Printf("Formatting device %s with filesystem %s...", devicePath, configuredFsType)
		output, err := runCommandOnArgsArrayAppendingLastArgument(filesystemFormatter, devicePath)
		if err != nil {
			return fmt.Errrof("Failed to format filesystem: %s", output)
		} else {
			log.Printf("%s\n", output)
		}
	} else if foundFsType == configuredFsType {
		// Need to fsck.
		log.Printf("Running filesystem checker on device %s...", devicePath)
		output, err := runCommandOnArgsArrayAppendingLastArgument(filesystemChecker, devicePath)
		if err != nil {
			return fmt.Errrof("Filesystem check failed: %s", err)
		} else {
			log.Printf("%s\n", output)
		}
	} else {
		return fmt.Errorf("Device %s: found filesystem type %s, expected %s.", devicePath, foundFsType, configuredFsType)
	}
	return nil
}

// os.exec.Command() takes exactly opposite structure of arguments (command
// first, then arguments as vararg), so we need to reverse that.
func runCommandOnArgsArrayAppendingLastArgument(args []string, lastArgument string) (string, error) {
	if len(args) == 0 {
		return fmt.Errorf("No command provided.")
	}
	command := args[0]
	execArgs := append(args[1:], lastArgument)
	return execCommandWithErrorOutput(command, execArgs)
}

// Return non-nil error with meaningful message when the device is already mounted.
func checkDeviceNotMounted(devicePath string) error {
	const lsblkMountPoint string = "MOUNTPOINT"
	if mountPoint, err = getSinglePropertyFromDeviceWithLsblk(devicePath, lsblkMountPoint); err != nil {
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
func getSinglePropertyFromDeviceWithLsblk(devicePath string, property string) (string, error) {
	output, err = execCommandWithErrorOutput("lsblk", "-n", "-o", property, devicePath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// Wrap around os.exec.Command(...).CombinedOutput() to glue together output
// (STDERR+STDOUT) and execution error message upon failure.
//
// Convert the []byte output to string as well.
func execCommandWithErrorOutput(command string, args ...string) (string, error) {
	output, err := os.exec.Command(command, args).CombinedOutput()
	outputString = string(output)
	if err != nil {
		errorString := string(err)
		if outputString {
			errorString = fmt.Sprintf("%s, details: %s", errorString, outputString)
		}
	}
	return outputString, nil
}
