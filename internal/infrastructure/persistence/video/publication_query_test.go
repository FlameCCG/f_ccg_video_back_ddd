package video_test

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	app "f_ccg_video_back_ddd/internal/application/video"

	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/video"
)

const (
	testNew = "new"
)

func TestAdminPublicationFieldsAndExplicitImmediateFilter(t *testing.T) {
	t.Parallel()

	f := newRevisionFixture(t)
	{
		err := f.db.AutoMigrate(&po.User{}, &po.VideoResource{}, &po.Part{}, &po.VideoPartResource{})
		require.NoError(t, err)
	}

	f.adminVideos = infra.NewAdminVideoQuery(f.db)
	f.detail = infra.NewDetailQuery(f.db)
	f.createUsers(t, po.User{Username: testAuthorAlice})

	planned := time.Now().Add(-time.Hour)
	approved := planned.Add(-time.Minute)
	scheduled := po.Video{Title: "scheduled", AuthorUsername: testAuthorAlice, Status: enum.VideoStatusPublished, IsScheduled: true, PublishAt: &planned, ApprovedAt: &approved}

	immediate := po.Video{Title: "immediate", AuthorUsername: testAuthorAlice, Status: enum.VideoStatusPublished}
	for _, row := range []*po.Video{&scheduled, &immediate} {
		{
			err := f.session().Create(row).Error
			require.NoError(t, err)
		}
	}

	for _, filter := range []bool{true, false} {
		items, total, err := f.adminVideos.List(t.Context(), app.AdminVideoListParams{IsScheduled: &filter, Page: 1, PageSize: 10})
		require.NoError(t, err, "filter=%v, items=%+v, total=%d, err=%v", filter, items, total, err)
		require.EqualValues(t, 1, total, "filter=%v, items=%+v, total=%d, err=%v", filter, items, total, err)
		require.Len(t, items, 1, "filter=%v, items=%+v, total=%d, err=%v", filter, items, total, err)
		require.Equal(t, items[0].IsScheduled, filter, "filter=%v, items=%+v, total=%d, err=%v", filter, items, total, err)

		require.False(t, filter && (items[0].PublishAt == nil || !items[0].PublishAt.Equal(planned) || items[0].ApprovedAt == nil || !items[0].ApprovedAt.Equal(approved)), "publication dates lost: %+v", items[0])

		require.False(t, !filter && (items[0].PublishAt != nil || items[0].ApprovedAt != nil), "immediate video acquired fake timestamps: %+v", items[0])
	}

	meta, _, err := f.detail.LoadAny(t.Context(), scheduled.ID)
	require.NoError(t, err, "admin detail lost publication info: meta=%+v, err=%v", meta, err)
	require.True(t, meta.IsScheduled, "admin detail lost publication info: meta=%+v, err=%v", meta, err)
	require.NotNil(t, meta.PublishAt, "admin detail lost publication info: meta=%+v, err=%v", meta, err)
	require.True(t, meta.PublishAt.Equal(planned), "admin detail lost publication info: meta=%+v, err=%v", meta, err)
	require.NotNil(t, meta.ApprovedAt, "admin detail lost publication info: meta=%+v, err=%v", meta, err)
	require.True(t, meta.ApprovedAt.Equal(approved), "admin detail lost publication info: meta=%+v, err=%v", meta, err)
}
