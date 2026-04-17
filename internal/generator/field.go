package generator

import (
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/protoc-contrib/protoc-gen-go-defaults/defaults"
)

const (
	wktDuration  protoreflect.FullName = "google.protobuf.Duration"
	wktTimestamp protoreflect.FullName = "google.protobuf.Timestamp"
)

var wrapperValueTypes = map[protoreflect.FullName]string{
	"google.protobuf.DoubleValue": "DoubleValue",
	"google.protobuf.FloatValue":  "FloatValue",
	"google.protobuf.Int64Value":  "Int64Value",
	"google.protobuf.UInt64Value": "UInt64Value",
	"google.protobuf.Int32Value":  "Int32Value",
	"google.protobuf.UInt32Value": "UInt32Value",
	"google.protobuf.BoolValue":   "BoolValue",
	"google.protobuf.StringValue": "StringValue",
	"google.protobuf.BytesValue":  "BytesValue",
}

var (
	durationpbIdent = protogen.GoImportPath("google.golang.org/protobuf/types/known/durationpb").Ident("Duration")
	durationpbNew   = protogen.GoImportPath("google.golang.org/protobuf/types/known/durationpb").Ident("New")
	timestamppbNew  = protogen.GoImportPath("google.golang.org/protobuf/types/known/timestamppb").Ident("Now")
	timestamppbTS   = protogen.GoImportPath("google.golang.org/protobuf/types/known/timestamppb").Ident("Timestamp")
	wrapperspbPath  = protogen.GoImportPath("google.golang.org/protobuf/types/known/wrapperspb")
)

// emitField writes the defaults code for a single field. For oneof fields,
// the whole oneof block is emitted the first time any of its members is
// encountered; subsequent fields in the same oneof are skipped.
func emitField(g *protogen.GeneratedFile, msg *protogen.Message, field *protogen.Field, seenOneofs map[protoreflect.FullName]bool) {
	if oo := field.Oneof; oo != nil && !oo.Desc.IsSynthetic() {
		if seenOneofs[oo.Desc.FullName()] {
			return
		}
		seenOneofs[oo.Desc.FullName()] = true
		emitOneof(g, msg, oo)
		return
	}

	fd, ok := fieldDefaults(field)
	if !ok {
		return
	}

	emitScalarOrMessage(g, field, fd)
}

func emitOneof(g *protogen.GeneratedFile, msg *protogen.Message, oo *protogen.Oneof) {
	oneofGoName := oo.GoName
	defaultName, hasDefault := oneofDefaultField(oo)

	var withDefaults []*protogen.Field
	for _, field := range oo.Fields {
		if _, ok := fieldDefaults(field); ok {
			withDefaults = append(withDefaults, field)
		}
	}
	if !hasDefault && len(withDefaults) == 0 {
		return
	}

	if hasDefault {
		for _, field := range oo.Fields {
			if string(field.Desc.Name()) != defaultName {
				continue
			}
			wrapper := oneofWrapperIdent(msg, field)
			g.P("if x.", oneofGoName, " == nil {")
			g.P("x.", oneofGoName, " = &", wrapper, "{}")
			g.P("}")
			break
		}
	}

	if len(withDefaults) == 0 {
		return
	}

	g.P("switch x := x.", oneofGoName, ".(type) {")
	for _, field := range withDefaults {
		fd, _ := fieldDefaults(field)
		wrapper := oneofWrapperIdent(msg, field)
		g.P("case *", wrapper, ":")
		emitScalarOrMessage(g, field, fd)
	}
	g.P("}")
}

func oneofWrapperIdent(msg *protogen.Message, field *protogen.Field) protogen.GoIdent {
	return protogen.GoIdent{
		GoName:       msg.GoIdent.GoName + "_" + field.GoName,
		GoImportPath: msg.GoIdent.GoImportPath,
	}
}

