#!/usr/bin/python3
"""Signed MiracleOS application updates from the public GitHub repository.

No release-supplied scripts run as root. A single current symlink activates an
immutable component set. Debian packages and personal files are not modified.
"""
import datetime
import fcntl
import hashlib
import json
import os
import pathlib
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile
import urllib.parse
import urllib.request

REPO = 'buschmeierflorian54-lgtm/miracleos'
BASE = pathlib.Path('/opt/miracleos/updates')
STATE = pathlib.Path('/var/lib/miracleos/updates')
KEY = pathlib.Path('/etc/miracleos/update-public.pem')
IMAGE = pathlib.Path('/etc/miracleos-image.json')
MAX_BUNDLE = 300 * 1024 * 1024
COMPONENTS = {
    'miracleos-desktop': '/opt/miracleos/miracleos-desktop',
    'luna-browser': '/usr/local/bin/luna-browser',
    'miracleos-sway': '/usr/local/libexec/miracleos-sway',
    'miracleos-app-tabs': '/usr/local/libexec/miracleos-app-tabs',
}

def atomic_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_suffix('.tmp')
    tmp.write_text(json.dumps(value, ensure_ascii=False) + '\n')
    tmp.chmod(0o644)
    os.replace(tmp, path)

def read_json(path, default=None):
    try:
        return json.loads(path.read_text())
    except FileNotFoundError:
        return default if default is not None else {}

def link(target, path):
    tmp = path.with_name(path.name + '.new')
    tmp.unlink(missing_ok=True)
    tmp.symlink_to(target)
    os.replace(tmp, path)

def installed():
    image = read_json(IMAGE)
    baseline = {'sequence': image.get('sequence', 0), 'version': image.get('version', 'Bisherige Installation')}
    return read_json(BASE / 'current' / 'release.json', baseline)

def status(**values):
    data = read_json(STATE / 'status.json')
    data.update(values)
    data['installed'] = installed().get('version', '')
    data['automatic'] = read_json(STATE / 'config.json', {'enabled': True}).get('enabled', True)
    atomic_json(STATE / 'status.json', data)

def fetch(url, limit, output=None):
    if urllib.parse.urlsplit(url).scheme != 'https':
        raise ValueError('HTTPS ist erforderlich.')
    request = urllib.request.Request(url, headers={'User-Agent': 'MiracleOS-Updater/1', 'Accept': 'application/vnd.github+json'})
    with urllib.request.urlopen(request, timeout=60) as response:
        if urllib.parse.urlsplit(response.url).scheme != 'https':
            raise ValueError('Unsichere Download-Weiterleitung.')
        total = 0
        parts = []
        while True:
            chunk = response.read(128 * 1024)
            if not chunk:
                break
            total += len(chunk)
            if total > limit:
                raise ValueError('Die Download-Größenbegrenzung wurde überschritten.')
            if output:
                output.write(chunk)
            else:
                parts.append(chunk)
        return b''.join(parts)

def asset_url(tag, name):
    if not re.fullmatch(r'[A-Za-z0-9._-]{1,100}', tag) or not re.fullmatch(r'[A-Za-z0-9._-]{1,100}', name):
        raise ValueError('Ungültiger Release- oder Dateiname.')
    return f'https://github.com/{REPO}/releases/download/{tag}/{name}'

def verify_manifest(raw, signature, directory, tag):
    if len(raw) > 16384 or len(signature) != 64:
        raise ValueError('Ungültige Update-Signatur.')
    message, sig = directory / 'manifest.json', directory / 'manifest.sig'
    message.write_bytes(raw)
    sig.write_bytes(signature)
    result = subprocess.run(['openssl', 'pkeyutl', '-verify', '-pubin', '-inkey', str(KEY), '-rawin', '-in', str(message), '-sigfile', str(sig)], capture_output=True)
    if result.returncode:
        raise ValueError('Die Update-Signatur ist ungültig. Nichts installiert.')
    manifest = json.loads(raw)
    if (manifest.get('format') != 1 or manifest.get('repository') != REPO or
            manifest.get('architecture') != 'amd64' or manifest.get('tag') != tag or
            type(manifest.get('sequence')) is not int or manifest['sequence'] <= 0 or
            not isinstance(manifest.get('version'), str) or not 1 <= len(manifest['version']) <= 100 or
            not re.fullmatch('[a-f0-9]{64}', manifest.get('sha256', '')) or
            type(manifest.get('size')) is not int or not 0 < manifest['size'] <= MAX_BUNDLE or
            manifest.get('asset') != 'miracleos-update-amd64.tar.gz'):
        raise ValueError('Das signierte Update passt nicht zu diesem System.')
    return manifest

