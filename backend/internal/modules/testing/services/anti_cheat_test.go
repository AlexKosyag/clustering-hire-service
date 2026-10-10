package services

import (
	"math"
	"strings"
	"testing"
)

// Хелперы в духе TS-спека: answer({ timeSpentSeconds, difficulty, poolTag, isCorrect }).

func iptr(v int) *int         { return &v }
func strptr(v string) *string { return &v }

func timedAnswer(opts ...func(*TimedAnswer)) TimedAnswer {
	a := TimedAnswer{TimeSpentSeconds: iptr(30), Difficulty: 0.5, PoolTag: strptr("backend_concept_1")}
	for _, o := range opts {
		o(&a)
	}
	return a
}

func withSeconds(n int) func(*TimedAnswer) {
	return func(a *TimedAnswer) { a.TimeSpentSeconds = iptr(n) }
}
func withDifficulty(v float64) func(*TimedAnswer) {
	return func(a *TimedAnswer) { a.Difficulty = v }
}
func withPool(s string) func(*TimedAnswer)  { return func(a *TimedAnswer) { a.PoolTag = strptr(s) } }
func withCorrect(v bool) func(*TimedAnswer) { return func(a *TimedAnswer) { a.IsCorrect = v } }

func hasFlag(flags []string, want string) bool {
	for _, f := range flags {
		if f == want {
			return true
		}
	}
	return false
}

func hasFlagWithPrefix(flags []string, prefix string) bool {
	for _, f := range flags {
		if strings.HasPrefix(f, prefix) {
			return true
		}
	}
	return false
}

// Порт backend/src/modules/testing/services/anti-cheat.spec.ts (AntiCheatService).

func TestAntiCheatTooFastAnswers(t *testing.T) {
	service := NewAntiCheat()

	t.Run("кладёт флаг при >3 быстрых ответах на сложные вопросы", func(t *testing.T) {
		answers := []TimedAnswer{
			timedAnswer(withSeconds(2), withDifficulty(0.8)),
			timedAnswer(withSeconds(1), withDifficulty(0.9)),
			timedAnswer(withSeconds(3), withDifficulty(0.7)),
			timedAnswer(withSeconds(2), withDifficulty(0.6)),
			timedAnswer(withSeconds(40), withDifficulty(0.8)),
		}
		result := service.Analyze(answers, nil)
		if !hasFlag(result.Flags, "MULTIPLE_TOO_FAST_ANSWERS") {
			t.Errorf("flags = %v, ожидался MULTIPLE_TOO_FAST_ANSWERS", result.Flags)
		}
	})

	t.Run("не срабатывает на нормальном темпе", func(t *testing.T) {
		answers := make([]TimedAnswer, 0, 10)
		for i := 0; i < 10; i++ {
			answers = append(answers, timedAnswer(withSeconds(30), withDifficulty(0.8)))
		}
		result := service.Analyze(answers, nil)
		if hasFlag(result.Flags, "MULTIPLE_TOO_FAST_ANSWERS") {
			t.Errorf("flags = %v, флаг не ожидался", result.Flags)
		}
		if result.IsSuspicious {
			t.Error("isSuspicious = true, ожидалось false")
		}
	})
}

func TestAntiCheatDifficultyPattern(t *testing.T) {
	service := NewAntiCheat()

	t.Run("детектит «лёгкие мимо, сложные идеально»", func(t *testing.T) {
		answers := []TimedAnswer{
			timedAnswer(withDifficulty(0.1), withCorrect(false)),
			timedAnswer(withDifficulty(0.2), withCorrect(false)),
			timedAnswer(withDifficulty(0.15), withCorrect(false)),
			timedAnswer(withDifficulty(0.8), withCorrect(true)),
			timedAnswer(withDifficulty(0.9), withCorrect(true)),
			timedAnswer(withDifficulty(0.85), withCorrect(true)),
		}
		result := service.Analyze(answers, nil)
		if !hasFlag(result.Flags, "SUSPICIOUS_DIFFICULTY_PATTERN") {
			t.Errorf("flags = %v, ожидался SUSPICIOUS_DIFFICULTY_PATTERN", result.Flags)
		}
	})

	t.Run("не срабатывает при обычном распределении верных ответов", func(t *testing.T) {
		answers := []TimedAnswer{
			timedAnswer(withDifficulty(0.1), withCorrect(true)),
			timedAnswer(withDifficulty(0.2), withCorrect(true)),
			timedAnswer(withDifficulty(0.15), withCorrect(true)),
			timedAnswer(withDifficulty(0.8), withCorrect(true)),
			timedAnswer(withDifficulty(0.9), withCorrect(false)),
			timedAnswer(withDifficulty(0.85), withCorrect(true)),
		}
		if hasFlag(service.Analyze(answers, nil).Flags, "SUSPICIOUS_DIFFICULTY_PATTERN") {
			t.Error("флаг SUSPICIOUS_DIFFICULTY_PATTERN не ожидался")
		}
	})
}

