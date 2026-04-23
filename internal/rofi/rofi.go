// Package rofi implements the one-shot keyboard-driven picker.
package rofi

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/NoneNameDeveloper/wireguard-ui/internal/config"
	"github.com/NoneNameDeveloper/wireguard-ui/internal/logger"
	"github.com/NoneNameDeveloper/wireguard-ui/internal/notify"
	"github.com/NoneNameDeveloper/wireguard-ui/internal/wg"
)

func Run(ctx context.Context, cfg config.Config, mgr *wg.Manager, n *notify.Notifier) error {
	snap := mgr.Refresh()

	var lines []string
	for _, t := range snap.Tunnels {
		lines = append(lines, formatLine(t))
	}
	if len(lines) == 0 {
		lines = append(lines, "(no tunnels found in "+cfg.ConfigDir+")")
	}

	choice, err := pick(ctx, cfg, strings.Join(lines, "\n"))
	if err != nil {
		return err
	}
	name := parseChoice(strings.TrimSpace(choice))
	if name == "" {
		return nil
	}

	logger.L().Info("rofi toggle", "tunnel", name)
	cctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	if res := mgr.Toggle(cctx, name); res.Err != nil {
		n.Error("wgtray: "+name+" failed", res.Err.Error())
		return res.Err
	}
	for _, t := range mgr.Last().Tunnels {
		if t.Name == name {
			if t.Active {
				n.Info("wgtray: "+name, "up")
			} else {
				n.Info("wgtray: "+name, "down")
			}
			break
		}
	}
	return nil
}

func formatLine(t wg.Tunnel) string {
	if !t.Active {
		return "○ " + t.Name
	}
	if !t.HasStats {
		return "● " + t.Name
	}
	hs := "never"
	if !t.LastHandshk.IsZero() {
		hs = time.Since(t.LastHandshk).Truncate(time.Second).String()
	}
	return fmt.Sprintf("● %s  %d peers  %s  hs %s", t.Name, t.Peers, t.Transfer, hs)
}

func parseChoice(s string) string {
	switch {
	case strings.HasPrefix(s, "["):
		if i := strings.Index(s, "]"); i >= 0 {
			s = strings.TrimSpace(s[i+1:])
		}
	case strings.HasPrefix(s, "●"):
		s = strings.TrimSpace(s[len("●"):])
	case strings.HasPrefix(s, "○"):
		s = strings.TrimSpace(s[len("○"):])
	}
	if i := strings.IndexAny(s, " \t"); i > 0 {
		s = s[:i]
	}
	return s
}

func pick(ctx context.Context, cfg config.Config, stdinText string) (string, error) {
	bin := cfg.RofiBin
	if bin == "" {
		bin = "rofi"
	}
	if _, err := exec.LookPath(bin); err != nil {
		if _, err2 := exec.LookPath("dmenu"); err2 == nil {
			bin = "dmenu"
		} else {
			return "", fmt.Errorf("neither %s nor dmenu found in PATH", cfg.RofiBin)
		}
	}

	args := cfg.RofiArgs
	switch {
	case bin == "dmenu" && (len(args) == 0 || args[0] != "-i"):
		args = []string{"-i", "-p", "wg"}
	case bin != "dmenu" && len(args) == 0:
		args = []string{"-dmenu", "-i", "-p", "wg"}
	}

	cctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(cctx, bin, args...)
	cmd.Stdin = strings.NewReader(stdinText)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		// rofi exits 1 on ESC; treat as cancellation.
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return "", nil
		}
		return "", fmt.Errorf("%s: %w", bin, err)
	}
	return out.String(), nil
}
