package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

func main() {
	goFile := os.Getenv("GOFILE")
	goLine := os.Getenv("GOLINE")

	targetLine := -1
	targetStructName := ""

	if goLine != "" {
		goLineInt, err := strconv.Atoi(goLine)
		if err != nil {
			panic(err)
		}
		targetLine = goLineInt
	} else {
		args := os.Args
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "missing args")
			os.Exit(1)
		}

		targetStructName = args[1]
	}

	genConfig, err := ParseFile(goFile, targetStructName, targetLine)
	if err != nil {
		panic(err)
	}

	result, err := generate(genConfig)
	if err != nil {
		panic(err)
	}

	inputBase := filepath.Dir(goFile)
	err = os.WriteFile(filepath.Join(inputBase, strings.ToLower(genConfig.StructName)+".gen.go"), result, 0755)
	if err != nil {
		panic(err)
	}
}

func ParseFile(input, targetStructName string, targetLine int) (*GeneratorConfig, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, input, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("could not parse file: %w", err)
	}

	genConfig, err := findStruct(fset, file, targetStructName, targetLine)
	if err != nil {
		return nil, err
	}

	return genConfig, nil
}

func findStruct(fset *token.FileSet, file *ast.File, targetStructName string, targetLine int) (*GeneratorConfig, error) {
	genConfig := GeneratorConfig{}
	genConfig.PackageName = file.Name.String()
	//genConfig.StructName = targetStructName
	genConfig.Fields = make([]Field, 0)

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
				var typeNameBuf bytes.Buffer
				err := printer.Fprint(&typeNameBuf, fset, field.Type)
				if err != nil {
					return nil, fmt.Errorf("failed printing %s", err)
				}

				for _, name := range field.Names {
					genConfig.Fields = append(genConfig.Fields, Field{
						Name: name.String(),
						Type: typeNameBuf.String(),
					})
				}
			}
		}
	}

	return &genConfig, nil
}

type GeneratorConfig struct {
	PackageName string
	StructName  string
	Fields      []Field
}

type Field struct {
	Name string
	Type string
}

func generate(genConfig *GeneratorConfig) ([]byte, error) {
	generatorTemplate := `// code generated

package {{ .PackageName }}

type {{ .StructName }}Builder struct {
	{{ toLower .StructName }} *{{ .StructName }}
}

func New{{ .StructName }}Builder() *{{ .StructName }}Builder {
	return &{{ .StructName }}Builder{
		{{ toLower .StructName }}: &{{ .StructName}}{},
	}
}
{{ range $i, $field := .Fields }}
func (builder *{{ $.StructName }}Builder) Set{{ $field.Name }}({{ toLower $field.Name }} {{ $field.Type }}) *{{ $.StructName }}Builder {
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

	return buf.Bytes(), nil
}
