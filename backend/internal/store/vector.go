package store

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// EMBEDDING_DIM — размерность векторов (pgvector vector(384)).
const EmbeddingDim = 384

// VectorHit — результат KNN-поиска.
type VectorHit struct {
	ID         string  `json:"id"`
	Similarity float64 `json:"similarity"`
}

// VectorRepository — raw-SQL доступ к pgvector (Prisma не видит Unsupported-колонки).
// Аналог src/modules/matching/vector.repository.ts.
type VectorRepository struct {
	db        *DB
	mu        sync.Mutex
	sqlBroken bool // sticky-флаг: при ошибке SQL — сортировка в памяти (BUG-REF: док 05 §7)
}

func NewVectorRepository(db *DB) *VectorRepository {
	return &VectorRepository{db: db}
}

// literal — санация вектора: ровно 384 конечных значения с 6 знаками.
func literal(vector []float64) string {
	safe := make([]string, EmbeddingDim)
	for i := 0; i < EmbeddingDim; i++ {
		var v float64
		if i < len(vector) {
			v = vector[i]
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			v = 0
		}
		safe[i] = strconv.FormatFloat(v, 'f', 6, 64)
	}
	return "[" + strings.Join(safe, ",") + "]"
}

func (r *VectorRepository) SetCandidateVector(ctx context.Context, candidateID string, vector []float64) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE candidate_profiles SET profile_embedding = $1::vector WHERE id = $2`, literal(vector), candidateID)
	return err
}

func (r *VectorRepository) SetNeedVector(ctx context.Context, needID string, vector []float64) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE employer_needs SET need_embedding = $1::vector WHERE id = $2`, literal(vector), needID)
	return err
}

// TopBySimilarity — KNN-выборка ближайших кандидатов (косинусное расстояние pgvector).
func (r *VectorRepository) TopBySimilarity(ctx context.Context, vector []float64, limit int) []VectorHit {
	if limit <= 0 {
		limit = 100
	}
	r.mu.Lock()
	broken := r.sqlBroken
	r.mu.Unlock()

	if !broken {
		rows, err := r.db.Pool.Query(ctx, `
          SELECT id, (1 - (profile_embedding <=> $1::vector))::float8 AS similarity
          FROM candidate_profiles
          WHERE profile_embedding IS NOT NULL
          ORDER BY profile_embedding <=> $1::vector
          LIMIT $2`, literal(vector), limit)
		if err == nil {
			defer rows.Close()
			var hits []VectorHit
			for rows.Next() {
				var id string
				var sim float64
				if err := rows.Scan(&id, &sim); err != nil {
					continue
				}
				if math.IsNaN(sim) {
					sim = 0
				}
				hits = append(hits, VectorHit{ID: id, Similarity: sim})
			}
			return hits
		}
		r.mu.Lock()
		r.sqlBroken = true
		r.mu.Unlock()
		log.Printf("[VectorRepo] KNN-запрос не выполнен, переход на fallback: %s", err.Error())
	}
	return r.TopBySimilarityInMemory(ctx, vector, limit)
}

// TopBySimilarityInMemory — fallback: чтение векторов текстом и сортировка в памяти (dot-product).
func (r *VectorRepository) TopBySimilarityInMemory(ctx context.Context, vector []float64, limit int) []VectorHit {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, profile_embedding::text AS v FROM candidate_profiles WHERE profile_embedding IS NOT NULL`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	target := parseVectorText(literal(vector))
	hits := make([]VectorHit, 0)
	for rows.Next() {
		var id, v string
		if err := rows.Scan(&id, &v); err != nil {
			continue
		}
		stored := parseVectorText(v)
		hits = append(hits, VectorHit{ID: id, Similarity: dot(target, stored)})
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Similarity > hits[j].Similarity })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

// IndexedIds — подмножество ids с непустым вектором.
func (r *VectorRepository) IndexedIds(ctx context.Context, ids []string) map[string]bool {
	out := map[string]bool{}
	if len(ids) == 0 {
		return out
	}
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id FROM candidate_profiles WHERE profile_embedding IS NOT NULL`)
	if err != nil {
		return out
	}
	defer rows.Close()
	all := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			all[id] = true
		}
	}
	for _, id := range ids {
		if all[id] {
			out[id] = true
		}
	}
	return out
}

// parseVectorText разбирает текстовое представление вектора ('[1,2,...]'::text).
func parseVectorText(s string) []float64 {
	var arr []float64
	if err := json.Unmarshal([]byte(s), &arr); err != nil {
		// формат pgvector: без кавычек — нормализуем
		s = strings.TrimPrefix(strings.TrimSpace(s), "[")
		s = strings.TrimSuffix(strings.TrimSpace(s), "]")
		for _, p := range strings.Split(s, ",") {
			f, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
			if err != nil {
				return nil
			}
			arr = append(arr, f)
		}
	}
	return arr
}

// dot — скалярное произведение (векторы L2-нормализованы → dot == cosine).
func dot(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	sum := 0.0
	for i := range a {
		sum += a[i] * b[i]
	}
	if math.IsNaN(sum) || math.IsInf(sum, 0) {
		return 0
	}
	return sum
}

// FormatVector — публичный помощник: текстовое представление вектора (для отладки/логов).
func FormatVector(v []float64) string { return fmt.Sprintf("vec[%d]", len(v)) }
