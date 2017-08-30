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
	"testing"
	utils "github.com/konlet/utils"
	"io"
	"io/ioutil"
	"strings"

	"golang.org/x/net/context"
	dockertypes "github.com/docker/engine-api/types"
	dockercontainer "github.com/docker/engine-api/types/container"
	dockernetwork "github.com/docker/engine-api/types/network"
	dockerstrslice "github.com/docker/engine-api/types/strslice"
	"fmt"
	"reflect"
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
    - name: 'VAR'
      value: 'VAL'`

const VOLUME_MANIFEST = `
spec:
  containers:
  - name: 'test-volume'
    image: 'gcr.io/google-containers/busybox:latest'
    volumeMounts:
    - name: 'vol1'
      mountPath: '/tmp/host-1'
    - name: 'vol2'
      mountPath: '/tmp/host-2'
  volumes:
  - name: 'vol1'
    hostPath:
      path: '/tmp'
  - name: 'vol2'
    emptyDir:
      medium: 'Memory'`

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

const MOCK_AUTH_TOKEN = "123123123="
const MOCK_CONTAINER_ID = "1234567"
const MOCK_EXISTING_CONTAINER_ID = "123123123"

type TestManifestProvider struct {
	Manifest string
}

func (provider TestManifestProvider) RetrieveManifest() ([]byte, error) {
	return []byte(provider.Manifest), nil
}

type MockDockerApi struct {
	PulledImage string
	ContainerName string
	CreateRequest *dockercontainer.Config
	HostConfig *dockercontainer.HostConfig
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
	return dockertypes.ContainerCreateResponse{ID: MOCK_CONTAINER_ID, }, nil
}

func (api *MockDockerApi) ContainerStart(ctx context.Context, container string) error {
	api.StartedContainer = container
	return nil
}

func (api *MockDockerApi) ContainerList(ctx context.Context, opts dockertypes.ContainerListOptions) ([]dockertypes.Container, error) {
	return []dockertypes.Container {
		dockertypes.Container { ID: MOCK_EXISTING_CONTAINER_ID, Names: []string{"/test-remove"} },
	}, nil
}

func (api *MockDockerApi) ContainerRemove(ctx context.Context, containerID string, opts dockertypes.ContainerRemoveOptions) error {
	api.RemovedContainer = containerID
	return nil
}

func TestExecStartup_simple(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	err := ExecStartup(
		TestManifestProvider{Manifest: SIMPLE_MANIFEST, },
		utils.ConstantTokenProvider{Token: MOCK_AUTH_TOKEN, },
		&utils.ContainerRunner{Client: mockDockerClient},
		false /* openIptables */,
	)

	assertNoError(t, err)
	assertEqual(t, "test-simple", mockDockerClient.ContainerName, "")
	assertEqual(t, "gcr.io/gce-containers/apache:v1", mockDockerClient.PulledImage, "")
	assertEqual(t, "gcr.io/gce-containers/apache:v1", mockDockerClient.CreateRequest.Image, "")
	assertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	assertEqual(t, "", mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultOptions(t)
}

func TestExecStartup_runCommand(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	err := ExecStartup(
		TestManifestProvider{Manifest: RUN_COMMAND_MANIFEST, },
		utils.ConstantTokenProvider{Token: MOCK_AUTH_TOKEN, },
		&utils.ContainerRunner{Client: mockDockerClient},
		false /* openIptables */,
	)

	assertNoError(t, err)
	assertEqual(t, "test-run-command", mockDockerClient.ContainerName, "")
	assertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	assertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	assertEqual(t, dockerstrslice.StrSlice([]string{"ls"}), mockDockerClient.CreateRequest.Entrypoint, "")
	assertEqual(t, dockerstrslice.StrSlice([]string{"-l", "/tmp"}), mockDockerClient.CreateRequest.Cmd, "")
	assertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	assertEqual(t, "", mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultOptions(t)
}

func TestExecStartup_runArgs(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	err := ExecStartup(
		TestManifestProvider{Manifest: RUN_ARGS_MANIFEST, },
		utils.ConstantTokenProvider{Token: MOCK_AUTH_TOKEN, },
		&utils.ContainerRunner{Client: mockDockerClient},
		false /* openIptables */,
	)

	assertNoError(t, err)
	assertEqual(t, "test-run-command", mockDockerClient.ContainerName, "")
	assertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	assertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	assertEqual(t, dockerstrslice.StrSlice([]string{"echo"}), mockDockerClient.CreateRequest.Entrypoint, "")
	assertEqual(t, dockerstrslice.StrSlice([]string{"-n", "Hello \" world", "Welco'me"}), mockDockerClient.CreateRequest.Cmd, "")
	assertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	assertEqual(t, "", mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultOptions(t)
}

func TestExecStartup_env(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	err := ExecStartup(
		TestManifestProvider{Manifest: ENVVARS_MANIFEST, },
		utils.ConstantTokenProvider{Token: MOCK_AUTH_TOKEN, },
		&utils.ContainerRunner{Client: mockDockerClient},
		false /* openIptables */,
	)

	assertNoError(t, err)
	assertEqual(t, "test-env-vars", mockDockerClient.ContainerName, "")
	assertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	assertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	assertEqual(t, dockerstrslice.StrSlice([]string{"env"}), mockDockerClient.CreateRequest.Entrypoint, "")
	assertEqual(t, []string{"VAR=VAL"}, mockDockerClient.CreateRequest.Env, "")
	assertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	assertEqual(t, "", mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultOptions(t)
}

func TestExecStartup_volumeMounts(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	err := ExecStartup(
		TestManifestProvider{Manifest: VOLUME_MANIFEST, },
		utils.ConstantTokenProvider{Token: MOCK_AUTH_TOKEN, },
		&utils.ContainerRunner{Client: mockDockerClient},
		false /* openIptables */,
	)

	assertNoError(t, err)
	tmpFsBinds := map[string]string{}
	tmpFsBinds["/tmp/host-2"] = ""
	assertEqual(t, "test-volume", mockDockerClient.ContainerName, "")
	assertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	assertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	assertEqual(t, []string{"/tmp:/tmp/host-1"}, mockDockerClient.HostConfig.Binds, "")
	assertEqual(t, tmpFsBinds, mockDockerClient.HostConfig.Tmpfs, "")
	assertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	assertEqual(t, "", mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultOptions(t)
}

func TestExecStartup_invalidVolumeMounts_multipleTypes(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	err := ExecStartup(
		TestManifestProvider{Manifest: INVALID_VOLUME_MANIFEST_MULTIPLE_TYPES, },
		utils.ConstantTokenProvider{Token: MOCK_AUTH_TOKEN, },
		&utils.ContainerRunner{Client: mockDockerClient},
		false /* openIptables */,
	)

	assertError(t, err, "Failed to start container: Invalid container declaration: Volume can have only one of the properties: hostPath or emptyDir, 2 properties found")
}

func TestExecStartup_invalidVolumeMounts_unmapped(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	err := ExecStartup(
		TestManifestProvider{Manifest: INVALID_VOLUME_MANIFEST_UNMAPPED, },
		utils.ConstantTokenProvider{Token: MOCK_AUTH_TOKEN, },
		&utils.ContainerRunner{Client: mockDockerClient},
		false /* openIptables */,
	)

	assertError(t, err, "Failed to start container: Invalid container declaration: Volume mount referers to undeclared volume with name 'testVolume'")
}

func TestExecStartup_invalidVolumeMounts_unrefererenced(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	err := ExecStartup(
		TestManifestProvider{Manifest: INVALID_VOLUME_MANIFEST_UNREFERENCED, },
		utils.ConstantTokenProvider{Token: MOCK_AUTH_TOKEN, },
		&utils.ContainerRunner{Client: mockDockerClient},
		false /* openIptables */,
	)

	assertNoError(t, err)
}

func TestExecStartup_invalidVolumeMounts_emptydirMedium(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	err := ExecStartup(
		TestManifestProvider{Manifest: INVALID_VOLUME_MANIFEST_EMPTYDIR_MEDIUM, },
		utils.ConstantTokenProvider{Token: MOCK_AUTH_TOKEN, },
		&utils.ContainerRunner{Client: mockDockerClient},
		false /* openIptables */,
	)

	assertError(t, err, "Failed to start container: Invalid container declaration: Unsupported emptyDir volume medium 'Tablet'")
}

func TestExecStartup_options(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	err := ExecStartup(
		TestManifestProvider{Manifest: OPTIONS_MANIFEST, },
		utils.ConstantTokenProvider{Token: MOCK_AUTH_TOKEN, },
		&utils.ContainerRunner{Client: mockDockerClient},
		false /* openIptables */,
	)

	assertNoError(t, err)
	assertEqual(t, "test-options", mockDockerClient.ContainerName, "")
	assertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	assertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	assertEqual(t, dockerstrslice.StrSlice([]string{"sleep"}), mockDockerClient.CreateRequest.Entrypoint, "")
	assertEqual(t, dockerstrslice.StrSlice([]string{"1000"}), mockDockerClient.CreateRequest.Cmd, "")
	assertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	assertEqual(t, mockDockerClient.HostConfig.Privileged, true, "")
	assertEqual(t, mockDockerClient.CreateRequest.StdinOnce, true, "")
	assertEqual(t, mockDockerClient.CreateRequest.Tty, true, "")
	assertEqual(t, "", mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultSystemOptions(t)
}

func TestExecStartup_removeContainer(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	err := ExecStartup(
		TestManifestProvider{Manifest: REMOVE_MANIFEST, },
		utils.ConstantTokenProvider{Token: MOCK_AUTH_TOKEN, },
		&utils.ContainerRunner{Client: mockDockerClient},
		false /* openIptables */,
	)

	assertNoError(t, err)
	assertEqual(t, "test-remove", mockDockerClient.ContainerName, "")
	assertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	assertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	assertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	assertEqual(t, MOCK_EXISTING_CONTAINER_ID, mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultSystemOptions(t)
}

func TestExecStartup_noMultiContainer(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	err := ExecStartup(
		TestManifestProvider{Manifest: MULTICONTAINER_MANIFEST, },
		utils.ConstantTokenProvider{Token: MOCK_AUTH_TOKEN, },
		&utils.ContainerRunner{Client: mockDockerClient},
		false /* openIptables */,
	)

	assertError(t, err, "Container declaration should include exactly 1 container, 2 found")
}

func TestExecStartup_emptyManifest(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	err := ExecStartup(
		TestManifestProvider{Manifest: "", },
		utils.ConstantTokenProvider{Token: MOCK_AUTH_TOKEN, },
		&utils.ContainerRunner{Client: mockDockerClient},
		false /* openIptables */,
	)

	assertError(t, err, "Container declaration should include exactly 1 container, 0 found")
}

func TestExecStartup_restartPolicy(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	err := ExecStartup(
		TestManifestProvider{Manifest: RESTART_POLICY_MANIFEST, },
		utils.ConstantTokenProvider{Token: MOCK_AUTH_TOKEN, },
		&utils.ContainerRunner{Client: mockDockerClient},
		false /* openIptables */,
	)

	assertNoError(t, err)
	assertEqual(t, "test-restart-policy", mockDockerClient.ContainerName, "")
	assertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.PulledImage, "")
	assertEqual(t, "gcr.io/google-containers/busybox:latest", mockDockerClient.CreateRequest.Image, "")
	assertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	assertEqual(t, "", mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultSystemOptions(t)
	assertEqual(t, mockDockerClient.HostConfig.Privileged, false, "")
	assertEqual(t, mockDockerClient.HostConfig.RestartPolicy.Name, "on-failure", "")
	assertEqual(t, mockDockerClient.CreateRequest.User, "", "")
	assertEqual(t, mockDockerClient.CreateRequest.StdinOnce, false, "")
	assertEqual(t, mockDockerClient.CreateRequest.Tty, false, "")
}

func TestExecStartup_invalidRestartPolicy(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	err := ExecStartup(
		TestManifestProvider{Manifest: INVALID_RESTART_POLICY_MANIFEST, },
		utils.ConstantTokenProvider{Token: MOCK_AUTH_TOKEN, },
		&utils.ContainerRunner{Client: mockDockerClient},
		false /* openIptables */,
	)

	assertError(t, err, "Failed to start container: Invalid container declaration: Unsupported container restart policy 'EachSunday'")
}

func TestExecStartup_problem(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	err := ExecStartup(
		TestManifestProvider{Manifest: PROBLEM_MANIFEST, },
		utils.ConstantTokenProvider{Token: MOCK_AUTH_TOKEN, },
		&utils.ContainerRunner{Client: mockDockerClient},
		false /* openIptables */,
	)

	assertNoError(t, err)
	tmpFsBinds := map[string]string{}
	tmpFsBinds["/tmp-tmpfs"] = ""
	assertEqual(t, []string{"/tmp:/tmp-host"}, mockDockerClient.HostConfig.Binds, "")
	assertEqual(t, tmpFsBinds, mockDockerClient.HostConfig.Tmpfs, "")
}

func TestExecStartup_ignorePodFields(t *testing.T) {
	mockDockerClient := &MockDockerApi{}
	err := ExecStartup(
		TestManifestProvider{Manifest: MANIFEST_WITH_IGNORED_POD_FIELDS, },
		utils.ConstantTokenProvider{Token: MOCK_AUTH_TOKEN, },
		&utils.ContainerRunner{Client: mockDockerClient},
		false /* openIptables */,
	)

	assertNoError(t, err)
	assertEqual(t, "test-simple", mockDockerClient.ContainerName, "")
	assertEqual(t, "gcr.io/gce-containers/apache:v1", mockDockerClient.PulledImage, "")
	assertEqual(t, "gcr.io/gce-containers/apache:v1", mockDockerClient.CreateRequest.Image, "")
	assertEqual(t, MOCK_CONTAINER_ID, mockDockerClient.StartedContainer, "")
	assertEqual(t, "", mockDockerClient.RemovedContainer, "")
	mockDockerClient.assertDefaultOptions(t)
}

func assertEqual(t *testing.T, a interface{}, b interface{}, message string) {
	if reflect.DeepEqual(a, b) {
		return
	}
	if len(message) == 0 {
		message = fmt.Sprintf("'%v' != '%v'", a, b)
	}
	t.Fatal(message)
}

func assertNoError(t *testing.T, err error) {
	if (err != nil) {
		message := fmt.Sprintf("%v", err)
		t.Fatalf("Unexpected error '%s'", message)
	}
}

func assertError(t *testing.T, err error, expected string) {
	if (err == nil) {
		t.Fatal("Exected error not to be null")
	}
	message := fmt.Sprintf("%v", err)
	if (message != expected) {
		t.Fatalf("Exected error to be '%s', but it was '%s'", expected, message)
	}
}

func (api *MockDockerApi) assertDefaultOptions(t *testing.T) {
	api.assertDefaultSystemOptions(t)
	assertEqual(t, api.HostConfig.Privileged, false, "")
	assertEqual(t, api.HostConfig.RestartPolicy.Name, "always", "")
	assertEqual(t, api.CreateRequest.User, "", "")
	assertEqual(t, api.CreateRequest.StdinOnce, false, "")
	assertEqual(t, api.CreateRequest.Tty, false, "")
}

func (api *MockDockerApi) assertDefaultSystemOptions(t *testing.T) {
	assertEqual(t, api.HostConfig.AutoRemove, false, "")
	assertEqual(t, api.HostConfig.NetworkMode.IsHost(), true, "")
}
