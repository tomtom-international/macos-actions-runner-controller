package utils

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"strconv"
)

// Type represents the stored type of Int32String.
type Type int64

const (
	Int Type = iota
	String
)

type Int32String struct {
	Type   Type
	IntVal int32
	StrVal string
}

// FromInt32 creates an Int32String object with an int32 value.
func FromInt32(val int32) Int32String {
	return Int32String{Type: Int, IntVal: val}
}

// FromString creates an Int32String object with a string value.
func FromString(val string) Int32String {
	return Int32String{Type: String, StrVal: val}
}

// Parse the given string and try to convert it to an int32 integer before
// setting it as a string value.
func Parse(val string) Int32String {
	i, err := strconv.ParseInt(val, 10, 32)
	if err != nil {
		return FromString(val)
	}
	return FromInt32(int32(i))
}

func (intstr *Int32String) UnmarshalJSON(value []byte) error {
	if value[0] == '"' {
		intstr.Type = String
		return json.Unmarshal(value, &intstr.StrVal)
	}
	intstr.Type = Int
	return json.Unmarshal(value, &intstr.IntVal)
}

func (intstr *Int32String) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		var intVal int32
		if err := node.Decode(&intVal); err == nil {
			intstr.Type = Int
			intstr.IntVal = intVal
			return nil
		}
		var strVal string
		if err := node.Decode(&strVal); err == nil {
			intstr.Type = String
			intstr.StrVal = strVal
			return nil
		}
		return fmt.Errorf("invalid scalar value for Int32String")
	default:
		return fmt.Errorf("unsupported YAML node kind: %v", node.Kind)
	}
}

// String returns the string value, or the Itoa of the int value.
func (intstr *Int32String) String() string {
	if intstr == nil {
		return "<nil>"
	}
	if intstr.Type == String {
		return intstr.StrVal
	}
	return strconv.Itoa(intstr.IntValue())
}

// IntValue returns the IntVal if type Int, or if
// it is a String, will attempt a conversion to int,
// returning 0 if a parsing error occurs.
func (intstr *Int32String) IntValue() int {
	if intstr.Type == String {
		i, _ := strconv.Atoi(intstr.StrVal)
		return i
	}
	return int(intstr.IntVal)
}

// MarshalJSON implements the json.Marshaller interface.
func (intstr Int32String) MarshalJSON() ([]byte, error) {
	switch intstr.Type {
	case Int:
		return json.Marshal(intstr.IntVal)
	case String:
		return json.Marshal(intstr.StrVal)
	default:
		return []byte{}, fmt.Errorf("impossible IntOrString.Type")
	}
}

func (intstr Int32String) MarshalYAML() (interface{}, error) {
	switch intstr.Type {
	case Int:
		return intstr.IntVal, nil
	case String:
		return intstr.StrVal, nil
	default:
		return nil, fmt.Errorf("unsupported type: %v", intstr.Type)
	}
}
