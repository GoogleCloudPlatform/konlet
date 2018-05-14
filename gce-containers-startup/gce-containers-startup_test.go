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
	"os"
	"strings"
	"testing"
	"time"

	dockertypes "github.com/docker/engine-api/types"
	dockercontainer "github.com/docker/engine-api/types/container"
	dockernetwork "github.com/docker/engine-api/types/network"
	dockerstrslice "github.com/docker/engine-api/types/strslice"
	utils "github.com/konlet/utils"

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

const INVALID_VOLUME_MANIFEST_MULTIPLE_TYPES = `
spec:
  containers:
  - name: 'test-volume'
    image: 'gcr.io/google-containers/busybox:latest'
    volumeMounts:
    - name: 'testVolume'
      mountPath: '/tmp/host'
  volumes:
  - name: 'testVolume'
    hostPath:
      path: '/tmp'
    emptyDir:
      medium: 'Memory'`

const INVALID_VOLUME_MANIFEST_UNMAPPED = `
spec:
  containers:
  - name: 'test-volume'
    image: 'gcr.io/google-containers/busybox:latest'
    volumeMounts:
    - name: 'testVolume'
      mountPath: '/tmp/host'`

const INVALID_VOLUME_MANIFEST_UNREFERENCED = `
spec:
  containers:
  - name: 'test-volume'
    image: 'gcr.io/google-containers/busybox:latest'
  volumes:
  - name: 'testVolume'
    emptyDir:
      medium: 'Memory'`

const INVALID_VOLUME_MANIFEST_EMPTYDIR_MEDIUM = `
spec:
  containers:
  - name: 'test-volume'
    image: 'gcr.io/google-containers/busybox:latest'
    command: ['ls']
    args: ["/tmp/host"]
    volumeMounts:
    - name: 'testVolume'
      mountPath: '/tmp/host'
  volumes:
  - name: 'testVolume'
    emptyDir:
      medium: 'Tablet'`

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

type TestMetadataProvider struct {
	Manifest         string
	DiskMetadataJson string
}

func (provider TestMetadataProvider) RetrieveManifest() ([]byte, error) {
	return []byte(provider.Manifest), nil
}

func (provider TestMetadataProvider) RetrieveDisksMetadataAsJson() ([]byte, error) {
	return []byte(provider.DiskMetadataJson), nil
}

type MockDockerApi struct {
	PulledImage      string
	ContainerName    string
	CreateRequest    *dockercontainer.Config
	HostConfig       *dockercontainer.HostConfig
	StartedContainer string
	RemovedContainer string
}

type MockCommand struct {
	callCount      int
	returnedOutput string
	returnedError  error
}

type MockCommandRunner struct {
	commands          map[string]*MockCommand
	statFiles         map[string]os.FileInfo
	expectedMkdirAlls map[string]bool
	t                 *testing.T
}

type minimalFileInfo struct {
	name     string
	size     int64
	fileMode os.FileMode
	modTime  time.Time
}

func (f minimalFileInfo) Name() string {
	return f.name
}

func (f minimalFileInfo) Size() int64 {
	return f.size
}

func (f minimalFileInfo) Mode() os.FileMode {
	return f.fileMode
}

func (f minimalFileInfo) ModTime() time.Time {
	return f.modTime
}

func (f minimalFileInfo) IsDir() bool {
	return f.fileMode.IsDir()
}

func (f minimalFileInfo) Sys() interface{} {
	return nil
}

func NewMockCommandRunner(t *testing.T) *MockCommandRunner {
	return &MockCommandRunner{commands: map[string]*MockCommand{}, statFiles: map[string]os.FileInfo{}, expectedMkdirAlls: map[string]bool{}, t: t}
}

func (m *MockCommandRunner) Run(commandAndArgs ...string) (string, error) {
	commandString := strings.Join(commandAndArgs, " ")
	if _, found := m.commands[commandString]; !found {
		m.t.Fatal(fmt.Sprintf("Unexpected os command called: %s", commandString))
	}
	m.commands[commandString].callCount += 1
	return m.commands[commandString].returnedOutput, m.commands[commandString].returnedError
}

