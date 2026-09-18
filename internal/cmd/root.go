// Package cmd is vivi's command line: it opens the browser, reports the
// version, and updates the binary. Everything else happens inside the TUI.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"

	tea "charm.land/bubbletea/v2"
	"github.com/mattn/go-isatty"

	"github.com/lucasassuncao/vivi/internal/app"
	"github.com/lucasassuncao/vivi/internal/tui"
	"github.com/lucasassuncao/vivi/internal/updater"
	"github.com/lucasassuncao/vivi/internal/vault"
	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags. A "go install" build gets none, so
// it falls back to the module version the toolchain stamped into the binary.
var Version = resolveVersion("dev", moduleVersion())

// resolveVersion prefers what the release build injected. "dev" is the value
// that means nothing was injected, and "(devel)" is the build info's own way of
// saying the same thing about a build from a working tree.
func resolveVersion(injected, fromBuild string) string {
	if injected != "dev" {
		return injected
	}
	if fromBuild == "" || fromBuild == "(devel)" {
		return "dev"
	}
	return fromBuild
}

func moduleVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return info.Main.Version
}

// NewRootCmd builds the command tree.
func NewRootCmd() *cobra.Command {
	var (
		themeName  string
		listThemes bool
		readOnly   string
	)

	root := &cobra.Command{
		Use:   "vivi",
		Short: "Browse and edit HashiCorp Vault from the terminal",
		Long: `vivi is an interactive browser for HashiCorp Vault.

It authenticates the way the vault CLI does, from VAULT_ADDR, VAULT_TOKEN and
VAULT_NAMESPACE, and has no configuration of its own.`,
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if listThemes {
				fmt.Fprint(cmd.OutOrStdout(), themeNames())
				return nil
			}

			colors, err := resolveTheme(themeName)
			if err != nil {
				return err
			}
			policy, err := resolveReadOnly(readOnly)
			if err != nil {
				return err
			}
			return run(cmd.Context(), colors, policy)
		},
	}

	root.Flags().StringVar(&themeName, "theme", "", "colour theme to render with (default: adaptive, follows the terminal background; also read from "+ThemeEnvVar+")")
	root.Flags().BoolVar(&listThemes, "list-themes", false, "list the available themes and exit")
	root.Flags().StringVar(&readOnly, "read-only", "", `refuse every write: "on" always, "prod" only when the server is not a recognised sandbox, "off" never (also read from `+ReadOnlyEnvVar+")")
	// A bare --read-only is the obvious spelling and has to mean the obvious, without this it would consume the next argument as its value.
	root.Flags().Lookup("read-only").NoOptDefVal = "on"

	root.SetVersionTemplate("vivi {{.Version}}\n")
	root.AddCommand(SelfUpdateCmd(Version))
	return root
}

// run performs the startup checks and then hands the terminal to the browser.
// The checks happen before the alternate screen: a TUI opening onto an empty
// tree cannot say whether the token is bad, the address wrong, or Vault empty.
func run(ctx context.Context, colors tui.Colors, readOnly app.ReadOnlyPolicy) (err error) {
	if ctx == nil {
		ctx = context.Background()
	}

	// Before anything else, and before any request: without a terminal there is
	// nothing this command can do, and it should cost nothing to find that out.
	if err := requireTerminal(); err != nil {
		return err
	}

	cfg := vault.ConfigFromEnv()
	client, err := vault.New(cfg)
	if err != nil {
		return err
	}

	token, err := vault.Preflight(ctx, cfg, client)
	if err != nil {
		return err
	}

	// The policy meets the address here and becomes an answer. It is resolved
	// once, before the alternate screen: a session that could become read-only
	// halfway through would be a session nobody could reason about.
	access := readOnly.Access(app.ClassifyEnvironment(client.Server().Address))

	// A TUI cannot print: stdout is what it draws on. VIVI_DEBUG names a file
	// to record keystrokes into instead, which is how to see what a terminal
	// and a keyboard layout actually send.
	var debug io.Writer
	if path := os.Getenv(DebugEnvVar); path != "" {
		f, openErr := os.OpenFile(filepath.Clean(path), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600) //#nosec G304 G703 -- the path is the user's own choice
		if openErr != nil {
			return fmt.Errorf("%s=%q: %w", DebugEnvVar, path, openErr)
		}
		// A close that fails is the log's last lines lost, worth a word once the
		// session is over and stderr is a terminal again.
		defer func() {
			if closeErr := f.Close(); closeErr != nil && err == nil {
				err = fmt.Errorf("%s: %w", DebugEnvVar, closeErr)
			}
		}()
		debug = f
	}

	model := tui.New(tui.Options{
		Client: client, Token: token, Colors: colors, Ctx: ctx,
		Access: access, ReadOnly: readOnly, Version: Version, Debug: debug,
	})
	// The alternate screen is asked for by the model's View now, the way every
	// terminal mode is in bubbletea v2: the option that used to be here is gone.
	program := tea.NewProgram(model, tea.WithContext(ctx))
	_, err = program.Run()
	return err
}

// requireTerminal refuses a session with no terminal on either end, which
// otherwise waits for a keystroke that cannot arrive. Two checks, so the
// message names the missing half: stdout and stdin have different fixes.
func requireTerminal() error {
	if !isTerminal(os.Stdout.Fd()) {
		return errors.New("stdout is not a terminal: vivi is an interactive browser and cannot draw into a pipe or a file")
	}
	if !isTerminal(os.Stdin.Fd()) {
		return errors.New("stdin is not a terminal: vivi is an interactive browser and has no keys to read")
	}
	return nil
}

// isTerminal covers the Windows terminals go-isatty reports separately: mintty,
// which is what Git Bash runs in, is a terminal that IsTerminal alone says no to.
func isTerminal(fd uintptr) bool {
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

// cleanOldBinary is a variable so a test can watch startup call it without
// having to own a real executable on disk,  the same seam updater.osExecutable
// and the model's clipboard use.
var cleanOldBinary = updater.CleanOldBinary

// Execute runs the CLI and reports failures on stderr. Bubble Tea runs under a
// context cancelled on SIGINT and SIGTERM, which covers every way a session
// ends that the model never sees: a kill, a closed window, a CI timeout.
func Execute() int {
	cleanOldBinary(os.Stderr)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := NewRootCmd().ExecuteContext(ctx); err != nil {
		// A cancelled context is the signal doing its job, not a failure to report: the user asked for this one.
		if errors.Is(err, context.Canceled) {
			return 0
		}
		fmt.Fprintln(os.Stderr, "vivi: "+err.Error())
		return 1
	}
	return 0
}
