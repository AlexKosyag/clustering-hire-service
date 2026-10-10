package matching

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"fsp-backend/internal/modules/candidateview"
	"fsp-backend/internal/store"
)

// Порт backend/src/modules/matching/indexing.service.ts (as-is).
// Индексация кандидатов: текстовое представление + запись вектора в pgvector.

type indexingService struct {
	db        *store.DB
	embedding *embeddingService
	vectors   *store.VectorRepository

	mu         sync.Mutex
	reindexing bool
}

// onModuleInit — OnModuleInit: подписка на готовность модели + стартовая индексация.
func (i *indexingService) onModuleInit() {
	// После загрузки ONNX-модели все вектора пересчитываются заново
	// (совместимость размерностей/семантики). В Go модели нет → не вызывается.
	i.embedding.onProviderReady(func() {
		if err := i.reindexAll(context.Background()); err != nil {
			log.Printf("[Indexing] Переиндексация не удалась: %s", err.Error())
		}
	})
	// Начальная индексация при старте (фоном, чтобы не задерживать boot)
	i.scheduleInitialIndex()
}

// buildText — текстовое представление профиля для embedding-а.
func (i *indexingService) buildText(profile *candidateview.Candidate, achievementNames []string) string {
	parts := []string{}

	if profile.Specialization != nil && *profile.Specialization != "" {
		parts = append(parts, *profile.Specialization)
	}

	grade := profile.VerifiedGrade
	if grade == nil || *grade == "" {
		grade = profile.DeclaredGrade
	}
	if grade != nil && *grade != "" {
		parts = append(parts, *grade)
	}

	if len(profile.Stack) > 0 {
		parts = append(parts, strings.Join(profile.Stack, " "))
	}
	if len(profile.Roles) > 0 {
		parts = append(parts, strings.Join(profile.Roles, " "))
	}
	if len(profile.SoftSkills) > 0 {
		parts = append(parts, strings.Join(profile.SoftSkills, " "))
	}
	if profile.City != nil && *profile.City != "" {
		parts = append(parts, *profile.City)
	}
	if profile.ExperienceYears != nil {
		parts = append(parts, jsNumString(*profile.ExperienceYears)+" лет опыта")
	}
	if profile.Bio != nil && *profile.Bio != "" {
		parts = append(parts, *profile.Bio)
	}
	if len(achievementNames) > 0 {
		parts = append(parts, strings.Join(achievementNames, " "))
	}

	return strings.Join(parts, " ")
}

// ensureCorpus — обучает TF-IDF на корпусе профилей (нужен только лексическому fallback-у).
func (i *indexingService) ensureCorpus(ctx context.Context) error {
	if i.embedding.currentProvider() == providerTransformers {
		return nil
	}
	if i.embedding.corpusFitted() {
		return nil
	}
	profiles, err := loadAllCandidates(ctx, i.db, 5000)
	if err != nil {
		return err
	}
	texts := make([]string, 0, len(profiles))
	for _, p := range profiles {
		texts = append(texts, i.buildText(p, achievementNames(p)))
	}
	i.embedding.fitCorpus(texts)
	return nil
}

// indexCandidate — полная индексация одного кандидата.
func (i *indexingService) indexCandidate(ctx context.Context, candidateID string) error {
	if err := i.ensureCorpus(ctx); err != nil {
		return err
	}
	candidates, err := loadCandidatesByID(ctx, i.db, []string{candidateID})
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return nil // findUnique → null
	}
	profile := candidates[0]
	text := i.buildText(profile, achievementNames(profile))
	vector := i.embedding.embed(text)
	return i.vectors.SetCandidateVector(ctx, profile.ID, vector)
}

// indexBatch — пакетная индексация (ошибка одного кандидата не прерывает пакет).
func (i *indexingService) indexBatch(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if err := i.ensureCorpus(ctx); err != nil {
		return err
	}
	for _, id := range ids {
		if err := i.indexCandidate(ctx, id); err != nil {
			log.Printf("[Indexing] Индексация %s не удалась: %s", id, err.Error())
		}
	}
	return nil
}

// reindexAll — полная переиндексация (до 5000 профилей).
func (i *indexingService) reindexAll(ctx context.Context) error {
	i.mu.Lock()
	if i.reindexing {
		i.mu.Unlock()
		return nil
	}
	i.reindexing = true
	i.mu.Unlock()
	defer func() {
		i.mu.Lock()
		i.reindexing = false
		i.mu.Unlock()
	}()

	started := time.Now()
	profiles, err := loadAllCandidates(ctx, i.db, 5000)
	if err != nil {
		return err
	}
	texts := make([]string, 0, len(profiles))
	for _, p := range profiles {
		texts = append(texts, i.buildText(p, achievementNames(p)))
	}
	i.embedding.fitCorpus(texts)

	for idx, profile := range profiles {
		if err := func() error {
			vector := i.embedding.embed(texts[idx])
			return i.vectors.SetCandidateVector(ctx, profile.ID, vector)
		}(); err != nil {
			log.Printf("[Indexing] Индексация %s не удалась: %s", profile.ID, err.Error())
		}
	}
	log.Printf("[Indexing] Проиндексировано кандидатов: %d за %dms", len(profiles), time.Since(started).Milliseconds())
	return nil
}

// scheduleInitialIndex — запускает полную индексацию в фоне (один раз при старте).
func (i *indexingService) scheduleInitialIndex() {
	time.AfterFunc(2*time.Second, func() {
		if err := i.reindexAll(context.Background()); err != nil {
			log.Printf("[Indexing] Начальная индексация: %s", err.Error())
		}
	})
}

// achievementNames — имена достижений кандидата.
func achievementNames(c *candidateview.Candidate) []string {
	names := make([]string, 0, len(c.Achievements))
	for _, a := range c.Achievements {
		names = append(names, a.Name)
	}
	return names
}
