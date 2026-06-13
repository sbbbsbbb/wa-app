package app

import "strings"

const (
	staticCommonProxyMode       = "COMMON_PROXY"
	staticNumberProbeProxyMode  = "STATIC_NUMBER_PROBE_PROXY"
	staticRegistrationProxyMode = "STATIC_REGISTRATION_PROXY"
)

func staticProxyRoute(name string, proxyURL string, mode string) WAProxyRoute {
	return WAProxyRoute{
		AccountID:   "static-" + name + "-proxy",
		RouteID:     "static-" + name + "-proxy",
		ProxyURL:    strings.TrimSpace(proxyURL),
		ProxyMode:   mode,
		CountryCode: "UNKNOWN",
	}
}

func isStaticProxyRoute(route WAProxyRoute) bool {
	return strings.HasPrefix(strings.TrimSpace(route.RouteID), "static-")
}
