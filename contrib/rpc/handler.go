package rpc

import (
	"context"
	"time"

	"github.com/thoas/bokchoy"
	"github.com/thoas/bokchoy/contrib/rpc/proto"
	"github.com/thoas/bokchoy/logging"
)

type Handler struct {
	bok    *bokchoy.Bokchoy
	logger logging.Logger
}

func getRequestOptions(req *proto.PublishTaskRequest) []bokchoy.Option {
	options := []bokchoy.Option{}

	if req.Countdown != nil {
		options = append(options, bokchoy.WithCountdown(*req.Countdown))
	}

	if req.TTL != nil {
		options = append(options, bokchoy.WithTTL(*req.TTL))
	}

	if req.MaxRetries != nil {
		options = append(options, bokchoy.WithMaxRetries(int(req.MaxRetries.Value)))
	}

	if len(req.RetryIntervals) != 0 {
		intervals := make([]time.Duration, len(req.RetryIntervals))

		for i := range req.RetryIntervals {
			intervals[i] = *req.RetryIntervals[i]
		}

		options = append(options, bokchoy.WithRetryIntervals(intervals))
	}

	if req.Timeout != nil {
		options = append(options, bokchoy.WithTimeout(*req.Timeout))
	}

	return options
}

func (h Handler) PublishTask(ctx context.Context, req *proto.PublishTaskRequest) (*proto.Task, error) {
	var payload interface{}
	err := h.bok.Serializer.Loads(req.Payload.Value, &payload)
	if err != nil {
		return nil, err
	}

	task, err := h.bok.Queue(req.Queue).Publish(ctx, payload, getRequestOptions(req)...)
	if err != nil {
		return nil, err
	}

	pb := TaskToProto(task)
	pb.Payload = req.Payload

	return pb, nil
}
