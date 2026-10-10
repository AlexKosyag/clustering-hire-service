package services

import (
	"math"
	"testing"
)

// irtQ — хелпер в духе TS-спека: q(id, difficulty, discrimination = 1, guessing = 0.25).
func irtQ(id string, difficulty, discrimination, guessing float64) IrtQuestion {
	return IrtQuestion{ID: id, Difficulty: difficulty, Discrimination: discrimination, Guessing: guessing}
}

// Порт backend/src/modules/testing/services/cat.spec.ts (CATService, 3PL IRT).
func TestCATProbability(t *testing.T) {
	cat := NewCAT()

	t.Run("на трудности вопроса даёт c + (1-c)/2", func(t *testing.T) {
		p := cat.Probability(0.5, 1, 0.5, 0.25)
		if math.Abs(p-(0.25+0.75/2)) > 1e-6 {
			t.Errorf("Probability(0.5,1,0.5,0.25) = %v, ожидалось %v", p, 0.25+0.75/2)
		}
	})

	t.Run("монотонно растёт с ростом theta", func(t *testing.T) {
		low := cat.Probability(-2, 1, 0, 0.25)
		mid := cat.Probability(0, 1, 0, 0.25)
		high := cat.Probability(2, 1, 0, 0.25)
		if !(low < mid && mid < high) {
			t.Errorf("ожидалось low < mid < high, получено %v / %v / %v", low, mid, high)
		}
	})

	t.Run("защищает от некорректных параметров", func(t *testing.T) {
		p := cat.Probability(math.NaN(), math.NaN(), math.NaN(), math.NaN())
		if !(p > 0) || !(p < 1) {
			t.Errorf("NaN-параметры дают %v, ожидалось значение в (0,1)", p)
		}
		if p5 := cat.Probability(0, 1, 0, 5); p5 > 1-1e-6 {
			t.Errorf("guessing=5 → %v, ожидалось ≤ %v", p5, 1-1e-6)
		}
	})
}

func TestCATFisherInformation(t *testing.T) {
	cat := NewCAT()

	t.Run("информационная функция максимальна около сложности вопроса", func(t *testing.T) {
		atDifficulty := cat.FisherInformation(0, irtQ("a", 0, 1, 0.25))
		farAway := cat.FisherInformation(3, irtQ("a", 0, 1, 0.25))
		if !(atDifficulty > 0) || !(atDifficulty > farAway) {
			t.Errorf("atDifficulty=%v, farAway=%v: ожидалось atDifficulty > farAway > 0", atDifficulty, farAway)
		}
	})

	t.Run("всегда конечна и неотрицательна", func(t *testing.T) {
		info := cat.FisherInformation(100, irtQ("a", -5, 3, 0.1))
		if math.IsNaN(info) || math.IsInf(info, 0) || info < 0 {
			t.Errorf("fisherInformation(100, a=-5) = %v: ожидалось конечное неотрицательное", info)
		}
	})
}

func TestCATEstimateTheta(t *testing.T) {
	cat := NewCAT()

	answers := func(correct bool, count int) []IrtAnswer {
		out := make([]IrtAnswer, 0, count)
		for i := 0; i < count; i++ {
			out = append(out, IrtAnswer{Question: irtQ("q", float64(i%5)*0.2, 1, 0.25), IsCorrect: correct})
		}
		return out
	}

	t.Run("все верные ответы дают высокий theta", func(t *testing.T) {
		if theta := cat.EstimateTheta(answers(true, 10)); !(theta > 1) {
			t.Errorf("theta = %v, ожидалось > 1", theta)
		}
	})

	t.Run("все неверные ответы дают низкий theta", func(t *testing.T) {
		if theta := cat.EstimateTheta(answers(false, 10)); !(theta < -1) {
			t.Errorf("theta = %v, ожидалось < -1", theta)
		}
	})

	t.Run("theta ограничен [-4, 4]", func(t *testing.T) {
		theta := cat.EstimateTheta(answers(true, 30))
		if theta > 4 || theta < -4 {
			t.Errorf("theta = %v, ожидалось в [-4, 4]", theta)
		}
	})

	t.Run("пустая история даёт 0", func(t *testing.T) {
		if theta := cat.EstimateTheta(nil); theta != 0 {
			t.Errorf("theta = %v, ожидалось 0", theta)
		}
	})
}

