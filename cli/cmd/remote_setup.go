package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/provider"
	remotesvc "github.com/datichb/openhub/cli/internal/services/remote"
)

func init() {
	remoteCmd.AddCommand(remoteSetupCmd())
}

func remoteSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: i18n.T("cmd.remote.setup.short"),
		Long:  i18n.T("cmd.remote.setup.long"),
		Args:  cobra.NoArgs,
		RunE:  runRemoteSetup,
	}
	f := cmd.Flags()
	f.String("name", "", i18n.T("cmd.remote.setup.flags.name"))
	f.String("url", "", i18n.T("cmd.remote.setup.flags.url"))
	f.String("group", "", i18n.T("cmd.remote.setup.flags.group"))
	f.String("runner-project", "", i18n.T("cmd.remote.setup.flags.runner_project"))
	f.String("tag", "", i18n.T("cmd.remote.setup.flags.tag"))
	f.String("builder", "", i18n.T("cmd.remote.setup.flags.builder"))
	f.String("arch", "", i18n.T("cmd.remote.setup.flags.arch"))
	f.String("timeout", "", i18n.T("cmd.remote.setup.flags.timeout"))
	f.String("token-key", "", i18n.T("cmd.remote.setup.flags.token_key"))
	f.String("token-env", "", i18n.T("cmd.remote.setup.flags.token_env"))
	f.Bool("no-create", false, i18n.T("cmd.remote.setup.flags.no_create"))
	f.Bool("force", false, i18n.T("cmd.remote.setup.flags.force"))
	f.Bool("llm", false, i18n.T("cmd.remote.setup.flags.llm"))
	f.String("llm-provider", "", i18n.T("cmd.remote.setup.flags.llm_provider"))
	f.String("llm-region", "", i18n.T("cmd.remote.setup.flags.llm_region"))
	f.String("llm-key-env", "", i18n.T("cmd.remote.setup.flags.llm_key_env"))
	f.StringArray("project", nil, i18n.T("cmd.remote.setup.flags.project"))
	f.String("project-token-env", "", i18n.T("cmd.remote.setup.flags.project_token_env"))
	f.Bool("teamstate", false, i18n.T("cmd.remote.setup.flags.teamstate"))
	f.String("teamstate-token-env", "", i18n.T("cmd.remote.setup.flags.teamstate_token_env"))
	f.String("oh-binary", "", i18n.T("cmd.remote.setup.flags.oh_binary"))
	return cmd
}

// remoteSetupInput reads values and secrets: flags and environment first,
// then the terminal (hidden input for secrets).
type remoteSetupInput struct {
	cmd *cobra.Command
	in  *bufio.Reader
	tty bool
}

func (r *remoteSetupInput) text(label, def string) (string, error) {
	if !r.tty {
		if def == "" {
			return "", errors.New(i18n.Tf("cmd.remote.setup.missing", label))
		}
		return def, nil
	}
	if def != "" {
		fmt.Fprintf(r.cmd.OutOrStdout(), "%s [%s] : ", label, def)
	} else {
		fmt.Fprintf(r.cmd.OutOrStdout(), "%s : ", label)
	}
	line, err := r.in.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	if v := strings.TrimSpace(line); v != "" {
		return v, nil
	}
	if def == "" {
		return "", errors.New(i18n.Tf("cmd.remote.setup.missing", label))
	}
	return def, nil
}

// secret returns the value of the environment variable envFlag names, else
// asks on the terminal.
func (r *remoteSetupInput) secret(envFlag, label string) (string, error) {
	if name, _ := r.cmd.Flags().GetString(envFlag); name != "" {
		v := strings.TrimSpace(os.Getenv(name))
		if v == "" {
			return "", errors.New(i18n.Tf("cmd.remote.setup.env_empty", name))
		}
		return v, nil
	}
	if !r.tty {
		return "", errors.New(i18n.Tf("cmd.remote.setup.secret_no_tty", label, "--"+envFlag))
	}
	fmt.Fprintf(r.cmd.OutOrStdout(), "%s : ", label)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(r.cmd.OutOrStdout())
	if err != nil {
		return "", err
	}
	v := strings.TrimSpace(string(b))
	if v == "" {
		return "", errors.New(i18n.Tf("cmd.remote.setup.missing", label))
	}
	return v, nil
}

var targetNameCleanRe = regexp.MustCompile(`[^a-z0-9-]+`)

// remoteTargetName derives a target name from a group path.
func remoteTargetName(group string) string {
	g := strings.ToLower(strings.Trim(group, "/"))
	g = targetNameCleanRe.ReplaceAllString(strings.ReplaceAll(g, "/", "-"), "-")
	g = strings.Trim(g, "-")
	if len(g) > 63 {
		g = strings.Trim(g[:63], "-")
	}
	if g == "" {
		return "default"
	}
	return g
}

// remoteDefaultsFromGit proposes the instance and group of the git remote of
// the current directory (origin).
func remoteDefaultsFromGit() (url, group string) {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return "", ""
	}
	gr, err := config.ParseGitRemote(string(out))
	if err != nil {
		return "", ""
	}
	host := gr.Host
	if i := strings.LastIndexByte(gr.Path, '/'); i > 0 {
		group = gr.Path[:i]
	}
	return "https://" + host, group
}

