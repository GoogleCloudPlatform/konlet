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

package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/konlet/gce-containers-startup/command"
	"github.com/GoogleCloudPlatform/konlet/gce-containers-startup/metadata"
	"github.com/GoogleCloudPlatform/konlet/gce-containers-startup/runtime"
	"github.com/GoogleCloudPlatform/konlet/gce-containers-startup/utils"
	"github.com/GoogleCloudPlatform/konlet/gce-containers-startup/volumes"

	dockertypes "github.com/docker/engine-api/types"
	dockercontainer "github.com/docker/engine-api/types/container"
	dockernetwork "github.com/docker/engine-api/types/network"
	dockerstrslice "github.com/docker/engine-api/types/strslice"

	"golang.org/x/net/context"
)

const SIMPLE_MANIFEST = `
spec:
  containers:
  - name: 'test-simple'
    image: 'gcr.io/gce-containers/apache:v1'`

const RUN_COMMAND_MANIFEST = `
spec:
  containers:
  - name: 'test-run-command'
    image: 'gcr.io/google-containers/busybox:latest'
    command: ['ls']
    args: ["-l", "/tmp"]`

const RUN_ARGS_MANIFEST = `
spec:
  containers:
  - name: 'test-run-command'
    image: 'gcr.io/google-containers/busybox:latest'
    command: ['echo']
    args: ["-n", "Hello \" world", "Welco'me"]`

const ENVVARS_MANIFEST = `
spec:
  containers:
  - name: 'test-env-vars'
    image: 'gcr.io/google-containers/busybox:latest'
    command: ['env']
    env:
    - name: 'VAR1'
      value: 'VAL1'
    - name: 'VAR2'
      value: 'VAL2'`

const VOLUME_MANIFEST = `
spec:
  containers:
  - name: 'test-volume'
    image: 'gcr.io/google-containers/busybox:latest'
    volumeMounts:
    - name: 'vol1'
      mountPath: '/tmp/host-1'
      readOnly: true
    - name: 'vol2'
      mountPath: '/tmp/host-2'
    - name: 'vol3'
      mountPath: '/tmp/host-3'
      readOnly: false
    - name: 'vol4'
      mountPath: '/tmp/host-4'
  volumes:
  - name: 'vol1'
    hostPath:
      path: '/tmp'
  - name: 'vol2'
    emptyDir:
      medium: 'Memory'
  - name: 'vol3'
    hostPath:
      path: '/tmp'
  - name: 'vol4'
    hostPath:
      path: '/tmp'`

const GCE_PD_VOLUME_MANIFEST = `
spec:
  containers:
  - name: 'test-volume'
    image: 'gcr.io/google-containers/busybox:latest'
    volumeMounts:
    - name: 'pd1'
      mountPath: '/tmp/pd1'
      readOnly: false
  volumes:
  - name: 'pd1'
    gcePersistentDisk:
      pdName: 'gce-pd-name-here'
      fsType: 'ext4'`

const GCE_PD_VOLUME_WITH_RO_MOUNT_MANIFEST = `
spec:
  containers:
  - name: 'test-volume'
    image: 'gcr.io/google-containers/busybox:latest'
    volumeMounts:
    - name: 'pd1'
      mountPath: '/tmp/pd1'
      readOnly: true
  volumes:
  - name: 'pd1'
    gcePersistentDisk:
      pdName: 'gce-pd-name-here'
      fsType: 'ext4'`

const GCE_PD_DEV_PATH = "/dev/disk/by-id/google-gce-pd-name-here"
const GCE_PD_HOST_MOUNT_PATH = "/mnt/disks/gce-containers-mounts/gce-persistent-disks/gce-pd-name-here"

const GCE_PD_VOLUME_INVALID_FS_MANIFEST = `
spec:
  containers:
  - name: 'test-volume'
    image: 'gcr.io/google-containers/busybox:latest'
    volumeMounts:
    - name: 'pd1'
      mountPath: '/tmp/pd1'
      readOnly: false
  volumes:
  - name: 'pd1'
    gcePersistentDisk:
      pdName: 'gce-pd-name-here'
      fsType: 'nfts'`

const GCE_PD_VOLUME_WITH_PARTITION_MANIFEST = `
spec:
  containers:
  - name: 'test-volume'
    image: 'gcr.io/google-containers/busybox:latest'
    volumeMounts:
    - name: 'pd1'
      mountPath: '/tmp/pd1'
      readOnly: false
  volumes:
  - name: 'pd1'
    gcePersistentDisk:
      pdName: 'gce-pd-name-here'
      fsType: 'ext4'
      partition: 8`

