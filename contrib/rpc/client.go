package rpc

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/wrappers"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/pkg/errors"
	"github.com/thoas/bokchoy/contrib/rpc/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

// NewClient initializes a new rpc client.
func NewClient(addr string) *Client {
	return &Client{
		addr: addr,
	}
}

type Client struct {
	addr string
}

func (c *Client) dial() (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(c.addr,
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor()),
	)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to connect to server at %s", c.addr)
	}

	return conn, nil
}

func (c *Client) PublishTask(ctx context.Context, queueName string, payload []byte) (*proto.Task, error) {
	conn, err := c.dial()
	if err != nil {
		return nil, err
	}

	defer conn.Close()
	clt := proto.NewBokchoyClient(conn)

	task, err := clt.PublishTask(ctx, &proto.PublishTaskRequest{
		Queue: queueName,
		Payload: &wrappers.BytesValue{
			Value: payload,
		},
	}, grpc_retry.WithMax(3),
		grpc_retry.WithPerRetryTimeout(1*time.Second),
		grpc_retry.WithCodes(codes.Unavailable))
	if err != nil {
		return nil, errors.Wrapf(err, "unable to publish task")
	}

	return task, nil
}
