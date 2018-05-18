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

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "auth.go",
        "iptables.go",
        "registry.go",
        "testing.go",
        "welcome-message.go",
    ],
    importpath = "github.com/GoogleCloudPlatform/konlet/gce-containers-startup/utils",
)

go_test(
    name = "go_default_test",
    srcs = [
        "utils_test.go",
    ],
    embed = [":go_default_library"],
    importpath = "github.com/GoogleCloudPlatform/konlet/gce-containers-startup/utils",
)