func TestCATSelectNext(t *testing.T) {
	pool := []IrtQuestion{
		irtQ("easy", 0.1, 1, 0.25),
		irtQ("mid", 0.5, 1, 0.25),
		irtQ("hard", 0.9, 1, 0.25),
		irtQ("mid2", 0.55, 1, 0.25),
	}

	t.Run("первый вопрос — ближайший к сложности 0.5", func(t *testing.T) {
		next := SelectNext(pool, nil, "session-1")
		if next == nil || next.ID != "mid" {
			t.Errorf("next = %v, ожидался вопрос mid", next)
		}
	})

	t.Run("исключает уже отвеченные вопросы", func(t *testing.T) {
		answered := []IrtAnswer{{Question: irtQ("mid", 0.5, 1, 0.25), IsCorrect: true}}
		next := SelectNext(pool, answered, "session-1")
		if next == nil {
			t.Fatal("next = nil, ожидался неотвеченный вопрос")
		}
		if next.ID == "mid" {
			t.Error("вопрос mid выбран повторно")
		}
	})

	t.Run("выбирает вопрос с максимальной информацией при theta ~ 0", func(t *testing.T) {
		answered := []IrtAnswer{
			{Question: irtQ("a", 0.5, 1, 0.25), IsCorrect: true},
			{Question: irtQ("b", 0.5, 1, 0.25), IsCorrect: true},
		}
		next := SelectNext(pool, answered, "session-1")
		// при положительной оценке предпочтительны более сложные, но не крайние вопросы
		if next == nil || next.ID == "easy" {
			t.Errorf("next = %v, ожидался не easy", next)
		}
	})

	t.Run("детерминирован для одной и той же сессии", func(t *testing.T) {
		answered := []IrtAnswer{{Question: irtQ("mid", 0.5, 1, 0.25), IsCorrect: false}}
		a := SelectNext(pool, answered, "session-1")
		b := SelectNext(pool, answered, "session-1")
		if a == nil || b == nil || a.ID != b.ID {
			t.Errorf("ожидалась детерминированность, получено %v и %v", a, b)
		}
	})

	t.Run("возвращает nil, когда пул исчерпан", func(t *testing.T) {
		answered := make([]IrtAnswer, 0, len(pool))
		for _, p := range pool {
			answered = append(answered, IrtAnswer{Question: p, IsCorrect: true})
		}
		if next := SelectNext(pool, answered, "session-1"); next != nil {
			t.Errorf("next = %v, ожидался nil", next)
		}
		if next := SelectNext([]IrtQuestion{}, nil, "session-1"); next != nil {
			t.Errorf("пустой пул: next = %v, ожидался nil", next)
		}
	})
}

func TestSessionOrderKey(t *testing.T) {
	t.Run("детерминирован и различается для разных пар", func(t *testing.T) {
		if SessionOrderKey("s1", "q1") != SessionOrderKey("s1", "q1") {
			t.Error("одинаковые пары дают разные ключи")
		}
		if SessionOrderKey("s1", "q1") == SessionOrderKey("s1", "q2") {
			t.Error("разные questionId дают одинаковый ключ")
		}
		if SessionOrderKey("s1", "q1") == SessionOrderKey("s2", "q1") {
			t.Error("разные sessionId дают одинаковый ключ")
		}
	})

	t.Run("возвращает 32-битное беззнаковое значение", func(t *testing.T) {
		key := SessionOrderKey("abc", "def")
		// uint32 по определению неотрицателен и цел — проверим детерминированность
		if key != SessionOrderKey("abc", "def") {
			t.Error("ключ недетерминирован")
		}
	})
}