func (m *MockCommandRunner) MkdirAll(path string, perm os.FileMode) error {
	if _, found := m.expectedMkdirAlls[path]; found {
		m.expectedMkdirAlls[path] = true
		return nil
	} else {
		return fmt.Errorf("MkdirAll() called on unexpected path: %s", path)
	}
}

func (m *MockCommandRunner) Stat(path string) (os.FileInfo, error) {
	if fileInfo, found := m.statFiles[path]; found {
		return fileInfo, nil
	} else {
		return minimalFileInfo{}, fmt.Errorf("MockCommandRunner.Stat(): No such file or directory: %s", path)
	}
}

func (m *MockCommandRunner) outputOnCall(commandAndArgs string, output string) {
	m.commands[commandAndArgs] = &MockCommand{callCount: 0, returnedOutput: output, returnedError: nil}
}

func (m *MockCommandRunner) errorOnCall(commandAndArgs string, err error) {
	m.commands[commandAndArgs] = &MockCommand{callCount: 0, returnedOutput: "", returnedError: err}
}

func (m *MockCommandRunner) registerMkdirAll(path string) {
	m.expectedMkdirAlls[path] = false
}

func (m *MockCommandRunner) registerDeviceForStat(path string) {
	m.statFiles[path] = minimalFileInfo{name: path, fileMode: os.ModeDevice}
}

func (m *MockCommandRunner) registerDirectoryForStat(path string) {
	m.statFiles[path] = minimalFileInfo{name: path, fileMode: os.ModeDir}
}

func (m *MockCommandRunner) assertCalled(commandAndArgs string) {
	command, found := m.commands[commandAndArgs]
	if !found || command.callCount == 0 {
		m.t.Fatal(fmt.Sprintf("Expected os command not called: %s", commandAndArgs))
	}
}

func (m *MockCommandRunner) assertAllCalled() {
	for commandAndArgs, command := range m.commands {
		if command.callCount == 0 {
			m.t.Fatal(fmt.Sprintf("Expected os command not called: %s", commandAndArgs))
		}
	}
	for mkdirAllPath, called := range m.expectedMkdirAlls {
		if !called {
			m.t.Fatal(fmt.Sprintf("Expected os.MkdirAll() not called: %s", mkdirAllPath))
		}
	}
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
		dockertypes.Container{ID: MOCK_EXISTING_CONTAINER_ID, Names: []string{"/test-remove"}},
	}, nil
}

func (api *MockDockerApi) ContainerRemove(ctx context.Context, containerID string, opts dockertypes.ContainerRemoveOptions) error {
	api.RemovedContainer = containerID
	return nil
}

func ExecStartupWithMocksAndFakes(mockDockerApi *MockDockerApi, mockCommandRunner *MockCommandRunner, manifest string, diskMetadata string) error {
	fakeMetadataProvider := TestMetadataProvider{Manifest: manifest, DiskMetadataJson: diskMetadata}
	volumesEnv := &utils.VolumesModuleEnv{OsCommandRunner: mockCommandRunner, MetadataProvider: fakeMetadataProvider}
	return ExecStartup(
		fakeMetadataProvider,
		utils.ConstantTokenProvider{Token: MOCK_AUTH_TOKEN},
		&utils.ContainerRunner{Client: mockDockerApi, VolumesEnv: volumesEnv},
		false, /* openIptables */
	)
}

func TestExecStartup_simple(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		SIMPLE_MANIFEST,
		SINGLE_DISK_METADATA)

	utils.AssertNoError(t, err)
	utils.AssertEqual(t, "test-simple", mockDockerClient.ContainerName, "")
	utils.AssertEqual(t, "gcr.io/gce-containers/apache:v1", mockDockerClient.PulledImage, "")
	utils.AssertEqual(t, "gcr.io/gce-containers/apache:v1", mockDockerClient.CreateRequest.Image, "")
	utils.AssertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	utils.AssertEqual(t, "", mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultOptions(t)
}

