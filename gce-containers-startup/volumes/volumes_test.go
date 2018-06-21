// Copyright 2018 Google Inc. All Rights Reserved.
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
	"errors"
	"fmt"
	"github.com/GoogleCloudPlatform/konlet/gce-containers-startup/command"
	"github.com/GoogleCloudPlatform/konlet/gce-containers-startup/metadata"
	api "github.com/GoogleCloudPlatform/konlet/gce-containers-startup/types"
	"github.com/GoogleCloudPlatform/konlet/gce-containers-startup/utils"
	yaml "gopkg.in/yaml.v2"
	"testing"
)

func Test_UnmountExistingVolumes_UnmountsVolumesInKonletVolumeDirectory(t *testing.T) {
	commandRunner := command.NewMockRunner(t)
	commandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts",
		`tmpfs /mnt/disks/gce-containers-mounts/tmpfss/volume tmpfs rw,nosuid,nodev 0 0
/dev/disk /mnt/disks/gce-containers-mounts/gce-persistent-disks/disk ext4 ro 0 0`)
	commandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- umount /mnt/disks/gce-containers-mounts/tmpfss/volume", "")
	commandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- umount /mnt/disks/gce-containers-mounts/gce-persistent-disks/disk", "")
	env := Env{
		OsCommandRunner: commandRunner,
	}
	err := env.UnmountExistingVolumes()
	utils.AssertNoError(t, err)
	commandRunner.AssertAllCalled()
}

func Test_UnmountExistingVolumes_DoesNotUnmountVolumesOutsideOfKonletVolumeDirectory(t *testing.T) {
	commandRunner := command.NewMockRunner(t)
	commandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts",
		"tmpfs /mnt/disks/kubernetes/volume tmpfs rw,nosuid,nodev 0 0")
	env := Env{
		OsCommandRunner: commandRunner,
	}
	err := env.UnmountExistingVolumes()
	utils.AssertNoError(t, err)
	commandRunner.AssertAllCalled()
}

func Test_UnmountExistingVolumes_ContinuesIfUnmountingFails(t *testing.T) {
	commandRunner := command.NewMockRunner(t)
	commandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts",
		`tmpfs /mnt/disks/gce-containers-mounts/tmpfss/volume tmpfs rw,nosuid,nodev 0 0
/dev/disk /mnt/disks/gce-containers-mounts/gce-persistent-disks/disk ext4 ro 0 0`)
	commandRunner.ErrorOnCall("nsenter --mount=/host_proc/1/ns/mnt -- umount /mnt/disks/gce-containers-mounts/tmpfss/volume", errors.New("volume in use"))
	commandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- umount /mnt/disks/gce-containers-mounts/gce-persistent-disks/disk", "")
	env := Env{
		OsCommandRunner: commandRunner,
	}
	err := env.UnmountExistingVolumes()
	utils.AssertError(t, err, "Failed to unmount tmpfs at /mnt/disks/gce-containers-mounts/tmpfss/volume: volume in use")
	commandRunner.AssertAllCalled()
}

func TestPrepareVolumesAndGetBindings_FailsIfContainerReferencesUndefinedVolume(t *testing.T) {
	declaration := `
spec:
  containers:
  - name: 'name'
    image: 'image'
    volumeMounts:
    - name: 'volume'
      mountPath: 'path'
      readOnly: true
`
	diskMetadata := "[]"
	env, spec := setup(t, declaration, diskMetadata)
	_, err := env.PrepareVolumesAndGetBindings(spec)
	utils.AssertError(t, err, "Invalid container declaration: Volume volume referenced in container name (index: 0) not found in volume definitions.")
}

func TestPrepareVolumesAndGetBindings_FailsIfVolumeIsNotReferenced(t *testing.T) {
	declaration := `
spec:
  containers:
  - name: 'name'
    image: 'image'
  volumes:
  - name: 'volume'
    hostPath:
      path: 'path'
`
	diskMetadata := "[]"
	env, spec := setup(t, declaration, diskMetadata)
	_, err := env.PrepareVolumesAndGetBindings(spec)
	utils.AssertError(t, err, "Invalid container declaration: Volume volume not referenced by any container.")
}

func TestPrepareVolumesAndGetBindings_FailsIfEmptyDirMediumIsNotMemory(t *testing.T) {
	declaration := `
  spec:
    containers:
    - name: 'name'
      image: 'image'
      volumeMounts:
      - name: 'volume'
        mountPath: 'containerPath'
    volumes:
    - name: 'volume'
      emptyDir:
        medium: 'Disk'
  `
	diskMetadata := "[]"
	env, spec := setup(t, declaration, diskMetadata)
	_, err := env.PrepareVolumesAndGetBindings(spec)
	utils.AssertError(t, err, "Volume volume: Unsupported emptyDir volume medium: Disk")
}

