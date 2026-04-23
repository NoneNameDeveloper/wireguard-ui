// Package notify wraps notify-send.
package notify

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/NoneNameDeveloper/wireguard-ui/internal/logger"
)

type Notifier struct {
	Enabled bool
	Bin     string
	App     string
}

func New(enabled bool) *Notifier {
	return &Notifier{Enabled: enabled, Bin: "notify-send", App: "wgtray"}
}

func (n *Notifier) Send(urgency, title, body string) {
	n.SendWithTimeout(urgency, title, body, -1)
}

func (n *Notifier) SendWithTimeout(urgency, title, body string, timeoutMs int) {
	if n == nil || !n.Enabled {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	args := []string{"-a", n.App, "-u", urgency, "-i", "network-vpn"}
	if timeoutMs >= 0 {
		args = append(args, "-t", fmt.Sprint(timeoutMs))
	}
	args = append(args, title)
	if body != "" {
		args = append(args, body)
	}
	if err := exec.CommandContext(ctx, n.Bin, args...).Run(); err != nil {
		logger.L().Debug("notify-send failed", "err", err)
	}
}

func (n *Notifier) Info(title, body string)  { n.Send("normal", title, body) }
func (n *Notifier) Warn(title, body string)  { n.Send("normal", title, body) }
func (n *Notifier) Error(title, body string) { n.Send("critical", title, body) }
