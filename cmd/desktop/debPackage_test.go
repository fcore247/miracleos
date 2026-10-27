package main

import (
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDebInspectionAndFileURI(t *testing.T) {
	if _, err := exec.LookPath("dpkg-deb"); err != nil {
		t.Skip("dpkg-deb unavailable")
	}
	dir := t.TempDir()
	root := filepath.Join(dir, "payload")
	if err := os.MkdirAll(filepath.Join(root, "DEBIAN"), 0755); err != nil {
		t.Fatal(err)
	}
	control := "Package: miracleos-installer-fixture\nVersion: 1.2.3\nArchitecture: all\nMaintainer: Test <test@example.invalid>\nDescription: Installer test package\n More details\n"
	if err := os.WriteFile(filepath.Join(root, "DEBIAN", "control"), []byte(control), 0644); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "must-not-run")
	if err := os.WriteFile(filepath.Join(root, "DEBIAN", "postinst"), []byte("#!/bin/sh\ntouch '"+marker+"'\n"), 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "Paket mit Leerzeichen.deb")
	if out, err := exec.Command("dpkg-deb", "--build", root, file).CombinedOutput(); err != nil {
		t.Fatalf("build fixture: %v: %s", err, out)
	}
	uri := (&url.URL{Scheme: "file", Path: file}).String()
	p, err := inspectDebPackage(uri)
	if err != nil {
		t.Fatal(err)
	}
	if p.Path != file || p.Name != "miracleos-installer-fixture" || p.Version != "1.2.3" || p.Architecture != "all" || p.Size == 0 {
		t.Fatalf("incorrect metadata: %+v", p)
	}
	if _, err = os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("inspection executed package script")
	}
}

func TestDebRejectsInvalidSelection(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "broken.deb")
	if err := os.WriteFile(bad, []byte("not a package"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{dir, filepath.Join(dir, "missing.deb"), "file://other-host/tmp/test.deb", bad + ".txt"} {
		if _, err := debPackagePath(path); err == nil {
			t.Errorf("accepted invalid selection %q", path)
		}
	}
	if _, err := exec.LookPath("dpkg-deb"); err == nil {
		if _, err := inspectDebPackage(bad); err == nil {
			t.Fatal("accepted corrupt package")
		}
	}
}
