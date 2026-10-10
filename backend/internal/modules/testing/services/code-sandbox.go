package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// CodeExecutionResult — результат одного запуска процесса.
type CodeExecutionResult struct {
	Success         bool   `json:"success"`
	Stdout          string `json:"stdout"`
	Stderr          string `json:"stderr"`
	ExecutionTimeMs int64  `json:"executionTimeMs"`
	TimedOut        bool   `json:"timedOut"`
	MemoryUsedKb    *int64 `json:"memoryUsedKb,omitempty"`
}

// TestCaseResult — результат одного тест-кейса.
type TestCaseResult struct {
	Passed          bool    `json:"passed"`
	Hidden          bool    `json:"hidden"`
	Input           *string `json:"input,omitempty"`
	ExpectedOutput  *string `json:"expectedOutput,omitempty"`
	ActualOutput    *string `json:"actualOutput,omitempty"`
	Error           *string `json:"error,omitempty"`
	ExecutionTimeMs int64   `json:"executionTimeMs"`
}

// SandboxOptions — опции запуска (timeLimitMs/memoryLimitMb; 0 = default).
type SandboxOptions struct {
	TimeLimitMs   int64
	MemoryLimitMb int
}

// CodeExecutionReport — отчёт по прогону тестов.
type CodeExecutionReport struct {
	TotalTests      int              `json:"totalTests"`
	PassedTests     int              `json:"passedTests"`
	FailedTests     int              `json:"failedTests"`
	Results         []TestCaseResult `json:"results"`
	Score           float64          `json:"score"` // 0-100
	ExecutionTimeMs int64            `json:"executionTimeMs"`
	SandboxUsed     bool             `json:"sandboxUsed"`
	Errors          []string         `json:"errors"`
}

// CodeSandbox — исполнение кода кандидата.
//
// Безопасность (MVP):
//   - изоляция процесса (в TS — spawn на хосте);
//   - лимиты времени/вывода.
//
// BUG-REF: T3 — SANDBOX_DOCKER/SANDBOX_MEMORY_MB читаются, но Docker-режим и
// лимит памяти НЕ реализованы; sandboxUsed лишь отражает флаг в отчёте.
type CodeSandbox struct {
	useDocker        bool
	defaultTimeoutMs int64
	defaultMemoryMb  int
}

// NewCodeSandbox — конструктор (значения — из config: SandboxTimeout,
// SandboxMemoryMB, SandboxDocker; в TS читаются из process.env).
func NewCodeSandbox(timeout time.Duration, memoryMB int, useDocker bool) *CodeSandbox {
	return &CodeSandbox{
		useDocker:        useDocker, // BUG-REF: T3 — не реализовано
		defaultTimeoutMs: timeout.Milliseconds(),
		defaultMemoryMb:  memoryMB, // BUG-REF: T3 — не реализовано
	}
}

// ExecuteWithTests — исполнение кода с тест-кейсами.
func (s *CodeSandbox) ExecuteWithTests(ctx context.Context, code, language string, sampleTests []SampleTest, hiddenTestsCount int, options *SandboxOptions) (*CodeExecutionReport, error) {
	startTime := time.Now()
	results := []TestCaseResult{}
	errors := []string{}

	// Generate hidden tests (deterministic but unknown to candidate)
	hiddenTests := s.generateHiddenTests(language, hiddenTestsCount)

	// Run sample tests
	for _, test := range sampleTests {
		result := s.runSingleTest(code, language, test, false, options)
		results = append(results, result)
	}

	// Run hidden tests
	for _, test := range hiddenTests {
		result := s.runSingleTest(code, language, test, true, options)
		results = append(results, result)
	}

	passedTests := 0
	for _, r := range results {
		if r.Passed {
			passedTests++
		}
	}
	totalTests := len(results)
	score := 0.0
	if totalTests > 0 {
		score = JSRound(float64(passedTests) / float64(totalTests) * 100)
	}

	return &CodeExecutionReport{
		TotalTests:      totalTests,
		PassedTests:     passedTests,
		FailedTests:     totalTests - passedTests,
		Results:         results,
		Score:           score,
		ExecutionTimeMs: time.Since(startTime).Milliseconds(),
		SandboxUsed:     s.useDocker, // BUG-REF: T3 — только флаг
		Errors:          errors,
	}, nil
}

