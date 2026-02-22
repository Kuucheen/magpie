package dto

import "time"

type RotatingProxy struct {
	ID                      uint64     `json:"id"`
	Name                    string     `json:"name"`
	InstanceID              string     `json:"instance_id,omitempty"`
	InstanceName            string     `json:"instance_name,omitempty"`
	InstanceRegion          string     `json:"instance_region,omitempty"`
	Protocol                string     `json:"protocol"`
	ListenProtocol          string     `json:"listen_protocol,omitempty"`
	TransportProtocol       string     `json:"transport_protocol,omitempty"`
	ListenTransportProtocol string     `json:"listen_transport_protocol,omitempty"`
	UptimeFilterType        string     `json:"uptime_filter_type,omitempty"`
	UptimePercentage        *float64   `json:"uptime_percentage,omitempty"`
	AliveProxyCount         int        `json:"alive_proxy_count"`
	ListenPort              uint16     `json:"listen_port"`
	AuthRequired            bool       `json:"auth_required"`
	AuthUsername            string     `json:"auth_username,omitempty"`
	AuthPassword            string     `json:"auth_password,omitempty"`
	ListenHost              string     `json:"listen_host,omitempty"`
	ListenAddress           string     `json:"listen_address,omitempty"`
	LastRotationAt          *time.Time `json:"last_rotation_at,omitempty"`
	LastServedProxy         string     `json:"last_served_proxy,omitempty"`
	ReputationLabels        []string   `json:"reputation_labels,omitempty"`
	CreatedAt               time.Time  `json:"created_at"`
}

type RotatingProxyCreateRequest struct {
	Name                    string   `json:"name"`
	InstanceID              string   `json:"instance_id,omitempty"`
	InstanceName            string   `json:"instance_name,omitempty"`
	InstanceRegion          string   `json:"instance_region,omitempty"`
	Protocol                string   `json:"protocol"`
	ListenProtocol          string   `json:"listen_protocol,omitempty"`
	TransportProtocol       string   `json:"transport_protocol,omitempty"`
	ListenTransportProtocol string   `json:"listen_transport_protocol,omitempty"`
	UptimeFilterType        string   `json:"uptime_filter_type,omitempty"`
	UptimePercentage        *float64 `json:"uptime_percentage,omitempty"`
	AuthRequired            bool     `json:"auth_required"`
	AuthUsername            string   `json:"auth_username,omitempty"`
	AuthPassword            string   `json:"auth_password,omitempty"`
	ReputationLabels        []string `json:"reputation_labels"`
}

type RotatingProxyNext struct {
	ProxyID  uint64 `json:"proxy_id"`
	IP       string `json:"ip"`
	Port     uint16 `json:"port"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	HasAuth  bool   `json:"has_auth"`
	Protocol string `json:"protocol"`
}
