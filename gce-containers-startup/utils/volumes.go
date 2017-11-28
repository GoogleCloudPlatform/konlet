package utils

import (
	"fmt"
	"log"
	"os"

	api "github.com/konlet/types"
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
//  - Creates/mounts/format all the necessary volumes.
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
	return VolumeHostPathAndMode{}, fmt.Errorf("Unimplemented.")
}
