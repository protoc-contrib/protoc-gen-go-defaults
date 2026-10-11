package generator_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/protoc-contrib/protoc-gen-go-defaults/internal/generator/testpb"
)

// expectedTest is the message the fixtures in pb/test.proto should produce
// when SetDefaults() is invoked on a zero value.
func expectedTest() *testpb.Test {
	return &testpb.Test{
		StringField:               "string_field",
		NumberField:               42,
		BoolField:                 true,
		EnumField:                 2,
		NumberValueField:          wrapperspb.Int64(43),
		StringValueField:          wrapperspb.String("string_value"),
		BoolValueField:            wrapperspb.Bool(false),
		DurationValueField:        durationpb.New(25401600000000000),
		Oneof:                     &testpb.Test_Two{Two: &testpb.OneOfTwo{StringField: "string_field"}},
		Descriptor_:               &descriptorpb.DescriptorProto{},
		TimeValueFieldWithDefault: &timestamppb.Timestamp{Seconds: -562032000},
		Bytes:                     []byte("??"),
	}
}

var _ = Describe("SetDefaults()", func() {
	It("populates scalars, wrappers, and the default oneof arm", func() {
		msg := &testpb.Test{}
		msg.SetDefaults()

		Expect(msg.TimeValueField).NotTo(BeNil())
		now := timestamppb.Now()
		Expect(msg.TimeValueField.Seconds).To(BeNumerically("~", now.Seconds, 1))
		msg.TimeValueField = nil
		Expect(msg).To(Equal(expectedTest()))
	})

	It("skips generation for messages with no declared defaults", func() {
		_, generated := interface{}(&testpb.NoDefaults{}).(interface{ SetDefaults() })
		Expect(generated).To(BeFalse())
	})

	Describe("recursion", func() {
		It("recurses into a set message field", func() {
			msg := &testpb.Test{MessageField: &testpb.Test{}}
			msg.SetDefaults()
			Expect(msg.MessageField.StringField).To(Equal("string_field"))
			Expect(msg.MessageField.NumberField).To(Equal(int64(42)))
		})

		It("leaves an unset recurse-only message field nil", func() {
			msg := &testpb.Test{}
			msg.SetDefaults()
			Expect(msg.MessageField).To(BeNil())
		})

		It("calls the hand-written SetDefaults of a (defaults.skip) message", func() {
			msg := &testpb.Test{Oneof: &testpb.Test_One{}}
			msg.SetDefaults()
			Expect(msg.GetOne().GetStringField()).To(Equal("hand-written"))
		})

		It("creates and fills the default oneof arm", func() {
			msg := &testpb.Types{}
			msg.SetDefaults()
			Expect(msg.GetTwo().GetStringField()).To(Equal("string_field"))
		})

		It("recurses into every non-nil element of a repeated field", func() {
			msg := &testpb.Test{RepeatedTestField: []*testpb.Test{{}, nil, {StringField: "set"}}}
			msg.SetDefaults()
			Expect(msg.RepeatedTestField[0].StringField).To(Equal("string_field"))
			Expect(msg.RepeatedTestField[1]).To(BeNil())
			Expect(msg.RepeatedTestField[2].StringField).To(Equal("set"))
			Expect(msg.RepeatedTestField[2].NumberField).To(Equal(int64(42)))
		})

		It("recurses into every non-nil value of a map field", func() {
			msg := &testpb.Test{MapOneField: map[string]*testpb.OneOfOne{"set": {}, "nil": nil}}
			msg.SetDefaults()
			Expect(msg.MapOneField["set"].GetStringField()).To(Equal("hand-written"))
			Expect(msg.MapOneField["nil"]).To(BeNil())
		})

		It("initializes a foreign message without recursing into it", func() {
			msg := &testpb.Test{}
			msg.SetDefaults()
			Expect(proto.Equal(msg.Descriptor_, &descriptorpb.DescriptorProto{})).To(BeTrue())
		})
	})

	It("routes unexported messages through _SetDefaults() exposed via a wrapper", func() {
		msg := &testpb.TestUnexported{}
		msg.SetDefaults()
		Expect(msg.StringField).To(HaveValue(Equal("string_field")))
		Expect(msg.NumberField).To(HaveValue(Equal(int64(42))))
	})

	Describe("Types message", func() {
		It("applies every scalar default including fixed64 (regression for GetFixed32/GetFixed64 bug)", func() {
			t := &testpb.Types{}
			t.SetDefaults()

			Expect(t.Float).To(BeNumerically("~", float32(0.42), 1e-6))
			Expect(t.Double).To(BeNumerically("~", 0.42, 1e-9))
			Expect(t.Int32).To(Equal(int32(42)))
			Expect(t.Int64).To(Equal(int64(42)))
			Expect(t.Uint32).To(Equal(uint32(42)))
			Expect(t.Uint64).To(Equal(uint64(42)))
			Expect(t.Sint32).To(Equal(int32(42)))
			Expect(t.Sint64).To(Equal(int64(42)))
			Expect(t.Fixed32).To(Equal(uint32(42)))
			Expect(t.Fixed64).To(Equal(uint64(42)))
			Expect(t.Sfixed32).To(Equal(int32(42)))
			Expect(t.Sfixed64).To(Equal(int64(42)))
			Expect(t.Bool).To(BeTrue())
			Expect(t.String_).To(Equal("42"))
			Expect(t.Bytes).To(Equal([]byte("42")))
			Expect(t.Enum).To(Equal(testpb.Types_ONE))
			Expect(t.EnumByName).To(Equal(testpb.Types_TWO))
		})

		It("does not overwrite fields that are already set", func() {
			t := &testpb.Types{Int32: 7, Bool: false, String_: "already"}
			t.SetDefaults()
			Expect(t.Int32).To(Equal(int32(7)))
			Expect(t.String_).To(Equal("already"))
			// Bool is false (zero), so the default (true) still applies.
			Expect(t.Bool).To(BeTrue())
		})

		It("applies wrapper and well-known type defaults", func() {
			t := &testpb.Types{}
			t.SetDefaults()

			Expect(t.DoubleValue.GetValue()).To(BeNumerically("~", 0.42, 1e-9))
			Expect(t.FloatValue.GetValue()).To(BeNumerically("~", float32(0.42), 1e-6))
			Expect(t.Int32Value.GetValue()).To(Equal(int32(42)))
			Expect(t.Int64Value.GetValue()).To(Equal(int64(42)))
			Expect(t.Uint32Value.GetValue()).To(Equal(uint32(42)))
			Expect(t.Uint64Value.GetValue()).To(Equal(uint64(42)))
			Expect(t.BoolValue).NotTo(BeNil())
			Expect(t.BoolValue.GetValue()).To(BeFalse())
			Expect(t.StringValue.GetValue()).To(Equal("42"))
			Expect(t.BytesValue.GetValue()).To(Equal([]byte("42")))

			Expect(t.Duration).NotTo(BeNil())
			Expect(t.Duration.AsDuration().Hours()).To(BeNumerically("~", 48.0, 1e-6))
			Expect(t.Timestamp).NotTo(BeNil())
			Expect(t.Message).NotTo(BeNil())
		})
	})

	It("respects proto3 optional presence", func() {
		enum := testpb.TestOptional_TWO
		want := &testpb.TestOptional{
			StringField: proto.String("string_field"),
			NumberField: proto.Int64(42),
			BoolField:   proto.Bool(true),
			EnumField:   &enum,
		}
		msg := &testpb.TestOptional{}
		msg.SetDefaults()
		Expect(proto.Equal(msg, want)).To(BeTrue())

		// A caller-provided value must not be overwritten.
		want.StringField = proto.String("other")
		msg = &testpb.TestOptional{StringField: proto.String("other")}
		msg.SetDefaults()
		Expect(proto.Equal(msg, want)).To(BeTrue())
	})
})
