package generator_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"

	"github.com/protoc-contrib/protoc-gen-go-defaults/defaults"
	"github.com/protoc-contrib/protoc-gen-go-defaults/internal/generator"
	"github.com/protoc-contrib/protoc-gen-go-defaults/internal/generator/testpb"
)

// runGenerator builds a CodeGeneratorRequest that asks the plugin to generate
// the given file descriptors (and nothing else), runs the generator, and
// returns the resulting response.
func runGenerator(files ...protoreflect.FileDescriptor) (*pluginpb.CodeGeneratorResponse, error) {
	req := &pluginpb.CodeGeneratorRequest{}
	seen := map[string]bool{}
	var walk func(fd protoreflect.FileDescriptor)
	walk = func(fd protoreflect.FileDescriptor) {
		if seen[fd.Path()] {
			return
		}
		seen[fd.Path()] = true
		imports := fd.Imports()
		for i := 0; i < imports.Len(); i++ {
			walk(imports.Get(i).FileDescriptor)
		}
		req.ProtoFile = append(req.ProtoFile, protodesc.ToFileDescriptorProto(fd))
	}
	for _, fd := range files {
		walk(fd)
		req.FileToGenerate = append(req.FileToGenerate, fd.Path())
	}
	plugin, err := protogen.Options{}.New(req)
	if err != nil {
		return nil, err
	}
	if err := generator.Generate(plugin); err != nil {
		return nil, err
	}
	return plugin.Response(), nil
}

func emittedContent(resp *pluginpb.CodeGeneratorResponse) string {
	var sb strings.Builder
	for _, f := range resp.File {
		sb.WriteString(f.GetContent())
		sb.WriteString("\n")
	}
	return sb.String()
}

var _ = Describe("generator.Generate", func() {
	It("emits Default methods for every non-ignored message in the fixtures", func() {
		resp, err := runGenerator(testpb.File_tests_pb_test_proto, testpb.File_tests_pb_types_proto)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Error).To(BeNil())
		Expect(resp.File).NotTo(BeEmpty())

		out := emittedContent(resp)
		Expect(out).To(ContainSubstring("func (x *Test) Default()"))
		Expect(out).To(ContainSubstring("func (x *TestOptional) Default()"))
		Expect(out).To(ContainSubstring("func (x *TestUnexported) _Default()"))
		Expect(out).To(ContainSubstring("func (x *Types) Default()"))
		Expect(out).To(ContainSubstring("func (x *OneOfThree) Default()"))
		// OneOfOne is annotated (defaults.ignored) and must not appear.
		Expect(out).NotTo(ContainSubstring("func (x *OneOfOne) Default()"))
	})

	It("handles wrappers, durations, and timestamps", func() {
		resp, err := runGenerator(testpb.File_tests_pb_test_proto)
		Expect(err).NotTo(HaveOccurred())
		out := emittedContent(resp)

		Expect(out).To(ContainSubstring("wrapperspb.Int64Value"))
		Expect(out).To(ContainSubstring("wrapperspb.StringValue"))
		Expect(out).To(ContainSubstring("wrapperspb.BoolValue"))
		Expect(out).To(ContainSubstring("durationpb.New"))
		Expect(out).To(ContainSubstring("timestamppb.Now"))
		Expect(out).To(ContainSubstring("timestamppb.Timestamp"))
	})

	It("selects the declared oneof arm when (defaults.oneof) is set", func() {
		resp, err := runGenerator(testpb.File_tests_pb_test_proto)
		Expect(err).NotTo(HaveOccurred())
		out := emittedContent(resp)

		Expect(out).To(ContainSubstring("x.Oneof = &Test_Two{}"))
		Expect(out).To(ContainSubstring("switch x := x.Oneof.(type)"))
	})

	Describe("error paths", func() {
		It("rejects duration defaults on non-Duration fields", func() {
			file := cloneFixture(testpb.File_tests_pb_types_proto)
			setFieldDefault(file, "Types", "string", &defaults.FieldDefaults{
				Type: &defaults.FieldDefaults_Duration{Duration: "1h"},
			})
			_, err := runGeneratorFromProto(file, deps(testpb.File_tests_pb_types_proto)...)
			Expect(err).To(MatchError(ContainSubstring("duration default requires google.protobuf.Duration")))
		})

		It("rejects an (defaults.oneof) that does not match any field", func() {
			file := cloneFixture(testpb.File_tests_pb_types_proto)
			setOneofDefault(file, "Types", "oneof", "missing")
			_, err := runGeneratorFromProto(file, deps(testpb.File_tests_pb_types_proto)...)
			Expect(err).To(MatchError(ContainSubstring("(defaults.oneof) references unknown field")))
		})

		It("rejects an enum default that is not a declared value", func() {
			file := cloneFixture(testpb.File_tests_pb_types_proto)
			setFieldDefault(file, "Types", "enum", &defaults.FieldDefaults{
				Type: &defaults.FieldDefaults_Enum{Enum: 99},
			})
			_, err := runGeneratorFromProto(file, deps(testpb.File_tests_pb_types_proto)...)
			Expect(err).To(MatchError(ContainSubstring("enum value 99 is not defined")))
		})
	})
})

