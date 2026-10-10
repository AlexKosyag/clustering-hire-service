// cmd/metrics — автоматизированная валидация платформы ФСП
// (порт backend/scripts/metrics.ts, docs/VALIDATION.md).
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	api      = strings.TrimRight(envOr("API_URL", "http://localhost:3000/api"), "/")
	frontend = strings.TrimRight(envOr("FRONTEND_URL", "http://localhost:8080"), "/")
	runAt    = time.Now()
)

func envOr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

// ============ Секции отчёта ============

type check struct {
	Name   string
	Ok     bool
	Detail string
}

type section struct {
	Title  string
	Checks []check
	Extra  string
}

var sections []section

func add(title string, checks []check, extra string) {
	sections = append(sections, section{Title: title, Checks: checks, Extra: extra})
}

// ============ HTTP ============

type httpResult struct {
	Status int
	Body   any
	MS     int64
}

type callOpts struct {
	Method  string
	Token   string
	Body    any
	Headers map[string]string
}

// call — аналог fetch(...) с замером времени (ошибка сети → status=0).
func call(url string, opts callOpts) httpResult {
	started := time.Now()

	method := opts.Method
	if method == "" {
		method = http.MethodGet
	}

	var reader io.Reader
	if opts.Body != nil {
		raw, err := json.Marshal(opts.Body)
		if err != nil {
			return httpResult{Status: 0, Body: map[string]any{"message": err.Error()}, MS: 0}
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return httpResult{Status: 0, Body: map[string]any{"message": err.Error()}}
	}
	req.Header.Set("Content-Type", "application/json")
	if opts.Token != "" {
		req.Header.Set("Authorization", "Bearer "+opts.Token)
	}
	for k, v := range opts.Headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		msg := err.Error()
		return httpResult{Status: 0, Body: map[string]any{"message": msg}, MS: time.Since(started).Milliseconds()}
	}
	defer resp.Body.Close()
	text, err := io.ReadAll(resp.Body)
	if err != nil {
		return httpResult{Status: 0, Body: map[string]any{"message": err.Error()}, MS: time.Since(started).Milliseconds()}
	}

	var body any = nil
	if len(text) > 0 {
		var parsed any
		if json.Unmarshal(text, &parsed) == nil {
			body = parsed
		} else {
			body = string(text)
		}
	}
	return httpResult{Status: resp.StatusCode, Body: body, MS: time.Since(started).Milliseconds()}
}

// ============ Утилиты разбора ответов ============

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

func field(m map[string]any, key string) any {
	if m == nil {
		return nil
	}
	return m[key]
}

func strField(m map[string]any, key string) string {
	if v, ok := field(m, key).(string); ok {
		return v
	}
	return ""
}

func numField(m map[string]any, key string) float64 {
	switch v := field(m, key).(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case json.Number:
		f, _ := v.Float64()
		return f
	}
	return 0
}

// norm — как в metrics.ts: lowercase + удалить [. \s -]+.
var normRe = strings.NewReplacer(".", "", " ", "", "-", "", "\t", "", "\n", "")

func norm(value string) string {
	return normRe.Replace(strings.ToLower(value))
}

func wait(ms int64) { time.Sleep(time.Duration(ms) * time.Millisecond) }

func rnd(min, max int) int {
	return min + rand.Intn(max-min+1) // Math.round(min + Math.random() * (max-min))
}

// stamp — суффикс email: Date.now().toString(36) + 4 случайных символов.
func stamp() string {
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
	suffix := make([]byte, 4)
	for i := range suffix {
		suffix[i] = alphabet[rand.Intn(len(alphabet))]
	}
	return strconv.FormatInt(time.Now().UnixMilli(), 36) + string(suffix)
}

const gradeOrder = "JUNIOR,MIDDLE,MIDDLE_PLUS,SENIOR,LEAD,PRINCIPAL"

func gradeIdx(g string) int {
	if g == "" {
		return -1
	}
	for i, part := range strings.Split(gradeOrder, ",") {
		if part == g {
			return i
		}
	}
	return -1
}

// ============ Регистрация ============

type account struct {
	Email    string
	Password string
	Token    string
	UserID   string
}

func register(role string) (account, error) {
	email := fmt.Sprintf("metrics-%s-%s@example.com", strings.ToLower(role), stamp())
	password := "Metrics123!"
	res := call(api+"/auth/register", callOpts{
		Method: "POST",
		Body:   map[string]any{"email": email, "password": password, "role": role},
	})
	if res.Status != 201 && res.Status != 200 {
		return account{}, fmt.Errorf("register failed: %d", res.Status)
	}
	body := asMap(res.Body)
	user := asMap(field(body, "user"))
	return account{
		Email:    email,
		Password: password,
		Token:    strField(body, "accessToken"),
		UserID:   strField(user, "id"),
	}, nil
}

// ============ Поиск docs/VALIDATION_REPORT.md ============

// reportPath — путь к docs/VALIDATION_REPORT.md: поднимаемся от текущего
// каталога вверх, пока не найдём каталог docs с VALIDATION.md (как
// path.resolve(__dirname, '../../docs/...') в TS).
func reportPath() string {
	dir, err := os.Getwd()
	if err != nil {
		return filepath.Join("docs", "VALIDATION_REPORT.md")
	}
	for i := 0; i < 6; i++ {
		docs := filepath.Join(dir, "docs")
		if _, err := os.Stat(filepath.Join(docs, "VALIDATION.md")); err == nil {
			return filepath.Join(docs, "VALIDATION_REPORT.md")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Join("docs", "VALIDATION_REPORT.md")
}

func main() {
	os.Exit(run())
}

func run() int {
	if err := openStore(); err != nil {
		fmt.Fprintln(os.Stderr, "FATAL:", err)
		return 2
	}
	defer closeStore()

	if err := smokeSection(); err != nil {
		fmt.Fprintln(os.Stderr, "FATAL:", err)
		return 2
	}
	if err := securitySection(); err != nil {
		fmt.Fprintln(os.Stderr, "FATAL:", err)
		return 2
	}
	matching, err := matchingSection()
	if err != nil {
		fmt.Fprintln(os.Stderr, "FATAL:", err)
		return 2
	}
	testing, err := testingSection()
	if err != nil {
		fmt.Fprintln(os.Stderr, "FATAL:", err)
		return 2
	}
	performance, err := performanceSection()
	if err != nil {
		fmt.Fprintln(os.Stderr, "FATAL:", err)
		return 2
	}
	writeReport(matching, testing, performance)

	var failed []check
	for _, s := range sections {
		for _, c := range s.Checks {
			if !c.Ok {
				failed = append(failed, c)
			}
		}
	}
	if len(failed) > 0 {
		fmt.Println("\nПроваленные проверки:")
		for _, f := range failed {
			fmt.Printf(" - %s: %s\n", f.Name, f.Detail)
		}
	}
	if len(failed) > 0 {
		return 1
	}
	return 0
}
