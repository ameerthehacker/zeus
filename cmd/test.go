package cmd

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// Markers printed by the @std/test module. Fields are separated by ASCII Unit Separator (0x1f),
// which won't appear in ordinary suite/test names or assertion messages. Every marker line starts
// with ztestMarker; anything else on stdout is user output and is passed through.
const (
	ztestSep    = "\x1f"
	ztestMarker = "\x1fZTEST\x1f"
)

// eventKind classifies a single rendered line within a file's result block.
type eventKind int

const (
	evSuiteEnter eventKind = iota // a describe(...) header
	evTest                        // an it(...) result
	evOutput                      // passthrough user output (console.log)
)

type event struct {
	kind    eventKind
	depth   int    // suite nesting depth (0 = top level inside the file)
	name    string // suite or test name
	passed  bool   // evTest only
	message string // evTest failure message
	text    string // evOutput only
}

type fileResult struct {
	path       string
	events     []event
	passed     int
	failed     int
	compileErr string // non-empty when `zeus build` failed
	runtimeErr string // non-empty when the test binary crashed (non-zero exit, no failures)
}

func (fr fileResult) failedFile() bool {
	return fr.failed > 0 || fr.compileErr != "" || fr.runtimeErr != ""
}

// progress carries incremental counts from workers to the spinner.
type progress struct {
	passDelta int
	failDelta int
	fileDone  bool
}

func testCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test [path]",
		Short: "Discover and run Zeus test files (*.test.zs, *.spec.zs)",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			root := getCurDir()
			if len(args) == 1 {
				root = args[0]
			}
			os.Exit(runTests(root))
		},
	}
}

// runTests discovers, compiles, and runs every test file under root, prints a Jest-style report,
// and returns the process exit code (non-zero if anything failed).
func runTests(root string) int {
	files, err := discoverTestFiles(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "zeus: error: %s\n", err.Error())
		return 1
	}
	if len(files) == 0 {
		fmt.Println("No test files found (looked for *.test.zs and *.spec.zs).")
		return 0
	}

	self, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "zeus: error: cannot locate the zeus executable: %s\n", err.Error())
		return 1
	}

	tmpDir, err := os.MkdirTemp("", "zeus-test-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "zeus: error: cannot create temp dir: %s\n", err.Error())
		return 1
	}
	defer os.RemoveAll(tmpDir)

	results := make([]fileResult, len(files))
	prog := make(chan progress, 256)
	spinnerDone := make(chan struct{})
	go runSpinner(prog, len(files), spinnerEnabled(), spinnerDone)

	// Bounded worker pool: file-level concurrency. Tests within a file run sequentially inside
	// their compiled binary (concurrent individual tests would need Zeus async/await).
	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 1
	}
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for i := range files {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = runTestFile(self, files[i], tmpDir, prog)
		}(i)
	}
	wg.Wait()
	close(prog)
	<-spinnerDone

	return renderReport(root, files, results)
}

// discoverTestFiles walks root and returns *.test.zs / *.spec.zs paths in sorted order.
// A path to a single file is accepted as-is. Build/vendor and hidden directories are skipped.
func discoverTestFiles(root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if isTestFile(filepath.Base(root)) {
			return []string{root}, nil
		}
		return nil, fmt.Errorf("%s is not a test file (*.test.zs, *.spec.zs)", root)
	}

	var files []string
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // ignore unreadable entries, keep going
		}
		if d.IsDir() {
			name := d.Name()
			if path != root && (name == "target" || name == "node_modules" || name == ".git" || strings.HasPrefix(name, ".")) {
				return fs.SkipDir
			}
			return nil
		}
		if isTestFile(d.Name()) {
			files = append(files, path)
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	sort.Strings(files)
	return files, nil
}

func isTestFile(name string) bool {
	return strings.HasSuffix(name, ".test.zs") || strings.HasSuffix(name, ".spec.zs")
}

