package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/Oktanetflow/sshm/internal/config"
	"github.com/Oktanetflow/sshm/internal/credstore"
	"github.com/Oktanetflow/sshm/internal/importer"
	"github.com/Oktanetflow/sshm/internal/sshclient"
	"github.com/Oktanetflow/sshm/internal/termwin"
	"github.com/Oktanetflow/sshm/internal/tui"
	"github.com/spf13/cobra"
)

var (
	flagConfigDir string
	flagLocalFwd  []string
	flagRemoteFwd []string
	flagDynFwd    []string
)

// Version is overridden at build time via -ldflags.
var Version = "1.0.0"

// Execute runs the root CLI command.
func Execute() error {
	tui.AppVersion = Version
	root := newRootCmd()
	return root.Execute()
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:               "sshm [alias | user@host]",
		Short:             "Minimalist CLI/TUI SSH client with first-class Windows support",
		Long:              "sshm — Minimalist, fast CLI & TUI SSH client for Windows with server grouping and secure default password saving.\n" + config.AppCopyright,
		Version:           Version,
		Args:              cobra.MaximumNArgs(1),
		SilenceUsage:      true,
		ValidArgsFunction: completeHostAliases,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, store, err := loadRuntime()
			if err != nil {
				return err
			}
			if len(args) == 0 {
				lastStatus := ""
				for {
					res, err := tui.Run(mgr, store, lastStatus)
					if err != nil {
						return err
					}
					if res == nil || res.ConnectAlias == "" {
						return nil
					}
					err = connectByTarget(cmd.Context(), mgr, store, res.ConnectAlias, flagLocalFwd, flagRemoteFwd, flagDynFwd)
					if err != nil {
						fmt.Fprintf(os.Stderr, "\nConnection failed: %v\n", err)
						if termwin.IsTerminal() {
							fmt.Fprint(os.Stderr, "Press Enter to return to sshm...")
							_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
						}
						lastStatus = fmt.Sprintf("Failed connecting to %s: %v", res.ConnectAlias, err)
						continue
					}
					return nil
				}
			}
			return connectByTarget(cmd.Context(), mgr, store, args[0], flagLocalFwd, flagRemoteFwd, flagDynFwd)
		},
	}

	root.SetVersionTemplate(fmt.Sprintf("sshm version {{.Version}}\n%s\n", config.AppCopyright))

	root.PersistentFlags().StringVar(&flagConfigDir, "config-dir", "", "Override config directory (default: %APPDATA%\\sshm)")
	root.Flags().StringArrayVarP(&flagLocalFwd, "local-forward", "L", nil, "Local port forward [bind:]port:host:port")
	root.Flags().StringArrayVarP(&flagRemoteFwd, "remote-forward", "R", nil, "Remote port forward [bind:]port:host:port")
	root.Flags().StringArrayVarP(&flagDynFwd, "dynamic-forward", "D", nil, "Dynamic SOCKS5 proxy [bind:]port")

	root.AddCommand(
		newConnectCmd(),
		newListCmd(),
		newAddCmd(),
		newEditCmd(),
		newRemoveCmd(),
		newGroupCmd(),
		newExecCmd(),
		newCopyCmd(),
		newImportCmd(),
		newExportCmd(),
		newHostKeyCmd(),
		newConfigCmd(),
	)

	return root
}

func loadRuntime() (*config.Manager, credstore.Store, error) {
	mgr, err := config.Load(flagConfigDir)
	if err != nil {
		return nil, nil, err
	}
	store, err := credstore.New(mgr.Config.General.CredentialBackend, mgr.Dir, func() ([]byte, error) {
		return promptSecret("Vault passphrase: ")
	})
	if err != nil {
		return nil, nil, err
	}
	return mgr, store, nil
}

func promptSecret(prompt string) ([]byte, error) {
	fmt.Fprint(os.Stderr, prompt)
	if termwin.IsTerminal() {
		pw, err := termwin.ReadPassword()
		fmt.Fprintln(os.Stderr)
		return pw, err
	}
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return []byte(strings.TrimRight(line, "\r\n")), nil
}

func promptLine(prompt, defaultVal string) (string, error) {
	if defaultVal != "" {
		fmt.Fprintf(os.Stderr, "%s [%s]: ", prompt, defaultVal)
	} else {
		fmt.Fprintf(os.Stderr, "%s: ", prompt)
	}
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	val := strings.TrimSpace(line)
	if val == "" {
		return defaultVal, nil
	}
	return val, nil
}

