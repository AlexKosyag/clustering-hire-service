// Package e2e — порт backend/test/app.e2e.spec.ts: реальное приложение (gin)
// на httptest-сервере + реальная БД из окружения (DATABASE_URL).
//
// Запуск: go test ./test/e2e/ -count=1 -timeout 5m
// Каждый прогон регистрирует свежие аккаунты — тест идемпотентен и не портит
// демо-данные сидера (как в TS).
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"fsp-backend/internal/app"
	"fsp-backend/internal/config"
)

var server *httptest.Server

func TestMain(m *testing.M) {
	cfg := config.Load()
	application, err := app.New(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e: не удалось поднять приложение:", err)
		os.Exit(1)
	}
	server = httptest.NewServer(application.Engine)
	code := m.Run()
	server.Close()
	application.Close()
	os.Exit(code)
}

// ============ HTTP-хелперы (аналог supertest) ============

type response struct {
	Status int
	Body   map[string]any
	Raw    string
	MS     int64
}

func call(t *testing.T, method, path string, body any, token string) response {
	t.Helper()
	start := time.Now()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, server.URL+path, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	var decoded map[string]any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &decoded)
	}
	return response{Status: resp.StatusCode, Body: decoded, Raw: string(raw), MS: time.Since(start).Milliseconds()}
}

func expectStatus(t *testing.T, r response, method, path string, want int) {
	t.Helper()
	if r.Status != want {
		t.Fatalf("%s %s: status %d, ожидалось %d; body: %s", method, path, r.Status, want, r.Raw)
	}
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
	if v, ok := field(m, key).(float64); ok {
		return v
	}
	return 0
}

func boolField(m map[string]any, key string) bool {
	if v, ok := field(m, key).(bool); ok {
		return v
	}
	return false
}

func arrField(m map[string]any, key string) []any {
	if v, ok := field(m, key).([]any); ok {
		return v
	}
	return nil
}

func stamp() string {
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
	suffix := make([]byte, 4)
	for i := range suffix {
		suffix[i] = alphabet[int(time.Now().UnixNano()+int64(i*7919))%len(alphabet)]
		if suffix[i] < 0 {
			suffix[i] = -suffix[i]
		}
	}
	return fmt.Sprintf("%s%s", strings.ToLower(fmt.Sprintf("%x", time.Now().UnixNano())), string(suffix))
}

type tokens struct {
	AccessToken  string
	RefreshToken string
	Email        string
	UserID       string
	Role         string
}

// ============ Тесты ============

