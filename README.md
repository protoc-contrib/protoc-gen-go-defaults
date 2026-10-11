# protoc-gen-go-defaults

[![CI](https://github.com/protoc-contrib/protoc-gen-go-defaults/actions/workflows/ci.yml/badge.svg)](https://github.com/protoc-contrib/protoc-gen-go-defaults/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/protoc-contrib/protoc-gen-go-defaults?include_prereleases)](https://github.com/protoc-contrib/protoc-gen-go-defaults/releases)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENCE)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![protoc](https://img.shields.io/badge/protoc-compatible-blue)](https://protobuf.dev)

A [protoc](https://protobuf.dev) plugin that generates `SetDefaults()` methods
for Protocol Buffer messages from field and message options. It works on Go
code produced by `protoc-gen-go` and emits plain Go: no reflection at runtime,
and no dependency beyond the packages the messages already import.

This project started as a fork of
[linka-cloud/protoc-gen-defaults](https://github.com/linka-cloud/protoc-gen-defaults)
and has since been significantly refactored: the code generator was rewritten
on top of `google.golang.org/protobuf/compiler/protogen` (replacing the
archived `lyft/protoc-gen-star`), the build and release pipeline now uses Nix

- `release-please`, tests were migrated to Ginkgo/Gomega, deprecated
  dependencies were removed, and several code-generation bugs (notably around
  `fixed64` defaults) were fixed.

## Features

- Annotate `.proto` fields with `(protoc.contrib.defaults.value)` to populate scalars,
  enums, bytes, wrappers, and `google.protobuf.{Duration,Timestamp}` on zero
  values.
- Pick the active arm of a `oneof` declaratively with `(protoc.contrib.defaults.oneof)`.
- Skip generation entirely (`(protoc.contrib.defaults.skip)`) or expose an unexported
  `_SetDefaults()` method (`(protoc.contrib.defaults.unexported)`) when you want to compose
  with custom logic.
- Recurse into nested messages, including every element of a repeated or map
  field, with direct `SetDefaults()` calls resolved at generation time: a
  missing method is a compile error, not a silent no-op.
- Callers without the concrete Go type (gRPC interceptors, middleware) apply
  defaults through an `interface{ SetDefaults() }` type assertion.
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

The options live in [protoc-types](https://github.com/protoc-contrib/protoc-types),
shared with [protoc-gen-rust-defaults](https://github.com/protoc-contrib/protoc-gen-rust-defaults),
and published to the Buf Schema Registry as
`buf.build/protoc-contrib/protoc-types`. Add it to your `buf.yaml` and import
`protoc/contrib/defaults/options.proto` in your `.proto` files:

```yaml
deps:
  - buf.build/protoc-contrib/protoc-types
```

| Extension                              | Scope          | Purpose                                                              |
| -------------------------------------- | -------------- | -------------------------------------------------------------------- |
| `(protoc.contrib.defaults.value)`      | FieldOptions   | Default value applied when the field is zero.                        |
| `(protoc.contrib.defaults.oneof)`      | OneofOptions   | Name of the arm to populate when the oneof is unset.                 |
| `(protoc.contrib.defaults.skip)`       | MessageOptions | Skip `SetDefaults()` generation for this message entirely.           |
| `(protoc.contrib.defaults.unexported)` | MessageOptions | Emit an unexported `_SetDefaults()` method instead of `SetDefaults()`. |

### Scalars and enums

```proto
import "protoc/contrib/defaults/options.proto";

message Account {
  string tier = 1 [(protoc.contrib.defaults.value).string = "free"];
  int64 max_requests = 2 [(protoc.contrib.defaults.value).int64 = 100];
  bool active = 3 [(protoc.contrib.defaults.value).bool = true];
  bytes salt = 4 [(protoc.contrib.defaults.value).bytes = "??"];

  enum Role { UNKNOWN = 0; USER = 1; ADMIN = 2; }
  Role role = 5 [(protoc.contrib.defaults.value).enum = 1];
  Role fallback_role = 6 [(protoc.contrib.defaults.value).enum_name = "USER"];
}
```

Use `enum_name` when you want the default to survive enum reordering; unknown
names fail loudly at generate time.

Generated:

```go
func (x *Account) SetDefaults() {
    if x.Tier == "" { x.Tier = "free" }
    if x.MaxRequests == 0 { x.MaxRequests = 100 }
    if !x.Active { x.Active = true }
    if len(x.Salt) == 0 { x.Salt = []byte("??") }
    if x.Role == 0 { x.Role = Account_USER }
    if x.FallbackRole == 0 { x.FallbackRole = Account_USER }
}
```

### Wrappers, durations, and timestamps

```proto
import "google/protobuf/duration.proto";
import "google/protobuf/timestamp.proto";
import "google/protobuf/wrappers.proto";

message Job {
  google.protobuf.StringValue name = 1 [(protoc.contrib.defaults.value).string = "anon"];
  google.protobuf.Duration retry_after = 2 [(protoc.contrib.defaults.value).duration = "30s"];
  google.protobuf.Timestamp deadline = 3 [(protoc.contrib.defaults.value).timestamp = "now"];
  google.protobuf.Timestamp epoch = 4 [(protoc.contrib.defaults.value).timestamp = "1970-01-01T00:00:00Z"];
}
```

Durations accept the Prometheus duration grammar (`ms`, `s`, `m`, `h`, `d`,
`w`, `y`). Timestamps accept `"now"` or any RFC3339/RFC822/RFC850/RFC1123
string.

### Oneofs

```proto
message Notification {
  oneof channel {
    option (protoc.contrib.defaults.oneof) = "email";
    Email email = 1 [(protoc.contrib.defaults.value).message = {initialize: true, recurse: true}];
    Sms   sms   = 2;
  }
}
```

`(protoc.contrib.defaults.oneof)` selects the arm to populate when no member is set, and
`(protoc.contrib.defaults.value).message` controls whether the arm is initialized and
whether its own `SetDefaults()` is invoked.

### Message-level controls

```proto
message Legacy {
  option (protoc.contrib.defaults.skip) = true;       // no SetDefaults() generated at all
}

message Composable {
  option (protoc.contrib.defaults.unexported) = true; // _SetDefaults() generated
  string tier = 1 [(protoc.contrib.defaults.value).string = "free"];
}
```

With `unexported`, you can write a custom `SetDefaults()` that delegates:

```go
func (x *Composable) SetDefaults() {
    x._SetDefaults()
    // custom logic here
}
```

### Recursion

`message = {recurse: true}` calls the nested message's `SetDefaults()`
directly. Whether that call is emitted depends on the target:

- **A message with annotations of its own** has a generated method, so the
  call is emitted. This also applies to a message in another package, which
  must be generated with this plugin too.
- **A `skip` or `unexported` message** is expected to have a hand-written
  `SetDefaults()`, so the call is emitted. If the method is missing, that's a
  compile error, not a silent no-op.
- **A message without annotations, or a foreign type** such as
  `google.protobuf.DescriptorProto`, has nothing to call. Recursion into it
  does nothing. `initialize` still creates it.

```go
// initialize + recurse
if x.Email == nil {
    x.Email = &Email{}
}
x.Email.SetDefaults()

// recurse only
if x.Profile != nil {
    x.Profile.SetDefaults()
}
```

#### Repeated and map fields

`message = {recurse: true}` is the only annotation a repeated or map field
accepts, and only when its elements (map values) are messages. It calls
`SetDefaults()` on every non-nil element, under the same rules for the target
as above. A repeated field has no single value to default and no element to
create, so a scalar default or `initialize` fails generation.

```proto
message BatchCreateBooksRequest {
  repeated CreateBookRequest requests = 1 [(protoc.contrib.defaults.value).message = {recurse: true}];
  map<string, Profile> profiles = 2 [(protoc.contrib.defaults.value).message = {recurse: true}];
}
```

```go
for _, v := range x.Requests {
    if v != nil {
        v.SetDefaults()
    }
}
```

## Applying defaults without the concrete type

Every generated `SetDefaults()`, and every hand-written one on a `skip` or
`unexported` message, satisfies `interface{ SetDefaults() }`. Code that only
holds a `proto.Message`, such as a gRPC interceptor, applies defaults with a
type assertion, without importing anything:

```go
func UnaryDefaults(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
    if d, ok := req.(interface{ SetDefaults() }); ok {
        d.SetDefaults()
    }
    return handler(ctx, req)
}
```

Messages without defaults don't implement it, so the assertion fails and
nothing happens. There is no reflection-based fallback: `dynamicpb` messages
and other types without a generated method are left untouched.

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

The test fixtures in `internal/generator/testpb` import the options from
`buf.build/protoc-contrib/protoc-types`; regenerate them with `buf generate`.

## License

[Apache 2.0](LICENCE)
