package rpc

import (
	"context"

	"github.com/thoas/bokchoy"
	"github.com/thoas/bokchoy/contrib/rpc/proto"
	"github.com/thoas/bokchoy/logging"
)

type Handler struct {
	bok    *bokchoy.Bokchoy
	logger logging.Logger
}

func (h Handler) PublishTask(ctx context.Context, req *proto.PublishTaskRequest) (*proto.Task, error) {
	var payload interface{}
	err := h.bok.Serializer.Loads(req.Payload.Value, &payload)
	if err != nil {
		return nil, err
	}

	task, err := h.bok.Queue(req.Queue).Publish(ctx, payload)
	if err != nil {
		return nil, err
	}

	pb := TaskToProto(task)
	pb.Payload = req.Payload

	return pb, nil
}