func TestAntiCheatPerfectPool(t *testing.T) {
	service := NewAntiCheat()

	t.Run("флаг при >5 идеальных ответах из одного пула", func(t *testing.T) {
		answers := make([]TimedAnswer, 0, 6)
		for i := 0; i < 6; i++ {
			answers = append(answers, timedAnswer(withPool("backend_concept_1"), withCorrect(true)))
		}
		result := service.Analyze(answers, nil)
		if !hasFlagWithPrefix(result.Flags, "PERFECT_SCORE_POOL_") {
			t.Errorf("flags = %v, ожидался префикс PERFECT_SCORE_POOL_", result.Flags)
		}
		if !hasFlag(result.Flags, "PERFECT_SCORE_POOL_BACKEND_CONCEPT_1") {
			t.Errorf("flags = %v, ожидался PERFECT_SCORE_POOL_BACKEND_CONCEPT_1", result.Flags)
		}
	})

	t.Run("нет флага при 5 и менее ответах в пуле", func(t *testing.T) {
		answers := make([]TimedAnswer, 0, 5)
		for i := 0; i < 5; i++ {
			answers = append(answers, timedAnswer(withPool("backend_concept_1"), withCorrect(true)))
		}
		if hasFlagWithPrefix(service.Analyze(answers, nil).Flags, "PERFECT_SCORE_POOL_") {
			t.Error("флаг PERFECT_SCORE_POOL_* не ожидался")
		}
	})
}

func TestAntiCheatThetaVolatility(t *testing.T) {
	service := NewAntiCheat()

	t.Run("флаг при >2 резких скачков", func(t *testing.T) {
		history := []float64{0, 3, -1, 2.5, -2, 3.5, 0}
		if !hasFlag(service.Analyze(nil, history).Flags, "THETA_VOLATILITY") {
			t.Error("ожидался флаг THETA_VOLATILITY")
		}
	})

	t.Run("нет флага при стабильной оценке", func(t *testing.T) {
		history := []float64{0, 0.2, 0.1, 0.3, 0.2, 0.4}
		if hasFlag(service.Analyze(nil, history).Flags, "THETA_VOLATILITY") {
			t.Error("флаг THETA_VOLATILITY не ожидался")
		}
	})
}

func TestAntiCheatSuspicionScore(t *testing.T) {
	service := NewAntiCheat()

	t.Run("isSuspicious только при двух и более флагах", func(t *testing.T) {
		single := service.Analyze([]TimedAnswer{
			timedAnswer(withSeconds(1), withDifficulty(0.9)),
			timedAnswer(withSeconds(1), withDifficulty(0.9)),
			timedAnswer(withSeconds(1), withDifficulty(0.9)),
			timedAnswer(withSeconds(1), withDifficulty(0.9)),
		}, nil)
		if len(single.Flags) != 1 {
			t.Errorf("flags = %v, ожидался ровно 1 флаг", single.Flags)
		}
		if single.IsSuspicious {
			t.Error("isSuspicious = true, ожидалось false")
		}
		if math.Abs(single.Confidence-0.35) > 0.005 {
			t.Errorf("confidence = %v, ожидалось ≈0.35", single.Confidence)
		}
	})

	t.Run("confidence ограничена единицей", func(t *testing.T) {
		history := []float64{0, 3, -1, 2.5, -2, 3.5, 0}
		fast := make([]TimedAnswer, 0, 4)
		for i := 0; i < 4; i++ {
			fast = append(fast, timedAnswer(withSeconds(1), withDifficulty(0.9), withPool("p1"), withCorrect(true)))
		}
		pool := make([]TimedAnswer, 0, 6)
		for i := 0; i < 6; i++ {
			pool = append(pool, timedAnswer(withSeconds(40), withDifficulty(0.5), withPool("p2"), withCorrect(true)))
		}
		result := service.Analyze(append(fast, pool...), history)
		if len(result.Flags) < 3 {
			t.Errorf("flags = %v, ожидалось ≥3 флагов", result.Flags)
		}
		if !result.IsSuspicious {
			t.Error("isSuspicious = false, ожидалось true")
		}
		if result.Confidence > 1 {
			t.Errorf("confidence = %v, ожидалось ≤ 1", result.Confidence)
		}
		if result.Confidence != 1 {
			t.Errorf("confidence = %v, ожидалось 1", result.Confidence)
		}
	})

	t.Run("чистая сессия не подозрительна", func(t *testing.T) {
		answers := make([]TimedAnswer, 0, 20)
		for i := 0; i < 20; i++ {
			answers = append(answers, timedAnswer(
				withSeconds(20+(i%5)),
				withDifficulty(0.3+float64(i%4)*0.2),
				withPool("concept_"+string(rune('0'+i%5))),
				withCorrect(i%3 != 0),
			))
		}
		result := service.Analyze(answers, []float64{0, 0.1, -0.1, 0.2, 0.15})
		if len(result.Flags) != 0 {
			t.Errorf("flags = %v, ожидался пустой список", result.Flags)
		}
		if result.IsSuspicious {
			t.Error("isSuspicious = true, ожидалось false")
		}
		if result.Confidence != 0 {
			t.Errorf("confidence = %v, ожидалось 0", result.Confidence)
		}
	})
}

