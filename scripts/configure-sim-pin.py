#!/usr/bin/env python3
"""Store a PIN bound to the connected SIM in the macOS login keychain.

The PIN is read without terminal echo and passed to security over stdin, never
in argv, a configuration file, or Git. This does not submit a PIN to the modem.
"""
import getpass
import json
import re
import shlex
import subprocess
import urllib.request


def main():
    request = urllib.request.Request(
        'http://127.0.0.1:7575/api/at',
        json.dumps({'command': 'AT+QCCID'}).encode(),
        {'Content-Type': 'application/json'},
    )
    with urllib.request.urlopen(request, timeout=30) as response:
        result = json.load(response).get('response', '')
    match = re.search(r'\+QCCID:\s*([0-9]{18,22})(?![0-9])', result)
    if not match:
        raise SystemExit('无法读取当前 SIM 标识，未保存 PIN。')
    pin = getpass.getpass('当前 SIM PIN（不会回显）：')
    if not re.fullmatch(r'[0-9]{4,8}', pin):
        raise SystemExit('PIN 必须为 4 至 8 位数字。')
    record = json.dumps({'pin': pin, 'iccid': match[1]}, separators=(',', ':'))
    command = ['add-generic-password', '-U', '-a', 'sim-auto-unlock', '-s',
               'local.djonehub.macos', '-T', '/usr/bin/security', '-w', record]
    subprocess.run(['/usr/bin/security', '-i'], input=shlex.join(command) + '\n',
                   text=True, capture_output=True, timeout=30, check=True)
    check = subprocess.run(['/usr/bin/security', 'find-generic-password', '-a',
                            'sim-auto-unlock', '-s', 'local.djonehub.macos', '-w'],
                           text=True, capture_output=True, timeout=10, check=True)
    if json.loads(check.stdout) != json.loads(record):
        raise SystemExit('钥匙串验证失败。')
    print('已保存并绑定当前 SIM。重连后仅在 SIM PIN 状态尝试自动解锁。')
    print('本操作不会清除此前失败的 PIN 重试保护；请先人工核对原因。')


if __name__ == '__main__':
    try:
        main()
    except Exception:
        raise SystemExit('配置失败，请检查服务或钥匙串访问权限；未输出凭据。')
