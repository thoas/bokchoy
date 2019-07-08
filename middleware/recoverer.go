package middleware

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/thoas/bokchoy"
)

func Recoverer(next bokchoy.Subscriber) bokchoy.Subscriber {
	return bokchoy.SubscriberFunc(func(r *bokchoy.Request) error {
		defer func() {
			if rvr := recover(); rvr != nil {
				logEntry := GetLogEntry(r)
				if logEntry != nil {
					logEntry.Panic(rvr, debug.Stack())
				} else {
					fmt.Fprintf(os.Stderr, "Panic: %+v\n", rvr)
					debug.PrintStack()
				}
			}
		}()

		next.Consume(r)

		return nil
	})
}
