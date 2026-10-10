package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"
)

func main() {
	goFile := os.Getenv("GOFILE")
	goLine := os.Getenv("GOLINE")

	targetLine := -1
	targetStructName := ""
	ignoreFields := make(IgnoreFields, 0)

	flagIgnore := flag.String("ignore", "", "a list of ignore struct fields")
	flag.Parse()

	for _, item := range strings.Split(*flagIgnore, ",") {
		ignoreFields[strings.TrimSpace(item)] = true
	}

	if goLine != "" {
		goLineInt, err := strconv.Atoi(goLine)
		if err != nil {
			panic(err)
		}

		targetLine = goLineInt
	} else {
		args := flag.Args()
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "missing args")
			os.Exit(1)
		}

		targetStructName = args[1]
	}

	GenBuilder(goFile, targetStructName, targetLine, ignoreFields)
}

type IgnoreFields map[string]bool

func (i IgnoreFields) Ignore(name string) bool {
	ignore, ok := i[name]
	if ok && ignore {
		return true
	}

	return false
}

func GenBuilder(input, targetStructName string, targetLine int, ignoreFields IgnoreFields) {
	genConfig, err := ParseFile(input, targetStructName, targetLine, ignoreFields)
	if err != nil {
		panic(err)
	}

	result, err := generate(genConfig)
	if err != nil {
		panic(err)
	}

	inputBase := filepath.Dir(input)

	err = os.WriteFile(filepath.Join(inputBase, filename(input, genConfig.StructName)), result, 0o755)
	if err != nil {
		panic(err)
	}
}

func ParseFile(input, targetStructName string, targetLine int, ignoreFields IgnoreFields) (*GeneratorConfig, error) {
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, input, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("could not parse file: %w", err)
	}

	genConfig, err := findStruct(fset, file, targetStructName, targetLine, ignoreFields)
	if err != nil {
		return nil, err
	}

	genConfig.BuildTags, err = findBuildTags(input)
	if err != nil {
		return nil, err
	}

	genConfig.Version = Version()

	return genConfig, nil
}

func filename(goFile, structName string) string {
	ext := filepath.Ext(goFile)

	return strings.TrimRight(goFile, ext) + "_" + strings.ToLower(structName) + "_gen.go"
}

func findBuildTags(input string) ([]string, error) {
	buildTags := make([]string, 0)

	inputSource, err := os.ReadFile(input)
	if err != nil {
		return nil, err
	}

	for _, line := range strings.Split(string(inputSource), "\n") {
		if strings.HasPrefix(line, "//go:build") || strings.HasPrefix(line, "// +build") {
			buildTags = append(buildTags, line)
		}
	}

	return buildTags, nil
}

func findImports(file *ast.File) map[string]Import {
	imports := make(map[string]Import, 0)

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}

		if genDecl.Tok != token.IMPORT {
			continue
		}

		for _, declSpec := range genDecl.Specs {
			importSpec, ok := declSpec.(*ast.ImportSpec)
			if !ok {
				continue
			}

			pathValue := ""

			var err error

			pathValue, err = strconv.Unquote(importSpec.Path.Value)
			if err != nil {
				pathValue = importSpec.Path.Value
			}

			name := ""
			if importSpec.Name != nil {
				name = importSpec.Name.String()
			} else {
				pathSplit := strings.Split(pathValue, "/")
				name = pathSplit[len(pathSplit)-1]
			}

			imports[name] = Import{
				Name: name,
				Path: pathValue,
			}
		}
	}

	return imports
}

func findStruct(fset *token.FileSet, file *ast.File, targetStructName string, targetLine int, ignoreFields IgnoreFields) (*GeneratorConfig, error) {
	genConfig := GeneratorConfig{}
	genConfig.PackageName = file.Name.String()
	genConfig.Fields = make([]Field, 0)

	foundImports := findImports(file)

	neededImports := make(map[string]Import, 0)

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}

		if genDecl.Tok != token.TYPE {
			continue
		}

		for _, declSpec := range genDecl.Specs {
			typeSpec, ok := declSpec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			if fset.Position(typeSpec.Name.NamePos).Line != targetLine+1 {
				if typeSpec.Name.Name != targetStructName {
					continue
				}
			}

			structType := typeSpec.Type.(*ast.StructType)

			genConfig.StructName = typeSpec.Name.String()

			for _, field := range structType.Fields.List {
				useField := false

				var typeNameBuf bytes.Buffer

				err := printer.Fprint(&typeNameBuf, fset, field.Type)
				if err != nil {
					return nil, fmt.Errorf("failed printing %s", err)
				}

				for _, name := range field.Names {
					if name.String() == "_" || ignoreFields.Ignore(name.String()) {
						continue
					}

					genConfig.Fields = append(genConfig.Fields, Field{
						Name: name.String(),
						Type: typeNameBuf.String(),
					})
					useField = true
				}

				if !useField {
					continue
				}

				starExpr, starExprOk := field.Type.(*ast.StarExpr)
				if starExprOk {
					selectorExpr, selOk := starExpr.X.(*ast.SelectorExpr)
					if selOk {
						x, ok := selectorExpr.X.(*ast.Ident)
						if ok {
							f, exist := foundImports[x.String()]
							if exist && !ignoreFields.Ignore(x.String()) {
								neededImports[x.String()] = f
							}
						}
					}
				}

				selectorExpr, selOk := field.Type.(*ast.SelectorExpr)
				if selOk {
					x, ok := selectorExpr.X.(*ast.Ident)
					if ok {
						f, exist := foundImports[x.String()]
						if exist && !ignoreFields.Ignore(x.String()) {
							neededImports[x.String()] = f
						}
					}
				}

				funcType, funcTypeOk := field.Type.(*ast.FuncType)
				if funcTypeOk {
					params := funcType.Params

					for _, param := range params.List {
						sExpr, sOk := param.Type.(*ast.SelectorExpr)
						if sOk {
							x, ok := sExpr.X.(*ast.Ident)
							if ok {
								neededImports[x.String()] = Import{
									Name: x.String(),
									Path: x.String(),
								}
							}
						}
					}
				}
			}
		}
	}

	imports := make([]Import, 0)
	for _, i := range neededImports {
		imports = append(imports, i)
	}

	sort.Slice(imports, func(i, j int) bool {
		return imports[i].Name < imports[j].Name
	})

	genConfig.Imports = imports

	return &genConfig, nil
}

