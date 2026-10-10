package matching

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"fsp-backend/internal/common"
	"fsp-backend/internal/config"
	"fsp-backend/internal/model"
	"fsp-backend/internal/modules/candidateview"
	"fsp-backend/internal/store"
)

// Порт backend/src/modules/matching/matching.service.ts (V1) + фасад публичного API.

// Cache — поверхность RedisService для кэша matching (Get/Set/DelByPattern).
// *cache.Redis удовлетворяет ей; nil → молча без кэша.
type Cache interface {
	Get(ctx context.Context, key string) *string
	Set(ctx context.Context, key, value string, ttlSeconds ...int)
	DelByPattern(ctx context.Context, pattern string)
}

// nopCache — кэш-заглушка (Redis недоступен/не передан).
type nopCache struct{}

func (nopCache) Get(context.Context, string) *string         { return nil }
func (nopCache) Set(context.Context, string, string, ...int) {}
func (nopCache) DelByPattern(context.Context, string)        {}

// Service — фасад matching-подсистемы (V1 + V2).
type Service struct {
	db        *store.DB
	vectors   *store.VectorRepository
	cfg       *config.Config
	cache     Cache
	embedding *embeddingService
	ml        *mlServiceClient

	scoring   scoringService
	scoringV2 scoringV2Service
	explain   explanationService
	indexing  indexingService
	query     queryUnderstandingService
}

// NewService — конструктор для интегратора (app.New).
// db/vectors/cache/cfg: nil → создаётся/отключается автоматически.
func NewService(db *store.DB, vectors *store.VectorRepository, cfg *config.Config, cache Cache) *Service {
	if cfg == nil {
		cfg = config.Load()
	}
	if vectors == nil {
		vectors = store.NewVectorRepository(db)
	}
	if cache == nil {
		cache = nopCache{}
	}
	ml := newMlServiceClient(cfg)
	s := &Service{
		db:        db,
		vectors:   vectors,
		cfg:       cfg,
		cache:     cache,
		embedding: newEmbeddingService(),
		ml:        ml,
		scoringV2: newScoringV2Service(ml),
		query:     newQueryUnderstandingService(ml),
	}
	s.indexing = indexingService{db: db, embedding: s.embedding, vectors: vectors}

	// OnModuleInit EmbeddingService + IndexingService
	s.embedding.onModuleInit(cfg)
	s.indexing.onModuleInit()
	return s
}

// ============ Публичный API фасада (api_stub.go) ============

// InvalidateCache — delByPattern('matching:cache:*').
// BUG-REF: M8 — общий неймспейс: инвалидация стирает кэш ОБЕИХ версий (search + search-v2).
func (s *Service) InvalidateCache(ctx context.Context) error {
	s.cache.DelByPattern(ctx, "matching:cache:*")
	return nil
}

// ReindexCandidate — полная переиндексация одного кандидата.
func (s *Service) ReindexCandidate(ctx context.Context, profileID string) error {
	return s.indexing.indexCandidate(ctx, profileID)
}

// IndexCandidate — ленивая индексация одного кандидата.
func (s *Service) IndexCandidate(ctx context.Context, profileID string) error {
	return s.indexing.indexCandidate(ctx, profileID)
}

// IndexBatch — пакетная индексация (по 25 из поиска).
func (s *Service) IndexBatch(ctx context.Context, ids []string) error {
	return s.indexing.indexBatch(ctx, ids)
}

// ReindexAll — фоновая полная переиндексация (до 5000 профилей).
func (s *Service) ReindexAll(ctx context.Context) error {
	return s.indexing.reindexAll(ctx)
}

// CurrentProvider — состояние embedding-провайдера (/api/health).
func (s *Service) CurrentProvider() string { return string(s.embedding.currentProvider()) }

// CurrentModel — имя загруженной модели ("tfidf" — ONNX недоступен в Go-сборке).
func (s *Service) CurrentModel() string {
	if m := s.embedding.currentModel(); m != "" {
		return m
	}
	return providerTfidf
}

// Dim — размерность embedding-а (384).
func (s *Service) Dim() int { return s.embedding.dim() }

// ============ V1: поиск ============

