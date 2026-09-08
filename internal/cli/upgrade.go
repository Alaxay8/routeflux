package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

const routefluxLatestInstallScriptURL = "https://github.com/Alaxay8/routeflux/releases/latest/download/install.sh"

var routefluxUpgradeInstallerPath = "/tmp/routeflux-install.sh"

type upgradeResult struct {
	Status         string `json:"status"`
	URL            string `json:"url"`
	ScriptPath     string `json:"script_path"`
	DownloadOutput string `json:"download_output,omitempty"`
	InstallOutput  string `json:"install_output,omitempty"`
}

func runUpgrade(cmd *cobra.Command, jsonOutput bool) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	selfUpdatePath := "/usr/libexec/routeflux-self-update"
	if os.Getenv("ROUTEFLUX_FORCE_UPGRADE") == "" {
		if _, err := os.Stat(selfUpdatePath); err == nil {
			external := exec.CommandContext(ctx, selfUpdatePath)
			var combined bytes.Buffer
			if jsonOutput {
				external.Stdout = &combined
				external.Stderr = &combined
			} else {
				external.Stdout = io.MultiWriter(cmd.OutOrStdout(), &combined)
				external.Stderr = io.MultiWriter(cmd.ErrOrStderr(), &combined)
			}
			if err := external.Run(); err != nil {
				return fmt.Errorf("self-update wrapper: %w", err)
			}

			if jsonOutput {
				output := combined.String()
				status := "ok"
				if strings.Contains(output, "ROUTEFLUX_SELF_UPDATE_STATUS=up-to-date") {
					status = "up-to-date"
				} else if strings.Contains(output, "ROUTEFLUX_SELF_UPDATE_STATUS=updated") {
					status = "updated"
				}

				lines := strings.Split(output, "\n")
				var cleanLines []string
				for _, line := range lines {
					if !strings.HasPrefix(line, "ROUTEFLUX_SELF_UPDATE_STATUS=") {
						cleanLines = append(cleanLines, line)
					}
				}
				cleanMsg := strings.TrimSpace(strings.Join(cleanLines, "\n"))

				res := upgradeResult{
					Status:        status,
					URL:           routefluxLatestInstallScriptURL,
					ScriptPath:    routefluxUpgradeInstallerPath,
					InstallOutput: cleanMsg,
				}
				return printOutput(cmd, true, res, "")
			}
			return nil
		}
	}

	result := upgradeResult{
		Status:     "ok",
		URL:        routefluxLatestInstallScriptURL,
		ScriptPath: routefluxUpgradeInstallerPath,
	}

	downloadOutput, err := runUpgradeCommand(ctx, cmd, jsonOutput, "wget", "-O", routefluxUpgradeInstallerPath, routefluxLatestInstallScriptURL)
	if err != nil {
		return fmt.Errorf("download latest installer: %w", err)
	}
	result.DownloadOutput = strings.TrimSpace(downloadOutput)

	installOutput, err := runUpgradeCommand(ctx, cmd, jsonOutput, "sh", routefluxUpgradeInstallerPath, "--no-deps")
	if err != nil {
		return fmt.Errorf("run latest installer: %w", err)
	}
	result.InstallOutput = strings.TrimSpace(installOutput)

	if jsonOutput {
		return printOutput(cmd, true, result, "")
	}

	return printOutput(cmd, false, nil, "Upgrade completed using the latest release installer.")
}

func runUpgradeCommand(ctx context.Context, cmd *cobra.Command, jsonOutput bool, name string, args ...string) (string, error) {
	external := exec.CommandContext(ctx, name, args...)

	var combined bytes.Buffer
	if jsonOutput {
		external.Stdout = &combined
		external.Stderr = &combined
	} else {
		external.Stdout = io.MultiWriter(cmd.OutOrStdout(), &combined)
		external.Stderr = io.MultiWriter(cmd.ErrOrStderr(), &combined)
	}

	if err := external.Run(); err != nil {
		return combined.String(), fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}

	return combined.String(), nil
}
