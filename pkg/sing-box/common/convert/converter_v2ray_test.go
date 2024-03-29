package convert

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestConvertsV2Ray(t *testing.T) {
	content := []byte("")

	out, err := ConvertsV2Ray(content)
	assert.Equal(t, nil, err)

	fmt.Println(out)
}

func TestConvertsClash(t *testing.T) {
	content := []byte("")

	out, err := ConvertsClash(content)
	assert.Equal(t, nil, err)

	fmt.Println(out)
}
