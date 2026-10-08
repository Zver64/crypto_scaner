package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"crypto-scanner/internal/apiclient"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"
)

// requestTimeout bounds every API call; the server ends a backtest well
// within it.
const requestTimeout = time.Minute

// cli holds the global flags and the profile and server a command talks to,
// which error messages name.
type cli struct {
	profileName string
	json        bool
	profile     string
	server      string
}

// run executes the command line and prints its error on stderr: with --json
// in the API error shape, otherwise as text. Any error exits with 1; an
// interrupted command exits quietly.
func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	c := &cli{}
	root := c.command()
	root.SetArgs(args)
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)
	err := root.ExecuteContext(ctx)
	switch {
	case err == nil:
		return 0
	case ctx.Err() != nil:
		return 1
	}
	failed := c.describe(err)
	c.json = errorOutputJSON(root, args)
	if c.json {
		raw, _ := json.Marshal(failed.response())
		fmt.Fprintln(stderr, string(raw))
	} else {
		fmt.Fprintln(stderr, "scanner: "+failed.Error())
	}
	return 1
}

// errorOutputJSON uses pflag's argument consumption but ignores validation
// failures, so a later --json still selects the error format. Copies keep
// recovery from changing the command's actual flag values.
func errorOutputJSON(root *cobra.Command, args []string) bool {
	var enabled bool
	flags := pflag.NewFlagSet("error output", pflag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.ParseErrorsWhitelist.UnknownFlags = true
	add := func(source *pflag.FlagSet) {
		source.VisitAll(func(flag *pflag.Flag) {
			if flags.Lookup(flag.Name) != nil {
				return
			}
			copied := *flag
			value := &errorFlagValue{kind: flag.Value.Type()}
			if flag.Name == "json" {
				value.enabled = &enabled
			}
			copied.Value = value
			flags.AddFlag(&copied)
		})
	}
	add(root.PersistentFlags())
	if command, _, _ := root.Find(args); command != nil {
		add(command.Flags())
		add(command.InheritedFlags())
	}
	args = slices.Clone(args)
	for {
		enabled = false
		err := flags.Parse(args)
		var syntax *pflag.InvalidSyntaxError
		if !errors.As(err, &syntax) {
			break
		}
		// Keep each token in place (it may also occur as a string flag's
		// value), but neutralize the invalid spelling for the recovery pass.
		// The original Cobra error is still the one reported to the user.
		for i, arg := range args {
			if arg == syntax.GetSpecifiedFlag() {
				args[i] = "invalid-flag:" + arg
			}
		}
	}
	return enabled
}

type errorFlagValue struct {
	kind    string
	enabled *bool
}

func (v *errorFlagValue) String() string { return "" }
func (v *errorFlagValue) Type() string   { return v.kind }
func (v *errorFlagValue) Set(value string) error {
	if v.enabled != nil {
		if enabled, err := strconv.ParseBool(value); err == nil {
			*v.enabled = enabled
		}
	}
	return nil
}

func (c *cli) command() *cobra.Command {
	root := &cobra.Command{
		Use:   "scanner",
		Short: "Write and backtest Crypto Scanner strategies through the API",
		Long: `Write and backtest Crypto Scanner strategies (CEL, see docs/strategy-language.md)
through the backend API. Profiles live in $XDG_CONFIG_HOME/scanner/config.json,
by default ~/.config/scanner/config.json. Errors go to stderr, with --json in the
API error shape {"error":{"code","message","details"},"request_id"}.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.CompletionOptions.SetDefaultShellCompDirective(cobra.ShellCompDirectiveNoFileComp)
	root.PersistentFlags().StringVar(&c.profileName, "profile", "", "server profile `NAME`; default $SCANNER_PROFILE, then the current one")
	root.PersistentFlags().BoolVar(&c.json, "json", false, "print the result as JSON, and errors as JSON on stderr")
	_ = root.RegisterFlagCompletionFunc("profile", completeProfiles)
	root.AddCommand(c.loginCommand(), c.profilesCommand(), c.useCommand(),
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

// completeStrategies completes the IDs of the saved strategies, each with its
// name.
func (c *cli) completeStrategies(command *cobra.Command, _ []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
	client, err := c.client()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	strategies, err := client.ListStrategiesWithResponse(command.Context())
	if err != nil || strategies.JSON200 == nil {
		return nil, cobra.ShellCompDirectiveError
	}
	completions := make([]cobra.Completion, len(strategies.JSON200.Items))
	for i, strategy := range strategies.JSON200.Items {
		completions[i] = cobra.CompletionWithDesc(strconv.FormatInt(strategy.Id, 10), strategy.Name)
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

// completeStrategyArg completes the strategy ID argument of update and
// delete.
func (c *cli) completeStrategyArg(command *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return c.completeStrategies(command, args, toComplete)
}

// completeFavorites completes the administrator's favorites, the coins
// strategies watch; a backtest takes any coin all the same.
func (c *cli) completeFavorites(command *cobra.Command, _ []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
	client, err := c.client()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	favorites, err := client.ListFavoritesWithResponse(command.Context())
	if err != nil || favorites.JSON200 == nil {
		return nil, cobra.ShellCompDirectiveError
	}
	completions := make([]cobra.Completion, len(favorites.JSON200.Items))
	for i, favorite := range favorites.JSON200.Items {
		completions[i] = favorite.Symbol
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
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
			c.server = saved.Server
			client, err := newClient(saved)
			if err != nil {
				return err
			}
			user, err := client.GetCurrentUserWithResponse(command.Context())
			if err == nil {
				err = c.check(user, user.JSON200 != nil)
			}
			if err != nil {
				return err
			}
			if !user.JSON200.Administrator {
				return failure("administrator_required", "the token does not belong to the scanner administrator")
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
			if c.json {
				return printJSON(command, map[string]string{"name": args[0], "server": saved.Server, "current": loaded.Current})
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
// a network in cleartext. The API lives at the root of the server, so a path
// would only lead to 404s.
func checkServer(server string) error {
	parsed, err := url.Parse(server)
	if err != nil || parsed.Host == "" {
		return usageError("--server %q is not a URL such as https://scanner.example", server)
	}
	if strings.Trim(parsed.Path, "/") != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return usageError("--server %q must be the server's root URL, such as https://scanner.example, without a path", server)
	}
	host := parsed.Hostname()
	ip := net.ParseIP(host)
	if parsed.Scheme == "https" || parsed.Scheme == "http" && (host == "localhost" || ip != nil && ip.IsLoopback()) {
		return nil
	}
	return usageError("--server must use https; http is allowed only for localhost")
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
		return "", failure("invalid_token", "read the token: %v", err)
	}
	token := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(token, "cst_") || strings.ContainsAny(token, " \t\r\n") {
		return "", failure("invalid_token", "standard input is not an API token (cst_…); create one in the Mini App settings")
	}
	return token, nil
}

// profileEntry is a profile in the --json output of profiles, never with
// its token.
type profileEntry struct {
	Name   string `json:"name"`
	Server string `json:"server"`
}

func (c *cli) profilesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "profiles",
		Short: "List profiles; * marks the current one",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			_, loaded, err := readConfig()
			if err != nil {
				return err
			}
			if c.json {
				entries := []profileEntry{}
				for _, name := range slices.Sorted(maps.Keys(loaded.Profiles)) {
					entries = append(entries, profileEntry{name, loaded.Profiles[name].Server})
				}
				return printJSON(command, struct {
					Current  string         `json:"current"`
					Profiles []profileEntry `json:"profiles"`
				}{loaded.Current, entries})
			}
			for _, name := range slices.Sorted(maps.Keys(loaded.Profiles)) {
				marker := " "
				if name == loaded.Current {
					marker = "*"
				}
				command.Printf("%s %s %s\n", marker, name, loaded.Profiles[name].Server)
			}
			if len(loaded.Profiles) == 0 {
				command.Println("no profiles; run scanner login NAME --server URL")
			}
			return nil
		},
	}
}

func (c *cli) useCommand() *cobra.Command {
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
				return noProfile(args[0])
			}
			loaded.Current = args[0]
			if err := saveConfig(path, loaded); err != nil {
				return err
			}
			if c.json {
				return printJSON(command, map[string]string{"current": loaded.Current})
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
		return nil, noProfile(name)
	}
	c.profile, c.server = name, selected.Server
	return newClient(selected)
}

// noProfile tells how to create the profile name, or to pick one when name
// is empty.
func noProfile(name string) error {
	if name == "" {
		return failure("no_profile", "no profile selected; run scanner login NAME --server URL, or pass --profile")
	}
	return failure("no_profile", "no profile %q; see scanner profiles, or run scanner login %s --server URL", name, name)
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