func completeHostAliases(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	mgr, err := config.Load(flagConfigDir)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var out []string
	for _, h := range mgr.HostsFile.Hosts {
		if strings.HasPrefix(strings.ToLower(h.Alias), strings.ToLower(toComplete)) {
			out = append(out, fmt.Sprintf("%s\t%s@%s [%s]", h.Alias, h.User, h.Hostname, h.Group))
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

func completeAliasesOrGroups(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	mgr, err := config.Load(flagConfigDir)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var out []string
	for _, g := range mgr.AllGroupNames() {
		tok := "@" + g
		if strings.HasPrefix(strings.ToLower(tok), strings.ToLower(toComplete)) {
			out = append(out, tok+"\tGroup")
		}
	}
	for _, h := range mgr.HostsFile.Hosts {
		if strings.HasPrefix(strings.ToLower(h.Alias), strings.ToLower(toComplete)) {
			out = append(out, h.Alias+"\tHost")
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

func parseForwardFlags(localSpecs, remoteSpecs, dynSpecs []string) ([]config.PortForward, error) {
	var fwds []config.PortForward
	for _, s := range localSpecs {
		f, err := sshclient.ParseLocalOrRemoteSpec("local", s)
		if err != nil {
			return nil, err
		}
		fwds = append(fwds, f)
	}
	for _, s := range remoteSpecs {
		f, err := sshclient.ParseLocalOrRemoteSpec("remote", s)
		if err != nil {
			return nil, err
		}
		fwds = append(fwds, f)
	}
	for _, s := range dynSpecs {
		f, err := sshclient.ParseDynamicSpec(s)
		if err != nil {
			return nil, err
		}
		fwds = append(fwds, f)
	}
	return fwds, nil
}

func resolveTargetHost(mgr *config.Manager, target string) (config.ResolvedHost, bool) {
	if h, _, ok := mgr.FindHost(target); ok {
		return mgr.ResolveHost(h), true
	}

	// Ad-hoc user@host[:port] or user@[ipv6]:port
	user := ""
	hostPort := target
	if idx := strings.LastIndex(target, "@"); idx >= 0 {
		user = target[:idx]
		hostPort = target[idx+1:]
	}
	hostname := hostPort
	port := 22
	if h, pStr, err := net.SplitHostPort(hostPort); err == nil {
		if p, perr := strconv.Atoi(pStr); perr == nil && p > 0 && p <= 65535 {
			hostname = h
			port = p
		}
	} else if strings.HasPrefix(hostPort, "[") && strings.HasSuffix(hostPort, "]") {
		hostname = strings.TrimSuffix(strings.TrimPrefix(hostPort, "["), "]")
	}

	adhoc := config.Host{
		Hostname:     hostname,
		Port:         port,
		User:         user,
		Group:        mgr.Config.General.DefaultGroup,
		Auth:         "auto",
		SavePassword: config.BoolPtr(mgr.Config.General.SavePasswordByDefault),
	}
	return mgr.ResolveHost(adhoc), false
}

func connectByTarget(ctx context.Context, mgr *config.Manager, store credstore.Store, target string, lFwds, rFwds, dFwds []string) error {
	extraForwards, err := parseForwardFlags(lFwds, rFwds, dFwds)
	if err != nil {
		return err
	}

	resolved, isSaved := resolveTargetHost(mgr, target)
	fmt.Fprintf(os.Stderr, "Connecting to %s@%s:%d...\n", resolved.EffectiveUser, resolved.Hostname, resolved.EffectivePort)

	// For ad-hoc connections with SavePasswordByDefault, optionally prompt to save after authenticating.
	var capturedPW []byte
	promptFn := func(p string) ([]byte, error) {
		sec, err := promptSecret(p)
		if err == nil && !isSaved {
			capturedPW = make([]byte, len(sec))
			copy(capturedPW, sec)
		}
		return sec, err
	}
	defer func() {
		if len(capturedPW) > 0 {
			credstore.ZeroBytes(capturedPW)
		}
	}()

	conn, err := sshclient.Connect(ctx, sshclient.ConnectOptions{
		Manager:       mgr,
		CredStore:     store,
		Host:          resolved,
		PromptSecret:  promptFn,
		ExtraForwards: extraForwards,
	})
	if err != nil {
		return err
	}
	defer conn.Close()

	// Offer quick save for ad-hoc connections if password saving is enabled.
	if !isSaved && termwin.IsTerminal() && mgr.Config.General.SavePasswordByDefault {
		ans, _ := promptLine(fmt.Sprintf("Save host %s@%s to sshm (password saved by default)? [Y/n]", resolved.EffectiveUser, resolved.Hostname), "Y")
		if strings.ToLower(strings.TrimSpace(ans)) == "y" || strings.ToLower(strings.TrimSpace(ans)) == "yes" {
			alias, _ := promptLine("Alias", resolved.Hostname)
			grp, _ := promptLine("Group", mgr.Config.General.DefaultGroup)
			saved, serr := mgr.UpsertHost(config.Host{
				Alias:        alias,
				Group:        grp,
				Hostname:     resolved.Hostname,
				Port:         resolved.EffectivePort,
				User:         resolved.EffectiveUser,
				Auth:         "password",
				SavePassword: config.BoolPtr(true),
			})
			if serr == nil && len(capturedPW) > 0 {
				_ = store.Set(saved.CredentialRef, capturedPW)
				fmt.Fprintf(os.Stderr, "Saved %q (password stored in %s).\n", saved.Alias, store.BackendName())
			}
		}
	}

	return sshclient.StartShell(ctx, conn.Client)
}

func newConnectCmd() *cobra.Command {
	var lFwds, rFwds, dFwds []string
	cmd := &cobra.Command{
		Use:               "connect <alias | user@host>",
		Aliases:           []string{"c", "ssh"},
		Short:             "Connect to a saved host or user@host",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeHostAliases,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, store, err := loadRuntime()
			if err != nil {
				return err
			}
			return connectByTarget(cmd.Context(), mgr, store, args[0], lFwds, rFwds, dFwds)
		},
	}
	cmd.Flags().StringArrayVarP(&lFwds, "local-forward", "L", nil, "Local port forward [bind:]port:host:port")
	cmd.Flags().StringArrayVarP(&rFwds, "remote-forward", "R", nil, "Remote port forward [bind:]port:host:port")
	cmd.Flags().StringArrayVarP(&dFwds, "dynamic-forward", "D", nil, "Dynamic SOCKS5 proxy [bind:]port")
	return cmd
}

func newListCmd() *cobra.Command {
	var group string
	var flat bool
	var asJSON bool

	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List saved SSH hosts grouped by category",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, _, err := loadRuntime()
			if err != nil {
				return err
			}
			hosts := mgr.HostsInGroup(group)
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(hosts)
			}
			if len(hosts) == 0 {
				fmt.Println("No saved hosts found. Run `sshm add` or `sshm` to add one.")
				return nil
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			if flat {
				fmt.Fprintln(tw, "ALIAS\tGROUP\tTARGET\tAUTH\tJUMP")
				for _, h := range hosts {
					fmt.Fprintf(tw, "%s\t%s\t%s@%s:%d\t%s\t%s\n", h.Alias, h.Group, h.EffectiveUser, h.Hostname, h.EffectivePort, h.Auth, h.EffectiveJumpHost)
				}
				return tw.Flush()
			}

			groups := mgr.AllGroupNames()
			if group != "" {
				groups = []string{group}
			}
			for _, g := range groups {
				gh := mgr.HostsInGroup(g)
				if len(gh) == 0 {
					continue
				}
				fmt.Fprintf(tw, "[%s] (%d)\n", g, len(gh))
				for _, h := range gh {
					pin := " "
					if h.Pinned {
						pin = "*"
					}
					jump := ""
					if h.EffectiveJumpHost != "" {
						jump = "via " + h.EffectiveJumpHost
					}
					fmt.Fprintf(tw, "  %s %s\t%s@%s:%d\t[%s]\t%s\n", pin, h.Alias, h.EffectiveUser, h.Hostname, h.EffectivePort, h.Auth, jump)
				}
				fmt.Fprintln(tw)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVarP(&group, "group", "g", "", "Filter by group name")
	cmd.Flags().BoolVar(&flat, "flat", false, "Display flat table without group headers")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output hosts as JSON")
	return cmd
}

func newAddCmd() *cobra.Command {
	var hostName, user, group, keyPath, auth, jump string
	var port int
	var tags []string
	var pin, noSavePassword, passwordStdin bool

	cmd := &cobra.Command{
		Use:   "add [alias]",
		Short: "Add a new SSH host (saves password by default)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, store, err := loadRuntime()
			if err != nil {
				return err
			}

			alias := ""
			if len(args) == 1 {
				alias = args[0]
			}

			interactive := hostName == ""
			if interactive {
				if alias == "" {
					alias, err = promptLine("Alias", "")
					if err != nil {
						return err
					}
				}
				hostName, err = promptLine("Host / IP", "")
				if err != nil {
					return err
				}
				portStr, err := promptLine("Port", "22")
				if err != nil {
					return err
				}
				if p, perr := strconv.Atoi(portStr); perr == nil && p > 0 {
					port = p
				}
				user, err = promptLine("User", "root")
				if err != nil {
					return err
				}
				group, err = promptLine("Group", mgr.Config.General.DefaultGroup)
				if err != nil {
					return err
				}
				if keyPath == "" && auth == "" {
					auth, err = promptLine("Auth (password/key/agent)", "password")
					if err != nil {
						return err
					}
					if strings.EqualFold(auth, "key") {
						keyPath, err = promptLine("Identity file path", "~/.ssh/id_ed25519")
						if err != nil {
							return err
						}
					}
				}
				if jump == "" {
					jump, err = promptLine("Jump host alias (optional)", "")
					if err != nil {
						return err
					}
				}
			}

			if alias == "" {
				alias = hostName
			}
			if group == "" {
				group = mgr.Config.General.DefaultGroup
			}
			if auth == "" {
				if keyPath != "" {
					auth = "key"
				} else {
					auth = "password"
				}
			}

			savePW := mgr.Config.General.SavePasswordByDefault
			if noSavePassword {
				savePW = false
			}

			saved, err := mgr.UpsertHost(config.Host{
				Alias:        alias,
				Group:        group,
				Hostname:     hostName,
				Port:         port,
				User:         user,
				Auth:         auth,
				IdentityFile: keyPath,
				JumpHost:     jump,
				Tags:         tags,
				Pinned:       pin,
				SavePassword: config.BoolPtr(savePW),
			})
			if err != nil {
				return err
			}

			if savePW && (strings.EqualFold(auth, "password") || passwordStdin) {
				var pw []byte
				if passwordStdin {
					raw, rerr := io.ReadAll(os.Stdin)
					if rerr != nil {
						return rerr
					}
					pw = []byte(strings.TrimRight(string(raw), "\r\n"))
				} else if termwin.IsTerminal() {
					pw, err = promptSecret("Password (leave blank to prompt on first connect): ")
					if err != nil {
						return err
					}
				}
				if len(pw) > 0 {
					defer credstore.ZeroBytes(pw)
					if err := store.Set(saved.CredentialRef, pw); err != nil {
						return fmt.Errorf("host saved, but failed saving password: %w", err)
					}
					fmt.Printf("Added host %q in group %q (password saved to %s).\n", saved.Alias, saved.Group, store.BackendName())
					return nil
				}
			}

			fmt.Printf("Added host %q in group %q.\n", saved.Alias, saved.Group)
			return nil
		},
	}

	cmd.Flags().StringVar(&hostName, "host", "", "Hostname or IP address")
	cmd.Flags().IntVarP(&port, "port", "p", 0, "SSH port (inherits group default or 22)")
	cmd.Flags().StringVarP(&user, "user", "u", "", "SSH username")
	cmd.Flags().StringVarP(&group, "group", "g", "", "Group name")
	cmd.Flags().StringVarP(&keyPath, "key", "i", "", "Path to SSH private key")
	cmd.Flags().StringVar(&auth, "auth", "", "Auth method: password, key, agent, auto")
	cmd.Flags().StringVarP(&jump, "jump", "J", "", "Jump host alias")
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "Comma-separated tags")
	cmd.Flags().BoolVar(&pin, "pin", false, "Pin host to top of group")
	cmd.Flags().BoolVar(&noSavePassword, "no-save-password", false, "Disable default password saving for this host")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "Read password from stdin (avoids shell history)")
	return cmd
}

func newEditCmd() *cobra.Command {
	var hostName, user, group, keyPath, auth, jump string
	var port int
	var updatePassword bool

	cmd := &cobra.Command{
		Use:               "edit <alias>",
		Short:             "Edit an existing host profile",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeHostAliases,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, store, err := loadRuntime()
			if err != nil {
				return err
			}
			h, _, ok := mgr.FindHost(args[0])
			if !ok {
				return fmt.Errorf("host %q not found", args[0])
			}

			if !cmd.Flags().Changed("host") && !cmd.Flags().Changed("user") && !cmd.Flags().Changed("port") &&
				!cmd.Flags().Changed("group") && !cmd.Flags().Changed("key") && !cmd.Flags().Changed("auth") &&
				!cmd.Flags().Changed("jump") && !updatePassword {
				// Interactive edit prompts
				h.Hostname, _ = promptLine("Host / IP", h.Hostname)
				pStr, _ := promptLine("Port", strconv.Itoa(h.Port))
				if p, perr := strconv.Atoi(pStr); perr == nil && p > 0 {
					h.Port = p
				}
				h.User, _ = promptLine("User", h.User)
				h.Group, _ = promptLine("Group", h.Group)
				h.Auth, _ = promptLine("Auth (password/key/agent)", h.Auth)
				if h.Auth == "key" {
					h.IdentityFile, _ = promptLine("Identity file", h.IdentityFile)
				}
				h.JumpHost, _ = promptLine("Jump host", h.JumpHost)
				ans, _ := promptLine("Update saved password? [y/N]", "N")
				updatePassword = strings.EqualFold(ans, "y") || strings.EqualFold(ans, "yes")
			} else {
				if cmd.Flags().Changed("host") {
					h.Hostname = hostName
				}
				if cmd.Flags().Changed("port") {
					h.Port = port
				}
				if cmd.Flags().Changed("user") {
					h.User = user
				}
				if cmd.Flags().Changed("group") {
					h.Group = group
				}
				if cmd.Flags().Changed("key") {
					h.IdentityFile = keyPath
				}
				if cmd.Flags().Changed("auth") {
					h.Auth = auth
				}
				if cmd.Flags().Changed("jump") {
					h.JumpHost = jump
				}
			}

			saved, err := mgr.UpsertHost(h)
			if err != nil {
				return err
			}
			if updatePassword {
				pw, err := promptSecret("New password: ")
				if err != nil {
					return err
				}
				defer credstore.ZeroBytes(pw)
				if err := store.Set(saved.CredentialRef, pw); err != nil {
					return err
				}
			}
			fmt.Printf("Updated host %q.\n", saved.Alias)
			return nil
		},
	}

	cmd.Flags().StringVar(&hostName, "host", "", "Hostname or IP")
	cmd.Flags().IntVarP(&port, "port", "p", 0, "Port")
	cmd.Flags().StringVarP(&user, "user", "u", "", "Username")
	cmd.Flags().StringVarP(&group, "group", "g", "", "Group name")
	cmd.Flags().StringVarP(&keyPath, "key", "i", "", "Identity file")
	cmd.Flags().StringVar(&auth, "auth", "", "Auth method")
	cmd.Flags().StringVarP(&jump, "jump", "J", "", "Jump host")
	cmd.Flags().BoolVar(&updatePassword, "update-password", false, "Prompt for a new saved password")
	return cmd
}

func newRemoveCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:               "rm <alias>",
		Aliases:           []string{"del", "delete", "remove"},
		Short:             "Delete a saved host and its stored credentials",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeHostAliases,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, store, err := loadRuntime()
			if err != nil {
				return err
			}
			if !force && termwin.IsTerminal() {
				ans, _ := promptLine(fmt.Sprintf("Delete host %q and its saved password? [y/N]", args[0]), "N")
				if !strings.EqualFold(ans, "y") && !strings.EqualFold(ans, "yes") {
					fmt.Println("Cancelled.")
					return nil
				}
			}
			deleted, err := mgr.DeleteHost(args[0])
			if err != nil {
				return err
			}
			if deleted.CredentialRef != "" {
				_ = store.Delete(deleted.CredentialRef)
				_ = store.Delete(credstore.KeyPassphraseRef(deleted.ID))
			}
			fmt.Printf("Deleted host %q.\n", deleted.Alias)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Delete without confirmation")
	return cmd
}

func newGroupCmd() *cobra.Command {
	grpCmd := &cobra.Command{
		Use:   "group",
		Short: "Manage server groups and group defaults",
	}

	lsCmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List all groups",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, _, err := loadRuntime()
			if err != nil {
				return err
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "GROUP\tHOSTS\tCOLOR\tDEFAULT USER\tDEFAULT PORT\tDEFAULT JUMP")
			for _, name := range mgr.AllGroupNames() {
				g := mgr.HostsFile.Groups[name]
				count := len(mgr.HostsInGroup(name))
				portStr := "-"
				if g.DefaultPort > 0 {
					portStr = strconv.Itoa(g.DefaultPort)
				}
				fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\t%s\n", name, count, g.Color, g.DefaultUser, portStr, g.DefaultJump)
			}
			return tw.Flush()
		},
	}

	var color, defUser, defKey, defJump string
	var defPort int
	createCmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create or update a group with optional inherited defaults",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, _, err := loadRuntime()
			if err != nil {
				return err
			}
			err = mgr.CreateOrUpdateGroup(args[0], config.Group{
				Color:               color,
				DefaultUser:         defUser,
				DefaultPort:         defPort,
				DefaultIdentityFile: defKey,
				DefaultJump:         defJump,
			})
			if err != nil {
				return err
			}
			fmt.Printf("Saved group %q.\n", args[0])
			return nil
		},
	}
	createCmd.Flags().StringVar(&color, "color", "", "Badge color (red, yellow, green, blue, cyan, magenta)")
	createCmd.Flags().StringVar(&defUser, "default-user", "", "Default SSH user for hosts in this group")
	createCmd.Flags().IntVar(&defPort, "default-port", 0, "Default SSH port for hosts in this group")
	createCmd.Flags().StringVar(&defKey, "default-key", "", "Default identity file for hosts in this group")
	createCmd.Flags().StringVar(&defJump, "default-jump", "", "Default jump host alias for hosts in this group")

	renameCmd := &cobra.Command{
		Use:   "rename <old> <new>",
		Short: "Rename a group and reassign its hosts",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, _, err := loadRuntime()
			if err != nil {
				return err
			}
			if err := mgr.RenameGroup(args[0], args[1]); err != nil {
				return err
			}
			fmt.Printf("Renamed group %q to %q.\n", args[0], args[1])
			return nil
		},
	}

	var deleteHosts bool
	rmCmd := &cobra.Command{
		Use:     "rm <name>",
		Aliases: []string{"delete"},
		Short:   "Remove a group (moves hosts to default unless --delete-hosts)",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, store, err := loadRuntime()
			if err != nil {
				return err
			}
			removed, err := mgr.DeleteGroup(args[0], deleteHosts)
			if err != nil {
				return err
			}
			for _, h := range removed {
				if h.CredentialRef != "" {
					_ = store.Delete(h.CredentialRef)
				}
				if h.ID != "" {
					_ = store.Delete(credstore.KeyPassphraseRef(h.ID))
				}
			}
			if deleteHosts {
				fmt.Printf("Deleted group %q and %d host(s).\n", args[0], len(removed))
			} else {
				fmt.Printf("Deleted group %q (hosts moved to %q).\n", args[0], mgr.Config.General.DefaultGroup)
			}
			return nil
		},
	}
	rmCmd.Flags().BoolVar(&deleteHosts, "delete-hosts", false, "Also delete all hosts in the group")

	grpCmd.AddCommand(lsCmd, createCmd, renameCmd, rmCmd)
	return grpCmd
}