def unpack(bundle, destination):
    seen, total = set(), 0
    with tarfile.open(bundle, 'r:gz') as archive:
        for member in archive:
            name = member.name
            if (name in seen or not member.isfile() or
                    not (name in COMPONENTS or re.fullmatch(r'runtime/[A-Za-z0-9_-]+\.py', name))):
                raise ValueError('Unerlaubter Inhalt im Update-Paket.')
            total += member.size
            if member.size <= 0 or total > 800 * 1024 * 1024:
                raise ValueError('Ungültige Größe im Update-Paket.')
            target = destination / name
            target.parent.mkdir(parents=True, exist_ok=True)
            with archive.extractfile(member) as source, target.open('xb') as output:
                shutil.copyfileobj(source, output)
            target.chmod(0o755)
            if name in COMPONENTS:
                with target.open('rb') as executable:
                    head = executable.read(20)
                if len(head) < 20 or head[:6] != b'\x7fELF\x02\x01' or head[18:20] != b'\x3e\x00':
                    raise ValueError('Keine passende amd64-Programmdatei.')
            seen.add(name)
    if not set(COMPONENTS).issubset(seen) or not any(n.startswith('runtime/') for n in seen):
        raise ValueError('Das Update-Paket ist unvollständig.')

def bootstrap():
    """Preserve the initial install; stable paths resolve via one version pointer."""
    releases = BASE / 'releases'
    releases.mkdir(parents=True, exist_ok=True)
    baseline = releases / 'baseline'
    if not (BASE / 'current').exists():
        if not baseline.exists():
            stage = pathlib.Path(tempfile.mkdtemp(prefix='.baseline-', dir=releases))
            try:
                for name, source in COMPONENTS.items():
                    shutil.copy2(source, stage / name)
                shutil.copytree('/opt/miracleos/runtime', stage / 'runtime')
                atomic_json(stage / 'release.json', installed())
                stage.chmod(0o755)
                os.replace(stage, baseline)
            finally:
                if stage.exists():
                    shutil.rmtree(stage)
        link('releases/baseline', BASE / 'current')
    if baseline.exists():
        baseline.chmod(0o755)
    for name, destination in COMPONENTS.items():
        link(BASE / 'current' / name, pathlib.Path(destination))

def activate(stage, manifest):
    bootstrap()
    destination = BASE / 'releases' / str(manifest['sequence'])
    atomic_json(stage / 'release.json', manifest)
    stage.chmod(0o755)
    if destination.exists():
        if read_json(destination / 'release.json') != manifest:
            raise ValueError('Versionskonflikt mit einem vorhandenen Update.')
        shutil.rmtree(stage)
    else:
        os.replace(stage, destination)
    destination.chmod(0o755)
    link(os.readlink(BASE / 'current'), BASE / 'previous')
    link('releases/' + destination.name, BASE / 'current')
    status(state='installed', available=manifest['version'], message='Update installiert. Neue Version nach erneuter Anmeldung verfügbar.', restart_required=True)

