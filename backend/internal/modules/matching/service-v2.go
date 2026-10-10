package matching

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"fsp-backend/internal/model"
	"fsp-backend/internal/modules/candidateview"
)

// Порт backend/src/modules/matching/matching-v2.service.ts (as-is, включая баги M2–M6).

// scoredItemV2 — промежуточный элемент V2-поиска (view + кандидат + семантика).
type scoredItemV2 struct {
	view      map[string]any
	candidate *candidateview.Candidate
	semantic  float64
}

// breakdownWeights — getWeight() из matching-v2.service.ts.
// BUG-REF: M5 — «rerank: 0.10» отображается в breakdown, хотя DEFAULT_WEIGHTS
// rerank не содержит (фактически rerank — аддитивный буст до +10 баллов).
var breakdownWeights = map[string]float64{
	"specialization": 0.18,
	"stack":          0.15,
	"semantic":       0.20,
	"grade":          0.12,
	"achievement":    0.08,
	"experience":     0.12,
	"testScore":      0.15,
	"rerank":         0.10,
}

// SearchV2 — ML query understanding + расширенный scoring + опциональный re-ranking.
func (s *Service) SearchV2(ctx context.Context, dto SearchCandidatesDTO) (map[string]any, error) {
	limit := effectiveLimit(dto.Limit)
	offset := effectiveOffset(dto.Offset)
	totalStart := time.Now()

	// Check cache
	cacheKey := s.cacheKey("search-v2", dto, limit, offset)
	if cached := s.cache.Get(ctx, cacheKey); cached != nil {
		var out map[string]any
		if err := json.Unmarshal([]byte(*cached), &out); err == nil {
			return out, nil
		}
		// corrupted cache — recompute
	}

	// ─── Step 1: Query Understanding ───
	queryStart := time.Now()
	queryText := strings.TrimSpace(derefStr(dto.Description) + " " + strings.Join(dto.Stack, " "))

	description := queryText
	if description == "" {
		description = s.defaultQueryTextV2(dto)
	}
	parsedNeed := s.query.parseNeed(ctx, description, specializationOf(dto.Specialization), gradeOf(dto.Grade), dto.Stack)
	queryTime := time.Since(queryStart).Milliseconds()

	// ─── Step 2: Retrieval ───
	retrievalStart := time.Now()

	// Hard filters (Prisma)
	pool, err := s.loadPool(ctx, dto)
	if err != nil {
		return nil, err
	}

	// Semantic retrieval via vector similarity
	// BUG-REF: M1 — вектор ML-сервиса (768-dim) молча усекается literal()-ом до 384.
	hits := s.vectors.TopBySimilarity(ctx, parsedNeed.Embedding, 3000)
	similarityById := map[string]float64{}
	for _, h := range hits {
		similarityById[h.ID] = h.Similarity
	}

	// Lazy re-indexing for unindexed candidates
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
				log.Printf("[MatchingV2] Lazy indexing: %s", err.Error())
			}
		}()
	}

	retrievalTime := time.Since(retrievalStart).Milliseconds()

	// ─── Step 3: Scoring ───
	scoringStart := time.Now()

	need := NeedV2{
		Specialization: parsedNeed.Specialization,
		Grade:          parsedNeed.Grade,
	}
	if need.Specialization == "" {
		need.Specialization = specializationOf(dto.Specialization)
	}
	if need.Grade == "" {
		need.Grade = gradeOf(dto.Grade)
	}
	// BUG-REF: M4 — stack = requiredSkills (если ML что-то нашёл) И requiredSkills
	// передаются отдельно → скиллы учитываются дважды (×1.5 и ×1.0).
	if len(parsedNeed.RequiredSkills) > 0 {
		need.Stack = parsedNeed.RequiredSkills
	} else if dto.Stack != nil {
		need.Stack = dto.Stack
	} else {
		need.Stack = []string{}
	}
	need.RequiredSkills = parsedNeed.RequiredSkills
	need.PreferredSkills = parsedNeed.PreferredSkills

	scoredCandidates := []scoredItemV2{}

	for _, candidate := range pool {
		_, indexed := similarityById[candidate.ID]
		semantic := 0.0
		if indexed && similarityById[candidate.ID] > 0 {
			semantic = similarityById[candidate.ID]
		}

		view := candidateview.ToPublic(candidate, indexed)
		candidateText := s.query.buildCandidateText(candidate)

		scoreable := ScoreableCandidateV2{
			Specialization:    specializationOf(candidate.Specialization),
			VerifiedGrade:     gradeOf(candidate.VerifiedGrade),
			DeclaredGrade:     gradeOf(candidate.DeclaredGrade),
			Stack:             candidate.Stack,
			AchievementPoints: candidateview.AchievementPoints(candidate),
			TestScore:         candidate.LastTestScore,
			ExperienceYears:   candidate.ExperienceYears,
		}
		score, err := s.scoringV2.calculateMatch(ctx, scoreable, need, semantic, candidateText, queryText)
		if err != nil {
			return nil, err
		}

		view["matchScoreV2"] = score.Total
		view["matchScore"] = score.Total // For backward compatibility
		view["matchBreakdown"] = map[string]any{}
		view["weightsSource"] = score.WeightsSource
		view["semanticSimilarity"] = semantic
		view["rerankerScore"] = nil
		view["retrievalRank"] = 0
		view["finalRank"] = 0

		// Build breakdown from contributions
		// BUG-REF: M2 — score читается по КЛЮЧУ ВКЛАДА ('specialization', 'stack', …),
		// тогда как components содержит ключи вида 'specializationMatch' → 0
		// (не ноль только у совпадающего 'testScore').
		componentsByKey := map[string]float64{
			"specializationMatch": score.Components.SpecializationMatch,
			"stackOverlap":        score.Components.StackOverlap,
			"semanticSimilarity":  score.Components.SemanticSimilarity,
			"gradeMatch":          score.Components.GradeMatch,
			"achievementBoost":    score.Components.AchievementBoost,
			"experienceRelevance": score.Components.ExperienceRelevance,
			"testScore":           score.Components.TestScore,
		}
		breakdown := map[string]any{}
		for key, contribution := range score.Contributions {
			breakdown[key] = map[string]any{
				"score":        componentsByKey[key], // || 0 в TS
				"weight":       s.getWeight(key),
				"contribution": contribution,
			}
		}
		view["matchBreakdown"] = breakdown

		// Build explanation (using V1 explanation for now, enhanced in future)
		view["matchExplanation"] = s.explain.generate(ExplanationCandidate{
			ScoreableCandidate: ScoreableCandidate{
				Specialization:    scoreable.Specialization,
				VerifiedGrade:     scoreable.VerifiedGrade,
				DeclaredGrade:     scoreable.DeclaredGrade,
				Stack:             scoreable.Stack,
				AchievementPoints: scoreable.AchievementPoints,
			},
			LastTestDate:     lastTestTime(candidate),
			AchievementNames: achievementNames(candidate),
		}, Need{
			Specialization: need.Specialization,
			Grade:          need.Grade,
			Stack:          need.Stack,
		}, MatchScore{
			Total: score.Total,
			Components: MatchComponents{
				CategoryMatch:      score.Components.SpecializationMatch,
				StackOverlap:       score.Components.StackOverlap,
				SemanticSimilarity: score.Components.SemanticSimilarity,
				GradeMatch:         score.Components.GradeMatch,
				AchievementBoost:   score.Components.AchievementBoost,
			},
		})

		scoredCandidates = append(scoredCandidates, scoredItemV2{view: view, candidate: candidate, semantic: semantic})
	}

	scoringTime := time.Since(scoringStart).Milliseconds()

	// ─── Step 4: Sort by score ───
	sort.SliceStable(scoredCandidates, func(a, b int) bool {
		return viewScore(scoredCandidates[a].view) > viewScore(scoredCandidates[b].view)
	})

	// Assign retrieval ranks
	for idx := range scoredCandidates {
		scoredCandidates[idx].view["retrievalRank"] = idx + 1
	}

	// ─── Step 5: Optional re-ranking (top candidates only) ───
	rerankingStart := time.Now()
	rerankTopK := minInt(limit+20, 50) // Re-rank a bit more than needed
	topCandidates := scoredCandidates
	if len(topCandidates) > rerankTopK {
		topCandidates = topCandidates[:rerankTopK]
	}

	if queryText != "" && len(topCandidates) > 1 && s.ml.available() {
		documents := make([]string, 0, len(topCandidates))
		for _, item := range topCandidates {
			documents = append(documents, s.query.buildCandidateText(item.candidate))
		}

		results, available := s.ml.Rerank(ctx, queryText, documents, &rerankTopK)
		if available {
			// Apply re-ranking scores
			for _, result := range results {
				if result.Index < 0 || result.Index >= len(topCandidates) {
					continue // TS: item может отсутствовать — guard
				}
				item := &topCandidates[result.Index]
				// Normalize cross-encoder score (typically -10 to 10) to 0-100
				normalizedScore := math.Max(0, math.Min(100, (result.Score+10)*5))
				item.view["rerankerScore"] = result.Score

				// Blend: 70% original score + 30% re-ranker score
				blendedScore := viewScore(item.view)*0.7 + normalizedScore*0.3
				item.view["matchScoreV2"] = jsRound1(blendedScore)
				item.view["matchScore"] = item.view["matchScoreV2"]
			}

			// Re-sort after re-ranking
			sort.SliceStable(topCandidates, func(a, b int) bool {
				return viewScore(topCandidates[a].view) > viewScore(topCandidates[b].view)
			})

			log.Printf("[MatchingV2] Re-ranked %d candidates", len(results))
		}
	}
	rerankingTime := time.Since(rerankingStart).Milliseconds()

	// Assign final ranks
	for idx := range topCandidates {
		topCandidates[idx].view["finalRank"] = idx + 1
	}

	// Merge top candidates back into full list
	rerankedIds := map[string]bool{}
	for _, t := range topCandidates {
		rerankedIds[viewID(t.view)] = true
	}
	finalCandidates := make([]scoredItemV2, 0, len(scoredCandidates))
	finalCandidates = append(finalCandidates, topCandidates...)
	for _, item := range scoredCandidates {
		if !rerankedIds[viewID(item.view)] {
			finalCandidates = append(finalCandidates, item)
		}
	}

	// ─── Step 6: Build result ───
	rendered := []map[string]any{}
	if offset < len(finalCandidates) {
		end := offset + limit
		if end > len(finalCandidates) {
			end = len(finalCandidates)
		}
		for _, item := range finalCandidates[offset:end] {
			rendered = append(rendered, item.view)
		}
	}

	parsedSpec := any(nil)
	if parsedNeed.Specialization != "" {
		parsedSpec = string(parsedNeed.Specialization)
	}
	parsedSeniority := any(nil)
	if parsedNeed.Grade != "" {
		parsedSeniority = string(parsedNeed.Grade)
	}
	totalMs := time.Since(totalStart).Milliseconds()

	result := map[string]any{
		"total":      len(finalCandidates),
		"candidates": rendered,
		"queryUnderstanding": map[string]any{
			"originalQuery":        queryText,
			"parsedSpecialization": parsedSpec,
			"parsedSeniority":      parsedSeniority,
			"parsedSkills":         parsedNeed.RequiredSkills,
			"embeddingProvider":    parsedNeed.EmbeddingProvider,
		},
		"timing": map[string]any{
			"retrieval": retrievalTime,
			"scoring":   scoringTime,
			"reranking": rerankingTime,
			"total":     totalMs,
		},
	}

	// Cache result
	ttl := s.cacheTTL()
	s.cache.Set(ctx, cacheKey, jsJSON(result), ttl)

	log.Printf("[MatchingV2] search-v2: pool=%d, rendered=%d, query=%dms, retrieval=%dms, scoring=%dms, reranking=%dms, total=%dms",
		len(pool), len(rendered), queryTime, retrievalTime, scoringTime, rerankingTime, totalMs)

	return result, nil
}

