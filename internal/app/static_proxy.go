package app

import "strings"

const staticCommonProxyMode = "COMMON_PROXY"

func staticProxyRoute(name string, proxyURL string, mode string) WAProxyRoute {
	return WAProxyRoute{
		AccountID:   "static-" + name + "-proxy",
		RouteID:     "static-" + name + "-proxy",
		ProxyURL:    strings.TrimSpace(proxyURL),
		ProxyMode:   mode,
		CountryCode: "UNKNOWN",
	}
}