func newExecCmd() *cobra.Command {
	var parallel bool
	cmd := &cobra.Command{
		Use:               "exec <alias | @group> <command>",
		Short:             "Run a remote command on a single host or an entire @group",
		Args:              cobra.MinimumNArgs(2),
		ValidArgsFunction: completeAliasesOrGroups,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, store, err := loadRuntime()
			if err != nil {
				return err
			}
			target := args[0]
			remoteCmd := strings.Join(args[1:], " ")

			var hosts []config.ResolvedHost
			if strings.HasPrefix(target, "@") {
				hosts = mgr.HostsInGroup(strings.TrimPrefix(target, "@"))
				if len(hosts) == 0 {
					return fmt.Errorf("no hosts found in group %q", strings.TrimPrefix(target, "@"))
				}
			} else {
				r, _ := resolveTargetHost(mgr, target)
				hosts = []config.ResolvedHost{r}
			}

			runOne := func(h config.ResolvedHost, prefix bool) error {
				conn, err := sshclient.Connect(cmd.Context(), sshclient.ConnectOptions{
					Manager:      mgr,
					CredStore:    store,
					Host:         h,
					PromptSecret: promptSecret,
				})
				if err != nil {
					return fmt.Errorf("[%s] %w", h.Alias, err)
				}
				defer conn.Close()

				var outW, errW io.Writer = os.Stdout, os.Stderr
				if prefix {
					outW = &prefixWriter{prefix: fmt.Sprintf("[%s] ", h.Alias), w: os.Stdout}
					errW = &prefixWriter{prefix: fmt.Sprintf("[%s] ", h.Alias), w: os.Stderr}
				}
				return sshclient.Exec(cmd.Context(), conn.Client, remoteCmd, outW, errW)
			}

			multi := len(hosts) > 1
			if !parallel || !multi {
				for _, h := range hosts {
					if err := runOne(h, multi); err != nil {
						fmt.Fprintln(os.Stderr, err)
					}
				}
				return nil
			}

			var wg sync.WaitGroup
			for _, h := range hosts {
				wg.Add(1)
				go func(rh config.ResolvedHost) {
					defer wg.Done()
					if err := runOne(rh, true); err != nil {
						fmt.Fprintln(os.Stderr, err)
					}
				}(h)
			}
			wg.Wait()
			return nil
		},
	}
	cmd.Flags().BoolVar(&parallel, "parallel", false, "Run across group hosts concurrently")
	return cmd
}

