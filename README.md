# protoc-gen-go-defaults

[![CI](https://github.com/protoc-contrib/protoc-gen-go-defaults/actions/workflows/ci.yml/badge.svg)](https://github.com/protoc-contrib/protoc-gen-go-defaults/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/protoc-contrib/protoc-gen-go-defaults?include_prereleases)](https://github.com/protoc-contrib/protoc-gen-go-defaults/releases)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENCE)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![protoc](https://img.shields.io/badge/protoc-compatible-blue)](https://protobuf.dev)
[![Coverage](https://raw.githubusercontent.com/protoc-contrib/protoc-gen-go-defaults/main/.github/octocov/badge.svg)](https://github.com/protoc-contrib/protoc-gen-go-defaults/actions/workflows/ci.yml)

A [protoc](https://protobuf.dev) plugin that generates `Default()` methods for
Protocol Buffer messages from field and message options. It works on Go code
produced by `protoc-gen-go`, adds compile-time defaults, and ships with a
reflection-based `defaults.Apply` helper for dynamic callers.

This project started as a fork of
[linka-cloud/protoc-gen-defaults](https://github.com/linka-cloud/protoc-gen-defaults)
and has since been significantly refactored: the code generator was rewritten
on top of `google.golang.org/protobuf/compiler/protogen` (replacing the
archived `lyft/protoc-gen-star`), the build and release pipeline now uses Nix
+ `release-please`, tests were migrated to Ginkgo/Gomega, deprecated
dependencies were removed, and several code-generation bugs (notably around
`fixed64` defaults) were fixed.

## Features

- Annotate `.proto` fields with `(defaults.value)` to populate scalars,
  enums, bytes, wrappers, and `google.protobuf.{Duration,Timestamp}` on zero
  values.
- Pick the active arm of a `oneof` declaratively with `(defaults.oneof)`.
- Skip generation entirely (`(defaults.skip)`) or expose an unexported
  `_Default()` method (`(defaults.unexported)`) when you want to compose
  with custom logic.
- Runtime `defaults.Apply(proto.Message)` mirrors the generated code via
  proto reflection for callers that do not have the concrete Go type.
- Respects proto3 optional presence: explicitly set fields are never
  overwritten.

## Installation

```bash
go install github.com/protoc-contrib/protoc-gen-go-defaults/cmd/protoc-gen-go-defaults@latest
```

## Usage

### With buf

Add the plugin to your `buf.gen.yaml`:

```yaml
version: v2
plugins:
  - remote: buf.build/protocolbuffers/go
    out: gen/proto/go
    opt:
      - paths=source_relative
  - local: protoc-gen-go-defaults
    out: gen/proto/go
    opt:
      - paths=source_relative
```

Then run:

```bash
buf generate
```

### With protoc

```bash
protoc \
  --go_out=. --go_opt=paths=source_relative \
  --go-defaults_out=. --go-defaults_opt=paths=source_relative \
  -I proto/ \
  proto/example/v1/example.proto
```

## Extensions

Import `defaults/defaults.proto` in your `.proto` files to use the options.

| Extension                  | Scope            | Purpose                                                      |
| -------------------------- | ---------------- | ------------------------------------------------------------ |
| `(defaults.value)`         | FieldOptions     | Default value applied when the field is zero.                |
| `(defaults.oneof)`         | OneofOptions     | Name of the arm to populate when the oneof is unset.         |
| `(defaults.skip)`          | MessageOptions   | Skip `Default()` generation for this message entirely.       |
| `(defaults.unexported)`    | MessageOptions   | Emit an unexported `_Default()` method instead of `Default()`. |

### Scalars and enums

```proto
import "defaults/defaults.proto";

message Account {
  string tier = 1 [(defaults.value).string = "free"];
  int64 max_requests = 2 [(defaults.value).int64 = 100];
  bool active = 3 [(defaults.value).bool = true];
  bytes salt = 4 [(defaults.value).bytes = "??"];

  enum Role { UNKNOWN = 0; USER = 1; ADMIN = 2; }
  Role role = 5 [(defaults.value).enum = 1];
}
```

Generated:

```go
func (x *Account) Default() {
    if x.Tier == "" { x.Tier = "free" }
    if x.MaxRequests == 0 { x.MaxRequests = 100 }
    if !x.Active { x.Active = true }
    if len(x.Salt) == 0 { x.Salt = []byte("??") }
    if x.Role == 0 { x.Role = Account_USER }
}
```

### Wrappers, durations, and timestamps

```proto
import "google/protobuf/duration.proto";
import "google/protobuf/timestamp.proto";
import "google/protobuf/wrappers.proto";

message Job {
  google.protobuf.StringValue name = 1 [(defaults.value).string = "anon"];
  google.protobuf.Duration retry_after = 2 [(defaults.value).duration = "30s"];
  google.protobuf.Timestamp deadline = 3 [(defaults.value).timestamp = "now"];
  google.protobuf.Timestamp epoch = 4 [(defaults.value).timestamp = "1970-01-01T00:00:00Z"];
}
```

Durations accept the Prometheus duration grammar (`ms`, `s`, `m`, `h`, `d`,
`w`, `y`). Timestamps accept `"now"` or any RFC3339/RFC822/RFC850/RFC1123
string.

### Oneofs

```proto
message Notification {
  oneof channel {
    option (defaults.oneof) = "email";
    Email email = 1 [(defaults.value).message = {defaults: true, initialize: true}];
    Sms   sms   = 2;
  }
}
```

`(defaults.oneof)` selects the arm to populate when no member is set, and
`(defaults.value).message` controls whether the arm is initialized and
whether its own `Default()` is invoked.

### Message-level controls

```proto
message Legacy {
  option (defaults.skip) = true;       // no Default() generated at all
}

message Composable {
  option (defaults.unexported) = true; // _Default() generated
  string tier = 1 [(defaults.value).string = "free"];
}
```

With `unexported`, you can write a custom `Default()` that delegates:

```go
func (x *Composable) Default() {
    x._Default()
    // custom logic here
}
```

## Reflection-based Apply

For callers that do not hold a concrete Go type (dynamic proto handlers,
generic middleware), use `defaults.Apply`:

```go
import (
    "github.com/protoc-contrib/protoc-gen-go-defaults/defaults"
    "google.golang.org/protobuf/proto"
)

func Populate(m proto.Message) {
    defaults.Apply(m)
}
```

`Apply` walks the message via `protoreflect` and applies the same defaults
the generated code would, including oneof selection and nested messages.

## CI Integration

Gate builds on the generated `*.pb.defaults.go` files being up-to-date by
running `buf generate` in CI and failing if the worktree is dirty.

## Contributing

To set up a development environment with [Nix](https://nixos.org):

```bash
nix develop
go test ./...
```

Or, without Nix, ensure `go`, `protoc`, and `buf` are on your `PATH`.

## License

[Apache 2.0](LICENCE)