func TestExecStartup_runCommand(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		RUN_COMMAND_MANIFEST,
		SINGLE_DISK_METADATA)

	utils.AssertNoError(t, err)
	utils.AssertEqual(t, "test-run-command", mockDockerClient.ContainerName, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	utils.AssertEqual(t, dockerstrslice.StrSlice([]string{"ls"}), mockDockerClient.CreateRequest.Entrypoint, "")
	utils.AssertEqual(t, dockerstrslice.StrSlice([]string{"-l", "/tmp"}), mockDockerClient.CreateRequest.Cmd, "")
	utils.AssertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	utils.AssertEqual(t, "", mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultOptions(t)
}

func TestExecStartup_runArgs(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		RUN_ARGS_MANIFEST,
		SINGLE_DISK_METADATA)

	utils.AssertNoError(t, err)
	utils.AssertEqual(t, "test-run-command", mockDockerClient.ContainerName, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	utils.AssertEqual(t, dockerstrslice.StrSlice([]string{"echo"}), mockDockerClient.CreateRequest.Entrypoint, "")
	utils.AssertEqual(t, dockerstrslice.StrSlice([]string{"-n", "Hello \" world", "Welco'me"}), mockDockerClient.CreateRequest.Cmd, "")
	utils.AssertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	utils.AssertEqual(t, "", mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultOptions(t)
}

func TestExecStartup_env(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		ENVVARS_MANIFEST,
		SINGLE_DISK_METADATA)

	utils.AssertNoError(t, err)
	utils.AssertEqual(t, "test-env-vars", mockDockerClient.ContainerName, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	utils.AssertEqual(t, dockerstrslice.StrSlice([]string{"env"}), mockDockerClient.CreateRequest.Entrypoint, "")
	utils.AssertEqual(t, []string{"VAR1=VAL1", "VAR2=VAL2"}, mockDockerClient.CreateRequest.Env, "")
	utils.AssertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	utils.AssertEqual(t, "", mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultOptions(t)
}

func TestExecStartup_volumeMounts(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		VOLUME_MANIFEST,
		SINGLE_DISK_METADATA)

	utils.AssertNoError(t, err)
	tmpFsBinds := map[string]string{}
	tmpFsBinds["/tmp/host-2"] = ""
	utils.AssertEqual(t, "test-volume", mockDockerClient.ContainerName, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	utils.AssertEqual(t, []string{"/tmp:/tmp/host-1:ro", "/tmp:/tmp/host-3", "/tmp:/tmp/host-4"}, mockDockerClient.HostConfig.Binds, "")
	utils.AssertEqual(t, tmpFsBinds, mockDockerClient.HostConfig.Tmpfs, "")
	utils.AssertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	utils.AssertEqual(t, "", mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultOptions(t)
}

func TestExecStartup_invalidVolumeMounts_multipleTypes(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		INVALID_VOLUME_MANIFEST_MULTIPLE_TYPES,
		SINGLE_DISK_METADATA)

	utils.AssertError(t, err, "Failed to start container: Invalid container declaration: Exactly one volume specification required for volume testVolume, 2 found.")
}

func TestExecStartup_invalidVolumeMounts_unmapped(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		INVALID_VOLUME_MANIFEST_UNMAPPED,
		SINGLE_DISK_METADATA)

	utils.AssertError(t, err, "Failed to start container: Invalid container declaration: Volume testVolume referenced in container test-volume (index: 0) not found in volume definitions.")
}

func TestExecStartup_invalidVolumeMounts_unrefererenced(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		INVALID_VOLUME_MANIFEST_UNREFERENCED,
		SINGLE_DISK_METADATA)

	utils.AssertError(t, err, "Failed to start container: Invalid container declaration: Volume testVolume not referenced by any container.")
}

func TestExecStartup_invalidVolumeMounts_emptydirMedium(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		INVALID_VOLUME_MANIFEST_EMPTYDIR_MEDIUM,
		SINGLE_DISK_METADATA)

	utils.AssertError(t, err, "Failed to start container: Volume testVolume: Unsupported emptyDir volume medium: Tablet")
}

