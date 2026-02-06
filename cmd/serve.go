package cmd

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/yesahem/burnenv/internal/server"
	"github.com/yesahem/burnenv/internal/ui"
)

var (
	serveAddr   string
	serveBaseURL string
)

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().StringVarP(&serveAddr, "addr", "a", ":8080", "Listen address")
	serveCmd.Flags().StringVar(&serveBaseURL, "base-url", "http://localhost:8080", "Base URL for generated links")
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the BurnEnv backend server",
	Long: `Starts the HTTP API for storing encrypted blobs.
In-memory only: restart loses all data. No persistence.`,
	RunE: runServe,
}

func runServe(cmd *cobra.Command, args []string) error {
	store := server.NewStore()
	defer store.Stop()

	handler := server.Handler(store, serveBaseURL)
	srv := &http.Server{
		Addr:    serveAddr,
		Handler: handler,
	}

	// Graceful shutdown
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		srv.Close()
	}()

	fmt.Fprintln(os.Stderr, ui.Success.Render("BurnEnv server listening on "+serveAddr))
	fmt.Fprintln(os.Stderr, ui.Muted.Render("Base URL: "+serveBaseURL))
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}
