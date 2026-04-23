package app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/NoneNameDeveloper/wireguard-ui/internal/wg"
)

// Watch emits one status line per state change, suitable for polybar tail=true.
// format: "plain" (machine-readable) or "polybar" (coloured format tags).
func Watch(ctx context.Context, mgr *wg.Manager, w io.Writer, format string) {
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	sub := mgr.Subscribe()

	var last string
	emit := func(s wg.Snapshot) {
		line := renderLine(s, format)
		if line == last {
			return
		}
		last = line
		_, _ = bw.WriteString(line)
		_ = bw.WriteByte('\n')
		_ = bw.Flush()
	}

	emit(mgr.Last())

	for {
		select {
		case <-ctx.Done():
			return
		case s, ok := <-sub:
			if !ok {
				return
			}
			emit(s)
		}
	}
}

func renderLine(s wg.Snapshot, format string) string {
	if format == "polybar" {
		return renderPolybar(s)
	}
	return renderPlain(s)
}

func renderPlain(s wg.Snapshot) string {
	if s.Err != nil {
		return "err " + compact(s.Err.Error())
	}
	active, act := countActive(s)
	switch active {
	case 0:
		return "down"
	case 1:
		return fmt.Sprintf("up %s %d %s", act.Name, act.Peers, orDash(strings.TrimSpace(act.Transfer)))
	default:
		return fmt.Sprintf("multi %d", active)
	}
}

const (
	colorUp   = "#2ecc71"
	colorDown = "#95a5a6"
	colorErr  = "#e74c3c"
)

func renderPolybar(s wg.Snapshot) string {
	if s.Err != nil {
		return wrap(colorErr, " WG err")
	}
	active, act := countActive(s)
	switch active {
	case 0:
		return wrap(colorDown, " off")
	case 1:
		return wrap(colorUp, " "+act.Name)
	default:
		return wrap(colorUp, fmt.Sprintf(" %d up", active))
	}
}

func countActive(s wg.Snapshot) (int, wg.Tunnel) {
	var act wg.Tunnel
	n := 0
	for _, t := range s.Tunnels {
		if t.Active {
			n++
			act = t
		}
	}
	return n, act
}

func wrap(color, text string) string { return "%{F" + color + "}" + text + "%{F-}" }

func compact(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 80 {
		s = s[:77] + "..."
	}
	return s
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
