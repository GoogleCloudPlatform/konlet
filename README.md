# GCE Containers Startup

GCE Container Startup agent is a software, which parses Docker container declaration and launches it on VM. 
The purpose of this project is to expose meaningful API for launching Docker container on virtual manchines.

## Building

Agent is deployed as Docker container, which is hosted in GCR registry. To build agent use bazel command:
```shell
$ bazel build gce-containers-startup
```

Export your GCP project name to environmental variable:
```shell
$ export MY_PROJECT=your-project-name
```

To build Docker image build binary to 'docker' directory and call:
```shell
$ docker build docker/Dockerfile -t gcr.io/$MY_PROJECT/gce-containers-startup
```

To push resulting Docker image to GCR registry you can use gcloud:
```shell
$ gcloud docker push gcr.io/$MY_PROJECT/gce-containers-startup
```

## Usage

If you want to install agent on your VM there is a systemd service definition, which you could copy into 
systemd services directory (eg. /usr/lib/systemd/system). If you decide to do so you need to also copy:

* get\_metadata\_value script to /usr/share/google directory

* gce-containers-konlet.sh script to /var/lib/google directory


You can also those scripts directly by starup-script, cloud-init or rc.d scripts.


## License

The work done has been licensed under Apache License 2.0. The license file can be found
[here](LICENSE). You can find out more about the license at:

http://www.apache.org/licenses/LICENSE-2.0
