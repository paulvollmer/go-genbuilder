package main

import (
	"go/ast"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_IgnoreFields_Ignore(t *testing.T) {
	t.Parallel()

	ignoreFields := IgnoreFields{
		"test": true,
	}

	assert.Equal(t, true, ignoreFields.Ignore("test"))
	assert.Equal(t, false, ignoreFields.Ignore("foo"))
}

func Test_generate(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		testDescription string
		input           *GeneratorConfig
		expected        string
	}{
		{
			testDescription: "simple",
			input: &GeneratorConfig{
				Version:     "0.0.0",
				PackageName: "testpackage",
				StructName:  "TestStruct",
				Imports:     nil,
				Fields: []Field{
					{
						Name: "TestField",
						Type: "testType",
					},
				},
				BuildTags: nil,
			},
			expected: `// Code generated by go-genbuilder v0.0.0. DO NOT EDIT.

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
`,
		},
		{
			testDescription: "with imports and build tags",
			input: &GeneratorConfig{
				Version:     "0.0.0",
				PackageName: "testpackage",
				StructName:  "TestStruct",
				Imports: []Import{
					{Name: "context", Path: "context"},
					{Name: "sample1", Path: "sample1"},
					{Name: "customalias", Path: "sample2"},
				},
				Fields: []Field{
					{
						Name: "TestField",
						Type: "testType",
					},
					{
						Name: "testOtherField",
						Type: "int",
					},
					{
						Name: "testFunc",
						Type: "func(ctx context.Context)",
					},
				},
				BuildTags: []string{
					"//go:build example",
					"// +build example",
				},
			},
			expected: `// Code generated by go-genbuilder v0.0.0. DO NOT EDIT.
//go:build example
// +build example

package testpackage

import (
	"context"
	"sample1"
	customalias "sample2"
)

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

func (builder *TestStructBuilder) SetTestOtherField(testotherfield int) *TestStructBuilder {
	builder.teststruct.testOtherField = testotherfield
	return builder
}

func (builder *TestStructBuilder) SetTestFunc(testfunc func(ctx context.Context)) *TestStructBuilder {
	builder.teststruct.testFunc = testfunc
	return builder
}

func (builder *TestStructBuilder) Build() *TestStruct {
	return builder.teststruct
}
`,
		},
	}

	for _, testcase := range testcases {
		testcase := testcase
		t.Run(testcase.testDescription, func(t *testing.T) {

			result, err := generate(testcase.input)
			assert.NoError(t, err)
			assert.Equal(t, testcase.expected, string(result))
		})
	}
}

func TestParseFile(t *testing.T) {
	t.Parallel()

	SetVersion("test")
	testcases := []struct {
		testDescrption   string
		input            string
		targetStructName string
		targetLine       int
		ignoreFields     map[string]bool
		expectedImports  []Import
		expectedFields   []Field
	}{
		{
			testDescrption:   "using targetStructName",
			input:            "./example/main.go",
			targetStructName: "Shape2D",
			targetLine:       -1,
			ignoreFields:     nil,
			expectedImports: []Import{
				{Name: "context", Path: "context"},
				{Name: "zap", Path: "go.uber.org/zap"},
			},
			expectedFields: []Field{
				{Name: "logger", Type: "zap.Logger"},
				{Name: "Kind", Type: "ShapeKind"},
				{Name: "X", Type: "int"},
				{Name: "Y", Type: "int"},
				{Name: "Callback", Type: "func(ctx context.Context)"},
			},
		},
		{
			testDescrption:   "using targetLine",
			input:            "./example/main.go",
			targetStructName: "",
			targetLine:       17,
			ignoreFields:     nil,
			expectedImports: []Import{
				{Name: "context", Path: "context"},
				{Name: "zap", Path: "go.uber.org/zap"},
			},
			expectedFields: []Field{
				{Name: "logger", Type: "zap.Logger"},
				{Name: "Kind", Type: "ShapeKind"},
				{Name: "X", Type: "int"},
				{Name: "Y", Type: "int"},
				{Name: "Callback", Type: "func(ctx context.Context)"},
			},
		},
		{
			testDescrption:   "using targetLine",
			input:            "./example/main.go",
			targetStructName: "",
			targetLine:       17,
			ignoreFields: map[string]bool{
				"logger":   true,
				"Kind":     true,
				"Y":        true,
				"Callback": true,
			},
			expectedImports: []Import{},
			expectedFields: []Field{
				{Name: "X", Type: "int"},
			},
		},
	}

	for _, testcase := range testcases {
		testcase := testcase
		t.Run(testcase.testDescrption, func(t *testing.T) {
			t.Parallel()

			actual, err := ParseFile(testcase.input, testcase.targetStructName, testcase.targetLine, testcase.ignoreFields)
			assert.NoError(t, err)
			assert.Equal(t, "test", actual.Version)
			assert.Equal(t, "main", actual.PackageName)
			assert.Equal(t, "Shape2D", actual.StructName)
			assert.Equal(t, []string{"//go:build example", "// +build example"}, actual.BuildTags)
			assert.Equal(t, testcase.expectedImports, actual.Imports)
			assert.Equal(t, testcase.expectedFields, actual.Fields)
		})
	}
}

func Test_findImports(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		testDescrption string
		file           *ast.File
		expected       map[string]Import
	}{
		{
			testDescrption: "simple import",
			file: &ast.File{
				Decls: []ast.Decl{
					&ast.GenDecl{
						Tok: token.IMPORT,
						Specs: []ast.Spec{
							&ast.ImportSpec{
								Name: nil,
								Path: &ast.BasicLit{Kind: token.STRING, Value: `"fmt"`},
							},
							&ast.ImportSpec{
								Name: nil,
								Path: &ast.BasicLit{Kind: token.STRING, Value: `"go.uber.org/zap"`},
							},
						},
					},
				},
			},
			expected: map[string]Import{
				"fmt": {Name: "fmt", Path: "fmt"},
				"zap": {Name: "zap", Path: "go.uber.org/zap"},
			},
		},
		{
			testDescrption: "import with name",
			file: &ast.File{
				Decls: []ast.Decl{
					&ast.GenDecl{
						Tok: token.IMPORT,
						Specs: []ast.Spec{
							&ast.ImportSpec{
								Name: nil,
								Path: &ast.BasicLit{Kind: token.STRING, Value: "fmt"},
							},
							&ast.ImportSpec{
								Name: &ast.Ident{Name: "thezap"},
								Path: &ast.BasicLit{Kind: token.STRING, Value: "go.uber.org/zap"},
							},
						},
					},
				},
			},
			expected: map[string]Import{
				"fmt":    {Name: "fmt", Path: "fmt"},
				"thezap": {Name: "thezap", Path: "go.uber.org/zap"},
			},
		},
	}

	for _, testcase := range testcases {
		testcase := testcase
		t.Run(testcase.testDescrption, func(t *testing.T) {
			t.Parallel()

			result := findImports(testcase.file)
			assert.Equal(t, testcase.expected, result)
		})
	}
}
