package comment_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	commentdomain "f_ccg_video_back_ddd/internal/domain/comment"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// TestNormalizeDraft 校验发布评论的规则与校验顺序（登录 → 评论对象 → 内容 → 长度）。
func TestNormalizeDraft(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		draft       commentdomain.Draft
		wantErr     error
		wantType    enum.CommentType
		wantContent string
	}{
		{
			name:    "未登录优先于其它校验",
			draft:   commentdomain.Draft{VideoID: 1, DynamicID: 2, Content: ""},
			wantErr: commentdomain.ErrUserNotLogin,
		},
		{
			name:    "视频与动态同时给出",
			draft:   commentdomain.Draft{UserID: 1, VideoID: 1, DynamicID: 2, Content: "hi"},
			wantErr: commentdomain.ErrTargetRequired,
		},
		{
			name:    "视频与动态都未给出",
			draft:   commentdomain.Draft{UserID: 1, Content: "hi"},
			wantErr: commentdomain.ErrTargetRequired,
		},
		{
			name:    "纯空白且无图片视为空评论",
			draft:   commentdomain.Draft{UserID: 1, VideoID: 1, Content: "  \n\t "},
			wantErr: commentdomain.ErrContentEmpty,
		},
		{
			name:        "纯空白但有图片可发布",
			draft:       commentdomain.Draft{UserID: 1, VideoID: 1, Content: "  ", Pictures: []string{"a.png"}},
			wantType:    enum.VideoCommentType,
			wantContent: "",
		},
		{
			name:    "按 rune 计长度超限",
			draft:   commentdomain.Draft{UserID: 1, VideoID: 1, Content: strings.Repeat("评", commentdomain.MaxContentRunes+1)},
			wantErr: commentdomain.ErrContentTooLong,
		},
		{
			name:        "恰好等于长度上限可发布",
			draft:       commentdomain.Draft{UserID: 1, VideoID: 1, Content: strings.Repeat("评", commentdomain.MaxContentRunes)},
			wantType:    enum.VideoCommentType,
			wantContent: strings.Repeat("评", commentdomain.MaxContentRunes),
		},
		{
			name:        "动态评论去首尾空白",
			draft:       commentdomain.Draft{UserID: 1, DynamicID: 9, Content: "  好评  "},
			wantType:    enum.DynamicCommentType,
			wantContent: "好评",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotType, err := tc.draft.Normalize()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}

			if tc.wantErr != nil {
				return
			}

			if gotType != tc.wantType {
				t.Fatalf("commentType = %q, want %q", gotType, tc.wantType)
			}

			if got.Content != tc.wantContent {
				t.Fatalf("content = %q, want %q", got.Content, tc.wantContent)
			}
		})
	}
}

