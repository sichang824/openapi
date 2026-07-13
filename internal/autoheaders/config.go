package autoheaders

import (
	"fmt"
	"strings"
)

const AutoHeadersEnv = "OAPI_AUTO_HEADERS"

func ResolveEnabled(flagValue, flagSet bool, lookupEnv func(string) (string, bool)) (bool, error) {
	if flagSet {
		return flagValue, nil
	}

	raw, ok := lookupEnv(AutoHeadersEnv)
	if !ok {
		return false, nil
	}

	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "0", "false", "off":
		return false, nil
	case "1", "true", "on":
		return true, nil
	default:
		return false, fmt.Errorf("invalid %s value %q: expected 1/true/on or 0/false/off", AutoHeadersEnv, raw)
	}
}
