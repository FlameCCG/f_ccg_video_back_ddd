package video_test

import (
	"errors"
	"testing"

	videodomain "f_ccg_video_back_ddd/internal/domain/video"
)

func TestDanmuConstructorProtectsContentAndDefaults(t *testing.T) {
	danmu, err := videodomain.NewDanmu(1, videodomain.SendDanmuCommand{VideoID: 2, Content: "  "})
	if danmu != nil || !errors.Is(err, videodomain.ErrDanmuContentEmpty) {
		t.Fatalf("invalid content must not produce entity: danmu=%+v err=%v", danmu, err)
	}
	danmu, err = videodomain.NewDanmu(1, videodomain.SendDanmuCommand{VideoID: 2, VideoPartID: 3, Content: " 内容 "})
	if err != nil || danmu.Content() != "内容" || danmu.Color() != videodomain.DefaultDanmuColor || danmu.FontSize() != videodomain.DefaultDanmuFontSize || danmu.VideoPartID() != 3 {
		t.Fatalf("constructor must normalize together: danmu=%+v err=%v", danmu, err)
	}
}
