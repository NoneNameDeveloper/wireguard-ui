// Package tray renders the StatusNotifierItem UI.
//
// Menu items are created once in onReady and then hidden/relabelled, because
// fyne.io/systray does not support removing items after Run().
package tray

import (
	"context"
	"fmt"
	"sort"
	"time"

	"fyne.io/systray"

	"github.com/NoneNameDeveloper/wireguard-ui/internal/config"
	"github.com/NoneNameDeveloper/wireguard-ui/internal/logger"
	"github.com/NoneNameDeveloper/wireguard-ui/internal/notify"
	"github.com/NoneNameDeveloper/wireguard-ui/internal/wg"
)

const maxTunnels = 64

type Tray struct {
	cfg     config.Config
	mgr     *wg.Manager
	notif   *notify.Notifier
	ctx     context.Context
	cancel  context.CancelFunc
	updates chan wg.Snapshot

	statusItem *systray.MenuItem
	items      []*tunnelItem
	refreshIt  *systray.MenuItem
	configIt   *systray.MenuItem
	aboutIt    *systray.MenuItem
	quitIt     *systray.MenuItem
}

type tunnelItem struct {
	item *systray.MenuItem
	name string
}

func New(cfg config.Config, mgr *wg.Manager, n *notify.Notifier) *Tray {
	return &Tray{cfg: cfg, mgr: mgr, notif: n, updates: make(chan wg.Snapshot, 8)}
}

func (t *Tray) Run(ctx context.Context) {
	t.ctx, t.cancel = context.WithCancel(ctx)
	systray.Run(t.onReady, t.onExit)
}

func (t *Tray) onExit() {
	logger.L().Info("tray exit")
	if t.cancel != nil {
		t.cancel()
	}
}

func (t *Tray) onReady() {
	defer recoverLog("onReady")

	systray.SetIcon(iconInactive)
	systray.SetTitle("")
	systray.SetTooltip("wgtray: WireGuard (loading…)")

	t.statusItem = systray.AddMenuItem("No tunnels", "Active tunnel status")
	t.statusItem.Disable()
	systray.AddSeparator()

	t.items = make([]*tunnelItem, maxTunnels)
	for i := 0; i < maxTunnels; i++ {
		it := systray.AddMenuItem("", "")
		it.Hide()
		ti := &tunnelItem{item: it}
		t.items[i] = ti
		go t.watchClicks(ti)
	}

	systray.AddSeparator()
	t.refreshIt = systray.AddMenuItem("Refresh", "Re-read tunnel state now")
	t.configIt = systray.AddMenuItem("Open config…", "Open wgtray config in $EDITOR")
	t.aboutIt = systray.AddMenuItem("About", "About wgtray")
	systray.AddSeparator()
	t.quitIt = systray.AddMenuItem("Quit", "Exit wgtray")

	go t.watchStatic()
	go t.pumpUpdates()

	sub := t.mgr.Subscribe()
	go func() {
		defer recoverLog("sub-forwarder")
		for {
			select {
			case <-t.ctx.Done():
				return
			case s, ok := <-sub:
				if !ok {
					return
				}
				select {
				case t.updates <- s:
				default:
				}
			}
		}
	}()

	t.updates <- t.mgr.Last()
}

func (t *Tray) pumpUpdates() {
	defer recoverLog("pumpUpdates")
	for {
		select {
		case <-t.ctx.Done():
			return
		case s := <-t.updates:
			t.render(s)
		}
	}
}

func (t *Tray) render(s wg.Snapshot) {
	defer recoverLog("render")

	sort.Slice(s.Tunnels, func(i, j int) bool { return s.Tunnels[i].Name < s.Tunnels[j].Name })

	activeCount := 0
	var activeName string
	for _, tun := range s.Tunnels {
		if tun.Active {
			activeCount++
			activeName = tun.Name
		}
	}

	switch {
	case s.Err != nil:
		systray.SetIcon(iconError)
		systray.SetTooltip("wgtray: " + s.Err.Error())
		t.statusItem.SetTitle("error: " + short(s.Err.Error(), 48))
	case activeCount == 0:
		systray.SetIcon(iconInactive)
		systray.SetTooltip("wgtray: no active tunnel")
		t.statusItem.SetTitle("inactive")
	case activeCount == 1:
		systray.SetIcon(iconActive)
		systray.SetTooltip("wgtray: " + activeName)
		t.statusItem.SetTitle("active: " + activeName)
	default:
		systray.SetIcon(iconActive)
		systray.SetTooltip(fmt.Sprintf("wgtray: %d active tunnels", activeCount))
		t.statusItem.SetTitle(fmt.Sprintf("%d active", activeCount))
	}

	for i, ti := range t.items {
		if i < len(s.Tunnels) {
			tun := s.Tunnels[i]
			ti.name = tun.Name
			ti.item.SetTitle(label(tun))
			ti.item.SetTooltip(tooltip(tun))
			if tun.Active {
				ti.item.Check()
			} else {
				ti.item.Uncheck()
			}
			ti.item.Show()
		} else {
			ti.name = ""
			ti.item.Hide()
			ti.item.Uncheck()
		}
	}
}

func (t *Tray) watchStatic() {
	defer recoverLog("watchStatic")
	for {
		select {
		case <-t.ctx.Done():
			return
		case <-t.refreshIt.ClickedCh:
			t.mgr.Refresh()
		case <-t.configIt.ClickedCh:
			go openConfig(t.cfg, t.notif)
		case <-t.aboutIt.ClickedCh:
			go showAbout(t.cfg, t.mgr, t.notif)
		case <-t.quitIt.ClickedCh:
			logger.L().Info("quit requested")
			systray.Quit()
			return
		}
	}
}

func (t *Tray) watchClicks(ti *tunnelItem) {
	defer recoverLog("watchClicks")
	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ti.item.ClickedCh:
			if ti.name == "" {
				continue
			}
			go t.doToggle(ti.name)
		}
	}
}

func (t *Tray) doToggle(name string) {
	defer recoverLog("doToggle")

	if t.cfg.ConfirmToggle {
		t.notif.Info("wgtray", "Toggling "+name+"…")
	}

	ctx, cancel := context.WithTimeout(t.ctx, 45*time.Second)
	defer cancel()

	if res := t.mgr.Toggle(ctx, name); res.Err != nil {
		t.notif.Error("wgtray: "+name+" failed", res.Err.Error())
		return
	}
	for _, tun := range t.mgr.Last().Tunnels {
		if tun.Name == name {
			if tun.Active {
				t.notif.Info("wgtray: "+name, "up")
			} else {
				t.notif.Info("wgtray: "+name, "down")
			}
			break
		}
	}
}

func label(t wg.Tunnel) string {
	mark := "○"
	if t.Active {
		mark = "●"
	}
	if t.Active && t.Peers > 0 {
		return fmt.Sprintf("%s  %s  (%d peers)", mark, t.Name, t.Peers)
	}
	return fmt.Sprintf("%s  %s", mark, t.Name)
}

func tooltip(t wg.Tunnel) string {
	if !t.Active {
		return t.Path
	}
	s := fmt.Sprintf("port %d", t.ListenPort)
	if t.Transfer != "" {
		s += "  " + t.Transfer
	}
	if !t.LastHandshk.IsZero() {
		s += "  hs " + time.Since(t.LastHandshk).Truncate(time.Second).String() + " ago"
	}
	return s
}

func short(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func recoverLog(where string) {
	if r := recover(); r != nil {
		logger.L().Error("panic recovered", "where", where, "panic", fmt.Sprint(r))
	}
}
