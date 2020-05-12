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

package runtime

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"strings"
	"unicode"

	"golang.org/x/net/context"

	dockerapi "github.com/docker/engine-api/client"
	dockertypes "github.com/docker/engine-api/types"
	dockercontainer "github.com/docker/engine-api/types/container"
	dockernetwork "github.com/docker/engine-api/types/network"
	dockerstrslice "github.com/docker/engine-api/types/strslice"

	"io"

	"github.com/GoogleCloudPlatform/konlet/gce-containers-startup/metadata"
	api "github.com/GoogleCloudPlatform/konlet/gce-containers-startup/types"
	"github.com/GoogleCloudPlatform/konlet/gce-containers-startup/volumes"
)

const DOCKER_UNIX_SOCKET = "unix:///var/run/docker.sock"
const CONTAINER_NAME_PREFIX = "klt"

// operationTimeout is the error returned when the docker operations are timeout.
type operationTimeout struct {
	err           error
	operationType string
}

type DockerApiClient interface {
	ImagePull(ctx context.Context, ref string, options dockertypes.ImagePullOptions) (io.ReadCloser, error)
	ContainerCreate(ctx context.Context, config *dockercontainer.Config, hostConfig *dockercontainer.HostConfig, networkingConfig *dockernetwork.NetworkingConfig, containerName string) (dockertypes.ContainerCreateResponse, error)
	ContainerStart(ctx context.Context, container string) error
	ContainerList(ctx context.Context, opts dockertypes.ContainerListOptions) ([]dockertypes.Container, error)
	ContainerRemove(ctx context.Context, containerID string, opts dockertypes.ContainerRemoveOptions) error
}

type OsCommandRunner interface {
	Run(...string) (string, error)
	MkdirAll(path string, perm os.FileMode) error
	Stat(name string) (os.FileInfo, error)
}

func (e operationTimeout) Error() string {
	return fmt.Sprintf("%s operation timeout: %v", e.operationType, e.err)
}

type ContainerRunner struct {
	Client     DockerApiClient
	VolumesEnv *volumes.Env
	RandEnv    *rand.Rand
}

// To produce deterministic results, tests can use a constant seed, while real runtime
// can seed based on entropy.
func generateRandomSuffix(length int, randEnv *rand.Rand) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyz")
	generated := make([]rune, length)
	for i := range generated {
		generated[i] = letters[randEnv.Intn(len(letters))]
	}
	return string(generated)
}

func GetDefaultRunner(osCommandRunner OsCommandRunner, metadataProvider metadata.Provider) (*ContainerRunner, error) {
	var dockerClient DockerApiClient
	var err error
	dockerClient, err = dockerapi.NewClient(DOCKER_UNIX_SOCKET, "", nil, nil)
	if err != nil {
		return nil, err
	}
	// In order to make container names and other randomly generated content
	// deterministic on the same machine during each restart cycle, we seed
	// the generator with hostname and boot time.
	var hostname string
	hostname, err = os.Hostname()
	if err != nil {
		return nil, err
	}
	var lastBootTime string
	lastBootTime, err = osCommandRunner.Run("who", "-b")
	if err != nil {
		return nil, err
	}
	hashedHostnameAndBoot := fnv.New64a()
	hashedHostnameAndBoot.Write([]byte(hostname))
	hashedHostnameAndBoot.Write([]byte(" * * * ")) // Some separator.
	hashedHostnameAndBoot.Write([]byte(lastBootTime))
	randEnv := rand.New(rand.NewSource(int64(hashedHostnameAndBoot.Sum64())))

	return &ContainerRunner{Client: dockerClient, RandEnv: randEnv, VolumesEnv: &volumes.Env{OsCommandRunner: osCommandRunner, MetadataProvider: metadataProvider}}, nil
}

