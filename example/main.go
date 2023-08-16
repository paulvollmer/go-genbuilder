package main

import "fmt"

type ShapeKind string

// Shape2D store the kind and position of a shape.
//
//go:generate ../go-genbuilder
type Shape2D struct {
	Kind ShapeKind
	X    int
	Y    int
}

func main() {
	shape := NewShape2DBuilder().
		SetKind("RECT").
		SetX(1).
		SetY(2).
		Build()

	fmt.Printf("%#v", shape)
}

type testStructAfterTarget int
