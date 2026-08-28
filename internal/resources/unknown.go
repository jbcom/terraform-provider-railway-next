// SPDX-License-Identifier: MPL-2.0

package resources

import (
	"reflect"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// ResolveUnknowns replaces every still-unknown attribute on a resource model
// with a known null, leaving configured and computed values untouched.
//
// **WHY THIS IS SHARED RATHER THAN PER-RESOURCE.** Terraform rejects an apply
// result containing an unknown:
//
//	Provider returned invalid result object after apply … the provider still
//	indicated an unknown value for … .builder
//
// On the happy path a resource's refresh resolves those from the API. On a
// PARTIAL FAILURE path — the object was created but a later step failed — the
// refresh has not run, so anything Optional+Computed the plan left unknown is
// still unknown. Persisting that trades an orphaned resource for a protocol
// error, which is not a fix.
//
// Every resource here has that shape, so the first version of this was a
// hand-written list of 26 `IsUnknown()` checks in `service.go` — and it missed
// four fields on the first attempt, including a `Set` typed as a `List`. A
// hand-maintained list of a model's fields is a list that drifts from the model.
//
// Reflection removes the class: it walks whatever fields the struct actually
// has, so adding an attribute needs no change here and a new resource gets the
// behaviour for free.
//
// **IT ONLY EVER NULLS UNKNOWNS.** A configured value is known and is left
// alone — nulling everything would discard the practitioner's own configuration
// from state, which is worse than the bug being fixed.
func ResolveUnknowns(model any) {
	value := reflect.ValueOf(model)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return
	}

	value = value.Elem()
	if value.Kind() != reflect.Struct {
		return
	}

	for i := range value.NumField() {
		field := value.Field(i)
		if !field.CanSet() {
			continue
		}

		current, ok := field.Interface().(attr.Value)
		if !ok || !current.IsUnknown() {
			continue
		}

		if null, ok := nullOf(current); ok {
			field.Set(reflect.ValueOf(null))
		}
	}
}

// nullOf returns the null of the same type as an unknown value.
//
// Typed explicitly rather than through `attr.Type().ValueType()`, because a
// collection's null needs its ELEMENT type — `types.ListNull(types.StringType)`
// rather than a bare list — and getting that wrong is a compile error in the
// hand-written version and a runtime mismatch here.
func nullOf(value attr.Value) (attr.Value, bool) {
	switch typed := value.(type) {
	case basetypes.StringValue:
		return types.StringNull(), true
	case basetypes.BoolValue:
		return types.BoolNull(), true
	case basetypes.Int64Value:
		return types.Int64Null(), true
	case basetypes.Float64Value:
		return types.Float64Null(), true
	case basetypes.NumberValue:
		return types.NumberNull(), true
	case basetypes.ListValue:
		return types.ListNull(typed.ElementType(nil)), true
	case basetypes.SetValue:
		return types.SetNull(typed.ElementType(nil)), true
	case basetypes.MapValue:
		return types.MapNull(typed.ElementType(nil)), true
	case basetypes.ObjectValue:
		return types.ObjectNull(typed.AttributeTypes(nil)), true
	default:
		// A type this does not know about is left alone rather than guessed at.
		// Leaving an unknown produces a clear protocol error naming the
		// attribute; inventing the wrong null produces silent drift.
		return nil, false
	}
}
