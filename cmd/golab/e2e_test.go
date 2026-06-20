package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestGolabBinaryE2EValidatorAndRender(t *testing.T) {
	repoRoot := findRepoRoot(t)
	binPath := filepath.Join(t.TempDir(), "golab")

	buildCtx, buildCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer buildCancel()
	runCmd(t, buildCtx, repoRoot, "go", "build", "-o", binPath, "./cmd/golab")

	scaleCtx, scaleCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer scaleCancel()
	scaleStdout, _ := runCmd(t, scaleCtx, repoRoot, binPath, "scale-test", "--seed", "42", "--target-bots", "20000", "--ticks", "50", "--warmup-ticks", "2")
	assertScaleSmoke(t, scaleStdout, 20000, 50)

	validatorScript := filepath.Join(repoRoot, "tools", "validate_colonies.sh")
	validateCtx, validateCancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer validateCancel()
	validateCmd := exec.CommandContext(validateCtx, validatorScript)
	validateCmd.Dir = repoRoot
	validateCmd.Env = append(os.Environ(),
		"GOLAB_BIN="+binPath,
		"SEEDS=1 2 3",
		"TICKS=1500",
		"INTERVAL=25",
		"TOP_BOTS=1",
	)
	validateStdout, _ := runPreparedCmd(t, validateCmd)

	if validateCmd.Args[0] != validatorScript {
		t.Fatalf("validator command = %q, want %q", validateCmd.Args[0], validatorScript)
	}
	if !envContains(validateCmd.Env, "GOLAB_BIN="+binPath) {
		t.Fatalf("validator env does not include temp binary path %q", binPath)
	}
	t.Logf("ran %s with GOLAB_BIN=%s", validatorScript, binPath)
	assertValidatorRows(t, validateStdout, []string{"1", "2", "3"})

	renderPath := filepath.Join(t.TempDir(), "pheromone.png")
	renderCtx, renderCancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer renderCancel()
	runCmd(t, renderCtx, repoRoot, binPath, "render", "--style", "pheromone", "--seed", "42", "--ticks", "800", "--output", renderPath)
	assertPNGFile(t, renderPath)

	biomeRenderPath := filepath.Join(t.TempDir(), "biome.png")
	biomeRenderCtx, biomeRenderCancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer biomeRenderCancel()
	runCmd(t, biomeRenderCtx, repoRoot, binPath, "render", "--style", "biome", "--seed", "42", "--ticks", "0", "--output", biomeRenderPath)
	assertPNGFile(t, biomeRenderPath)
}

func assertScaleSmoke(t *testing.T, output string, targetBots, ticks int) {
	t.Helper()
	var payload struct {
		Command             string  `json:"command"`
		Rows                int     `json:"rows"`
		Cols                int     `json:"cols"`
		TargetBots          int     `json:"target_bots"`
		InitialLiveBots     int     `json:"initial_live_bots"`
		Ticks               int     `json:"ticks"`
		LogicTicksPerSecond float64 `json:"logic_ticks_per_second"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("parse scale-test JSON: %v\noutput:\n%s", err, output)
	}
	if payload.Command != "scale-test" {
		t.Fatalf("scale command = %q, want scale-test", payload.Command)
	}
	if payload.Rows != 400 || payload.Cols != 600 {
		t.Fatalf("scale board = %dx%d, want 400x600", payload.Rows, payload.Cols)
	}
	if payload.TargetBots != targetBots || payload.InitialLiveBots != targetBots {
		t.Fatalf("scale target/live = %d/%d, want %d", payload.TargetBots, payload.InitialLiveBots, targetBots)
	}
	if payload.Ticks != ticks {
		t.Fatalf("scale ticks = %d, want %d", payload.Ticks, ticks)
	}
	if payload.LogicTicksPerSecond <= 0 {
		t.Fatalf("scale logic TPS = %f, want positive", payload.LogicTicksPerSecond)
	}
}

func runCmd(t *testing.T, ctx context.Context, dir string, name string, args ...string) (string, string) {
	t.Helper()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return runPreparedCmd(t, cmd)
}

func runPreparedCmd(t *testing.T, cmd *exec.Cmd) (string, string) {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s failed: %v\nstdout:\n%s\nstderr:\n%s", strings.Join(cmd.Args, " "), err, stdout.String(), stderr.String())
	}
	return stdout.String(), stderr.String()
}

func assertValidatorRows(t *testing.T, output string, seeds []string) {
	t.Helper()
	reader := csv.NewReader(strings.NewReader(output))
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("parse validator TSV: %v\noutput:\n%s", err, output)
	}
	if len(records) != len(seeds)+1 {
		t.Fatalf("validator row count = %d, want %d\noutput:\n%s", len(records), len(seeds)+1, output)
	}

	wantHeader := []string{
		"seed",
		"events",
		"resource_rain",
		"food_rain",
		"colony_support",
		"settlement_support",
		"final_live",
		"final_divisions",
		"final_active",
		"final_solo",
		"final_max_members",
		"final_max_connected",
		"peak_members",
		"peak_connected",
		"pheromone_cells",
	}
	if got := strings.Join(records[0], "\t"); got != strings.Join(wantHeader, "\t") {
		t.Fatalf("validator header = %q, want %q", got, strings.Join(wantHeader, "\t"))
	}

	for i, seed := range seeds {
		row := records[i+1]
		if len(row) != len(wantHeader) {
			t.Fatalf("validator row %d has %d fields, want %d: %v", i+1, len(row), len(wantHeader), row)
		}
		if row[0] != seed {
			t.Fatalf("validator seed row %d = %q, want %q", i+1, row[0], seed)
		}
		for fieldIndex, value := range row {
			if _, err := strconv.Atoi(value); err != nil {
				t.Fatalf("validator row %d field %q is not an integer: %q", i+1, wantHeader[fieldIndex], value)
			}
		}
	}
}

func assertPNGFile(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read render output %s: %v", path, err)
	}
	if len(data) <= 8 {
		t.Fatalf("render output %s is too small: %d bytes", path, len(data))
	}
	if !bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		t.Fatalf("render output %s does not have a PNG signature", path)
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate repo root containing go.mod")
		}
		dir = parent
	}
}

func envContains(env []string, value string) bool {
	for _, item := range env {
		if item == value {
			return true
		}
	}
	return false
}
