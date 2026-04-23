// Package wg manages WireGuard tunnels via wgctrl (netlink) and wg-quick.
package wg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/NoneNameDeveloper/wireguard-ui/internal/config"
	"github.com/NoneNameDeveloper/wireguard-ui/internal/logger"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type Tunnel struct {
	Name        string
	Path        string
	Active      bool
	HasStats    bool // false when we only know the device name (no CAP_NET_ADMIN)
	Peers       int
	Transfer    string
	LastHandshk time.Time
	ListenPort  int
}

type Snapshot struct {
	At        time.Time
	Tunnels   []Tunnel
	configDir string
	Err       error
}

func (s Snapshot) ConfigDir() string { return s.configDir }

type Manager struct {
	cfg config.Config

	mu      sync.RWMutex
	last    Snapshot
	toggles map[string]*sync.Mutex
	togMu   sync.Mutex

	subs   map[chan Snapshot]struct{}
	subsMu sync.Mutex
}

func New(cfg config.Config) *Manager {
	return &Manager{
		cfg:     cfg,
		toggles: map[string]*sync.Mutex{},
		subs:    map[chan Snapshot]struct{}{},
	}
}

// Subscribe returns a buffered channel of snapshots. Slow consumers lose
// intermediate frames; they never block producers.
func (m *Manager) Subscribe() <-chan Snapshot {
	ch := make(chan Snapshot, 4)
	m.subsMu.Lock()
	m.subs[ch] = struct{}{}
	m.subsMu.Unlock()
	return ch
}

func (m *Manager) Run(ctx context.Context) {
	m.refresh()
	t := time.NewTicker(m.cfg.PollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.refresh()
		}
	}
}

func (m *Manager) Last() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.last
}

func (m *Manager) Refresh() Snapshot { return m.refresh() }

func (m *Manager) refresh() Snapshot {
	defer recoverLog("refresh")

	snap := Snapshot{At: time.Now(), configDir: m.cfg.ConfigDir}

	configs, err := m.listConfigs()
	if err != nil {
		snap.Err = err
		logger.L().Warn("list configs", "err", err)
	}

	active, hasStats := m.activeDevices()
	byName := map[string]*wgtypes.Device{}
	for _, d := range active {
		byName[d.Name] = d
	}

	seen := map[string]bool{}
	for _, c := range configs {
		t := Tunnel{Name: c.Name, Path: c.Path}
		if d, ok := byName[c.Name]; ok {
			t.Active = true
			if hasStats {
				t.HasStats = true
				t.Peers = len(d.Peers)
				t.ListenPort = d.ListenPort
				var rx, tx int64
				var last time.Time
				for _, p := range d.Peers {
					rx += p.ReceiveBytes
					tx += p.TransmitBytes
					if p.LastHandshakeTime.After(last) {
						last = p.LastHandshakeTime
					}
				}
				t.Transfer = fmt.Sprintf("↓%s ↑%s", humanBytes(rx), humanBytes(tx))
				t.LastHandshk = last
			}
		}
		snap.Tunnels = append(snap.Tunnels, t)
		seen[c.Name] = true
	}
	for name, d := range byName {
		if seen[name] {
			continue
		}
		ext := Tunnel{Name: name, Active: true}
		if hasStats {
			ext.HasStats = true
			ext.Peers = len(d.Peers)
			ext.ListenPort = d.ListenPort
		}
		snap.Tunnels = append(snap.Tunnels, ext)
	}
	sort.Slice(snap.Tunnels, func(i, j int) bool { return snap.Tunnels[i].Name < snap.Tunnels[j].Name })

	m.mu.Lock()
	m.last = snap
	m.mu.Unlock()

	m.broadcast(snap)
	return snap
}

func (m *Manager) broadcast(s Snapshot) {
	m.subsMu.Lock()
	defer m.subsMu.Unlock()
	for ch := range m.subs {
		select {
		case ch <- s:
		default:
		}
	}
}

type cfgFile struct{ Name, Path string }

func (m *Manager) listConfigs() ([]cfgFile, error) {
	entries, err := os.ReadDir(m.cfg.ConfigDir)
	if err != nil {
		return nil, err
	}
	var out []cfgFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".conf") {
			continue
		}
		base := strings.TrimSuffix(name, ".conf")
		// wg-quick interface names: <=15 chars, no slashes/whitespace.
		if base == "" || len(base) > 15 || strings.ContainsAny(base, "/ \t") {
			continue
		}
		out = append(out, cfgFile{Name: base, Path: filepath.Join(m.cfg.ConfigDir, name)})
	}
	return out, nil
}

