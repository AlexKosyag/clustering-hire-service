package matching

import (
	"math"
	"testing"
)

// TF-IDF embedding (fallback semantic-слоя) — порт embedding/tfidf.spec.ts.

func contains(tokens []string, want string) bool {
	for _, t := range tokens {
		if t == want {
			return true
		}
	}
	return false
}

func TestTokenize(t *testing.T) {
	t.Run("нормализует регистр и убирает пунктуацию", func(t *testing.T) {
		tokens := tokenize("Node.js и PostgreSQL!")
		if !contains(tokens, "node") {
			t.Errorf("ожидался токен 'node', получено %v", tokens)
		}
		if !contains(tokens, "postgresql") {
			t.Errorf("ожидался токен 'postgresql', получено %v", tokens)
		}
		if contains(tokens, "и") {
			t.Errorf("токен 'и' должен отбрасываться, получено %v", tokens)
		}
	})

	t.Run("строит биграммы для словосочетаний", func(t *testing.T) {
		tokens := tokenize("высокая нагрузка")
		if !contains(tokens, "высокая") || !contains(tokens, "нагрузка") || !contains(tokens, "высокая_нагрузка") {
			t.Errorf("ожидались токены высокая/нагрузка/высокая_нагрузка, получено %v", tokens)
		}
	})

	t.Run("пустой ввод даёт пустой список", func(t *testing.T) {
		if got := tokenize(""); len(got) != 0 {
			t.Errorf("tokenize(\"\") = %v, ожидался []", got)
		}
		if got := tokenize("   "); len(got) != 0 {
			t.Errorf("tokenize(\"   \") = %v, ожидался []", got)
		}
	})

	t.Run("однобуквенные слова отбрасываются", func(t *testing.T) {
		want := []string{"bb", "dd", "bb_dd"}
		got := tokenize("a bb c dd")
		if len(got) != len(want) {
			t.Fatalf("tokenize = %v, ожидался %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("tokenize[%d] = %q, ожидался %q", i, got[i], want[i])
			}
		}
	})
}

func TestTfidfVectorizer(t *testing.T) {
	corpus := []string{
		"backend разработчик nodejs postgresql redis",
		"frontend разработчик react typescript css",
		"devops инженер kubernetes docker terraform",
	}

	t.Run("после fit помечен как обученный", func(t *testing.T) {
		v := newTfidfVectorizer()
		if v.isFitted() {
			t.Fatal("новый векторайзер не должен быть обучен")
		}
		v.fit(corpus)
		if !v.isFitted() {
			t.Error("после fit векторайзер должен быть обучен")
		}
	})

	t.Run("пустой корпус не обучает векторайзер", func(t *testing.T) {
		v := newTfidfVectorizer()
		v.fit([]string{})
		if v.isFitted() {
			t.Error("пустой корпус не должен обучать векторайзер")
		}
	})

	t.Run("возвращает вектор размерности 384 с единичной нормой", func(t *testing.T) {
		v := newTfidfVectorizer()
		v.fit(corpus)
		vector := v.embed(corpus[0])
		if len(vector) != EMBEDDING_DIM {
			t.Fatalf("длина вектора = %d, ожидалась %d", len(vector), EMBEDDING_DIM)
		}
		norm := 0.0
		for _, x := range vector {
			if !isFinite(x) {
				t.Fatalf("вектор содержит нечисловую компоненту: %v", vector)
			}
			norm += x * x
		}
		norm = math.Sqrt(norm)
		if math.Abs(norm-1) > 1e-5 {
			t.Errorf("норма вектора = %v, ожидалась ~1", norm)
		}
	})

	t.Run("одинаковые тексты дают косинусную близость ~1", func(t *testing.T) {
		v := newTfidfVectorizer()
		v.fit(corpus)
		sim := cosineSimilarity(v.embed(corpus[0]), v.embed(corpus[0]))
		if math.Abs(sim-1) > 1e-6 {
			t.Errorf("cosine(a, a) = %v, ожидалась ~1", sim)
		}
	})

	t.Run("релевантный текст ближе к запросу, чем нерелевантный", func(t *testing.T) {
		v := newTfidfVectorizer()
		v.fit(append([]string{}, append(corpus, "ищем backend разработчика с опытом работы с redis и postgresql")...))
		query := v.embed("backend разработчик postgresql redis")
		relevant := v.embed("ищем backend разработчика с опытом работы с redis и postgresql")
		unrelated := v.embed("frontend разработчик react typescript css")
		if cosineSimilarity(query, relevant) <= cosineSimilarity(query, unrelated) {
			t.Errorf("релевантный текст (%v) должен быть ближе к запросу, чем нерелевантный (%v)",
				cosineSimilarity(query, relevant), cosineSimilarity(query, unrelated))
		}
	})

	t.Run("работает и до вызова fit (деградация, а не исключение)", func(t *testing.T) {
		v := newTfidfVectorizer()
		vector := v.embed("nodejs backend")
		if len(vector) != EMBEDDING_DIM {
			t.Fatalf("длина вектора = %d, ожидалась %d", len(vector), EMBEDDING_DIM)
		}
		for _, x := range vector {
			if !isFinite(x) {
				t.Fatalf("вектор содержит нечисловую компоненту: %v", vector)
			}
		}
	})
}