func TestExecStartup_options(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		OPTIONS_MANIFEST,
		SINGLE_DISK_METADATA)

	utils.AssertNoError(t, err)
	utils.AssertEqual(t, "test-options", mockDockerClient.ContainerName, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	utils.AssertEqual(t, dockerstrslice.StrSlice([]string{"sleep"}), mockDockerClient.CreateRequest.Entrypoint, "")
	utils.AssertEqual(t, dockerstrslice.StrSlice([]string{"1000"}), mockDockerClient.CreateRequest.Cmd, "")
	utils.AssertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	utils.AssertEqual(t, mockDockerClient.HostConfig.Privileged, true, "")
	utils.AssertEqual(t, mockDockerClient.CreateRequest.OpenStdin, true, "")
	utils.AssertEqual(t, mockDockerClient.CreateRequest.Tty, true, "")
	utils.AssertEqual(t, "", mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultSystemOptions(t)
}

func TestExecStartup_removeContainer(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		REMOVE_MANIFEST,
		SINGLE_DISK_METADATA)

	utils.AssertNoError(t, err)
	utils.AssertEqual(t, "test-remove", mockDockerClient.ContainerName, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	utils.AssertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	utils.AssertEqual(t, MOCK_EXISTING_CONTAINER_ID, mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultSystemOptions(t)
}

func TestExecStartup_noMultiContainer(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		MULTICONTAINER_MANIFEST,
		SINGLE_DISK_METADATA)

	utils.AssertError(t, err, "Container declaration should include exactly 1 container, 2 found")
}

func TestExecStartup_emptyManifest(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		"",
		SINGLE_DISK_METADATA)

	utils.AssertError(t, err, "Container declaration should include exactly 1 container, 0 found")
}

func TestExecStartup_restartPolicy(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		RESTART_POLICY_MANIFEST,
		SINGLE_DISK_METADATA)

	utils.AssertNoError(t, err)
	utils.AssertEqual(t, "test-restart-policy", mockDockerClient.ContainerName, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	utils.AssertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	utils.AssertEqual(t, "", mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultSystemOptions(t)
	utils.AssertEqual(t, mockDockerClient.HostConfig.Privileged, false, "")
	utils.AssertEqual(t, mockDockerClient.HostConfig.RestartPolicy.Name, "on-failure", "")
	utils.AssertEqual(t, mockDockerClient.CreateRequest.User, "", "")
	utils.AssertEqual(t, mockDockerClient.CreateRequest.OpenStdin, false, "")
	utils.AssertEqual(t, mockDockerClient.CreateRequest.Tty, false, "")
}

func TestExecStartup_invalidRestartPolicy(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		INVALID_RESTART_POLICY_MANIFEST,
		SINGLE_DISK_METADATA)

	utils.AssertError(t, err, "Failed to start container: Invalid container declaration: Unsupported container restart policy 'EachSunday'")
}

func TestExecStartup_problem(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		PROBLEM_MANIFEST,
		SINGLE_DISK_METADATA)

	utils.AssertNoError(t, err)
	tmpFsBinds := map[string]string{}
	tmpFsBinds["/tmp-tmpfs"] = ""
	utils.AssertEqual(t, []string{"/tmp:/tmp-host"}, mockDockerClient.HostConfig.Binds, "")
	utils.AssertEqual(t, tmpFsBinds, mockDockerClient.HostConfig.Tmpfs, "")
}

func TestExecStartup_ignorePodFields(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)
	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		MANIFEST_WITH_IGNORED_POD_FIELDS,
		SINGLE_DISK_METADATA)

	utils.AssertNoError(t, err)
	utils.AssertEqual(t, "test-simple", mockDockerClient.ContainerName, "")
	utils.AssertEqual(t, "gcr.io/gce-containers/apache:v1", mockDockerClient.PulledImage, "")
	utils.AssertEqual(t, "gcr.io/gce-containers/apache:v1", mockDockerClient.CreateRequest.Image, "")
	utils.AssertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	utils.AssertEqual(t, "", mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultOptions(t)
}

func TestExecStartup_pdValidAndFormatted(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)

	// Prepare the whole PD command chain.
	mockCommandRunner.registerDeviceForStat(GCE_PD_DEV_PATH)
	mockCommandRunner.registerMkdirAll(GCE_PD_HOST_MOUNT_PATH)
	mockCommandRunner.outputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o FSTYPE "+GCE_PD_DEV_PATH, "ext4")
	mockCommandRunner.outputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o MOUNTPOINT "+GCE_PD_DEV_PATH, "")
	mockCommandRunner.outputOnCall("fsck.ext4 -p "+GCE_PD_DEV_PATH, "fsck running running... done!")
	mockCommandRunner.outputOnCall(fmt.Sprintf("nsenter --mount=/host_proc/1/ns/mnt -- mount -o rw -t ext4 %s %s", GCE_PD_DEV_PATH, GCE_PD_HOST_MOUNT_PATH), "")

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		GCE_PD_VOLUME_MANIFEST,
		GCE_ATTACHED_RW_DISK_METADATA)

	utils.AssertNoError(t, err)
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	utils.AssertEqual(t, []string{GCE_PD_HOST_MOUNT_PATH + ":/tmp/pd1"}, mockDockerClient.HostConfig.Binds, "")
	utils.AssertEqual(t, map[string]string{}, mockDockerClient.HostConfig.Tmpfs, "")
	utils.AssertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	utils.AssertEqual(t, "", mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultOptions(t)

	mockCommandRunner.assertAllCalled()
}

