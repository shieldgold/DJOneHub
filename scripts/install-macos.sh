#!/bin/sh
# Install the locally built service for the current macOS user.
set -eu
ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
python3 - "${ROOT_DIR}" <<'PY'
import os
from pathlib import Path
import plistlib
import shutil
import subprocess
import sys

source = Path(sys.argv[1]) / 'dist/djonehub-macos'
if not source.is_file():
    raise SystemExit('Build first: sh scripts/build-macos.sh')
home = Path.home()
app = home / 'Library/Application Support/DJOneHub'
logs = home / 'Library/Logs/DJOneHub'
agents = home / 'Library/LaunchAgents'
label = 'local.djonehub.macos'
plist = agents / (label + '.plist')
domain = 'gui/' + str(os.getuid())
for directory in (app, logs, agents):
    directory.mkdir(parents=True, exist_ok=True)
app.chmod(0o700)
logs.chmod(0o700)
if plist.exists():
    subprocess.run(['launchctl', 'bootout', domain, str(plist)], check=False)
binary = app / 'djonehub-macos'
shutil.copy2(source, binary)
binary.chmod(0o755)
config = {
    'Label': label,
    'ProgramArguments': [str(binary), '-listen', '127.0.0.1:7575'],
    'WorkingDirectory': str(app),
    'EnvironmentVariables': {'PATH': '/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin'},
    'RunAtLoad': True,
    'KeepAlive': True,
    'ThrottleInterval': 10,
    'Umask': 0o077,
    'StandardOutPath': str(logs / 'service.log'),
    'StandardErrorPath': str(logs / 'service.log'),
}
with plist.open('wb') as f:
    plistlib.dump(config, f)
plist.chmod(0o600)
subprocess.run(['plutil', '-lint', str(plist)], check=True)
subprocess.run(['launchctl', 'bootstrap', domain, str(plist)], check=True)
print('Installed login service: ' + label)
print('Management page: http://127.0.0.1:7575')
PY
