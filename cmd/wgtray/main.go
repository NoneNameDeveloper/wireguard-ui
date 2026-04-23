// Command wgtray is a minimal WireGuard tray / rofi / polybar frontend.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/NoneNameDeveloper/wireguard-ui/internal/app"
	"github.com/NoneNameDeveloper/wireguard-ui/internal/config"
	"github.com/NoneNameDeveloper/wireguard-ui/internal/logger"
	"github.com/NoneNameDeveloper/wireguard-ui/internal/notify"
	"github.com/NoneNameDeveloper/wireguard-ui/internal/rofi"
	"github.com/NoneNameDeveloper/wireguard-ui/internal/tray"
	"github.com/NoneNameDeveloper/wireguard-ui/internal/wg"
)

var Version = "dev"

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "wgtray: fatal panic: %v\n%s\n", r, debug.Stack())
			os.Exit(2)
		}
	}()

	var (
		cfgPath     = flag.String("config", "", "config file (default $XDG_CONFIG_HOME/wgtray/config.toml)")
		rofiMode    = flag.Bool("rofi", false, "one-shot rofi picker, then exit")
		statusMode  = flag.Bool("status", false, "print tunnel status and exit")
		watchMode   = flag.Bool("watch", false, "tail-print one status line per change (polybar tail=true)")
		watchFormat = flag.String("format", "plain", "watch format: plain | polybar")
		infoMode    = flag.Bool("info", false, "show a notification with current state and exit")
		downAll     = flag.Bool("down-all", false, "bring every active tunnel down and exit")
		initMode    = flag.Bool("init", false, "write an annotated default config and exit")
		printConfig = flag.Bool("print-config", false, "print effective config and exit")
		versionMode = flag.Bool("version", false, "print version and exit")
	)
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(),
			"wgtray %s — WireGuard tray / rofi / polybar frontend\n\nUsage: %s [flags]\n\n",
			Version, os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	tray.Version = Version

	if *versionMode {
		fmt.Println("wgtray", Version)
		return
	}

	cfg, cfgErr := config.Load(*cfgPath)
	_ = logger.Init(cfg.LogFile, logger.ParseLevel(cfg.LogLevel))
	log := logger.L()

	if cfgErr != nil {
		log.Warn("config", "err", cfgErr)
		fmt.Fprintf(os.Stderr, "wgtray: %v (using defaults)\n", cfgErr)
	}
	log.Info("start", "version", Version, "config", cfg.SourcePath, "log_file", cfg.LogFile)

	if *initMode {
		path, err := config.WriteDefaultIfMissing(cfg.SourcePath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "wgtray:", err)
			os.Exit(1)
		}
		if path == "" {
			fmt.Println("config already exists:", cfg.SourcePath)
		} else {
			fmt.Println("wrote:", path)
		}
		return
	}

	if *printConfig {
		app.PrintConfig(os.Stdout, cfg)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mgr := wg.New(cfg)
	n := notify.New(cfg.Notifications)

	switch {
	case *statusMode:
		mgr.Refresh()
		app.PrintStatus(os.Stdout, mgr.Last())
	case *downAll:
		snap := mgr.Refresh()
		for _, t := range snap.Tunnels {
			if !t.Active {
				continue
			}
			if res := mgr.Down(ctx, t.Name); res.Err != nil {
				log.Warn("down", "tunnel", t.Name, "err", res.Err)
				n.Error("wgtray: "+t.Name+" failed", res.Err.Error())
			} else {
				n.Info("wgtray: "+t.Name, "down")
			}
		}
	case *watchMode:
		mgr.Refresh()
		go mgr.Run(ctx)
		app.Watch(ctx, mgr, os.Stdout, *watchFormat)
	case *infoMode:
		mgr.Refresh()
		app.ShowInfo(mgr.Last(), n)
	case *rofiMode:
		if err := rofi.Run(ctx, cfg, mgr, n); err != nil {
			log.Error("rofi", "err", err)
			os.Exit(1)
		}
	default:
		go mgr.Run(ctx)
		time.Sleep(150 * time.Millisecond)
		t := tray.New(cfg, mgr, n)
		t.Run(ctx)
	}
}
