package sshConn

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	sshconfig "github.com/ncode/ssh_config"
	"golang.org/x/crypto/ssh"
)

// matchExecTimeout bounds how long a `Match ... exec "..."` probe may run.
// OpenSSH itself does not impose a timeout, but pretty resolves many hosts in
// a row (including per-host ProxyJump resolution) so we cap each probe to
// avoid hanging the CLI if a user's exec command never returns.
const matchExecTimeout = 10 * time.Second

// matchExecFunc evaluates `Match ... exec "<cmd>"` directives. It is a package
// variable so tests can substitute a deterministic stub without shelling out.
// It must return (true, nil) if the command exits with status 0, (false, nil)
// otherwise, and only return a non-nil error for conditions the caller should
// surface (currently unused by the resolver because we do not opt into strict
// mode).
var matchExecFunc = shellMatchExec

// shellMatchExec runs cmd via the local shell (sh -c / cmd /C on Windows) and
// reports whether it succeeded. The command has already had its %-tokens
// expanded by ssh_config before this function is invoked.
func shellMatchExec(cmd string) (bool, error) {
	if strings.TrimSpace(cmd) == "" {
		return false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), matchExecTimeout)
	defer cancel()

	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.CommandContext(ctx, "cmd", "/C", cmd)
	} else {
		c = exec.CommandContext(ctx, "/bin/sh", "-c", cmd)
	}
	c.Stdin = nil
	c.Stdout = nil
	c.Stderr = nil

	if err := c.Run(); err != nil {
		// Any non-zero exit or failure to launch counts as "did not match",
		// mirroring OpenSSH's treatment of Match exec probes.
		return false, nil
	}
	return true, nil
}

type SSHConfigPaths struct {
	User   string
	System string
}

type SSHConfigResolver struct {
	user   *sshconfig.Config
	system *sshconfig.Config
}

type HostSpec struct {
	Alias   string
	Host    string
	Port    int
	User    string
	PortSet bool
	UserSet bool
}

type ResolvedHost struct {
	Alias         string
	Host          string
	Port          int
	User          string
	IdentityFiles []string
	ProxyJump     []string
}

func LoadSSHConfig(paths SSHConfigPaths) (*SSHConfigResolver, error) {
	userCfg, err := loadConfig(paths.User)
	if err != nil {
		return nil, err
	}
	systemCfg, err := loadConfig(paths.System)
	if err != nil {
		return nil, err
	}
	return &SSHConfigResolver{user: userCfg, system: systemCfg}, nil
}

func loadConfig(path string) (*sshconfig.Config, error) {
	if path == "" {
		return nil, nil
	}
	expanded := expandPath(path)
	file, err := os.Open(expanded)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()
	return sshconfig.Decode(file)
}

func (r *SSHConfigResolver) ResolveHost(spec HostSpec, fallbackUser string) (ResolvedHost, error) {
	alias := spec.Host
	if spec.Alias != "" {
		alias = spec.Alias
	}
	resolved := ResolvedHost{Alias: alias, Host: alias}

	hostName, err := r.getValue(alias, "HostName")
	if err != nil {
		return ResolvedHost{}, err
	}
	if hostName != "" {
		resolved.Host = hostName
	}

	userValue := spec.User
	userSet := spec.UserSet
	if !userSet {
		userValue, err = r.getValue(alias, "User")
		if err != nil {
			return ResolvedHost{}, err
		}
		if userValue != "" {
			userSet = true
		}
	}
	if !userSet {
		if fallbackUser != "" {
			userValue = fallbackUser
		} else {
			userValue = currentUser()
		}
	}
	resolved.User = userValue

	portValue := spec.Port
	portSet := spec.PortSet
	if !portSet {
		portStr, err := r.getValue(alias, "Port")
		if err != nil {
			return ResolvedHost{}, err
		}
		if portStr != "" {
			portValue, err = strconv.Atoi(portStr)
			if err != nil {
				return ResolvedHost{}, fmt.Errorf("invalid port %q: %w", portStr, err)
			}
			portSet = true
		}
	}
	if !portSet {
		portValue = 22
	}
	resolved.Port = portValue

	identityFiles, err := r.getAllValues(alias, "IdentityFile")
	if err != nil {
		return ResolvedHost{}, err
	}
	resolved.IdentityFiles = identityFiles

	proxyJump, err := r.getValue(alias, "ProxyJump")
	if err != nil {
		return ResolvedHost{}, err
	}
	// OpenSSH treats `ProxyJump none` as an explicit opt-out that cancels
	// ProxyJump inherited from broader-matching blocks. Skip parsing in that
	// case so we don't try to dial the literal host "none".
	if proxyJump != "" && !strings.EqualFold(strings.TrimSpace(proxyJump), "none") {
		resolved.ProxyJump = ParseProxyJump(proxyJump)
	}

	return resolved, nil
}

