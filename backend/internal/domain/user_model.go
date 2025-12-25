package domain

import (
	"magpie/internal/api/dto"
	"time"
)

type User struct {
	ID       uint   `gorm:"primaryKey;autoIncrement"`
	Email    string `gorm:"uniqueIndex;not null;size:255"`
	Password string `gorm:"not null;size:100;check:length(password) >= 8" json:"-"`
	Role     string `gorm:"not null;default:'user';check:role IN ('user', 'admin')"`

	//Settings
	HTTPProtocol               bool   `gorm:"not null;default:false"`
	HTTPSProtocol              bool   `gorm:"not null;default:true"`
	SOCKS4Protocol             bool   `gorm:"not null;default:false"`
	SOCKS5Protocol             bool   `gorm:"not null;default:false"`
	Timeout                    uint16 `gorm:"not null;default:7500"`
	Retries                    uint8  `gorm:"not null;default:2"`
	UseHttpsForSocks           bool   `gorm:"not null;default:true"`
	TransportProtocol          string `gorm:"not null;default:'tcp'"`
	AutoRemoveFailingProxies   bool   `gorm:"not null;default:false"`
	AutoRemoveFailureThreshold uint8  `gorm:"not null;default:3"`

	//Relations
	Judges       []Judge        `gorm:"many2many:user_judges;"`
	Proxies      []Proxy        `gorm:"many2many:user_proxies;"`
	ProxyHistory []ProxyHistory `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	ScrapeSites  []ScrapeSite   `gorm:"many2many:user_scrape_site;"`
	CreatedAt    time.Time      `gorm:"autoCreateTime"`
}

func (u *User) ToUserSettings(simpleUserJudges []dto.SimpleUserJudge, scrapingSources []string) dto.UserSettings {
	return dto.UserSettings{
		HTTPProtocol:               u.HTTPProtocol,
		HTTPSProtocol:              u.HTTPSProtocol,
		SOCKS4Protocol:             u.SOCKS4Protocol,
		SOCKS5Protocol:             u.SOCKS5Protocol,
		Timeout:                    u.Timeout,
		Retries:                    u.Retries,
		UseHttpsForSocks:           u.UseHttpsForSocks,
		TransportProtocol:          u.TransportProtocol,
		AutoRemoveFailingProxies:   u.AutoRemoveFailingProxies,
		AutoRemoveFailureThreshold: u.AutoRemoveFailureThreshold,
		SimpleUserJudges:           simpleUserJudges,
		ScrapingSources:            scrapingSources,
	}
}

func (u *User) GetProtocolMap() map[string]int {
	protocols := make(map[string]int)

	if u.HTTPProtocol {
		protocols["http"] = 1
	}
	if u.HTTPSProtocol {
		protocols["https"] = 2
	}
	if u.SOCKS4Protocol {
		protocols["socks4"] = 3
	}
	if u.SOCKS5Protocol {
		protocols["socks5"] = 4
	}

	return protocols
}
