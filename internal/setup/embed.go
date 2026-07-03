package setup

import (
	"embed"
	"io/fs"
)

//go:embed scripts/*.sh
var scriptFS embed.FS

func Script(name string) ([]byte, error) {
	return fs.ReadFile(scriptFS, "scripts/"+name)
}
