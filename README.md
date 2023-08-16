# gen-builder

A golang code generator tool to generate `Builder` pattern code.

## Usage

```go
//go:generate go run github.com/paulvollmer/go-genbuilder
type Shape2D struct {
	Kind ShapeKind
	X    int
	Y    int
}
```
