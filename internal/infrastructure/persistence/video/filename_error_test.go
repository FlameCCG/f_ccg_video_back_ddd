package video

import (
	"errors"
	"strings"
	"testing"

	videodomain "f_ccg_video_back_ddd/internal/domain/video"
)

func TestSourceFilenameErrorTranslation(t *testing.T) {
	dbErr := errors.New("Error 1406: Data too long for column 'source_file_name'")
	longName := strings.Repeat("字", 256)
	if got := mapSourceFileNameDBError(dbErr, longName); got.Error() != (videodomain.UploadPolicy{}).SourceFileName(longName).Error() {
		t.Fatal(got)
	}
	if got := mapSourceFileNameDBError(dbErr, "short.mp4"); got.Error() != "源文件名过长（最多 255 个字符）：请缩短文件名后重新选择并上传" {
		t.Fatal(got)
	}
	other := errors.New("Data too long for column 'title'")
	if mapSourceFileNameDBError(other, longName) != other || mapSourceFileNameDBError(nil, "") != nil {
		t.Fatal("unrelated error changed")
	}
}
