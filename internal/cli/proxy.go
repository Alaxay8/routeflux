package cli

import (
	"fmt"
	"strconv"

	"github.com/Alaxay8/routeflux/internal/app"
	"github.com/Alaxay8/routeflux/internal/domain"
	"github.com/spf13/cobra"
)

func newProxyCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "proxy", Short: "Configure local and LAN SOCKS5/HTTP proxies"}
	render := func(cmd *cobra.Command, p domain.ProxySettings) error {
		return printOutput(cmd, opts.jsonOutput, p, fmt.Sprintf("allow-lan=%t\nsocks-port=%d\nhttp-port=%d", p.AllowLAN, p.SOCKSPort, p.HTTPPort))
	}
	get := &cobra.Command{Use: "get", Short: "Show proxy settings", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		settings, err := opts.service.GetSettings()
		if err != nil {
			return err
		}
		return render(cmd, settings.Proxy)
	}}
	var lan string
	var socks, http int
	set := &cobra.Command{Use: "set", Short: "Apply proxy settings together", Args: cobra.NoArgs, Example: "routeflux proxy set --allow-lan true --socks-port 10808 --http-port 10809", RunE: func(cmd *cobra.Command, args []string) error {
		update := app.ProxyUpdate{}
		if cmd.Flags().Changed("allow-lan") {
			value, err := strconv.ParseBool(lan)
			if err != nil {
				return fmt.Errorf("allow-lan must be true or false: %w", err)
			}
			update.AllowLAN = &value
		}
		if cmd.Flags().Changed("socks-port") {
			update.SOCKSPort = &socks
		}
		if cmd.Flags().Changed("http-port") {
			update.HTTPPort = &http
		}
		if update.AllowLAN == nil && update.SOCKSPort == nil && update.HTTPPort == nil {
			return fmt.Errorf("provide --allow-lan, --socks-port, or --http-port")
		}
		p, err := opts.service.ConfigureProxy(cmd.Context(), update)
		if err != nil {
			return err
		}
		return render(cmd, p)
	}}
	set.Flags().StringVar(&lan, "allow-lan", "false", "Listen on all IPv4 interfaces (true or false); restrict access with the system firewall")
	set.Flags().IntVar(&socks, "socks-port", 10808, "SOCKS5 listener port")
	set.Flags().IntVar(&http, "http-port", 10809, "HTTP listener port")
	cmd.AddCommand(get, set)
	return cmd
}
