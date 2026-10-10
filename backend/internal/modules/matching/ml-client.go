package matching

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"fsp-backend/internal/config"
)

// Порт backend/src/modules/matching/ml-client.service.ts (as-is, включая баги).
// Client for Python ML Service (FastAPI): embeddings, cross-encoder re-ranking,
// NER + query parsing, TF-IDF fallback when ML unavailable.

// ============ Типы ответов ML-сервиса (интерфейсы TS) ============

// ParsedQuery — результат разбора запроса ML-сервисом / локальным fallback-ом.
type ParsedQuery struct {
	Specialization  *string  `json:"specialization"`
	Seniority       *string  `json:"seniority"`
	RequiredSkills  []string `json:"requiredSkills"`
	PreferredSkills []string `json:"preferredSkills"`
	Context         *string  `json:"context"`
	Intent          string   `json:"intent"`
}

// QueryUnderstanding — полный результат parseQuery: разбор + эмбеддинг.
type QueryUnderstanding struct {
	OriginalQuery     string      `json:"originalQuery"`
	Parsed            ParsedQuery `json:"parsed"`
	Embedding         []float64   `json:"embedding"`
	EmbeddingProvider string      `json:"embeddingProvider"`
	LatencyMs         int64       `json:"latencyMs"`
}

// RerankResult — элемент переранжирования.
type RerankResult struct {
	Index    int     `json:"index"`
	Score    float64 `json:"score"`
	Document string  `json:"document"`
}

// NEREntity — сущность NER (в ответе ML есть start/end; локальный fallback их не отдаёт).
type NEREntity struct {
	Text  string `json:"text"`
	Label string `json:"label"`
	Start *int   `json:"start,omitempty"`
	End   *int   `json:"end,omitempty"`
}

// Внутренние типы ответов HTTP API.
type healthResponse struct {
	Status string `json:"status"`
	Models struct {
		Embedding struct {
			Ready bool   `json:"ready"`
			Name  string `json:"name"`
			Dim   int    `json:"dim"`
		} `json:"embedding"`
		Reranker struct {
			Ready bool   `json:"ready"`
			Name  string `json:"name"`
		} `json:"reranker"`
		Ner struct {
			Ready bool `json:"ready"`
		} `json:"ner"`
	} `json:"models"`
}

type embedResponse struct {
	Embedding []float64 `json:"embedding"`
	Dim       int       `json:"dim"`
	Provider  string    `json:"provider"`
	Cached    bool      `json:"cached"`
	LatencyMs int64     `json:"latency_ms"`
}

type embedBatchResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
	Dim        int         `json:"dim"`
	Provider   string      `json:"provider"`
	LatencyMs  int64       `json:"latency_ms"`
}

type parseQueryResponse struct {
	Parsed            ParsedQuery `json:"parsed"`
	Embedding         []float64   `json:"embedding"`
	EmbeddingProvider string      `json:"embedding_provider"`
	LatencyMs         int64       `json:"latency_ms"`
}

type rerankAPIResponse struct {
	Results   []RerankResult `json:"results"`
	LatencyMs int64          `json:"latency_ms"`
}

type nerAPIResponse struct {
	Entities  []NEREntity `json:"entities"`
	LatencyMs int64       `json:"latency_ms"`
}

// ============ MlServiceClient ============

// mlAPI — поверхность ML-клиента, используемая скорингом/сервисами V2
// (DI как в TS: MatchingScoringServiceV2/QueryUnderstandingService получают MlServiceClient).
type mlAPI interface {
	available() bool
	ParseQuery(ctx context.Context, query string) QueryUnderstanding
	Rerank(ctx context.Context, query string, documents []string, topK *int) ([]RerankResult, bool)
}

type mlServiceClient struct {
	http    *http.Client
	baseURL string
	timeout time.Duration

	mu               sync.Mutex
	availablePtr     *bool
	lastHealthCheck  time.Time
	healthIntervalMs int64

	// Local TF-IDF fallback (when ML service unavailable)
	tfidfVocab  map[string]int
	tfidfIdf    map[string]float64
	tfidfFitted bool
}

