package version

// Version is the version string, set at build time via ldflags.
// Example: go build -ldflags "-X github.com/redhatinsights/rhc/pkg/version.Version=1.2.3"
var Version = "dev"
