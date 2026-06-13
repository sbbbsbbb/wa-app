package app

import (
	"context"
	"strings"

	waappv1 "github.com/byte-v-forge/wa-app/gen/go/byte/v/forge/waapp/v1"
)

type waProxyStage string

const (
	waProxyStageProbe        waProxyStage = "probe"
	waProxyStageRegistration waProxyStage = "registration"

	waProxySourceRequestOverride = "REQUEST_OVERRIDE"
	waProxySourceRequestPolicy   = "REQUEST_POLICY"
	waProxySourceAccountStage    = "ACCOUNT_STAGE"
	waProxySourceAccountDefault  = "ACCOUNT_DEFAULT"
	waProxySourceSystemStage     = "SYSTEM_STAGE"
	waProxySourceSystemCommon    = "SYSTEM_COMMON"
	waProxySourceDirect          = "DIRECT"

	waProxyModeRequestOverride = "REQUEST_PROXY"
	waProxyModeDirect          = "DIRECT"
	waProxyModeCommon          = "COMMON_PROXY"
)

type waProxyResolveRequest struct {
	Stage       waProxyStage
	Payload     map[string]any
	WAAccountID string
	CountryCode string
}

func (s *Server) resolveWAProxyRoute(ctx context.Context, req waProxyResolveRequest) (WAProxyRoute, bool, error) {
	countryCode := normalizeProxyCountryCode(firstNonEmpty(req.CountryCode, proxyCountryCodeFromPayload(req.Payload)))
	if proxyURL := actionProxyURL(req.Payload); proxyURL != "" {
		route := staticProxyRoute("request-override", proxyURL, waProxyModeRequestOverride)
		route.CountryCode = countryCode
		route.Source = waProxySourceRequestOverride
		route.PolicyMode = waProxyModeRequestOverride
		return route, true, nil
	}
	if policy, err := waAccountProxyPolicyFromPayload(req.Payload); err != nil {
		return WAProxyRoute{}, false, err
	} else if policy != nil {
		if stagePolicy, _ := waProxyStagePolicy(policy, req.Stage); stagePolicy != nil {
			return s.resolveWAProxyStagePolicy(stagePolicy, waProxySourceRequestPolicy, countryCode)
		}
	}
	accountID := firstNonEmpty(req.WAAccountID, textField(req.Payload, "wa_account_id"))
	if accountID != "" {
		route, useProxy, handled, err := s.resolveWAAccountProxyRoute(ctx, accountID, req.Stage, countryCode)
		if handled || err != nil {
			return route, useProxy, err
		}
	}
	if route, ok := s.resolveSystemStageProxyRoute(req.Stage, countryCode); ok {
		return route, true, nil
	}
	if route, ok := s.resolveSystemCommonProxyRoute(countryCode); ok {
		return route, true, nil
	}
	return WAProxyRoute{ProxyMode: waProxyModeDirect, CountryCode: "LOCAL", Source: waProxySourceDirect, PolicyMode: waProxyModeDirect}, false, nil
}

func (s *Server) resolveWAAccountProxyRoute(ctx context.Context, accountID string, stage waProxyStage, countryCode string) (WAProxyRoute, bool, bool, error) {
	normalizedID, err := requireWAAccountID(accountID)
	if err != nil {
		return WAProxyRoute{}, false, true, err
	}
	account, err := s.getWAAccount(ctx, normalizedID)
	if err != nil {
		return WAProxyRoute{}, false, true, err
	}
	policy, source := waProxyStagePolicy(account.GetProxyPolicy(), stage)
	if policy == nil {
		return WAProxyRoute{}, false, false, nil
	}
	route, useProxy, err := s.resolveWAProxyStagePolicy(policy, source, countryCode)
	return route, useProxy, true, err
}

func waProxyStagePolicy(policy *waappv1.WAAccountProxyPolicy, stage waProxyStage) (*waappv1.WAProxyStagePolicy, string) {
	if policy == nil {
		return nil, ""
	}
	stagePolicy := policy.GetRegistrationPolicy()
	if stage == waProxyStageProbe {
		stagePolicy = policy.GetProbePolicy()
	}
	if !emptyWAProxyStagePolicy(stagePolicy) {
		return stagePolicy, waProxySourceAccountStage
	}
	if !emptyWAProxyStagePolicy(policy.GetDefaultPolicy()) {
		return policy.GetDefaultPolicy(), waProxySourceAccountDefault
	}
	return nil, ""
}

