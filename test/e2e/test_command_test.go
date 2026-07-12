package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runZeusTest runs `zeus test <dir>` with NO_COLOR + an explicit ZEUS_HOME (so the runner's
// `zeus build` subprocesses resolve @std/test and the runtime/GC regardless of how `go test`
// was invoked) and returns the combined output and exit code.
func runZeusTest(t *testing.T, compilerPath, dir string) (string, int) {
	t.Helper()

	rootDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}

	cmd := exec.Command(compilerPath, "test", dir)
	cmd.Env = append(os.Environ(), "ZEUS_HOME="+rootDir, "NO_COLOR=1")
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("Failed to run `zeus test`: %v\nOutput: %s", err, out)
		}
	}
	return string(out), exitCode
}

// TestCommand exercises the `zeus test` runner against fixture directories, asserting the exit
// code and the rendered summary counts.
func TestCommand(t *testing.T) {
	compilerPath := buildCompiler(t)

	base, err := filepath.Abs("testdata/test_runner")
	if err != nil {
		t.Fatalf("Failed to resolve fixtures dir: %v", err)
	}

	t.Run("mixed pass and fail exits non-zero", func(t *testing.T) {
		out, exit := runZeusTest(t, compilerPath, filepath.Join(base, "mixed"))
		if exit != 1 {
			t.Errorf("expected exit code 1, got %d\nOutput:\n%s", exit, out)
		}
		for _, want := range []string{
			"✓ adds",
			"✗ bad",
			"boom",
			"Tests  1 failed, 3 passed, 4 total",
			"Test Files  1 failed, 1 passed, 2 total",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q\nOutput:\n%s", want, out)
			}
		}
	})

	t.Run("all passing exits zero", func(t *testing.T) {
		out, exit := runZeusTest(t, compilerPath, filepath.Join(base, "allpass"))
		if exit != 0 {
			t.Errorf("expected exit code 0, got %d\nOutput:\n%s", exit, out)
		}
		if !strings.Contains(out, "Tests  1 passed, 1 total") {
			t.Errorf("output missing summary line\nOutput:\n%s", out)
		}
	})

	t.Run("no test files exits zero", func(t *testing.T) {
		empty := t.TempDir()
		out, exit := runZeusTest(t, compilerPath, empty)
		if exit != 0 {
			t.Errorf("expected exit code 0, got %d\nOutput:\n%s", exit, out)
		}
		if !strings.Contains(out, "No test files found") {
			t.Errorf("expected 'No test files found' message\nOutput:\n%s", out)
		}
	})
}
