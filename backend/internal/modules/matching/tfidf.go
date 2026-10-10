package matching

import (
	"math"
	"strings"
	"unicode/utf16"

	"fsp-backend/internal/store"
)

// Порт backend/src/modules/matching/embedding/tfidf.ts (as-is, включая баги).
// Лексический fallback для semantic matching (384-dim, детерминированный).

const EMBEDDING_DIM = store.EmbeddingDim // 384

// fnv1a — 32-битный FNV-1a по UTF-16 code units (паритет с JS charCodeAt + Math.imul).
// Возвращает uint32 (JS: hash >>> 0).
func fnv1a(str string) uint32 {
	hash := uint32(0x811c9dc5)
	for _, u := range utf16.Encode([]rune(str)) {
		hash ^= uint32(u)
		// Math.imul(hash, 0x01000193) — умножение со свёрткой в 32 бита
		hash = uint32(int32(hash) * int32(0x01000193))
	}
	return hash
}

// tokenize — нормализует регистр, убирает пунктуацию, строит биграммы (word_word).
func tokenize(text string) []string {
	if text == "" {
		return []string{}
	}
	normalized := strings.ToLower(text)
	normalized = strings.ReplaceAll(normalized, "ё", "е")
	normalized = replaceNonAlnum(normalized)
	normalized = strings.TrimSpace(normalized)
	if normalized == "" {
		return []string{}
	}
	words := []string{}
	for _, w := range strings.Fields(normalized) {
		if len([]rune(w)) > 1 { // фильтр длины > 1 (JS: w.length > 1)
			words = append(words, w)
		}
	}
	tokens := make([]string, 0, len(words)*2)
	tokens = append(tokens, words...)
	for i := 0; i < len(words)-1; i++ {
		// биграммы для устойчивости к фразам
		tokens = append(tokens, words[i]+"_"+words[i+1])
	}
	return tokens
}

// replaceNonAlnum — замена последовательностей вне [a-zа-я0-9+#] на пробел.
func replaceNonAlnum(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '+' || r == '#' ||
			(r >= 'а' && r <= 'я') {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	return b.String()
}

type tfidfVectorizer struct {
	idf    map[string]float64
	docs   int
	fitted bool
}

func newTfidfVectorizer() *tfidfVectorizer {
	return &tfidfVectorizer{
		idf:    map[string]float64{},
		docs:   0,
		fitted: false,
	}
}

func (v *tfidfVectorizer) isFitted() bool { return v.fitted }

// fit обучает IDF на корпусе профилей/описаний.
func (v *tfidfVectorizer) fit(texts []string) {
	df := map[string]int{}
	for _, text := range texts {
		seen := map[string]bool{}
		for _, token := range tokenize(text) {
			if !seen[token] {
				seen[token] = true
				df[token]++
			}
		}
	}
	v.docs = len(texts)
	v.idf = map[string]float64{}
	for token, count := range df {
		v.idf[token] = math.Log((1+float64(v.docs))/(1+float64(count))) + 1
	}
	v.fitted = v.docs > 0
}

func (v *tfidfVectorizer) idfOf(token string) float64 {
	if val, ok := v.idf[token]; ok {
		return val
	}
	return math.Log(1+float64(v.docs)) + 1
}

// embed возвращает L2-нормированный вектор 384 dim. Никогда не бросает исключение.
func (v *tfidfVectorizer) embed(text string) []float64 {
	tokens := tokenize(text)
	tf := map[string]int{}
	order := []string{} // порядок первой встречаемости (Map-порядок в JS)
	for _, t := range tokens {
		if _, ok := tf[t]; !ok {
			order = append(order, t)
		}
		tf[t]++
	}

	vector := make([]float64, EMBEDDING_DIM)
	for _, token := range order {
		count := tf[token]
		weight := (1 + math.Log(float64(count))) * v.idfOf(token)
		hash := fnv1a(token)
		index := int(hash % EMBEDDING_DIM)
		sign := 1.0
		if hash&0x80000000 != 0 { // (hash & 0x80000000) >>> 31
			sign = -1
		}
		vector[index] += sign * weight
	}
	return normalize(vector)
}

// normalize — L2-нормализация с guard-ом от нулевой нормы (защита от NaN в cosine).
func normalize(vector []float64) []float64 {
	sum := 0.0
	for _, v := range vector {
		if isFinite(v) {
			sum += v * v
		}
	}
	if sum < 1e-12 || !isFinite(sum) {
		return unitVector()
	}
	norm := math.Sqrt(sum)
	out := make([]float64, len(vector))
	for i, v := range vector {
		if isFinite(v) {
			out[i] = v / norm
		} else {
			out[i] = 0
		}
	}
	return out
}

// unitVector — единичный вектор 384 dim (элемент 0 = 1).
func unitVector() []float64 {
	vector := make([]float64, EMBEDDING_DIM)
	vector[0] = 1
	return vector
}

// cosineSimilarity — косинусное сходство с защитой от NaN/разной длины/пустых входов.
func cosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	dot, na, nb := 0.0, 0.0, 0.0
	for i := 0; i < len(a); i++ {
		if !isFinite(a[i]) || !isFinite(b[i]) {
			continue
		}
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na < 1e-12 || nb < 1e-12 {
		return 0
	}
	value := dot / (math.Sqrt(na) * math.Sqrt(nb))
	if !isFinite(value) {
		return 0
	}
	return value
}

func isFinite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }
