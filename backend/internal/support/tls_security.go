package support

const (
	allowInsecureUpstreamTLSEnv = "MAGPIE_ALLOW_INSECURE_UPSTREAM_TLS"
)

func AllowInsecureUpstreamTLS() bool {
	return GetEnvBool(allowInsecureUpstreamTLSEnv, false)
}
