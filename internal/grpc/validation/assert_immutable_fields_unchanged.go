package validation

import (
	"reflect"
	"strings"

	"go.datum.net/iam/internal/validation/field"

	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

func AssertImmutableFieldsUnchanged(updateMaskPaths []string, existing proto.Message, updated proto.Message) field.ErrorList {
	errs := field.ErrorList{}

	for _, path := range updateMaskPaths {
		originalValue := getFieldValue(path, existing)
		updatedValue := getFieldValue(path, updated)
		isFieldImmutable := isFieldImmutable(path, existing)

		if !reflect.DeepEqual(originalValue, updatedValue) && isFieldImmutable {
			errs = append(errs, field.Invalid(field.NewPath(path), updatedValue, "immutable field has been modified"))
		}
	}

	return errs
}

func isFieldImmutable(path string, message proto.Message) bool {
	components := strings.Split(path, ".")
	current := message.ProtoReflect()

	for i, component := range components {
		field := current.Descriptor().Fields().ByName(protoreflect.Name(component))

		value := current.Get(field)

		if i == len(components)-1 {
			opts := field.Options().(*descriptorpb.FieldOptions)
			fieldBehaviors := proto.GetExtension(opts, annotations.E_FieldBehavior).([]annotations.FieldBehavior)

			// Check if the field has the IMMUTABLE behavior
			isImmutable := false
			for _, behavior := range fieldBehaviors {
				if behavior == annotations.FieldBehavior_IMMUTABLE {
					isImmutable = true
					break
				}
			}

			return isImmutable
		}

		current = value.Message()
	}

	return false
}
