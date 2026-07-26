package po

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// User 用户.
type User struct {
	Model
	Username    string     `gorm:"size:50;not null;uniqueIndex;comment:用户名" json:"username"`
	Email       string     `gorm:"size:100;not null;uniqueIndex;comment:邮箱" json:"email"`
	Password    string     `gorm:"size:255;not null;comment:密码" json:"-"`
	Avatar      string     `gorm:"size:500;comment:头像URL" json:"avatar"`
	Description string     `gorm:"size:500;comment:个人简介" json:"description"`
	Gender      int        `gorm:"default:0;comment:性别 0-未知 1-男 2-女" json:"gender"`
	Birthday    *time.Time `gorm:"comment:生日" json:"birthday"`
	Level       int        `gorm:"default:1;comment:用户等级" json:"level"`
	// OpenID 与 RegisterSource 组合唯一（见 FlagDB 普通唯一索引 uk_users_open_id_source）。
	// 邮箱等非 OAuth 注册应将 open_id 置为 NULL（空串会在 BeforeCreate/BeforeSave 规范为 NULL），
	// 以便多行共用「无 open_id」而不触发唯一冲突。
	OpenID         string           `gorm:"size:50;comment:第三方登录ID" json:"openID"`
	RegisterSource string           `gorm:"size:50;comment:注册来源" json:"registerSource"`
	Exp            int64            `gorm:"default:0;comment:经验值" json:"exp"`
	CoinCount      float64          `gorm:"type:decimal(10,1);default:0;comment:硬币数量" json:"coinCount"`
	Status         enum.UserStatus  `gorm:"default:1;comment:状态 1-正常 2-封禁 3-永久封禁" json:"status"`
	CurrentRoleID  uint             `gorm:"default:0;comment:当前角色ID" json:"currentRoleID"`
	RegisterIP     string           `gorm:"size:20;comment:注册IP" json:"registerIP"`
	LastLoginIP    string           `gorm:"size:20;comment:最后登录IP" json:"lastLoginIP"`
	RegisterAddr   string           `gorm:"size:200;comment:注册时的地址" json:"registerAddr"`
	LastLoginAddr  string           `gorm:"size:200;comment:最后登录时的地址" json:"lastLoginAddr"`
	Roles          []*Role          `gorm:"many2many:user_roles;" json:"roles,omitempty"`
	UserConf       *UserConf        `gorm:"foreignKey:UserID" json:"userConf,omitempty"`
	Videos         []Video          `gorm:"foreignKey:AuthorUsername;references:Username" json:"videos,omitempty"`
	Comments       []Comment        `gorm:"foreignKey:UserID" json:"comments,omitempty"`
	Folders        []FavoriteFolder `gorm:"foreignKey:UserID" json:"folders,omitempty"`
}

// TableName 方法.
func (User) TableName() string {
	return "users"
}

// BeforeCreate 将空 open_id 写为 NULL，配合 uk_users_open_id_source 允许多个邮箱用户。
func (u *User) BeforeCreate(tx *gorm.DB) error {
	return u.normalizeOpenIDColumn(tx)
}

// BeforeSave 更新时同样规范空 open_id，避免把 NULL 又写回空串后撞唯一索引。
func (u *User) BeforeSave(tx *gorm.DB) error {
	return u.normalizeOpenIDColumn(tx)
}

func (u *User) normalizeOpenIDColumn(tx *gorm.DB) error {
	if u == nil || tx == nil {
		return nil
	}

	// map 形式的部分更新（Update("col", v) / Updates(map{...})）：
	// 只有本次语句确实要写 open_id 时才规范化。
	//
	// 原实现无条件调用 SetColumn，而 SetColumn 对 map 目标是直接写入 Dest，
	// 于是任何与 open_id 无关的部分更新（如硬币变动 Update("coin_count", x)）
	// 都会额外拼上 open_id = NULL，静默清空 OAuth 用户的第三方绑定。
	// 见 report.md FIND-005【行为变更】。
	if dest, ok := tx.Statement.Dest.(map[string]any); ok {
		raw, exists := dest["open_id"]
		if !exists {
			return nil
		}

		if openID, isString := raw.(string); isString && strings.TrimSpace(openID) == "" {
			dest["open_id"] = nil
		}

		return nil
	}

	if strings.TrimSpace(u.OpenID) != "" {
		return nil
	}

	u.OpenID = ""
	// 强制写 SQL NULL（string 零值默认是 ''，会破坏「多行空 open_id」语义）。
	tx.Statement.SetColumn("open_id", nil)

	return nil
}

// AfterCreate 创建用户后，自动创建用户配置.
//
// 默认横幅 ID 来自 ctx（由仓储层从注入的配置写入，见 WithDefaultUserBannerID）；
// 原实现在此直接读 global.Config()，新架构禁止 PO 反向依赖全局配置。
func (u User) AfterCreate(tx *gorm.DB) error {
	userConf := UserConf{
		UserID:   u.ID,
		BannerID: DefaultUserBannerIDFromContext(tx.Statement.Context),
	}
	if err := tx.Create(&userConf).Error; err != nil {
		zap.L().Error("create user conf failed", zap.Error(err))
		return err
	}
	// 默认收藏夹
	defaultFolder := FavoriteFolder{
		UserID:    u.ID,
		Name:      "默认收藏夹",
		IsDefault: true,
	}
	if err := tx.Create(&defaultFolder).Error; err != nil {
		zap.L().Error("create default favorite folder failed", zap.Error(err))
		return err
	}

	return nil
}
