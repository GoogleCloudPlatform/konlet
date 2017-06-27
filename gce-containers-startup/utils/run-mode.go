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

package utils

import (
	"fmt"
	"io/ioutil"
	"flag"
	"golang.org/x/net/context"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"log"
	"strings"

	dockertypes "github.com/docker/engine-api/types"
	dockercontainer "github.com/docker/engine-api/types/container"
	dockernetwork "github.com/docker/engine-api/types/network"
	dockerapi "github.com/docker/engine-api/client"
	dockerstrslice "github.com/docker/engine-api/types/strslice"

	api "github.com/konlet/types"
	"io"
)

const DOCKER_UNIX_SOCKET = "unix:///var/run/docker.sock"

var (
	gcploggingFlag = flag.Bool("gcp-logging", true, "whether to configure GCP Logging")
)

// operationTimeout is the error returned when the docker operations are timeout.
type operationTimeout struct {
	err error
	operationType string
}

type DockerApiClient interface {
	ImagePull(ctx context.Context, ref string, options dockertypes.ImagePullOptions) (io.ReadCloser, error)
	ContainerCreate(ctx context.Context, config *dockercontainer.Config, hostConfig *dockercontainer.HostConfig, networkingConfig *dockernetwork.NetworkingConfig, containerName string) (dockertypes.ContainerCreateResponse, error)
	ContainerStart(ctx context.Context, container string) error
	ContainerList(ctx context.Context, opts dockertypes.ContainerListOptions) ([]dockertypes.Container, error)
	ContainerRemove(ctx context.Context, containerID string, opts dockertypes.ContainerRemoveOptions) error
}

func (e operationTimeout) Error() string {
	return fmt.Sprintf("%s operation timeout: %v", e.operationType, e.err)
}

type ContainerRunner struct {
	Client DockerApiClient
}

func GetDefaultRunner() (*ContainerRunner, error) {
	var dockerClient DockerApiClient
	var err error
	dockerClient, err = dockerapi.NewClient(DOCKER_UNIX_SOCKET, "", nil, nil)
	if err != nil {
		return nil, err
	}
	return &ContainerRunner{Client: dockerClient,}, nil
}

func (runner ContainerRunner) RunContainer(auth string, spec api.ContainerSpecStruct, detach bool) error {
	err := pullImage(runner.Client, auth, spec.Containers[0])
	if err != nil {
		return err
	}

	err = deleteOldContainer(runner.Client, spec.Containers[0])
	if err != nil {
		return err
	}

	var id string
	id, err = createContainer(runner.Client, spec)
	if err != nil {
		return err
	}

	err = startContainer(runner.Client, id)
	if err != nil {
		return err
	}

	return nil
}

func pullImage(dockerClient DockerApiClient, auth string, spec api.Container) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	authStruct := dockertypes.AuthConfig{
		Username: "_token",
		Password: auth,
	}
	base64Auth, err := base64EncodeAuth(authStruct)
	if err != nil {
		return err
	}

	opts := dockertypes.ImagePullOptions{}
	opts.RegistryAuth = base64Auth

	log.Printf("Pulling image: '%s'", spec.Image)
	resp, err := dockerClient.ImagePull(ctx, spec.Image, opts)
	if err != nil {
		return err
	}
	defer resp.Close()

	body, err := ioutil.ReadAll(resp)
	if err != nil {
		return err
	}
	log.Printf("Received ImagePull response: (%s).\n", body)

	return nil
}

func findIdForName(containers []dockertypes.Container, containerName string) (string, bool) {
	var searchName = "/" + containerName
	for _,container := range containers {
		for _,name := range container.Names {
			if name == searchName {
				return container.ID, true
			}
		}
	}
	return "", false
}

func deleteOldContainer(dockerClient DockerApiClient, spec api.Container) error {
	var containerName = spec.Name;
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	listOpts := dockertypes.ContainerListOptions{All: true}
	resp, err := dockerClient.ContainerList(ctx, listOpts)

	if err != nil {
		return err
	}

	containerID, exists := findIdForName(resp, containerName)
	if !exists {
		log.Printf("Container with name '%s' has not yet been run.\n", containerName)
		return nil
	}

	log.Printf("Removing previous container '%s' (ID: %s)\n", containerName, containerID)
	rmOpts := dockertypes.ContainerRemoveOptions{
		Force: true,
	}
	return dockerClient.ContainerRemove(ctx, containerID, rmOpts)
}

