#!/bin/sh
set -eu

root_dir=$(cd "$(dirname "$0")/.." && pwd)
work_dir="$root_dir/.pretty-test"
key_path="$work_dir/id_ed25519"

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required tool: $1" >&2
    exit 1
  fi
}

need_cmd ssh-keygen
need_cmd ssh-keyscan

mkdir -p "$work_dir"

if [ ! -f "$key_path" ]; then
  ssh-keygen -t ed25519 -N "" -f "$key_path"
fi

if [ -n "${PRETTY_AUTHORIZED_KEY:-}" ]; then
  printf "%s\n" "$PRETTY_AUTHORIZED_KEY" > "$work_dir/authorized_keys"
else
  cp "$key_path.pub" "$work_dir/authorized_keys"
fi
chmod 600 "$key_path" "$work_dir/authorized_keys"

if [ ! -f "$work_dir/password.txt" ]; then
  if command -v openssl >/dev/null 2>&1; then
    password=$(openssl rand -base64 18)
  elif command -v python3 >/dev/null 2>&1; then
    password=$(python3 - <<'PY'
import secrets, string
alphabet = string.ascii_letters + string.digits
print(''.join(secrets.choice(alphabet) for _ in range(24)))
PY
)
  else
    echo "missing openssl or python3 for password generation" >&2
    exit 1
  fi
  printf "%s\n" "$password" > "$work_dir/password.txt"
  chmod 600 "$work_dir/password.txt"
fi

password=$(cat "$work_dir/password.txt")
cat > "$work_dir/sshd.env" <<EOF
PRETTY_USER=pretty
PRETTY_PASSWORD=$password
EOF

: > "$work_dir/known_hosts"
ssh-keyscan -p 2221 -H localhost 2>/dev/null >> "$work_dir/known_hosts" || true
ssh-keyscan -p 2222 -H localhost 2>/dev/null >> "$work_dir/known_hosts" || true
ssh-keyscan -p 2223 -H localhost 2>/dev/null >> "$work_dir/known_hosts" || true

cat > "$work_dir/pretty.yaml" <<EOF
known_hosts: $work_dir/known_hosts
groups:
  testbed:
    user: pretty
    hosts:
      - localhost:2221
      - localhost:2222
      - localhost:2223
EOF

if [ ! -s "$work_dir/known_hosts" ]; then
  echo "warning: known_hosts is empty; start the testbed and re-run to populate it" >&2
fi