func newMlServiceClient(cfg *config.Config) *mlServiceClient {
	baseURL := "http://localhost:8000"
	timeout := 5 * time.Second
	if cfg != nil {
		if cfg.MLServiceURL != "" {
			baseURL = cfg.MLServiceURL
		}
		if cfg.MLServiceTimeout > 0 {
			timeout = cfg.MLServiceTimeout
		}
	}
	return &mlServiceClient{
		http:             &http.Client{},
		baseURL:          baseURL,
		timeout:          timeout,
		healthIntervalMs: 30000,
		tfidfVocab:       map[string]int{},
		tfidfIdf:         map[string]float64{},
	}
}

// available — текущая доступность ML-сервиса (без проверки сети).
func (m *mlServiceClient) availableFlag() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.availablePtr != nil && *m.availablePtr
}

// CheckHealth — GET /health с кэшем 30 секунд.
func (m *mlServiceClient) CheckHealth(ctx context.Context, force bool) bool {
	m.mu.Lock()
	now := time.Now()
	if !force && m.availablePtr != nil && now.Sub(m.lastHealthCheck).Milliseconds() < m.healthIntervalMs {
		avail := *m.availablePtr
		m.mu.Unlock()
		return avail
	}
	m.mu.Unlock()

	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	ok := false
	var models healthResponse
	err := m.doJSON(reqCtx, http.MethodGet, m.baseURL+"/health", nil, 2*time.Second, &models)
	if err == nil {
		ok = models.Status == "ok"
	}

	m.mu.Lock()
	m.availablePtr = &ok
	m.lastHealthCheck = now
	m.mu.Unlock()

	if err == nil && ok {
		log.Printf("[MlClient] ML Service available — embedding=%v, reranker=%v",
			models.Models.Embedding.Ready, models.Models.Reranker.Ready)
	}
	return ok
}

// available — геттер без сетевой проверки (паритет с TS get available).
func (m *mlServiceClient) available() bool { return m.availableFlag() }

// doJSON — выполнение HTTP-запроса с JSON-телом и декодированием ответа.
func (m *mlServiceClient) doJSON(ctx context.Context, method, url string, body any, timeout time.Duration, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, method, url, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := m.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}

// Embed — generate embedding for text. Tries ML service first, falls back to local TF-IDF.
func (m *mlServiceClient) Embed(ctx context.Context, text string) (embedding []float64, provider string, latencyMs int64) {
	t0 := time.Now()

	if m.CheckHealth(ctx, false) {
		var resp embedResponse
		err := m.doJSON(ctx, http.MethodPost, m.baseURL+"/embed", map[string]any{"text": text}, m.timeout, &resp)
		if err == nil {
			return resp.Embedding, resp.Provider, time.Since(t0).Milliseconds()
		}
		log.Printf("[MlClient] ML embed failed: %s, using TF-IDF fallback", err.Error())
	}

	// Fallback: local TF-IDF
	embedding = m.tfidfEmbed(text)
	return embedding, "tfidf-local", time.Since(t0).Milliseconds()
}

// EmbedBatch — batch embedding (more efficient).
func (m *mlServiceClient) EmbedBatch(ctx context.Context, texts []string) (embeddings [][]float64, provider string) {
	if m.CheckHealth(ctx, false) {
		timeout := time.Duration(int64(m.timeout)*int64(len(texts))) * time.Millisecond
		if timeout > 30*time.Second {
			timeout = 30 * time.Second
		}
		var resp embedBatchResponse
		err := m.doJSON(ctx, http.MethodPost, m.baseURL+"/embed/batch",
			map[string]any{"texts": texts, "normalize": true}, timeout, &resp)
		if err == nil {
			return resp.Embeddings, resp.Provider
		}
		log.Printf("[MlClient] ML batch embed failed: %s", err.Error())
	}

	// Fallback
	out := make([][]float64, len(texts))
	for i, t := range texts {
		out[i] = m.tfidfEmbed(t)
	}
	return out, "tfidf-local"
}

