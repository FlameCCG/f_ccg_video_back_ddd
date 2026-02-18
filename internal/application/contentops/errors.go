package contentops

import "errors"

// 内容运营上下文的应用层错误。
//
// interfaces 层将其映射为对外响应文案，修改措辞时需遵守接口契约。
var (
	// ErrBannerPartitionFilterUnsupported 只有首页类型的轮播图支持按分区筛选。
	ErrBannerPartitionFilterUnsupported = errors.New("只有类型1支持按分区筛选")
	// ErrSiteConfigNotInitialized 站点配置未初始化。
	ErrSiteConfigNotInitialized = errors.New("站点配置未初始化")
	// ErrConfigItemUnsupported 不支持的配置项名称。
	ErrConfigItemUnsupported = errors.New("不支持的配置项")
	// ErrConfigFileRead 读取配置文件失败。
	ErrConfigFileRead = errors.New("读取配置文件失败")
	// ErrConfigFileWrite 更新配置文件失败。
	ErrConfigFileWrite = errors.New("更新配置文件失败")
)
