// Package platform keeps scheduling contracts for business maintenance use cases.
package platform

import "context"

type AutoUnbanJob interface{ Run(context.Context) }
type CommentLikeSyncJob interface{ Run(context.Context) }
type HotRankDecayJob interface{ Run(context.Context) }
type VideoStatSyncJob interface{ Run(context.Context) }

// UploadCleanupJob runs the video upload cleanup use case.
type UploadCleanupJob interface{ Run(context.Context) }
