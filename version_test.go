package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetVersion(t *testing.T) {
	SetVersion("test-version")

	result := Version()
	assert.Equal(t, "test-version", result)
}
