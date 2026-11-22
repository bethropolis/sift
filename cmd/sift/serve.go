package main

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/bethropolis/sift/internal/config"
	"github.com/bethropolis/sift/internal/serve"
)

// newServeCmd starts the browser-frontend server: an embedded Svelte SPA
// backed by the same scan/rank/render engine as the TUI and the MCP server.
func newServeCmd(cfg *config.Config) *cobra.Command {
	serveCfg := &serve.Config{
		Listen:         serve.DefaultListen,
		PairTTL:        serve.DefaultPairTTL,
		PairSessionTTL: serve.DefaultPairSessionTTL,
	}

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the browser file picker",
		Long: `Start a local web server for the embedded browser file picker.

Local use stays loopback-only with a generated token; opening the printed
URL logs in automatically. Remote use requires --allow-remote plus a
password, explicit roots and hosts, and exactly one TLS story.

With --app the UI opens in a chromeless Chromium window, logs itself in with
a short-lived one-time token, and the server exits when that window closes
(use --keep-alive to keep it running). Falls back to a normal browser tab,
then to the printed URL.

With --pair the UI prints a one-time QR for a phone. Paired sessions are
read-only and always redacted. With no --root it serves the current working
directory. Use an HTTPS --public-url (Tailscale Serve, Cloudflare Tunnel,
Caddy, Nginx) for PWA install and Share; --insecure-http is available for a
trusted LAN but is pair-only.`,
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
			if err := applyProfile(cfg, cmd); err != nil {
				return err
			}
			if serveCfg.InsecureHTTP {
				cmd.PrintErrln("WARNING: pairing over plain HTTP; Android Chrome needs HTTPS for PWA install and Share.")
			}
			log := slog.New(slog.NewTextHandler(os.Stderr, nil))
			srv, err := serve.New(serveCfg, cfg, log)
			if err != nil {
				return err
			}
			return srv.Run()
		},
	}

	// Shared engine flags (style, budget, ignore, secrets, scoring...) so
	// profiles, .sift.toml, and completion keep working for serve.
	config.RegisterFlags(cfg, cmd.Flags())
	registerServeFlags(cmd.Flags(), serveCfg)
	return cmd
}

// registerServeFlags binds the serve-only flags. Secrets never travel as
// flag values (shell history, ps): the password comes from a file or env.
func registerServeFlags(fs *pflag.FlagSet, serveCfg *serve.Config) {
	fs.StringVar(&serveCfg.Listen, "listen", serveCfg.Listen, "Host:port to bind (default loopback)")
	fs.StringArrayVar(&serveCfg.Roots, "root", nil, "Allowed project root (repeatable; default cwd with --pair, $HOME locally, required remotely)")
	fs.StringArrayVar(&serveCfg.Deny, "deny", nil, "Extra denylisted path (repeatable)")
	fs.StringVar(&serveCfg.PasswordFile, "password-file", "", "File holding the password (also SIFT_SERVE_PASSWORD)")
	fs.BoolVar(&serveCfg.AllowRemote, "allow-remote", false, "Required for any non-loopback bind")
	fs.StringVar(&serveCfg.TLSCert, "tls-cert", "", "TLS certificate file")
	fs.StringVar(&serveCfg.TLSKey, "tls-key", "", "TLS key file")
	fs.BoolVar(&serveCfg.TLSSelfSigned, "tls-self-signed", false, "Serve with an ephemeral self-signed certificate")
	fs.BoolVar(&serveCfg.BehindProxy, "behind-proxy", false, "Trust X-Forwarded-Proto/For behind a TLS proxy")
	fs.StringArrayVar(&serveCfg.AllowedHosts, "allowed-host", nil, "Extra allowed Host value (repeatable)")
	fs.BoolVar(&serveCfg.AllowUnredacted, "allow-unredacted", false, "Remote only: permit redact=false")
	fs.BoolVar(&serveCfg.AllowClone, "allow-clone", false, "Enable cloning repositories in the web UI (requires git)")
	fs.BoolVar(&serveCfg.Pair, "pair", false, "Serve a read-only mobile session via a one-time QR token")
	fs.StringVar(&serveCfg.PublicURL, "public-url", "", "External HTTPS base URL encoded in the pairing QR")
	fs.DurationVar(&serveCfg.PairTTL, "pair-ttl", serveCfg.PairTTL, "Pairing token lifetime")
	fs.DurationVar(&serveCfg.PairSessionTTL, "pair-session-ttl", serveCfg.PairSessionTTL, "Absolute lifetime of a paired read-only session")
	fs.BoolVar(&serveCfg.InsecureHTTP, "insecure-http", false, "Pair only: allow a plain-HTTP QR URL (loud warning)")
	fs.DurationVar(&serveCfg.IdleTimeout, "idle-timeout", 0, "Exit after no requests for this long (e.g. '30m')")
	fs.BoolVar(&serveCfg.Open, "open", false, "Open the browser (local only)")
	fs.BoolVar(&serveCfg.App, "app", false, "Open the UI in a chromeless app window and stop the server with it (local only)")
	fs.BoolVar(&serveCfg.KeepAlive, "keep-alive", false, "With --app, keep serving after the window closes")
}