const OPTIONS_MANIFEST = `
spec:
  containers:
  - name: 'test-options'
    image: 'gcr.io/google-containers/busybox:latest'
    command: ['sleep']
    args: ['1000']
    securityContext:
      privileged: true
    tty: true
    stdin: true`

const MULTICONTAINER_MANIFEST = `
spec:
  containers:
  - name: 'test-options-1'
    image: 'gcr.io/google-containers/busybox:latest'
  - name: 'test-options-2'
    image: 'gcr.io/google-containers/busybox:latest'`

const REMOVE_MANIFEST = `
spec:
  containers:
  - name: 'test-remove'
    image: 'gcr.io/google-containers/busybox:latest'`

const RESTART_POLICY_MANIFEST = `
spec:
  restartPolicy: OnFailure
  containers:
  - name: 'test-restart-policy'
    image: 'gcr.io/google-containers/busybox:latest'`

const INVALID_RESTART_POLICY_MANIFEST = `
spec:
  restartPolicy: EachSunday
  containers:
  - name: 'test-restart-policy'
    image: 'gcr.io/google-containers/busybox:latest'`

const PROBLEM_MANIFEST = `
spec:
  containers:
    - name: test-07-rc01
      image: gcr.io/google-containers/busybox
      command:
        - ls
      args:
        - /
      volumeMounts:
        - name: host-path-1
          mountPath: /tmp-host
          readOnly: false
        - name: emptydir-1
          mountPath: /tmp-tmpfs
  restartPolicy: OnFailure
  volumes:
    - name: host-path-1
      hostPath:
        path: /tmp
    - name: emptydir-1
      emptyDir:
        medium: Memory`

const MANIFEST_WITH_IGNORED_POD_FIELDS = `
apiVersion: 'v1'
kind: 'Pod'
spec:
  containers:
  - name: 'test-simple'
    image: 'gcr.io/gce-containers/apache:v1'`

const TMPFS_HOST_MOUNT_PATH_PREFIX = "/mnt/disks/gce-containers-mounts/tmpfss/"

const SINGLE_DISK_METADATA = `
[
	{
		"deviceName": "main-instance-disk",
		"index": 0,
		"mode": "READ_WRITE",
		"type": "PERSISTENT"
	}
]`

const GCE_ATTACHED_RO_DISK_METADATA = `
[
	{
		"deviceName": "main-instance-disk",
		"index": 0,
		"mode": "READ_WRITE",
		"type": "PERSISTENT"
	},
	{
		"deviceName": "gce-pd-name-here",
		"index": 1,
		"mode": "READ_ONLY",
		"type": "PERSISTENT"
	}
]`

const GCE_ATTACHED_RW_DISK_METADATA = `
[
	{
		"deviceName": "main-instance-disk",
		"index": 0,
		"mode": "READ_WRITE",
		"type": "PERSISTENT"
	},
	{
		"deviceName": "gce-pd-name-here",
		"index": 1,
		"mode": "READ_WRITE",
		"type": "PERSISTENT"
	}
]`

const MOCK_AUTH_TOKEN = "123123123="
const MOCK_CONTAINER_ID = "1234567"
const MOCK_EXISTING_CONTAINER_ID = "123123123"

type MockDockerApi struct {
	PulledImage      string
	ContainerName    string
	CreateRequest    *dockercontainer.Config
	HostConfig       *dockercontainer.HostConfig
	StartedContainer string
	RemovedContainer string
}

func (api *MockDockerApi) ImagePull(ctx context.Context, ref string, options dockertypes.ImagePullOptions) (io.ReadCloser, error) {
	api.PulledImage = ref
	return ioutil.NopCloser(strings.NewReader("--- Image pulled ---")), nil
}

func (api *MockDockerApi) ContainerCreate(ctx context.Context, config *dockercontainer.Config, hostConfig *dockercontainer.HostConfig, networkingConfig *dockernetwork.NetworkingConfig, containerName string) (dockertypes.ContainerCreateResponse, error) {
	api.ContainerName = containerName
	api.CreateRequest = config
	api.HostConfig = hostConfig
	return dockertypes.ContainerCreateResponse{ID: MOCK_CONTAINER_ID}, nil
}

func (api *MockDockerApi) ContainerStart(ctx context.Context, container string) error {
	api.StartedContainer = container
	return nil
}

