package(default_visibility = ["//visibility:public"])

licenses(["notice"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "cgo_genrule",
    "go_library",
)

go_library(
    name = "gopkg.in/yaml.v2",
    srcs = [
        "gopkg.in/yaml.v2/apic.go",
        "gopkg.in/yaml.v2/decode.go",
        "gopkg.in/yaml.v2/emitterc.go",
        "gopkg.in/yaml.v2/encode.go",
        "gopkg.in/yaml.v2/parserc.go",
        "gopkg.in/yaml.v2/readerc.go",
        "gopkg.in/yaml.v2/resolve.go",
        "gopkg.in/yaml.v2/scannerc.go",
        "gopkg.in/yaml.v2/sorter.go",
        "gopkg.in/yaml.v2/writerc.go",
        "gopkg.in/yaml.v2/yaml.go",
        "gopkg.in/yaml.v2/yamlh.go",
        "gopkg.in/yaml.v2/yamlprivateh.go",
    ],
)

go_library(
    name = "github.com/docker/engine-api/types",
    srcs = [
        "github.com/docker/engine-api/types/auth.go",
        "github.com/docker/engine-api/types/client.go",
        "github.com/docker/engine-api/types/configs.go",
        "github.com/docker/engine-api/types/seccomp.go",
        "github.com/docker/engine-api/types/stats.go",
        "github.com/docker/engine-api/types/types.go",
    ],
    deps = [
        "//vendor:github.com/docker/engine-api/types/container",
        "//vendor:github.com/docker/engine-api/types/filters",
        "//vendor:github.com/docker/engine-api/types/network",
        "//vendor:github.com/docker/engine-api/types/registry",
        "//vendor:github.com/docker/go-connections/nat",
        "//vendor:github.com/docker/go-units",
    ],
)

go_library(
    name = "github.com/docker/engine-api/types/container",
    srcs = [
        "github.com/docker/engine-api/types/container/config.go",
        "github.com/docker/engine-api/types/container/host_config.go",
        "github.com/docker/engine-api/types/container/hostconfig_unix.go",
    ],
    deps = [
        "//vendor:github.com/docker/engine-api/types/blkiodev",
        "//vendor:github.com/docker/engine-api/types/strslice",
        "//vendor:github.com/docker/go-connections/nat",
        "//vendor:github.com/docker/go-units",
    ],
)

go_library(
    name = "github.com/docker/engine-api/types/blkiodev",
    srcs = ["github.com/docker/engine-api/types/blkiodev/blkio.go"],
)

go_library(
    name = "github.com/docker/engine-api/types/strslice",
    srcs = ["github.com/docker/engine-api/types/strslice/strslice.go"],
)

go_library(
    name = "github.com/docker/go-connections/nat",
    srcs = [
        "github.com/docker/go-connections/nat/nat.go",
        "github.com/docker/go-connections/nat/parse.go",
        "github.com/docker/go-connections/nat/sort.go",
    ],
)

go_library(
    name = "github.com/docker/go-units",
    srcs = [
        "github.com/docker/go-units/duration.go",
        "github.com/docker/go-units/size.go",
        "github.com/docker/go-units/ulimit.go",
    ],
)

go_library(
    name = "github.com/docker/engine-api/types/filters",
    srcs = ["github.com/docker/engine-api/types/filters/parse.go"],
    deps = ["//vendor:github.com/docker/engine-api/types/versions"],
)

go_library(
    name = "github.com/docker/engine-api/types/versions",
    srcs = ["github.com/docker/engine-api/types/versions/compare.go"],
)

go_library(
    name = "github.com/docker/engine-api/types/network",
    srcs = ["github.com/docker/engine-api/types/network/network.go"],
)

go_library(
    name = "github.com/docker/engine-api/types/registry",
    srcs = ["github.com/docker/engine-api/types/registry/registry.go"],
)

go_library(
    name = "github.com/docker/engine-api/client",
    srcs = [
        "github.com/docker/engine-api/client/client.go",
        "github.com/docker/engine-api/client/client_unix.go",
        "github.com/docker/engine-api/client/container_attach.go",
        "github.com/docker/engine-api/client/container_commit.go",
        "github.com/docker/engine-api/client/container_copy.go",
        "github.com/docker/engine-api/client/container_create.go",
        "github.com/docker/engine-api/client/container_diff.go",
        "github.com/docker/engine-api/client/container_exec.go",
        "github.com/docker/engine-api/client/container_export.go",
        "github.com/docker/engine-api/client/container_inspect.go",
        "github.com/docker/engine-api/client/container_kill.go",
        "github.com/docker/engine-api/client/container_list.go",
        "github.com/docker/engine-api/client/container_logs.go",
        "github.com/docker/engine-api/client/container_pause.go",
        "github.com/docker/engine-api/client/container_remove.go",
        "github.com/docker/engine-api/client/container_rename.go",
        "github.com/docker/engine-api/client/container_resize.go",
        "github.com/docker/engine-api/client/container_restart.go",
        "github.com/docker/engine-api/client/container_start.go",
        "github.com/docker/engine-api/client/container_stats.go",
        "github.com/docker/engine-api/client/container_stop.go",
        "github.com/docker/engine-api/client/container_top.go",
        "github.com/docker/engine-api/client/container_unpause.go",
        "github.com/docker/engine-api/client/container_update.go",
        "github.com/docker/engine-api/client/container_wait.go",
        "github.com/docker/engine-api/client/errors.go",
        "github.com/docker/engine-api/client/events.go",
        "github.com/docker/engine-api/client/hijack.go",
        "github.com/docker/engine-api/client/image_build.go",
        "github.com/docker/engine-api/client/image_create.go",
        "github.com/docker/engine-api/client/image_history.go",
        "github.com/docker/engine-api/client/image_import.go",
        "github.com/docker/engine-api/client/image_inspect.go",
        "github.com/docker/engine-api/client/image_list.go",
        "github.com/docker/engine-api/client/image_load.go",
        "github.com/docker/engine-api/client/image_pull.go",
        "github.com/docker/engine-api/client/image_push.go",
        "github.com/docker/engine-api/client/image_remove.go",
        "github.com/docker/engine-api/client/image_save.go",
        "github.com/docker/engine-api/client/image_search.go",
        "github.com/docker/engine-api/client/image_tag.go",
        "github.com/docker/engine-api/client/info.go",
        "github.com/docker/engine-api/client/interface.go",
        "github.com/docker/engine-api/client/login.go",
        "github.com/docker/engine-api/client/network_connect.go",
        "github.com/docker/engine-api/client/network_create.go",
        "github.com/docker/engine-api/client/network_disconnect.go",
        "github.com/docker/engine-api/client/network_inspect.go",
        "github.com/docker/engine-api/client/network_list.go",
        "github.com/docker/engine-api/client/network_remove.go",
        "github.com/docker/engine-api/client/request.go",
        "github.com/docker/engine-api/client/version.go",
        "github.com/docker/engine-api/client/volume_create.go",
        "github.com/docker/engine-api/client/volume_inspect.go",
        "github.com/docker/engine-api/client/volume_list.go",
        "github.com/docker/engine-api/client/volume_remove.go",
    ],
    deps = [
        "//vendor:github.com/docker/distribution/reference",
        "//vendor:github.com/docker/engine-api/client/transport",
        "//vendor:github.com/docker/engine-api/client/transport/cancellable",
        "//vendor:github.com/docker/engine-api/types",
        "//vendor:github.com/docker/engine-api/types/container",
        "//vendor:github.com/docker/engine-api/types/filters",
        "//vendor:github.com/docker/engine-api/types/network",
        "//vendor:github.com/docker/engine-api/types/reference",
        "//vendor:github.com/docker/engine-api/types/registry",
        "//vendor:github.com/docker/engine-api/types/time",
        "//vendor:github.com/docker/go-connections/sockets",
        "//vendor:github.com/docker/go-connections/tlsconfig",
        "//vendor:golang.org/x/net/context",
    ],
)

go_library(
    name = "github.com/docker/distribution/reference",
    srcs = [
        "github.com/docker/distribution/reference/reference.go",
        "github.com/docker/distribution/reference/regexp.go",
    ],
    deps = ["//vendor:github.com/docker/distribution/digest"],
)

go_library(
    name = "github.com/docker/distribution/digest",
    srcs = [
        "github.com/docker/distribution/digest/digest.go",
        "github.com/docker/distribution/digest/digester.go",
        "github.com/docker/distribution/digest/doc.go",
        "github.com/docker/distribution/digest/set.go",
        "github.com/docker/distribution/digest/verifiers.go",
    ],
)

go_library(
    name = "github.com/docker/engine-api/client/transport",
    srcs = [
        "github.com/docker/engine-api/client/transport/client.go",
        "github.com/docker/engine-api/client/transport/transport.go",
    ],
    deps = ["//vendor:github.com/docker/go-connections/sockets"],
)

go_library(
    name = "github.com/docker/go-connections/sockets",
    srcs = [
        "github.com/docker/go-connections/sockets/inmem_socket.go",
        "github.com/docker/go-connections/sockets/proxy.go",
        "github.com/docker/go-connections/sockets/sockets.go",
        "github.com/docker/go-connections/sockets/sockets_unix.go",
        "github.com/docker/go-connections/sockets/tcp_socket.go",
        "github.com/docker/go-connections/sockets/unix_socket.go",
    ],
    deps = [
        "//vendor:github.com/Sirupsen/logrus",
        "//vendor:github.com/opencontainers/runc/libcontainer/user",
        "//vendor:golang.org/x/net/proxy",
    ],
)

go_library(
    name = "github.com/Sirupsen/logrus",
    srcs = [
        "github.com/Sirupsen/logrus/entry.go",
        "github.com/Sirupsen/logrus/exported.go",
        "github.com/Sirupsen/logrus/formatter.go",
        "github.com/Sirupsen/logrus/hooks.go",
        "github.com/Sirupsen/logrus/json_formatter.go",
        "github.com/Sirupsen/logrus/logger.go",
        "github.com/Sirupsen/logrus/logrus.go",
        "github.com/Sirupsen/logrus/terminal_linux.go",
        "github.com/Sirupsen/logrus/terminal_notwindows.go",
        "github.com/Sirupsen/logrus/text_formatter.go",
    ],
)

go_library(
    name = "github.com/opencontainers/runc/libcontainer/user",
    srcs = [
        "github.com/opencontainers/runc/libcontainer/user/lookup.go",
        "github.com/opencontainers/runc/libcontainer/user/lookup_unix.go",
        "github.com/opencontainers/runc/libcontainer/user/user.go",
    ],
)

go_library(
    name = "golang.org/x/net/proxy",
    srcs = [
        "golang.org/x/net/proxy/direct.go",
        "golang.org/x/net/proxy/per_host.go",
        "golang.org/x/net/proxy/proxy.go",
        "golang.org/x/net/proxy/socks5.go",
    ],
)

go_library(
    name = "github.com/docker/engine-api/client/transport/cancellable",
    srcs = [
        "github.com/docker/engine-api/client/transport/cancellable/canceler.go",
        "github.com/docker/engine-api/client/transport/cancellable/cancellable.go",
    ],
    deps = [
        "//vendor:github.com/docker/engine-api/client/transport",
        "//vendor:golang.org/x/net/context",
    ],
)

go_library(
    name = "github.com/docker/engine-api/types/reference",
    srcs = ["github.com/docker/engine-api/types/reference/image_reference.go"],
    tags = ["automanaged"],
    deps = ["//vendor:github.com/docker/distribution/reference"],
)

go_library(
    name = "github.com/docker/engine-api/types/time",
    srcs = ["github.com/docker/engine-api/types/time/timestamp.go"],
)

go_library(
    name = "github.com/docker/go-connections/tlsconfig",
    srcs = [
        "github.com/docker/go-connections/tlsconfig/config.go",
        "github.com/docker/go-connections/tlsconfig/config_client_ciphers.go",
    ],
    deps = ["//vendor:github.com/Sirupsen/logrus"],
)

go_library(
    name = "golang.org/x/net/context",
    srcs = [
        "golang.org/x/net/context/context.go",
        "golang.org/x/net/context/go17.go",
    ],
)

go_library(
    name = "golang.org/x/net/context/ctxhttp",
    srcs = ["golang.org/x/net/context/ctxhttp/ctxhttp.go"],
    deps = ["//vendor:golang.org/x/net/context"],
)

go_library(
    name = "github.com/google/shlex",
    srcs = ["github.com/google/shlex/shlex.go"],
)
