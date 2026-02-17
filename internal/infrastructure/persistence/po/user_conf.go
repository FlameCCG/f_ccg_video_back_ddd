package po

import "gorm.io/datatypes"

// UserConf 用户配置表.
type UserConf struct {
	UserID        uint           `gorm:"primaryKey;unique;comment:用户ID" json:"-"`
	User          User           `gorm:"foreignKey:UserID" json:"-"`
	LikeTags      datatypes.JSON `gorm:"type:json;comment:喜欢的标签" json:"likeTags"`
	BannerID      uint           `gorm:"default:0;comment:用户主页默认横幅ID" json:"bannerId"`
	BannerURL     string         `gorm:"size:500;comment:自定义横幅URL" json:"bannerUrl"`
	OpenCollect   bool           `gorm:"default:true;comment:是否公开收藏" json:"openCollect"`
	OpenFans      bool           `gorm:"default:true;comment:是否公开粉丝" json:"openFans"`
	OpenFollow    bool           `gorm:"default:true;comment:是否公开关注" json:"openFollow"`
	OpenLikeVideo bool           `gorm:"default:true;comment:是否公开最近点赞的视频" json:"openLikeVideo"`
	OpenCoinVideo bool           `gorm:"default:true;comment:是否公开最近投币的视频" json:"openCoinVideo"`
}
