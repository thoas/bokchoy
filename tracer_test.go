package bokchoy_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/thoas/bokchoy"
)

func TestTracer_Error(t *testing.T) {
	var (
		ctx = context.Background()
		err = fmt.Errorf("Unexpected error")
	)

	bokchoy.DefaultTracer.Log(ctx, "An error has occured", err)
}