// TestDecideDelete 删除评论的授权判定分支。
func TestDecideDelete(t *testing.T) {
	t.Parallel()

	rootID := uint(7)

	cases := []struct {
		name       string
		comment    *commentdomain.Comment
		operatorID uint
		want       commentdomain.DeleteDecision
	}{
		{name: "评论不存在", comment: nil, operatorID: 1, want: commentdomain.DeleteDenied},
		{
			name:       "评论作者本人",
			comment:    commentdomain.RestoreComment(commentdomain.Snapshot{UserID: 5, VideoID: 3}),
			operatorID: 5,
			want:       commentdomain.DeleteAllowed,
		},
		{
			name:       "他人的视频评论转视频作者校验",
			comment:    commentdomain.RestoreComment(commentdomain.Snapshot{UserID: 5, VideoID: 3}),
			operatorID: 9,
			want:       commentdomain.DeleteNeedsVideoAuthorCheck,
		},
		{
			name:       "他人的动态评论转动态作者校验",
			comment:    commentdomain.RestoreComment(commentdomain.Snapshot{UserID: 5, DynamicID: 3, RootID: &rootID}),
			operatorID: 9,
			want:       commentdomain.DeleteNeedsDynamicAuthorCheck,
		},
		{
			name:       "既无视频也无动态直接拒绝",
			comment:    commentdomain.RestoreComment(commentdomain.Snapshot{UserID: 5}),
			operatorID: 9,
			want:       commentdomain.DeleteDenied,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.comment.DeletionDecision(tc.operatorID); got != tc.want {
				t.Fatalf("DecideDelete = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestValidatePinTarget 置顶目标校验：仅一级评论、且必须关联作品。
func TestValidatePinTarget(t *testing.T) {
	t.Parallel()

	rootID := uint(3)

	cases := []struct {
		name    string
		comment *commentdomain.Comment
		wantErr error
	}{
		{name: "评论不存在", comment: nil, wantErr: commentdomain.ErrCommentNotFound},
		{name: "二级评论不可置顶", comment: commentdomain.RestoreComment(commentdomain.Snapshot{VideoID: 1, RootID: &rootID}), wantErr: commentdomain.ErrPinRootOnly},
		{name: "未关联作品", comment: commentdomain.RestoreComment(commentdomain.Snapshot{}), wantErr: commentdomain.ErrPinTargetMissing},
		{name: "视频一级评论可置顶", comment: commentdomain.RestoreComment(commentdomain.Snapshot{VideoID: 1}), wantErr: nil},
		{name: "动态一级评论可置顶", comment: commentdomain.RestoreComment(commentdomain.Snapshot{DynamicID: 1}), wantErr: nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := tc.comment.EnsurePinnable(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("ValidatePinTarget = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// TestResolveRootID 回复层级恒定为两级：父评论是根则取父 ID，否则沿用父的根 ID。
func TestResolveRootID(t *testing.T) {
	t.Parallel()

	grandRoot := uint(10)

	if got := (commentdomain.ParentBrief{RootID: nil}).RootForReply(20); got == nil || *got != 20 {
		t.Fatalf("回复根评论时 rootID = %v, want 20", got)
	}

	if got := (commentdomain.ParentBrief{RootID: &grandRoot}).RootForReply(20); got == nil || *got != 10 {
		t.Fatalf("回复二级评论时 rootID = %v, want 10", got)
	}
}

// TestNormalizeAtUserIDs @ 名单规范化：剔除 0 与自己、去重并保持顺序。
func TestNormalizeAtUserIDs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		actorID uint
		input   []uint
		want    []uint
	}{
		{name: "未登录返回空", actorID: 0, input: []uint{2, 3}, want: nil},
		{name: "空名单返回空", actorID: 1, input: nil, want: nil},
		{name: "全是自己与 0 返回空", actorID: 1, input: []uint{1, 0, 1}, want: nil},
		{name: "去重并保持顺序", actorID: 1, input: []uint{5, 3, 5, 0, 1, 3}, want: []uint{5, 3}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := (commentdomain.Draft{UserID: tc.actorID, AtUserIDs: tc.input}).NormalizeAtUserIDs(); !slices.Equal(got, tc.want) {
				t.Fatalf("NormalizeAtUserIDs = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestNormalizePaging 分页纠偏：前台上限 50、管理端上限 100，越界回落默认值。
func TestNormalizePaging(t *testing.T) {
	t.Parallel()

	cases := []struct {
		page, pageSize           int
		wantPage, wantSize       int
		adminWantPage, adminSize int
	}{
		{page: 0, pageSize: 0, wantPage: 1, wantSize: commentdomain.DefaultPageSize, adminWantPage: 1, adminSize: commentdomain.DefaultPageSize},
		{page: -3, pageSize: 10, wantPage: 1, wantSize: 10, adminWantPage: 1, adminSize: 10},
		{page: 2, pageSize: commentdomain.MaxPageSize, wantPage: 2, wantSize: commentdomain.MaxPageSize, adminWantPage: 2, adminSize: commentdomain.MaxPageSize},
		{page: 2, pageSize: 80, wantPage: 2, wantSize: commentdomain.DefaultPageSize, adminWantPage: 2, adminSize: 80},
		{page: 2, pageSize: 200, wantPage: 2, wantSize: commentdomain.DefaultPageSize, adminWantPage: 2, adminSize: commentdomain.DefaultPageSize},
	}

	for _, tc := range cases {
		page, size := (commentdomain.PagePolicy{}).Normalize(tc.page, tc.pageSize)
		if page != tc.wantPage || size != tc.wantSize {
			t.Fatalf("NormalizePaging(%d, %d) = (%d, %d), want (%d, %d)",
				tc.page, tc.pageSize, page, size, tc.wantPage, tc.wantSize)
		}

		page, size = (commentdomain.PagePolicy{}).NormalizeAdmin(tc.page, tc.pageSize)
		if page != tc.adminWantPage || size != tc.adminSize {
			t.Fatalf("NormalizeAdminPaging(%d, %d) = (%d, %d), want (%d, %d)",
				tc.page, tc.pageSize, page, size, tc.adminWantPage, tc.adminSize)
		}
	}
}
