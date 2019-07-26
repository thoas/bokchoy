package rpc

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"google.golang.org/grpc"

	"github.com/thoas/bokchoy"
	"github.com/thoas/bokchoy/contrib/rpc/proto"
	"github.com/thoas/bokchoy/logging"
)

// Server is a rpc server which contains gRPC.
type Server struct {
	logger logging.Logger
	bok    *bokchoy.Bokchoy
	srv    *grpc.Server
	port   int
}

// NewServer initializes a new Server.
func NewServer(bok *bokchoy.Bokchoy, port int) *Server {
	s := grpc.NewServer()

	logger := bok.Logger.With(logging.String("server", "rpc"))

	proto.RegisterBokchoyServer(s, &Handler{
		bok:    bok,
		logger: logger,
	})

	return &Server{
		bok:    bok,
		srv:    s,
		port:   port,
		logger: logger,
	}
}

func (s *Server) String() string {
	return "rpc"
}

// Start starts the RPC server.
func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf(":%s", strconv.Itoa(s.port))

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	s.logger.Info(ctx, "Start server", logging.String("addr", addr))

	err = s.srv.Serve(lis)
	if err != nil {
		return err
	}

	return nil
}

// Stop stops the RPC server.
func (s *Server) Stop(ctx context.Context) {
	s.srv.GracefulStop()

	s.logger.Info(ctx, "Server shutdown")
}

var _ bokchoy.Server = (*Server)(nil)
