package paths

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

var ErrUnavailable = errors.New("desktop_paths_unavailable")

const (
	productDir = "Gist"
	dataDir    = "data"
	database   = "gist.db"
	config     = "desktop.json"
	recovery   = "recovery"
	logs       = "logs"
	updates    = "updates"
	webview    = "webview"
)

type Paths struct {
	Root        string
	DataDir     string
	DBPath      string
	ConfigPath  string
	RecoveryDir string
	LogsDir     string
	UpdatesDir  string
	WebViewDir  string
}

type LocalAppDataResolver interface { ResolveLocalAppData() (string, error) }

func Resolve(resolver LocalAppDataResolver) (Paths, error) {
	if resolver == nil { return Paths{}, ErrUnavailable }
	base, err := resolver.ResolveLocalAppData()
	if err != nil {
		return Paths{}, fmt.Errorf("%w: resolve known folder: %v", ErrUnavailable, err)
	}
	base = strings.TrimSpace(base)
	if strings.IndexByte(base, 0) >= 0 {
		return Paths{}, ErrUnavailable
	}
	base = filepath.Clean(base)
	if base == "." || base == "" || !filepath.IsAbs(base) || filepath.VolumeName(base)+string(filepath.Separator) == base {
		return Paths{}, ErrUnavailable
	}
	base, err = filepath.Abs(base)
	if err != nil {
		return Paths{}, fmt.Errorf("%w: canonicalize known folder", ErrUnavailable)
	}
	root := filepath.Clean(filepath.Join(base, productDir))
	data := filepath.Join(root, dataDir)
	return Paths{Root: root, DataDir: data, DBPath: filepath.Join(data, database), ConfigPath: filepath.Join(root, config), RecoveryDir: filepath.Join(root, recovery), LogsDir: filepath.Join(root, logs), UpdatesDir: filepath.Join(root, updates), WebViewDir: filepath.Join(root, webview)}, nil
}
