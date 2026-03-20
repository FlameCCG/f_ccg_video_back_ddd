package entity_test

import (
	"errors"
	"testing"

	videoerrors "f_ccg_video_back_ddd/internal/domain/video/domainerr"
	"f_ccg_video_back_ddd/internal/domain/video/entity"
)

func TestDanmuConstructorProtectsContentAndDefaults(t *testing.T) {
	danmu, err := entity.NewDanmu(1, entity.SendDanmuCommand{VideoID: 2, Content: "  "})
	if danmu != nil || !errors.Is(err, videoerrors.ErrDanmuContentEmpty) {
		t.Fatalf("invalid content must not produce entity: danmu=%+v err=%v", danmu, err)
	}
	danmu, err = entity.NewDanmu(1, entity.SendDanmuCommand{VideoID: 2, VideoPartID: 3, Content: " 内容 "})
	if err != nil || danmu.Content != "内容" || danmu.Color != entity.DefaultDanmuColor || danmu.FontSize != entity.DefaultDanmuFontSize || danmu.VideoPartID != 3 {
		t.Fatalf("constructor must normalize together: danmu=%+v err=%v", danmu, err)
	}
}
