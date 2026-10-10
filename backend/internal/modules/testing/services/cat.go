// Package services — внутренние движки testing engine (CAT/IRT, скоринг,
// анти-чит, генерация заданий, песочница кода, LLM-оценка) —
// порт backend/src/modules/testing/services/*.service.ts.
package services

import (
	"math"
	"sort"
)

// IrtQuestion — IRT-параметры вопроса (3PL).
type IrtQuestion struct {
	ID             string
	Difficulty     float64
	Discrimination float64
	Guessing       float64
}

// IrtAnswer — ответ кандидата с IRT-параметрами вопроса.
type IrtAnswer struct {
	Question  IrtQuestion
	IsCorrect bool
}

// IrtCapable — источник IRT-параметров (IrtQuestion, PoolQuestion).
type IrtCapable interface {
	IrtParams() IrtQuestion
}

func (q IrtQuestion) IrtParams() IrtQuestion { return q }

// SessionOrderKey — детерминированный порядок вопроса внутри сессии
// (анти-чит: уникальная последовательность у каждого кандидата).
// FNV-1a 32 от "sessionId:questionId"; charCodeAt — UTF-16 code units,
// Math.imul(hash, 0x01000193) — 32-битное умножение, >>> 0 — uint32.
func SessionOrderKey(sessionID, questionID string) uint32 {
	input := sessionID + ":" + questionID
	hash := uint32(0x811c9dc5)
	for _, unit := range jsUTF16(input) {
		hash ^= uint32(unit)
		hash *= 0x01000193
	}
	return hash
}

// CAT — Computer Adaptive Testing (3PL IRT):
//   - следующий вопрос выбирается по максимальной Fisher Information при текущем theta,
//   - theta оценивается методом максимального правдоподобия (Newton-Raphson).
//
// Все вычисления детерминированы — повторный запрос даёт тот же вопрос.
type CAT struct{}

// NewCAT — конструктор (в TS — CATService без зависимостей).
func NewCAT() *CAT { return &CAT{} }

// Probability — P(правильный ответ) по 3PL: c + (1-c)/(1+e^{-a(theta-b)}).
func (c *CAT) Probability(theta, a, b, guessing float64) float64 {
	ability := theta
	if !isFinite(ability) {
		ability = 0
	}
	discrimination := a
	if !isFinite(discrimination) || !(discrimination > 0) {
		discrimination = 1
	}
	difficulty := b
	if !isFinite(difficulty) {
		difficulty = 0.5
	}
	guess := guessing
	if !isFinite(guess) || guess < 0 || guess >= 1 {
		guess = 0.25
	}
	p := guess + (1-guess)/(1+math.Exp(-discrimination*(ability-difficulty)))
	if !isFinite(p) {
		return 1 - 1e-6
	}
	return math.Min(1-1e-6, math.Max(1e-6, p))
}

// FisherInformation — Fisher Information для 3PL-модели.
func (c *CAT) FisherInformation(theta float64, question IrtQuestion) float64 {
	a := question.Discrimination
	b := question.Difficulty
	guessing := question.Guessing
	p := c.Probability(theta, a, b, guessing)
	exp := math.Exp(-a * (theta - b))
	denom := math.Pow(1+exp, 2)
	if denom == 0 {
		return 0
	}
	pPrime := (a * (1 - guessing) * exp) / denom
	value := (pPrime * pPrime) / (p * (1 - p))
	if !isFinite(value) {
		return 0
	}
	return value
}

// EstimateTheta — оценка theta (MLE, Newton-Raphson / Fisher scoring).
// Используется ожидаемая информация I(theta) >= 0 вместо наблюдаемого гессиана:
// гессиан log-правдоподобия 3PL меняет знак в области p < c и ломал оценку
// (например, «все ответы неверны» давало theta = 0 вместо нижней границы).
func (c *CAT) EstimateTheta(answers []IrtAnswer) float64 {
	if len(answers) == 0 {
		return 0
	}
	theta := 0.0
	const tolerance = 0.001
	const maxIter = 50

	for iter := 0; iter < maxIter; iter++ {
		first := 0.0 // dL/d(theta)
		info := 0.0  // ожидаемая информация I(theta) >= 0

		for _, answer := range answers {
			a := answer.Question.Discrimination
			b := answer.Question.Difficulty
			guessing := answer.Question.Guessing
			p := c.Probability(theta, a, b, guessing)
			u := 0.0
			if answer.IsCorrect {
				u = 1
			}
			cSafe := guessing
			if !isFinite(cSafe) || cSafe < 0 || cSafe >= 1 {
				cSafe = 0.25
			}
			pc := p - cSafe
			if pc <= 0 || p <= 0 || p >= 1 {
				continue
			}

			first += (a * (u - p) * pc) / (p * (1 - cSafe))
			info += (a * a * pc * pc * (1 - p)) / ((1 - cSafe) * p)
		}

		if !isFinite(first) || !isFinite(info) || info < 1e-9 {
			break
		}

		step := first / info
		if !isFinite(step) {
			break
		}

		// демпфер: один шаг не больше ±0.5, иначе на старте возможен разлёт
		theta += math.Max(-0.5, math.Min(0.5, step))
		theta = math.Max(-4, math.Min(4, theta))

		if math.Abs(step) < tolerance {
			break
		}
	}

	if !isFinite(theta) {
		return 0
	}
	return theta
}

// SelectNext — выбор следующего вопроса из пула (повторные вопросы исключаются).
// При равной информации побеждает порядок сессии — /current и /answer согласованы.
func SelectNext[T IrtCapable](available []T, answered []IrtAnswer, sessionID string) *T {
	answeredIDs := make(map[string]bool, len(answered))
	for _, a := range answered {
		answeredIDs[a.Question.ID] = true
	}
	pool := make([]T, 0, len(available))
	for _, q := range available {
		if !answeredIDs[q.IrtParams().ID] {
			pool = append(pool, q)
		}
	}
	if len(pool) == 0 {
		return nil
	}

	cat := &CAT{}

	if len(answered) == 0 {
		// Первый вопрос — ближайший к средней сложности (docs: difficulty ~0.5)
		sorted := make([]T, len(pool))
		copy(sorted, pool)
		sort.SliceStable(sorted, func(i, j int) bool {
			xi, xj := sorted[i].IrtParams(), sorted[j].IrtParams()
			diff := math.Abs(xi.Difficulty-0.5) - math.Abs(xj.Difficulty-0.5)
			if diff != 0 {
				return diff < 0
			}
			return SessionOrderKey(sessionID, xi.ID) < SessionOrderKey(sessionID, xj.ID)
		})
		return &sorted[0]
	}

	theta := cat.EstimateTheta(answered)
	best := -1
	bestInfo := math.Inf(-1)

	for i := range pool {
		info := cat.FisherInformation(theta, pool[i].IrtParams())
		isBetter := best == -1 ||
			info > bestInfo+1e-12 ||
			(math.Abs(info-bestInfo) <= 1e-12 &&
				SessionOrderKey(sessionID, pool[i].IrtParams().ID) < SessionOrderKey(sessionID, pool[best].IrtParams().ID))
		if isBetter {
			bestInfo = info
			best = i
		}
	}
	return &pool[best]
}

func isFinite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
