package benchmark

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

//go:generate ../go-genbuilder
type Shape struct {
	X    int
	Y    int
	Name string
}

func BenchmarkExample(b *testing.B) {
	for i := 0; i < b.N; i++ {
		shape := NewShapeBuilder().
			SetX(1).
			SetY(2).
			SetName("test").
			Build()

		assert.Equal(b, 1, shape.X)
	}

	b.ReportAllocs()
}
