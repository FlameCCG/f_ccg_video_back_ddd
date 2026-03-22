package video_test

import (
	"context"
	"errors"
	"testing"
	"time"

	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	videodomain "f_ccg_video_back_ddd/internal/domain/video"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"go.uber.org/zap"
)

type resultPublicationRepository struct {
	draft *videodomain.PublishDraft
	err   error
}

func (r *resultPublicationRepository) Persist(_ context.Context, draft *videodomain.PublishDraft) (*videodomain.PublicationResult, error) {
	r.draft = draft
	if r.err != nil {
		return nil, r.err
	}
	return &videodomain.PublicationResult{
		Video: videodomain.RestoreVideo(videodomain.VideoSnapshot{ID: 75, Status: enum.VideoStatusReview}),
		Parts: []videodomain.VideoPart{{ID: 502, VideoID: 75, SortOrder: 1}, {ID: 501, VideoID: 75, SortOrder: 0}},
	}, nil
}

type publicationPartitions struct {
	videodomain.PartitionRepository
}

func (publicationPartitions) FindByID(context.Context, uint) (*videodomain.Partition, error) {
	return &videodomain.Partition{ID: 1, IsActive: true, IsSubmittable: true}, nil
}

type publicationTranscodes struct {
	tasks  []app.TranscodeTask
	queued [][2]uint
}

func (r *publicationTranscodes) PublishTranscodeTask(_ context.Context, task app.TranscodeTask) error {
	r.tasks = append(r.tasks, task)
	return nil
}

func (r *publicationTranscodes) MarkTranscodeQueued(_ context.Context, videoID, partID uint) {
	r.queued = append(r.queued, [2]uint{videoID, partID})
}

func TestMultipartPublicationDispatchesReturnedPartIdentities(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[fail], func(t *testing.T) {
			repo := &resultPublicationRepository{}
			if fail {
				repo.err = errors.New("publication failed")
			}
			transcodes := &publicationTranscodes{}
			tasks := ctxutil.NewDetachedTaskGroup()
			t.Cleanup(func() { _ = tasks.Stop(context.Background()) })
			provider := infraconfig.NewProvider(&config.Config{})
			service := app.NewPublishService(repo, nil, publicationPartitions{}, newUploadService(&stubDriver{
				completed: true, completedFilePath: "/videos/source.mp4",
			}), transcodes, app.NewVectorSyncService(nil, nil, nil), provider, tasks, zap.NewNop())
			result, err := service.PublishVideo(t.Context(), app.PublishVideoCommand{AuthorID: 7, AuthorUsername: "alice", PartitionID: 1,
				Parts: []app.PublishPartInput{{FileName: "first.mp4", FileHash: "first"}, {FileName: "second.mp4", FileHash: "second"}},
			})
			waitCtx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			if err := tasks.Wait(waitCtx); err != nil {
				t.Fatal(err)
			}
			if repo.draft.Video.ID() != 0 || repo.draft.Parts[0].ID != 0 || repo.draft.Parts[1].ID != 0 {
				t.Fatal("publication depends on mutating input identities")
			}
			if fail {
				if !errors.Is(err, repo.err) || result != nil || len(transcodes.tasks) != 0 || len(transcodes.queued) != 0 {
					t.Fatalf("failed save dispatched work: result=%+v tasks=%+v err=%v", result, transcodes.tasks, err)
				}
				return
			}
			if err != nil || result.VideoID != 75 || result.Status != enum.VideoStatusReview || len(transcodes.tasks) != 2 || len(transcodes.queued) != 2 {
				t.Fatalf("result=%+v tasks=%+v queued=%v err=%v", result, transcodes.tasks, transcodes.queued, err)
			}
			for i, want := range []struct {
				partID   uint
				fileHash string
			}{{502, "second"}, {501, "first"}} {
				task := transcodes.tasks[i]
				if task.VideoID != 75 || task.VideoPartID != want.partID || task.FileHash != want.fileHash || transcodes.queued[i] != [2]uint{75, want.partID} {
					t.Fatalf("persisted part/source mapping lost: task=%+v queued=%v", task, transcodes.queued)
				}
			}
		})
	}
}
