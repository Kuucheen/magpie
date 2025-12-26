package domain

import (
	"strings"
	"time"

	"magpie/internal/security"

	"gorm.io/gorm"
)

type RotatingProxy struct {
	ID                      uint64     `gorm:"primaryKey;autoIncrement"`
	UserID                  uint       `gorm:"not null;index:idx_rotating_user_name,priority:1"`
	Name                    string     `gorm:"not null;size:120;index:idx_rotating_user_name,priority:2"`
	ProtocolID              int        `gorm:"not null;index"`
	Protocol                Protocol   `gorm:"foreignKey:ProtocolID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	ListenProtocolID        int        `gorm:"index"`
	ListenProtocol          Protocol   `gorm:"foreignKey:ListenProtocolID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	TransportProtocol       string     `gorm:"not null;default:'tcp'"`
	ListenTransportProtocol string     `gorm:"not null;default:'tcp'"`
	ListenPort              uint16     `gorm:"uniqueIndex"`
	AuthRequired            bool       `gorm:"not null;default:false"`
	AuthUsername            string     `gorm:"size:120;default:''"`
	AuthPassword            string     `gorm:"-" json:"-"`
	AuthPasswordEncrypted   string     `gorm:"column:auth_password;default:''"`
	ReputationLabels        StringList `gorm:"type:jsonb;default:'[]'"`
	LastProxyID             *uint64    `gorm:"column:last_proxy_id"`
	LastRotationAt          *time.Time
	CreatedAt               time.Time `gorm:"autoCreateTime"`
	UpdatedAt               time.Time `gorm:"autoUpdateTime"`
}

func (RotatingProxy) TableName() string {
	return "rotating_proxies"
}

func (rp *RotatingProxy) BeforeSave(_ *gorm.DB) error {
	if rp.ListenProtocolID == 0 {
		rp.ListenProtocolID = rp.ProtocolID
	}

	rp.TransportProtocol = strings.ToLower(strings.TrimSpace(rp.TransportProtocol))
	if rp.TransportProtocol == "" {
		rp.TransportProtocol = "tcp"
	}

	rp.ListenTransportProtocol = strings.ToLower(strings.TrimSpace(rp.ListenTransportProtocol))
	if rp.ListenTransportProtocol == "" {
		rp.ListenTransportProtocol = rp.TransportProtocol
	}

	if rp.AuthRequired && rp.AuthPassword != "" {
		encrypted, err := security.EncryptProxySecret(rp.AuthPassword)
		if err != nil {
			return err
		}
		rp.AuthPasswordEncrypted = encrypted
	} else {
		rp.AuthPasswordEncrypted = ""
	}
	return nil
}

func (rp *RotatingProxy) AfterFind(_ *gorm.DB) error {
	if rp.AuthPasswordEncrypted == "" {
		rp.AuthPassword = ""
		return nil
	}

	password, _, err := security.DecryptProxySecret(rp.AuthPasswordEncrypted)
	if err != nil {
		return err
	}
	rp.AuthPassword = password
	return nil
}