// emitScalarOrMessage handles a single field's default emission. Must not be
// called for oneof fields with a containing oneof (use emitOneof instead).
func emitScalarOrMessage(g *protogen.GeneratedFile, field *protogen.Field, fd *defaults.FieldDefaults) {
	name := field.GoName
	msgKind := field.Desc.Kind() == protoreflect.MessageKind
	var wktName protoreflect.FullName
	if msgKind && field.Message != nil {
		wktName = field.Message.Desc.FullName()
	}

	switch v := fd.Type.(type) {
	case *defaults.FieldDefaults_Float:
		emitSimple(g, field, fmt.Sprintf("%v", v.Float), wktName)
	case *defaults.FieldDefaults_Double:
		emitSimple(g, field, fmt.Sprintf("%v", v.Double), wktName)
	case *defaults.FieldDefaults_Int32:
		emitSimple(g, field, strconv.FormatInt(int64(v.Int32), 10), wktName)
	case *defaults.FieldDefaults_Int64:
		emitSimple(g, field, strconv.FormatInt(v.Int64, 10), wktName)
	case *defaults.FieldDefaults_Uint32:
		emitSimple(g, field, strconv.FormatUint(uint64(v.Uint32), 10), wktName)
	case *defaults.FieldDefaults_Uint64:
		emitSimple(g, field, strconv.FormatUint(v.Uint64, 10), wktName)
	case *defaults.FieldDefaults_Sint32:
		emitSimple(g, field, strconv.FormatInt(int64(v.Sint32), 10), wktName)
	case *defaults.FieldDefaults_Sint64:
		emitSimple(g, field, strconv.FormatInt(v.Sint64, 10), wktName)
	case *defaults.FieldDefaults_Fixed32:
		emitSimple(g, field, strconv.FormatUint(uint64(v.Fixed32), 10), wktName)
	case *defaults.FieldDefaults_Fixed64:
		emitSimple(g, field, strconv.FormatUint(v.Fixed64, 10), wktName)
	case *defaults.FieldDefaults_Sfixed32:
		emitSimple(g, field, strconv.FormatInt(int64(v.Sfixed32), 10), wktName)
	case *defaults.FieldDefaults_Sfixed64:
		emitSimple(g, field, strconv.FormatInt(v.Sfixed64, 10), wktName)
	case *defaults.FieldDefaults_Bool:
		emitSimple(g, field, strconv.FormatBool(v.Bool), wktName)
	case *defaults.FieldDefaults_String_:
		emitSimple(g, field, strconv.Quote(v.String_), wktName)
	case *defaults.FieldDefaults_Bytes:
		emitBytes(g, field, v.Bytes, wktName)
	case *defaults.FieldDefaults_Enum:
		emitSimple(g, field, strconv.FormatUint(uint64(v.Enum), 10), wktName)
	case *defaults.FieldDefaults_EnumName:
		emitEnumName(g, field, findEnumValueByName(field.Enum, v.EnumName))
	case *defaults.FieldDefaults_Duration:
		emitDuration(g, field, v.Duration)
	case *defaults.FieldDefaults_Timestamp:
		emitTimestamp(g, field, v.Timestamp)
	case *defaults.FieldDefaults_Message:
		emitMessageField(g, field, v.Message, name)
	}
}

// emitSimple emits the basic "set when zero" assignment for scalar and
// wrapper-typed fields.
func emitSimple(g *protogen.GeneratedFile, field *protogen.Field, value string, wktName protoreflect.FullName) {
	name := field.GoName
	if wkt, ok := wrapperValueTypes[wktName]; ok {
		val := value
		if wkt == "BytesValue" {
			val = "[]byte(" + value + ")"
		}
		g.P("if x.", name, " == nil {")
		g.P("x.", name, " = &", wrapperspbPath.Ident(wkt), "{Value: ", val, "}")
		g.P("}")
		return
	}

	if field.Desc.HasOptionalKeyword() {
		goType := scalarGoType(field)
		g.P("if x.", name, " == nil {")
		g.P("v := ", goType, "(", value, ")")
		g.P("x.", name, " = &v")
		g.P("}")
		return
	}

	zero := scalarZero(field)
	g.P("if x.", name, " == ", zero, " {")
	g.P("x.", name, " = ", value)
	g.P("}")
}