// Search — поиск кандидатов V1 ({total, candidates[]}).
func (s *Service) Search(ctx context.Context, dto SearchCandidatesDTO) (map[string]any, error) {
	limit := effectiveLimit(dto.Limit)
	offset := effectiveOffset(dto.Offset)
	cacheKey := s.cacheKey("search", dto, limit, offset)

	if cached := s.cache.Get(ctx, cacheKey); cached != nil {
		var out map[string]any
		if err := json.Unmarshal([]byte(*cached), &out); err == nil {
			return out, nil
		}
		// повреждённый кэш — пересчитываем
	}

	started := time.Now()
	pool, err := s.loadPool(ctx, dto)
	if err != nil {
		return nil, err
	}

	need := Need{
		Specialization: specializationOf(dto.Specialization),
		Grade:          gradeOf(dto.Grade),
		Stack:          dto.Stack,
	}
	if need.Stack == nil {
		need.Stack = []string{}
	}

	queryText := strings.TrimSpace(derefStr(dto.Description) + " " + strings.Join(dto.Stack, " "))
	needVector := s.embedding.embed(queryText)
	if queryText == "" {
		needVector = s.embedding.embed(s.defaultQueryTextV1(need))
	}
	hits := s.vectors.TopBySimilarity(ctx, needVector, 3000)
	similarityById := map[string]float64{}
	for _, h := range hits {
		similarityById[h.ID] = h.Similarity
	}

	// Ленивая переиндексация кандидатов, у которых ещё нет вектора
	// BUG-REF: M6 — KNN глобальный (без префильтрации), пересечение в памяти.
	unindexed := []string{}
	for _, c := range pool {
		if _, ok := similarityById[c.ID]; !ok {
			unindexed = append(unindexed, c.ID)
		}
	}
	if len(unindexed) > 0 {
		batch := unindexed
		if len(batch) > 25 {
			batch = batch[:25]
		}
		go func() {
			if err := s.IndexBatch(context.Background(), batch); err != nil {
				log.Printf("[Matching] Ленивая индексация: %s", err.Error())
			}
		}()
	}

	scored := make([]map[string]any, 0, len(pool))
	for _, candidate := range pool {
		_, indexed := similarityById[candidate.ID]
		semantic := 0.0
		if indexed && similarityById[candidate.ID] > 0 {
			semantic = similarityById[candidate.ID]
		}
		view := candidateview.ToPublic(candidate, indexed)
		scoreable := ScoreableCandidate{
			Specialization:    specializationOf(candidate.Specialization),
			VerifiedGrade:     gradeOf(candidate.VerifiedGrade),
			DeclaredGrade:     gradeOf(candidate.DeclaredGrade),
			Stack:             candidate.Stack,
			AchievementPoints: candidateview.AchievementPoints(candidate),
		}
		score := s.scoring.calculateMatch(scoreable, need, semantic)
		view["matchScore"] = score.Total
		view["matchComponents"] = score.Components
		view["matchExplanation"] = s.explain.generate(ExplanationCandidate{
			ScoreableCandidate: scoreable,
			LastTestDate:       lastTestTime(candidate),
			AchievementNames:   achievementNames(candidate),
		}, need, score)
		scored = append(scored, view)
	}

	sort.SliceStable(scored, func(a, b int) bool {
		return viewScore(scored[a]) > viewScore(scored[b])
	})

	candidates := scored
	if offset < len(candidates) {
		end := offset + limit
		if end > len(candidates) {
			end = len(candidates)
		}
		candidates = scored[offset:end]
	} else {
		candidates = []map[string]any{}
	}

	result := map[string]any{
		"total":      len(scored),
		"candidates": candidates,
	}

	ttl := s.cacheTTL()
	s.cache.Set(ctx, cacheKey, jsJSON(result), ttl)
	log.Printf("[Matching] search: pool=%d, rendered=%d, %dms",
		len(pool), len(candidates), time.Since(started).Milliseconds())
	return result, nil
}

