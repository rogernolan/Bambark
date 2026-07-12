#!/bin/sh
set -eu

if [ "$(id -u)" -eq 0 ]; then
	printf '%s\n' 'refusing to run as root; use a normal user with a user systemd session' >&2
	exit 1
fi

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
binary_dir="${HOME}/.local/bin"
config_dir="${HOME}/.config/bambark"
unit_dir="${HOME}/.config/systemd/user"
binary_path="${binary_dir}/bambark"
unit_path="${unit_dir}/bambark.service"
config_path="${config_dir}/bambark.env"
tmp_binary="${binary_path}.tmp.$$"

cleanup() {
	rm -f "$tmp_binary"
}
trap cleanup EXIT INT TERM

mkdir -p "$binary_dir" "$config_dir" "$unit_dir"

printf '%s\n' 'building Bambark'
go build -o "$tmp_binary" "$repo_dir/cmd/bambark"
chmod 0755 "$tmp_binary"
mv "$tmp_binary" "$binary_path"

if [ ! -e "$config_path" ]; then
	cat >&2 <<EOF
No environment file exists at:
  $config_path

Create it from deploy/bambark.env.example, then run this script again.
EOF
	exit 1
fi

install -m 0644 "$repo_dir/deploy/bambark.user.service" "$unit_path"
systemctl --user daemon-reload
systemctl --user enable bambark.service >/dev/null
systemctl --user restart bambark.service

printf 'deployed and restarted %s\n' "$binary_path"

