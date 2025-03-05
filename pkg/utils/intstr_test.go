package utils

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFromInt32(t *testing.T) {
	i := FromInt32(543)
	assert.Equal(t, Int, i.Type, "Expected Type=Int")
	assert.Equal(t, int32(543), i.IntVal, "Expected IntVal=543")

}

func TestFromString(t *testing.T) {
	i := FromString("545")
	assert.Equal(t, String, i.Type, "Expected Type=String")
	assert.Equal(t, "545", i.StrVal, "Expected StrVal=\"545\"")
}

type IntOrStringHolder struct {
	IoSVal Int32String `json:"val"`
}

func TestIntOrStringUnmarshalJSON(t *testing.T) {
	cases := []struct {
		input  string
		result Int32String
	}{
		{"{\"val\": 432}", FromInt32(432)},
		{"{\"val\": \"432\"}", FromString("432")},
	}

	for _, c := range cases {
		var result IntOrStringHolder
		err := json.Unmarshal([]byte(c.input), &result)
		assert.NoError(t, err, "Failed to unmarshal input '%v'", c.input)
		assert.Equal(t, c.result, result.IoSVal, "Failed to unmarshal input '%v'", c.input)
	}
}

func TestIntOrStringMarshalJSON(t *testing.T) {
	cases := []struct {
		input  Int32String
		result string
	}{
		{FromInt32(543), "{\"val\":543}"},
		{FromString("543"), "{\"val\":\"543\"}"},
	}

	for _, c := range cases {
		input := IntOrStringHolder{c.input}
		result, err := json.Marshal(&input)
		assert.NoError(t, err, "Failed to marshal input '%v'", input)
		assert.JSONEq(t, c.result, string(result), "Failed to marshal input '%v'", input)
	}
}

// TODO: Add marchalYaml tests
