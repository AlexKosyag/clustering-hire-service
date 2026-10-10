package services

import (
	"math"
	"regexp"
)

// BehavioralData — поведенческая телеметрия клиента (docs 06 §6.2).
// Поля-указатели: nil соответствует undefined в JS (поле не прислано).
type BehavioralData struct {
	TimeSpentSeconds      *float64
	CopyPasteEvents       *float64
	TabSwitches           *float64
	KeystrokeCount        *float64
	IdleTimeSeconds       *float64
	MouseMovementDistance *float64
	FocusLossEvents       *float64
	PasteSources          []string // откуда вставлялось (paste-события)
}

// BehavioralDetail — метрика одного сигнала.
type BehavioralDetail struct {
	Value      float64 `json:"value"`
	Threshold  float64 `json:"threshold"`
	Suspicious bool    `json:"suspicious"`
}

// BehavioralAnalysis — результат поведенческого анализа.
type BehavioralAnalysis struct {
	SuspicionScore float64                     `json:"suspicionScore"` // 0-100, выше = подозрительнее
	Flags          []string                    `json:"flags"`
	Details        map[string]BehavioralDetail `json:"details"`
	Confidence     float64                     `json:"confidence"` // 0-1
}

// KeystrokePattern — ритм ввода (верификация личности).
type KeystrokePattern struct {
	AverageIntervalMs float64 `json:"averageIntervalMs"`
	VarianceMs        float64 `json:"varianceMs"`
	Burstiness        float64 `json:"burstiness"`        // 0 = равномерно, 1 = рывками
	RhythmConsistency float64 `json:"rhythmConsistency"` // 0-1
}

// BehavioralAntiCheat — V2: поведенческий анти-чит (keystroke, copy-paste, tab-switch).
type BehavioralAntiCheat struct {
	thresholds behavioralThresholds
}

type behavioralThresholds struct {
	minTimePerDifficulty struct {
		easy   float64 // секунды
		medium float64
		hard   float64
	}
	maxCopyPastePerTest   float64
	maxTabSwitchesPerTest float64
	minKeystrokesPerChar  float64 // хотя бы 0.3 нажатия на символ
	maxIdleRatio          float64 // максимум 30% простое
	suspiciousFastAnswer  float64 // секунды
}

// NewBehavioralAntiCheat — конструктор (пороги — как в TS).
func NewBehavioralAntiCheat() *BehavioralAntiCheat {
	t := behavioralThresholds{
		maxCopyPastePerTest:   3,
		maxTabSwitchesPerTest: 5,
		minKeystrokesPerChar:  0.3,
		maxIdleRatio:          0.3,
		suspiciousFastAnswer:  5,
	}
	t.minTimePerDifficulty.easy = 10
	t.minTimePerDifficulty.medium = 20
	t.minTimePerDifficulty.hard = 40
	return &BehavioralAntiCheat{thresholds: t}
}

var pasteSourceRe = regexp.MustCompile(`(?i)stackoverflow|github|chatgpt|claude|bard|google`)