type prefixWriter struct {
	mu     sync.Mutex
	prefix string
	w      io.Writer
}

func (p *prefixWriter) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	lines := strings.Split(string(b), "\n")
	for i, line := range lines {
		if i == len(lines)-1 && line == "" {
			continue
		}
		if _, err := fmt.Fprintf(p.w, "%s%s\n", p.prefix, line); err != nil {
			return 0, err
		}
	}
	return len(b), nil
}

func newCopyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cp <src> <dst>",
		Short: "Copy files to or from a saved host over SFTP (e.g. sshm cp ./app.zip prod:/tmp/)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, store, err := loadRuntime()
			if err != nil {
				return err
			}
			srcAlias, srcRemote, srcIsRemote := parseRemotePath(args[0])
			dstAlias, dstRemote, dstIsRemote := parseRemotePath(args[1])

			if srcIsRemote == dstIsRemote {
				return errors.New("exactly one of <src> or <dst> must be a remote path (alias:/path)")
			}

			alias := srcAlias
			if dstIsRemote {
				alias = dstAlias
			}
			resolved, _ := resolveTargetHost(mgr, alias)
			conn, err := sshclient.Connect(cmd.Context(), sshclient.ConnectOptions{
				Manager:      mgr,
				CredStore:    store,
				Host:         resolved,
				PromptSecret: promptSecret,
			})
			if err != nil {
				return err
			}
			defer conn.Close()

			if dstIsRemote {
				n, err := sshclient.UploadFile(conn.Client, args[0], dstRemote)
				if err != nil {
					return err
				}
				fmt.Printf("Uploaded %d bytes -> %s:%s\n", n, alias, dstRemote)
			} else {
				n, err := sshclient.DownloadFile(conn.Client, srcRemote, args[1])
				if err != nil {
					return err
				}
				fmt.Printf("Downloaded %d bytes <- %s:%s\n", n, alias, srcRemote)
			}
			return nil
		},
	}
}