func (runner ContainerRunner) RunContainer(auth string, spec api.ContainerSpecStruct, detach bool) error {
	var id string
	var err error
	id, err = createContainer(runner, auth, spec)
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

	authStruct := dockertypes.AuthConfig{}
	if auth != "" {
		authStruct.Username = "_token"
		authStruct.Password = auth
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

// deleteOldContainers deletes all containers started by konlet.
// rawName is a container name without any generate prefixes or suffixes.
func deleteOldContainers(dockerClient DockerApiClient, rawName string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	listOpts := dockertypes.ContainerListOptions{All: true}
	containers, err := dockerClient.ContainerList(ctx, listOpts)
	if err != nil {
		return err
	}
	idsNames := containersStartedByKonlet(containers, rawName)
	if len(idsNames) == 0 {
		log.Print("No containers created by previous runs of Konlet found.\n")
		return nil
	}
	for id, name := range idsNames {
		log.Printf("Removing a container created by a previous run of Konlet: '%s' (ID: %s)\n", name, id)
		rmOpts := dockertypes.ContainerRemoveOptions{
			Force: true,
		}
		if err := dockerClient.ContainerRemove(ctx, id, rmOpts); err != nil {
			return err
		}
	}
	return nil
}

// containersStartedByKonlet filters a given list of containers to return a map
// from container id to container name for containers started by konlet.
// These are all containers whose names match one of two cases:
// 1. the name starts with '/klt-',
// 2. the name is '/<rawName>', where rawName is a legacy container name passed as an argument.
// NOTE: The reason for the leading slash is Docker's convention of adding these
// to names specified by the user.
// NOTE: The reason for two cases above is that historically konlet started
// containers without prefixes or suffixes and it would fail to delete an old
// container after system update to a newer version if it only looked for the prefix.
// See https://github.com/GoogleCloudPlatform/konlet/issues/50
func containersStartedByKonlet(containers []dockertypes.Container, rawName string) map[string]string {
	var (
		// Matches containers started by konlet.
		namePattern1 = "/klt-"
		// The legacy pattern, see the comment on top of the function.
		namePattern2 = "/" + rawName
	)
	idsNames := make(map[string]string)
	for _, container := range containers {
		for _, name := range container.Names {
			if strings.HasPrefix(name, namePattern1) || name == namePattern2 {
				idsNames[container.ID] = name
				break
			}
		}
	}
	return idsNames
}

func createContainer(runner ContainerRunner, auth string, spec api.ContainerSpecStruct) (string, error) {
	if len(spec.Containers) != 1 {
		return "", fmt.Errorf("Exactly one container in declaration expected.")
	}

	container := spec.Containers[0]
	generatedContainerName := fmt.Sprintf("%s-%s-%s", CONTAINER_NAME_PREFIX, container.Name, generateRandomSuffix(4, runner.RandEnv))
	log.Printf("Configured container '%s' will be started with name '%s'.\n", container.Name, generatedContainerName)

	if err := pullImage(runner.Client, auth, container); err != nil {
		return "", err
	}

	if err := deleteOldContainers(runner.Client, container.Name); err != nil {
		return "", err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	printWarningIfLikelyHasMistake(container.Command, container.Args)

	var runCommand dockerstrslice.StrSlice
	if container.Command != nil {
		runCommand = dockerstrslice.StrSlice(container.Command)
	}
	var runArgs dockerstrslice.StrSlice
	if container.Args != nil {
		runArgs = dockerstrslice.StrSlice(container.Args)
	}

	if err := runner.VolumesEnv.UnmountExistingVolumes(); err != nil {
		log.Printf("Error: failed to unmount volumes:\n%v", err)
	}
	containerVolumeBindingConfigurationMap, volumePrepareError := runner.VolumesEnv.PrepareVolumesAndGetBindings(spec)
	if volumePrepareError != nil {
		return "", volumePrepareError
	}
	volumeBindingConfiguration, volumeBindingFound := containerVolumeBindingConfigurationMap[container.Name]
	if !volumeBindingFound {
		return "", fmt.Errorf("Volume binding configuration for container %s not found in the map. This should not happen.", container.Name)
	}
	// Docker-API compatible types.
	hostPathBinds := []string{}
	for _, hostPathBindConfiguration := range volumeBindingConfiguration {
		hostPathBind := fmt.Sprintf("%s:%s", hostPathBindConfiguration.HostPath, hostPathBindConfiguration.ContainerPath)
		if hostPathBindConfiguration.ReadOnly {
			hostPathBind = fmt.Sprintf("%s:ro", hostPathBind)
		}
		hostPathBinds = append(hostPathBinds, hostPathBind)
	}

	env := []string{}
	for _, envVar := range container.Env {
		env = append(env, fmt.Sprintf("%s=%s", envVar.Name, envVar.Value))
	}

	restartPolicyName := "always"
	if spec.RestartPolicy == nil || *spec.RestartPolicy == api.RestartPolicyAlways {
		restartPolicyName = "always"
	} else if *spec.RestartPolicy == api.RestartPolicyOnFailure {
		restartPolicyName = "on-failure"
	} else if *spec.RestartPolicy == api.RestartPolicyNever {
		restartPolicyName = "no"
	} else {
		return "", fmt.Errorf(
			"Invalid container declaration: Unsupported container restart policy '%s'", *spec.RestartPolicy)
	}

	opts := dockertypes.ContainerCreateConfig{
		Name: generatedContainerName,
		Config: &dockercontainer.Config{
			Entrypoint: runCommand,
			Cmd:        runArgs,
			Image:      container.Image,
			Env:        env,
			OpenStdin:  container.StdIn,
			Tty:        container.Tty,
		},
		HostConfig: &dockercontainer.HostConfig{
			Binds:       hostPathBinds,
			AutoRemove:  false,
			NetworkMode: "host",
			Privileged:  container.SecurityContext.Privileged,
			LogConfig: dockercontainer.LogConfig{
				Type: "json-file",
				Config: map[string]string{
					"max-size": "500m",
					"max-file": "3",
				},
			},
			RestartPolicy: dockercontainer.RestartPolicy{
				Name: restartPolicyName,
			},
		},
	}

	createResp, err := runner.Client.ContainerCreate(
		ctx, opts.Config, opts.HostConfig, opts.NetworkingConfig, opts.Name)
	if ctxErr := contextError(ctx, "Create container"); ctxErr != nil {
		return "", ctxErr
	}
	if err != nil {
		return "", err
	}
	log.Printf("Created a container with name '%s' and ID: %s", generatedContainerName, createResp.ID)

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

func printWarningIfLikelyHasMistake(command []string, args []string) {
	var commandAndArgs []string
	if command != nil {
		commandAndArgs = append(commandAndArgs, command...)
	}
	if args != nil {
		commandAndArgs = append(commandAndArgs, args...)
	}
	if len(commandAndArgs) == 1 && containsWhitespace(commandAndArgs[0]) {
		fields := strings.Fields(commandAndArgs[0])
		if len(fields) > 1 {
			log.Printf("Warning: executable \"%s\" contains whitespace, which is "+
				"likely not what you intended. If your intention was to provide "+
				"arguments to \"%s\" and you are using gcloud, use the "+
				"\"--container-arg\" option. If you are using Google Cloud Console, "+
				"specify the arguments separately under \"Command and arguments\" in "+
				"\"Advanced container options\".", commandAndArgs[0], fields[0])
		} else {
			log.Printf("Warning: executable \"%s\" contains whitespace, which is "+
				"likely not what you intended. Maybe you accidentally left "+
				"leading/trailing whitespace?", commandAndArgs[0])
		}
	}
}

func containsWhitespace(s string) bool {
	for _, r := range s {
		if unicode.IsSpace(r) {
			return true
		}
	}
	return false
}