// ParseQuery — full query understanding: parse + embed.
func (m *mlServiceClient) ParseQuery(ctx context.Context, query string) QueryUnderstanding {
	t0 := time.Now()

	if m.CheckHealth(ctx, false) {
		var resp parseQueryResponse
		err := m.doJSON(ctx, http.MethodPost, m.baseURL+"/parse-query",
			map[string]any{"query": query}, m.timeout, &resp)
		if err == nil {
			return QueryUnderstanding{
				OriginalQuery:     query,
				Parsed:            resp.Parsed,
				Embedding:         resp.Embedding,
				EmbeddingProvider: resp.EmbeddingProvider,
				LatencyMs:         time.Since(t0).Milliseconds(),
			}
		}
		log.Printf("[MlClient] ML parse query failed: %s", err.Error())
	}

	// Fallback: local parsing + TF-IDF embedding
	parsed := m.localParseQuery(query)
	embedding := m.tfidfEmbed(query)
	return QueryUnderstanding{
		OriginalQuery:     query,
		Parsed:            parsed,
		Embedding:         embedding,
		EmbeddingProvider: "tfidf-local",
		LatencyMs:         time.Since(t0).Milliseconds(),
	}
}

// Rerank — cross-encoder re-ranking. Возвращает переранжированные индексы со скорами.
func (m *mlServiceClient) Rerank(ctx context.Context, query string, documents []string, topK *int) ([]RerankResult, bool) {
	if m.CheckHealth(ctx, false) {
		// JS: { query, documents, top_k: topK } — undefined-ключ не сериализуется
		body := map[string]any{"query": query, "documents": documents}
		if topK != nil {
			body["top_k"] = *topK
		}
		var resp rerankAPIResponse
		err := m.doJSON(ctx, http.MethodPost, m.baseURL+"/rerank", body, 10*time.Second, &resp)
		if err == nil {
			return resp.Results, true
		}
		log.Printf("[MlClient] ML rerank failed: %s", err.Error())
	}

	// Fallback: no re-ranking (return original order)
	results := make([]RerankResult, len(documents))
	for i, doc := range documents {
		results[i] = RerankResult{Index: i, Score: 0.5, Document: doc}
	}
	return results, false
}

// Ner — Named Entity Recognition.
func (m *mlServiceClient) Ner(ctx context.Context, text string) []NEREntity {
	if m.CheckHealth(ctx, false) {
		var resp nerAPIResponse
		err := m.doJSON(ctx, http.MethodPost, m.baseURL+"/ner", map[string]any{"text": text}, m.timeout, &resp)
		if err == nil {
			return resp.Entities
		}
		// Fallback below
	}

	// Simple keyword-based fallback
	return m.localNer(text)
}

// ─────────────────────────── Local Fallbacks ───────────────────────────

// tfidfEmbed — local TF-IDF embedding (when ML service unavailable).
// Uses feature hashing for consistent 768-dim output.
// BUG-REF: M1 — tfidfVocab/tfidfIdf никогда не обучаются (tfidfFitted не выставляется),
// индекс всегда hashString(token) % 768, а 768-dim вектор далее усекается
// store.literal до 384 при записи в pgvector.
func (m *mlServiceClient) tfidfEmbed(text string) []float64 {
	tokens := m.tokenize(text)
	vector := make([]float64, 768)

	for _, token := range tokens {
		var idx int
		if i, ok := m.tfidfVocab[token]; ok {
			idx = i
		} else {
			// Feature hashing
			idx = int(m.hashString(token) % 768)
		}
		weight := 1.0
		if w, ok := m.tfidfIdf[token]; ok {
			weight = w
		}
		vector[idx] += weight
	}

	// Normalize
	norm := 0.0
	for _, v := range vector {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		out := make([]float64, len(vector))
		for i, v := range vector {
			out[i] = v / norm
		}
		return out
	}
	return vector
}

