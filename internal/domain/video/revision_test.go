package video_test

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	domain "f_ccg_video_back_ddd/internal/domain/video"
)

func TestReviseVisibility(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC)
	future, past := now.Add(time.Hour), now.Add(-time.Hour)

	public, private := false, true
	//nolint:paralleltest // Cases share parent fixtures or must finish before their parent resource cleanup.
	for _, tt := range []struct {
		name    string
		from    enum.VideoStatus
		privacy *bool
		when    *time.Time
		review  bool
		want    enum.VideoStatus
	}{
		{"metadata preserves review", enum.VideoStatusReview, nil, nil, false, enum.VideoStatusReview},
		{"time only schedules public", enum.VideoStatusPublished, nil, &future, false, enum.VideoStatusScheduled},
		{"time only still needs review", enum.VideoStatusPublished, nil, &future, true, enum.VideoStatusReview},
		{"time only preserves privacy", enum.VideoStatusPrivate, nil, &future, false, enum.VideoStatusPrivate},
		{"make private", enum.VideoStatusPublished, &private, &future, false, enum.VideoStatusPrivate},
		{"publish private", enum.VideoStatusPrivate, &public, nil, false, enum.VideoStatusPublished},
		{"publish needs review", enum.VideoStatusPrivate, &public, nil, true, enum.VideoStatusReview},
		{"reschedule to past", enum.VideoStatusScheduled, nil, &past, false, enum.VideoStatusPublished},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v := domain.Video{ID: 1, AuthorUsername: testAlice, Status: tt.from, Views: 99}
			{
				err := v.Revise(testAlice, domain.Revision{IsPrivate: tt.privacy, PublishTime: tt.when}, now, tt.review)
				require.NoError(t, err)
			}

			require.Equal(t, tt.want, v.Status, "status=%v, views=%d; want %v, 99", v.Status, v.Views, tt.want)
			require.EqualValues(t, 99, v.Views, "status=%v, views=%d; want %v, 99", v.Status, v.Views, tt.want)
		})
	}
}

func TestRevisePreservesPublicationTypeAndClearsApprovalWhenResubmitted(t *testing.T) {
	t.Parallel()

	now := time.Now()
	approved, future := now.Add(-time.Hour), now.Add(time.Hour)
	v := &domain.Video{ID: 1, AuthorUsername: testAlice, Status: enum.VideoStatusScheduled, IsScheduled: true, ApprovedAt: &approved}

	title := "new title"
	{
		err := v.Revise(testAlice, domain.Revision{Title: &title}, now, true)
		require.NoError(t, err)
	}

	require.True(t, v.IsScheduled, "metadata revision lost publication info: %+v", v)
	require.NotNil(t, v.ApprovedAt, "metadata revision lost publication info: %+v", v)
	require.True(t, v.ApprovedAt.Equal(approved), "metadata revision lost publication info: %+v", v)

	{
		err := v.Revise(testAlice, domain.Revision{PublishTime: &future}, now, true)
		require.NoError(t, err)
	}

	require.True(t, v.IsScheduled, "resubmission retained stale approval: %+v", v)
	require.Equal(t, enum.VideoStatusReview, v.Status, "resubmission retained stale approval: %+v", v)
	require.Nil(t, v.ApprovedAt, "resubmission retained stale approval: %+v", v)

	private := true
	{
		err := v.Revise(testAlice, domain.Revision{IsPrivate: &private}, now, true)
		require.NoError(t, err)
	}

	require.False(t, v.IsScheduled, "cancelled scheduling retained current publication type: %+v", v)
	require.Equal(t, enum.VideoStatusPrivate, v.Status, "cancelled scheduling retained current publication type: %+v", v)
}

func TestReviseRejectsInvalidAggregateWithoutMutation(t *testing.T) {
	t.Parallel()

	title, public := "changed", false

	//nolint:paralleltest // Cases share parent fixtures or must finish before their parent resource cleanup.
	for _, tt := range []struct {
		name   string
		video  *domain.Video
		author string
		want   error
	}{
		{"missing", nil, testAlice, domain.ErrRecordNotFound},
		{"deleted cannot revive", &domain.Video{AuthorUsername: testAlice, Status: enum.VideoStatusDeleted}, testAlice, domain.ErrRecordNotFound},
		{"different author", &domain.Video{AuthorUsername: testAlice, Status: enum.VideoStatusPublished}, "bob", domain.ErrRecordNotFound},
		{"empty author", &domain.Video{Status: enum.VideoStatusPublished}, "", domain.ErrRecordNotFound},
		{"unknown state", &domain.Video{AuthorUsername: testAlice, Status: 99}, testAlice, domain.ErrVideoStatusInvalid},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var before domain.Video
			if tt.video != nil {
				before = *tt.video
			}

			err := tt.video.Revise(tt.author, domain.Revision{Title: &title, IsPrivate: &public}, time.Now(), false)
			require.ErrorIs(t, err, tt.want, "error=%v, want %v", err, tt.want)

			require.False(t, tt.video != nil && *tt.video != before, "rejected revision mutated video")
		})
	}
}
