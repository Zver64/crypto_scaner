package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"crypto-scanner/internal/apiclient"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// requestTimeout bounds every API call; the server ends a backtest well
// within it.
const requestTimeout = time.Minute

type cli struct {
	profileName string
	json        bool
}

// run executes the command line; cobra prints every error, and any error
// exits with 1.
func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	c := &cli{}
	root := c.command()
	root.SetArgs(args)
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if root.ExecuteContext(ctx) != nil {
		return 1
	}
	return 0
}

func (c *cli) command() *cobra.Command {
	root := &cobra.Command{
		Use:   "scanner",
		Short: "Write and backtest Crypto Scanner strategies through the API",
		Long: `Write and backtest Crypto Scanner strategies (CEL, see docs/strategy-language.md)
through the backend API. Profiles live in ~/.config/scanner/config.json.`,
		SilenceUsage: true,
	}
	root.SetErrPrefix("scanner:")
	root.CompletionOptions.SetDefaultShellCompDirective(cobra.ShellCompDirectiveNoFileComp)
	root.PersistentFlags().StringVar(&c.profileName, "profile", "", "server profile `NAME`; default $SCANNER_PROFILE, then the current one")
	root.PersistentFlags().BoolVar(&c.json, "json", false, "print the API response as JSON")
	_ = root.RegisterFlagCompletionFunc("profile", completeProfiles)
	root.AddCommand(c.loginCommand(), profilesCommand(), useCommand(),
		c.varsCommand(), c.validateCommand(), c.favoritesCommand(), c.strategiesCommand(), c.indicatorsCommand(), c.backtestCommand())
	return root
}

// completeProfiles completes the profile names of the local configuration.
func completeProfiles(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
	_, loaded, err := readConfig()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	return slices.Sorted(maps.Keys(loaded.Profiles)), cobra.ShellCompDirectiveNoFileComp
}

func (c *cli) loginCommand() *cobra.Command {
	var server string
	command := &cobra.Command{
		Use:   "login NAME --server URL",
		Short: "Read an API token (cst_…) from stdin, check it, and save it as profile NAME",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := checkServer(server); err != nil {
				return err
			}
			token, err := readToken(command)
			if err != nil {
				return err
			}
			saved := profile{Server: strings.TrimRight(server, "/"), Token: token}
			client, err := newClient(saved)
			if err != nil {
				return err
			}
			user, err := client.GetCurrentUserWithResponse(command.Context())
			if err == nil {
				err = check(user, user.JSON200 != nil)
			}
			if err != nil {
				return fmt.Errorf("check the token: %w", err)
			}
			if !user.JSON200.Administrator {
				return errors.New("the token does not belong to the scanner administrator")
			}
			path, loaded, err := readConfig()
			if err != nil {
				return err
			}
			loaded.Profiles[args[0]] = saved
			if loaded.Current == "" {
				loaded.Current = args[0]
			}
			if err := saveConfig(path, loaded); err != nil {
				return err
			}
			command.Printf("saved profile %s (%s); current: %s\n", args[0], saved.Server, loaded.Current)
			return nil
		},
	}
	command.Flags().StringVar(&server, "server", "", "server `URL`: https, or http on localhost")
	_ = command.MarkFlagRequired("server")
	return command
}

// checkServer accepts https, and http only on the loopback interface: the
// token never expires and carries administrator rights, so it must not cross
// a network in cleartext.
func checkServer(server string) error {
	parsed, err := url.Parse(server)
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("--server %q is not a URL such as https://scanner.example", server)
	}
	host := parsed.Hostname()
	ip := net.ParseIP(host)
	if parsed.Scheme == "https" || parsed.Scheme == "http" && (host == "localhost" || ip != nil && ip.IsLoopback()) {
		return nil
	}
	return errors.New("--server must use https; http is allowed only for localhost")
}

