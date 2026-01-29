#!/bin/sh
set -eu

PRETTY_USER="${PRETTY_USER:-pretty}"
if [ -z "${PRETTY_PASSWORD:-}" ]; then
  echo "PRETTY_PASSWORD is required" >&2
  exit 1
fi

AUTHORIZED_KEYS_PATH="${AUTHORIZED_KEYS_PATH:-/authorized_keys}"

if ! id -u "$PRETTY_USER" >/dev/null 2>&1; then
  useradd -m -s /bin/sh "$PRETTY_USER"
fi

echo "$PRETTY_USER:$PRETTY_PASSWORD" | chpasswd

home_dir="/home/$PRETTY_USER"
ssh_dir="$home_dir/.ssh"
mkdir -p "$ssh_dir"

if [ ! -f "$AUTHORIZED_KEYS_PATH" ]; then
  echo "authorized_keys not found at $AUTHORIZED_KEYS_PATH" >&2
  exit 1
fi

cp "$AUTHORIZED_KEYS_PATH" "$ssh_dir/authorized_keys"
chown -R "$PRETTY_USER:$PRETTY_USER" "$ssh_dir"
chmod 700 "$ssh_dir"
chmod 600 "$ssh_dir/authorized_keys"

ssh-keygen -A

if grep -q "^PasswordAuthentication" /etc/ssh/sshd_config; then
  sed -i 's/^PasswordAuthentication.*/PasswordAuthentication yes/' /etc/ssh/sshd_config
else
  echo "PasswordAuthentication yes" >> /etc/ssh/sshd_config
fi

if grep -q "^PubkeyAuthentication" /etc/ssh/sshd_config; then
  sed -i 's/^PubkeyAuthentication.*/PubkeyAuthentication yes/' /etc/ssh/sshd_config
else
  echo "PubkeyAuthentication yes" >> /etc/ssh/sshd_config
fi

if grep -q "^PermitRootLogin" /etc/ssh/sshd_config; then
  sed -i 's/^PermitRootLogin.*/PermitRootLogin no/' /etc/ssh/sshd_config
else
  echo "PermitRootLogin no" >> /etc/ssh/sshd_config
fi

exec /usr/sbin/sshd -D -e