// runSingleTest — исполнение одного тест-кейса.
func (s *CodeSandbox) runSingleTest(code, language string, test SampleTest, hidden bool, options *SandboxOptions) (out TestCaseResult) {
	startTime := time.Now()
	timeout := s.defaultTimeoutMs
	if options != nil && options.TimeLimitMs != 0 {
		timeout = options.TimeLimitMs
	}

	// аналог try/catch в TS runSingleTest
	defer func() {
		if rec := recover(); rec != nil {
			out = TestCaseResult{
				Passed:          false,
				Hidden:          hidden,
				Input:           hiddenInput(hidden, test.Input),
				ExpectedOutput:  hiddenExpected(hidden, test.ExpectedOutput),
				Error:           strPtr(fmt.Sprintf("%v", rec)),
				ExecutionTimeMs: time.Since(startTime).Milliseconds(),
			}
		}
	}()

	var result CodeExecutionResult
	switch language {
	case "javascript":
		result = s.executeJavaScript(code, test.Input, timeout)
	case "python":
		result = s.executePython(code, test.Input, timeout)
	case "go":
		result = s.executeGo(code, test.Input, timeout)
	default:
		return TestCaseResult{
			Passed:          false,
			Hidden:          hidden,
			Input:           hiddenInput(hidden, test.Input),
			ExpectedOutput:  hiddenExpected(hidden, test.ExpectedOutput),
			Error:           strPtr(fmt.Sprintf("Unsupported language: %s", language)),
			ExecutionTimeMs: time.Since(startTime).Milliseconds(),
		}
	}

	actualOutput := strings.TrimSpace(result.Stdout)
	expectedOutput := strings.TrimSpace(test.ExpectedOutput)
	passed := !result.TimedOut && actualOutput == expectedOutput

	out = TestCaseResult{
		Passed:          passed,
		Hidden:          hidden,
		Input:           hiddenInput(hidden, test.Input),
		ExpectedOutput:  hiddenExpected(hidden, test.ExpectedOutput),
		ExecutionTimeMs: result.ExecutionTimeMs,
	}
	if !passed {
		out.ActualOutput = strPtr(actualOutput)
	}
	if result.TimedOut {
		out.Error = strPtr("Timeout")
	} else if result.Stderr != "" {
		out.Error = strPtr(result.Stderr)
	}
	return out
}

func hiddenInput(hidden bool, input string) *string {
	if hidden {
		return nil
	}
	return strPtr(input)
}

func hiddenExpected(hidden bool, expected string) *string {
	if hidden {
		return nil
	}
	return strPtr(expected)
}

// executeJavaScript — исполнение JS-кода в изолированном процессе.
func (s *CodeSandbox) executeJavaScript(code, input string, timeoutMs int64) CodeExecutionResult {
	// Wrap code with test harness
	harness := fmt.Sprintf(`
      const __input = %s;
      const __outputs = [];
      const console = {
        log: (...args) => __outputs.push(args.map(String).join(' ')),
        error: (...args) => __outputs.push(args.map(String).join(' ')),
        warn: (...args) => __outputs.push(args.map(String).join(' ')),
      };

      try {
        %s

        // If function is defined, call it with input
        if (typeof solution === 'function') {
          const result = solution(__input);
          console.log(typeof result === 'object' ? JSON.stringify(result) : String(result));
        } else if (typeof solve === 'function') {
          const result = solve(__input);
          console.log(typeof result === 'object' ? JSON.stringify(result) : String(result));
        }

        process.stdout.write(__outputs.join('\n'));
      } catch (e) {
        process.stderr.write(String(e));
        process.exit(1);
      }
    `, jsonString(input), code)

	return s.runProcess("node", []string{"-e", harness}, timeoutMs, nil)
}

// executePython — исполнение Python-кода в изолированном процессе.
func (s *CodeSandbox) executePython(code, input string, timeoutMs int64) CodeExecutionResult {
	harness := fmt.Sprintf(`
import sys
import json

__input = %s

%s

# Call function if defined
try:
    if 'solution' in dir():
        result = solution(__input)
        print(result if not isinstance(result, (dict, list)) else json.dumps(result))
    elif 'solve' in dir():
        result = solve(__input)
        print(result if not isinstance(result, (dict, list)) else json.dumps(result))
except Exception as e:
    print(str(e), file=sys.stderr)
    sys.exit(1)
`, jsonString(input), code)

	return s.runProcess("python3", []string{"-c", harness}, timeoutMs, nil)
}

// executeGo — исполнение Go-кода в изолированном процессе.
func (s *CodeSandbox) executeGo(code, input string, timeoutMs int64) CodeExecutionResult {
	// Go requires compilation, more complex setup
	// For MVP, we'll use a simplified approach
	harness := fmt.Sprintf(`package main

import (
    "encoding/json"
    "fmt"
    "os"
)

func main() {
    var input string
    json.NewDecoder(os.Stdin).Decode(&input)

    %s

    // Call Solution function if defined
    result := Solution(input)
    fmt.Println(result)
}
`, code)

	return s.runProcess("go", []string{"run", "-"}, timeoutMs, &harness)
}

const outputLimit = 1024 * 1024 // лимит вывода 1 МБ (как в TS)