func (api *MockDockerApi) ContainerList(ctx context.Context, opts dockertypes.ContainerListOptions) ([]dockertypes.Container, error) {
	return []dockertypes.Container{
		dockertypes.Container{ID: MOCK_EXISTING_CONTAINER_ID, Names: []string{"/klt-test-remove-xvlb"}},
	}, nil
}

func (api *MockDockerApi) ContainerRemove(ctx context.Context, containerID string, opts dockertypes.ContainerRemoveOptions) error {
	api.RemovedContainer = containerID
	return nil
}

func ExecStartupWithMocksAndFakes(mockDockerApi *MockDockerApi, mockCommandRunner *command.MockRunner, manifest string, diskMetadata string) error {
	metadataProviderStub := metadata.ProviderStub{Manifest: manifest, DiskMetadataJson: diskMetadata}
	volumesEnv := &volumes.Env{OsCommandRunner: mockCommandRunner, MetadataProvider: metadataProviderStub}
	// This will generate "xvlb" suffix for container name.
	randEnv := rand.New(rand.NewSource(1))
	return ExecStartup(
		metadataProviderStub,
		utils.ConstantTokenProvider{Token: MOCK_AUTH_TOKEN},
		&runtime.ContainerRunner{Client: mockDockerApi, VolumesEnv: volumesEnv, RandEnv: randEnv},
		false, /* openIptables */
	)
}

func TestExecStartup_simple(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := command.NewMockRunner(t)
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts", "")
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		SIMPLE_MANIFEST,
		SINGLE_DISK_METADATA)

	utils.AssertNoError(t, err)
	utils.AssertEqual(t, "klt-test-simple-xvlb", mockDockerClient.ContainerName, "")
	utils.AssertEqual(t, "gcr.io/gce-containers/apache:v1", mockDockerClient.PulledImage, "")
	utils.AssertEqual(t, "gcr.io/gce-containers/apache:v1", mockDockerClient.CreateRequest.Image, "")
	utils.AssertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	utils.AssertEqual(t, MOCK_EXISTING_CONTAINER_ID, mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultOptions(t)
}

