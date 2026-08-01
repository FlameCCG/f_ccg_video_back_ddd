package router

import (
	videohandler "f_ccg_video_back_ddd/internal/interfaces/http/handler/video"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"github.com/gin-gonic/gin"
)

// registerVideoCommonRoutes 装配 /v1/common/video 路由（原 router/video_router.go，method + path 逐行一致）。
func registerVideoCommonRoutes(
	g *gin.RouterGroup,
	mw *middlewares.Middlewares,
	h *videohandler.Handler,
) {
	videoGroup := g.Group("/video")
	{
		videoGroup.GET("home", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), middlewares.BindQueryParams[videohandler.HomeFeedReq], h.HomeFeedView)
		videoGroup.GET("home/search/top", middlewares.RequestTimeout(ctxutil.RequestFastTimeout), h.HomeSearchTopView)
		videoGroup.GET("hot", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), middlewares.BindQueryParams[videohandler.VideoHotReq], h.HotView)
		videoGroup.GET("rank", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), middlewares.BindQueryParams[videohandler.VideoRankReq], h.RankView)
		videoGroup.GET("search", middlewares.RequestTimeout(ctxutil.RequestSlowTimeout), middlewares.BindQueryParams[videohandler.VideoSearchReq], h.VideoSearchView)
		videoGroup.GET("search/suggest", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), middlewares.BindQueryParams[videohandler.VideoSuggestReq], h.SuggestView)
		videoGroup.GET("detail", middlewares.RequestTimeout(ctxutil.RequestSlowTimeout), middlewares.BindQueryParams[videohandler.VideoDetailReq], h.DetailView)
		videoGroup.GET("recommend", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), middlewares.BindQueryParams[videohandler.VideoRecommendReq], h.RecommendView)
		videoGroup.GET("partitions", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), h.ListSubmittablePartitionsView)
		videoGroup.POST("view", middlewares.RequestTimeout(ctxutil.RequestFastTimeout), mw.JwtOptional(), middlewares.JsonBindParams[videohandler.VideoViewReq], h.ViewView)
		videoGroup.GET("danmu/history",
			middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout),
			mw.JwtOptional(),
			middlewares.BindQueryParams[videohandler.DanmuHistoryReq],
			h.DanmuHistoryView)
		videoGroup.GET("danmu/room/list",
			middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout),
			mw.JwtOptional(),
			middlewares.BindQueryParams[videohandler.DanmuRoomListReq],
			h.DanmuRoomListView)
		videoGroup.GET("danmu/ws", middlewares.BindQueryParams[videohandler.DanmuWSReq], h.DanmuWSView)
	}

	authGroup := videoGroup.Group("")
	{
		// 互动操作 (点赞、投币、收藏)
		authGroup.POST("like", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), mw.JwtAuth(), middlewares.JsonBindParams[videohandler.VideoLikeReq], h.LikeView)
		authGroup.POST("coin", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), mw.JwtAuth(), middlewares.JsonBindParams[videohandler.VideoCoinReq], h.CoinView)
		authGroup.POST("favorite", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), mw.JwtAuth(), middlewares.JsonBindParams[videohandler.VideoFavoriteReq], h.FavoriteView)
		authGroup.POST("triple", middlewares.RequestTimeout(ctxutil.RequestSlowTimeout), mw.JwtAuth(), middlewares.JsonBindParams[videohandler.VideoTripleReq], h.TripleView)
		authGroup.POST("report", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), mw.JwtAuth(), middlewares.BindParams[videohandler.VideoReportReq], h.ReportVideoView)
		authGroup.POST("danmu/send",
			middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout),
			mw.JwtAuth(),
			mw.UserBanCheck(),
			middlewares.JsonBindParams[videohandler.DanmuSendReq],
			h.SendDanmuView,
		)
		authGroup.POST("danmu/report", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), mw.JwtAuth(), middlewares.JsonBindParams[videohandler.DanmuReportReq], h.ReportDanmuView)
		authGroup.POST("danmu/like", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), mw.JwtAuth(), middlewares.JsonBindParams[videohandler.DanmuLikeReq], h.DanmuLikeView)
		authGroup.DELETE("danmu", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), mw.JwtAuth(), middlewares.JsonBindParams[videohandler.DeleteDanmuReq], h.DeleteDanmuView)
		authGroup.GET("danmu/creator/list",
			middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout),
			mw.JwtAuth(),
			middlewares.BindQueryParams[videohandler.CreatorDanmuListReq],
			h.CreatorDanmuListView,
		)

		// 用户个人数据 (历史记录、收藏夹)
		authGroup.POST("history", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), mw.JwtAuth(), middlewares.JsonBindParams[videohandler.SaveHistoryReq], h.SaveHistoryView)
		authGroup.GET("history/list", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), mw.JwtAuth(), middlewares.BindQueryParams[videohandler.HistoryListReq], h.GetVideoHistoryView)
		authGroup.DELETE("history", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), mw.JwtAuth(), middlewares.JsonBindParams[videohandler.DeleteHistoryReq], h.DeleteHistoryView)
		authGroup.POST("folder", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), mw.JwtAuth(), middlewares.JsonBindParams[videohandler.CreateFolderReq], h.CreateFolderView)
		authGroup.DELETE("folder", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), mw.JwtAuth(), middlewares.JsonBindParams[videohandler.DeleteFolderReq], h.DeleteFolderView)
		authGroup.GET("folder", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), mw.JwtAuth(), middlewares.BindQueryParams[videohandler.FolderListReq], h.FolderListView)
		authGroup.GET("folder/videos", middlewares.RequestTimeout(ctxutil.RequestDefaultTimeout), mw.JwtAuth(), middlewares.BindQueryParams[videohandler.FolderVideosReq], h.FolderVideosView)

		// 投稿发布与管理
		authGroup.POST("image/upload", middlewares.RequestTimeout(ctxutil.RequestUploadTimeout), mw.JwtAuth(), h.UploadImageView)
		authGroup.POST("publish",
			middlewares.RequestTimeout(ctxutil.RequestLongTimeout),
			mw.JwtAuth(),
			mw.UserBanCheck(),
			middlewares.JsonBindParams[videohandler.PublishVideoReq],
			h.PublishVideoView,
		)
		authGroup.PUT("publish",
			middlewares.RequestTimeout(ctxutil.RequestSlowTimeout),
			mw.JwtAuth(),
			mw.UserBanCheck(),
			middlewares.JsonBindParams[videohandler.UpdateVideoReq],
			h.UpdateVideoView,
		)

		// 分片上传子路由
		uploadGroup := authGroup.Group("upload")
		{
			uploadGroup.POST("chunk",
				middlewares.RequestTimeout(ctxutil.RequestUploadTimeout),
				mw.JwtAuth(),
				mw.UserBanCheck(),
				middlewares.BindParams[videohandler.UploadChunkReq],
				h.UploadChunkView,
			)
			uploadGroup.GET("status", middlewares.RequestTimeout(ctxutil.RequestSlowTimeout), mw.JwtAuth(), middlewares.BindQueryParams[videohandler.ChunkStatusReq], h.ChunkStatusView)
			uploadGroup.POST("complete",
				middlewares.RequestTimeout(ctxutil.RequestUploadTimeout),
				mw.JwtAuth(),
				mw.UserBanCheck(),
				middlewares.JsonBindParams[videohandler.CompleteUploadReq],
				h.CompleteUploadView,
			)
		}
	}
}

