package workspace

import "path/filepath"

const (
	SourceEntryName    = "index.yaml"
	BundleArtifactName = "openapi.yaml"
)

func SourceEntryPath(dir string) string {
	return filepath.Join(filepath.Clean(dir), SourceEntryName)
}

func BundleArtifactPath(dir string) string {
	return filepath.Join(filepath.Clean(dir), BundleArtifactName)
}
