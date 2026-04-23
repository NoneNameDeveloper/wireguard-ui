// Package config loads wgtray's TOML configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	ConfigDir     string        `toml:"config_dir"`
	PollInterval  time.Duration `toml:"poll_interval"`
	PrivilegeTool string        `toml:"privilege_tool"`
	WgQuickBin    string        `toml:"wg_quick_bin"`
	Notifications bool          `toml:"notifications"`
	LogFile       string        `toml:"log_file"`
	LogLevel      string        `toml:"log_level"`
	ExclusiveMode bool          `toml:"exclusive_mode"`
	ConfirmToggle bool          `toml:"confirm_toggle"`
	RofiBin       string        `toml:"rofi_bin"`
	RofiArgs      []string      `toml:"rofi_args"`
	IconActive    string        `toml:"icon_active"`
	IconInactive  string        `toml:"icon_inactive"`
	IconError     string        `toml:"icon_error"`

	SourcePath string `toml:"-"`
}

func Defaults() Config {
	return Config{
		ConfigDir:     "/etc/wireguard",
		PollInterval:  3 * time.Second,
		PrivilegeTool: "pkexec",
		Notifications: true,
		LogFile:       filepath.Join(xdgState(), "wgtray", "wgtray.log"),
		LogLevel:      "info",
		ExclusiveMode: true,
		RofiBin:       "rofi",
		RofiArgs:      []string{"-dmenu", "-i", "-p", "wg"},
		IconActive:    "network-vpn",
		IconInactive:  "network-vpn-disconnected",
		IconError:     "network-error",
	}
}

func DefaultPath() string {
	return filepath.Join(xdgConfig(), "wgtray", "config.toml")
}

// Load reads path (or DefaultPath) and returns a fully-populated Config.
// A missing file is not an error — defaults are returned.
func Load(path string) (Config, error) {
	cfg := Defaults()
	if path == "" {
		path = DefaultPath()
	}
	cfg.SourcePath = path

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read %s: %w", path, err)
	}
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return cfg, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := cfg.validate(); err != nil {
		return cfg, fmt.Errorf("invalid %s: %w", path, err)
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.ConfigDir == "" {
		c.ConfigDir = "/etc/wireguard"
	}
	if c.PollInterval < 500*time.Millisecond {
		c.PollInterval = 3 * time.Second
	}
	switch c.PrivilegeTool {
	case "pkexec", "sudo":
	case "":
		c.PrivilegeTool = "pkexec"
	default:
		return fmt.Errorf("privilege_tool must be pkexec or sudo, got %q", c.PrivilegeTool)
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	return nil
}

// WriteDefaultIfMissing writes an annotated default config if path does not
// exist. Returns the written path, or "" if the file was already present.
func WriteDefaultIfMissing(path string) (string, error) {
	if path == "" {
		path = DefaultPath()
	}
	if _, err := os.Stat(path); err == nil {
		return "", nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(sample), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func xdgConfig() string {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

func xdgState() string {
	if v := os.Getenv("XDG_STATE_HOME"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state")
}

const sample = `# wgtray configuration. Every field is optional.

config_dir     = "/etc/wireguard"
poll_interval  = "3s"
privilege_tool = "pkexec"        # or "sudo" (requires NOPASSWD entry)
notifications  = true
log_level      = "info"          # debug | info | warn | error
exclusive_mode = true            # bring other tunnels down when raising one
confirm_toggle = false           # notify before running wg-quick

rofi_bin  = "rofi"
rofi_args = ["-dmenu", "-i", "-p", "wg"]

icon_active   = "network-vpn"
icon_inactive = "network-vpn-disconnected"
icon_error    = "network-error"
`
