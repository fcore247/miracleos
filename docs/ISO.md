# MiracleOS ISO

The ISO contains a clean Debian 13 amd64 system, the Linux kernel, MiracleOS Desktop,
Sway, networking, audio, Firefox and the official Spotify Debian package. It does not
contain a copy of a development VM, user accounts, SSH keys or Spotify credentials.

Build on a disposable Debian 13 machine:

```sh
sudo bash scripts/iso/build.sh /home/builder/miracleos-live
```

For a dated release, pass its version and monotonically increasing update sequence,
for example `sudo env MIRACLEOS_VERSION=0.1.0-test.20261027 MIRACLEOS_SEQUENCE=2026102701 MIRACLEOS_SWAY_BUILD_DIR=/mnt/build/sway-source bash scripts/iso/build.sh /mnt/build/live`.
Use a fresh compositor source directory when its patches have changed. The image
records its sequence so automatic updates cannot replace it with an older release.

The build runs Go tests and the installer's Python safety tests, then writes
`live-image-amd64.hybrid.iso` and `SHA256SUMS`. The hybrid image supports DVD and USB
boot on x86-64 machines, with BIOS and UEFI bootloaders. Secure Boot is not configured
in this test image.

Booting the ISO opens the installation screen. The user selects an unmounted local
disk, confirms deletion and confirms again. The installer creates BIOS boot, EFI and
ext4 system partitions, copies the clean image and installs both bootloaders. The
next boot from disk opens first-run setup: name, password, optional Spotify sign-in
and playback, then normal password login. Spotify sign-in takes place in Spotify's
real application and its profile is transferred to the newly created account.

Alt+F toggles the Spotify overlay. MPRIS supplies cover, title, artist, album,
playback position, transport buttons, volume, shuffle and repeat. Unsupported
controls are disabled.

Settings → System → Reset asks for confirmation, verifies the user's system
password and checks the inserted matching ISO, then asks for final confirmation.
It boots the ISO kernel with kexec and identifies the installed root filesystem
by UUID. Only that root filesystem's parent disk is repartitioned and reinstalled.
The reset erases every partition on that system disk; it does not securely overwrite
every physical sector. Other disks are not installation targets. Failure to find the
matching ISO or load its kernel prevents deletion.

Test installation and reset in a separate VM. Never use a development or personal
disk as an installation target. No installation or reset is performed just by
building the ISO or opening a confirmation dialog.