func createContainer(dockerClient DockerApiClient, spec api.ContainerSpecStruct) (string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	container := spec.Containers[0]
	var runCommand dockerstrslice.StrSlice
	if container.Command != nil {
		runCommand = dockerstrslice.StrSlice(container.Command)
	}

	var runArgs dockerstrslice.StrSlice
	if container.Args != nil {
		runArgs = dockerstrslice.StrSlice(container.Args)
	}

	hostPathBinds := []string{}
	tmpFsBinds := map[string]string{}

	volumesMap := map[string]api.Volume{}
	for _, apiVolume := range spec.Volumes {
		volumesMap[apiVolume.Name] = apiVolume
	}

	volumeMountMap := map[string]int{}
	for _, apiVolumeMount := range container.VolumeMounts {
		volumeMountMap[apiVolumeMount.Name] = 0
	}

	log.Printf("Found %d volume mounts in container declaration", len(container.VolumeMounts))
	for _, apiVolumeMount := range container.VolumeMounts {
		volumesInSpec := 0
		apiVolume, volumeExists := volumesMap[apiVolumeMount.Name]

		if (!volumeExists) {
			return "", fmt.Errorf(
				"Invalid container declaration: Volume mount referers to undeclared volume with name '%s'", apiVolumeMount.Name)
		}

		if apiVolume.HostPath != nil {
			volumesInSpec++
			hostPathBind := fmt.Sprintf("%s:%s", apiVolume.HostPath.Path, apiVolumeMount.MountPath)
			hostPathBinds = append(hostPathBinds, hostPathBind)
		}
		if apiVolume.EmptyDir != nil {
			volumesInSpec++
			tmpFsOpts := []string{}

			if (apiVolume.EmptyDir.Medium != "Memory") {
				return "", fmt.Errorf(
					"Invalid container declaration: Unsupported emptyDir volume medium '%s'", apiVolume.EmptyDir.Medium)
			}
			tmpFsBinds[apiVolumeMount.MountPath] = strings.Join(tmpFsOpts, ",")
		}
		if volumesInSpec != 1 {
			return "", fmt.Errorf(
				"Invalid container declaration: Volume can have only one of the properties: hostPath or emptyDir, %d properties found", volumesInSpec)
		}
	}

	// Check for unreferenced volumes
	for _, apiVolume := range spec.Volumes {
		_, volumeMountExists := volumeMountMap[apiVolume.Name]
		if (!volumeMountExists) {
			log.Printf("Warning: Container declaration has no volume mount for volume with name '%s'", apiVolume.Name)
		}
	}

	env := []string{}
	for _, envVar := range container.Env {
		env = append(env, fmt.Sprintf("%s=\"%s\"", envVar.Name, envVar.Value))
	}

	logConfig := dockercontainer.LogConfig{}
	if *gcploggingFlag {
		logConfig.Type = "gcplogs"
	}

	opts := dockertypes.ContainerCreateConfig{
		Name: container.Name,
		Config: &dockercontainer.Config{
			Entrypoint: runCommand,
			Cmd:        runArgs,
			Image:      container.Image,
			Env:        env,
			StdinOnce:  container.StdIn,
			Tty:        container.Tty,
		},
		HostConfig: &dockercontainer.HostConfig{
			Binds: hostPathBinds,
			Tmpfs: tmpFsBinds,
			AutoRemove: true,
			NetworkMode: "host",
			Privileged: container.SecurityContext.Privileged,
			LogConfig: logConfig,
		},
	}

	createResp, err := dockerClient.ContainerCreate(
		ctx, opts.Config, opts.HostConfig, opts.NetworkingConfig, opts.Name)
	if ctxErr := contextError(ctx, "Create container"); ctxErr != nil {
		return "", ctxErr
	}
	if err != nil {
		return "", err
	}
	log.Printf("Created a container with name '%s' and ID: %s", container.Name, createResp.ID)

	return createResp.ID, nil
}

func startContainer(dockerClient DockerApiClient, id string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Printf("Starting a container with ID: %s", id)
	return dockerClient.ContainerStart(ctx, id)
}

func base64EncodeAuth(auth dockertypes.AuthConfig) (string, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(auth); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf.Bytes()), nil
}

func contextError(ctx context.Context, operationType string) error {
	if ctx.Err() == context.DeadlineExceeded {
		return operationTimeout{err: ctx.Err(), operationType: operationType}
	}
	return ctx.Err()
}