// tokenize — локальный токенайзер ML-клиента (regex [^\w\s] → пробел).
func (m *mlServiceClient) tokenize(text string) []string {
	lower := strings.ToLower(text)
	var b strings.Builder
	b.Grow(len(lower))
	for _, r := range lower {
		// JS \w = [A-Za-z0-9_]
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' ||
			(r >= 'A' && r <= 'Z') || r == ' ' || r == '\t' || r == '\n' || r == '\r' ||
			r == '\f' || r == '\v' {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	var out []string
	for _, t := range strings.Fields(b.String()) {
		if len([]rune(t)) > 1 {
			out = append(out, t)
		}
	}
	if out == nil {
		out = []string{}
	}
	return out
}

// hashString — (hash * 31 + charCodeAt(i)) | 0, затем Math.abs.
// По UTF-16 code units (паритет с JS charCodeAt).
func (m *mlServiceClient) hashString(str string) int64 {
	var hash int32 = 0
	for _, u := range utf16.Encode([]rune(str)) {
		hash = int32(int64(hash)*31 + int64(u)) // | 0 (свёртка в 32 бита)
	}
	if hash < 0 {
		return -int64(hash) // Math.abs (включая -2147483648 → 2147483648)
	}
	return int64(hash)
}

// localParseQuery — локальный разбор запроса (fallback, ordered словари).
func (m *mlServiceClient) localParseQuery(query string) ParsedQuery {
	lower := strings.ToLower(query)

	// Simple specialization detection (порядок Object.entries = порядок объявления)
	specKeywords := []struct {
		spec     string
		keywords []string
	}{
		{"BACKEND", []string{"backend", "бэкенд", "server", "api", "микросервис"}},
		{"FRONTEND", []string{"frontend", "фронтенд", "react", "vue", "angular"}},
		{"FULLSTACK", []string{"fullstack", "full-stack", "полный стек"}},
		{"MOBILE", []string{"mobile", "мобайл", "ios", "android", "flutter"}},
		{"DEVOPS", []string{"devops", "инфраструктура", "kubernetes", "docker"}},
		{"DATA_SCIENCE", []string{"data science", "ml", "machine learning", "аналитик"}},
		{"QA", []string{"qa", "тестировщик", "quality"}},
		{"SECURITY", []string{"security", "безопасность"}},
	}

	var specialization *string
	for _, entry := range specKeywords {
		for _, k := range entry.keywords {
			if strings.Contains(lower, k) {
				v := entry.spec
				specialization = &v
				break
			}
		}
		if specialization != nil {
			break
		}
	}

	// Simple grade detection
	gradeKeywords := []struct {
		grade    string
		keywords []string
	}{
		{"JUNIOR", []string{"junior", "джуниор", "начинающий"}},
		{"MIDDLE", []string{"middle", "мидл"}},
		{"SENIOR", []string{"senior", "синьор", "опытный"}},
		{"LEAD", []string{"lead", "лид", "team lead"}},
	}

	var seniority *string
	for _, entry := range gradeKeywords {
		for _, k := range entry.keywords {
			if strings.Contains(lower, k) {
				v := entry.grade
				seniority = &v
				break
			}
		}
		if seniority != nil {
			break
		}
	}

	// Extract tech skills (common ones)
	techSkills := []string{
		"node.js", "nodejs", "javascript", "typescript", "react", "vue", "angular",
		"python", "django", "flask", "fastapi", "java", "spring", "go", "golang",
		"c++", "c#", ".net", "rust", "php", "ruby", "swift", "kotlin",
		"postgresql", "mysql", "mongodb", "redis", "elasticsearch",
		"docker", "kubernetes", "aws", "gcp", "azure",
		"graphql", "rest", "grpc", "kafka", "rabbitmq",
		"microservices", "ci/cd", "git",
	}

	requiredSkills := []string{}
	for _, skill := range techSkills {
		if strings.Contains(lower, skill) {
			requiredSkills = append(requiredSkills, skill)
		}
	}

	ctx := jsSlice(query, 200)
	return ParsedQuery{
		Specialization:  specialization,
		Seniority:       seniority,
		RequiredSkills:  requiredSkills,
		PreferredSkills: []string{},
		Context:         &ctx,
		Intent:          "hire",
	}
}

// localNer — простой keyword-based fallback NER.
func (m *mlServiceClient) localNer(text string) []NEREntity {
	results := []NEREntity{}
	lower := strings.ToLower(text)

	techTerms := []string{
		"node.js", "react", "python", "docker", "kubernetes", "postgresql",
		"redis", "kafka", "grpc", "graphql", "typescript", "javascript",
	}
	for _, term := range techTerms {
		if strings.Contains(lower, term) {
			results = append(results, NEREntity{Text: term, Label: "TECH"})
		}
	}

	return results
}
