package cli

import "runtime/debug"

var version = buildVersion()

func buildVersion() string {
	return resolveBuildVersion(debug.ReadBuildInfo())
}

func resolveBuildVersion(info *debug.BuildInfo, ok bool) string {
	if !ok || info == nil || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "dev"
	}
	return info.Main.Version
}