func TestExecStartup_runCommand(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := command.NewMockRunner(t)
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts", "")
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		RUN_COMMAND_MANIFEST,
		SINGLE_DISK_METADATA)

	utils.AssertNoError(t, err)
	utils.AssertEqual(t, "klt-test-run-command-xvlb", mockDockerClient.ContainerName, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	utils.AssertEqual(t, dockerstrslice.StrSlice([]string{"ls"}), mockDockerClient.CreateRequest.Entrypoint, "")
	utils.AssertEqual(t, dockerstrslice.StrSlice([]string{"-l", "/tmp"}), mockDockerClient.CreateRequest.Cmd, "")
	utils.AssertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	utils.AssertEqual(t, MOCK_EXISTING_CONTAINER_ID, mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultOptions(t)
}

func TestExecStartup_runArgs(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := command.NewMockRunner(t)
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts", "")
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		RUN_ARGS_MANIFEST,
		SINGLE_DISK_METADATA)

	utils.AssertNoError(t, err)
	utils.AssertEqual(t, "klt-test-run-command-xvlb", mockDockerClient.ContainerName, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	utils.AssertEqual(t, dockerstrslice.StrSlice([]string{"echo"}), mockDockerClient.CreateRequest.Entrypoint, "")
	utils.AssertEqual(t, dockerstrslice.StrSlice([]string{"-n", "Hello \" world", "Welco'me"}), mockDockerClient.CreateRequest.Cmd, "")
	utils.AssertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	utils.AssertEqual(t, MOCK_EXISTING_CONTAINER_ID, mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultOptions(t)
}

func TestExecStartup_env(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := command.NewMockRunner(t)
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts", "")
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		ENVVARS_MANIFEST,
		SINGLE_DISK_METADATA)

	utils.AssertNoError(t, err)
	utils.AssertEqual(t, "klt-test-env-vars-xvlb", mockDockerClient.ContainerName, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	utils.AssertEqual(t, dockerstrslice.StrSlice([]string{"env"}), mockDockerClient.CreateRequest.Entrypoint, "")
	utils.AssertEqual(t, []string{"VAR1=VAL1", "VAR2=VAL2"}, mockDockerClient.CreateRequest.Env, "")
	utils.AssertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	utils.AssertEqual(t, MOCK_EXISTING_CONTAINER_ID, mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultOptions(t)
}

func TestExecStartup_volumeMounts(t *testing.T) {
	const EMPTYDIR_HOST_MOUNT_PATH = TMPFS_HOST_MOUNT_PATH_PREFIX + "vol2"
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := command.NewMockRunner(t)
	mockCommandRunner.RegisterMkdirAll(EMPTYDIR_HOST_MOUNT_PATH)
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts", "")
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- mount -o rw -t tmpfs tmpfs "+EMPTYDIR_HOST_MOUNT_PATH, "")
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		VOLUME_MANIFEST,
		SINGLE_DISK_METADATA)

	utils.AssertNoError(t, err)
	utils.AssertEqual(t, "klt-test-volume-xvlb", mockDockerClient.ContainerName, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	utils.AssertEqual(t,
		[]string{"/tmp:/tmp/host-1:ro", EMPTYDIR_HOST_MOUNT_PATH + ":/tmp/host-2", "/tmp:/tmp/host-3", "/tmp:/tmp/host-4"},
		mockDockerClient.HostConfig.Binds,
		"")
	utils.AssertEmpty(t, mockDockerClient.HostConfig.Tmpfs, "")
	utils.AssertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	utils.AssertEqual(t, MOCK_EXISTING_CONTAINER_ID, mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultOptions(t)
}

func TestExecStartup_options(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := command.NewMockRunner(t)
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts", "")
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		OPTIONS_MANIFEST,
		SINGLE_DISK_METADATA)

	utils.AssertNoError(t, err)
	utils.AssertEqual(t, "klt-test-options-xvlb", mockDockerClient.ContainerName, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	utils.AssertEqual(t, dockerstrslice.StrSlice([]string{"sleep"}), mockDockerClient.CreateRequest.Entrypoint, "")
	utils.AssertEqual(t, dockerstrslice.StrSlice([]string{"1000"}), mockDockerClient.CreateRequest.Cmd, "")
	utils.AssertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	utils.AssertEqual(t, mockDockerClient.HostConfig.Privileged, true, "")
	utils.AssertEqual(t, mockDockerClient.CreateRequest.OpenStdin, true, "")
	utils.AssertEqual(t, mockDockerClient.CreateRequest.Tty, true, "")
	utils.AssertEqual(t, MOCK_EXISTING_CONTAINER_ID, mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultSystemOptions(t)
}

func TestExecStartup_removeContainer(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := command.NewMockRunner(t)
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts", "")
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		REMOVE_MANIFEST,
		SINGLE_DISK_METADATA)

	utils.AssertNoError(t, err)
	utils.AssertEqual(t, "klt-test-remove-xvlb", mockDockerClient.ContainerName, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	utils.AssertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	utils.AssertEqual(t, MOCK_EXISTING_CONTAINER_ID, mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultSystemOptions(t)
}

func TestExecStartup_noMultiContainer(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := command.NewMockRunner(t)
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts", "")
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		MULTICONTAINER_MANIFEST,
		SINGLE_DISK_METADATA)

	utils.AssertError(t, err, "Container declaration should include exactly 1 container, 2 found")
}

func TestExecStartup_emptyManifest(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := command.NewMockRunner(t)
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts", "")
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		"",
		SINGLE_DISK_METADATA)

	utils.AssertError(t, err, "Container declaration should include exactly 1 container, 0 found")
}

func TestExecStartup_restartPolicy(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := command.NewMockRunner(t)
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts", "")
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		RESTART_POLICY_MANIFEST,
		SINGLE_DISK_METADATA)

	utils.AssertNoError(t, err)
	utils.AssertEqual(t, "klt-test-restart-policy-xvlb", mockDockerClient.ContainerName, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	utils.AssertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	utils.AssertEqual(t, MOCK_EXISTING_CONTAINER_ID, mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultSystemOptions(t)
	utils.AssertEqual(t, mockDockerClient.HostConfig.Privileged, false, "")
	utils.AssertEqual(t, mockDockerClient.HostConfig.RestartPolicy.Name, "on-failure", "")
	utils.AssertEqual(t, mockDockerClient.CreateRequest.User, "", "")
	utils.AssertEqual(t, mockDockerClient.CreateRequest.OpenStdin, false, "")
	utils.AssertEqual(t, mockDockerClient.CreateRequest.Tty, false, "")
}

func TestExecStartup_invalidRestartPolicy(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := command.NewMockRunner(t)
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts", "")
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		INVALID_RESTART_POLICY_MANIFEST,
		SINGLE_DISK_METADATA)

	utils.AssertError(t, err, "Failed to start container: Invalid container declaration: Unsupported container restart policy 'EachSunday'")
}

func TestExecStartup_problem(t *testing.T) {
	const EMPTYDIR_HOST_MOUNT_PATH = TMPFS_HOST_MOUNT_PATH_PREFIX + "emptydir-1"
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := command.NewMockRunner(t)
	mockCommandRunner.RegisterMkdirAll(EMPTYDIR_HOST_MOUNT_PATH)
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts", "")
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- mount -o rw -t tmpfs tmpfs "+EMPTYDIR_HOST_MOUNT_PATH, "")
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		PROBLEM_MANIFEST,
		SINGLE_DISK_METADATA)

	utils.AssertNoError(t, err)
	utils.AssertEqual(t,
		[]string{"/tmp:/tmp-host", EMPTYDIR_HOST_MOUNT_PATH + ":/tmp-tmpfs"},
		mockDockerClient.HostConfig.Binds,
		"")
	utils.AssertEmpty(t, mockDockerClient.HostConfig.Tmpfs, "")
}

func TestExecStartup_ignorePodFields(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := command.NewMockRunner(t)
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts", "")
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		MANIFEST_WITH_IGNORED_POD_FIELDS,
		SINGLE_DISK_METADATA)

	utils.AssertNoError(t, err)
	utils.AssertEqual(t, "klt-test-simple-xvlb", mockDockerClient.ContainerName, "")
	utils.AssertEqual(t, "gcr.io/gce-containers/apache:v1", mockDockerClient.PulledImage, "")
	utils.AssertEqual(t, "gcr.io/gce-containers/apache:v1", mockDockerClient.CreateRequest.Image, "")
	utils.AssertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	utils.AssertEqual(t, MOCK_EXISTING_CONTAINER_ID, mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultOptions(t)
}

func TestExecStartup_pdValidAndFormatted(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := command.NewMockRunner(t)
	// Prepare the whole PD command chain.
	mockCommandRunner.RegisterDeviceForStat(GCE_PD_DEV_PATH)
	mockCommandRunner.RegisterMkdirAll(GCE_PD_HOST_MOUNT_PATH)
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts", "")
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o FSTYPE "+GCE_PD_DEV_PATH, "ext4")
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o MOUNTPOINT "+GCE_PD_DEV_PATH, "")
	mockCommandRunner.OutputOnCall("fsck.ext4 -p "+GCE_PD_DEV_PATH, "fsck running running... done!")
	mockCommandRunner.OutputOnCall(fmt.Sprintf("nsenter --mount=/host_proc/1/ns/mnt -- mount -o rw -t ext4 %s %s", GCE_PD_DEV_PATH, GCE_PD_HOST_MOUNT_PATH), "")

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		GCE_PD_VOLUME_MANIFEST,
		GCE_ATTACHED_RW_DISK_METADATA)

	utils.AssertNoError(t, err)
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	utils.AssertEqual(t, []string{GCE_PD_HOST_MOUNT_PATH + ":/tmp/pd1"}, mockDockerClient.HostConfig.Binds, "")
	utils.AssertEmpty(t, mockDockerClient.HostConfig.Tmpfs, "")
	utils.AssertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	utils.AssertEqual(t, MOCK_EXISTING_CONTAINER_ID, mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultOptions(t)

	mockCommandRunner.AssertAllCalled()
}

func TestExecStartup_pdValidAndFormatted_mountReadOnly(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := command.NewMockRunner(t)

	// Prepare the whole PD command chain.
	mockCommandRunner.RegisterDeviceForStat(GCE_PD_DEV_PATH)
	mockCommandRunner.RegisterMkdirAll(GCE_PD_HOST_MOUNT_PATH)
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts", "")
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o FSTYPE "+GCE_PD_DEV_PATH, "ext4")
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o MOUNTPOINT "+GCE_PD_DEV_PATH, "")
	mockCommandRunner.OutputOnCall("fsck.ext4 -p "+GCE_PD_DEV_PATH, "fsck running running... done!")
	mockCommandRunner.OutputOnCall(fmt.Sprintf("nsenter --mount=/host_proc/1/ns/mnt -- mount -o ro -t ext4 %s %s", GCE_PD_DEV_PATH, GCE_PD_HOST_MOUNT_PATH), "")

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		GCE_PD_VOLUME_WITH_RO_MOUNT_MANIFEST,
		GCE_ATTACHED_RW_DISK_METADATA)

	utils.AssertNoError(t, err)
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	utils.AssertEqual(t, []string{GCE_PD_HOST_MOUNT_PATH + ":/tmp/pd1:ro"}, mockDockerClient.HostConfig.Binds, "")
	utils.AssertEmpty(t, mockDockerClient.HostConfig.Tmpfs, "")
	utils.AssertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	utils.AssertEqual(t, MOCK_EXISTING_CONTAINER_ID, mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultOptions(t)

	mockCommandRunner.AssertAllCalled()
}

func TestExecStartup_pdValidAndFormatted_attachedReadOnly_mountReadOnly(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := command.NewMockRunner(t)

	// Prepare the whole PD command chain.
	mockCommandRunner.RegisterDeviceForStat(GCE_PD_DEV_PATH)
	mockCommandRunner.RegisterMkdirAll(GCE_PD_HOST_MOUNT_PATH)
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts", "")
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o MOUNTPOINT "+GCE_PD_DEV_PATH, "")
	mockCommandRunner.OutputOnCall(fmt.Sprintf("nsenter --mount=/host_proc/1/ns/mnt -- mount -o ro -t ext4 %s %s", GCE_PD_DEV_PATH, GCE_PD_HOST_MOUNT_PATH), "")

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		GCE_PD_VOLUME_WITH_RO_MOUNT_MANIFEST,
		GCE_ATTACHED_RO_DISK_METADATA)

	utils.AssertNoError(t, err)
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	utils.AssertEqual(t, []string{GCE_PD_HOST_MOUNT_PATH + ":/tmp/pd1:ro"}, mockDockerClient.HostConfig.Binds, "")
	utils.AssertEmpty(t, mockDockerClient.HostConfig.Tmpfs, "")
	utils.AssertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	utils.AssertEqual(t, MOCK_EXISTING_CONTAINER_ID, mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultOptions(t)

	mockCommandRunner.AssertAllCalled()
}

func TestExecStartup_pdValidButMetadataNotFound(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := command.NewMockRunner(t)
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts", "")
	// Prepare the whole PD command chain.
	mockCommandRunner.RegisterDeviceForStat(GCE_PD_DEV_PATH)
	mockCommandRunner.RegisterMkdirAll(GCE_PD_HOST_MOUNT_PATH)

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		GCE_PD_VOLUME_MANIFEST,
		SINGLE_DISK_METADATA) // GCE PD not in this metadata.

	utils.AssertError(t, err, "Failed to start container: Volume pd1: Could not determine if the GCE Persistent Disk gce-pd-name-here is attached read-only or read-write.")
}

func TestExecStartup_pdValidAndFormatted_attachedReadOnlyButReadWriteWanted(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := command.NewMockRunner(t)
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts", "")
	// Prepare the whole PD command chain.
	mockCommandRunner.RegisterDeviceForStat(GCE_PD_DEV_PATH)

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		GCE_PD_VOLUME_MANIFEST,
		GCE_ATTACHED_RO_DISK_METADATA)

	utils.AssertError(t, err, "Failed to start container: Volume pd1: Volume mount requires read-write access, but the GCE Persistent Disk gce-pd-name-here is attached read-only.")
}

func TestExecStartup_pdWithPartitionValidAndFormatted(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := command.NewMockRunner(t)

	// Prepare the whole PD command chain.
	devPath := GCE_PD_DEV_PATH + "-part8"
	mockCommandRunner.RegisterDeviceForStat(devPath)
	mockCommandRunner.RegisterMkdirAll(GCE_PD_HOST_MOUNT_PATH)
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts", "")
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o FSTYPE "+devPath, "ext4")
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o MOUNTPOINT "+devPath, "")
	mockCommandRunner.OutputOnCall("fsck.ext4 -p "+devPath, "fsck running running... done!")
	mockCommandRunner.OutputOnCall(fmt.Sprintf("nsenter --mount=/host_proc/1/ns/mnt -- mount -o rw -t ext4 %s %s", devPath, GCE_PD_HOST_MOUNT_PATH), "")

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		GCE_PD_VOLUME_WITH_PARTITION_MANIFEST,
		GCE_ATTACHED_RW_DISK_METADATA)

	utils.AssertNoError(t, err)
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	utils.AssertEqual(t, []string{GCE_PD_HOST_MOUNT_PATH + ":/tmp/pd1"}, mockDockerClient.HostConfig.Binds, "")
	utils.AssertEmpty(t, mockDockerClient.HostConfig.Tmpfs, "")
	utils.AssertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	utils.AssertEqual(t, MOCK_EXISTING_CONTAINER_ID, mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultOptions(t)

	mockCommandRunner.AssertAllCalled()
}

func TestExecStartup_pdNoSuchDevice(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := command.NewMockRunner(t)
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts", "")

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		GCE_PD_VOLUME_MANIFEST,
		GCE_ATTACHED_RW_DISK_METADATA)

	utils.AssertError(t, err, "Failed to start container: Volume pd1: Device /dev/disk/by-id/google-gce-pd-name-here access error: MockRunner.Stat(): No such file or directory: /dev/disk/by-id/google-gce-pd-name-here")
}