func TestPrepareVolumesAndGetBindings_ReturnsBindingForHostPathMount(t *testing.T) {
	declaration := `
spec:
  containers:
  - name: 'name'
    image: 'image'
    volumeMounts:
    - name: 'volume'
      mountPath: 'containerPath'
  volumes:
  - name: 'volume'
    hostPath:
      path: 'hostPath'
`
	diskMetadata := "[]"
	env, spec := setup(t, declaration, diskMetadata)
	bindings, err := env.PrepareVolumesAndGetBindings(spec)
	utils.AssertNoError(t, err)
	expectedBindings := map[string][]HostPathBindConfiguration{
		"name": {
			{
				HostPath:      "hostPath",
				ContainerPath: "containerPath",
				ReadOnly:      false,
			},
		},
	}
	utils.AssertEqual(t, bindings, expectedBindings, "")
}

func TestPrepareVolumesAndGetBindings_CorrectlySetsReadOnlyMode(t *testing.T) {
	declaration := `
spec:
  containers:
  - name: 'name'
    image: 'image'
    volumeMounts:
    - name: 'volume'
      mountPath: 'containerPath1'
      readOnly: true
    - name: 'volume'
      mountPath: 'containerPath2'
      readOnly: false
  volumes:
  - name: 'volume'
    hostPath:
      path: 'hostPath'
`
	diskMetadata := "[]"
	env, spec := setup(t, declaration, diskMetadata)
	bindings, err := env.PrepareVolumesAndGetBindings(spec)
	utils.AssertNoError(t, err)
	expectedBindings := map[string][]HostPathBindConfiguration{
		"name": {
			{
				HostPath:      "hostPath",
				ContainerPath: "containerPath1",
				ReadOnly:      true,
			},
			{
				HostPath:      "hostPath",
				ContainerPath: "containerPath2",
				ReadOnly:      false,
			},
		},
	}
	utils.AssertEqual(t, bindings, expectedBindings, "")
}

func TestPrepareVolumesAndGetBindings_DoesNotCreateHostDirectoryForHostPathMount(t *testing.T) {
	declaration := `
spec:
  containers:
  - name: 'name'
    image: 'image'
    volumeMounts:
    - name: 'volume'
      mountPath: 'containerPath'
  volumes:
  - name: 'volume'
    hostPath:
      path: 'hostPath'
`
	diskMetadata := "[]"
	commandRunner := command.NewMockRunner(t)
	commandRunner.FailOnUnexpectedCalls = true
	env, spec := setupWithCommandRunner(t, declaration, diskMetadata, commandRunner)
	_, err := env.PrepareVolumesAndGetBindings(spec)
	utils.AssertNoError(t, err)
}

func TestPrepareVolumesAndGetBindings_ReturnsBindingToHostPathForEmptyDirMount(t *testing.T) {
	declaration := `
spec:
  containers:
  - name: 'name'
    image: 'image'
    volumeMounts:
    - name: 'volume'
      mountPath: 'containerPath'
  volumes:
  - name: 'volume'
    emptyDir:
      medium: 'Memory'
`
	diskMetadata := "[]"
	env, spec := setup(t, declaration, diskMetadata)
	bindings, err := env.PrepareVolumesAndGetBindings(spec)
	utils.AssertNoError(t, err)
	expectedBindings := map[string][]HostPathBindConfiguration{
		"name": {
			{
				HostPath:      "/mnt/disks/gce-containers-mounts/tmpfss/volume",
				ContainerPath: "containerPath",
				ReadOnly:      false,
			},
		},
	}
	utils.AssertEqual(t, bindings, expectedBindings, "")
}

func TestPrepareVolumesAndGetBindings_CreatesTMPFSHostDirectoryForEmptyDir(t *testing.T) {
	declaration := `
spec:
  containers:
  - name: 'name'
    image: 'image'
    volumeMounts:
    - name: 'volume'
      mountPath: 'containerPath'
  volumes:
  - name: 'volume'
    emptyDir:
      medium: 'Memory'
`
	diskMetadata := "[]"
	commandRunner := command.NewMockRunner(t)
	commandRunner.RegisterMkdirAll("/mnt/disks/gce-containers-mounts/tmpfss/volume")
	commandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- mount -o rw -t tmpfs tmpfs /mnt/disks/gce-containers-mounts/tmpfss/volume", "")
	env, spec := setupWithCommandRunner(t, declaration, diskMetadata, commandRunner)
	_, err := env.PrepareVolumesAndGetBindings(spec)
	utils.AssertNoError(t, err)
	commandRunner.AssertAllCalled()
}

func TestPrepareVolumesAndGetBindings_CreatesCorrectBindingsForMultipleVolumes(t *testing.T) {
	declaration := `
spec:
  containers:
  - name: 'name'
    image: 'image'
    volumeMounts:
    - name: 'volume1'
      mountPath: 'containerPath1'
    - name: 'volume2'
      mountPath: 'containerPath2'
  volumes:
  - name: 'volume1'
    emptyDir:
      medium: 'Memory'
  - name: 'volume2'
    hostPath:
      path: 'hostPath'
`
	diskMetadata := "[]"
	env, spec := setup(t, declaration, diskMetadata)
	bindings, err := env.PrepareVolumesAndGetBindings(spec)
	utils.AssertNoError(t, err)
	expectedBindings := map[string][]HostPathBindConfiguration{
		"name": {
			{
				HostPath:      "/mnt/disks/gce-containers-mounts/tmpfss/volume1",
				ContainerPath: "containerPath1",
			},
			{
				HostPath:      "hostPath",
				ContainerPath: "containerPath2",
			},
		},
	}
	utils.AssertEqual(t, bindings, expectedBindings, "")
}

