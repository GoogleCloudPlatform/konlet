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

package types

/* Structure describing single container */
type Container struct{
	/* Name of the container */
	Name string

	/* URL of the container, eg. gcr.io/google-containers/busybox:latest */
	Image string `yaml:"image"`

	/* Command to run in the container */
	Command string `yaml:"command"`

	/* Args to be passed to command */
	Args []string `yaml:"args"`

	/* Volumes to be mounted for container */
	Volumes []struct {
		// Only one of Tmpfs or HostPath should be present
		Tmpfs *TmpfsVolume `yaml:"tmpfs"`
		HostPath *HostPathVolume `yaml:"hostPath"`
	}

	/* Environment variables to be passed to container */
	Env []struct {
		Name string
		Value string
	}

	/* Container security context */
	SecurityContext SecurityContextDeclaration `yaml:"securityContext"`

	/* Should container have STDIN open */
	StdIn bool

	/* Allocate pseudo-TTY for container */
	Tty bool
}

type SecurityContextDeclaration struct {
	/* Should container be run in privileged mode */
	Privileged bool
}

type HostPathVolume struct{
	/* Should the volume be read-only */
	ReadOnly  bool `yaml:"readOnly"`
	/* Path on container */
	MountPath string `yaml:"mountPath"`
	/* Path on host */
	Path string
}

type TmpfsVolume struct {
	/* Path on container */
	MountPath string `yaml:"mountPath"`
	/* Size of Volume */
	Size      string
}

/* Structure describing single container */
type ContainerSpec struct {
	/* Version of the metadata */
	Version string

	/* Array of container, must be size 1 */
	Containers []Container
}
