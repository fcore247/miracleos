package main

import (
	"bytes"
	"context"
	"debug/pe"
	"encoding/binary"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func testWindowsExecutable(t *testing.T, machine uint16, name string) string {
	t.Helper()
	data := make([]byte, 128)
	copy(data, "MZ")
	binary.LittleEndian.PutUint32(data[60:], 128)
	buffer := bytes.NewBuffer(data)
	buffer.WriteString("PE\x00\x00")
	if err := binary.Write(buffer, binary.LittleEndian, pe.FileHeader{Machine: machine}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, buffer.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestWineInstalledRuntime(t *testing.T) {
	if os.Getenv("MIRACLEOS_WINE_TEST") != "1" {
		t.Skip("enable on the Wine test VM")
	}
	if !windowsSupportInstalled() {
		t.Fatal("32/64-bit Wine support missing")
	}
	prefix := filepath.Join(t.TempDir(), "windows")
	log, err := os.CreateTemp("", "miracle-wine-smoke-*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	defer os.Remove(log.Name())
	if err = prepareWindowsEnvironment(prefix, log); err != nil {
		t.Fatal(err)
	}
	defer exec.Command("env", "WINEPREFIX="+prefix, "wineserver", "-k").Run()
	files, err := filepath.Glob("/usr/lib/*/wine/*-windows/cmd.exe")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 2 {
		t.Fatalf("expected both Wine architectures, got %v", files)
	}
	for _, file := range files {
		if _, err = windowsAppPath(file); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		cmd := exec.CommandContext(ctx, "wine", file, "/c", "echo", "MiracleOS-Wine-OK")
		cmd.Env = windowsEnvironment(prefix)
		out, runErr := cmd.CombinedOutput()
		cancel()
		if runErr != nil || !strings.Contains(string(out), "MiracleOS-Wine-OK") {
			t.Fatalf("%s: %v: %s", file, runErr, out)
		}
		t.Logf("%s executed successfully", file)
	}
}

func TestWindowsFileValidationAndArguments(t *testing.T) {
	for _, machine := range []uint16{pe.IMAGE_FILE_MACHINE_I386, pe.IMAGE_FILE_MACHINE_AMD64} {
		path := testWindowsExecutable(t, machine, "Setup & example.EXE")
		uri := (&url.URL{Scheme: "file", Path: path}).String()
		if got, err := windowsAppPath(uri); err != nil || got != path {
			t.Fatalf("valid EXE: %q %v", got, err)
		}
		cmd := windowsCommand(path, t.TempDir())
		if len(cmd.Args) != 2 || cmd.Args[1] != path {
			t.Fatalf("file path was split or interpreted: %q", cmd.Args)
		}
	}
	arm := testWindowsExecutable(t, pe.IMAGE_FILE_MACHINE_ARM64, "arm.exe")
	if _, err := windowsAppPath(arm); err == nil {
		t.Fatal("accepted unsupported architecture")
	}
	bad := filepath.Join(t.TempDir(), "bad.exe")
	_ = os.WriteFile(bad, []byte("MZ not a Windows executable"), 0644)
	for _, path := range []string{bad, "file://remote-host/tmp/file.exe", filepath.Dir(bad)} {
		if _, err := windowsAppPath(path); err == nil {
			t.Fatalf("accepted %q", path)
		}
	}
	msi := windowsCommand("/tmp/setup with spaces.msi", "/tmp/prefix")
	if len(msi.Args) != 4 || msi.Args[1] != "msiexec" || msi.Args[3] != "/tmp/setup with spaces.msi" {
		t.Fatal(msi.Args)
	}
}

func TestWindowsPrefixInitializationSerialized(t *testing.T) {
	dir := t.TempDir()
	prefix := filepath.Join(dir, "prefix")
	boot := filepath.Join(dir, "wineboot")
	if err := os.WriteFile(boot, []byte("#!/bin/sh\nprintf 'init\\n' >> \"$WINEPREFIX/count\"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	log, err := os.Create(filepath.Join(dir, "log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	var group sync.WaitGroup
	for i := 0; i < 4; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if err := prepareWindowsEnvironment(prefix, log); err != nil {
				t.Error(err)
			}
		}()
	}
	group.Wait()
	count, err := os.ReadFile(filepath.Join(prefix, "count"))
	if err != nil || string(count) != "init\n" {
		t.Fatalf("initialization not serialized: %q %v", count, err)
	}
	t.Setenv("WINEPREFIX", "/wrong")
	t.Setenv("WINEARCH", "win32")
	for _, entry := range windowsEnvironment(prefix) {
		if strings.HasPrefix(entry, "WINEARCH=") || entry == "WINEPREFIX=/wrong" {
			t.Fatal("inherited incompatible Wine environment")
		}
	}
}
