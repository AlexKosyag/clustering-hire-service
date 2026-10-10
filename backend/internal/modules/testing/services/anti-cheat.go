package services

import "strings"

// CheatAnalysis — результат статистического анти-чита.
type CheatAnalysis struct {
	IsSuspicious bool     `json:"isSuspicious"`
	Flags        []string `json:"flags"`
	Confidence   float64  `json:"confidence"`
}

// TimedAnswer — ответ с таймингом и метаданными для анализа.
type TimedAnswer struct {
	TimeSpentSeconds *int
	Difficulty       float64
	PoolTag          *string
	IsCorrect        bool
}

// AntiCheat — анализ времени, паттернов сложности, «идеальных» пулов,
// волатильности theta, а также рандомизация порядка вариантов ответа.
type AntiCheat struct{}

// NewAntiCheat — конструктор (в TS — AntiCheatService без зависимостей).
func NewAntiCheat() *AntiCheat { return &AntiCheat{} }

// Analyze — статистический анализ сессии (docs 06 §6.1).
func (a *AntiCheat) Analyze(answers []TimedAnswer, thetaHistory []float64) CheatAnalysis {
	flags := []string{}

	// 1. Слишком быстрые ответы на сложных вопросах
	tooFast := 0
	for _, ans := range answers {
		if ans.TimeSpentSeconds != nil && *ans.TimeSpentSeconds < 5 && ans.Difficulty > 0.5 {
			tooFast++
		}
	}
	if tooFast > 3 {
		flags = append(flags, "MULTIPLE_TOO_FAST_ANSWERS")
	}

	// 2. Паттерн «сложные — идеально, лёгкие — провал»
	easy, hard := []TimedAnswer{}, []TimedAnswer{}
	for _, ans := range answers {
		if ans.Difficulty < 0.3 {
			easy = append(easy, ans)
		}
		if ans.Difficulty > 0.7 {
			hard = append(hard, ans)
		}
	}
	if len(easy) >= 3 && len(hard) >= 3 {
		easyCorrect := 0
		for _, ans := range easy {
			if ans.IsCorrect {
				easyCorrect++
			}
		}
		hardCorrect := 0
		for _, ans := range hard {
			if ans.IsCorrect {
				hardCorrect++
			}
		}
		if hardCorrect > easyCorrect && easyCorrect < 2 {
			flags = append(flags, "SUSPICIOUS_DIFFICULTY_PATTERN")
		}
	}

	// 3. Идеальный результат по всему пулу вопросов
	// (порядок групп — как в JS Map: по первому появлению poolTag)
	byPool := map[string][]TimedAnswer{}
	poolOrder := []string{}
	for _, ans := range answers {
		if ans.PoolTag == nil {
			continue
		}
		if _, ok := byPool[*ans.PoolTag]; !ok {
			poolOrder = append(poolOrder, *ans.PoolTag)
		}
		byPool[*ans.PoolTag] = append(byPool[*ans.PoolTag], ans)
	}
	for _, pool := range poolOrder {
		list := byPool[pool]
		allCorrect := len(list) > 0
		for _, ans := range list {
			if !ans.IsCorrect {
				allCorrect = false
				break
			}
		}
		if len(list) > 5 && allCorrect {
			flags = append(flags, "PERFECT_SCORE_POOL_"+strings.ToUpper(pool))
		}
	}

	// 4. Резкие скачки theta между ответами
	if len(thetaHistory) >= 4 {
		jumps := 0
		for i := 1; i < len(thetaHistory); i++ {
			if abs(thetaHistory[i]-thetaHistory[i-1]) > 2 {
				jumps++
			}
		}
		if jumps > 2 {
			flags = append(flags, "THETA_VOLATILITY")
		}
	}

	return CheatAnalysis{
		IsSuspicious: len(flags) >= 2,
		Flags:        flags,
		Confidence:   min1(JSRound(float64(len(flags))*0.35*100) / 100),
	}
}

// ShuffleOptions — порядок вариантов ответа, уникальный для каждой сессии
// (клиент не видит isCorrect). Детерминированная ротация:
// shift = sessionOrderKey % options.length.
func ShuffleOptions[T any](sessionID, questionID string, options []T) []T {
	if len(options) <= 1 {
		out := make([]T, len(options))
		copy(out, options)
		return out
	}
	shift := int(SessionOrderKey(sessionID, questionID) % uint32(len(options)))
	out := make([]T, 0, len(options))
	out = append(out, options[shift:]...)
	out = append(out, options[:shift]...)
	return out
}

// ThetaHistory — история theta после каждого ответа — вход для детекции волатильности.
func (a *AntiCheat) ThetaHistory(answers []IrtAnswer, estimate func([]IrtAnswer) float64) []float64 {
	history := []float64{}
	for i := 1; i <= len(answers); i++ {
		history = append(history, estimate(answers[:i]))
	}
	return history
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func min1(v float64) float64 {
	if v < 1 {
		return v
	}
	return 1
}
