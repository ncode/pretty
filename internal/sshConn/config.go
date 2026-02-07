package sshConn

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	sshconfig "github.com/ncode/ssh_config"
	"golang.org/x/crypto/ssh"
)

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
	if proxyJump != "" {
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
	ctx := sshconfig.Context{HostArg: alias, OriginalHost: alias, LocalUser: currentUser()}
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

func ParseProxyJump(value string) []string {
	parts := strings.Split(value, ",")
	jumps := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			jumps = append(jumps, trimmed)
		}
	}
	return jumps
}

func LoadIdentityFiles(paths []string) ([]ssh.AuthMethod, error) {
	methods := make([]ssh.AuthMethod, 0, len(paths))
	for _, path := range paths {
		expanded := expandPath(path)
		key, err := os.ReadFile(expanded)
		if err != nil {
			return nil, err
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, err
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	return methods, nil
}
