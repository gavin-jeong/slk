package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeIMERunner struct {
	paths              map[string]string
	macismAfterInstall bool
	current            string
	runErr             map[string]error
	outputs            []string
	runs               []string
	lookups            []string
}

func (f *fakeIMERunner) LookPath(file string) (string, error) {
	f.lookups = append(f.lookups, file)
	if p, ok := f.paths[file]; ok {
		return p, nil
	}
	if file == "macism" && f.macismAfterInstall {
		for _, run := range f.runs {
			if strings.Contains(run, "install macism") {
				return "/opt/homebrew/bin/macism", nil
			}
		}
	}
	return "", errors.New("not found")
}

func (f *fakeIMERunner) Run(ctx context.Context, name string, args ...string) error {
	call := name + " " + strings.Join(args, " ")
	f.runs = append(f.runs, strings.TrimSpace(call))
	if err := f.runErr[strings.Join(args, " ")]; err != nil {
		return err
	}
	if len(args) == 1 {
		f.current = args[0]
	}
	return nil
}

func (f *fakeIMERunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	if len(f.outputs) > 0 {
		out := f.outputs[0]
		f.outputs = f.outputs[1:]
		return out, nil
	}
	return f.current, nil
}

func TestBootstrapIMEConfigWritesMissingSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("# Sendbird\n[workspaces.sendbird]\nteam_id = \"T0\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	r := &fakeIMERunner{
		paths:   map[string]string{"macism": "/opt/homebrew/bin/macism"},
		current: "com.apple.inputmethod.Korean.2SetKorean",
	}

	wrote, err := bootstrapIMEConfigIfMissingWith(path, "darwin", r)
	if err != nil {
		t.Fatal(err)
	}
	if !wrote {
		t.Fatal("expected bootstrap to write config")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"[general.ime]",
		"auto_switch = true",
		"normal_input_source = \"com.apple.keylayout.US\"",
		"switcher_command = \"macism\"",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("config missing %q:\n%s", want, text)
		}
	}
	if got, want := r.current, "com.apple.inputmethod.Korean.2SetKorean"; got != want {
		t.Fatalf("current source after bootstrap = %q, want restored %q", got, want)
	}
}

func TestBootstrapIMEConfigLeavesExistingSectionUntouched(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	initial := "[general.ime]\nauto_switch = false\n"
	if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}
	r := &fakeIMERunner{paths: map[string]string{"macism": "/opt/homebrew/bin/macism"}}

	wrote, err := bootstrapIMEConfigIfMissingWith(path, "darwin", r)
	if err != nil {
		t.Fatal(err)
	}
	if wrote {
		t.Fatal("expected no write for existing [general.ime]")
	}
	data, _ := os.ReadFile(path)
	if string(data) != initial {
		t.Fatalf("config changed: %q", string(data))
	}
	if len(r.runs) != 0 {
		t.Fatalf("runner invoked despite existing config: %#v", r.runs)
	}
}

func TestEnsureMacismInstallsWithBrew(t *testing.T) {
	r := &fakeIMERunner{
		paths:              map[string]string{"brew": "/opt/homebrew/bin/brew"},
		macismAfterInstall: true,
	}

	path, err := ensureMacism(r)
	if err != nil {
		t.Fatal(err)
	}
	if path != "/opt/homebrew/bin/macism" {
		t.Fatalf("path = %q, want macism path", path)
	}
	joined := strings.Join(r.runs, "\n")
	if !strings.Contains(joined, "tap laishulu/homebrew") || !strings.Contains(joined, "install macism") {
		t.Fatalf("brew install commands not run: %#v", r.runs)
	}
}

func TestBootstrapIMEConfigSkipsWhenMacismUnavailable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	r := &fakeIMERunner{}

	wrote, err := bootstrapIMEConfigIfMissingWith(path, "darwin", r)
	if err != nil {
		t.Fatal(err)
	}
	if wrote {
		t.Fatal("expected no write when macism and brew are missing")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("config should not be created, stat err=%v", err)
	}
}

func TestDiscoverEnglishInputSourceSkipsUnavailableCandidates(t *testing.T) {
	r := &fakeIMERunner{
		current: "com.apple.inputmethod.Korean.2SetKorean",
		runErr: map[string]error{
			"com.apple.keylayout.US": errors.New("missing"),
		},
	}
	got, err := discoverEnglishInputSource(r, "macism", []string{"com.apple.keylayout.US", "com.apple.keylayout.ABC"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "com.apple.keylayout.ABC" {
		t.Fatalf("source = %q, want ABC", got)
	}
}
