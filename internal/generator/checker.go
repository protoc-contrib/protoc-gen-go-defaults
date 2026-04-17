package generator

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/common/model"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/protoc-contrib/protoc-gen-go-defaults/defaults"
)

// parseDuration accepts the same format as prometheus/common/model.ParseDuration:
// a sequence of number+unit pairs where unit is one of y, w, d, h, m, s, ms.
// This is a superset of time.ParseDuration so expressions like "42w" work.
func parseDuration(s string) (time.Duration, error) {
	d, err := model.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	return time.Duration(d), nil
}

// parseTime attempts several RFC variants, returning the first successful
// parse. The accepted formats match the historical behavior of the plugin.
func parseTime(s string) (time.Time, error) {
	for _, format := range []string{
		time.RFC822,
		time.RFC822Z,
		time.RFC850,
		time.RFC1123,
		time.RFC1123Z,
		time.RFC3339,
	} {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("cannot parse timestamp; supported formats: RFC822 / RFC822Z / RFC850 / RFC1123 / RFC1123Z / RFC3339")
}

// check validates a single message's default-value annotations. It returns
// an error when the annotations are inconsistent with the field types, so
// protoc-gen-go-defaults fails loudly at generate time rather than producing
// code that silently does the wrong thing.
func check(msg *protogen.Message) error {
	for _, field := range msg.Fields {
		if err := checkField(field); err != nil {
			return fmt.Errorf("field %s: %w", field.Desc.Name(), err)
		}
	}
	for _, oo := range msg.Oneofs {
		if err := checkOneof(oo); err != nil {
			return fmt.Errorf("oneof %s: %w", oo.Desc.Name(), err)
		}
	}
	return nil
}

func checkOneof(oo *protogen.Oneof) error {
	name, ok := oneofDefaultField(oo)
	if !ok {
		return nil
	}
	for _, field := range oo.Fields {
		if string(field.Desc.Name()) == name {
			return nil
		}
	}
	return fmt.Errorf("(defaults.oneof) references unknown field %q", name)
}

func checkField(field *protogen.Field) error {
	fd, ok := fieldDefaults(field)
	if !ok {
		return nil
	}
	if field.Desc.IsList() || field.Desc.IsMap() {
		return fmt.Errorf("defaults are not supported on repeated or map fields")
	}

	kind := field.Desc.Kind()
	wkt := wellKnownType(field)

	switch fd.Type.(type) {
	case *defaults.FieldDefaults_Float:
		return expectKindOrWrapper(field, kind, protoreflect.FloatKind, "google.protobuf.FloatValue", wkt)
	case *defaults.FieldDefaults_Double:
		return expectKindOrWrapper(field, kind, protoreflect.DoubleKind, "google.protobuf.DoubleValue", wkt)
	case *defaults.FieldDefaults_Int32:
		return expectKindOrWrapper(field, kind, protoreflect.Int32Kind, "google.protobuf.Int32Value", wkt)
	case *defaults.FieldDefaults_Int64:
		return expectKindOrWrapper(field, kind, protoreflect.Int64Kind, "google.protobuf.Int64Value", wkt)
	case *defaults.FieldDefaults_Uint32:
		return expectKindOrWrapper(field, kind, protoreflect.Uint32Kind, "google.protobuf.UInt32Value", wkt)
	case *defaults.FieldDefaults_Uint64:
		return expectKindOrWrapper(field, kind, protoreflect.Uint64Kind, "google.protobuf.UInt64Value", wkt)
	case *defaults.FieldDefaults_Sint32:
		return expectKind(kind, protoreflect.Sint32Kind)
	case *defaults.FieldDefaults_Sint64:
		return expectKind(kind, protoreflect.Sint64Kind)
	case *defaults.FieldDefaults_Fixed32:
		return expectKind(kind, protoreflect.Fixed32Kind)
	case *defaults.FieldDefaults_Fixed64:
		return expectKind(kind, protoreflect.Fixed64Kind)
	case *defaults.FieldDefaults_Sfixed32:
		return expectKind(kind, protoreflect.Sfixed32Kind)
	case *defaults.FieldDefaults_Sfixed64:
		return expectKind(kind, protoreflect.Sfixed64Kind)
	case *defaults.FieldDefaults_Bool:
		return expectKindOrWrapper(field, kind, protoreflect.BoolKind, "google.protobuf.BoolValue", wkt)
	case *defaults.FieldDefaults_String_:
		return expectKindOrWrapper(field, kind, protoreflect.StringKind, "google.protobuf.StringValue", wkt)
	case *defaults.FieldDefaults_Bytes:
		return expectKindOrWrapper(field, kind, protoreflect.BytesKind, "google.protobuf.BytesValue", wkt)
	case *defaults.FieldDefaults_Enum:
		if kind != protoreflect.EnumKind {
			return fmt.Errorf("expected enum kind, got %s", kind)
		}
		return checkEnumValue(field, fd.GetEnum())
	case *defaults.FieldDefaults_EnumName:
		if kind != protoreflect.EnumKind {
			return fmt.Errorf("expected enum kind, got %s", kind)
		}
		if findEnumValueByName(field.Enum, fd.GetEnumName()) == nil {
			return fmt.Errorf("enum value %q is not defined in %s", fd.GetEnumName(), field.Enum.Desc.FullName())
		}
	case *defaults.FieldDefaults_Duration:
		if wkt != wktDuration {
			return fmt.Errorf("duration default requires google.protobuf.Duration, got %s", describeType(field))
		}
		if _, err := parseDuration(fd.GetDuration()); err != nil {
			return fmt.Errorf("invalid duration %q: %w", fd.GetDuration(), err)
		}
	case *defaults.FieldDefaults_Timestamp:
		if wkt != wktTimestamp {
			return fmt.Errorf("timestamp default requires google.protobuf.Timestamp, got %s", describeType(field))
		}
		v := strings.TrimSpace(fd.GetTimestamp())
		if strings.EqualFold(v, "now") {
			return nil
		}
		if _, err := parseTime(v); err != nil {
			return fmt.Errorf("invalid timestamp %q: %w", fd.GetTimestamp(), err)
		}
	case *defaults.FieldDefaults_Message:
		if kind != protoreflect.MessageKind {
			return fmt.Errorf("message default requires a message-typed field, got %s", kind)
		}
		if wkt == "google.protobuf.Any" {
			return fmt.Errorf("google.protobuf.Any fields are not supported")
		}
		if wkt == wktDuration {
			return fmt.Errorf("use (defaults.value).duration on google.protobuf.Duration fields")
		}
		if wkt == wktTimestamp {
			return fmt.Errorf("use (defaults.value).timestamp on google.protobuf.Timestamp fields")
		}
	}
	return nil
}

func expectKind(got, want protoreflect.Kind) error {
	if got != want {
		return fmt.Errorf("expected %s, got %s", want, got)
	}
	return nil
}

func expectKindOrWrapper(field *protogen.Field, got, want protoreflect.Kind, wrapper protoreflect.FullName, wkt protoreflect.FullName) error {
	if got == want {
		return nil
	}
	if got == protoreflect.MessageKind && wkt == wrapper {
		return nil
	}
	return fmt.Errorf("expected %s or %s, got %s", want, wrapper, describeType(field))
}

func wellKnownType(field *protogen.Field) protoreflect.FullName {
	if field.Desc.Kind() != protoreflect.MessageKind || field.Message == nil {
		return ""
	}
	return field.Message.Desc.FullName()
}

func describeType(field *protogen.Field) string {
	if field.Desc.Kind() == protoreflect.MessageKind && field.Message != nil {
		return string(field.Message.Desc.FullName())
	}
	return field.Desc.Kind().String()
}

func checkEnumValue(field *protogen.Field, value uint32) error {
	if field.Enum == nil {
		return fmt.Errorf("enum field has no enum descriptor")
	}
	for _, v := range field.Enum.Values {
		if uint32(v.Desc.Number()) == value {
			return nil
		}
	}
	return fmt.Errorf("enum value %d is not defined in %s", value, field.Enum.Desc.FullName())
}

func findEnumValueByName(enum *protogen.Enum, name string) *protogen.EnumValue {
	if enum == nil {
		return nil
	}
	for _, v := range enum.Values {
		if string(v.Desc.Name()) == name {
			return v
		}
	}
	return nil
}
