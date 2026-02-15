package database

import "gorm.io/gorm"

var checkerUserSelectColumns = []string{
	"id",
	"http_protocol",
	"http_s_protocol",
	"socks4_protocol",
	"socks5_protocol",
	"timeout",
	"retries",
	"use_https_for_socks",
	"transport_protocol",
	"auto_remove_failing_proxies",
	"auto_remove_failure_threshold",
}

func preloadCheckerUsers(db *gorm.DB) *gorm.DB {
	return db.Select(checkerUserSelectColumns)
}

func preloadUserIDsOnly(db *gorm.DB) *gorm.DB {
	return db.Select("id")
}
