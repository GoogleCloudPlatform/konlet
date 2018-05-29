# Copyright 2018 Google Inc. All Rights Reserved.
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

package(default_visibility = ["//visibility:public"])

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "volumes",
    srcs = [
        "volumes.go",
    ],
    importpath = "github.com/GoogleCloudPlatform/konlet/gce-containers-startup/volumes",
    deps = [
        "//gce-containers-startup/metadata",
        "//gce-containers-startup/types:go_default_library",
    ],
)

go_test(
    name = "volumes_test",
    srcs = [
        "volumes_test.go",
    ],
    embed = [":volumes"],
    importpath = "github.com/GoogleCloudPlatform/konlet/gce-containers-startup/volumes",
    deps = [
        "//gce-containers-startup/command",
        "//gce-containers-startup/metadata",
        "//gce-containers-startup/types:go_default_library",
        "//gce-containers-startup/utils:go_default_library",
        "//gce-containers-startup/vendor:gopkg.in/yaml.v2",
    ],
)
