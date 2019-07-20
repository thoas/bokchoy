// inspired from https://github.com/go-chi/chi/blob/master/middleware/timeout.go

package middleware

import (
	"context"
	"time"

	"github.com/thoas/bokchoy"
)

// Timeout is a middleware that cancels ctx after a given timeout and return
//
// It's required that you select the ctx.Done() channel to check for the signal
// if the context has reached its deadline and return, otherwise the timeout
// signal will be just ignored.
//
// ie. a handler may look like:
//
//  queue.HandlerFunc(func(r *bokchoy.Request) {
// 	 ctx := r.Context()
// 	 processTime := time.Duration(rand.Intn(4)+1) * time.Second
//
// 	 select {
// 	 case <-ctx.Done():
// 	 	return
//
// 	 case <-time.After(processTime):
// 	 	 // The above channel simulates some hard work.
// 	 }
//
// 	 return nil
//  })
//
func Timeout(timeout time.Duration) func(next bokchoy.Handler) bokchoy.Handler {
	return func(next bokchoy.Handler) bokchoy.Handler {
		fn := func(r *bokchoy.Request) error {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			var err error

			defer func() {
				cancel()
				err = ctx.Err()
			}()

			err = next.Handle(r.WithContext(ctx))

			return err
		}
		return bokchoy.HandlerFunc(fn)
	}
}
