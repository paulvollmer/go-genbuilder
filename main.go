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
					if ignoreFields.Ignore(name.String()) {
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
	{{ toLower .StructName }} *{{ .StructName }}
}

func New{{ .StructName }}Builder() *{{ .StructName }}Builder {
	return &{{ .StructName }}Builder{
		{{ toLower .StructName }}: &{{ .StructName}}{},
	}
}
{{ range $i, $field := .Fields }}
func (builder *{{ $.StructName }}Builder) Set{{ title $field.Name }}({{ toLower $field.Name }} {{ $field.Type }}) *{{ $.StructName }}Builder {
	builder.{{ toLower $.StructName }}.{{ $field.Name }} = {{ toLower $field.Name }}
	return builder
}
{{ end }}
func (builder *{{ .StructName }}Builder) Build() *{{ .StructName }} {
	return builder.{{ toLower .StructName }}
}
`

	tmpl := template.New("gen")

	tmpl.Funcs(template.FuncMap{
		"toLower": func(s string) string {
			return strings.ToLower(s)
		},
		"title": func(s string) string {
			return strings.Title(s)
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
