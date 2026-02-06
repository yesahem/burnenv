package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yesahem/burnenv/internal/ui"
)

var (
	// jsonOutput outputs responses as JSON for scripting
	jsonOutput bool
)

var rootCmd = &cobra.Command{
	Use:   "burnenv",
	Short: "Zero-retention, ephemeral secret-sharing for developers",
	Long: `BurnEnv: Secrets that self-destruct.

BurnEnv is NOT a password manager, vault, or SaaS dashboard.
Secrets are encrypted locally, stored only as ciphertext, and
destroyed after delivery or expiry. The server never sees plaintext.

Run without arguments to launch the interactive TUI.`,
	RunE: runRoot,
}

func runRoot(cmd *cobra.Command, args []string) error {
	if jsonOutput {
		// With --json, show help instead of TUI
		return cmd.Help()
	}
	return ui.RunMainTUI()
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output responses as JSON (for scripting)")
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, ui.Error.Render(err.Error()))
		os.Exit(1)
	}
}
