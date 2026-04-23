package tray

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/NoneNameDeveloper/wireguard-ui/internal/config"
	"github.com/NoneNameDeveloper/wireguard-ui/internal/logger"
	"github.com/NoneNameDeveloper/wireguard-ui/internal/notify"
	"github.com/NoneNameDeveloper/wireguard-ui/internal/wg"
)

var Version = "dev"

func showAbout(cfg config.Config, mgr *wg.Manager, n *notify.Notifier) {
	defer recoverLog("showAbout")
	snap := mgr.Last()
	body := fmt.Sprintf(
		"version %s  (%s/%s)\nconfig: %s\nlog:    %s\ntunnels: %d   active: %d",
		Version, runtime.GOOS, runtime.GOARCH,
		cfg.SourcePath, cfg.LogFile,
		len(snap.Tunnels), countActive(snap),
	)
	n.Info("wgtray", body)
	logger.L().Info("about",
		"version", Version, "config", cfg.SourcePath,
		"tunnels", len(snap.Tunnels), "active", countActive(snap))
}

func countActive(s wg.Snapshot) int {
	n := 0
	for _, t := range s.Tunnels {
		if t.Active {
			n++
		}
	}
	return n
}

func openConfig(cfg config.Config, n *notify.Notifier) {
	defer recoverLog("openConfig")

	path := cfg.SourcePath
	if path == "" {
		path = config.DefaultPath()
	}
	if written, _ := config.WriteDefaultIfMissing(path); written != "" {
		logger.L().Info("wrote default config", "path", written)
	}

	candidates := []string{firstOf(os.Getenv("VISUAL"), os.Getenv("EDITOR")), "xdg-open"}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, c := range candidates {
		parts := strings.Fields(c)
		if len(parts) == 0 {
			continue
		}
		if _, err := exec.LookPath(parts[0]); err != nil {
			continue
		}
		cmd := exec.CommandContext(ctx, parts[0], append(parts[1:], path)...)
		if err := cmd.Start(); err == nil {
			go func() { _ = cmd.Wait() }()
			n.Info("wgtray", "opened "+path)
			return
		}
	}
	n.Error("wgtray", "no editor/xdg-open available for "+path)
}

func firstOf(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}