func TestAntiCheatShuffleOptions(t *testing.T) {
	t.Run("перемешивает детерминированно и сохраняет состав", func(t *testing.T) {
		options := []string{"a", "b", "c", "d"}
		first := ShuffleOptions("s1", "q1", options)
		second := ShuffleOptions("s1", "q1", options)
		if len(first) != len(second) {
			t.Fatalf("длины различаются: %v vs %v", first, second)
		}
		for i := range first {
			if first[i] != second[i] {
				t.Fatalf("порядок недетерминирован: %v vs %v", first, second)
			}
		}
		sortedFirst := append([]string(nil), first...)
		sortedOptions := append([]string(nil), options...)
		sortStrings(sortedFirst)
		sortStrings(sortedOptions)
		for i := range sortedFirst {
			if sortedFirst[i] != sortedOptions[i] {
				t.Fatalf("состав изменился: %v vs %v", sortedFirst, sortedOptions)
			}
		}
		if &first[0] == &options[0] {
			t.Error("shuffleOptions должен возвращать новый срез")
		}
	})

	t.Run("разные сессии дают разный порядок хотя бы для части вопросов", func(t *testing.T) {
		options := []string{"a", "b", "c", "d", "e"}
		base := strings.Join(ShuffleOptions("session-0", "question-X", options), ",")
		differs := false
		for i := 1; i <= 20; i++ {
			value := strings.Join(ShuffleOptions("session-"+string(rune('0'+i%10)), "question-X", options), ",")
			if value != base {
				differs = true
				break
			}
		}
		if !differs {
			t.Error("все сессии дали одинаковый порядок")
		}
	})

	t.Run("не ломает пустой и одиночный списки", func(t *testing.T) {
		if got := ShuffleOptions("s", "q", []string{}); len(got) != 0 {
			t.Errorf("пустой список → %v", got)
		}
		got := ShuffleOptions("s", "q", []string{"only"})
		if len(got) != 1 || got[0] != "only" {
			t.Errorf("одиночный список → %v", got)
		}
	})
}

func TestAntiCheatThetaHistory(t *testing.T) {
	service := NewAntiCheat()
	cat := NewCAT()

	t.Run("строит оценку на каждом префиксе ответов", func(t *testing.T) {
		answers := []IrtAnswer{
			{Question: irtQ("q1", 0.5, 1, 0.25), IsCorrect: true},
			{Question: irtQ("q2", 0.5, 1, 0.25), IsCorrect: true},
			{Question: irtQ("q3", 0.5, 1, 0.25), IsCorrect: false},
		}
		history := service.ThetaHistory(answers, func(prefix []IrtAnswer) float64 {
			return cat.EstimateTheta(prefix)
		})
		if len(history) != len(answers) {
			t.Fatalf("длина истории = %d, ожидалась %d", len(history), len(answers))
		}
		for i, h := range history {
			if math.IsNaN(h) || math.IsInf(h, 0) {
				t.Fatalf("history[%d] = %v, ожидалось конечное значение", i, h)
			}
		}
		if !(history[0] > history[2]) {
			t.Errorf("history[0]=%v не больше history[2]=%v", history[0], history[2])
		}
	})
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