func TestExecStartup_pdUnsupportedFilesystem(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := command.NewMockRunner(t)
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts", "")

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		GCE_PD_VOLUME_INVALID_FS_MANIFEST,
		GCE_ATTACHED_RW_DISK_METADATA)
	utils.AssertError(t, err, "Failed to start container: Volume pd1: Unsupported filesystem type: nfts")
}

func TestExecStartup_pdValidAndNotFormatted(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := command.NewMockRunner(t)

	// Prepare the whole PD command chain.
	mockCommandRunner.RegisterDeviceForStat(GCE_PD_DEV_PATH)
	mockCommandRunner.RegisterMkdirAll(GCE_PD_HOST_MOUNT_PATH)
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts", "")
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o FSTYPE "+GCE_PD_DEV_PATH, "")
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o MOUNTPOINT "+GCE_PD_DEV_PATH, "")
	mockCommandRunner.OutputOnCall("mkfs.ext4 "+GCE_PD_DEV_PATH, "omnomnom formatting formatting... done!")
	mockCommandRunner.OutputOnCall(fmt.Sprintf("nsenter --mount=/host_proc/1/ns/mnt -- mount -o rw -t ext4 %s %s", GCE_PD_DEV_PATH, GCE_PD_HOST_MOUNT_PATH), "")

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		GCE_PD_VOLUME_MANIFEST,
		GCE_ATTACHED_RW_DISK_METADATA)

	utils.AssertNoError(t, err)
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	utils.AssertEqual(t, []string{GCE_PD_HOST_MOUNT_PATH + ":/tmp/pd1"}, mockDockerClient.HostConfig.Binds, "")
	utils.AssertEmpty(t, mockDockerClient.HostConfig.Tmpfs, "")
	utils.AssertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	utils.AssertEqual(t, MOCK_EXISTING_CONTAINER_ID, mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultOptions(t)

	mockCommandRunner.AssertAllCalled()
}

