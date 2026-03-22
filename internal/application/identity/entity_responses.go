package identity

import (
	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"time"
)

// UserDTO preserves the existing response contract.
type UserDTO struct {
	RefreshTokenHash   string          `json:"-"`
	MustChangePassword bool            `json:"mustChangePassword,omitempty"`
	ID                 uint            `json:"id"`
	CreatedAt          time.Time       `json:"-"`
	UpdatedAt          time.Time       `json:"-"`
	Username           string          `json:"username"`
	Email              string          `json:"email"`
	Password           string          `json:"-"`
	Avatar             string          `json:"avatar"`
	Description        string          `json:"description"`
	Gender             int             `json:"gender"`
	Birthday           *time.Time      `json:"birthday"`
	Level              int             `json:"level"`
	OpenID             string          `json:"openID"`
	RegisterSource     string          `json:"registerSource"`
	Exp                int64           `json:"exp"`
	CoinCount          float64         `json:"coinCount"`
	Status             enum.UserStatus `json:"status"`
	CurrentRoleID      uint            `json:"currentRoleID"`
	RegisterIP         string          `json:"registerIP"`
	LastLoginIP        string          `json:"lastLoginIP"`
	RegisterAddr       string          `json:"registerAddr"`
	LastLoginAddr      string          `json:"lastLoginAddr"`
}

func ToUserDTO(value *identitydomain.User) *UserDTO {
	if value == nil {
		return nil
	}
	return &UserDTO{
		MustChangePassword: value.MustChangePassword(),
		ID:                 value.ID(),
		Username:           value.Username(),
		Email:              value.Email(),
		Avatar:             value.Avatar(),
		Description:        value.Description(),
		Gender:             value.Gender(),
		Birthday:           value.Birthday(),
		Level:              value.Level(),
		OpenID:             value.OpenID(),
		RegisterSource:     value.RegisterSource(),
		Exp:                value.Exp(),
		CoinCount:          value.CoinCount(),
		Status:             value.Status(),
		CurrentRoleID:      value.CurrentRoleID(),
		RegisterIP:         value.RegisterIP(),
		LastLoginIP:        value.LastLoginIP(),
		RegisterAddr:       value.RegisterAddr(),
		LastLoginAddr:      value.LastLoginAddr(),
	}
}

// BanRecordDTO preserves the existing response contract.
type BanRecordDTO struct {
	ID         uint            `json:"id"`
	CreatedAt  time.Time       `json:"-"`
	UpdatedAt  time.Time       `json:"-"`
	UserID     uint            `json:"userId"`
	Status     enum.UserStatus `json:"status"`
	Reason     string          `json:"reason"`
	Days       int             `json:"days"`
	StartAt    time.Time       `json:"startAt"`
	EndAt      *time.Time      `json:"endAt,omitempty"`
	OperatorID uint            `json:"operatorId"`
	IsUnbanned bool            `json:"isUnbanned"`
}

func ToBanRecordDTO(value *identitydomain.BanRecord) *BanRecordDTO {
	if value == nil {
		return nil
	}
	return &BanRecordDTO{
		ID:         value.ID(),
		UserID:     value.UserID(),
		Status:     value.Status(),
		Reason:     value.Reason(),
		Days:       value.Days(),
		StartAt:    value.StartAt(),
		EndAt:      value.EndAt(),
		OperatorID: value.OperatorID(),
		IsUnbanned: value.IsUnbanned(),
	}
}
