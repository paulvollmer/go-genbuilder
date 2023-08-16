package main

import (
	"bytes"
	"testing"
)

func Test_generate(t *testing.T) {
	genConfig := &GeneratorConfig{
		PackageName: "testpackage",
		StructName:  "TestStruct",
		Fields: []Field{
			{
				Name: "TestField",
				Type: "testType",
			},
		},
	}

	actual, err := generate(genConfig)
	if err != nil {
		t.Errorf("expected no error but got %v", err)
		return
	}

	expected := []byte(`// code generated

package testpackage

type TestStructBuilder struct {
	teststruct *TestStruct
}

func NewTestStructBuilder() *TestStructBuilder {
	return &TestStructBuilder{
		teststruct: &TestStruct{},
	}
}

func (builder *TestStructBuilder) SetTestField(testfield testType) *TestStructBuilder {
	builder.teststruct.TestField = testfield
	return builder
}

func (builder *TestStructBuilder) Build() *TestStruct {
	return builder.teststruct
}
`)

	if !bytes.Equal(expected, actual) {
		t.Errorf("expected result is not equal.\n\n%s\n\nbut got\n\n%s", expected, actual)
		return
	}
}

func TestParseFile(t *testing.T) {
	testcases := []struct {
		name             string
		input            string
		targetStructName string
		targetLine       int
	}{
		{
			name:             "using targetStructName",
			input:            "./example/main.go",
			targetStructName: "Shape2D",
			targetLine:       -1,
		},
		{
			name:             "using targetLine",
			input:            "./example/main.go",
			targetStructName: "",
			targetLine:       10,
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {

			actual, err := ParseFile(testcase.input, testcase.targetStructName, testcase.targetLine)
			if err != nil {
				t.Errorf("expected no error but got %v", err)
				return
			}

			if actual.PackageName != "main" {
				t.Errorf(`expected PackageName is "main" but got %q`, actual.PackageName)
				return
			}
			if actual.StructName != "Shape2D" {
				t.Errorf(`expected StructName is "Shape2D" but got %q`, actual.PackageName)
				return
			}
			if len(actual.Fields) != 3 {
				t.Errorf(`expected number of Fields is 3 but got %v`, len(actual.Fields))
				return
			}
			if actual.Fields[0].Name != "Kind" {
				t.Errorf(`expected Fields[0].Name is "Kind" but got %v`, actual.Fields[0].Name)
				return
			}
			if actual.Fields[0].Type != "ShapeKind" {
				t.Errorf(`expected Fields[0].Type is "ShapeKind" but got %v`, actual.Fields[0].Type)
				return
			}
			if actual.Fields[1].Name != "X" {
				t.Errorf(`expected Fields[1].Name is "X" but got %v`, actual.Fields[1].Name)
				return
			}
			if actual.Fields[1].Type != "int" {
				t.Errorf(`expected Fields[1].Type is "int" but got %v`, actual.Fields[1].Type)
				return
			}
			if actual.Fields[2].Name != "Y" {
				t.Errorf(`expected Fields[2].Name is "Y" but got %v`, actual.Fields[2].Name)
				return
			}
			if actual.Fields[2].Type != "int" {
				t.Errorf(`expected Fields[2].Type is "int" but got %v`, actual.Fields[2].Type)
				return
			}
		})
	}
}
