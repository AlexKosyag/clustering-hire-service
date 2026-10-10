package matching

// Экспорт TF-IDF-векторизатора для cmd/seed-embeddings
// (порт prisma/embeddings.ts: один fit по общему корпусу профилей и потребностей;
// в самом пакете векторизатор используется через unexported-тип).

// SeedVectorizer — TF-IDF векторизатор для офлайн-предзаполнения.
type SeedVectorizer struct{ v *tfidfVectorizer }

// NewSeedVectorizer — конструктор векторизатора.
func NewSeedVectorizer() *SeedVectorizer { return &SeedVectorizer{v: newTfidfVectorizer()} }

// Fit — обучение на корпусе текстов.
func (s *SeedVectorizer) Fit(texts []string) { s.v.fit(texts) }

// Embed — вектор текста (384-dim, L2-нормализованный).
func (s *SeedVectorizer) Embed(text string) []float64 { return s.v.embed(text) }
