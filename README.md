# DEPRECATED Compute Engine Container Startup Agent

> [!NOTE]
> The container startup agent is deprecated. To configure and deploy a container on VM
or on VMs in a MIG during VM creation, use a startup script or cloud-init. You can also consider
using one of the managed services such as Google Kubernetes Engine (GKE) or Batch to run containers on the VM.
For more information, see [Migrate containers that were deployed on VMs during VM creation](https://cloud.google.com/compute/docs/containers/migrate-containers).

The Compute Engine container startup agent starts a container deployed on a VM
instance using [Deploying Containers on VMs and Managed Instance Groups](
https://cloud.google.com/compute/docs/instance-groups/deploying-docker-containers) method.

The agent parses container declaration that is stored in VM instance metadata
under `gce-container-declaration` key and starts the container with the declared
configuration options.

The agent is bundled with [Container-Optimized OS](
https://cloud.google.com/container-optimized-os/docs/) image, version 62 or higher.

## Building the agent

Container startup agent is deployed as a Docker container and is hosted in
Container Registry:
[gcr.io/gce-containers/konlet](http://gcr.io/gce-containers/konlet).

To build the agent using bazel run this command from the repository root:
```shell
$ bazel build gce-containers-startup
```

Export your GCP project name to an environment variable:
```shell
$ export MY_PROJECT=your-project-name
```

To build a Docker image, copy the build binary to the 'docker' directory and
invoke the docker build command:
```shell
$ cp bazel-bin/gce-containers-startup/gce-containers-startup docker/
$ docker build docker/ -t gcr.io/$MY_PROJECT/gce-containers-startup
```

To push resulting Docker image to Google Container Registry you can use [gcloud
command](https://cloud.google.com/container-registry/docs/pushing-and-pulling):
```shell
$ gcloud docker -- push gcr.io/$MY_PROJECT/gce-containers-startup
```

## Usage

If you would like to install the container startup agent on your VM, copy its systemd service definition and scripts:

* Copy `systemd` service definition `konlet-startup.service` into systemd services directory, e.g. `/usr/lib/systemd/system`.

* Copy `get_metadata_value` script to `/usr/share/google` directory.

* Copy `konlet-startup` script to `/var/lib/google` directory


You can also those scripts directly by a [startup script](https://cloud.google.com/compute/docs/startupscript),
`cloud-init` or `rc.d` scripts.


## License

Compute Engine container startup agent is licensed under Apache License 2.0. The
license is available in the [LICENSE file](LICENSE).

You can find out more about the Apache License 2.0 at:
[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0).