func TestExecStartup_pdValidAndFormatted_mountReadOnly(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)

	// Prepare the whole PD command chain.
	mockCommandRunner.registerDeviceForStat(GCE_PD_DEV_PATH)
	mockCommandRunner.registerMkdirAll(GCE_PD_HOST_MOUNT_PATH)
	mockCommandRunner.outputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o FSTYPE "+GCE_PD_DEV_PATH, "ext4")
	mockCommandRunner.outputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o MOUNTPOINT "+GCE_PD_DEV_PATH, "")
	mockCommandRunner.outputOnCall("fsck.ext4 -p "+GCE_PD_DEV_PATH, "fsck running running... done!")
	mockCommandRunner.outputOnCall(fmt.Sprintf("nsenter --mount=/host_proc/1/ns/mnt -- mount -o ro -t ext4 %s %s", GCE_PD_DEV_PATH, GCE_PD_HOST_MOUNT_PATH), "")

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		GCE_PD_VOLUME_WITH_RO_MOUNT_MANIFEST,
		GCE_ATTACHED_RW_DISK_METADATA)

	utils.AssertNoError(t, err)
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	utils.AssertEqual(t, []string{GCE_PD_HOST_MOUNT_PATH + ":/tmp/pd1:ro"}, mockDockerClient.HostConfig.Binds, "")
	utils.AssertEqual(t, map[string]string{}, mockDockerClient.HostConfig.Tmpfs, "")
	utils.AssertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	utils.AssertEqual(t, "", mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultOptions(t)

	mockCommandRunner.assertAllCalled()
}

func TestExecStartup_pdValidAndFormatted_attachedReadOnly_mountReadOnly(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)

	// Prepare the whole PD command chain.
	mockCommandRunner.registerDeviceForStat(GCE_PD_DEV_PATH)
	mockCommandRunner.registerMkdirAll(GCE_PD_HOST_MOUNT_PATH)
	mockCommandRunner.outputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o MOUNTPOINT "+GCE_PD_DEV_PATH, "")
	mockCommandRunner.outputOnCall(fmt.Sprintf("nsenter --mount=/host_proc/1/ns/mnt -- mount -o ro -t ext4 %s %s", GCE_PD_DEV_PATH, GCE_PD_HOST_MOUNT_PATH), "")

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		GCE_PD_VOLUME_WITH_RO_MOUNT_MANIFEST,
		GCE_ATTACHED_RO_DISK_METADATA)

	utils.AssertNoError(t, err)
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	utils.AssertEqual(t, []string{GCE_PD_HOST_MOUNT_PATH + ":/tmp/pd1:ro"}, mockDockerClient.HostConfig.Binds, "")
	utils.AssertEqual(t, map[string]string{}, mockDockerClient.HostConfig.Tmpfs, "")
	utils.AssertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	utils.AssertEqual(t, "", mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultOptions(t)

	mockCommandRunner.assertAllCalled()
}

func TestExecStartup_pdValidButMetadataNotFound(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)

	// Prepare the whole PD command chain.
	mockCommandRunner.registerDeviceForStat(GCE_PD_DEV_PATH)
	mockCommandRunner.registerMkdirAll(GCE_PD_HOST_MOUNT_PATH)

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		GCE_PD_VOLUME_MANIFEST,
		SINGLE_DISK_METADATA) // GCE PD not in this metadata.

	utils.AssertError(t, err, "Failed to start container: Volume pd1: Could not determine if the GCE Persistent Disk gce-pd-name-here is attached read-only or read-write.")
}

