package comment

import "context"

// CommentLikeQueue exposes the existing Redis processing queue and acknowledgement.
type CommentLikeQueue interface {
	Available() bool
	NextBatch(context.Context) ([]string, error)
	Ack(context.Context, int) error
}