// registerVideoAdminRoutes 装配 /v1/admin/video 路由（原 router/video_admin_router.go，method + path 逐行一致）。
func registerVideoAdminRoutes(
	g *gin.RouterGroup,
	mw *middlewares.Middlewares,
	h *videohandler.AdminHandler,
) {
	videoGroup := g.Group("/video")
	{
		slowGroup := videoGroup.Group("")
		slowGroup.Use(middlewares.RequestTimeout(ctxutil.RequestSlowTimeout), mw.AuthAll())
		{
			slowGroup.GET("detail", middlewares.BindQueryParams[videohandler.AdminVideoDetailReq], h.AdminDetailView)
			slowGroup.GET("list", middlewares.BindQueryParams[videohandler.AdminVideoListReq], h.AdminListView)
			slowGroup.GET("report/list", middlewares.BindQueryParams[videohandler.AdminReportListReq], h.AdminReportListView)
			slowGroup.PUT("report/handle", middlewares.JsonBindParams[videohandler.AdminReportHandleReq], h.AdminReportHandleView)
			slowGroup.PUT("review", middlewares.JsonBindParams[videohandler.AdminVideoReviewReq], h.AdminReviewView)
			slowGroup.PUT("restore", middlewares.JsonBindParams[videohandler.AdminRestoreVideoReq], h.AdminRestoreVideoView)
			slowGroup.GET("danmu/list", middlewares.BindQueryParams[videohandler.AdminDanmuListReq], h.AdminDanmuListView)
			slowGroup.DELETE("danmu", middlewares.JsonBindParams[videohandler.AdminDeleteDanmuReq], h.AdminDeleteDanmuView)
			slowGroup.POST("partition", middlewares.JsonBindParams[videohandler.CreatePartitionReq], h.CreatePartitionView)
			slowGroup.PUT("partition", middlewares.JsonBindParams[videohandler.UpdatePartitionReq], h.UpdatePartitionView)
			slowGroup.DELETE("partition", middlewares.JsonBindParams[videohandler.DeletePartitionReq], h.DeletePartitionView)
			slowGroup.GET("partitions", middlewares.BindQueryParams[videohandler.PartitionListReq], h.ListPartitionView)
			slowGroup.GET(
				"transcode/progress",
				middlewares.BindQueryParams[videohandler.AdminTranscodeProgressReq],
				h.AdminTranscodeProgressView,
			)
			slowGroup.POST(
				"vector/rebuild",
				middlewares.JsonBindParams[videohandler.AdminRebuildVectorIndexReq],
				h.AdminRebuildVectorIndexView,
			)
			slowGroup.GET("vector/status", h.AdminVectorIndexStatusView)
		}

		// SSE: long-lived; JWT via query token for EventSource; Casbin same as other admin routes.
		sseGroup := videoGroup.Group("")
		sseGroup.Use(
			middlewares.RequestTimeout(ctxutil.RequestTranscodeProgressSSETimeout),
			mw.AuthAllWithQueryToken("token"),
		)
		{
			sseGroup.GET(
				"transcode/progress/stream",
				middlewares.BindQueryParams[videohandler.AdminTranscodeProgressReq],
				h.AdminTranscodeProgressStreamView,
			)
		}

		heavyGroup := videoGroup.Group("")
		heavyGroup.Use(middlewares.RequestTimeout(ctxutil.RequestHeavyTimeout), mw.AuthAll())
		{
			heavyGroup.DELETE("delete", middlewares.JsonBindParams[videohandler.AdminDeleteVideoReq], h.AdminDeleteVideoView)
		}
	}
}