def check(apply=False, automatic=False):
    if pathlib.Path('/run/live/medium/live/filesystem.squashfs').exists():
        status(state='live', message='Updates werden erst auf dem installierten System aktiviert.')
        return
    if automatic and not read_json(STATE / 'config.json', {'enabled': True}).get('enabled', True):
        return
    status(state='checking', checked=datetime.datetime.now(datetime.timezone.utc).isoformat(), message='GitHub-Releases werden geprüft …')
    releases = json.loads(fetch(f'https://api.github.com/repos/{REPO}/releases?per_page=20', 1024 * 1024))
    if not isinstance(releases, list):
        raise ValueError('Ungültige GitHub-Antwort.')
    current = installed().get('sequence', 0)
    best = None
    STATE.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='check-', dir=STATE) as temp:
        directory = pathlib.Path(temp)
        for release in releases:
            assets = {a.get('name') for a in release.get('assets', [])}
            if release.get('draft') or not {'miracleos-update.json', 'miracleos-update.json.sig', 'miracleos-update-amd64.tar.gz'}.issubset(assets):
                continue
            tag = release['tag_name']
            raw = fetch(asset_url(tag, 'miracleos-update.json'), 16384)
            sig = fetch(asset_url(tag, 'miracleos-update.json.sig'), 64)
            manifest = verify_manifest(raw, sig, directory, tag)
            if manifest['sequence'] > current and (best is None or manifest['sequence'] > best['sequence']):
                best = manifest
        if best is None:
            status(state='current', message='Keine neuere signierte MiracleOS-Version verfügbar.', available='')
            return
        if not apply:
            status(state='available', available=best['version'], message='Eine neue signierte Version ist verfügbar.')
            return
        if shutil.disk_usage(BASE.parent).free < 2 * 1024 ** 3:
            raise ValueError('Für das Update werden mindestens 2 GB freier Speicher benötigt.')
        status(state='downloading', available=best['version'], message='Signiertes Update wird heruntergeladen …')
        bundle = directory / 'bundle.tar.gz'
        with bundle.open('wb') as output:
            fetch(asset_url(best['tag'], best['asset']), best['size'], output)
        if bundle.stat().st_size != best['size']:
            raise ValueError('Das Update wurde nicht vollständig heruntergeladen.')
        with bundle.open('rb') as source:
            if hashlib.file_digest(source, 'sha256').hexdigest() != best['sha256']:
                raise ValueError('Die Update-Prüfsumme stimmt nicht. Nichts installiert.')
        (BASE / 'releases').mkdir(parents=True, exist_ok=True)
        stage = pathlib.Path(tempfile.mkdtemp(prefix='.download-', dir=BASE / 'releases'))
        try:
            unpack(bundle, stage)
            activate(stage, best)
        finally:
            if stage.exists():
                shutil.rmtree(stage)

def rollback():
    previous = BASE / 'previous'
    if not previous.is_symlink():
        raise ValueError('Keine vorherige Version vorhanden.')
    target = os.readlink(previous)
    if not re.fullmatch(r'releases/(baseline|[0-9]+)', target) or not (BASE / target / 'release.json').is_file():
        raise ValueError('Ungültige vorherige Version.')
    old = os.readlink(BASE / 'current')
    link(target, BASE / 'current')
    link(old, previous)
    atomic_json(STATE / 'config.json', {'enabled': False})
    status(state='rolled-back', message='Vorherige Version wiederhergestellt. Automatische Updates pausiert; bitte erneut anmelden.', restart_required=True)

def main():
    if os.geteuid() != 0:
        raise ValueError('Administratorrechte erforderlich.')
    STATE.mkdir(parents=True, exist_ok=True)
    with pathlib.Path('/var/lib/miracleos/operation.lock').open('w') as system_lock, (STATE / 'operation.lock').open('w') as lock:
        fcntl.flock(system_lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        action = sys.argv[1] if len(sys.argv) > 1 else 'check'
        if action in ('check', 'apply', 'auto'):
            check(apply=action != 'check', automatic=action == 'auto')
        elif action in ('enable', 'disable'):
            atomic_json(STATE / 'config.json', {'enabled': action == 'enable'})
            status(message='Automatische Updates ' + ('aktiviert.' if action == 'enable' else 'pausiert.'))
        elif action == 'rollback':
            rollback()
        elif action == 'bootstrap':
            bootstrap()
            status(state='ready', message='Automatische Updates sind eingerichtet.')
        else:
            raise ValueError('Unbekannte Update-Aktion.')

if __name__ == '__main__':
    try:
        main()
    except BlockingIOError:
        print('Eine Update-Prüfung läuft bereits.', file=sys.stderr)
        sys.exit(1)
    except Exception as error:
        status(state='error', message=str(error))
        print(str(error), file=sys.stderr)
        sys.exit(1)
