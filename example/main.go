//go:build example
// +build example

package main

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

type ShapeKind string

// Shape2D store the kind and position of a shape.
//
//go:generate ../go-genbuilder -ignore logger
type Shape2D struct {
	logger   zap.Logger
	Kind     ShapeKind
	X        int
	Y        int
	Callback func(ctx context.Context)
	Type     string
}

// main builds a Shape2D and prints it.
func main() {
	shape := NewShape2DBuilder().
		SetKind("RECT").
		SetX(1).
		SetY(2).
		SetType("foo").
		Build()

	fmt.Printf("%#v", shape)
}

type testStructAfterTarget int
