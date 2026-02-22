package domain

import (
	"os"
	"strings"
	"time"

	"magpie/internal/security"

	"gorm.io/gorm"
)

type RotatingProxy struct {
	ID                      uint64     `gorm:"primaryKey;autoIncrement"`
	UserID                  uint       `gorm:"not null;index:idx_rotating_user_name,priority:1"`
	Name                    string     `gorm:"not null;size:120;index:idx_rotating_user_name,priority:2"`
	InstanceID              string     `gorm:"size:191;index:idx_rotating_instance_port,priority:1"`
	InstanceName            string     `gorm:"size:120;default:''"`
	InstanceRegion          string     `gorm:"size:120;default:''"`
	ProtocolID              int        `gorm:"not null;index"`
	Protocol                Protocol   `gorm:"foreignKey:ProtocolID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	ListenProtocolID        int        `gorm:"index"`
	ListenProtocol          Protocol   `gorm:"foreignKey:ListenProtocolID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	TransportProtocol       string     `gorm:"not null;default:'tcp'"`
	ListenTransportProtocol string     `gorm:"not null;default:'tcp'"`
	UptimeFilterType        string     `gorm:"size:8;default:''"`
	UptimePercentage        *float64   `gorm:"type:numeric(5,2)"`
	ListenPort              uint16     `gorm:"uniqueIndex:idx_rotating_instance_port,priority:2"`
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
	if strings.TrimSpace(rp.InstanceID) == "" {
		rp.InstanceID = defaultInstanceID()
	}
	if strings.TrimSpace(rp.InstanceName) == "" {
		rp.InstanceName = rp.InstanceID
	}
	if strings.TrimSpace(rp.InstanceRegion) == "" {
		rp.InstanceRegion = "Unknown"
	}

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

func defaultInstanceID() string {
	if value := strings.TrimSpace(os.Getenv("MAGPIE_INSTANCE_ID")); value != "" {
		return value
	}
	hostname, err := os.Hostname()
	if err == nil && strings.TrimSpace(hostname) != "" {
		return strings.TrimSpace(hostname)
	}
	return "default"
}