// AnalyzeQuestion — поведенческий анализ одного вопроса (аддитивно, 0-100).
func (s *BehavioralAntiCheat) AnalyzeQuestion(behavioral BehavioralData, questionDifficulty float64, answerLength int) BehavioralAnalysis {
	flags := []string{}
	details := map[string]BehavioralDetail{}
	suspicionScore := 0.0

	ts := behavioral.TimeSpentSeconds

	// 1. Time analysis
	expectedMinTime := s.getExpectedMinTime(questionDifficulty)

	if ts != nil && *ts < s.thresholds.suspiciousFastAnswer && questionDifficulty > 0.5 {
		flags = append(flags, "TOO_FAST_FOR_HARD_QUESTION")
		suspicionScore += 30
	}

	if ts != nil {
		timeRatio := *ts / expectedMinTime
		if timeRatio < 0.3 && *ts < 10 {
			flags = append(flags, "SUSPICIOUSLY_QUICK")
			suspicionScore += 20
		}
		details["timeSpent"] = BehavioralDetail{
			Value:      *ts,
			Threshold:  expectedMinTime,
			Suspicious: *ts < expectedMinTime*0.5,
		}
	} else {
		// behavioral.timeSpentSeconds === undefined в TS — все сравнения ложные
		details["timeSpent"] = BehavioralDetail{Threshold: expectedMinTime}
	}

	// 2. Copy-paste analysis
	cp := behavioral.CopyPasteEvents
	if cp != nil && *cp > 0 {
		if answerLength > 0 {
			_ = *cp / float64(answerLength) // pasteRatio — в TS вычисляется, но не используется
		}

		if *cp >= 3 {
			flags = append(flags, "MULTIPLE_COPY_PASTE")
			suspicionScore += 25
		}

		if float64(answerLength) > 100 && *cp == 1 && (float64(answerLength) / *cp) > 50 {
			// Single paste of large text
			flags = append(flags, "LARGE_PASTE_DETECTED")
			suspicionScore += 35
		}

		// Check paste sources (if available)
		if len(behavioral.PasteSources) > 0 {
			suspicious := false
			for _, src := range behavioral.PasteSources {
				if pasteSourceRe.MatchString(src) {
					suspicious = true
					break
				}
			}
			if suspicious {
				flags = append(flags, "EXTERNAL_SOURCE_PASTE")
				suspicionScore += 40
			}
		}
	}

	copyPasteDetail := BehavioralDetail{Threshold: s.thresholds.maxCopyPastePerTest}
	if cp != nil {
		copyPasteDetail.Value = *cp
		copyPasteDetail.Suspicious = *cp > s.thresholds.maxCopyPastePerTest
	}
	details["copyPaste"] = copyPasteDetail

	// 3. Tab switching / focus loss
	tab := behavioral.TabSwitches
	if tab != nil && *tab > s.thresholds.maxTabSwitchesPerTest {
		flags = append(flags, "EXCESSIVE_TAB_SWITCHING")
		suspicionScore += 20
	}

	if behavioral.FocusLossEvents != nil && *behavioral.FocusLossEvents > 3 {
		flags = append(flags, "FREQUENT_FOCUS_LOSS")
		suspicionScore += 15
	}

	tabDetail := BehavioralDetail{Threshold: s.thresholds.maxTabSwitchesPerTest}
	if tab != nil {
		tabDetail.Value = *tab
		tabDetail.Suspicious = *tab > s.thresholds.maxTabSwitchesPerTest
	}
	details["tabSwitches"] = tabDetail

	// 4. Idle time analysis
	idleRatio := 0.0
	if ts != nil && *ts > 0 {
		if behavioral.IdleTimeSeconds != nil {
			idleRatio = *behavioral.IdleTimeSeconds / *ts
		} else {
			idleRatio = math.NaN() // undefined / x → NaN в JS
		}
	}

	if idleRatio > s.thresholds.maxIdleRatio {
		flags = append(flags, "EXCESSIVE_IDLE_TIME")
		suspicionScore += 15
	}

	details["idleRatio"] = BehavioralDetail{
		Value:      JSRound(idleRatio*100) / 100,
		Threshold:  s.thresholds.maxIdleRatio,
		Suspicious: idleRatio > s.thresholds.maxIdleRatio,
	}

	// 5. Keystroke pattern analysis (if available)
	ks := behavioral.KeystrokeCount
	if ks != nil && *ks > 0 && answerLength > 0 {
		keystrokeRatio := *ks / float64(answerLength)

		// Too few keystrokes for the answer length (pasted)
		if keystrokeRatio < s.thresholds.minKeystrokesPerChar && answerLength > 50 {
			flags = append(flags, "LOW_KEYSTROKE_DENSITY")
			suspicionScore += 20
		}
	}

	// Clamp suspicion score
	suspicionScore = math.Min(100, suspicionScore)

	return BehavioralAnalysis{
		SuspicionScore: suspicionScore,
		Flags:          flags,
		Details:        details,
		Confidence:     s.calculateConfidence(flags, behavioral),
	}
}