func runRemoteSetup(cmd *cobra.Command, _ []string) error {
	a := MustApp()
	ctx := ctxOf(cmd)
	out := cmd.OutOrStdout()
	f := cmd.Flags()
	in := &remoteSetupInput{cmd: cmd, in: bufio.NewReader(os.Stdin), tty: interactive()}
	str := func(name string) string { v, _ := f.GetString(name); return strings.TrimSpace(v) }

	// Existing target: by name, else the only one, else none.
	var t config.RemoteTarget
	name := str("name")
	if name != "" {
		if cur := a.Config.Remote.Target(name); cur != nil {
			t = *cur
		}
	} else if len(a.Config.Remote.Targets) == 1 && str("url") == "" && str("group") == "" {
		t = a.Config.Remote.Targets[0]
	}
	gitURL, gitGroup := remoteDefaultsFromGit()
	pick := func(flag, cur, detected string) string {
		if v := str(flag); v != "" {
			return v
		}
		if cur != "" {
			return cur
		}
		return detected
	}
	var err error
	if t.URL, err = in.text(i18n.T("cmd.remote.setup.ask.url"), pick("url", t.URL, firstNonEmpty(gitURL, "https://gitlab.com"))); err != nil {
		return err
	}
	if t.Group, err = in.text(i18n.T("cmd.remote.setup.ask.group"), pick("group", t.Group, gitGroup)); err != nil {
		return err
	}
	t.Group = strings.Trim(t.Group, "/")
	if t.Name == "" {
		t.Name = firstNonEmpty(name, remoteTargetName(t.Group))
	}
	if v := str("runner-project"); v != "" {
		t.RunnerProject = v
	}
	for flag, dst := range map[string]*string{"tag": &t.Tag, "builder": &t.Builder, "arch": &t.Arch, "timeout": &t.Timeout, "token-key": &t.TokenKey} {
		if v := str(flag); v != "" {
			*dst = v
		}
	}
	if err := t.Validate(); err != nil {
		return err
	}

	req := remotesvc.SetupRequest{Target: t, Force: mustBool(f, "force"), OhBinary: str("oh-binary")}
	req.Create = !mustBool(f, "no-create")

	// API token: environment, else the keychain, else the terminal.
	if str("token-env") != "" {
		if req.Token, err = in.secret("token-env", i18n.T("cmd.remote.setup.ask.token")); err != nil {
			return err
		}
	} else if a.Secrets != nil {
		if cur, _ := a.Secrets.Get(ctx, t.TokenKeyOrDefault()); cur == "" {
			fmt.Fprintln(out, i18n.Tf("cmd.remote.setup.token_help", t.URL))
			if req.Token, err = in.secret("token-env", i18n.T("cmd.remote.setup.ask.token")); err != nil {
				return err
			}
		}
	}

	if mustBool(f, "llm") || str("llm-key-env") != "" {
		prov := firstNonEmpty(str("llm-provider"), string(provider.Bedrock))
		switch provider.Name(prov) {
		case provider.Bedrock, provider.Anthropic, provider.OpenRouter:
		default:
			return errors.New(i18n.Tf("cmd.remote.setup.llm_provider_unknown", prov))
		}
		region := str("llm-region")
		if region == "" && prov == string(provider.Bedrock) {
			region = "eu-west-1"
		}
		key, err := in.secret("llm-key-env", i18n.Tf("cmd.remote.setup.ask.llm_key", prov))
		if err != nil {
			return err
		}
		req.LLM = &remotesvc.LLMSecret{Provider: prov, Region: region, Key: key}
	}

	projects, _ := f.GetStringArray("project")
	if len(projects) > 1 && str("project-token-env") != "" {
		return errors.New(i18n.T("cmd.remote.setup.project_env_single"))
	}
	for _, p := range projects {
		tok, err := in.secret("project-token-env", i18n.Tf("cmd.remote.setup.ask.project_token", p))
		if err != nil {
			return err
		}
		req.Projects = append(req.Projects, remotesvc.ProjectToken{Path: strings.Trim(p, "/"), Token: tok})
	}
	if mustBool(f, "teamstate") || str("teamstate-token-env") != "" {
		if req.TeamStateToken, err = in.secret("teamstate-token-env", i18n.T("cmd.remote.setup.ask.teamstate_token")); err != nil {
			return err
		}
	}

	fmt.Fprintln(out, i18n.Tf("cmd.remote.setup.running", t.RunnerProjectPath(), t.URL))
	rep, err := newRemoteService(a).Setup(ctx, req)
	if rep != nil {
		printRemoteReport(out, rep)
	}
	if err != nil {
		return err
	}
	if !rep.OK() {
		return errors.New(i18n.T("cmd.remote.setup.incomplete"))
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, i18n.Tf("cmd.remote.setup.done", rep.Target.Name))
	return nil
}

func mustBool(f interface{ GetBool(string) (bool, error) }, name string) bool {
	v, _ := f.GetBool(name)
	return v
}
