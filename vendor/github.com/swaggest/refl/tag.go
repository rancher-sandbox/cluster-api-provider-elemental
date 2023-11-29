package refl

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// SentinelError is a constant error.
type SentinelError string

// Error implements error.
func (se SentinelError) Error() string {
	return string(se)
}

// Sentinel errors.
const (
	ErrNeedPointer          = SentinelError("could not find field value in struct")
	ErrMissingFieldValue    = SentinelError("can not take address of structure, please pass a pointer")
	ErrMissingStructOrField = SentinelError("structPtr and fieldPtr are required")
	ErrEmptyFields          = SentinelError("empty fields")
	ErrStructExpected       = SentinelError("struct expected")
)

// HasTaggedFields checks if the structure has fields with tag name.
func HasTaggedFields(i interface{}, tagName string) bool {
	found := false

	WalkTaggedFields(reflect.ValueOf(i), func(v reflect.Value, sf reflect.StructField, tag string) {
		found = true
	}, tagName)

	return found
}

// WalkFieldFn defines callback.
type WalkFieldFn func(v reflect.Value, sf reflect.StructField, path []reflect.StructField)

// WalkFieldsRecursively walks scalar and non-scalar fields of a struct recursively and calls user function on them.
func WalkFieldsRecursively(v reflect.Value, f WalkFieldFn) {
	walkFieldsRecursively(v, f, nil)
}

func walkFieldsRecursively(v reflect.Value, f WalkFieldFn, path []reflect.StructField) {
	if v.Kind() == 0 {
		return
	}

	t := v.Type()

	for t.Kind() == reflect.Ptr {
		t = t.Elem()
		v = v.Elem()
	}

	if t.Kind() != reflect.Struct {
		return
	}

	for i := 0; i < t.NumField(); i++ {
		var (
			field    = t.Field(i)
			fieldVal reflect.Value
		)

		if v.IsValid() {
			fieldVal = v.Field(i)
		} else {
			fieldVal = reflect.Zero(field.Type)
		}

		if fieldVal.CanAddr() {
			fieldVal = fieldVal.Addr()
		}

		// Don't traverse unexported non-anonymous fields.
		if field.PkgPath != "" && !field.Anonymous {
			continue
		}

		f(fieldVal, field, path)

		if len(path) > 100 {
			pp := ""
			for _, p := range path[0:10] {
				pp += "." + p.Name
			}

			panic("too deep recursion, possible cyclic reference: " + pp)
		}

		walkFieldsRecursively(fieldVal, f, append(path, field))
	}
}

// WalkTaggedFieldFn defines callback.
type WalkTaggedFieldFn func(v reflect.Value, sf reflect.StructField, tag string)

// WalkTaggedFields iterates top level fields of structure including anonymous embedded fields.
// If tagName is empty function is called for all top level fields.
func WalkTaggedFields(v reflect.Value, f WalkTaggedFieldFn, tagName string) {
	if v.Kind() == 0 {
		return
	}

	t := v.Type()

	for t.Kind() == reflect.Ptr {
		t = t.Elem()
		v = v.Elem()
	}

	if t.Kind() != reflect.Struct {
		return
	}

	for i := 0; i < t.NumField(); i++ {
		var (
			field    = t.Field(i)
			fieldVal reflect.Value
		)

		if v.IsValid() {
			fieldVal = v.Field(i)
		} else {
			fieldVal = reflect.Zero(field.Type)
		}

		tag := field.Tag.Get(tagName)
		tag = strings.Split(tag, ",")[0]

		if field.Anonymous {
			if tag != "-" {
				if fieldVal.CanAddr() {
					fieldVal = fieldVal.Addr()
				}

				WalkTaggedFields(fieldVal, f, tagName)
			}

			continue
		}

		if tagName != "" && (tag == "" || tag == "-") {
			continue
		}

		f(fieldVal, field, tag)
	}
}

// ReadBoolTag reads bool value from field tag into a value.
func ReadBoolTag(tag reflect.StructTag, name string, holder *bool) error {
	value, ok := tag.Lookup(name)
	if ok {
		v, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("failed to parse bool value %s in tag %s: %w", value, name, err)
		}

		*holder = v
	}

	return nil
}

// ReadBoolPtrTag reads bool value from field tag into a pointer.
func ReadBoolPtrTag(tag reflect.StructTag, name string, holder **bool) error {
	value, ok := tag.Lookup(name)
	if ok {
		v, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("failed to parse bool value %s in tag %s: %w", value, name, err)
		}

		*holder = &v
	}

	return nil
}

// ReadStringTag reads string value from field tag into a value.
func ReadStringTag(tag reflect.StructTag, name string, holder *string) {
	if holder == nil {
		return
	}

	value, ok := tag.Lookup(name)
	if ok {
		if *holder != "" && value == "-" {
			*holder = ""

			return
		}

		*holder = value
	}
}

