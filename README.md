# gen-builder ![CI](https://github.com/paulvollmer/go-genbuilder/actions/workflows/ci.yml/badge.svg)

A golang code generator tool to generate `Builder` pattern code.

## Usage

Add a `go:generate` annotation to a struct

```go
//go:generate go run github.com/paulvollmer/go-genbuilder
type Shape2D struct {
	Kind ShapeKind
	X    int
	Y    int
}
```

And then use the generated code to create a `Shape2D` instance.

```go
shape := NewShape2DBuilder().
		SetKind("RECT").
		SetX(1).
		SetY(2).
		Build()
```
In case you want to ignore some fields use the `-ignore` flag to create a comma separated list of fields to ignore:

```go
//go:generate go run github.com/paulvollmer/go-genbuilder -ignore X,Y
type Shape2D struct {
	Kind ShapeKind
	X    int
	Y    int
}
```
