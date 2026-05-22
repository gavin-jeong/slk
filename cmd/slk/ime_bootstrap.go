package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gammons/slk/internal/config"
)

type imeBootstrapRunner interface {
	LookPath(file string) (string, error)
	Run(ctx context.Context, name string, args ...string) error
	Output(ctx context.Context, name string, args ...string) (string, error)
}

type systemIMEBootstrapRunner struct{}

func (systemIMEBootstrapRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (systemIMEBootstrapRunner) Run(ctx context.Context, name string, args ...string) error {
	return exec.CommandContext(ctx, name, args...).Run()
}

func (systemIMEBootstrapRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).Output()
	return strings.TrimSpace(string(out)), err
}

func bootstrapIMEConfigIfMissing(configPath string) (bool, error) {
	return bootstrapIMEConfigIfMissingWith(configPath, runtime.GOOS, systemIMEBootstrapRunner{})
}

func bootstrapIMEConfigIfMissingWith(configPath, goos string, runner imeBootstrapRunner) (bool, error) {
	if goos != "darwin" || config.HasGeneralIME(configPath) {
		return false, nil
	}

	macism, err := ensureMacism(runner)
	if err != nil {
		log.Printf("ime bootstrap skipped: %v", err)
		return false, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	current, err := runner.Output(ctx, macism)
	cancel()
	if err != nil || strings.TrimSpace(current) == "" {
		if err == nil {
			err = errors.New("current input source is empty")
		}
		log.Printf("ime bootstrap skipped: cannot read current input source: %v", err)
		return false, nil
	}
	current = strings.TrimSpace(current)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := runner.Run(ctx, macism, current); err != nil {
			log.Printf("ime bootstrap: restore input source %q failed: %v", current, err)
		}
	}()

	normal, err := discoverEnglishInputSource(runner, macism, []string{
		"com.apple.keylayout.US",
		"com.apple.keylayout.ABC",
	})
	if err != nil {
		log.Printf("ime bootstrap skipped: %v", err)
		return false, nil
	}

	if err := appendGeneralIMEConfigBlock(configPath, normal); err != nil {
		return false, err
	}
	log.Printf("ime bootstrap: enabled macism auto-switch normal_input_source=%q insert_source_detected=%q", normal, current)
	return true, nil
}

func ensureMacism(runner imeBootstrapRunner) (string, error) {
	if path, err := runner.LookPath("macism"); err == nil {
		return path, nil
	}
	brew, err := runner.LookPath("brew")
	if err != nil {
		return "", errors.New("macism not found and brew not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := runner.Run(ctx, brew, "tap", "laishulu/homebrew"); err != nil {
		return "", fmt.Errorf("brew tap laishulu/homebrew: %w", err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := runner.Run(ctx, brew, "install", "macism"); err != nil {
		return "", fmt.Errorf("brew install macism: %w", err)
	}
	if path, err := runner.LookPath("macism"); err == nil {
		return path, nil
	}
	return "", errors.New("macism installed but not found on PATH")
}

func discoverEnglishInputSource(runner imeBootstrapRunner, macism string, candidates []string) (string, error) {
	for _, candidate := range candidates {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := runner.Run(ctx, macism, candidate)
		cancel()
		if err != nil {
			continue
		}
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		current, err := runner.Output(ctx, macism)
		cancel()
		if err == nil && strings.TrimSpace(current) == candidate {
			return candidate, nil
		}
	}
	return "", errors.New("no usable English input source found")
}

func appendGeneralIMEConfigBlock(configPath, normalInputSource string) error {
	var existing []byte
	if data, err := os.ReadFile(configPath); err == nil {
		existing = data
	} else if !os.IsNotExist(err) {
		return err
	}

	var b strings.Builder
	if len(existing) > 0 {
		b.Write(existing)
		if !strings.HasSuffix(string(existing), "\n") {
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	b.WriteString("[general.ime]\n")
	b.WriteString("auto_switch = true\n")
	fmt.Fprintf(&b, "normal_input_source = %s\n", tomlString(normalInputSource))
	b.WriteString("switcher_command = \"macism\"\n")
	b.WriteString("restore_insert = true\n")
	b.WriteString("timeout_ms = 200\n")

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(configPath, []byte(b.String()), 0644)
}