// runTestFile compiles one test file to a temp binary and runs it, streaming the result markers
// (so the spinner reflects live per-test progress) and collecting them into a fileResult.
func runTestFile(self, path, tmpDir string, prog chan<- progress) fileResult {
	fr := fileResult{path: path}

	// Isolate each file's obj cache + binary so parallel builds don't race on the cache dir.
	sum := sha256.Sum256([]byte(path))
	fileDir := filepath.Join(tmpDir, hex.EncodeToString(sum[:8]))
	if err := os.MkdirAll(fileDir, 0o755); err != nil {
		fr.runtimeErr = err.Error()
		prog <- progress{fileDone: true}
		return fr
	}
	bin := filepath.Join(fileDir, "test.out")

	// Compile via a `zeus build` subprocess: Compiler.Compile calls os.Exit on error, so we must
	// not call it in-process, and a subprocess isolates a bad file from the rest of the run.
	buildCmd := exec.Command(self, "build", path, "-o", bin, "--target-dir", fileDir)
	if out, err := buildCmd.CombinedOutput(); err != nil {
		fr.compileErr = strings.TrimRight(string(out), "\n")
		if fr.compileErr == "" {
			fr.compileErr = err.Error()
		}
		prog <- progress{fileDone: true}
		return fr
	}

	// Run, streaming stdout line by line.
	runCmd := exec.Command(bin)
	stdout, err := runCmd.StdoutPipe()
	if err != nil {
		fr.runtimeErr = err.Error()
		prog <- progress{fileDone: true}
		return fr
	}
	var stderrBuf bytes.Buffer
	runCmd.Stderr = &stderrBuf
	if err := runCmd.Start(); err != nil {
		fr.runtimeErr = err.Error()
		prog <- progress{fileDone: true}
		return fr
	}

	depth := 0
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, ztestMarker) {
			fr.events = append(fr.events, event{kind: evOutput, depth: depth, text: line})
			continue
		}
		parts := strings.SplitN(line, ztestSep, 5)
		// parts[0]="", parts[1]="ZTEST", parts[2]=kind, parts[3]=name, parts[4]=message
		if len(parts) < 3 {
			continue
		}
		kind := parts[2]
		name := ""
		if len(parts) >= 4 {
			name = parts[3]
		}
		switch kind {
		case "ENTER":
			fr.events = append(fr.events, event{kind: evSuiteEnter, depth: depth, name: name})
			depth++
		case "EXIT":
			if depth > 0 {
				depth--
			}
		case "PASS":
			fr.events = append(fr.events, event{kind: evTest, depth: depth, name: name, passed: true})
			fr.passed++
			prog <- progress{passDelta: 1}
		case "FAIL":
			msg := ""
			if len(parts) >= 5 {
				msg = parts[4]
			}
			fr.events = append(fr.events, event{kind: evTest, depth: depth, name: name, passed: false, message: msg})
			fr.failed++
			prog <- progress{failDelta: 1}
		}
	}

	// If the scanner stopped early (a line longer than the buffer cap, or a read error), drain the
	// rest of stdout so the child never blocks on a full pipe — otherwise Wait() below can deadlock.
	scanErr := scanner.Err()
	if scanErr != nil {
		_, _ = io.Copy(io.Discard, stdout)
	}

	if waitErr := runCmd.Wait(); waitErr != nil {
		// The binary normally exits 0 (assertion throws are caught inside `it`); any non-zero exit
		// means it crashed / threw outside a test, so always surface it — even if some tests failed.
		msg := strings.TrimSpace(stderrBuf.String())
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			fr.runtimeErr = fmt.Sprintf("test binary exited with code %d", exitErr.ExitCode())
		} else {
			fr.runtimeErr = waitErr.Error()
		}
		if msg != "" {
			fr.runtimeErr += "\n" + msg
		}
	}
	if scanErr != nil && fr.runtimeErr == "" {
		fr.runtimeErr = "error reading test output: " + scanErr.Error()
	}

	prog <- progress{fileDone: true}
	return fr
}

// runSpinner renders a live progress line to stderr from the workers' incremental counts. When
// disabled (non-TTY or NO_COLOR) it just drains the channel so workers never block.
func runSpinner(prog <-chan progress, totalFiles int, enabled bool, done chan<- struct{}) {
	defer close(done)
	if !enabled {
		for range prog {
		}
		return
	}

	frames := []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	var tests, failing, filesDone, fi int
	draw := func() {
		fmt.Fprintf(os.Stderr, "\r\033[K%c  %d tests · %d failing · %d/%d files",
			frames[fi], tests, failing, filesDone, totalFiles)
	}
	for {
		select {
		case p, ok := <-prog:
			if !ok {
				fmt.Fprint(os.Stderr, "\r\033[K") // clear the line before the report prints
				return
			}
			tests += p.passDelta + p.failDelta
			failing += p.failDelta
			if p.fileDone {
				filesDone++
			}
		case <-ticker.C:
			fi = (fi + 1) % len(frames)
			draw()
		}
	}
}

