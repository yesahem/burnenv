package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yesahem/burnenv/internal/client"
	"github.com/yesahem/burnenv/internal/crypto"
	"github.com/yesahem/burnenv/internal/store"
	"github.com/yesahem/burnenv/internal/ui"
	"golang.org/x/term"
)

var (
	expiryMinutes int
	maxViews      int
	password      string
	serverURL     string
	useTUI        bool
)

func init() {
	rootCmd.AddCommand(createCmd)
	createCmd.Flags().IntVar(&expiryMinutes, "expiry", 3, "Expiry in minutes (2-10)")
	createCmd.Flags().IntVar(&maxViews, "max-views", 1, "Maximum number of views before destruction")
	createCmd.Flags().StringVar(&password, "password", "", "Password (prefer BURNENV_PASSWORD env; avoid passing on CLI)")
	createCmd.Flags().StringVar(&serverURL, "server", "", "Server base URL (e.g. http://localhost:8080). Omit for local mock.")
	createCmd.Flags().BoolVar(&useTUI, "tui", false, "Use interactive TUI mode")
}

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a burn link from secret data",
	Long: `Reads secret from STDIN or interactive prompt.
Encrypts locally and sends only ciphertext to the server.`,
	RunE: runCreate,
}

func runCreate(cmd *cobra.Command, args []string) error {
	// TUI mode: interactive only, skip when piping or --json
	stat, _ := os.Stdin.Stat()
	isInteractive := (stat.Mode() & os.ModeCharDevice) != 0
	if useTUI && isInteractive && !jsonOutput {
		link, err := ui.RunCreateTUI(expiryMinutes, maxViews, serverURL)
		if err != nil {
			return err
		}
		// Link on stdout for piping; status was shown in TUI
		fmt.Println(link)
		return nil
	}

	// Read secret: STDIN if available, else interactive
	secret, err := readSecret()
	if err != nil {
		return err
	}
	if len(secret) == 0 {
		return fmt.Errorf("secret cannot be empty")
	}

	// Validate and get options
	if expiryMinutes < 2 || expiryMinutes > 10 {
		return fmt.Errorf("expiry must be between 2 and 10 minutes")
	}
	if maxViews < 1 {
		maxViews = 1
	}

	if password == "" {
		password = os.Getenv("BURNENV_PASSWORD")
	}
	if password == "" {
		pw, err := promptPassword("Password: ")
		if err != nil {
			return err
		}
		password = pw
	}
	if password == "" {
		return fmt.Errorf("password cannot be empty")
	}

	// Encrypt locally (server never sees plaintext)
	payload, err := crypto.Encrypt(secret, password)
	if err != nil {
		return err
	}

	expiry := time.Now().Add(time.Duration(expiryMinutes) * time.Minute).Unix()
	payload.Expiry = expiry
	payload.MaxViews = maxViews

	url := serverURL
	if url == "" {
		url = os.Getenv("BURNENV_SERVER")
	}
	var link string
	if url != "" {
		link, err = client.Create(url, payload)
		if err != nil {
			return fmt.Errorf("server: %w", err)
		}
	} else {
		mock, err := store.NewMockStore("")
		if err != nil {
			return err
		}
		link, err = mock.Save(payload)
		if err != nil {
			return err
		}
	}

	// Output
	if jsonOutput {
		out := struct {
			Link          string `json:"link"`
			ExpiryMinutes int    `json:"expiry_minutes"`
			MaxViews      int    `json:"max_views"`
		}{Link: link, ExpiryMinutes: expiryMinutes, MaxViews: maxViews}
		enc := json.NewEncoder(os.Stdout)
		return enc.Encode(out)
	}

	if url != "" {
		fmt.Fprintln(os.Stderr, ui.Success.Render("✓ Secret encrypted. Burn link:"))
	} else {
		fmt.Fprintln(os.Stderr, ui.Success.Render("✓ Secret encrypted. Burn link (mock - local file):"))
	}
	fmt.Println(ui.Link.Render(link))
	fmt.Fprintln(os.Stderr, ui.Muted.Render(fmt.Sprintf("Expires: %d min | Max views: %d", expiryMinutes, maxViews)))
	return nil
}

// readSecret reads from stdin if it's a pipe, else prompts interactively.
func readSecret() ([]byte, error) {
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Stdin is a pipe
		var data []byte
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			data = append(data, scanner.Bytes()...)
			data = append(data, '\n')
		}
		if err := scanner.Err(); err != nil {
			return nil, err
		}
		if len(data) > 0 && data[len(data)-1] == '\n' {
			data = data[:len(data)-1]
		}
		return data, nil
	}

	// Interactive prompt
	fmt.Print("Secret (end with Ctrl+D or empty line): ")
	reader := bufio.NewReader(os.Stdin)
	var lines []string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSuffix(line, "\n")
		if line == "" {
			break
		}
		lines = append(lines, line)
	}
	return []byte(strings.Join(lines, "\n")), nil
}

func promptPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	defer fmt.Fprintln(os.Stderr)
	return readPassword()
}

// readPassword reads a line from stdin with terminal echo disabled.
func readPassword() (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		var s string
		_, err := fmt.Scanln(&s)
		return s, err
	}
	b, err := term.ReadPassword(fd)
	return string(b), err
}