func TestExecStartup_pdValidAndNotFormattedButMkfsFails(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := command.NewMockRunner(t)

	// Prepare the whole PD command chain.
	mockCommandRunner.RegisterDeviceForStat(GCE_PD_DEV_PATH)
	mockCommandRunner.RegisterMkdirAll(GCE_PD_HOST_MOUNT_PATH)
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts", "")
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o FSTYPE "+GCE_PD_DEV_PATH, "")
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o MOUNTPOINT "+GCE_PD_DEV_PATH, "")
	mockCommandRunner.ErrorOnCall("mkfs.ext4 "+GCE_PD_DEV_PATH, fmt.Errorf("mkfs enters an infinite loop for a while"))

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		GCE_PD_VOLUME_MANIFEST,
		GCE_ATTACHED_RW_DISK_METADATA)
	utils.AssertError(t, err, "Failed to start container: Volume pd1: Failed to format filesystem: mkfs enters an infinite loop for a while")
}

func TestExecStartup_pdValidButContainsSubpartitions(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := command.NewMockRunner(t)

	// Prepare the whole PD command chain.
	mockCommandRunner.RegisterDeviceForStat(GCE_PD_DEV_PATH)
	mockCommandRunner.RegisterMkdirAll(GCE_PD_HOST_MOUNT_PATH)
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts", "")
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o FSTYPE "+GCE_PD_DEV_PATH, "\next4\n")
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o MOUNTPOINT "+GCE_PD_DEV_PATH, "\n\n")
	// Debug lsblk run:
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk", "debugging data so useful")

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		GCE_PD_VOLUME_MANIFEST,
		GCE_ATTACHED_RW_DISK_METADATA)
	utils.AssertError(t, err, "Failed to start container: Volume pd1: Received multiline output from lsblk. The device likely contains subpartitions:\ndebugging data so useful")
}