func (r *SSHConfigResolver) getValue(alias, key string) (string, error) {
	if r.user != nil {
		result, err := r.resolve(r.user, alias)
		if err != nil {
			return "", err
		}
		val := result.Get(key)
		if strings.TrimSpace(val) != "" {
			return val, nil
		}
	}
	if r.system != nil {
		result, err := r.resolve(r.system, alias)
		if err != nil {
			return "", err
		}
		val := result.Get(key)
		if strings.TrimSpace(val) != "" {
			return val, nil
		}
	}
	return "", nil
}

func (r *SSHConfigResolver) getAllValues(alias, key string) ([]string, error) {
	values := []string{}
	if r.user != nil {
		result, err := r.resolve(r.user, alias)
		if err != nil {
			return nil, err
		}
		vals := result.GetAll(key)
		for _, val := range vals {
			if strings.TrimSpace(val) == "" {
				continue
			}
			values = append(values, val)
		}
	}
	if r.system != nil {
		result, err := r.resolve(r.system, alias)
		if err != nil {
			return nil, err
		}
		vals := result.GetAll(key)
		for _, val := range vals {
			if strings.TrimSpace(val) == "" {
				continue
			}
			values = append(values, val)
		}
	}
	return values, nil
}

func (r *SSHConfigResolver) resolve(cfg *sshconfig.Config, alias string) (*sshconfig.Result, error) {
	if cfg == nil {
		return nil, nil
	}
	ctx := sshconfig.Context{
		HostArg:      alias,
		OriginalHost: alias,
		LocalUser:    currentUser(),
		// Providing Exec enables `Match host X exec "..."` blocks to be
		// evaluated. Without it the library silently treats every exec
		// predicate as non-matching, which causes directives like
		//     Match host jump-alias exec "nc -zG 1 primary.example.net 22"
		//         HostName primary.example.net
		// to be skipped, leaving the alias unresolvable via DNS.
		Exec: matchExecFunc,
	}
	return cfg.Resolve(ctx)
}

func currentUser() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	if env := os.Getenv("USER"); env != "" {
		return env
	}
	return os.Getenv("LOGNAME")
}

func expandPath(path string) string {
	if path == "" {
		return path
	}
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// ParseProxyJump splits a ProxyJump value into individual jump hosts. Empty
// components are dropped, and the OpenSSH "none" sentinel (which disables
// ProxyJump) collapses the result to an empty slice so callers never try to
// dial a literal host named "none".
func ParseProxyJump(value string) []string {
	parts := strings.Split(value, ",")
	jumps := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if strings.EqualFold(trimmed, "none") {
			return nil
		}
		jumps = append(jumps, trimmed)
	}
	return jumps
}

// LoadIdentityFiles returns SSH auth methods for every identity file that
// contains a usable private key locally.
//
// IdentityFile entries that the local process cannot use directly (missing
// files, public-key-only files backing a hardware token such as yubikey-agent,
// or passphrase-protected keys) are skipped so that authentication can still
// proceed through the SSH agent. This mirrors OpenSSH's behaviour, which
// silently tolerates these cases instead of aborting the connection.
func LoadIdentityFiles(paths []string) ([]ssh.AuthMethod, error) {
	methods := make([]ssh.AuthMethod, 0, len(paths))
	for _, path := range paths {
		expanded := expandPath(path)
		key, err := os.ReadFile(expanded)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("unable to read identity file %q: %w", expanded, err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			if isAgentCoveredIdentity(key, err) {
				continue
			}
			return nil, fmt.Errorf("unable to parse identity file %q: %w", expanded, err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	return methods, nil
}

// isAgentCoveredIdentity reports whether an identity file that we failed to
// parse as a private key should be delegated to the SSH agent. Typical cases:
//
//   - The file stores only a public key (e.g. a hardware-token pub key in
//     `~/.ssh/...`) while the private half lives on the token itself and is
//     exposed exclusively through a signing agent.
//   - The key is encrypted and no passphrase is available to this process.
func isAgentCoveredIdentity(data []byte, parseErr error) bool {
	if _, ok := parseErr.(*ssh.PassphraseMissingError); ok {
		return true
	}
	if _, _, _, _, err := ssh.ParseAuthorizedKey(data); err == nil {
		return true
	}
	return false
}