// activeDevices returns WG devices from the kernel. hasStats is true only when
// the call returned full peer data; otherwise peer/transfer fields are unknown
// and must not be fabricated as zero.
func (m *Manager) activeDevices() ([]*wgtypes.Device, bool) {
	defer recoverLog("activeDevices")

	if c, err := wgctrl.New(); err == nil {
		defer c.Close()
		if devs, err := c.Devices(); err == nil {
			return devs, true
		} else {
			logger.L().Debug("wgctrl Devices failed", "err", err)
		}
	} else {
		logger.L().Debug("wgctrl New failed", "err", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "wg", "show", "interfaces").Output()
	if err != nil {
		return sysfsWG(), false
	}
	names := strings.Fields(strings.TrimSpace(string(out)))
	devs := make([]*wgtypes.Device, 0, len(names))
	for _, n := range names {
		devs = append(devs, &wgtypes.Device{Name: n})
	}
	return devs, false
}

func sysfsWG() []*wgtypes.Device {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return nil
	}
	var out []*wgtypes.Device
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join("/sys/class/net", e.Name(), "uevent"))
		if err != nil {
			continue
		}
		if strings.Contains(string(b), "DEVTYPE=wireguard") {
			out = append(out, &wgtypes.Device{Name: e.Name()})
		}
	}
	return out
}

type ToggleResult struct {
	Tunnel  string
	Cmdline string
	Stdout  string
	Stderr  string
	Err     error
}

func (m *Manager) Up(ctx context.Context, name string) ToggleResult {
	if m.cfg.ExclusiveMode {
		for _, t := range m.Last().Tunnels {
			if t.Active && t.Name != name {
				_ = m.Down(ctx, t.Name)
			}
		}
	}
	return m.runQuick(ctx, "up", name)
}

func (m *Manager) Down(ctx context.Context, name string) ToggleResult {
	return m.runQuick(ctx, "down", name)
}

func (m *Manager) Toggle(ctx context.Context, name string) ToggleResult {
	for _, t := range m.Last().Tunnels {
		if t.Name == name {
			if t.Active {
				return m.Down(ctx, name)
			}
			return m.Up(ctx, name)
		}
	}
	return m.Up(ctx, name)
}

func (m *Manager) lockFor(name string) *sync.Mutex {
	m.togMu.Lock()
	defer m.togMu.Unlock()
	mu, ok := m.toggles[name]
	if !ok {
		mu = &sync.Mutex{}
		m.toggles[name] = mu
	}
	return mu
}

func (m *Manager) runQuick(ctx context.Context, action, name string) ToggleResult {
	defer recoverLog("runQuick")

	mu := m.lockFor(name)
	mu.Lock()
	defer mu.Unlock()

	res := ToggleResult{Tunnel: name}

	wgq := m.cfg.WgQuickBin
	if wgq == "" {
		p, err := exec.LookPath("wg-quick")
		if err != nil {
			res.Err = fmt.Errorf("wg-quick not found in PATH")
			return res
		}
		wgq = p
	}

	argv := []string{m.cfg.PrivilegeTool}
	if m.cfg.PrivilegeTool == "sudo" {
		argv = append(argv, "-n")
	}
	argv = append(argv, wgq, action, name)
	res.Cmdline = strings.Join(argv, " ")

	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cctx, argv[0], argv[1:]...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	logger.L().Info("wg-quick", "action", action, "tunnel", name, "cmd", res.Cmdline)
	err := cmd.Run()
	res.Stdout = stdout.String()
	res.Stderr = stderr.String()
	if err != nil {
		res.Err = fmt.Errorf("%s %s: %w (%s)", action, name, err, strings.TrimSpace(res.Stderr))
		logger.L().Warn("wg-quick failed",
			"action", action, "tunnel", name, "err", err, "stderr", strings.TrimSpace(res.Stderr))
	}

	m.refresh()
	return res
}

func recoverLog(where string) {
	if r := recover(); r != nil {
		logger.L().Error("panic recovered", "where", where, "panic", fmt.Sprint(r))
	}
}

func humanBytes(n int64) string {
	const (
		K = 1 << 10
		M = 1 << 20
		G = 1 << 30
	)
	switch {
	case n >= G:
		return fmt.Sprintf("%.2fG", float64(n)/float64(G))
	case n >= M:
		return fmt.Sprintf("%.1fM", float64(n)/float64(M))
	case n >= K:
		return fmt.Sprintf("%.1fK", float64(n)/float64(K))
	default:
		return fmt.Sprintf("%dB", n)
	}
}
