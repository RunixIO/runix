package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/runixio/runix/internal/config"
	"github.com/runixio/runix/pkg/types"
	"github.com/spf13/cobra"
)

type deployRunner func(name string, args ...string) error

func newDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy <target>",
		Short: "Deploy the current config to a configured remote target",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, err := resolveDeployConfigPath()
			if err != nil {
				return err
			}

			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}

			return runDeploy(args[0], configPath, cfg, runExternalCommand)
		},
	}

	return cmd
}

func resolveDeployConfigPath() (string, error) {
	if cfgFile != "" {
		return filepath.Abs(cfgFile)
	}
	if env := os.Getenv("RUNIX_CONFIG"); env != "" {
		return filepath.Abs(env)
	}
	for _, name := range []string{"runix.yaml", "runix.yml", "runix.json", "runix.toml"} {
		if _, err := os.Stat(name); err == nil {
			return filepath.Abs(name)
		}
	}
	return "", fmt.Errorf("config file not found")
}

func runDeploy(targetName string, configPath string, cfg *types.RunixConfig, runner deployRunner) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	target, ok := cfg.Deploy[targetName]
	if !ok {
		return fmt.Errorf("deploy target %q not found", targetName)
	}

	if target.PreDeploy != "" {
		if err := runner("sh", "-c", target.PreDeploy); err != nil {
			return fmt.Errorf("pre_deploy failed: %w", err)
		}
	}

	remote := deployRemote(target)
	if err := runner("ssh", sshArgs(target, remote, "mkdir -p "+shellQuote(target.Path))...); err != nil {
		return fmt.Errorf("remote mkdir failed: %w", err)
	}

	remoteConfig := strings.TrimRight(target.Path, "/") + "/" + filepath.Base(configPath)
	if err := runner("scp", scpArgs(target, configPath, remote+":"+remoteConfig)...); err != nil {
		return fmt.Errorf("config upload failed: %w", err)
	}

	if target.PostDeploy != "" {
		if err := runner("ssh", sshArgs(target, remote, remoteShell(target.Path, target.PostDeploy))...); err != nil {
			return fmt.Errorf("post_deploy failed: %w", err)
		}
	}

	if target.ReloadCommand != "" {
		if err := runner("ssh", sshArgs(target, remote, remoteShell(target.Path, target.ReloadCommand))...); err != nil {
			return fmt.Errorf("reload_command failed: %w", err)
		}
	}

	return nil
}

func runExternalCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func deployRemote(target types.DeployTarget) string {
	return target.User + "@" + target.Host
}

func sshArgs(target types.DeployTarget, remote string, command string) []string {
	args := []string{}
	if target.Port > 0 {
		args = append(args, "-p", fmt.Sprintf("%d", target.Port))
	}
	args = append(args, remote, command)
	return args
}

func scpArgs(target types.DeployTarget, src string, dst string) []string {
	args := []string{}
	if target.Port > 0 {
		args = append(args, "-P", fmt.Sprintf("%d", target.Port))
	}
	args = append(args, src, dst)
	return args
}

func remoteShell(path string, command string) string {
	return "cd " + shellQuote(path) + " && " + command
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