func TestExecStartup_pdValidAndFormatted_attachedReadOnlyButReadWriteWanted(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)

	// Prepare the whole PD command chain.
	mockCommandRunner.registerDeviceForStat(GCE_PD_DEV_PATH)

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		GCE_PD_VOLUME_MANIFEST,
		GCE_ATTACHED_RO_DISK_METADATA)

	utils.AssertError(t, err, "Failed to start container: Volume pd1: Volume mount requires read-write access, but the GCE Persistent Disk gce-pd-name-here is attached read-only.")
}

func TestExecStartup_pdWithPartitionValidAndFormatted(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)

	// Prepare the whole PD command chain.
	devPath := GCE_PD_DEV_PATH + "-part8"
	mockCommandRunner.registerDeviceForStat(devPath)
	mockCommandRunner.registerMkdirAll(GCE_PD_HOST_MOUNT_PATH)
	mockCommandRunner.outputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o FSTYPE "+devPath, "ext4")
	mockCommandRunner.outputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o MOUNTPOINT "+devPath, "")
	mockCommandRunner.outputOnCall("fsck.ext4 -p "+devPath, "fsck running running... done!")
	mockCommandRunner.outputOnCall(fmt.Sprintf("nsenter --mount=/host_proc/1/ns/mnt -- mount -o rw -t ext4 %s %s", devPath, GCE_PD_HOST_MOUNT_PATH), "")

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		GCE_PD_VOLUME_WITH_PARTITION_MANIFEST,
		GCE_ATTACHED_RW_DISK_METADATA)

	utils.AssertNoError(t, err)
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	utils.AssertEqual(t, []string{GCE_PD_HOST_MOUNT_PATH + ":/tmp/pd1"}, mockDockerClient.HostConfig.Binds, "")
	utils.AssertEqual(t, map[string]string{}, mockDockerClient.HostConfig.Tmpfs, "")
	utils.AssertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	utils.AssertEqual(t, "", mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultOptions(t)

	mockCommandRunner.assertAllCalled()
}

func TestExecStartup_pdNoSuchDevice(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		GCE_PD_VOLUME_MANIFEST,
		GCE_ATTACHED_RW_DISK_METADATA)

	utils.AssertError(t, err, "Failed to start container: Volume pd1: Device /dev/disk/by-id/google-gce-pd-name-here access error: MockCommandRunner.Stat(): No such file or directory: /dev/disk/by-id/google-gce-pd-name-here")
}

func TestExecStartup_pdUnsupportedFilesystem(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		GCE_PD_VOLUME_INVALID_FS_MANIFEST,
		GCE_ATTACHED_RW_DISK_METADATA)
	utils.AssertError(t, err, "Failed to start container: Volume pd1: Unsupported filesystem type: nfts")
}

func TestExecStartup_pdValidAndNotFormatted(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)

	// Prepare the whole PD command chain.
	mockCommandRunner.registerDeviceForStat(GCE_PD_DEV_PATH)
	mockCommandRunner.registerMkdirAll(GCE_PD_HOST_MOUNT_PATH)
	mockCommandRunner.outputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o FSTYPE "+GCE_PD_DEV_PATH, "")
	mockCommandRunner.outputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o MOUNTPOINT "+GCE_PD_DEV_PATH, "")
	mockCommandRunner.outputOnCall("mkfs.ext4 "+GCE_PD_DEV_PATH, "omnomnom formatting formatting... done!")
	mockCommandRunner.outputOnCall(fmt.Sprintf("nsenter --mount=/host_proc/1/ns/mnt -- mount -o rw -t ext4 %s %s", GCE_PD_DEV_PATH, GCE_PD_HOST_MOUNT_PATH), "")

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		GCE_PD_VOLUME_MANIFEST,
		GCE_ATTACHED_RW_DISK_METADATA)

	utils.AssertNoError(t, err)
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	utils.AssertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	utils.AssertEqual(t, []string{GCE_PD_HOST_MOUNT_PATH + ":/tmp/pd1"}, mockDockerClient.HostConfig.Binds, "")
	utils.AssertEqual(t, map[string]string{}, mockDockerClient.HostConfig.Tmpfs, "")
	utils.AssertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	utils.AssertEqual(t, "", mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultOptions(t)

	mockCommandRunner.assertAllCalled()
}