// SemanticSearch — чисто семантический поиск (без жёстких фильтров).
func (s *Service) SemanticSearch(ctx context.Context, dto SemanticQueryDTO) (map[string]any, error) {
	vector := s.embedding.embed(dto.Query)
	perPage := dto.Limit
	if perPage == 0 {
		perPage = 10 // dto.limit || 10
	}
	hits := s.vectors.TopBySimilarity(ctx, vector, minInt(perPage*5, 200))

	limitIDs := perPage
	if len(hits) < limitIDs {
		limitIDs = len(hits)
	}
	ids := make([]string, 0, limitIDs)
	for i := 0; i < limitIDs; i++ {
		ids = append(ids, hits[i].ID)
	}
	if len(ids) == 0 {
		return map[string]any{"candidates": []map[string]any{}}, nil
	}

	ordered, err := loadCandidatesOrdered(ctx, s.db, ids)
	if err != nil {
		return nil, err
	}
	similarityById := map[string]float64{}
	for _, h := range hits {
		similarityById[h.ID] = h.Similarity
	}

	candidates := make([]map[string]any, 0, len(ordered))
	for _, p := range ordered {
		view := candidateview.ToPublic(p, true)
		view["similarity"] = round4(similarityById[p.ID])
		view["reason"] = semanticReason(p)
		candidates = append(candidates, view)
	}
	return map[string]any{"candidates": candidates}, nil
}

// Recommendations — collaborative filtering + новые кандидаты в категории работодателя.
func (s *Service) Recommendations(ctx context.Context, employerID string) (map[string]any, error) {
	var industry *string
	err := s.db.Pool.QueryRow(ctx,
		`SELECT industry FROM employer_profiles WHERE id = $1`, employerID).Scan(&industry)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	invitedByMe, err := s.invitationCandidateIDs(ctx, `SELECT candidate_id FROM invitations WHERE employer_id = $1`, employerID)
	if err != nil {
		return nil, err
	}

	var specialization *string
	err = s.db.Pool.QueryRow(ctx,
		`SELECT specialization FROM employer_needs WHERE employer_id = $1 ORDER BY created_at DESC LIMIT 1`,
		employerID).Scan(&specialization)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// 1. Кандидатов, которых приглашали похожие работодатели (та же индустрия)
	similarQuery := `SELECT id FROM employer_profiles WHERE id <> $1`
	args := []any{employerID}
	if industry != nil && *industry != "" {
		similarQuery += ` AND industry = $2`
		args = append(args, *industry)
	}
	similarQuery += ` LIMIT 20`
	rows, err := s.db.Pool.Query(ctx, similarQuery, args...)
	if err != nil {
		return nil, err
	}
	similarEmployers := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		similarEmployers = append(similarEmployers, id)
	}
	rows.Close()

	peerInvited := []string{}
	if len(similarEmployers) > 0 {
		peerInvited, err = s.invitationCandidateIDsBatch(ctx, similarEmployers)
		if err != nil {
			return nil, err
		}
	}

	peerIds := []string{}
	seenPeer := map[string]bool{}
	for _, id := range peerInvited {
		if invitedByMe[id] || seenPeer[id] {
			continue
		}
		seenPeer[id] = true
		peerIds = append(peerIds, id)
		if len(peerIds) >= 5 {
			break
		}
	}

	peerCandidates, err := loadCandidatesOrdered(ctx, s.db, peerIds)
	if err != nil {
		return nil, err
	}

	// 2. Новые кандидаты (последние 14 дней)
	freshCutoff := time.Now().Add(-14 * 24 * time.Hour)
	freshQuery := `SELECT id FROM candidate_profiles WHERE created_at >= $1 AND id <> ALL($2::text[])`
	freshArgs := []any{freshCutoff, peerIds}
	if specialization != nil && *specialization != "" {
		freshQuery += ` AND specialization = $3`
		freshArgs = append(freshArgs, *specialization)
	}
	freshQuery += ` ORDER BY created_at DESC LIMIT 5`
	frows, err := s.db.Pool.Query(ctx, freshQuery, freshArgs...)
	if err != nil {
		return nil, err
	}
	freshIds := []string{}
	for frows.Next() {
		var id string
		if err := frows.Scan(&id); err != nil {
			frows.Close()
			return nil, err
		}
		freshIds = append(freshIds, id)
	}
	frows.Close()

	freshCandidates, err := loadCandidatesOrdered(ctx, s.db, freshIds)
	if err != nil {
		return nil, err
	}

	peerViews := make([]map[string]any, 0, len(peerCandidates))
	for _, c := range peerCandidates {
		peerViews = append(peerViews, candidateview.ToPublic(c, true))
	}
	freshViews := make([]map[string]any, 0, len(freshCandidates))
	for _, c := range freshCandidates {
		freshViews = append(freshViews, candidateview.ToPublic(c, true))
	}

	freshReason := "Новые кандидаты на платформе"
	if specialization != nil && *specialization != "" {
		freshReason = "Новые кандидаты в категории " + *specialization
	}

	groups := []map[string]any{}
	if len(peerViews) > 0 {
		groups = append(groups, map[string]any{
			"reason":     "Похожие работодатели приглашали этих кандидатов",
			"candidates": peerViews,
		})
	}
	if len(freshViews) > 0 {
		groups = append(groups, map[string]any{
			"reason":     freshReason,
			"candidates": freshViews,
		})
	}

	return map[string]any{"recommendations": groups}, nil
}

