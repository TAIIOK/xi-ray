package version

import "fmt"

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func String() string {
	return fmt.Sprintf("xiaomi-vless %s (%s, %s)", Version, Commit, BuildDate)
}
