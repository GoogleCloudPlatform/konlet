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

// RestartPolicy describes how the container should be restarted.
// Only one of the following restart policies may be specified.
// If none of the following policies is specified, the default one
// is RestartPolicyAlways.
type RestartPolicy string

const (
	RestartPolicyAlways    RestartPolicy = "Always"
	RestartPolicyOnFailure RestartPolicy = "OnFailure"
	RestartPolicyNever     RestartPolicy = "Never"
)

/* Structure describing single container */
type Container struct {
	/* Name of the container */
	Name string

	/* URL of the container, eg. gcr.io/google-containers/busybox:latest */
	Image string `yaml:"image"`

	/* Command to run in the container */
	Command []string `yaml:"command"`

	/* Args to be passed to command */
	Args []string `yaml:"args"`

	/* Volumes to be mounted for container */
	VolumeMounts []struct {
		/* Name of the volume to mount */
		Name string `yaml:"name"`
		/* Path on container */
		MountPath string `yaml:"mountPath"`
		/* Should the volume be read-only */
		ReadOnly bool `yaml:"readOnly"`
	} `yaml:"volumeMounts"`

	/* Environment variables to be passed to container */
	Env []struct {
		/* Name of environment variable */
		Name string
		/* Value of environment variable */
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

/* Structure describing single container */
type ContainerSpec struct {
	/* Body of the specification */
	Spec ContainerSpecStruct
}

type ContainerSpecStruct struct {
	/* Array of container, must be size 1 */
	Containers []Container

	/* Array of volumes, must correspond to containers */
	Volumes []Volume

	/* Restart policy for containers */
	RestartPolicy *RestartPolicy `yaml:"restartPolicy"`
}

type Volume struct {
	Name string
	// Only one of EmptyDir, HostPath or GcePersistentDiskVolume should be present
	EmptyDir          *EmptyDirVolume          `yaml:"emptyDir"`
	HostPath          *HostPathVolume          `yaml:"hostPath"`
	GcePersistentDisk *GcePersistentDiskVolume `yaml:"gcePersistentDisk"`
}

// EmptyDirVolume represents an empty directory (hence the name) that can be
// mounted into a container.
type EmptyDirVolume struct {
	// The only currently supported Medium is "Memory"
	Medium string
}

type HostPathVolume struct {
	Path string
}

type GcePersistentDiskVolume struct {
	PdName    string `yaml:"pdName"`
	FsType    string `yaml:"fsType,omitempty"`
	Partition int    `yaml:"partition,omitempty"`
}
