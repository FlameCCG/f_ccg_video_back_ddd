package identity

// ValidateUploadID 校验 fileHash，供接口层在读取 multipart 前检查上传标识。
func (s *UserBannerService) ValidateUploadID(fileHash string) error {
	return s.storage.ValidateUploadID(fileHash)
}
