package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/NoneNameDeveloper/wireguard-ui/internal/notify"
	"github.com/NoneNameDeveloper/wireguard-ui/internal/wg"
)

// ShowInfo sends a 10-second notification with current tunnel state.
// Summary carries the active tunnel so content survives narrow dunst windows.
func ShowInfo(s wg.Snapshot, n *notify.Notifier) {
	if s.Err != nil {
		n.SendWithTimeout("critical", "wgtray", s.Err.Error(), 10000)
		return
	}
	if len(s.Tunnels) == 0 {
		n.SendWithTimeout("normal", "wgtray", "no tunnels in "+s.ConfigDir(), 10000)
		return
	}

	var act *wg.Tunnel
	var inactive []wg.Tunnel
	for i := range s.Tunnels {
		t := s.Tunnels[i]
		if t.Active && act == nil {
			act = &s.Tunnels[i]
			continue
		}
		inactive = append(inactive, t)
	}

	var summary, body string
	switch {
	case act == nil:
		summary = "WireGuard ○ off"
	case act.HasStats:
		hs := "never"
		if !act.LastHandshk.IsZero() {
			hs = time.Since(act.LastHandshk).Truncate(time.Second).String() + " ago"
		}
		summary = fmt.Sprintf("WireGuard ● %s  %s", act.Name, act.Transfer)
		body = fmt.Sprintf("%d peers · handshake %s", act.Peers, hs)
	default:
		summary = "WireGuard ● " + act.Name
		body = "(stats need CAP_NET_ADMIN — see README)"
	}

	if len(inactive) > 0 {
		names := make([]string, len(inactive))
		for i, t := range inactive {
			names[i] = t.Name
		}
		line := "○ " + strings.Join(names, ", ")
		if body == "" {
			body = line
		} else {
			body += "\n" + line
		}
	}

	n.SendWithTimeout("normal", summary, body, 10000)
}