// readToken reads the token without echo from a terminal, or the whole
// standard input otherwise.
func readToken(command *cobra.Command) (string, error) {
	var raw []byte
	var err error
	if file, ok := command.InOrStdin().(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		command.PrintErr("API token: ")
		raw, err = term.ReadPassword(int(file.Fd()))
		command.PrintErrln()
	} else {
		raw, err = io.ReadAll(io.LimitReader(command.InOrStdin(), 4096))
	}
	if err != nil {
		return "", fmt.Errorf("read the token: %w", err)
	}
	token := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(token, "cst_") || strings.ContainsAny(token, " \t\r\n") {
		return "", errors.New("standard input is not an API token (cst_…); create one in the Mini App settings")
	}
	return token, nil
}

func profilesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "profiles",
		Short: "List profiles; * marks the current one",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			_, loaded, err := readConfig()
			if err != nil {
				return err
			}
			for _, name := range slices.Sorted(maps.Keys(loaded.Profiles)) {
				marker := " "
				if name == loaded.Current {
					marker = "*"
				}
				command.Printf("%s %s %s\n", marker, name, loaded.Profiles[name].Server)
			}
			if len(loaded.Profiles) == 0 {
				command.Println("no profiles; run scanner login")
			}
			return nil
		},
	}
}

func useCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "use NAME",
		Short: "Make NAME the current profile",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(command *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return completeProfiles(command, args, toComplete)
		},
		RunE: func(command *cobra.Command, args []string) error {
			path, loaded, err := readConfig()
			if err != nil {
				return err
			}
			if _, ok := loaded.Profiles[args[0]]; !ok {
				return fmt.Errorf("no profile %q; run scanner login", args[0])
			}
			loaded.Current = args[0]
			if err := saveConfig(path, loaded); err != nil {
				return err
			}
			command.Printf("current: %s\n", loaded.Current)
			return nil
		},
	}
}

// client returns the API client of the selected profile: --profile, then
// $SCANNER_PROFILE, then the current one.
func (c *cli) client() (*apiclient.ClientWithResponses, error) {
	_, loaded, err := readConfig()
	if err != nil {
		return nil, err
	}
	name := c.profileName
	if name == "" {
		name = os.Getenv("SCANNER_PROFILE")
	}
	if name == "" {
		name = loaded.Current
	}
	selected, ok := loaded.Profiles[name]
	if !ok {
		return nil, fmt.Errorf("no profile %q; run scanner login", name)
	}
	return newClient(selected)
}

func newClient(selected profile) (*apiclient.ClientWithResponses, error) {
	return apiclient.NewClientWithResponses(selected.Server,
		apiclient.WithHTTPClient(&http.Client{
			Timeout: requestTimeout,
			// The API never redirects; following one could send the token
			// elsewhere.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}),
		apiclient.WithRequestEditorFn(func(_ context.Context, request *http.Request) error {
			request.Header.Set("Authorization", "Bearer "+selected.Token)
			return nil
		}))
}

// apiResponse is a generated response; every operation declares these errors.
type apiResponse interface {
	StatusCode() int
	GetJSON401() *apiclient.ErrorResponse
	GetJSON403() *apiclient.ErrorResponse
	GetJSON500() *apiclient.ErrorResponse
}

// check returns nil when a generated response holds its expected success, and
// otherwise the code and message of the API error it decoded, among the
// operation's own failures and the common ones, or its HTTP status.
func check(response apiResponse, succeeded bool, failures ...*apiclient.ErrorResponse) error {
	if succeeded {
		return nil
	}
	status := response.StatusCode()
	failure := cmp.Or(append(failures, response.GetJSON401(), response.GetJSON403(), response.GetJSON500())...)
	switch {
	case failure != nil && failure.Error.Code != "" && status == http.StatusUnauthorized:
		return fmt.Errorf("%s: %s (revoked or wrong token? run scanner login)", failure.Error.Code, failure.Error.Message)
	case failure != nil && failure.Error.Code != "":
		return fmt.Errorf("%s: %s", failure.Error.Code, failure.Error.Message)
	case status >= 200 && status < 300:
		return fmt.Errorf("unexpected API response (HTTP %d); check the server URL and proxy", status)
	default:
		return fmt.Errorf("HTTP %d", status)
	}
}