// renderReport prints the per-file result blocks (in discovery order) and the summary, and returns
// the process exit code.
func renderReport(root string, files []string, results []fileResult) int {
	var b strings.Builder
	var testsPassed, testsFailed, filesPassed, filesFailed int

	for _, fr := range results {
		rel := relDisplayPath(root, fr.path)
		// Counts come from runTestFile's per-file tallies (single source of truth); the event
		// loop below is purely for rendering.
		testsPassed += fr.passed
		testsFailed += fr.failed

		if fr.compileErr != "" {
			filesFailed++
			b.WriteString(cBadgeFail.Sprint(" FAIL ") + " " + rel + cDim.Sprint(" · compile error") + "\n")
			b.WriteString(indentText(stripANSIIfNoColor(fr.compileErr), "        ") + "\n")
			continue
		}

		if fr.failedFile() {
			filesFailed++
			b.WriteString(cBadgeFail.Sprint(" FAIL ") + " " + rel + "\n")
		} else {
			filesPassed++
			b.WriteString(cBadgePass.Sprint(" PASS ") + " " + rel + "\n")
		}

		for _, ev := range fr.events {
			indent := strings.Repeat("  ", ev.depth+1)
			switch ev.kind {
			case evSuiteEnter:
				b.WriteString(indent + cSuiteName.Sprint(ev.name) + "\n")
			case evTest:
				if ev.passed {
					b.WriteString(indent + cCheck.Sprint("✓") + " " + ev.name + "\n")
				} else {
					line := indent + cCross.Sprint("✗") + " " + ev.name
					if ev.message != "" {
						line += cDim.Sprint(" — " + ev.message)
					}
					b.WriteString(line + "\n")
				}
			case evOutput:
				if strings.TrimSpace(ev.text) != "" {
					b.WriteString(indent + cDim.Sprint(ev.text) + "\n")
				}
			}
		}

		if fr.runtimeErr != "" {
			b.WriteString("        " + cRuntime.Sprint("runtime error: ") + firstLine(stripANSIIfNoColor(fr.runtimeErr)) + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(summaryLine("Test Files", filesPassed, filesFailed, len(files)))
	b.WriteString(summaryLine("Tests", testsPassed, testsFailed, testsPassed+testsFailed))

	fmt.Print(b.String())

	if testsFailed > 0 || filesFailed > 0 {
		return 1
	}
	return 0
}

func summaryLine(label string, passed, failed, total int) string {
	parts := []string{}
	if failed > 0 {
		parts = append(parts, cFailCount.Sprintf("%d failed", failed))
	}
	if passed > 0 {
		parts = append(parts, cPassCount.Sprintf("%d passed", passed))
	}
	parts = append(parts, fmt.Sprintf("%d total", total))
	return fmt.Sprintf("%s  %s\n", cDim.Sprintf("%11s", label), strings.Join(parts, cDim.Sprint(", ")))
}

// --- small presentation helpers -------------------------------------------------------------

// Styles built on fatih/color (already a dependency, used by internal/logger). color.NoColor is
// set once at startup from NO_COLOR and stdout's TTY state, so every Sprint below degrades to
// plain text automatically — no manual gating needed.
var (
	cSuiteName = color.New(color.Bold)
	cCheck     = color.New(color.FgGreen)
	cCross     = color.New(color.FgRed)
	cDim       = color.New(color.Faint)
	cRuntime   = color.New(color.FgRed)
	cBadgePass = color.New(color.BgGreen, color.FgBlack, color.Bold)
	cBadgeFail = color.New(color.BgRed, color.FgWhite, color.Bold)
	cPassCount = color.New(color.FgGreen, color.Bold)
	cFailCount = color.New(color.FgRed, color.Bold)
)

// spinnerEnabled reports whether stderr is an interactive terminal (and color is allowed), so we
// don't spew spinner control codes into pipes / CI logs.
func spinnerEnabled() bool {
	if color.NoColor {
		return false
	}
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func relDisplayPath(root, path string) string {
	// When root is a single file, Rel(root, path) is "." — fall back to the base name. Likewise
	// for anything that escapes root ("..") or fails to relativize.
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return filepath.Base(path)
	}
	return rel
}

// ansiSGR matches ANSI SGR (color) escape sequences like "\x1b[31m" or truecolor "\x1b[38;2;…m".
var ansiSGR = regexp.MustCompile("\x1b\\[[0-9;]*m")

// stripANSIIfNoColor removes color escapes from relayed subprocess output (the Zeus compiler and
// runtime colorize regardless of NO_COLOR) so `NO_COLOR=1 zeus test` output stays plain.
func stripANSIIfNoColor(s string) string {
	if !color.NoColor {
		return s
	}
	return ansiSGR.ReplaceAllString(s, "")
}

func indentText(text, indent string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for i, l := range lines {
		lines[i] = indent + l
	}
	return strings.Join(lines, "\n")
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