// cloneFixture returns a deep copy of fd.File() as a FileDescriptorProto so
// tests can mutate field options without affecting the shared registry.
func cloneFixture(fd protoreflect.FileDescriptor) *descriptorpb.FileDescriptorProto {
	return proto.Clone(protodesc.ToFileDescriptorProto(fd)).(*descriptorpb.FileDescriptorProto)
}

// deps returns every transitive FileDescriptorProto for fd (excluding fd
// itself), suitable for appending to a CodeGeneratorRequest.
func deps(fd protoreflect.FileDescriptor) []*descriptorpb.FileDescriptorProto {
	var out []*descriptorpb.FileDescriptorProto
	seen := map[string]bool{fd.Path(): true}
	var walk func(f protoreflect.FileDescriptor)
	walk = func(f protoreflect.FileDescriptor) {
		imports := f.Imports()
		for i := 0; i < imports.Len(); i++ {
			child := imports.Get(i).FileDescriptor
			if seen[child.Path()] {
				continue
			}
			seen[child.Path()] = true
			walk(child)
			out = append(out, protodesc.ToFileDescriptorProto(child))
		}
	}
	walk(fd)
	return out
}

func runGeneratorFromProto(target *descriptorpb.FileDescriptorProto, extra ...*descriptorpb.FileDescriptorProto) (*pluginpb.CodeGeneratorResponse, error) {
	req := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{target.GetName()},
	}
	req.ProtoFile = append(req.ProtoFile, extra...)
	req.ProtoFile = append(req.ProtoFile, target)
	plugin, err := protogen.Options{}.New(req)
	if err != nil {
		return nil, err
	}
	if err := generator.Generate(plugin); err != nil {
		return nil, err
	}
	return plugin.Response(), nil
}

func setFieldDefault(file *descriptorpb.FileDescriptorProto, msgName, fieldName string, v *defaults.FieldDefaults) {
	msg := findMessage(file, msgName)
	Expect(msg).NotTo(BeNil(), "message %s not found", msgName)
	field := findField(msg, fieldName)
	Expect(field).NotTo(BeNil(), "field %s not found in %s", fieldName, msgName)
	if field.Options == nil {
		field.Options = &descriptorpb.FieldOptions{}
	}
	proto.SetExtension(field.Options, defaults.E_Value, v)
}

func setOneofDefault(file *descriptorpb.FileDescriptorProto, msgName, oneofName, target string) {
	msg := findMessage(file, msgName)
	Expect(msg).NotTo(BeNil(), "message %s not found", msgName)
	for _, oo := range msg.OneofDecl {
		if oo.GetName() != oneofName {
			continue
		}
		if oo.Options == nil {
			oo.Options = &descriptorpb.OneofOptions{}
		}
		proto.SetExtension(oo.Options, defaults.E_Oneof, target)
		return
	}
	Fail("oneof " + oneofName + " not found in " + msgName)
}

func findMessage(file *descriptorpb.FileDescriptorProto, name string) *descriptorpb.DescriptorProto {
	for _, m := range file.MessageType {
		if m.GetName() == name {
			return m
		}
	}
	return nil
}

func findField(msg *descriptorpb.DescriptorProto, name string) *descriptorpb.FieldDescriptorProto {
	for _, f := range msg.Field {
		if f.GetName() == name {
			return f
		}
	}
	return nil
}