// parseRemotePath distinguishes "alias:/remote/path" from Windows drive paths like "C:\foo\bar".
func parseRemotePath(spec string) (alias, remotePath string, isRemote bool) {
	idx := strings.Index(spec, ":")
	if idx <= 1 {
		// Single letter before ':' is a Windows drive letter (e.g. C:\...)
		return "", "", false
	}
	return spec[:idx], spec[idx+1:], true
}

func newImportCmd() *cobra.Command {
	var sshCfg, putty bool
	var sshCfgPath, backupFile, group string

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import hosts from ~/.ssh/config, PuTTY Registry, or an encrypted backup",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, store, err := loadRuntime()
			if err != nil {
				return err
			}

			if backupFile != "" {
				pass, err := promptSecret("Backup passphrase: ")
				if err != nil {
					return err
				}
				defer credstore.ZeroBytes(pass)
				hCount, cCount, err := importer.ImportEncrypted(mgr, store, pass, backupFile)
				if err != nil {
					return err
				}
				fmt.Printf("Imported %d host(s) and %d credential(s) from %s.\n", hCount, cCount, backupFile)
				return nil
			}

			if !sshCfg && !putty {
				return errors.New("specify one of --ssh-config, --putty, or --backup <file>")
			}

			var imported []config.Host
			if sshCfg {
				hs, err := importer.ImportSSHConfig(sshCfgPath, group)
				if err != nil {
					return err
				}
				imported = append(imported, hs...)
			}
			if putty {
				hs, err := importer.ImportPuTTYSessions(group)
				if err != nil {
					return err
				}
				imported = append(imported, hs...)
			}

			count := 0
			for _, h := range imported {
				if _, err := mgr.UpsertHost(h); err != nil {
					return err
				}
				count++
			}
			fmt.Printf("Imported %d host(s).\n", count)
			return nil
		},
	}

	cmd.Flags().BoolVar(&sshCfg, "ssh-config", false, "Import hosts from OpenSSH config")
	cmd.Flags().StringVar(&sshCfgPath, "ssh-config-path", "~/.ssh/config", "Path to OpenSSH config file")
	cmd.Flags().BoolVar(&putty, "putty", false, "Import sessions from PuTTY Windows Registry")
	cmd.Flags().StringVar(&backupFile, "backup", "", "Import encrypted .sshm.enc backup file")
	cmd.Flags().StringVarP(&group, "group", "g", "", "Target group for imported hosts")
	return cmd
}

