package e2e

import (
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestSpec represents a single test case in a spec file
type TestSpec struct {
	Name           string   `json:"name"`
	Exit           string   `json:"exit"`
	Entry          string   `json:"entry"`
	Stdout         string   `json:"stdout,omitempty"`
	Stderr         string   `json:"stderr,omitempty"`          // Expected exact stderr output
	StderrContains []string `json:"stderr_contains,omitempty"` // Strings that must be present in stderr
	NoColor        bool     `json:"no_color,omitempty"`        // Run with NO_COLOR=1
	CompileError   bool     `json:"compile_error,omitempty"`   // Expect compilation to fail
}

// buildCompiler builds the Zeus compiler and runtime, placing the compiler in the project root
func buildCompiler(t *testing.T) string {
	t.Helper()

	// Change to project root (two levels up from test/e2e)
	rootDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}

	compilerPath := filepath.Join(rootDir, "zeus")

	// Build the compiler
	cmd := exec.Command("go", "build", "-tags", "llvm19", "-o", compilerPath, "zeus.go")
	cmd.Dir = rootDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build compiler: %v\nOutput: %s", err, output)
	}

	t.Logf("Built Zeus compiler at: %s", compilerPath)
	return compilerPath
}

// readSpecFiles recursively reads all spec.json files under the specs directory
func readSpecFiles(t *testing.T, specsDir string) map[string][]TestSpec {
	t.Helper()

	specs := make(map[string][]TestSpec)

	err := filepath.WalkDir(specsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.Name() == "spec.json" {
			// Read and parse the spec file
			data, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("Failed to read spec file %s: %v", path, err)
				return nil
			}

			var testSpecs []TestSpec
			if err := json.Unmarshal(data, &testSpecs); err != nil {
				t.Errorf("Failed to parse spec file %s: %v", path, err)
				return nil
			}

			// Use the directory name as the test suite name
			suiteDir := filepath.Dir(path)
			suiteName := filepath.Base(suiteDir)
			specs[suiteName] = testSpecs

			t.Logf("Loaded %d test specs from %s", len(testSpecs), path)
		}

		return nil
	})

	if err != nil {
		t.Fatalf("Failed to walk specs directory: %v", err)
	}

	return specs
}

// runTestSpec runs a single test specification
func runTestSpec(t *testing.T, compilerPath, suiteDir, outputDir string, spec TestSpec) {
	t.Helper()

	var err error

	// Get project root directory
	rootDir, err := filepath.Abs("../..")
	if err != nil {
		t.Errorf("Failed to get project root: %v", err)
		return
	}

	// Prepare paths
	entryFile := filepath.Join(suiteDir, spec.Entry)
	outputFile := filepath.Join(outputDir, "a.out")

	// Ensure entry file exists
	if _, err = os.Stat(entryFile); os.IsNotExist(err) {
		t.Errorf("Entry file does not exist: %s", entryFile)
		return
	}

	// Compile the Zeus file - run from project root
	compileCmd := exec.Command(compilerPath, "build", filepath.Join(rootDir, "test/e2e", entryFile), "-o", filepath.Join(rootDir, "test/e2e", outputFile))
	compileCmd.Dir = rootDir
	compileOutput, compileErr := compileCmd.CombinedOutput()
	if spec.CompileError {
		if compileErr == nil {
			t.Errorf("Test '%s': expected compile error but compilation succeeded", spec.Name)
			return
		}
		for _, expected := range spec.StderrContains {
			if !strings.Contains(string(compileOutput), expected) {
				t.Errorf("Test '%s': compiler output does not contain %q\nActual output:\n%s",
					spec.Name, expected, compileOutput)
			}
		}
		return
	}
	if compileErr != nil {
		t.Errorf("Failed to compile %s: %v\nOutput: %s", entryFile, compileErr, compileOutput)
		return
	}

	// Execute the compiled program and capture stdout/stderr
	execCmd := exec.Command(outputFile)
	
	// Set NO_COLOR environment variable if specified
	if spec.NoColor {
		execCmd.Env = append(os.Environ(), "NO_COLOR=1")
	}

	var stdoutBuf, stderrBuf strings.Builder
	execCmd.Stdout = &stdoutBuf
	execCmd.Stderr = &stderrBuf

	err = execCmd.Run()
	actualStdout := stdoutBuf.String()
	actualStderr := stderrBuf.String()

	// Get the exit code
	var actualExitCode int
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			actualExitCode = exitError.ExitCode()
		} else {
			t.Errorf("Failed to execute %s: %v", outputFile, err)
			return
		}
	} else {
		actualExitCode = 0
	}

	// Parse expected exit code
	expectedExitCode, err := strconv.Atoi(spec.Exit)
	if err != nil {
		t.Errorf("Invalid exit code in spec: %s", spec.Exit)
		return
	}

	// Compare exit codes
	if actualExitCode != expectedExitCode {
		t.Errorf("Test '%s' failed: expected exit code %d, got %d\nStderr: %s",
			spec.Name, expectedExitCode, actualExitCode, actualStderr)
	}

	// Compare stdout if specified
	if spec.Stdout != "" {
		expectedStdout := spec.Stdout
		if actualStdout != expectedStdout {
			t.Errorf("Test '%s' failed: stdout mismatch\nExpected: %q\nActual: %q",
				spec.Name, expectedStdout, actualStdout)
		}
	}

	// Compare stderr if specified (exact match)
	if spec.Stderr != "" {
		expectedStderr := spec.Stderr
		if actualStderr != expectedStderr {
			t.Errorf("Test '%s' failed: stderr mismatch\nExpected:\n%s\nActual:\n%s",
				spec.Name, expectedStderr, actualStderr)
		}
	}

	// Check stderr contains specified strings
	for _, expected := range spec.StderrContains {
		if !strings.Contains(actualStderr, expected) {
			t.Errorf("Test '%s' failed: stderr does not contain expected string\nExpected to contain: %q\nActual stderr:\n%s",
				spec.Name, expected, actualStderr)
		}
	}
}