// runProcess — запуск процесса с таймаутом и лимитом вывода.
func (s *CodeSandbox) runProcess(command string, args []string, timeoutMs int64, stdin *string) CodeExecutionResult {
	startTime := time.Now()

	cmd := exec.Command(command, args...)
	// Security: limit environment
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"NODE_ENV=sandbox",
	}
	if stdin != nil {
		cmd.Stdin = strings.NewReader(*stdin)
	}

	var stdout, stderr cappedBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	timedOut := false
	var killOnce sync.Once
	kill := func() {
		killOnce.Do(func() {
			if cmd.Process != nil {
				_ = cmd.Process.Kill() // SIGKILL (в TS: child.kill('SIGKILL'))
			}
		})
	}
	stdout.onOverflow = kill
	stderr.onOverflow = kill

	if err := cmd.Start(); err != nil {
		// аналог события 'error' в TS
		return CodeExecutionResult{
			Success:         false,
			Stdout:          stdout.String(),
			Stderr:          err.Error(),
			ExecutionTimeMs: time.Since(startTime).Milliseconds(),
			TimedOut:        false,
		}
	}

	timer := time.AfterFunc(time.Duration(timeoutMs)*time.Millisecond, func() {
		timedOut = true
		kill()
	})

	waitErr := cmd.Wait()
	timer.Stop()

	exitCode := 0
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return CodeExecutionResult{
				Success:         false,
				Stdout:          stdout.String(),
				Stderr:          waitErr.Error(),
				ExecutionTimeMs: time.Since(startTime).Milliseconds(),
				TimedOut:        timedOut,
			}
		}
	}

	return CodeExecutionResult{
		Success:         exitCode == 0 && !timedOut,
		Stdout:          stdout.String(),
		Stderr:          stderr.String(),
		ExecutionTimeMs: time.Since(startTime).Milliseconds(),
		TimedOut:        timedOut,
	}
}

// cappedBuffer — буфер с лимитом 1 МБ: при превышении процесс убивается
// (в TS: if (stdout.length > 1024 * 1024) child.kill('SIGKILL')).
type cappedBuffer struct {
	mu         sync.Mutex
	buf        strings.Builder
	overflowed bool
	onOverflow func()
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.overflowed {
		return len(p), nil
	}
	if b.buf.Len()+len(p) > outputLimit {
		b.overflowed = true
		rest := outputLimit - b.buf.Len()
		if rest > 0 {
			b.buf.Write(p[:rest])
		}
		if b.onOverflow != nil {
			b.onOverflow()
		}
		return len(p), nil
	}
	b.buf.Write(p)
	return len(p), nil
}

func (b *cappedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// generateHiddenTests — генерация скрытых тестов для языка.
// Детерминированно, но непредсказуемо для кандидата.
// BUG-REF: T2 — заглушка: входы генерируются LCG, но expectedOutput не
// вычисляется («hidden_${i}») — такие тесты практически всегда «падают»,
// поэтому score по code_execution занижен и нестабилен.
func (s *CodeSandbox) generateHiddenTests(language string, count int) []SampleTest {
	// For MVP: generate simple test cases
	// In production: read from database
	tests := []SampleTest{}

	// Example: generate tests for common function signatures
	testGenerators := map[string]func(i int) SampleTest{
		"javascript": func(i int) SampleTest {
			return SampleTest{
				Input:          jsonString(testInputValue(i)),
				ExpectedOutput: "", // Will be computed
			}
		},
		"python": func(i int) SampleTest {
			return SampleTest{
				Input:          jsonString(testInputValue(i)),
				ExpectedOutput: "", // Will be computed
			}
		},
	}

	for i := 0; i < count; i++ {
		generator, ok := testGenerators[language]
		if ok {
			test := generator(i)
			// For hidden tests, we don't know the expected output in advance
			// The actual test harness will compute it
			expected := test.ExpectedOutput
			if expected == "" {
				expected = fmt.Sprintf("hidden_%d", i)
			}
			tests = append(tests, SampleTest{Input: test.Input, ExpectedOutput: expected})
		}
	}

	return tests
}

// testInputValue — детерминированный вход (LCG).
func testInputValue(seed int) any {
	rng := NewLCG(seed)
	typ := int(rng.Float() * 3) // Math.floor(rng() * 3)

	switch typ {
	case 0:
		return int(rng.Float() * 100) // Math.floor(rng() * 100)
	case 1:
		return fmt.Sprintf("test_%d", seed)
	case 2:
		return []int{seed, seed * 2, seed * 3}
	default:
		return seed
	}
}

// jsonString — JSON.stringify(value) для значений входа.
func jsonString(value any) string {
	s, ok := value.(string)
	if ok {
		b, _ := json.Marshal(s)
		return string(b)
	}
	b, err := json.Marshal(value)
	if err != nil {
		return "null"
	}
	return string(b)
}