func newExportCmd() *cobra.Command {
	var withPasswords bool
	cmd := &cobra.Command{
		Use:   "export <output-file>",
		Short: "Export encrypted backup (.sshm.enc) protected by a passphrase",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, store, err := loadRuntime()
			if err != nil {
				return err
			}
			pass, err := promptSecret("Export passphrase: ")
			if err != nil {
				return err
			}
			defer credstore.ZeroBytes(pass)

			if err := importer.ExportEncrypted(mgr, store, withPasswords, pass, args[0]); err != nil {
				return err
			}
			fmt.Printf("Exported encrypted backup to %s (passwords included: %v).\n", args[0], withPasswords)
			return nil
		},
	}
	cmd.Flags().BoolVar(&withPasswords, "with-passwords", true, "Include saved credentials inside the encrypted backup")
	return cmd
}

func newHostKeyCmd() *cobra.Command {
	hkCmd := &cobra.Command{
		Use:   "hostkey",
		Short: "Manage known_hosts entries",
	}
	rmCmd := &cobra.Command{
		Use:               "remove <alias | hostname>",
		Short:             "Remove a host key from known_hosts",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeHostAliases,
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, _, err := loadRuntime()
			if err != nil {
				return err
			}
			hostname := args[0]
			port := 22
			if h, _, ok := mgr.FindHost(args[0]); ok {
				r := mgr.ResolveHost(h)
				hostname = r.Hostname
				port = r.EffectivePort
			}
			n, err := sshclient.RemoveHostKey(mgr.Config.General.KnownHostsPath, hostname, port)
			if err != nil {
				return err
			}
			fmt.Printf("Removed %d known_hosts entry/entries for %s.\n", n, hostname)
			return nil
		},
	}
	hkCmd.AddCommand(rmCmd)
	return hkCmd
}