// TestMain clears the object cache before running any e2e tests to ensure fresh builds.
func TestMain(m *testing.M) {
	rootDir, err := filepath.Abs("../..")
	if err == nil {
		for _, cacheDir := range []string{
			filepath.Join(rootDir, "target", "debug", "obj"),
			filepath.Join(rootDir, "target", "release", "obj"),
		} {
			_ = os.RemoveAll(cacheDir)
		}
	}
	os.Exit(m.Run())
}

// TestE2E is the main e2e test function
func TestE2E(t *testing.T) {
	// Build the compiler
	compilerPath := buildCompiler(t)
	defer func() {
		// Clean up the compiler binary
		if err := os.Remove(compilerPath); err != nil {
			t.Logf("Warning: Failed to remove compiler binary: %v", err)
		}
	}()

	// Prepare output directory
	outputDir := "out"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output directory: %v", err)
	}
	defer func() {
		// Clean up output directory
		if err := os.RemoveAll(outputDir); err != nil {
			t.Logf("Warning: Failed to remove output directory: %v", err)
		}
	}()

	// Read all spec files
	specsDir := "./specs"
	allSpecs := readSpecFiles(t, specsDir)

	if len(allSpecs) == 0 {
		t.Fatal("No spec files found")
	}

	// Run tests for each suite
	totalTests := 0
	failedTests := 0

	for suiteName, specs := range allSpecs {
		t.Run(suiteName, func(t *testing.T) {
			suiteDir := filepath.Join(specsDir, suiteName)

			for _, spec := range specs {
				totalTests++
				t.Run(sanitizeTestName(spec.Name), func(t *testing.T) {
					runTestSpec(t, compilerPath, suiteDir, outputDir, spec)
					if t.Failed() {
						failedTests++
					}
				})
			}
		})
	}

	t.Logf("E2E Test Summary: %d total, %d passed, %d failed",
		totalTests, totalTests-failedTests, failedTests)
}

// sanitizeTestName converts a test name to a valid Go test name
func sanitizeTestName(name string) string {
	// Replace spaces and special characters with underscores
	sanitized := strings.ReplaceAll(name, " ", "_")
	sanitized = strings.ReplaceAll(sanitized, "-", "_")
	sanitized = strings.ReplaceAll(sanitized, "(", "_")
	sanitized = strings.ReplaceAll(sanitized, ")", "_")
	sanitized = strings.ReplaceAll(sanitized, ":", "_")

	// Remove multiple consecutive underscores
	for strings.Contains(sanitized, "__") {
		sanitized = strings.ReplaceAll(sanitized, "__", "_")
	}

	// Remove leading/trailing underscores
	sanitized = strings.Trim(sanitized, "_")

	return sanitized
}
