#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
deploy_script="$script_dir/deploy.sh"
unit_file="$script_dir/../deploy/bambark.user.service"

test -x "$deploy_script"
grep -q 'id -u' "$deploy_script"
grep -q 'systemctl --user daemon-reload' "$deploy_script"
grep -q 'systemctl --user restart bambark.service' "$deploy_script"
test -f "$unit_file"
grep -q '^ExecStart=%h/.local/bin/bambark$' "$unit_file"
grep -q '^EnvironmentFile=%h/.config/bambark/bambark.env$' "$unit_file"

echo "deploy checks passed"