func newConfigCmd() *cobra.Command {
	cfgCmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect or update sshm global settings",
	}

	pathCmd := &cobra.Command{
		Use:   "path",
		Short: "Print config directory and file paths",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, _, err := loadRuntime()
			if err != nil {
				return err
			}
			fmt.Printf("Directory : %s\nConfig    : %s\nHosts     : %s\n", mgr.Dir, mgr.ConfigPath(), mgr.HostsPath())
			return nil
		},
	}

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show current config.toml contents",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, _, err := loadRuntime()
			if err != nil {
				return err
			}
			data, err := os.ReadFile(mgr.ConfigPath())
			if err != nil {
				return err
			}
			fmt.Print(string(data))
			return nil
		},
	}

	setCmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Update a config setting (save_password_by_default, credential_backend, theme, strict_host_key_checking, auto_probe_on_start)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, _, err := loadRuntime()
			if err != nil {
				return err
			}
			key, val := strings.ToLower(args[0]), strings.TrimSpace(args[1])
			if err := applyConfigSetting(mgr, key, val); err != nil {
				return err
			}
			if err := mgr.SaveConfig(); err != nil {
				return err
			}
			fmt.Printf("Updated %s = %s\n", key, val)
			return nil
		},
	}

	cfgCmd.AddCommand(pathCmd, showCmd, setCmd)
	return cfgCmd
}

