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

FROM gcr.io/google_containers/ubuntu-slim:0.6
MAINTAINER gjaskiewicz@google.com

RUN apt-get update && apt-get install -y iptables mount util-linux
COPY gce-containers-startup /bin/gce-containers-startup
COPY e2fslibs_1.43.5-1_amd64.deb /tmp/e2fslibs.deb
COPY e2fsprogs_1.43.5-1_amd64.deb /tmp/e2fsprogs.deb
RUN dpkg -i /tmp/e2fslibs.deb
RUN dpkg -i /tmp/e2fsprogs.deb
RUN rm /tmp/e2fslibs.deb /tmp/e2fsprogs.deb

CMD ["/bin/gce-containers-startup"]

