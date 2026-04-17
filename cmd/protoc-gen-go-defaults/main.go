// Command protoc-gen-go-defaults is a protoc plugin that emits Default()
// methods for proto messages annotated with the defaults.* extensions.
package main

import (
	"context"
	"log/slog"
	"runtime/debug"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"

	"github.com/protoc-extensions/protoc-gen-go-defaults/internal/generator"
)

// version is set at build time via ldflags (e.g. -X main.version=0.1.0).
// When empty, the value falls back to Go module build info.
var version string

func main() {
	ctx := context.Background()

	opts := protogen.Options{}
	opts.Run(func(plugin *protogen.Plugin) error {
		plugin.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
		if v := resolvedVersion(); v != "" {
			slog.DebugContext(ctx, "protoc-gen-go-defaults", slog.String("version", v))
		}
		return generator.Generate(plugin)
	})
}

func resolvedVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		return info.Main.Version
	}
	return ""
}
