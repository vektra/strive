package strive

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func (v *Value) ForceString() string {
	switch v.GetValueType() {
	case Value_STRING:
		return v.GetStrVal()
	case Value_BYTES:
		return string(v.GetBytesVal())
	case Value_INT:
		return strconv.FormatInt(v.GetIntVal(), 10)
	case Value_BOOL:
		if v.GetBoolVal() {
			return "1"
		} else {
			return "0"
		}
	case Value_RANGES:
		var parts []string

		for _, r := range v.GetRangeVal().Ranges {
			parts = append(parts, fmt.Sprintf("%d-%d", r.GetStart(), r.GetEnd()))
		}

		return "[" + strings.Join(parts, ",") + "]"
	case Value_LIST:
		var list []string

		for _, sub := range v.GetListVal() {
			list = append(list, sub.ForceString())
		}
	}

	return v.String()
}

func (v *Variable) StringKV() string {
	return v.GetName() + "=" + v.Value.ForceString()
}

func NewVariable(name string, val *Value) *Variable {
	return &Variable{
		Name:  &name,
		Value: val,
	}
}

func NewStringValue(str string) *Value {
	return &Value{
		ValueType: Value_STRING.Enum(),
		StrVal:    &str,
	}
}

func NewIntValue(i int64) *Value {
	return &Value{
		ValueType: Value_INT.Enum(),
		IntVal:    &i,
	}
}

func NewBoolValue(b bool) *Value {
	return &Value{
		ValueType: Value_BOOL.Enum(),
		BoolVal:   &b,
	}
}

func NewRangesValue(ranges ...*Range) *Value {
	return &Value{
		ValueType: Value_RANGES.Enum(),
		RangeVal:  &Ranges{Ranges: ranges},
	}
}

func (v *Value) Interface() interface{} {
	switch v.GetValueType() {
	case Value_STRING:
		return v.GetStrVal()
	case Value_INT:
		return v.GetIntVal()
	case Value_BOOL:
		return v.GetBoolVal()
	case Value_BYTES:
		return v.GetBytesVal()
	case Value_RANGES:
		return v.GetRangeVal()
	case Value_LIST:
		var list []interface{}

		for _, sub := range v.ListVal {
			list = append(list, sub.Interface())
		}
	case Value_MAP:
		m := make(map[string]interface{})

		for _, sub := range v.MapVal {
			m[sub.GetName()] = sub.GetValue().Interface()
		}
	}

	panic("unreachable")
}

type JSONVariables []*Variable

func (j JSONVariables) MarshalJSON() ([]byte, error) {
	out := make(map[string]interface{})

	for _, attr := range j {
		out[attr.GetName()] = attr.GetValue().Interface()
	}

	return json.Marshal(out)
}
