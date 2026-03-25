package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/Alaxay8/routeflux/internal/app"
)

func newDaemonCmd(opts *rootOptions) *cobra.Command {
	var tick time.Duration
	var once bool

	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Run the background subscription refresh daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if tick <= 0 {
				return fmt.Errorf("scheduler tick must be greater than zero")
			}

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
			defer stop()

			if err := opts.service.RestoreRuntime(ctx); err != nil {
				if _, writeErr := fmt.Fprintf(cmd.ErrOrStderr(), "restore runtime: %v\n", err); writeErr != nil {
					return writeErr
				}
			}

			scheduler := app.NewScheduler(opts.service)
			scheduler.SetTick(tick)

			if once {
				scheduler.RunOnce(ctx)
				return nil
			}

			scheduler.Start(ctx)
			<-ctx.Done()
			scheduler.Stop()
			return nil
		},
	}

	cmd.Flags().DurationVar(&tick, "tick", time.Minute, "Background refresh scan interval")
	cmd.Flags().BoolVar(&once, "once", false, "Run one refresh scan and exit")

	return cmd
}
