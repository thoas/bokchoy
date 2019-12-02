package bokchoy_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thoas/bokchoy"
	"github.com/thoas/go-funk"
)

func TestUtils_ID(t *testing.T) {
	is := assert.New(t)

	iteration := 1000

	ids := make([]string, iteration)

	for i := 0; i < iteration; i++ {
		ids[i] = bokchoy.ID()
	}

	is.Len(funk.UniqString(ids), iteration)
}