type GeneratorConfig struct {
	Version     string
	BuildTags   []string
	PackageName string
	StructName  string
	Imports     []Import
	Fields      []Field
}

type Import struct {
	Name string
	Path string
}

type Field struct {
	Name string
	Type string
}

// receiverName is the generated method receiver. Parameter names must not reuse it.
const receiverName = "builder"

// identName returns a lower-case identifier for generated code.
// Lowercasing a field such as Type produces the keyword "type", which cannot be a parameter name.
func identName(name string) string {
	ident := strings.ToLower(name)
	if token.IsKeyword(ident) {
		return ident + "Arg"
	}

	return ident
}

// paramName is the setter parameter for a field. A field named Builder would otherwise
// redeclare the receiver.
func paramName(name string) string {
	ident := identName(name)
	if ident == receiverName {
		return ident + "Arg"
	}

	return ident
}

// setterNames returns the method name for each field. Names that title-case to the
// same setter, such as Foo and foo, keep the original spelling. Blank fields are skipped.
func setterNames(fields []Field) []string {
	preferred := make([]string, len(fields))
	count := make(map[string]int, len(fields))

	for i, field := range fields {
		if field.Name == "_" {
			continue
		}

		preferred[i] = "Set" + strings.Title(field.Name)
		count[preferred[i]]++
	}

	taken := make(map[string]bool, len(fields))
	names := make([]string, len(fields))

	for i, field := range fields {
		if field.Name == "_" {
			continue
		}

		name := preferred[i]
		if count[name] > 1 {
			name = "Set" + field.Name
		}

		for taken[name] {
			name += "_"
		}

		names[i] = name
		taken[name] = true
	}

	return names
}

func generate(genConfig *GeneratorConfig) ([]byte, error) {
	generatorTemplate := `// Code generated by go-genbuilder v{{ .Version }}. DO NOT EDIT.
{{- range $1, $buildTag := .BuildTags }}
{{ $buildTag }}
{{- end }}

package {{ .PackageName }}
{{ if withImports }}
import (
{{- range $1, $import := .Imports }}
	{{ if ne $import.Name $import.Path }}{{ $import.Name }} {{ end }}"{{ $import.Path }}"
{{- end }}
)
{{ end }}
type {{ .StructName }}Builder struct {
	{{ ident .StructName }} *{{ .StructName }}
}

func New{{ .StructName }}Builder() *{{ .StructName }}Builder {
	return &{{ .StructName }}Builder{
		{{ ident .StructName }}: &{{ .StructName}}{},
	}
}
{{ range $i, $field := .Fields }}
{{- if ne (setter $i) "" }}
func ({{ receiver }} *{{ $.StructName }}Builder) {{ setter $i }}({{ param $field.Name }} {{ $field.Type }}) *{{ $.StructName }}Builder {
	{{ receiver }}.{{ ident $.StructName }}.{{ $field.Name }} = {{ param $field.Name }}
	return {{ receiver }}
}
{{ end }}
{{- end }}
func ({{ receiver }} *{{ .StructName }}Builder) Build() *{{ .StructName }} {
	return {{ receiver }}.{{ ident .StructName }}
}
`

	setterName := setterNames(genConfig.Fields)

	tmpl := template.New("gen")

	tmpl.Funcs(template.FuncMap{
		"ident": identName,
		"param": paramName,
		"receiver": func() string {
			return receiverName
		},
		"setter": func(i int) string {
			if i < 0 || i >= len(setterName) {
				return ""
			}

			return setterName[i]
		},
		"withImports": func() bool {
			if len(genConfig.Imports) > 0 {
				return true
			}

			return false
		},
	})

	tmpl, err := tmpl.Parse(generatorTemplate)
	if err != nil {
		return nil, err
	}

	buf := bytes.Buffer{}

	err = tmpl.Execute(&buf, genConfig)
	if err != nil {
		return nil, err
	}

	return format.Source(buf.Bytes())
}
