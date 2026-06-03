package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/mdp/qrterminal/v3"
	"github.com/spf13/cobra"

	"github.com/dhruvsaxena1998/cleo/internal/reconcile"
	"github.com/dhruvsaxena1998/cleo/internal/serve"
	"github.com/dhruvsaxena1998/cleo/internal/state"
)

func newServeCmd(getCtx func() *Ctx) *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start a read-only remote view of your sessions (scan the QR from your phone)",
		Long: "Start an opt-in, read-only HTTP server on your LAN that lists every\n" +
			"session and which one needs your attention. Scan the printed QR from a\n" +
			"phone on the same network. It never attaches to or sends input to a\n" +
			"session. Access is gated by a per-run token in the URL. Press Ctrl-C to stop.",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getCtx()

			token, err := serve.NewToken()
			if err != nil {
				return err
			}

			// The remote view reuses the `cleo ls` data path: reconcile against
			// tmux, then read durable state. No new tmux calls beyond reconcile.
			snapshot := func() []state.Session {
				_ = reconcile.RunOpts(c.State, c.Tmux, reconcile.Options{
					IdleTimeout:     c.Config.Timeouts.IdleToCompletedTimeout,
					SpawningTimeout: c.Config.Timeouts.SpawningTimeout,
				})
				sessions, _ := c.State.List()
				return sessions
			}

			srv, err := serve.NewServer(token, snapshot)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			ip := serve.LANIP()
			if ip == "" {
				ip = "localhost"
				fmt.Fprintln(out, "warning: could not detect a LAN IP; using localhost (reachable only from this machine)")
			}
			url := serve.URL(ip, port, token)

			addr := fmt.Sprintf("0.0.0.0:%d", port)
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				return fmt.Errorf("cannot bind %s: %w", addr, err)
			}

			fmt.Fprintf(out, "\ncleo remote view — read-only\n\n")
			qrterminal.GenerateHalfBlock(url, qrterminal.L, out)
			fmt.Fprintf(out, "\nScan the code, or open on a device on this network:\n  %s\n\n", url)
			fmt.Fprintln(out, "Anyone with this URL on your network can view (not control) your sessions.")
			fmt.Fprintln(out, "Press Ctrl-C to stop.")

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			httpSrv := &http.Server{
				Handler:           srv.Handler(),
				ReadHeaderTimeout: 5 * time.Second,
			}
			go func() {
				<-ctx.Done()
				shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				_ = httpSrv.Shutdown(shutCtx)
			}()

			err = httpSrv.Serve(ln)
			if errors.Is(err, http.ErrServerClosed) {
				fmt.Fprintln(out, "\nstopped.")
				return nil
			}
			return err
		},
	}
	cmd.Flags().IntVar(&port, "port", 7777, "port to bind on all network interfaces")
	return cmd
}