func (s *Server) resolveWAProxyStagePolicy(policy *waappv1.WAProxyStagePolicy, source string, countryCode string) (WAProxyRoute, bool, error) {
	mode := normalizeWAProxyPolicyMode(policy.GetMode())
	switch mode {
	case waappv1.WAProxyPolicyMode_WA_PROXY_POLICY_MODE_INHERIT:
		return WAProxyRoute{}, false, nil
	case waappv1.WAProxyPolicyMode_WA_PROXY_POLICY_MODE_DIRECT:
		return WAProxyRoute{ProxyMode: waProxyModeDirect, CountryCode: "LOCAL", Source: source, PolicyMode: mode.String()}, false, nil
	case waappv1.WAProxyPolicyMode_WA_PROXY_POLICY_MODE_COMMON_PROXY:
		route, ok := s.resolveSystemCommonProxyRoute(countryCode)
		if !ok {
			return WAProxyRoute{}, false, NewError(waappv1.WaErrorCode_WA_ERROR_CODE_ROUTE_UNAVAILABLE, "WA common proxy is not configured", true)
		}
		route.Source = source
		route.PolicyMode = mode.String()
		return route, true, nil
	default:
		return WAProxyRoute{}, false, NewError(waappv1.WaErrorCode_WA_ERROR_CODE_VALIDATION_FAILED, "WA proxy policy mode is unsupported", false)
	}
}

func (s *Server) resolveSystemStageProxyRoute(stage waProxyStage, countryCode string) (WAProxyRoute, bool) {
	if s == nil {
		return WAProxyRoute{}, false
	}
	if proxyURL := s.systemStageProxyURL(stage); proxyURL != "" {
		name := string(stage)
		mode := staticNumberProbeProxyMode
		if stage == waProxyStageRegistration {
			name = "registration"
			mode = staticRegistrationProxyMode
		}
		route := staticProxyRoute(name, proxyURL, mode)
		route.CountryCode = countryCode
		route.Source = waProxySourceSystemStage
		route.PolicyMode = mode
		return route, true
	}
	return WAProxyRoute{}, false
}

func (s *Server) resolveSystemCommonProxyRoute(countryCode string) (WAProxyRoute, bool) {
	if s == nil || strings.TrimSpace(s.commonProxyURL) == "" {
		return WAProxyRoute{}, false
	}
	route := staticProxyRoute("common", s.commonProxyURL, staticCommonProxyMode)
	route.CountryCode = countryCode
	route.Source = waProxySourceSystemCommon
	route.PolicyMode = waProxyModeCommon
	return route, true
}

func (s *Server) systemStageProxyURL(stage waProxyStage) string {
	if s == nil {
		return ""
	}
	switch stage {
	case waProxyStageProbe:
		return strings.TrimSpace(s.numberProbeProxyURL)
	case waProxyStageRegistration:
		return strings.TrimSpace(s.registrationProxyURL)
	default:
		return ""
	}
}

func waProxySummary(route WAProxyRoute, useProxy bool) map[string]any {
	if !useProxy {
		return map[string]any{"success": true, "accepted": true, "proxy_mode": waProxyModeDirect, "country_code": "LOCAL", "source": waProxySourceDirect}
	}
	result := map[string]any{
		"success":      true,
		"accepted":     true,
		"proxy_mode":   firstNonEmpty(route.ProxyMode, "PROXY"),
		"country_code": firstNonEmpty(route.CountryCode, "UNKNOWN"),
	}
	if route.Source != "" {
		result["source"] = route.Source
	}
	if route.PolicyMode != "" {
		result["policy_mode"] = route.PolicyMode
	}
	if route.AccountID != "" {
		result["account_id"] = route.AccountID
	}
	if route.RouteID != "" {
		result["route_id"] = route.RouteID
	}
	return result
}
