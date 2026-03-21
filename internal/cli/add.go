package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Alaxay8/routeflux/internal/app"
)

func newAddCmd(opts *rootOptions) *cobra.Command {
	var rawURL string
	var rawPayload string
	var name string

	cmd := &cobra.Command{
		Use:   "add [subscription-link-or-json]",
		Short: "Paste a subscription link, key, or 3x-ui/Xray JSON",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if rawURL == "" && rawPayload == "" && len(args) == 0 {
				if _, err := fmt.Fprintln(cmd.ErrOrStderr(), "Paste subscription link, key, or 3x-ui JSON. Press Ctrl-D when done:"); err != nil {
					return err
				}
			}

			req, err := prepareAddRequest(rawURL, rawPayload, name, args, cmd.InOrStdin())
			if err != nil {
				return err
			}

			sub, err := opts.service.AddSubscription(context.Background(), req)
			if err != nil {
				return err
			}

			return printOutput(cmd, opts.jsonOutput, sub, fmt.Sprintf("Added %s (%s) with %d node(s)", sub.DisplayName, sub.ID, len(sub.Nodes)))
		},
	}

	cmd.Flags().StringVar(&rawURL, "url", "", "Subscription URL")
	cmd.Flags().StringVar(&rawPayload, "raw", "", "Raw subscription payload")
	cmd.Flags().StringVar(&name, "name", "", "Optional display name")

	return cmd
}

func prepareAddRequest(rawURL, rawBody, name string, args []string, input io.Reader) (app.AddSubscriptionRequest, error) {
	req := app.AddSubscriptionRequest{Name: name}

	if strings.TrimSpace(rawURL) != "" || strings.TrimSpace(rawBody) != "" {
		req.URL = strings.TrimSpace(rawURL)
		req.Raw = strings.TrimSpace(rawBody)
		return req, nil
	}

	var candidate string
	if len(args) > 0 {
		candidate = strings.TrimSpace(args[0])
	} else {
		payload, err := io.ReadAll(input)
		if err != nil {
			return app.AddSubscriptionRequest{}, fmt.Errorf("read input: %w", err)
		}
		candidate = strings.TrimSpace(string(payload))
	}

	if candidate == "" {
		return app.AddSubscriptionRequest{}, fmt.Errorf("empty input: paste a subscription link, key, or 3x-ui JSON")
	}

	if looksLikeRemoteURL(candidate) {
		req.URL = candidate
		return req, nil
	}

	req.Raw = candidate
	return req, nil
}

func looksLikeRemoteURL(candidate string) bool {
	candidate = strings.TrimSpace(candidate)
	return strings.HasPrefix(candidate, "http://") || strings.HasPrefix(candidate, "https://")
}