// CreateNeedV1 — создание потребности (V1-логика matching.service.createNeed).
// HTTP-маршрут /matching/v2/need использует V2-логику (CreateNeed).
func (s *Service) CreateNeedV1(ctx context.Context, employerProfileID string, dto EmployerNeedDTO) (map[string]any, error) {
	stack := dto.Stack
	if stack == nil {
		stack = []string{}
	}

	id := uuid.NewString()
	if _, err := s.db.Pool.Exec(ctx, `
		INSERT INTO employer_needs (id, employer_id, description, specialization, grade, stack, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, employerProfileID, dto.Description, dto.Specialization, dto.Grade, stack, model.Now(),
	); err != nil {
		return nil, err
	}

	vector := s.embedding.embed(dto.Description + " " + strings.Join(stack, " "))
	if err := s.vectors.SetNeedVector(ctx, id, vector); err != nil {
		return nil, err
	}

	search, err := s.Search(ctx, SearchCandidatesDTO{
		Description:    &dto.Description,
		Specialization: dto.Specialization,
		Grade:          dto.Grade,
		Stack:          stack,
		Limit:          10,
		Offset:         0,
	})
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"id":              id,
		"description":     dto.Description,
		"specialization":  dto.Specialization,
		"grade":           dto.Grade,
		"stack":           stack,
		"candidatesCount": search["total"],
		"topCandidates":   search["candidates"],
	}, nil
}

// ============ Внутренние помощники ============

// loadPool — findMany({where: buildWhere(dto), include: candidateInclude, take: 3000}).
func (s *Service) loadPool(ctx context.Context, dto SearchCandidatesDTO) ([]*candidateview.Candidate, error) {
	where, args := s.buildWhere(dto)
	query := `SELECT ` + candidateColumns + candidateFrom + where + ` LIMIT 3000`
	return scanCandidates(ctx, s.db, query, args...)
}

// buildWhere — Prisma-фильтры: specialization, ±1 грейд, есть достижения ФСП.
func (s *Service) buildWhere(dto SearchCandidatesDTO) (string, []any) {
	conds := []string{}
	args := []any{}

	if spec := specializationOf(dto.Specialization); spec != "" {
		args = append(args, string(spec))
		conds = append(conds, `cp.specialization = $`+strconv.Itoa(len(args)))
	}

	if grade := gradeOf(dto.Grade); grade != "" {
		idx := gradeIndexOf(grade)
		allowed := []string{}
		for _, pos := range []int{idx - 1, idx, idx + 1} {
			if pos >= 0 && pos < len(GRADE_ORDER) {
				allowed = append(allowed, string(GRADE_ORDER[pos]))
			}
		}
		args = append(args, allowed)
		n := strconv.Itoa(len(args))
		conds = append(conds,
			`(cp.verified_grade::text = ANY($`+n+`) OR (cp.verified_grade IS NULL AND cp.declared_grade::text = ANY($`+n+`)))`)
	}

	if dto.HasAchievements != nil && *dto.HasAchievements {
		conds = append(conds, `EXISTS (SELECT 1 FROM candidate_achievements ca WHERE ca.candidate_id = cp.id)`)
	}

	if len(conds) == 0 {
		return "", args
	}
	return ` WHERE ` + strings.Join(conds, " AND "), args
}

// defaultQueryTextV1 — fallback-текст запроса из структурированной потребности.
func (s *Service) defaultQueryTextV1(need Need) string {
	parts := []string{}
	if need.Specialization != "" {
		parts = append(parts, string(need.Specialization))
	}
	if need.Grade != "" {
		parts = append(parts, string(need.Grade))
	}
	parts = append(parts, need.Stack...)
	return strings.Join(filterNonEmpty(parts), " ")
}

// defaultQueryTextV2 — fallback-текст запроса из DTO.
func (s *Service) defaultQueryTextV2(dto SearchCandidatesDTO) string {
	parts := []string{}
	if dto.Specialization != nil && *dto.Specialization != "" {
		parts = append(parts, *dto.Specialization)
	}
	if dto.Grade != nil && *dto.Grade != "" {
		parts = append(parts, *dto.Grade)
	}
	if dto.Stack != nil {
		parts = append(parts, dto.Stack...)
	}
	return strings.Join(filterNonEmpty(parts), " ")
}

// cacheKey — sha1(jsJSON(payload) + jsJSON([limit, offset])), как cacheKey() в TS.
func (s *Service) cacheKey(prefix string, dto SearchCandidatesDTO, limit, offset int) string {
	payload := map[string]any{}
	if dto.Description != nil {
		payload["description"] = *dto.Description
	}
	if dto.Specialization != nil {
		payload["specialization"] = *dto.Specialization
	}
	if dto.Grade != nil {
		payload["grade"] = *dto.Grade
	}
	if dto.Stack != nil {
		payload["stack"] = dto.Stack
	}
	if dto.HasAchievements != nil {
		payload["hasAchievements"] = *dto.HasAchievements
	}
	// limit/offset — эффективные (клампленные) значения: инициализаторы свойств класса
	payload["limit"] = limit
	payload["offset"] = offset

	normalized := jsJSON(payload)
	extra := jsJSON([]int{limit, offset})
	sum := sha1.Sum([]byte(normalized + extra))
	return "matching:cache:" + prefix + ":" + hex.EncodeToString(sum[:])
}

// cacheTTL — matching.cacheTtlSeconds || 900.
func (s *Service) cacheTTL() int {
	if s.cfg != nil && s.cfg.MatchingCacheTTLSeconds > 0 {
		return s.cfg.MatchingCacheTTLSeconds
	}
	return 900
}

// invitationCandidateIDs — candidateId приглашений работодателя.
func (s *Service) invitationCandidateIDs(ctx context.Context, query, employerID string) (map[string]bool, error) {
	rows, err := s.db.Pool.Query(ctx, query, employerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// invitationCandidateIDsBatch — peerInvitations: по списку работодателей, created_at DESC LIMIT 100.
func (s *Service) invitationCandidateIDsBatch(ctx context.Context, employerIDs []string) ([]string, error) {
	rows, err := s.db.Pool.Query(ctx,
		`SELECT candidate_id FROM invitations WHERE employer_id = ANY($1::text[])
		 ORDER BY created_at DESC LIMIT 100`, employerIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ============ JS-семантика (общие помощники) ============

// jsJSON — JSON.stringify в стиле JS (без HTML-экранирования, без завершающего \n).
func jsJSON(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return ""
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

// effectiveLimit — Math.min(Math.max(dto.limit ?? 20, 1), 100).
func effectiveLimit(limit int) int {
	if limit <= 0 {
		limit = 20 // dto.limit ?? 20 (Go zero-value = поле отсутствует)
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	return limit
}

// effectiveOffset — Math.max(dto.offset ?? 0, 0).
func effectiveOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func nowMs() int64 { return time.Now().UnixMilli() }

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func specializationOf(p *string) model.Specialization {
	if p == nil {
		return ""
	}
	return model.Specialization(*p)
}

func gradeOf(p *string) model.Grade {
	if p == nil {
		return ""
	}
	return model.Grade(*p)
}

func lastTestTime(c *candidateview.Candidate) *time.Time {
	if c.LastTestDate == nil {
		return nil
	}
	t := c.LastTestDate.Time
	return &t
}

func viewScore(view map[string]any) float64 {
	if v, ok := view["matchScore"].(float64); ok {
		return v
	}
	return 0
}

// round4 — Number(sim.toFixed(4)).
func round4(v float64) float64 {
	if !isFinite(v) {
		return 0
	}
	return math.Round(v*10000) / 10000
}

// semanticReason — reason из V1/V2 semantic search.
func semanticReason(p *candidateview.Candidate) string {
	if p.Bio != nil && *p.Bio != "" {
		return jsSlice(*p.Bio, 160)
	}
	stack := "не указан"
	if joined := strings.Join(p.Stack, ", "); joined != "" {
		stack = joined
	}
	return "Стек: " + stack
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func filterNonEmpty(parts []string) []string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// RegisterRoutes — маршруты matching-движка (аналог MatchingController).
func (s *Service) RegisterRoutes(r *common.Router) {
	NewHandler(s).Register(r)
}