func applyConfigSetting(mgr *config.Manager, key, val string) error {
	switch key {
	case "save_password_by_default":
		b, err := strconv.ParseBool(val)
		if err != nil {
			return fmt.Errorf("invalid boolean for %s: %w", key, err)
		}
		mgr.Config.General.SavePasswordByDefault = b
	case "credential_backend":
		v := strings.ToLower(val)
		switch v {
		case "wincred", "dpapi", "vault":
			mgr.Config.General.CredentialBackend = v
		default:
			return fmt.Errorf("invalid credential_backend %q (must be one of: wincred, dpapi, vault)", val)
		}
	case "default_group":
		if val == "" {
			return errors.New("default_group cannot be empty")
		}
		mgr.Config.General.DefaultGroup = val
	case "strict_host_key_checking":
		v := strings.ToLower(val)
		switch v {
		case "ask", "yes", "no":
			mgr.Config.General.StrictHostKeyChecking = v
		default:
			return fmt.Errorf("invalid strict_host_key_checking %q (must be one of: ask, yes, no)", val)
		}
	case "theme":
		v := strings.ToLower(val)
		switch v {
		case "minimal-dark", "minimal-light", "mono":
			mgr.Config.UI.Theme = v
		default:
			return fmt.Errorf("invalid theme %q (must be one of: minimal-dark, minimal-light, mono)", val)
		}
	case "auto_probe_on_start":
		b, err := strconv.ParseBool(val)
		if err != nil {
			return fmt.Errorf("invalid boolean for %s: %w", key, err)
		}
		mgr.Config.UI.AutoProbeOnStart = b
	case "use_ssh_agent":
		b, err := strconv.ParseBool(val)
		if err != nil {
			return fmt.Errorf("invalid boolean for %s: %w", key, err)
		}
		mgr.Config.SSH.UseSSHAgent = b
	case "keepalive_interval_sec":
		n, err := strconv.Atoi(val)
		if err != nil || n < 0 {
			return fmt.Errorf("keepalive_interval_sec must be a non-negative integer, got %q", val)
		}
		mgr.Config.SSH.KeepaliveIntervalSec = n
	default:
		return fmt.Errorf("unknown config key %q", key)
	}
	return nil
}
