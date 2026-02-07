[![Go Report Card](https://goreportcard.com/badge/github.com/ncode/pretty)](https://goreportcard.com/report/github.com/ncode/pretty)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![codecov](https://codecov.io/gh/ncode/pretty/graph/badge.svg?token=BCUQ77HCLY)](https://codecov.io/gh/ncode/pretty)

# pretty

`Parallel remote execution tty` - (Yet another parallel ssh/shell)

- Run commands across many hosts with colored, prefixed output.
- Keep an interactive prompt with a per-host shell session.
- Run async jobs in separate SSH sessions and track status.
- Load hosts from args, config groups, or a hosts file.

## Installation
Requires Go 1.25.

```
go install github.com/ncode/pretty@latest
```

## Quick start
```
pretty host1 host2 host3
pretty -G prod
pretty -H /tmp/hosts.txt
```

## Configuration
`pretty` looks for a config file named `.pretty` in your home directory with a supported extension:
`$HOME/.pretty.yaml`, `$HOME/.pretty.yml`, `$HOME/.pretty.json`, or `$HOME/.pretty.toml`.
Use `--config` to point at an explicit path.

Optional keys:
- `username`: SSH username override (falls back to SSH config, then current shell user).
- `known_hosts`: path to a known_hosts file for host key verification.
- `groups.<name>`: host groups as wrapper objects with `hosts` and optional `user`.
- `prompt`: interactive prompt string (UTF-8 supported). `--prompt` overrides config.

Example:
```
known_hosts: /Users/me/.ssh/known_hosts
prompt: "pretty> "
groups:
  web:
    user: deploy
    hosts:
      - web1.example.com
      - web2.example.com:2222
```

Host key verification:
- If `known_hosts` is set and loads successfully, it is used.
- Otherwise `~/.ssh/known_hosts` is used if it loads successfully.
- If neither can be loaded, host keys are not verified.
- A loaded known_hosts file must contain each host key or connections will fail.

Notes:
- Group entries must use the wrapper schema with a `hosts` list.
- Auth uses your SSH agent (`SSH_AUTH_SOCK`) and IdentityFile entries from SSH config. Load keys with `ssh-add`.
- Host resolution follows OpenSSH-style `Host` and `Match` evaluation from your SSH config.

## Host specs
Accepted formats:
- `host` (defaults to port 22)
- `host:port`
- `user@host`
- `user@host:port`
- `[ipv6]:port` (required to specify a port with IPv6)
- `user@[ipv6]:port`

Hosts files (`-H`) accept one entry per line in the same formats. Blank lines are ignored.

## Flags
- `--config <path>`: config file path.
- `--prompt <string>`: prompt to display in the interactive shell.
- `-G`, `--hostGroup <name>`: load `groups.<name>` from config.
- `-H`, `--hostsFile <path>`: read hosts from a file (one host per line).
- `-h`, `--help`: help for pretty.

Host selection behavior:
- At least one of positional hosts, `--hostGroup`, or `--hostsFile` is required.
- With no positional hosts, `--hostGroup` loads only the group.
- With more than one positional host, `--hostGroup` appends the group.
- With exactly one positional host, `--hostGroup` is currently ignored.
- `--hostsFile` always appends its hosts.

## Interactive commands
```
:help
:list
:status [id]
:async <command>
:scroll
:bye
exit
```

Notes:
- `:list` shows connection status per host.
- `:status` shows the last normal job plus the last two async jobs; `:status <id>` targets a single job.
- `:async` runs a command in a new SSH session per host and returns to the prompt immediately.
- `:scroll` enters scroll mode for the output viewport (output scrolling is disabled otherwise); press `esc` to return to the prompt.
- Use Up/Down arrows to navigate command history (persisted in `history_file`).
- `Ctrl+C` forwards to remote sessions; press twice within 500ms to quit locally.
- `Ctrl+Z` forwards to remote sessions (suspend).

## How it works
- Starts one persistent SSH shell session per host for interactive commands.
- Wraps each command with a sentinel to capture per-host exit codes.
- Runs async commands in fresh SSH sessions and updates job status as they finish.
- Prefixes output with `host:port` and assigns a stable color per host.
- Keeps the last 10,000 output lines in the UI buffer.

## Local SSHD testbed
Use the local SSHD testbed to exercise `pretty` against three localhost targets.

Generate keys, password, and a ready-to-use config file:
```
export PRETTY_AUTHORIZED_KEY="$(ssh-add -L | grep 'my-key' | head -n1)"
./scripts/ssh-testbed-setup.sh
```

Start the testbed:
```
docker compose -f docker-compose.sshd.yml up -d --build
```

Re-run setup after the containers are running to populate `.pretty-test/known_hosts`:
```
./scripts/ssh-testbed-setup.sh
```

If you want to use the generated test key instead of an existing agent key:
```
ssh-add .pretty-test/id_ed25519
```

Example run:
```
pretty --config .pretty-test/pretty.yaml -G testbed
```
Then run `whoami` to confirm each host responds as `pretty`.

The generated password is stored at `.pretty-test/password.txt` for manual `ssh` testing if needed.

## Why pretty?
`pretty` is a tool to control interactive shells across multiple hosts from a single point.

### Motivation
After using [polysh](http://guichaz.free.fr/polysh) for a long time, it came with
the motivation to try to write my own parallel shell in Go. In the end the tool worked
so well and I decided to open source the code.

## Limitations
- SSH authentication uses the local agent and SSH config IdentityFile entries; there is no keyfile flag.
