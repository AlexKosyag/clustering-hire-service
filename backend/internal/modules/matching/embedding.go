package matching

import (
	"log"
	"sync"

	"fsp-backend/internal/config"
)

// Порт backend/src/modules/matching/embedding.service.ts.
// ONNX/transformers в Go-сборке недоступны → мгновенная ПОСТОЯННАЯ деградация
// на TF-IDF (см. BUG-REF ниже — тексты логов дословно из TS).

type embedProvider = string // 'tfidf' | 'transformers'

const (
	providerTfidf        embedProvider = "tfidf"
	providerTransformers embedProvider = "transformers"
)

type embeddingService struct {
	mu             sync.Mutex
	tfidf          *tfidfVectorizer
	provider       embedProvider
	modelName      string
	readyCallbacks []func()
	switchPending  bool
}

func newEmbeddingService() *embeddingService {
	return &embeddingService{
		tfidf:    newTfidfVectorizer(),
		provider: providerTfidf,
	}
}

// onModuleInit — аналог OnModuleInit EmbeddingService.
func (e *embeddingService) onModuleInit(cfg *config.Config) {
	mode := cfg.EmbeddingProvider
	if mode == "" {
		mode = "auto"
	}
	if mode == "tfidf" {
		log.Printf("[Embedding] EMBEDDING_PROVIDER=tfidf — семантика отключена, используется лексический fallback")
		return
	}
	// Загрузка модели идёт в фоне: запросы в это время обслуживаются TF-IDF.
	// BUG: в Go нет ONNX-рантайма (@xenova/transformers) → деградация немедленная
	// и постоянная (в TS модели могли загрузиться динамически).
	e.initTransformers(mode == "transformers")
}

// initTransformers — паритет с TS, но импорт пакета всегда неуспешен.
func (e *embeddingService) initTransformers(required bool) {
	if e.switching() {
		return
	}
	e.setSwitching(true)
	defer e.setSwitching(false)

	// try { await import('@xenova/transformers') } catch (e) → warn + return
	log.Printf("[Embedding] Пакет @xenova/transformers недоступен: ONNX-рантайм отсутствует в Go-сборке. Работаем на TF-IDF.")
	if required {
		log.Printf("[Embedding] Ни одна модель не загрузилась — включён лексический fallback (TF-IDF).")
	}
}

func (e *embeddingService) switching() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.switchPending
}

func (e *embeddingService) setSwitching(v bool) {
	e.mu.Lock()
	e.switchPending = v
	e.mu.Unlock()
}

// onProviderReady — регистрация callback-а: вызывается при переходе TF-IDF -> transformers
// (нужна переиндексация). В Go переход невозможен → callback вызывается только если
// провайдер уже transformers (никогда).
func (e *embeddingService) onProviderReady(cb func()) {
	e.mu.Lock()
	e.readyCallbacks = append(e.readyCallbacks, cb)
	ready := e.provider == providerTransformers
	e.mu.Unlock()
	if ready {
		cb()
	}
}

func (e *embeddingService) notifyReady() {
	e.mu.Lock()
	cbs := append([]func(){}, e.readyCallbacks...)
	e.mu.Unlock()
	for _, cb := range cbs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[Embedding] Ошибка переиндексации после загрузки модели: %v", r)
				}
			}()
			cb()
		}()
	}
}

func (e *embeddingService) currentProvider() embedProvider {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.provider
}

func (e *embeddingService) currentModel() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.modelName
}

func (e *embeddingService) dim() int { return EMBEDDING_DIM }

// fitCorpus — обучение TF-IDF на корпусе (вызывается при индексации).
func (e *embeddingService) fitCorpus(texts []string) {
	e.tfidf.fit(texts)
}

func (e *embeddingService) corpusFitted() bool { return e.tfidf.isFitted() }

// embed — 384 finite-числа, никогда не бросает исключение.
func (e *embeddingService) embed(text string) []float64 {
	e.mu.Lock()
	isTF := e.provider == providerTfidf
	e.mu.Unlock()
	if !isTF {
		// transformers-ветка в Go-сборке недостижима (pipe == null всегда).
	}
	return e.tfidf.embed(text)
}

// embedLexicalSync — синхронный лексический вектор — для построения
// текстовых представлений без I/O.
func (e *embeddingService) embedLexicalSync(text string) []float64 {
	return e.tfidf.embed(text)
}

func (e *embeddingService) unit() []float64 { return unitVector() }
