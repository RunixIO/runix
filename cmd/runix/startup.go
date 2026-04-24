package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newStartupCmd() *cobra.Command {
	var (
		platform  string
		uninstall bool
	)

	cmd := &cobra.Command{
		Use:   "startup",
		Short: "Install or uninstall Runix as a system service",
		Long: `Install Runix as a system service so it starts automatically on boot.
Supports systemd (Linux) and launchd (macOS).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if uninstall {
				return uninstallService(platform)
			}
			return installService(platform)
		},
	}

	cmd.Flags().StringVar(&platform, "platform", detectInitSystem(), "init system: systemd, launchd")
	cmd.Flags().BoolVar(&uninstall, "uninstall", false, "uninstall the startup service")

	return cmd
}

// detectInitSystem returns the appropriate init system for the current OS.
func detectInitSystem() string {
	if _, err := os.Stat("/run/systemd/system"); err == nil {
		return "systemd"
	}
	if _, err := os.Stat("/Library/LaunchDaemons"); err == nil {
		return "launchd"
	}
	return "systemd"
}

func installService(platform string) error {
	switch platform {
	case "systemd":
		return installSystemd()
	case "launchd":
		return installLaunchd()
	default:
		return fmt.Errorf("unsupported platform: %s (supported: systemd, launchd)", platform)
	}
}

func uninstallService(platform string) error {
	switch platform {
	case "systemd":
		return uninstallSystemd()
	case "launchd":
		return uninstallLaunchd()
	default:
		return fmt.Errorf("unsupported platform: %s", platform)
	}
}

func installSystemd() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable: %w", err)
	}

	unit := fmt.Sprintf(`[Unit]
Description=Runix Process Manager
After=network.target

[Service]
Type=simple
ExecStart=%s daemon run
Restart=on-failure
RestartSec=5
Environment=RUNIX_DAEMON=1

[Install]
WantedBy=default.target
`, exe)

	unitName := "runix.service"
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user")

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("creating systemd dir: %w", err)
	}

	unitPath := filepath.Join(configDir, unitName)
	if err := os.WriteFile(unitPath, []byte(unit), 0o644); err != nil {
		return fmt.Errorf("writing unit file: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Installed systemd user service to %s\n", unitPath)
	fmt.Fprintln(os.Stdout, "Run the following to enable and start:")
	fmt.Fprintln(os.Stdout, "  systemctl --user daemon-reload")
	fmt.Fprintln(os.Stdout, "  systemctl --user enable --now runix.service")
	return nil
}

func uninstallSystemd() error {
	unitName := "runix.service"
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user")
	unitPath := filepath.Join(configDir, unitName)

	if _, err := os.Stat(unitPath); os.IsNotExist(err) {
		fmt.Fprintln(os.Stdout, "Service not installed.")
		return nil
	}

	fmt.Fprintln(os.Stdout, "Run the following to disable and remove:")
	fmt.Fprintln(os.Stdout, "  systemctl --user disable --now runix.service")
	fmt.Fprintf(os.Stdout, "  rm %s\n", unitPath)
	fmt.Fprintln(os.Stdout, "  systemctl --user daemon-reload")
	return nil
}

func installLaunchd() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable: %w", err)
	}

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>io.runix.daemon</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>daemon</string>
        <string>run</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/runix.launchd.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/runix.launchd.log</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>RUNIX_DAEMON</key>
        <string>1</string>
    </dict>
</dict>
</plist>
`, exe)

	label := "io.runix.daemon.plist"
	launchDir := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents")
	plistPath := filepath.Join(launchDir, label)

	if err := os.MkdirAll(launchDir, 0o755); err != nil {
		return fmt.Errorf("creating LaunchAgents dir: %w", err)
	}

	if err := os.WriteFile(plistPath, []byte(plist), 0o644); err != nil {
		return fmt.Errorf("writing plist: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Installed launchd agent to %s\n", plistPath)
	fmt.Fprintln(os.Stdout, "Run the following to load:")
	fmt.Fprintf(os.Stdout, "  launchctl load %s\n", plistPath)
	return nil
}

func uninstallLaunchd() error {
	label := "io.runix.daemon.plist"
	launchDir := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents")
	plistPath := filepath.Join(launchDir, label)

	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		fmt.Fprintln(os.Stdout, "Service not installed.")
		return nil
	}

	fmt.Fprintln(os.Stdout, "Run the following to unload and remove:")
	fmt.Fprintf(os.Stdout, "  launchctl unload %s\n", plistPath)
	fmt.Fprintf(os.Stdout, "  rm %s\n", plistPath)
	return nil
}
