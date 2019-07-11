package middleware

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/pkg/errors"
	"github.com/thoas/bokchoy"
)

func Recoverer(next bokchoy.Handler) bokchoy.Handler {
	return bokchoy.HandlerFunc(func(r *bokchoy.Request) error {
		var err error

		defer func() {
			if rvr := recover(); rvr != nil {
				logEntry := GetLogEntry(r)
				if logEntry != nil {
					logEntry.Panic(rvr, debug.Stack())
				} else {
					fmt.Fprintf(os.Stderr, "Panic: %+v\n", rvr)
					debug.PrintStack()
				}

				var ok bool
				if err, ok = rvr.(error); !ok {
					err = fmt.Errorf("%v", err)
				}

				err = errors.WithStack(err)
			}
		}()

		next.Handle(r)

		return err
	})
}
