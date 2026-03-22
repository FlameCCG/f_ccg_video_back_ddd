package identity

import (
	"strings"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// User 用户实体（身份上下文的聚合根）。
//
// 状态只通过构造、业务方法和 RestoreUser 改变。HTTP 响应使用独立 DTO，不序列化本实体。
// 硬币余额只读；写入只能走 CoinAccount。
type User struct {
	refreshTokenHash   string
	mustChangePassword bool
	id                 uint
	createdAt          time.Time
	updatedAt          time.Time
	username           string
	email              string
	password           string
	avatar             string
	description        string
	gender             int
	birthday           *time.Time
	level              int
	openID             string
	registerSource     string
	exp                int64
	coinCount          float64
	status             enum.UserStatus
	currentRoleID      uint
	registerIP         string
	lastLoginIP        string
	registerAddr       string
	lastLoginAddr      string
}

// UserSnapshot 是用户的持久化快照，字段公开供基础设施重建。
type UserSnapshot struct {
	RefreshTokenHash   string
	MustChangePassword bool
	ID                 uint
	CreatedAt          time.Time
	UpdatedAt          time.Time
	Username           string
	Email              string
	Password           string
	Avatar             string
	Description        string
	Gender             int
	Birthday           *time.Time
	Level              int
	OpenID             string
	RegisterSource     string
	Exp                int64
	CoinCount          float64
	Status             enum.UserStatus
	CurrentRoleID      uint
	RegisterIP         string
	LastLoginIP        string
	RegisterAddr       string
	LastLoginAddr      string
}

// RestoreUser 从持久化快照恢复用户，不重新执行注册校验。
func RestoreUser(snapshot UserSnapshot) *User {
	return &User{
		refreshTokenHash:   snapshot.RefreshTokenHash,
		mustChangePassword: snapshot.MustChangePassword,
		id:                 snapshot.ID,
		createdAt:          snapshot.CreatedAt,
		updatedAt:          snapshot.UpdatedAt,
		username:           snapshot.Username,
		email:              snapshot.Email,
		password:           snapshot.Password,
		avatar:             snapshot.Avatar,
		description:        snapshot.Description,
		gender:             snapshot.Gender,
		birthday:           (stateCopy{}).time(snapshot.Birthday),
		level:              snapshot.Level,
		openID:             snapshot.OpenID,
		registerSource:     snapshot.RegisterSource,
		exp:                snapshot.Exp,
		coinCount:          snapshot.CoinCount,
		status:             snapshot.Status,
		currentRoleID:      snapshot.CurrentRoleID,
		registerIP:         snapshot.RegisterIP,
		lastLoginIP:        snapshot.LastLoginIP,
		registerAddr:       snapshot.RegisterAddr,
		lastLoginAddr:      snapshot.LastLoginAddr,
	}
}

// Snapshot 返回脱离实体的副本。修改返回值不会影响实体。
func (u *User) Snapshot() UserSnapshot {
	if u == nil {
		return UserSnapshot{}
	}

	return UserSnapshot{
		RefreshTokenHash:   u.refreshTokenHash,
		MustChangePassword: u.mustChangePassword,
		ID:                 u.id,
		CreatedAt:          u.createdAt,
		UpdatedAt:          u.updatedAt,
		Username:           u.username,
		Email:              u.email,
		Password:           u.password,
		Avatar:             u.avatar,
		Description:        u.description,
		Gender:             u.gender,
		Birthday:           (stateCopy{}).time(u.birthday),
		Level:              u.level,
		OpenID:             u.openID,
		RegisterSource:     u.registerSource,
		Exp:                u.exp,
		CoinCount:          u.coinCount,
		Status:             u.status,
		CurrentRoleID:      u.currentRoleID,
		RegisterIP:         u.registerIP,
		LastLoginIP:        u.lastLoginIP,
		RegisterAddr:       u.registerAddr,
		LastLoginAddr:      u.lastLoginAddr,
	}
}

// NewEmailUser 按现有邮箱注册字段构造用户，不增加新的创建限制。
func NewEmailUser(username, passwordHash, email, registerIP, registerAddr string, defaultRoleID uint) *User {
	return &User{
		username:       username,
		password:       passwordHash,
		email:          email,
		registerIP:     registerIP,
		registerAddr:   registerAddr,
		registerSource: RegisterSourceEmail,
		gender:         enum.GenderUnknown,
		currentRoleID:  defaultRoleID,
	}
}

// NewOAuthUser 按现有第三方首次登录字段构造用户，不增加新的创建限制。
func NewOAuthUser(username, email, avatar, openID, registerIP, registerAddr, registerSource string, defaultRoleID uint) *User {
	return &User{
		username:       username,
		email:          email,
		avatar:         avatar,
		openID:         openID,
		registerIP:     registerIP,
		registerAddr:   registerAddr,
		registerSource: registerSource,
		gender:         enum.GenderUnknown,
		currentRoleID:  defaultRoleID,
	}
}

func (u *User) ID() uint                 { return u.orZero().id }
func (u *User) CreatedAt() time.Time     { return u.orZero().createdAt }
func (u *User) UpdatedAt() time.Time     { return u.orZero().updatedAt }
func (u *User) Username() string         { return u.orZero().username }
func (u *User) Email() string            { return u.orZero().email }
func (u *User) Password() string         { return u.orZero().password }
func (u *User) RefreshTokenHash() string { return u.orZero().refreshTokenHash }
func (u *User) MustChangePassword() bool { return u.orZero().mustChangePassword }
func (u *User) Avatar() string           { return u.orZero().avatar }
func (u *User) Description() string      { return u.orZero().description }
func (u *User) Gender() int              { return u.orZero().gender }
func (u *User) Level() int               { return u.orZero().level }
func (u *User) OpenID() string           { return u.orZero().openID }
func (u *User) RegisterSource() string   { return u.orZero().registerSource }
func (u *User) Exp() int64               { return u.orZero().exp }
func (u *User) CoinCount() float64       { return u.orZero().coinCount }
func (u *User) Status() enum.UserStatus  { return u.orZero().status }
func (u *User) CurrentRoleID() uint      { return u.orZero().currentRoleID }
func (u *User) RegisterIP() string       { return u.orZero().registerIP }
func (u *User) LastLoginIP() string      { return u.orZero().lastLoginIP }
func (u *User) RegisterAddr() string     { return u.orZero().registerAddr }
func (u *User) LastLoginAddr() string    { return u.orZero().lastLoginAddr }

// Birthday 返回生日副本，调用方修改不会写回实体。
func (u *User) Birthday() *time.Time {
	if u == nil {
		return nil
	}

	return (stateCopy{}).time(u.birthday)
}

func (u *User) orZero() User {
	if u == nil {
		return User{}
	}

	return *u
}

// FormattedBirthday 按 2006-01-02 格式化生日；为空时返回空串。
func (u *User) FormattedBirthday() string {
	if u == nil {
		return ""
	}

	return (ProfilePolicy{}).FormatBirthday(u.birthday)
}

// EmailMatches 判断目标邮箱是否就是当前已绑定邮箱。
func (u *User) EmailMatches(email string) bool {
	return u != nil && u.email != "" && u.email == email
}

// CheckOldPassword 仅在已设密时要求旧密码正确。空旧密码不会触发匹配函数。
func (u *User) CheckOldPassword(oldPassword string, matches func(passwordHash string) bool) error {
	if u == nil || !u.RequiresOldPassword() {
		return nil
	}

	if strings.TrimSpace(oldPassword) == "" {
		return ErrOldPasswordRequired
	}

	if matches == nil || !matches(u.password) {
		return ErrOldPasswordIncorrect
	}

	return nil
}

// ReusesInitialPassword 报告强制改密用户是否仍提交了当前密码。
func (u *User) ReusesInitialPassword(newPasswordMatchesCurrent bool) bool {
	return u != nil && u.mustChangePassword && newPasswordMatchesCurrent
}

// HasTimedBan 判断用户是否处于限时封禁（不含永久封禁）。
func (u *User) HasTimedBan() bool {
	return u != nil && u.status == enum.UserStatusBanned
}

// IsBanned 判断用户当前是否处于封禁态（限时或永久）。
func (u *User) IsBanned() bool {
	if u == nil {
		return false
	}

	return u.status == enum.UserStatusBanned || u.status == enum.UserStatusPermanentlyBanned
}

// IsPermanentlyBanned 判断用户是否被永久封禁。
func (u *User) IsPermanentlyBanned() bool {
	if u == nil {
		return false
	}

	return u.status == enum.UserStatusPermanentlyBanned
}

// PlanProfile 把资料入参翻译成部分更新。失败时不修改用户。
// 用户名占用由调用方在进入本方法前判定。
func (u *User) PlanProfile(in ProfileInput) (ProfileChanges, error) {
	if u == nil {
		return ProfileChanges{}, ErrUserNotFound
	}

	var changes ProfileChanges

	if in.Username != "" && in.Username != u.username {
		username := in.Username
		changes.Username = &username
	}

	if in.Avatar != "" {
		avatar := in.Avatar
		changes.Avatar = &avatar
	}

	if in.Description != "" {
		description := in.Description
		changes.Description = &description
	}

	if in.Gender != nil {
		if err := (ProfilePolicy{}).ValidateGender(*in.Gender); err != nil {
			return ProfileChanges{}, err
		}

		gender := *in.Gender
		changes.Gender = &gender
	}

	if in.Birthday != "" {
		birthday, err := (ProfilePolicy{}).ParseBirthday(in.Birthday)
		if err != nil {
			return ProfileChanges{}, err
		}

		changes.Birthday = &birthday
	}

	if changes.IsEmpty() {
		return ProfileChanges{}, ErrNoFieldsToUpdate
	}

	return changes, nil
}

func (stateCopy) time(src *time.Time) *time.Time {
	if src == nil {
		return nil
	}

	value := *src

	return &value
}
