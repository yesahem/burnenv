package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"github.com/yesahem/burnenv/internal/client"
	"github.com/yesahem/burnenv/internal/crypto"
	"github.com/yesahem/burnenv/internal/store"
	"github.com/yesahem/burnenv/internal/ui"
)

var openTUI bool

var openCmd = &cobra.Command{
	Use:   "open [url]",
	Short: "Retrieve, decrypt, and burn a secret",
	Long: `Fetches encrypted payload, decrypts locally, prints to stdout.
Triggers immediate destruction on the server.`,
	Args: cobra.ExactArgs(1),
	RunE: runOpen,
}

func init() {
	rootCmd.AddCommand(openCmd)
	openCmd.Flags().BoolVar(&openTUI, "tui", false, "Use interactive TUI mode")
}

func runOpen(cmd *cobra.Command, args []string) error {
	target := args[0]

	// Fetch payload: URL (server) or file path (mock)
	var payload *crypto.EncryptedPayload
	var err error
	if isURL(target) {
		payload, err = client.Get(target)
		if err != nil {
			return err
		}
	} else {
		mock, err := store.NewMockStore("")
		if err != nil {
			return err
		}
		payload, err = mock.Load(target)
		if err != nil {
			return fmt.Errorf("load: %w", err)
		}
	}

	// TUI mode for password + result display
	fd := int(os.Stdin.Fd())
	isTerminal := term.IsTerminal(fd)
	if openTUI && isTerminal {
		plaintext, err := ui.RunOpenTUI(payload)
		if err != nil {
			return err
		}
		ui.PrintPlaintextToStdout(plaintext)
		return nil
	}

	// Password
	password := os.Getenv("BURNENV_PASSWORD")
	if password == "" {
		pw, err := promptPassword(ui.Prompt.Render("Password: "))
		if err != nil {
			return err
		}
		password = pw
	}
	if password == "" {
		return fmt.Errorf("password cannot be empty")
	}

	// Decrypt locally
	plaintext, err := crypto.Decrypt(payload, password)
	if err != nil {
		return err
	}

	// Print to stdout
	os.Stdout.Write(plaintext)
	if len(plaintext) > 0 && plaintext[len(plaintext)-1] != '\n' {
		os.Stdout.Write([]byte{'\n'})
	}

	// Destruction notice (to stderr so it doesn't pollute piped output)
	fmt.Fprintln(os.Stderr, ui.Burn.Render("🔥 Secret retrieved and burned. One-time use complete."))
	return nil
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