// SemanticSearchV2 — семантический поиск с ML-эмбеддингами и query understanding.
func (s *Service) SemanticSearchV2(ctx context.Context, dto SemanticQueryDTO) (map[string]any, error) {
	parsed := s.ml.ParseQuery(ctx, dto.Query)
	vector := parsed.Embedding

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

	parsedSpec := any(nil)
	if spec := toSpecialization(parsed.Parsed.Specialization); spec != "" {
		parsedSpec = string(spec)
	}
	var parsedSeniority any
	if parsed.Parsed.Seniority != nil {
		parsedSeniority = *parsed.Parsed.Seniority
	}

	candidates := make([]map[string]any, 0, len(ordered))
	for _, p := range ordered {
		view := candidateview.ToPublic(p, true)
		view["similarity"] = round4(similarityById[p.ID])
		view["reason"] = semanticReason(p)
		candidates = append(candidates, view)
	}

	return map[string]any{
		"queryUnderstanding": map[string]any{
			"originalQuery":        dto.Query,
			"parsedSpecialization": parsedSpec,
			"parsedSeniority":      parsedSeniority,
			"parsedSkills":         parsed.Parsed.RequiredSkills,
			"embeddingProvider":    parsed.EmbeddingProvider,
		},
		"candidates": candidates,
	}, nil
}

