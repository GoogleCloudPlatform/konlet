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

package(default_visibility = ["//visibility:public"])

licenses(["notice"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_binary",
    "go_library",
    "go_test",
)
load("@io_bazel_rules_docker//docker:docker.bzl", "docker_build")
load("@bazel_tools//tools/build_defs/pkg:pkg.bzl", "pkg_deb", "pkg_tar")

go_binary(
    name = "gce-containers-startup",
    srcs = [
        "gce-containers-startup.go",
    ],
    importpath = "github.com/GoogleCloudPlatform/konlet/gce-containers-startup",
    deps = [
        "//gce-containers-startup/command",
        "//gce-containers-startup/metadata",
        "//gce-containers-startup/runtime",
        "//gce-containers-startup/types:go_default_library",
        "//gce-containers-startup/utils:go_default_library",
        "//gce-containers-startup/vendor:gopkg.in/yaml.v2",
        "//gce-containers-startup/volumes",
    ],
)

go_library(
    name = "go_default_library",
    srcs = [
        "gce-containers-startup.go",
    ],
    importpath = "github.com/GoogleCloudPlatform/konlet/gce-containers-startup",
    deps = [
        "//gce-containers-startup/command",
        "//gce-containers-startup/metadata",
        "//gce-containers-startup/runtime",
        "//gce-containers-startup/types:go_default_library",
        "//gce-containers-startup/utils:go_default_library",
        "//gce-containers-startup/vendor:gopkg.in/yaml.v2",
        "//gce-containers-startup/volumes",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "gce-containers-startup_test.go",
    ],
    embed = [":go_default_library"],
    importpath = "github.com/GoogleCloudPlatform/konlet/gce-containers-startup",
    deps = [
        "//gce-containers-startup/utils:go_default_library",
        "//gce-containers-startup/vendor:github.com/docker/engine-api/client",
        "//gce-containers-startup/vendor:github.com/docker/engine-api/types",
        "//gce-containers-startup/vendor:github.com/docker/engine-api/types/container",
        "//gce-containers-startup/vendor:github.com/docker/engine-api/types/network",
        "//gce-containers-startup/vendor:github.com/docker/engine-api/types/strslice",
        "//gce-containers-startup/vendor:golang.org/x/net/context",
        "//gce-containers-startup/vendor:gopkg.in/yaml.v2",
    ],
)