func TestExecStartup_pdValidAndNotFormattedButMkfsFails(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)

	// Prepare the whole PD command chain.
	mockCommandRunner.registerDeviceForStat(GCE_PD_DEV_PATH)
	mockCommandRunner.registerMkdirAll(GCE_PD_HOST_MOUNT_PATH)
	mockCommandRunner.outputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o FSTYPE "+GCE_PD_DEV_PATH, "")
	mockCommandRunner.outputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o MOUNTPOINT "+GCE_PD_DEV_PATH, "")
	mockCommandRunner.errorOnCall("mkfs.ext4 "+GCE_PD_DEV_PATH, fmt.Errorf("mkfs enters an infinite loop for a while"))

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		GCE_PD_VOLUME_MANIFEST,
		GCE_ATTACHED_RW_DISK_METADATA)
	utils.AssertError(t, err, "Failed to start container: Volume pd1: Failed to format filesystem: mkfs enters an infinite loop for a while")
}

func TestExecStartup_pdValidButAlreadyMounted(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)

	// Prepare the whole PD command chain.
	mockCommandRunner.registerDeviceForStat(GCE_PD_DEV_PATH)
	mockCommandRunner.registerMkdirAll(GCE_PD_HOST_MOUNT_PATH)
	mockCommandRunner.outputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o FSTYPE "+GCE_PD_DEV_PATH, "ext4")
	mockCommandRunner.outputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o MOUNTPOINT "+GCE_PD_DEV_PATH, "/somewhere/over/the/rainbow")

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		GCE_PD_VOLUME_MANIFEST,
		GCE_ATTACHED_RW_DISK_METADATA)

	utils.AssertError(t, err, "Failed to start container: Volume pd1: Device /dev/disk/by-id/google-gce-pd-name-here is already mounted at /somewhere/over/the/rainbow")
}

func TestExecStartup_pdValidButLsblkFails(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)

	// Prepare the whole PD command chain.
	mockCommandRunner.registerDeviceForStat(GCE_PD_DEV_PATH)
	mockCommandRunner.registerMkdirAll(GCE_PD_HOST_MOUNT_PATH)
	mockCommandRunner.errorOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o FSTYPE "+GCE_PD_DEV_PATH, fmt.Errorf("SOMETHING WICKED THIS WAY COMES"))
	mockCommandRunner.outputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o MOUNTPOINT "+GCE_PD_DEV_PATH, "")

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		GCE_PD_VOLUME_MANIFEST,
		GCE_ATTACHED_RW_DISK_METADATA)

	utils.AssertError(t, err, "Failed to start container: Volume pd1: SOMETHING WICKED THIS WAY COMES")
}

func TestExecStartup_pdValidButMountFails(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	mockCommandRunner := NewMockCommandRunner(t)

	// Prepare the whole PD command chain.
	mockCommandRunner.registerDeviceForStat(GCE_PD_DEV_PATH)
	mockCommandRunner.registerMkdirAll(GCE_PD_HOST_MOUNT_PATH)
	mockCommandRunner.outputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o FSTYPE "+GCE_PD_DEV_PATH, "ext4")
	mockCommandRunner.outputOnCall("nsenter --mount=/host_proc/1/ns/mnt -- lsblk -n -o MOUNTPOINT "+GCE_PD_DEV_PATH, "")
	mockCommandRunner.outputOnCall("fsck.ext4 -p "+GCE_PD_DEV_PATH, "fsck running running... done!")
	mockCommandRunner.errorOnCall(fmt.Sprintf("nsenter --mount=/host_proc/1/ns/mnt -- mount -o rw -t ext4 %s %s", GCE_PD_DEV_PATH, GCE_PD_HOST_MOUNT_PATH), fmt.Errorf("REFUSED TO MOUNT"))

	err := ExecStartupWithMocksAndFakes(
		mockDockerClient,
		mockCommandRunner,
		GCE_PD_VOLUME_MANIFEST,
		GCE_ATTACHED_RW_DISK_METADATA)
	utils.AssertError(t, err, "Failed to start container: Volume pd1: Failed to mount /dev/disk/by-id/google-gce-pd-name-here at /mnt/disks/gce-containers-mounts/gce-persistent-disks/gce-pd-name-here: REFUSED TO MOUNT")
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
