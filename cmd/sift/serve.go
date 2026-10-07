package main

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/serve"
)

var serveCfg = &serve.Config{Listen: serve.DefaultListen}

func init() {
	rootCmd.AddCommand(serveCmd)
	// Shared engine flags (style, budget, ignore, secrets, scoring...) so
	// profiles, .sift.toml, and completion keep working for serve.
	config.RegisterFlags(cfg, serveCmd.Flags())
	registerServeFlags(serveCmd.Flags())
}

// registerServeFlags binds the serve-only flags. Secrets never travel as
// flag values (shell history, ps): the password comes from a file or env.
func registerServeFlags(fs *pflag.FlagSet) {
	fs.StringVar(&serveCfg.Listen, "listen", serveCfg.Listen, "Host:port to bind (default loopback)")
	fs.StringArrayVar(&serveCfg.Roots, "root", nil, "Allowed project root (repeatable; default $HOME locally, required remotely)")
	fs.StringArrayVar(&serveCfg.Deny, "deny", nil, "Extra denylisted path (repeatable)")
	fs.StringVar(&serveCfg.PasswordFile, "password-file", "", "File holding the password (also SIFT_SERVE_PASSWORD)")
	fs.BoolVar(&serveCfg.AllowRemote, "allow-remote", false, "Required for any non-loopback bind")
	fs.StringVar(&serveCfg.TLSCert, "tls-cert", "", "TLS certificate file")
	fs.StringVar(&serveCfg.TLSKey, "tls-key", "", "TLS key file")
	fs.BoolVar(&serveCfg.TLSSelfSigned, "tls-self-signed", false, "Serve with an ephemeral self-signed certificate")
	fs.BoolVar(&serveCfg.BehindProxy, "behind-proxy", false, "Trust X-Forwarded-Proto/For behind a TLS proxy")
	fs.StringArrayVar(&serveCfg.AllowedHosts, "allowed-host", nil, "Extra allowed Host value (repeatable)")
	fs.BoolVar(&serveCfg.AllowUnredacted, "allow-unredacted", false, "Remote only: permit redact=false")
	fs.BoolVar(&serveCfg.InsecureHTTP, "insecure-http", false, "Remote without TLS (loud warning)")
	fs.DurationVar(&serveCfg.IdleTimeout, "idle-timeout", 0, "Exit after no requests for this long (e.g. '30m')")
	fs.BoolVar(&serveCfg.Open, "open", false, "Open the browser (local only)")
	fs.BoolVar(&serveCfg.App, "app", false, "Open the UI in a chromeless app window and stop the server with it (local only)")
	fs.BoolVar(&serveCfg.KeepAlive, "keep-alive", false, "With --app, keep serving after the window closes")
}

// serveCmd starts the browser-frontend server: an embedded Svelte SPA backed
// by the same scan/rank/render engine as the TUI and the MCP server.
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve the browser file picker",
	Long: `Start a local web server for the embedded browser file picker.

Local use stays loopback-only with a generated token; opening the printed
URL logs in automatically. Remote use requires --allow-remote plus a
password, explicit roots and hosts, and exactly one TLS story.

With --app the UI opens in a chromeless Chromium window, logs itself in with
a short-lived one-time token, and the server exits when that window closes
(use --keep-alive to keep it running). Falls back to a normal browser tab,
then to the printed URL.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Snapshot explicitly-passed flags before profile resolution: each
		// request re-applies them over the target project's own .sift.toml
		// (CLI flag precedence), so the startup directory never leaks into
		// other projects. Serve-only flags never match an engine flag and
		// are skipped per request.
		serveCfg.EngineFlagOverrides = map[string]string{}
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			if f.Changed {
				serveCfg.EngineFlagOverrides[f.Name] = f.Value.String()
			}
		})
		if err := applyProfile(cmd); err != nil {
			return err
		}
		if serveCfg.InsecureHTTP && serveCfg.AllowRemote {
			cmd.PrintErrln("WARNING: serving remote traffic over plain HTTP; credentials and code cross the network unencrypted.")
		}
		log := slog.New(slog.NewTextHandler(os.Stderr, nil))
		srv, err := serve.New(serveCfg, cfg, log)
		if err != nil {
			return err
		}
		return srv.Run()
	},
}
