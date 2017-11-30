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
    "go_prefix",
    "go_library",
    "go_test",
)
load("@io_bazel_rules_docker//docker:docker.bzl", "docker_build")
load("@bazel_tools//tools/build_defs/pkg:pkg.bzl", "pkg_deb", "pkg_tar")

go_prefix("github.com/konlet")

go_binary(
    name = "gce-containers-startup",
    srcs = [
        "gce-containers-startup.go",
    ],
    deps = [
        "//types:go_default_library",
        "//utils:go_default_library",
        "//vendor:gopkg.in/yaml.v2",
    ],
)

go_library(
    name = "go_default_library",
    srcs = [
        "gce-containers-startup.go",
    ],
    deps = [
        "//types:go_default_library",
        "//utils:go_default_library",
        "//vendor:gopkg.in/yaml.v2",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "gce-containers-startup.go",
        "gce-containers-startup_test.go",
        "utils_test.go",
    ],
    deps = [
        "//types:go_default_library",
        "//utils:go_default_library",
        "//vendor:github.com/docker/engine-api/client",
        "//vendor:github.com/docker/engine-api/types",
        "//vendor:github.com/docker/engine-api/types/container",
        "//vendor:github.com/docker/engine-api/types/network",
        "//vendor:github.com/docker/engine-api/types/strslice",
        "//vendor:golang.org/x/net/context",
        "//vendor:gopkg.in/yaml.v2",
    ],
)
