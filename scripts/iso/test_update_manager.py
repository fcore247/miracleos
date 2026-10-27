import hashlib
import importlib.util
import io
import json
import pathlib
import subprocess
import tarfile
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('updates', pathlib.Path(__file__).with_name('update-manager.py'))
u = importlib.util.module_from_spec(spec)
spec.loader.exec_module(u)

class UpdatesTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = pathlib.Path(self.temp.name)
        self.addCleanup(self.temp.cleanup)
        self.key = self.root / 'private.pem'
        self.pub = self.root / 'public.pem'
        subprocess.run(['openssl', 'genpkey', '-algorithm', 'ED25519', '-out', str(self.key)], check=True, capture_output=True)
        subprocess.run(['openssl', 'pkey', '-in', str(self.key), '-pubout', '-out', str(self.pub)], check=True, capture_output=True)
        for name, value in [('BASE', self.root / 'opt'), ('STATE', self.root / 'state'), ('KEY', self.pub), ('IMAGE', self.root / 'image.json')]:
            p = patch.object(u, name, value)
            p.start()
            self.addCleanup(p.stop)
        self.manifest = dict(format=1, repository=u.REPO, architecture='amd64', tag='test-1', sequence=1, version='Test 1', sha256='a'*64, size=100, asset='miracleos-update-amd64.tar.gz')

    def signature(self, raw):
        message, sig = self.root / 'message', self.root / 'signature'
        message.write_bytes(raw)
        subprocess.run(['openssl', 'pkeyutl', '-sign', '-inkey', str(self.key), '-rawin', '-in', str(message), '-out', str(sig)], check=True, capture_output=True)
        return sig.read_bytes()

    def test_signature_rejects_tampering_and_wrong_release(self):
        raw = json.dumps(self.manifest).encode()
        sig = self.signature(raw)
        self.assertEqual(u.verify_manifest(raw, sig, self.root, 'test-1')['sequence'], 1)
        with self.assertRaises(ValueError): u.verify_manifest(raw+b' ', sig, self.root, 'test-1')
        with self.assertRaises(ValueError): u.verify_manifest(raw, sig, self.root, 'wrong-tag')

    def archive(self, extra=None):
        package = self.root / 'package.tar.gz'
        elf = b'\x7fELF\x02\x01'+b'\x00'*12+b'\x3e\x00'+b'\x00'*20
        with tarfile.open(package, 'w:gz') as archive:
            for name in [*u.COMPONENTS, 'runtime/test.py']:
                data = b'print(1)' if name.startswith('runtime/') else elf
                member = tarfile.TarInfo(name)
                member.size = len(data)
                archive.addfile(member, io.BytesIO(data))
            if extra:
                archive.addfile(extra, io.BytesIO(b'x'*extra.size) if extra.isfile() else None)
        return package

    def test_extract_rejects_escape_symlink_and_duplicates(self):
        for name, kind in [('../outside', tarfile.REGTYPE), ('luna-browser', tarfile.SYMTYPE), ('luna-browser', tarfile.REGTYPE)]:
            with self.subTest(name=name, kind=kind):
                member = tarfile.TarInfo(name)
                member.type = kind
                member.linkname = '/etc/shadow'
                member.size = 1 if kind == tarfile.REGTYPE else 0
                with tempfile.TemporaryDirectory(dir=self.root) as temp:
                    with self.assertRaises(ValueError): u.unpack(self.archive(member), pathlib.Path(temp))
        self.assertFalse((self.root / 'outside').exists())

    def test_extract_valid_component_set(self):
        output = self.root / 'stage'
        output.mkdir()
        u.unpack(self.archive(), output)
        self.assertTrue((output / 'runtime/test.py').exists())
        self.assertEqual((output / 'luna-browser').stat().st_mode & 0o777, 0o755)

    def test_activation_and_rollback_preserve_version_sets(self):
        old = u.BASE / 'releases/baseline'
        old.mkdir(parents=True)
        u.atomic_json(old / 'release.json', dict(version='Old', sequence=0))
        u.link('releases/baseline', u.BASE / 'current')
        stage = u.BASE / 'releases/staging'
        stage.mkdir()
        stage.chmod(0o700)
        with patch.object(u, 'bootstrap'):
            u.activate(stage, self.manifest)
        self.assertEqual(u.installed()['sequence'], 1)
        self.assertEqual((u.BASE / 'releases/1').stat().st_mode & 0o777, 0o755)
        self.assertTrue(old.exists())
        u.rollback()
        self.assertEqual(u.installed()['sequence'], 0)
        self.assertFalse(u.read_json(u.STATE / 'config.json')['enabled'])
        self.assertTrue((u.BASE / 'releases/1').exists())

    def test_bad_download_never_activates(self):
        bundle = self.archive()
        manifest = {**self.manifest, 'size': bundle.stat().st_size}
        raw = json.dumps(manifest).encode()
        sig = self.signature(raw)
        release = [{'tag_name':'test-1', 'assets':[{'name':n} for n in ['miracleos-update.json','miracleos-update.json.sig','miracleos-update-amd64.tar.gz']]}]
        def download(url, limit, output=None):
            if '/releases?' in url: return json.dumps(release).encode()
            if url.endswith('.json.sig'): return sig
            if url.endswith('.json'): return raw
            output.write(bundle.read_bytes())
        u.BASE.parent.mkdir(exist_ok=True)
        with patch.object(u, 'fetch', download), patch.object(u, 'activate') as activate:
            with self.assertRaises(ValueError): u.check(apply=True)
            activate.assert_not_called()
        self.assertFalse((u.BASE / 'current').exists())

    def test_older_release_is_not_downloaded(self):
        old = u.BASE / 'current'
        old.mkdir(parents=True)
        u.atomic_json(old / 'release.json', {'version':'New', 'sequence':10})
        raw = json.dumps(self.manifest).encode()
        sig = self.signature(raw)
        release = [{'tag_name':'test-1', 'assets':[{'name':n} for n in ['miracleos-update.json','miracleos-update.json.sig','miracleos-update-amd64.tar.gz']]}]
        def download(url, limit, output=None):
            if '/releases?' in url: return json.dumps(release).encode()
            if url.endswith('.json.sig'): return sig
            if url.endswith('.json'): return raw
            self.fail('Old bundle must not be downloaded')
        with patch.object(u, 'fetch', download): u.check(apply=True)
        self.assertEqual(u.installed()['sequence'], 10)

    def test_fresh_iso_never_downloads_an_older_update(self):
        u.atomic_json(u.IMAGE, {'version': 'ISO 20261027', 'sequence': 2026102701})
        self.assertFalse((u.BASE / 'current').exists())
        raw = json.dumps(self.manifest).encode()
        sig = self.signature(raw)
        release = [{'tag_name': 'test-1', 'assets': [{'name': n} for n in ['miracleos-update.json', 'miracleos-update.json.sig', 'miracleos-update-amd64.tar.gz']]}]
        def download(url, limit, output=None):
            if '/releases?' in url: return json.dumps(release).encode()
            if url.endswith('.json.sig'): return sig
            if url.endswith('.json'): return raw
            self.fail('Older bundle must not replace a fresh ISO')
        with patch.object(u, 'fetch', download): u.check(apply=True)
        self.assertEqual(u.installed()['sequence'], 2026102701)

if __name__ == '__main__': unittest.main()
