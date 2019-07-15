package rpc

import (
	"time"

	"github.com/thoas/bokchoy"
	"github.com/thoas/bokchoy/contrib/rpc/proto"
)

func TaskToProto(task *bokchoy.Task) *proto.Task {
	pb := &proto.Task{
		ID:          task.ID,
		Name:        task.Name,
		Status:      int64(task.Status),
		MaxRetries:  int64(task.MaxRetries),
		TTL:         &task.TTL,
		ETA:         &task.ETA,
		Timeout:     &task.Timeout,
		PublishedAt: &task.PublishedAt,
		ProcessedAt: &task.ProcessedAt,
		StartedAt:   &task.StartedAt,
	}

	retryIntervals := make([]*time.Duration, len(task.RetryIntervals))
	for i := range task.RetryIntervals {
		retryIntervals[i] = &task.RetryIntervals[i]
	}

	pb.RetryIntervals = retryIntervals

	return pb
}
