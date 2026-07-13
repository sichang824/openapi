package autoheaders

import (
	"fmt"
	"net/url"
	"strings"
)

func ValidateOrigin(specServer, targetBaseURL string) (string, error) {
	target, err := url.Parse(targetBaseURL)
	if err != nil || !target.IsAbs() || target.Hostname() == "" {
		return "", fmt.Errorf("automatic headers require an absolute, parseable target base URL")
	}

	contract, err := url.Parse(specServer)
	if strings.TrimSpace(specServer) == "" || err != nil || !contract.IsAbs() || contract.Hostname() == "" || strings.Contains(specServer, "{") {
		return "cannot verify automatic-header target origin against the OpenAPI server URL", nil
	}
	if !sameURLOrigin(contract, target) {
		return "", fmt.Errorf(
			"automatic headers blocked because --base-url changes the OpenAPI server origin from %s to %s",
			urlOrigin(contract),
			urlOrigin(target),
		)
	}
	return "", nil
}

func sameURLOrigin(left, right *url.URL) bool {
	return strings.EqualFold(left.Scheme, right.Scheme) &&
		strings.EqualFold(left.Hostname(), right.Hostname()) &&
		urlPort(left) == urlPort(right)
}

func urlPort(value *url.URL) string {
	if port := value.Port(); port != "" {
		return port
	}
	switch strings.ToLower(value.Scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

func urlOrigin(value *url.URL) string {
	return value.Scheme + "://" + value.Host
}