// CreateNeed — создание потребности V2: query understanding + подборка.
func (s *Service) CreateNeed(ctx context.Context, employerProfileID string, dto EmployerNeedDTO) (map[string]any, error) {
	// Parse query with ML
	parsedNeed := s.query.parseNeed(ctx, dto.Description, specializationOf(dto.Specialization), gradeOf(dto.Grade), dto.Stack)

	// Store need
	storedSpec := parsedNeed.Specialization
	if storedSpec == "" {
		storedSpec = specializationOf(dto.Specialization)
	}
	storedGrade := parsedNeed.Grade
	if storedGrade == "" {
		storedGrade = gradeOf(dto.Grade)
	}
	storedStack := []string{}
	if len(parsedNeed.RequiredSkills) > 0 {
		storedStack = parsedNeed.RequiredSkills
	} else if dto.Stack != nil {
		storedStack = dto.Stack
	}

	id := uuid.NewString()
	if _, err := s.db.Pool.Exec(ctx, `
		INSERT INTO employer_needs (id, employer_id, description, specialization, grade, stack, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, employerProfileID, dto.Description, storedSpecPtr(storedSpec), storedGradePtr(storedGrade), storedStack, model.Now(),
	); err != nil {
		return nil, err
	}

	// Store vector
	if err := s.vectors.SetNeedVector(ctx, id, parsedNeed.Embedding); err != nil {
		return nil, err
	}

	// Run search
	searchStack := dto.Stack
	if searchStack == nil {
		searchStack = []string{}
	}
	search, err := s.SearchV2(ctx, SearchCandidatesDTO{
		Description:    &dto.Description,
		Specialization: dto.Specialization,
		Grade:          dto.Grade,
		Stack:          searchStack,
		Limit:          10,
		Offset:         0,
	})
	if err != nil {
		return nil, err
	}

	storedSpecOut := any(nil)
	if storedSpec != "" {
		storedSpecOut = string(storedSpec)
	}
	storedGradeOut := any(nil)
	if storedGrade != "" {
		storedGradeOut = string(storedGrade)
	}

	return map[string]any{
		"id":                 id,
		"description":        dto.Description,
		"specialization":     storedSpecOut,
		"grade":              storedGradeOut,
		"stack":              storedStack,
		"queryUnderstanding": search["queryUnderstanding"],
		"candidatesCount":    search["total"],
		"topCandidates":      search["candidates"],
		"timing":             search["timing"],
	}, nil
}

// getWeight — вес компоненты для отображения в breakdown.
func (s *Service) getWeight(key string) float64 {
	if w, ok := breakdownWeights[key]; ok {
		return w
	}
	return 0
}

// storedSpecPtr/storedGradePtr — enum → параметр INSERT (пустое значение → NULL).
func storedSpecPtr(v model.Specialization) *string {
	if v == "" {
		return nil
	}
	s := string(v)
	return &s
}

func storedGradePtr(v model.Grade) *string {
	if v == "" {
		return nil
	}
	g := string(v)
	return &g
}

func viewID(view map[string]any) string {
	if v, ok := view["id"].(string); ok {
		return v
	}
	return ""
}