func TestExecStartup_pdValidButAlreadyMounted(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := command.NewMockRunner(t)

	// Prepare the whole PD command chain.
	mockCommandRunner.RegisterDeviceForStat(GCE_PD_DEV_PATH)
	mockCommandRunner.RegisterMkdirAll(GCE_PD_HOST_MOUNT_PATH)
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts", "")
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o FSTYPE "+GCE_PD_DEV_PATH, "ext4")
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o MOUNTPOINT "+GCE_PD_DEV_PATH, "/somewhere/over/the/rainbow")

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		GCE_PD_VOLUME_MANIFEST,
		GCE_ATTACHED_RW_DISK_METADATA)

	utils.AssertError(t, err, "Failed to start container: Volume pd1: Device /dev/disk/by-id/google-gce-pd-name-here is already mounted at /somewhere/over/the/rainbow")
}

func TestExecStartup_pdValidButLsblkFails(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := command.NewMockRunner(t)

	// Prepare the whole PD command chain.
	mockCommandRunner.RegisterDeviceForStat(GCE_PD_DEV_PATH)
	mockCommandRunner.RegisterMkdirAll(GCE_PD_HOST_MOUNT_PATH)
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts", "")
	mockCommandRunner.ErrorOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o FSTYPE "+GCE_PD_DEV_PATH, fmt.Errorf("SOMETHING WICKED THIS WAY COMES"))
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o MOUNTPOINT "+GCE_PD_DEV_PATH, "")

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		GCE_PD_VOLUME_MANIFEST,
		GCE_ATTACHED_RW_DISK_METADATA)

	utils.AssertError(t, err, "Failed to start container: Volume pd1: SOMETHING WICKED THIS WAY COMES")
}