func TestPrepareVolumesAndGetBindings_FailsIfMultipleVolumeTypesAreSpecified(t *testing.T) {
	declaration := `
spec:
  containers:
  - name: 'name'
    image: 'image'
    volumeMounts:
    - name: 'volume'
      mountPath: 'containerPath'
  volumes:
  - name: 'volume'
    emptyDir:
      medium: 'Memory'
    hostPath:
      path: 'hostPath'
`
	diskMetadata := "[]"
	env, spec := setup(t, declaration, diskMetadata)
	_, err := env.PrepareVolumesAndGetBindings(spec)
	utils.AssertError(t, err, "Invalid container declaration: Exactly one volume specification required for volume volume, 2 found.")
}

func TestPrepareVolumesAndGetBindings_MultipleMountsCanReferenceTheSameVolume(t *testing.T) {
	declaration := `
spec:
  containers:
  - name: 'name'
    image: 'image'
    volumeMounts:
    - name: 'volume'
      mountPath: 'containerPath1'
      readOnly: false
    - name: 'volume'
      mountPath: 'containerPath2'
      readOnly: true
  volumes:
  - name: 'volume'
    hostPath:
      path: 'hostPath'
`
	diskMetadata := "[]"
	env, spec := setup(t, declaration, diskMetadata)
	bindings, err := env.PrepareVolumesAndGetBindings(spec)
	utils.AssertNoError(t, err)
	expectedBindings := map[string][]HostPathBindConfiguration{
		"name": {
			{
				HostPath:      "hostPath",
				ContainerPath: "containerPath1",
				ReadOnly:      false,
			},
			{
				HostPath:      "hostPath",
				ContainerPath: "containerPath2",
				ReadOnly:      true,
			},
		},
	}
	utils.AssertEqual(t, bindings, expectedBindings, "")
}

func TestPrepareVolumesAndGetBindings_MultipleHostPathVolumesCanHaveTheSameHostPath(t *testing.T) {
	declaration := `
spec:
  containers:
  - name: 'name'
    image: 'image'
    volumeMounts:
    - name: 'volume1'
      mountPath: 'containerPath1'
      readOnly: false
    - name: 'volume2'
      mountPath: 'containerPath2'
      readOnly: true
  volumes:
  - name: 'volume1'
    hostPath:
      path: 'hostPath'
  - name: 'volume2'
    hostPath:
      path: 'hostPath'
`
	diskMetadata := "[]"
	env, spec := setup(t, declaration, diskMetadata)
	bindings, err := env.PrepareVolumesAndGetBindings(spec)
	utils.AssertNoError(t, err)
	expectedBindings := map[string][]HostPathBindConfiguration{
		"name": {
			{
				HostPath:      "hostPath",
				ContainerPath: "containerPath1",
				ReadOnly:      false,
			},
			{
				HostPath:      "hostPath",
				ContainerPath: "containerPath2",
				ReadOnly:      true,
			},
		},
	}
	utils.AssertEqual(t, bindings, expectedBindings, "")
}

// setup is a variant of setupWithCommandRunner that uses a MockRunner that
// does not fail if specific commands were not run. Use it if you don't care
// about the command interaction with the os.
func setup(t *testing.T, declaration string, diskMetadata string) (Env, api.ContainerSpecStruct) {
	runner := command.NewMockRunner(t)
	runner.FailOnUnexpectedCalls = false
	return setupWithCommandRunner(t, declaration, diskMetadata, runner)
}

// setupWithCommandRunner is a utility function that performs common setup for
// all volume tests. It takes a container declaration, disk metadata and
// a command runner and returns a pair of:
// - volume environment that can be used to test its only exported method,
//   PrepareVolumesAndGetBindings.
// - a container spec struct, which is the only expected argument to this
//   method.
// You can configure the command runner to set appropriate expectations and
// responses and later check if they were matched.
func setupWithCommandRunner(t *testing.T, declaration string, diskMetadata string, runner OsCommandRunner) (Env, api.ContainerSpecStruct) {
	metadataProvider := metadata.ProviderStub{
		Manifest:         declaration,
		DiskMetadataJson: diskMetadata,
	}
	env := Env{
		OsCommandRunner:  runner,
		MetadataProvider: metadataProvider,
	}
	spec, err := parseDeclaration(declaration)
	utils.AssertNoError(t, err) // check if declaration is syntactically ok
	return env, *spec
}

func parseDeclaration(declaration string) (*api.ContainerSpecStruct, error) {
	spec := api.ContainerSpec{}
	if err := yaml.Unmarshal([]byte(declaration), &spec); err != nil {
		return nil, fmt.Errorf("Cannot parse container declaration '%s': %v", declaration, err)
	}
	return &spec.Spec, nil
}
