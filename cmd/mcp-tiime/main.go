// Command mcp-tiime runs an MCP server exposing Tiime data over stdio.
//
// Run "mcp-tiime login [email|sms|otp]" to authenticate from a terminal (MFA
// included) and cache a refresh token, and "mcp-tiime check" to list the
// accessible companies. AI agents can log in with the start_login and
// complete_login tools instead.
package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/fx"
	"golang.org/x/term"

	"mcp-tiime/internal/config"
	"mcp-tiime/internal/tiime"
	"mcp-tiime/internal/tools"
)

const version = "0.1.0"

func main() {
	command := ""
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	switch command {
	case "login":
		runCommand(login)
	case "check":
		runCommand(check)
	case "":
		runServer()
	default:
		fmt.Fprintf(os.Stderr, "mcp-tiime: unknown command %q, expected login or check\n", command)
		os.Exit(2)
	}
}

func providers() fx.Option {
	return fx.Provide(
		config.Load,
		newLogger,
		newHTTPClient,
		newAuthenticator,
		newClient,
		newHandler,
	)
}

// runCommand runs a one-shot command. Invoked functions run while the
// application is built, so interactive prompts are not bound by start timeouts.
func runCommand(
	command any,
) {
	app := fx.New(fx.NopLogger, providers(), fx.Invoke(command))
	if err := app.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "mcp-tiime:", err)
		os.Exit(1)
	}
}

func runServer() {
	app := fx.New(
		// stdout carries the MCP protocol: nothing else may write to it.
		fx.NopLogger,
		providers(),
		fx.Invoke(serve),
	)
	// fx.NopLogger silences startup errors, so they are reported here.
	if err := app.Start(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "mcp-tiime:", err)
		os.Exit(1)
	}

	signal := <-app.Wait()
	if err := app.Stop(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "mcp-tiime:", err)
	}

	os.Exit(signal.ExitCode)
}

func newLogger(
	cfg config.Config,
) *slog.Logger {
	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		level = slog.LevelInfo
	}

	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}

func newHTTPClient() *http.Client {
	return &http.Client{Timeout: 60 * time.Second}
}

func newAuthenticator(
	cfg config.Config,
	httpClient *http.Client,
) *tiime.Authenticator {
	return tiime.NewAuthenticator(tiime.AuthConfig{
		AuthURL:    cfg.AuthURL,
		ClientID:   cfg.ClientID,
		Email:      cfg.Email,
		Password:   cfg.Password,
		CachePath:  cfg.TokenCache,
		HTTPClient: httpClient,
	})
}

func newClient(
	cfg config.Config,
	auth *tiime.Authenticator,
	httpClient *http.Client,
) *tiime.Client {
	return tiime.NewClient(cfg.APIURL, cfg.AppVersion, auth, httpClient)
}

func newHandler(
	cfg config.Config,
	client *tiime.Client,
	auth *tiime.Authenticator,
) *tools.Handler {
	return tools.NewHandler(client, auth, cfg.CompanyID, cfg.PDFDir)
}

func serve(
	lc fx.Lifecycle,
	shutdowner fx.Shutdowner,
	logger *slog.Logger,
	handler *tools.Handler,
) {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "mcp-tiime",
		Title:   "Tiime",
		Version: version,
	}, &mcp.ServerOptions{Logger: logger})
	tools.Register(server, handler)

	ctx, cancel := context.WithCancel(context.Background())

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				err := server.Run(ctx, &mcp.StdioTransport{})
				if err != nil && ctx.Err() == nil {
					logger.Error("MCP server stopped", "error", err)
					_ = shutdowner.Shutdown(fx.ExitCode(1))
					return
				}
				_ = shutdowner.Shutdown()
			}()
			return nil
		},
		OnStop: func(context.Context) error {
			cancel()
			return nil
		},
	})
}

func login(
	cfg config.Config,
	httpClient *http.Client,
) error {
	prompter := newTerminalPrompter()
	ctx := context.Background()

	if cfg.Email == "" {
		email, err := prompter.Prompt(ctx, "Tiime email")
		if err != nil {
			return err
		}
		cfg.Email = email
	}
	if cfg.Password == "" {
		password, err := prompter.secret("Password")
		if err != nil {
			return err
		}
		cfg.Password = password
	}

	method := ""
	if len(os.Args) > 2 {
		method = os.Args[2]
	}

	auth := newAuthenticator(cfg, httpClient)
	challenge, err := auth.StartLogin(ctx, method)
	for err == nil && challenge != nil {
		var code string
		code, err = prompter.Prompt(ctx, challenge.Message+"\nVerification code")
		if err != nil {
			return err
		}
		challenge, err = auth.CompleteLogin(ctx, code)
	}
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, "Logged in. Session saved to", cfg.TokenCache)

	return nil
}

func check(
	client *tiime.Client,
) error {
	companies, err := client.Companies(context.Background())
	if err != nil {
		return fmt.Errorf(
			"checking Tiime access: %w",
			err,
		)
	}

	for _, company := range companies {
		fmt.Printf("%d\t%s\n", company.ID, company.Name)
	}

	return nil
}

// terminalPrompter asks questions on stderr and reads answers from stdin.
type terminalPrompter struct {
	reader *bufio.Reader
}

func newTerminalPrompter() *terminalPrompter {
	return &terminalPrompter{reader: bufio.NewReader(os.Stdin)}
}

// Prompt implements tiime.Prompter.
func (p *terminalPrompter) Prompt(
	_ context.Context,
	message string,
) (string, error) {
	fmt.Fprintf(os.Stderr, "%s : ", message)

	line, err := p.reader.ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf(
			"reading answer: %w",
			err,
		)
	}

	return strings.TrimSpace(line), nil
}

func (p *terminalPrompter) secret(
	message string,
) (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return p.Prompt(context.Background(), message)
	}

	fmt.Fprintf(os.Stderr, "%s : ", message)
	password, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf(
			"reading password: %w",
			err,
		)
	}

	return string(password), nil
}