func TestE2E(t *testing.T) {
	var (
		candidate          tokens
		employer           tokens
		candidateProfileID string
		invitationID       string
		candidateEmail     = "e2e-candidate-" + stamp() + "@example.com"
		sessionID          string
	)

	// ---- Health и доступность ----

	t.Run("GET /api/health — ok, pgvector-размерность 384", func(t *testing.T) {
		r := call(t, "GET", "/api/health", nil, "")
		expectStatus(t, r, "GET", "/api/health", 200)
		if strField(r.Body, "status") != "ok" {
			t.Errorf("status = %v, ожидался ok", field(r.Body, "status"))
		}
		services := asMap(field(r.Body, "services"))
		if strField(services, "database") != "up" {
			t.Errorf("services.database = %v, ожидался up", field(services, "database"))
		}
		embedding := asMap(field(services, "embedding"))
		if numField(embedding, "dim") != 384 {
			t.Errorf("services.embedding.dim = %v, ожидалась 384", field(embedding, "dim"))
		}
	})

	t.Run("незащищённые маршруты доступны без токена", func(t *testing.T) {
		expectStatus(t, call(t, "GET", "/api/auth/me", nil, ""), "GET", "/api/auth/me", 401)
		expectStatus(t, call(t, "GET", "/api/candidates/profile", nil, ""), "GET", "/api/candidates/profile", 401)
	})

	t.Run("некорректный JSON/payload → 4xx, а не 500", func(t *testing.T) {
		r := call(t, "POST", "/api/auth/register", map[string]any{
			"email": "not-an-email", "password": "1", "role": "CANDIDATE",
		}, "")
		expectStatus(t, r, "POST", "/api/auth/register", 400)
		if field(r.Body, "message") == nil {
			t.Error("message отсутствует в ответе")
		}
	})

	// ---- Auth: регистрация, вход, роли ----

	t.Run("регистрирует кандидата и выдаёт токены", func(t *testing.T) {
		r := call(t, "POST", "/api/auth/register", map[string]any{
			"email": candidateEmail, "password": "E2eTest1234!", "role": "CANDIDATE",
		}, "")
		expectStatus(t, r, "POST", "/api/auth/register", 201)
		candidate = tokens{
			AccessToken:  strField(r.Body, "accessToken"),
			RefreshToken: strField(r.Body, "refreshToken"),
			Email:        candidateEmail,
		}
		if candidate.AccessToken == "" || candidate.RefreshToken == "" {
			t.Fatalf("токены не выданы: %s", r.Raw)
		}
		user := asMap(field(r.Body, "user"))
		if strField(user, "role") != "CANDIDATE" {
			t.Errorf("user.role = %v, ожидался CANDIDATE", field(user, "role"))
		}
		candidate.UserID = strField(user, "id")
	})

	t.Run("дубликат email → 409", func(t *testing.T) {
		expectStatus(t, call(t, "POST", "/api/auth/register", map[string]any{
			"email": candidateEmail, "password": "E2eTest1234!", "role": "CANDIDATE",
		}, ""), "POST", "/api/auth/register", 409)
	})

	t.Run("вход по паролю возвращает тот же профиль", func(t *testing.T) {
		r := call(t, "POST", "/api/auth/login", map[string]any{
			"email": candidateEmail, "password": "E2eTest1234!",
		}, "")
		expectStatus(t, r, "POST", "/api/auth/login", 200)
		user := asMap(field(r.Body, "user"))
		if strField(user, "email") != candidateEmail {
			t.Errorf("user.email = %v, ожидался %s", field(user, "email"), candidateEmail)
		}
		if strField(r.Body, "accessToken") == "" {
			t.Error("accessToken отсутствует")
		}
	})

	t.Run("неверный пароль → отказ", func(t *testing.T) {
		r := call(t, "POST", "/api/auth/login", map[string]any{
			"email": candidateEmail, "password": "WrongPassword1!",
		}, "")
		if r.Status < 400 || r.Status >= 500 {
			t.Errorf("status = %d, ожидался 4xx", r.Status)
		}
	})

	t.Run("refresh выдаёт новую пару токенов", func(t *testing.T) {
		r := call(t, "POST", "/api/auth/refresh", map[string]any{
			"refreshToken": candidate.RefreshToken,
		}, "")
		expectStatus(t, r, "POST", "/api/auth/refresh", 200)
		if strField(r.Body, "accessToken") == "" {
			t.Error("accessToken отсутствует")
		}
		candidate.AccessToken = strField(r.Body, "accessToken")
	})

	t.Run("GET /api/auth/me возвращает пользователя", func(t *testing.T) {
		r := call(t, "GET", "/api/auth/me", nil, candidate.AccessToken)
		expectStatus(t, r, "GET", "/api/auth/me", 200)
		if strField(r.Body, "email") != candidateEmail {
			t.Errorf("email = %v, ожидался %s", field(r.Body, "email"), candidateEmail)
		}
	})

	// ---- Кандидат: профиль ----

	t.Run("профиль создаётся автоматически при регистрации", func(t *testing.T) {
		r := call(t, "GET", "/api/candidates/profile", nil, candidate.AccessToken)
		expectStatus(t, r, "GET", "/api/candidates/profile", 200)
		candidateProfileID = strField(r.Body, "id")
		if candidateProfileID == "" {
			t.Fatal("id профиля отсутствует")
		}
		if _, ok := r.Body["declaredGrade"]; !ok {
			t.Error("declaredGrade отсутствует в ответе")
		}
		if _, ok := field(r.Body, "stack").([]any); !ok {
			t.Error("stack не массив")
		}
	})

	t.Run("обновление профиля сохраняет стек и специализацию", func(t *testing.T) {
		r := call(t, "PUT", "/api/candidates/profile", map[string]any{
			"firstName":      "E2E",
			"lastName":       "Tester",
			"specialization": "BACKEND",
			"declaredGrade":  "MIDDLE",
			"stack":          []string{"Node.js", "PostgreSQL", "Docker"},
			"city":           "Москва",
			"bio":            "E2E-кандидат для проверки API",
		}, candidate.AccessToken)
		expectStatus(t, r, "PUT", "/api/candidates/profile", 200)
		stack := toStringSlice(field(r.Body, "stack"))
		want := []string{"Node.js", "PostgreSQL", "Docker"}
		if strings.Join(stack, ",") != strings.Join(want, ",") {
			t.Errorf("stack = %v, ожидалось %v", stack, want)
		}
		if strField(r.Body, "specialization") != "BACKEND" {
			t.Errorf("specialization = %v, ожидалась BACKEND", field(r.Body, "specialization"))
		}
	})

	// ---- Разграничение прав (roles) ----

	t.Run("работодатель не берёт профиль кандидата", func(t *testing.T) {
		r := call(t, "POST", "/api/auth/login", map[string]any{
			"email": "test@employer.com", "password": "Test123!",
		}, "")
		expectStatus(t, r, "POST", "/api/auth/login", 200)
		user := asMap(field(r.Body, "user"))
		if strField(user, "role") != "EMPLOYER" {
			t.Fatalf("user.role = %v, ожидался EMPLOYER (проверьте сид)", field(user, "role"))
		}
		employer = tokens{
			AccessToken:  strField(r.Body, "accessToken"),
			RefreshToken: strField(r.Body, "refreshToken"),
			Email:        "test@employer.com",
		}
		expectStatus(t, call(t, "GET", "/api/candidates/profile", nil, employer.AccessToken),
			"GET", "/api/candidates/profile", 403)
	})

	t.Run("кандидат не берёт работодательские маршруты", func(t *testing.T) {
		expectStatus(t, call(t, "GET", "/api/employers/dashboard", nil, candidate.AccessToken),
			"GET", "/api/employers/dashboard", 403)
	})

	// ---- Matching Engine ----

	t.Run("поиск кандидатов: hard-фильтры, скоринг, объяснение", func(t *testing.T) {
		r := call(t, "POST", "/api/matching/search", map[string]any{
			"description":    "Ищем backend-разработчика на Node.js: REST API, PostgreSQL, Redis, очереди, высокая нагрузка",
			"specialization": "BACKEND",
			"grade":          "MIDDLE",
			"stack":          []string{"Node.js", "PostgreSQL", "Redis", "Docker"},
			"limit":          10,
		}, employer.AccessToken)
		expectStatus(t, r, "POST", "/api/matching/search", 201)
		// метрика VALIDATION.md: поиск < 500 мс
		if r.MS >= 500 {
			t.Errorf("поиск занял %d мс, цель < 500 мс", r.MS)
		}
		if numField(r.Body, "total") <= 0 {
			t.Errorf("total = %v, ожидалось > 0", field(r.Body, "total"))
		}
		candidates := asMaps(field(r.Body, "candidates"))
		if len(candidates) == 0 {
			t.Fatal("candidates пуст")
		}
		prev := 101.0
		for _, item := range candidates {
			if strField(item, "specialization") != "BACKEND" {
				t.Errorf("кандидат %v не прошёл hard-фильтр", field(item, "specialization"))
			}
			score := numField(item, "matchScore")
			if score < 0 || score > 100 {
				t.Errorf("matchScore = %v вне [0,100]", score)
			}
			explanation := asMap(field(item, "matchExplanation"))
			if len(asMaps(field(explanation, "details"))) == 0 {
				t.Error("matchExplanation.details пуст")
			}
			if score > prev {
				t.Errorf("выдача не отсортирована по убыванию score: %v после %v", score, prev)
			}
			prev = score
		}
	})

	t.Run("semantic-поиск возвращает кандидатов с косинусной близостью", func(t *testing.T) {
		r := call(t, "POST", "/api/matching/semantic", map[string]any{
			"query": "разработчик с опытом работы с Redis и очередями сообщений", "limit": 5,
		}, employer.AccessToken)
		expectStatus(t, r, "POST", "/api/matching/semantic", 201)
		candidates := asMaps(field(r.Body, "candidates"))
		if len(candidates) == 0 {
			t.Fatal("candidates пуст")
		}
		first := candidates[0]
		similarity := numField(first, "similarity")
		if similarity < 0 || similarity > 1 {
			t.Errorf("similarity = %v, ожидалось в [0,1]", similarity)
		}
		if _, ok := first["reason"].(string); !ok {
			t.Error("reason отсутствует или не строка")
		}
	})

	t.Run("рекомендации вакансий для работодателя не падают", func(t *testing.T) {
		expectStatus(t, call(t, "GET", "/api/matching/recommendations", nil, employer.AccessToken),
			"GET", "/api/matching/recommendations", 200)
	})

	t.Run("поиск без указания специализации работает и не выдаёт NaN", func(t *testing.T) {
		r := call(t, "POST", "/api/matching/search", map[string]any{
			"description": "нужен инженер для поддержки продакшена", "limit": 5,
		}, employer.AccessToken)
		expectStatus(t, r, "POST", "/api/matching/search", 201)
		for _, item := range asMaps(field(r.Body, "candidates")) {
			score := numField(item, "matchScore")
			if score != score { // NaN
				t.Errorf("matchScore = NaN")
			}
		}
	})

	// ---- Testing Engine: полный цикл CAT-теста ----

	t.Run("старт теста создаёт сессию с 20 вопросами", func(t *testing.T) {
		r := call(t, "POST", "/api/testing/start", map[string]any{
			"specialization": "BACKEND", "declaredGrade": "MIDDLE",
		}, candidate.AccessToken)
		expectStatus(t, r, "POST", "/api/testing/start", 201)
		sessionID = strField(r.Body, "sessionId")
		if sessionID == "" {
			t.Fatal("sessionId отсутствует")
		}
		if numField(r.Body, "totalQuestions") != 20 {
			t.Errorf("totalQuestions = %v, ожидалось 20", field(r.Body, "totalQuestions"))
		}
		if numField(r.Body, "timeLimitMinutes") <= 0 {
			t.Errorf("timeLimitMinutes = %v, ожидалось > 0", field(r.Body, "timeLimitMinutes"))
		}
	})

	t.Run("текущий вопрос не утекает ответы (no correctAnswer/isCorrect/explanation)", func(t *testing.T) {
		r := call(t, "GET", "/api/testing/session/"+sessionID+"/current", nil, candidate.AccessToken)
		expectStatus(t, r, "GET", "/api/testing/session/"+sessionID+"/current", 200)
		question := asMap(field(r.Body, "question"))
		if question == nil {
			t.Fatal("question отсутствует")
		}
		if strField(question, "id") == "" {
			t.Error("question.id отсутствует")
		}
		for _, leak := range []string{`"correctAnswer"`, `"isCorrect"`, `"explanation"`} {
			if strings.Contains(r.Raw, leak) {
				t.Errorf("утечка %s в ответе /current", leak)
			}
		}
		if strField(question, "questionType") == "multiple_choice" && len(asMaps(field(question, "options"))) < 2 {
			t.Error("у multiple_choice менее 2 вариантов")
		}
	})

	t.Run("повторный старт при активной сессии → 409", func(t *testing.T) {
		expectStatus(t, call(t, "POST", "/api/testing/start", map[string]any{
			"specialization": "BACKEND",
		}, candidate.AccessToken), "POST", "/api/testing/start", 409)
	})

	t.Run("ответы принимаются и накапливаются, завершение выдаёт результат", func(t *testing.T) {
		answered := 0
		for i := 0; i < 20; i++ {
			current := call(t, "GET", "/api/testing/session/"+sessionID+"/current", nil, candidate.AccessToken)
			expectStatus(t, current, "GET", "/api/testing/session/"+sessionID+"/current", 200)
			question := asMap(field(current.Body, "question"))
			if question == nil {
				break
			}
			body := map[string]any{
				"questionId":       field(question, "id"),
				"timeSpentSeconds": 8,
			}
			if strField(question, "questionType") == "multiple_choice" {
				options := asMaps(field(question, "options"))
				body["answer"] = field(options[i%len(options)], "id")
			} else {
				body["answerText"] = "Использую SELECT JOIN транзакции валидацию тесты документацию try catch"
			}
			expectStatus(t, call(t, "POST", "/api/testing/session/"+sessionID+"/answer", body, candidate.AccessToken),
				"POST", "/api/testing/session/"+sessionID+"/answer", 201)
			answered++
		}
		if answered <= 10 {
			t.Errorf("отвечено %d вопросов, ожидалось > 10", answered)
		}

		r := call(t, "POST", "/api/testing/session/"+sessionID+"/complete", map[string]any{}, candidate.AccessToken)
		expectStatus(t, r, "POST", "/api/testing/session/"+sessionID+"/complete", 201)
		score := numField(r.Body, "score")
		if score < 0 || score > 100 {
			t.Errorf("score = %v, ожидалось в [0,100]", score)
		}
		recommendation := strField(r.Body, "recommendation")
		if recommendation != "PASS" && recommendation != "FAIL" && recommendation != "UPGRADE" {
			t.Errorf("recommendation = %v, ожидался PASS|FAIL|UPGRADE", recommendation)
		}
		theta := numField(r.Body, "theta")
		if theta < -4 || theta > 4 {
			t.Errorf("theta = %v, ожидалось в [-4,4]", theta)
		}
		if field(r.Body, "antiCheat") == nil {
			t.Error("antiCheat отсутствует")
		}
		if strField(r.Body, "cooldownUntil") == "" {
			t.Error("cooldownUntil отсутствует")
		}
	})

	t.Run("после теста повторный старт блокируется cooldown", func(t *testing.T) {
		r := call(t, "POST", "/api/testing/start", map[string]any{
			"specialization": "BACKEND",
		}, candidate.AccessToken)
		if r.Status != 400 {
			t.Errorf("status = %d, ожидался 400", r.Status)
		}
	})

	// ---- Вакансии ----

	t.Run("список вакансий содержит данные сидера (30+)", func(t *testing.T) {
		r := call(t, "GET", "/api/vacancies?limit=5&onlyActive=false", nil, "")
		expectStatus(t, r, "GET", "/api/vacancies", 200)
		if numField(r.Body, "total") < 30 {
			t.Errorf("total = %v, ожидалось ≥ 30 (проверьте сид)", field(r.Body, "total"))
		}
		vacancies := asMaps(field(r.Body, "vacancies"))
		if len(vacancies) == 0 || strField(vacancies[0], "title") == "" {
			t.Error("vacancies[0].title отсутствует")
		}
	})

	t.Run("детальная вакансия возвращается по id", func(t *testing.T) {
		list := call(t, "GET", "/api/vacancies?limit=1", nil, "")
		expectStatus(t, list, "GET", "/api/vacancies", 200)
		vacancies := asMaps(field(list.Body, "vacancies"))
		if len(vacancies) == 0 {
			t.Fatal("нет вакансий")
		}
		id := strField(vacancies[0], "id")
		expectStatus(t, call(t, "GET", "/api/vacancies/"+id, nil, ""), "GET", "/api/vacancies/"+id, 200)
	})

	// ---- Приглашения и приватность контактов ----

	t.Run("создание приглашения работодателем, контакты скрыты", func(t *testing.T) {
		r := call(t, "POST", "/api/invitations", map[string]any{
			"candidateId": candidateProfileID,
			"message":     "Приглашаем обсудить вакансию backend-разработчика",
			"salaryFrom":  250000,
			"salaryTo":    350000,
		}, employer.AccessToken)
		expectStatus(t, r, "POST", "/api/invitations", 201)
		invitationID = strField(r.Body, "id")
		if invitationID == "" {
			t.Fatal("id приглашения отсутствует")
		}

		detail := call(t, "GET", "/api/invitations/"+invitationID, nil, employer.AccessToken)
		expectStatus(t, detail, "GET", "/api/invitations/"+invitationID, 200)
		c := asMap(field(detail.Body, "candidate"))
		if !boolField(c, "contactsLocked") {
			t.Error("candidate.contactsLocked = false, ожидалось true")
		}
		if field(c, "contacts") != nil {
			t.Errorf("candidate.contacts = %v, ожидался null", field(c, "contacts"))
		}
	})

	t.Run("поиск кандидатов не раскрывает email (privacy by design)", func(t *testing.T) {
		r := call(t, "GET", "/api/employers/candidates/search?specialization=BACKEND&limit=3", nil, employer.AccessToken)
		expectStatus(t, r, "GET", "/api/employers/candidates/search", 200)
		candidates := asMaps(field(r.Body, "candidates"))
		if len(candidates) == 0 {
			t.Fatal("candidates пуст")
		}
		if _, exists := candidates[0]["email"]; exists {
			t.Error("email кандидата утёк в выдаче")
		}
	})

	t.Run("принятие приглашения раскрывает контакты только приглашённому", func(t *testing.T) {
		mine := call(t, "GET", "/api/invitations", nil, candidate.AccessToken)
		expectStatus(t, mine, "GET", "/api/invitations", 200)
		if len(asMaps(field(mine.Body, "invitations"))) == 0 {
			t.Fatal("у кандидата нет приглашений")
		}

		expectStatus(t, call(t, "POST", "/api/invitations/"+invitationID+"/accept", map[string]any{}, candidate.AccessToken),
			"POST", "/api/invitations/"+invitationID+"/accept", 200)

		detail := call(t, "GET", "/api/invitations/"+invitationID, nil, employer.AccessToken)
		expectStatus(t, detail, "GET", "/api/invitations/"+invitationID, 200)
		if strField(detail.Body, "status") != "ACCEPTED" {
			t.Errorf("status = %v, ожидался ACCEPTED", field(detail.Body, "status"))
		}
		c := asMap(field(detail.Body, "candidate"))
		contacts := asMap(field(c, "contacts"))
		if strField(contacts, "email") != candidateEmail {
			t.Errorf("contacts.email = %v, ожидался %s", field(contacts, "email"), candidateEmail)
		}
	})

	// ---- FSP mock, уведомления, документация ----

	t.Run("FSP-каталог доступен", func(t *testing.T) {
		r := call(t, "GET", "/api/fsp/catalog", nil, candidate.AccessToken)
		expectStatus(t, r, "GET", "/api/fsp/catalog", 200)
		if r.Body == nil {
			t.Error("пустое тело ответа")
		}
	})

	t.Run("уведомления кандидата содержат записи", func(t *testing.T) {
		r := call(t, "GET", "/api/notifications", nil, candidate.AccessToken)
		expectStatus(t, r, "GET", "/api/notifications", 200)
		if numField(r.Body, "total") <= 0 {
			t.Errorf("total = %v, ожидались записи", field(r.Body, "total"))
		}
	})

	t.Run("Swagger отдаётся по /api/docs", func(t *testing.T) {
		r := call(t, "GET", "/api/docs-json", nil, "")
		expectStatus(t, r, "GET", "/api/docs-json", 200)
		if strField(r.Body, "openapi") == "" {
			t.Error("openapi отсутствует")
		}
		if len(asMap(field(r.Body, "paths"))) <= 20 {
			t.Errorf("путей в спецификации: %d, ожидалось > 20", len(asMap(field(r.Body, "paths"))))
		}
	})
}

// ============ Утилиты разбора ============

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

func asMaps(v any) []map[string]any {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func toStringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
