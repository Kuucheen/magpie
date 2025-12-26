package support

import "strings"

const (
	TransportTCP   = "tcp"
	TransportQUIC  = "quic"
	TransportHTTP3 = "http3"
)

var transportProtocolSet = map[string]struct{}{
	TransportTCP:   {},
	TransportQUIC:  {},
	TransportHTTP3: {},
}

func NormalizeTransportProtocol(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if _, ok := transportProtocolSet[value]; ok {
		return value
	}
	return TransportTCP
}

func ResolveCheckerTransportProtocol(value string) string {
	return NormalizeTransportProtocol(value)
}

func IsHTTP3Transport(value string) bool {
	switch NormalizeTransportProtocol(value) {
	case TransportQUIC, TransportHTTP3:
		return true
	default:
		return false
	}
}