func emitEnumName(g *protogen.GeneratedFile, field *protogen.Field, enumValue *protogen.EnumValue) {
	name := field.GoName
	if field.Desc.HasOptionalKeyword() {
		g.P("if x.", name, " == nil {")
		g.P("v := ", enumValue.GoIdent)
		g.P("x.", name, " = &v")
		g.P("}")
		return
	}
	zero := scalarZero(field)
	g.P("if x.", name, " == ", zero, " {")
	g.P("x.", name, " = ", enumValue.GoIdent)
	g.P("}")
}

func emitBytes(g *protogen.GeneratedFile, field *protogen.Field, value []byte, wktName protoreflect.FullName) {
	name := field.GoName
	lit := strconv.Quote(string(value))
	if wktName == "google.protobuf.BytesValue" {
		g.P("if x.", name, " == nil {")
		g.P("x.", name, " = &", wrapperspbPath.Ident("BytesValue"), "{Value: []byte(", lit, ")}")
		g.P("}")
		return
	}
	g.P("if len(x.", name, ") == 0 {")
	g.P("x.", name, " = []byte(", lit, ")")
	g.P("}")
}

func emitDuration(g *protogen.GeneratedFile, field *protogen.Field, raw string) {
	d, err := parseDuration(raw)
	if err != nil {
		// checker.go should have caught this; if we get here, fall back to a
		// compile-time error so the user notices.
		g.P("var _ = ", durationpbIdent, "{} // invalid duration: ", strconv.Quote(raw))
		return
	}
	g.P("if x.", field.GoName, " == nil {")
	g.P("x.", field.GoName, " = ", durationpbNew, "(", strconv.FormatInt(int64(d), 10), ")")
	g.P("}")
}

func emitTimestamp(g *protogen.GeneratedFile, field *protogen.Field, raw string) {
	v := strings.TrimSpace(raw)
	if strings.EqualFold(v, "now") {
		g.P("if x.", field.GoName, " == nil {")
		g.P("x.", field.GoName, " = ", timestamppbNew, "()")
		g.P("}")
		return
	}
	t, err := parseTime(v)
	if err != nil {
		g.P("var _ = ", timestamppbTS, "{} // invalid timestamp: ", strconv.Quote(raw))
		return
	}
	g.P("if x.", field.GoName, " == nil {")
	g.P("x.", field.GoName, " = &", timestamppbTS, "{Seconds: ", strconv.FormatInt(t.Unix(), 10), ", Nanos: ", strconv.Itoa(t.Nanosecond()), "}")
	g.P("}")
}

func emitMessageField(g *protogen.GeneratedFile, field *protogen.Field, md *defaults.MessageDefaults, name string) {
	if md.GetInitialize() {
		g.P("if x.", name, " == nil {")
		g.P("x.", name, " = &", field.Message.GoIdent, "{}")
		g.P("}")
	}
	if !md.GetRecurse() {
		return
	}
	g.P("if v, ok := interface{}(x.", name, ").(interface{ SetDefaults() }); ok && x.", name, " != nil {")
	g.P("v.SetDefaults()")
	g.P("}")
}

func scalarGoType(field *protogen.Field) string {
	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		return "bool"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return "int32"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return "int64"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return "uint32"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "uint64"
	case protoreflect.FloatKind:
		return "float32"
	case protoreflect.DoubleKind:
		return "float64"
	case protoreflect.StringKind:
		return "string"
	case protoreflect.EnumKind:
		if field.Enum != nil {
			return string(field.Enum.GoIdent.GoName)
		}
	}
	return ""
}

func scalarZero(field *protogen.Field) string {
	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		return "false"
	case protoreflect.StringKind:
		return `""`
	case protoreflect.BytesKind:
		return "nil"
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return "nil"
	default:
		return "0"
	}
}