// AnalyzeKeystrokePattern — анализ ритма ввода для верификации личности.
// BUG-REF: T8 — реализован и покрыт тестами, но нигде не вызывается (заготовка).
func (s *BehavioralAntiCheat) AnalyzeKeystrokePattern(intervals []float64) *KeystrokePattern {
	if len(intervals) < 10 {
		return nil
	}

	mean := 0.0
	for _, v := range intervals {
		mean += v
	}
	mean /= float64(len(intervals))

	variance := 0.0
	for _, x := range intervals {
		variance += (x - mean) * (x - mean)
	}
	variance /= float64(len(intervals))
	stdDev := math.Sqrt(variance)

	// Burstiness: ratio of stdDev to mean
	burstiness := 0.0
	if mean > 0 {
		burstiness = math.Min(1, stdDev/mean)
	}

	// Rhythm consistency: how consistent is the typing rhythm
	// Low variance = high consistency
	rhythmConsistency := 0.0
	if mean > 0 {
		rhythmConsistency = math.Max(0, 1-stdDev/mean)
	}

	return &KeystrokePattern{
		AverageIntervalMs: JSRound(mean*100) / 100,
		VarianceMs:        JSRound(variance*100) / 100,
		Burstiness:        JSRound(burstiness*100) / 100,
		RhythmConsistency: JSRound(rhythmConsistency*100) / 100,
	}
}

// AnalyzeSession — поведенческий анализ всей сессии.
func (s *BehavioralAntiCheat) AnalyzeSession(questionAnalyses []BehavioralAnalysis, thetaHistory []float64) BehavioralAnalysis {
	allFlags := map[string]bool{}
	flagOrder := []string{}
	totalSuspicion := 0.0

	for _, analysis := range questionAnalyses {
		totalSuspicion += analysis.SuspicionScore
		for _, flag := range analysis.Flags {
			if !allFlags[flag] {
				allFlags[flag] = true
				flagOrder = append(flagOrder, flag)
			}
		}
	}

	avgSuspicion := 0.0
	if len(questionAnalyses) > 0 {
		avgSuspicion = totalSuspicion / float64(len(questionAnalyses))
	}

	// Additional session-level flags
	flags := flagOrder

	// Theta volatility check
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

	// Multiple suspicious questions
	suspiciousQuestions := 0
	for _, a := range questionAnalyses {
		if a.SuspicionScore > 50 {
			suspiciousQuestions++
		}
	}
	if suspiciousQuestions >= 3 {
		flags = append(flags, "MULTIPLE_SUSPICIOUS_QUESTIONS")
	}

	finalSuspicion := math.Min(100, avgSuspicion*1.5+float64(suspiciousQuestions)*10)

	return BehavioralAnalysis{
		SuspicionScore: JSRound(finalSuspicion),
		Flags:          flags,
		Details:        map[string]BehavioralDetail{},
		Confidence:     math.Min(1, float64(len(flags))*0.2),
	}
}

func (s *BehavioralAntiCheat) getExpectedMinTime(difficulty float64) float64 {
	if difficulty < 0.3 {
		return s.thresholds.minTimePerDifficulty.easy
	}
	if difficulty < 0.6 {
		return s.thresholds.minTimePerDifficulty.medium
	}
	return s.thresholds.minTimePerDifficulty.hard
}

func (s *BehavioralAntiCheat) calculateConfidence(flags []string, behavioral BehavioralData) float64 {
	confidence := 0.0

	// More data points = higher confidence
	if behavioral.KeystrokeCount != nil && *behavioral.KeystrokeCount > 0 {
		confidence += 0.2
	}
	if behavioral.CopyPasteEvents != nil {
		confidence += 0.2
	}
	if behavioral.TabSwitches != nil {
		confidence += 0.2
	}
	if behavioral.TimeSpentSeconds != nil && *behavioral.TimeSpentSeconds > 0 {
		confidence += 0.2
	}
	if behavioral.IdleTimeSeconds != nil {
		confidence += 0.2
	}

	return math.Min(1, confidence)
}
