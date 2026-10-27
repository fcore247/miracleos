#!/bin/bash
# Build in a disposable Debian 13 VM, never in the user's original VM.
set -euo pipefail
repo=$(cd "$(dirname "$0")/../.." && pwd)
build=${1:-/home/builder/miracleos-live}
luna=${2:-/home/builder/luna-linux/build/bin/luna-browser}
if [ "$EUID" -ne 0 ]; then echo 'Run with sudo.' >&2; exit 1; fi
if [ ! -f "$luna" ]; then echo 'Pass the tested Luna Linux binary as the second argument.' >&2; exit 1; fi
luna=$(realpath "$luna")
sway_root=${MIRACLEOS_SWAY_BUILD_DIR:-/home/builder/sway-source}
version=${MIRACLEOS_VERSION:-0.1-test}
sequence=${MIRACLEOS_SEQUENCE:-0}
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y live-build debootstrap xorriso squashfs-tools mtools dosfstools \
  golang-go build-essential libgtk-3-dev libgtk-layer-shell-dev libvte-2.91-dev \
  libglib2.0-dev libwebkit2gtk-4.1-dev curl gnupg rsync xvfb
echo 'deb-src http://deb.debian.org/debian trixie main' > /etc/apt/sources.list.d/miracleos-source.list
apt-get update
apt-get build-dep -y sway
mkdir -p "$sway_root"
cd "$sway_root"
if [ ! -f sway-1.10.1/build-miracleos/sway/sway ]; then
  apt-get source sway=1.10.1-2
  sh "$repo/compositor/build.sh" "$sway_root/sway-1.10.1"
fi
cd "$repo"
python3 -m unittest discover -s scripts/iso -p 'test_*.py'
python3 -m py_compile scripts/iso/system-helper.py
xvfb-run -a go test ./...
MIRACLEOS_DBUS_TEST=1 dbus-run-session -- xvfb-run -a go test ./cmd/desktop -run TestSpotifyDBus -count=1
go build -o /tmp/miracleos-desktop ./cmd/desktop
go build -o /tmp/miracleos-app-tabs ./cmd/app-tabs
mkdir -p "$build"
cd "$build"
lb config --distribution trixie --architectures amd64 --binary-images iso-hybrid \
  --archive-areas 'main contrib non-free non-free-firmware' \
  --bootloaders 'syslinux grub-efi' --debian-installer none \
  --iso-volume MIRACLEOS --iso-application MiracleOS --iso-publisher MiracleOS \
  --bootappend-live 'boot=live components username=miracle locales=de_DE.UTF-8 keyboard-layouts=de quiet' \
  --security true --updates true --memtest none
cp -a /usr/share/live/build/bootloaders config/
sed -i 's/^timeout 0/timeout 50/' config/bootloaders/isolinux/isolinux.cfg
sed -i 's/Live system/MiracleOS starten und installieren/g' config/bootloaders/syslinux_common/live.cfg.in
cp "$repo/scripts/iso/grub.cfg" config/bootloaders/grub-pc/grub.cfg
mkdir -p config/package-lists config/hooks/live config/includes.chroot/opt/miracleos/runtime \
  config/includes.chroot/usr/local/lib/miracleos config/includes.chroot/etc \
  config/includes.chroot/usr/local/libexec \
  config/includes.chroot/usr/share/backgrounds/miracleos config/includes.binary
cat > config/package-lists/miracleos.list.chroot <<'EOF'
linux-image-amd64
firmware-linux
firmware-iwlwifi
firmware-realtek
live-boot
live-config
live-config-systemd
systemd-sysv
sudo
python3
greetd
gnome-keyring
libpam-gnome-keyring
libglib2.0-bin
sway
libgtk-3-0t64
libgtk-layer-shell0
libvte-2.91-0
foot
firefox-esr
xwayland
dbus-user-session
network-manager
network-manager-gnome
pipewire
pipewire-pulse
pulseaudio-utils
alsa-utils
hyperv-daemons
libwebkit2gtk-4.1-0
libjson-glib-1.0-0
wireplumber
lxpolkit
pkexec
desktop-file-utils
wine
wine64
fonts-wine
xdg-desktop-portal
xdg-desktop-portal-wlr
xdg-utils
libnotify-bin
fonts-dejavu
fonts-noto-color-emoji
locales
ca-certificates
openssl
curl
gnupg
rsync
parted
dosfstools
e2fsprogs
grub-pc-bin
grub-efi-amd64-bin
grub2-common
kexec-tools
EOF
install -m 755 /tmp/miracleos-desktop config/includes.chroot/opt/miracleos/miracleos-desktop
install -m 755 /tmp/miracleos-app-tabs config/includes.chroot/usr/local/libexec/miracleos-app-tabs
install -D -m 755 "$luna" config/includes.chroot/usr/local/bin/luna-browser
mkdir -p config/includes.chroot/usr/share/applications
cat > config/includes.chroot/usr/share/applications/luna-browser.desktop <<'EOF'
[Desktop Entry]
Type=Application
Name=Luna Browser
Exec=luna-browser %U
Icon=web-browser
Terminal=false
Categories=Network;WebBrowser;
MimeType=text/html;x-scheme-handler/http;x-scheme-handler/https;
EOF
install -m 755 "$sway_root/sway-1.10.1/build-miracleos/sway/sway" config/includes.chroot/usr/local/libexec/miracleos-sway
install -m 755 "$repo"/runtime/*.py config/includes.chroot/opt/miracleos/runtime/
install -m 755 "$repo/scripts/iso/system-helper.py" config/includes.chroot/usr/local/lib/miracleos/
install -m 755 "$repo/scripts/iso/update-manager.py" config/includes.chroot/usr/local/lib/miracleos/
install -D -m 644 "$repo/scripts/iso/update-public.pem" config/includes.chroot/etc/miracleos/update-public.pem
install -m 755 "$repo/scripts/iso/configure-updates.sh" config/includes.chroot/usr/local/lib/miracleos/
install -m 755 "$repo/scripts/iso/configure-image.sh" config/hooks/live/0900-miracleos.hook.chroot
cp "$repo/cmd/desktop/images/wallpapers/"*.png config/includes.chroot/usr/share/backgrounds/miracleos/
image_id=$(cat /tmp/miracleos-desktop /tmp/miracleos-app-tabs "$luna" "$repo/scripts/iso/system-helper.py" "$repo/scripts/iso/update-manager.py" "$repo/scripts/iso/update-public.pem" "$repo/scripts/iso/configure-image.sh" "$repo/scripts/iso/grub.cfg" "$sway_root/sway-1.10.1/build-miracleos/sway/sway" | sha256sum | cut -d' ' -f1)
printf '{"name":"MiracleOS","version":"%s","sequence":%s,"architecture":"amd64","image_id":"%s"}\n' "$version" "$sequence" "$image_id" > config/includes.chroot/etc/miracleos-image.json
cp config/includes.chroot/etc/miracleos-image.json config/includes.binary/
# Larger XZ blocks and the x86 filter keep the Wine-enabled ISO compact.
export MKSQUASHFS_OPTIONS=${MKSQUASHFS_OPTIONS:--b 1M -Xbcj x86}
lb build
sha256sum live-image-amd64.hybrid.iso > SHA256SUMS