func TestNormalize(t *testing.T) {
	t.Run("нормализует в единичную длину", func(t *testing.T) {
		result := normalize([]float64{3, 4})
		if math.Abs(result[0]-0.6) > 1e-6 || math.Abs(result[1]-0.8) > 1e-6 {
			t.Errorf("normalize([3,4]) = %v, ожидался [0.6, 0.8]", result)
		}
	})

	t.Run("guard от нулевого и нечислового вектора", func(t *testing.T) {
		assertUnit := func(name string, got []float64) {
			want := unitVector()
			if len(got) != len(want) {
				t.Fatalf("%s: длина = %d, ожидалась %d", name, len(got), len(want))
			}
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("%s: %v не равно unitVector()", name, got)
				}
			}
		}
		assertUnit("normalize([0,0,0])", normalize([]float64{0, 0, 0}))
		assertUnit("normalize([NaN,0])", normalize([]float64{math.NaN(), 0}))

		guarded := normalize([]float64{math.Inf(1), 1})
		if len(guarded) != 2 {
			t.Errorf("длина guarded = %d, ожидалась 2", len(guarded))
		}
		for _, x := range guarded {
			if !isFinite(x) {
				t.Errorf("guarded содержит нечисловую компоненту: %v", guarded)
			}
		}
	})

	t.Run("заменяет NaN-компоненты нулями", func(t *testing.T) {
		result := normalize([]float64{1, math.NaN(), 1})
		for _, x := range result {
			if !isFinite(x) {
				t.Errorf("normalize заменил NaN не нулём: %v", result)
			}
		}
	})
}

func TestCosineSimilarity(t *testing.T) {
	t.Run("единичный вектор сам с собой даёт 1", func(t *testing.T) {
		v := unitVector()
		if sim := cosineSimilarity(v, v); math.Abs(sim-1) > 1e-10 {
			t.Errorf("cosine(unit, unit) = %v, ожидалась 1", sim)
		}
	})

	t.Run("ортогональные векторы дают 0", func(t *testing.T) {
		if sim := cosineSimilarity([]float64{1, 0}, []float64{0, 1}); math.Abs(sim) > 1e-10 {
			t.Errorf("cosine([1,0],[0,1]) = %v, ожидалась 0", sim)
		}
	})

	t.Run("защищает от разной длины и пустых входов", func(t *testing.T) {
		if sim := cosineSimilarity([]float64{1, 0}, []float64{1}); sim != 0 {
			t.Errorf("разная длина: cosine = %v, ожидалась 0", sim)
		}
		if sim := cosineSimilarity([]float64{}, []float64{}); sim != 0 {
			t.Errorf("пустые векторы: cosine = %v, ожидалась 0", sim)
		}
		if sim := cosineSimilarity(nil, []float64{1}); sim != 0 {
			t.Errorf("nil-вектор: cosine = %v, ожидалась 0", sim)
		}
	})

	t.Run("игнорирует нечисловые компоненты", func(t *testing.T) {
		value := cosineSimilarity([]float64{1, math.NaN()}, []float64{1, 5})
		if !isFinite(value) {
			t.Fatalf("cosine = %v, ожидалось конечное число", value)
		}
		if math.Abs(value-1) > 1e-6 {
			t.Errorf("cosine([1,NaN],[1,5]) = %v, ожидалась ~1", value)
		}
	})
}
