package bokchoy_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thoas/bokchoy"
)

func TestRequest_String(t *testing.T) {
	is := assert.New(t)

	req := &bokchoy.Request{Task: &bokchoy.Task{}}
	is.NotZero(req.String())
}
