// Package app hosts small glue helpers (status/config printing, watch, info).
package app

import (
	"fmt"
	"io"
	"time"

	"github.com/NoneNameDeveloper/wireguard-ui/internal/config"
	"github.com/NoneNameDeveloper/wireguard-ui/internal/wg"
)

func PrintStatus(w io.Writer, s wg.Snapshot) {
	if s.Err != nil {
		fmt.Fprintln(w, "error:", s.Err)
	}
	if len(s.Tunnels) == 0 {
		fmt.Fprintln(w, "(no tunnels)")
		return
	}
	fmt.Fprintf(w, "%-16s  %-8s  %-6s  %-20s  %s\n", "TUNNEL", "STATE", "PEERS", "TRANSFER", "LAST HANDSHAKE")
	for _, t := range s.Tunnels {
		state := "down"
		if t.Active {
			state = "up"
		}
		peers, transfer, hs := "-", "-", "-"
		if t.HasStats {
			peers = fmt.Sprintf("%d", t.Peers)
			transfer = t.Transfer
			if !t.LastHandshk.IsZero() {
				hs = time.Since(t.LastHandshk).Truncate(time.Second).String() + " ago"
			}
		}
		fmt.Fprintf(w, "%-16s  %-8s  %-6s  %-20s  %s\n",
			t.Name, state, peers, transfer, hs)
	}
}

func PrintConfig(w io.Writer, c config.Config) {
	fmt.Fprintf(w, "source:          %s\n", c.SourcePath)
	fmt.Fprintf(w, "config_dir:      %s\n", c.ConfigDir)
	fmt.Fprintf(w, "poll_interval:   %s\n", c.PollInterval)
	fmt.Fprintf(w, "privilege_tool:  %s\n", c.PrivilegeTool)
	fmt.Fprintf(w, "wg_quick_bin:    %s\n", c.WgQuickBin)
	fmt.Fprintf(w, "notifications:   %v\n", c.Notifications)
	fmt.Fprintf(w, "log_file:        %s\n", c.LogFile)
	fmt.Fprintf(w, "log_level:       %s\n", c.LogLevel)
	fmt.Fprintf(w, "exclusive_mode:  %v\n", c.ExclusiveMode)
	fmt.Fprintf(w, "confirm_toggle:  %v\n", c.ConfirmToggle)
	fmt.Fprintf(w, "rofi_bin:        %s\n", c.RofiBin)
	fmt.Fprintf(w, "rofi_args:       %v\n", c.RofiArgs)
}
