#!/bin/bash

# Copyright 2017 Google Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Exit the script if any command has an error.
set -o errexit

readonly GET_METADATA_VALUE='/var/lib/google/get_metadata_value'
IS_CONTAINER_SPEC=$(${GET_METADATA_VALUE} "attributes/" | grep "gce-container-declaration" | wc -l)

if [[ IS_CONTAINER_SPEC -eq 0 ]]; then
  echo "No metadata present - not running containers"
  exit 0
fi

docker run --privileged \
  -v=/var/run/docker.sock:/var/run/docker.sock \
  -v=/etc/profile.d:/host/etc/profile.d \
  --net="host" \
  gcr.io/gce-containers/konlet:v.0.6 \
  2>&1 >/dev/ttyS1

