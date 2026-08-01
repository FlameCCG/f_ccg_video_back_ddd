package identity

// ValidateUploadID 暴露上传标识（fileHash）校验，供接口层在读取 multipart 前做前置校验。
//
// 原 api/user_api/upload_banner.go 先校验 fileHash、再读 FormFile：
// 顺序影响错误文案（fileHash 非法时不会回「请上传横幅文件」），故必须能单独调用。
// UploadBanner 内部仍会再校验一次，保证任何调用路径都不会漏校验。
func (s *UserBannerService) ValidateUploadID(fileHash string) error {
	return s.storage.ValidateUploadID(fileHash)
}