// ReadStringPtrTag reads string value from field tag into a pointer.
func ReadStringPtrTag(tag reflect.StructTag, name string, holder **string) {
	value, ok := tag.Lookup(name)
	if ok {
		if *holder != nil && **holder != "" && value == "-" {
			*holder = nil

			return
		}

		*holder = &value
	}
}

// ReadIntTag reads int64 value from field tag into a value.
func ReadIntTag(tag reflect.StructTag, name string, holder *int64) error {
	value, ok := tag.Lookup(name)
	if ok {
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse int value %s in tag %s: %w", value, name, err)
		}

		*holder = v
	}

	return nil
}

// ReadIntPtrTag reads int64 value from field tag into a pointer.
func ReadIntPtrTag(tag reflect.StructTag, name string, holder **int64) error {
	value, ok := tag.Lookup(name)
	if ok {
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse int value %s in tag %s: %w", value, name, err)
		}

		*holder = &v
	}

	return nil
}

// ReadFloatTag reads float64 value from field tag into a value.
func ReadFloatTag(tag reflect.StructTag, name string, holder *float64) error {
	value, ok := tag.Lookup(name)
	if ok {
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("failed to parse float value %s in tag %s: %w", value, name, err)
		}

		*holder = v
	}

	return nil
}

// ReadFloatPtrTag reads float64 value from field tag into a pointer.
func ReadFloatPtrTag(tag reflect.StructTag, name string, holder **float64) error {
	value, ok := tag.Lookup(name)
	if ok {
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("failed to parse float value %s in tag %s: %w", value, name, err)
		}

		*holder = &v
	}

	return nil
}

// JoinErrors joins non-nil errors.
func JoinErrors(errs ...error) error {
	join := ""

	for _, err := range errs {
		if err != nil {
			join += ", " + err.Error()
		}
	}

	if join != "" {
		return errors.New(join[2:]) //nolint:goerr113
	}

	return nil
}

// PopulateFieldsFromTags extracts values from field tag and puts them in according property of structPtr.
func PopulateFieldsFromTags(structPtr interface{}, fieldTag reflect.StructTag) error {
	pv := reflect.ValueOf(structPtr).Elem()
	pt := pv.Type()

	var errs []error

	for i := 0; i < pv.NumField(); i++ {
		ptf := pt.Field(i)
		tagName := strings.ToLower(ptf.Name[0:1]) + ptf.Name[1:]
		pvf := pv.Field(i).Addr().Interface()

		var err error

		switch v := pvf.(type) {
		case **string:
			ReadStringPtrTag(fieldTag, tagName, v)
		case *string:
			ReadStringTag(fieldTag, tagName, v)
		case **int64:
			err = ReadIntPtrTag(fieldTag, tagName, v)
		case *int64:
			err = ReadIntTag(fieldTag, tagName, v)
		case **float64:
			err = ReadFloatPtrTag(fieldTag, tagName, v)
		case *float64:
			err = ReadFloatTag(fieldTag, tagName, v)
		case **bool:
			err = ReadBoolPtrTag(fieldTag, tagName, v)
		case *bool:
			err = ReadBoolTag(fieldTag, tagName, v)
		}

		if err != nil {
			errs = append(errs, err)
		}
	}

	return JoinErrors(errs...)
}

// FindTaggedName returns tagged name of an entity field.
//
// Entity field is defined by pointer to owner structure and pointer to field in that structure.
//
//	entity := MyEntity{}
//	name, found := sm.FindTaggedName(&entity, &entity.UpdatedAt, "db")
func FindTaggedName(structPtr, fieldPtr interface{}, tagName string) (string, error) {
	if structPtr == nil || fieldPtr == nil {
		return "", ErrMissingStructOrField
	}

	v := reflect.Indirect(reflect.ValueOf(structPtr))

	if !v.CanAddr() {
		return "", ErrNeedPointer
	}

	found := false
	name := ""

	unsafeAddr := reflect.ValueOf(fieldPtr).Elem().UnsafeAddr()

	WalkTaggedFields(v, func(v reflect.Value, sf reflect.StructField, tag string) {
		if found {
			return
		}

		if v.UnsafeAddr() == unsafeAddr {
			name = tag
			found = true
		}
	}, tagName)

	if found {
		return name, nil
	}

	return "", ErrMissingFieldValue
}

// Tagged will try to find tagged name and panic on error.
func Tagged(structPtr, fieldPtr interface{}, tagName string) string {
	name, err := FindTaggedName(structPtr, fieldPtr, tagName)
	if err != nil {
		panic(err)
	}

	return name
}