func TestExecStartup_pdValidButMountFails(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := command.NewMockRunner(t)

	// Prepare the whole PD command chain.
	mockCommandRunner.RegisterDeviceForStat(GCE_PD_DEV_PATH)
	mockCommandRunner.RegisterMkdirAll(GCE_PD_HOST_MOUNT_PATH)
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts", "")
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o FSTYPE "+GCE_PD_DEV_PATH, "ext4")
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o MOUNTPOINT "+GCE_PD_DEV_PATH, "")
	mockCommandRunner.OutputOnCall("fsck.ext4 -p "+GCE_PD_DEV_PATH, "fsck running running... done!")
	mockCommandRunner.ErrorOnCall(fmt.Sprintf("nsenter --mount=/host_proc/1/ns/mnt -- mount -o rw -t ext4 %s %s", GCE_PD_DEV_PATH, GCE_PD_HOST_MOUNT_PATH), fmt.Errorf("REFUSED TO MOUNT"))

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		GCE_PD_VOLUME_MANIFEST,
		GCE_ATTACHED_RW_DISK_METADATA)
	utils.AssertError(t, err, "Failed to start container: Volume pd1: Failed to mount /dev/disk/by-id/google-gce-pd-name-here at /mnt/disks/gce-containers-mounts/gce-persistent-disks/gce-pd-name-here: REFUSED TO MOUNT")
}

func TestExecStartup_unmountsExistingVolumes(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := command.NewMockRunner(t)
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- cat /proc/mounts",
		"tmpfs /mnt/disks/gce-containers-mounts/tmpfss/volume tmpfs rw,nosuid,nodev 0 0")
	mockCommandRunner.OutputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- umount /mnt/disks/gce-containers-mounts/tmpfss/volume", "")
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		SIMPLE_MANIFEST,
		SINGLE_DISK_METADATA)
	utils.AssertNoError(t, err)
	mockCommandRunner.AssertAllCalled()
}

func (api *MockDockerApi) assertDefaultOptions(t *testing.T) {
	api.assertDefaultSystemOptions(t)
	utils.AssertEqual(t, api.HostConfig.Privileged, false, "")
	utils.AssertEqual(t, api.HostConfig.RestartPolicy.Name, "always", "")
	utils.AssertEqual(t, api.CreateRequest.User, "", "")
	utils.AssertEqual(t, api.CreateRequest.OpenStdin, false, "")
	utils.AssertEqual(t, api.CreateRequest.Tty, false, "")
}

func (api *MockDockerApi) assertDefaultSystemOptions(t *testing.T) {
	utils.AssertEqual(t, api.HostConfig.AutoRemove, false, "")
	utils.AssertEqual(t, api.HostConfig.NetworkMode.IsHost(), true, "")
}
