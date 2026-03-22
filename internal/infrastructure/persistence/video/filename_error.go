package video

import (
	videodomain "f_ccg_video_back_ddd/internal/domain/video"
	"fmt"
	"strings"
)

// mapSourceFileNameDBError 将源文件名列超长等 DB 错误转为可读业务文案。
func mapSourceFileNameDBError(err error, sourceFileName string) error {
	if err == nil {
		return nil
	}

	msg := err.Error()
	if strings.Contains(msg, "source_file_name") &&
		(strings.Contains(msg, "Data too long") || strings.Contains(msg, "1406")) {
		if vErr := (videodomain.UploadPolicy{}).SourceFileName(sourceFileName); vErr != nil {
			return vErr
		}

		return fmt.Errorf(
			"源文件名过长（最多 %d 个字符）：请缩短文件名后重新选择并上传",
			videodomain.MaxSourceFileNameLen,
		)
	}

	return err
}
