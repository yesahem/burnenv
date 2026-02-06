package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yesahem/burnenv/internal/client"
	"github.com/yesahem/burnenv/internal/ui"
)

var revokeCmd = &cobra.Command{
	Use:   "revoke [url]",
	Short: "Manually destroy a secret before retrieval",
	Long:  `Sends DELETE to the server to burn the secret without ever retrieving it.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runRevoke,
}

func init() {
	rootCmd.AddCommand(revokeCmd)
}

func runRevoke(cmd *cobra.Command, args []string) error {
	link := args[0]
	if !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") {
		return fmt.Errorf("revoke requires a server URL (file paths cannot be revoked)")
	}
	if err := client.Revoke(link); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, ui.Burn.Render("🧨 Secret revoked and burned."))
	return nil
}
