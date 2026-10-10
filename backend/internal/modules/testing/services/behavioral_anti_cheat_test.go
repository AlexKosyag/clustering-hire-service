package services

import "testing"

// Порт backend/src/modules/testing/services/behavioral-anti-cheat.spec.ts.

func bd(timeSpent, copyPaste, tabSwitches, keystrokes, idle float64) BehavioralData {
	return BehavioralData{
		TimeSpentSeconds: &timeSpent,
		CopyPasteEvents:  &copyPaste,
		TabSwitches:      &tabSwitches,
		KeystrokeCount:   &keystrokes,
		IdleTimeSeconds:  &idle,
	}
}

func TestBehavioralAnalyzeQuestion(t *testing.T) {
	service := NewBehavioralAntiCheat()

	t.Run("should flag too-fast answers on hard questions", func(t *testing.T) {
		result := service.AnalyzeQuestion(bd(3, 0, 0, 50, 0), 0.8, 100)
		if !hasFlag(result.Flags, "TOO_FAST_FOR_HARD_QUESTION") {
			t.Errorf("flags = %v, ожидался TOO_FAST_FOR_HARD_QUESTION", result.Flags)
		}
		if !(result.SuspicionScore > 20) {
			t.Errorf("suspicionScore = %v, ожидалось > 20", result.SuspicionScore)
		}
	})

	t.Run("should flag multiple copy-paste events", func(t *testing.T) {
		result := service.AnalyzeQuestion(bd(60, 5, 1, 10, 5), 0.5, 200)
		if !hasFlag(result.Flags, "MULTIPLE_COPY_PASTE") {
			t.Errorf("flags = %v, ожидался MULTIPLE_COPY_PASTE", result.Flags)
		}
	})

	t.Run("should flag large paste detection", func(t *testing.T) {
		result := service.AnalyzeQuestion(bd(30, 1, 0, 5, 0), 0.5, 200)
		if !hasFlag(result.Flags, "LARGE_PASTE_DETECTED") {
			t.Errorf("flags = %v, ожидался LARGE_PASTE_DETECTED", result.Flags)
		}
	})

	t.Run("should flag excessive tab switching", func(t *testing.T) {
		result := service.AnalyzeQuestion(bd(60, 0, 10, 100, 5), 0.5, 100)
		if !hasFlag(result.Flags, "EXCESSIVE_TAB_SWITCHING") {
			t.Errorf("flags = %v, ожидался EXCESSIVE_TAB_SWITCHING", result.Flags)
		}
	})

	t.Run("should have low suspicion for honest behavior", func(t *testing.T) {
		result := service.AnalyzeQuestion(bd(45, 0, 1, 80, 5), 0.5, 100)
		if !(result.SuspicionScore < 30) {
			t.Errorf("suspicionScore = %v, ожидалось < 30", result.SuspicionScore)
		}
		if len(result.Flags) > 1 {
			t.Errorf("flags = %v, ожидалось ≤ 1 флага", result.Flags)
		}
	})
}

func TestBehavioralAnalyzeSession(t *testing.T) {
	service := NewBehavioralAntiCheat()

	t.Run("should aggregate flags from multiple questions", func(t *testing.T) {
		analyses := []BehavioralAnalysis{
			{SuspicionScore: 30, Flags: []string{"FLAG_A"}, Details: map[string]BehavioralDetail{}, Confidence: 0.8},
			{SuspicionScore: 40, Flags: []string{"FLAG_B"}, Details: map[string]BehavioralDetail{}, Confidence: 0.8},
			{SuspicionScore: 20, Flags: []string{}, Details: map[string]BehavioralDetail{}, Confidence: 0.8},
		}
		result := service.AnalyzeSession(analyses, nil)
		if !hasFlag(result.Flags, "FLAG_A") || !hasFlag(result.Flags, "FLAG_B") {
			t.Errorf("flags = %v, ожидались FLAG_A и FLAG_B", result.Flags)
		}
		if !(result.SuspicionScore > 0) {
			t.Errorf("suspicionScore = %v, ожидалось > 0", result.SuspicionScore)
		}
	})

	t.Run("should flag theta volatility", func(t *testing.T) {
		thetaHistory := []float64{0, 3, -2, 2.5, -1, 4}
		analyses := []BehavioralAnalysis{
			{SuspicionScore: 10, Flags: []string{}, Details: map[string]BehavioralDetail{}, Confidence: 0.5},
		}
		if !hasFlag(service.AnalyzeSession(analyses, thetaHistory).Flags, "THETA_VOLATILITY") {
			t.Error("ожидался флаг THETA_VOLATILITY")
		}
	})

	t.Run("should flag multiple suspicious questions", func(t *testing.T) {
		analyses := []BehavioralAnalysis{
			{SuspicionScore: 60, Flags: []string{}, Details: map[string]BehavioralDetail{}, Confidence: 0.8},
			{SuspicionScore: 70, Flags: []string{}, Details: map[string]BehavioralDetail{}, Confidence: 0.8},
			{SuspicionScore: 55, Flags: []string{}, Details: map[string]BehavioralDetail{}, Confidence: 0.8},
		}
		if !hasFlag(service.AnalyzeSession(analyses, nil).Flags, "MULTIPLE_SUSPICIOUS_QUESTIONS") {
			t.Error("ожидался флаг MULTIPLE_SUSPICIOUS_QUESTIONS")
		}
	})
}

func TestBehavioralAnalyzeKeystrokePattern(t *testing.T) {
	service := NewBehavioralAntiCheat()

	t.Run("should return nil for insufficient data", func(t *testing.T) {
		if result := service.AnalyzeKeystrokePattern([]float64{100, 100, 100}); result != nil {
			t.Errorf("result = %v, ожидался nil", result)
		}
	})

	t.Run("should analyze consistent typing pattern", func(t *testing.T) {
		intervals := make([]float64, 20)
		for i := range intervals {
			intervals[i] = 100
		}
		result := service.AnalyzeKeystrokePattern(intervals)
		if result == nil {
			t.Fatal("result = nil, ожидался паттерн")
		}
		if !(result.RhythmConsistency > 0.9) {
			t.Errorf("rhythmConsistency = %v, ожидалось > 0.9", result.RhythmConsistency)
		}
		if !(result.Burstiness < 0.1) {
			t.Errorf("burstiness = %v, ожидалось < 0.1", result.Burstiness)
		}
	})

	t.Run("should analyze bursty typing pattern", func(t *testing.T) {
		intervals := []float64{50, 50, 50, 200, 200, 200, 50, 50, 50, 200, 200, 200, 50, 50, 50, 200, 200, 200, 50, 50}
		result := service.AnalyzeKeystrokePattern(intervals)
		if result == nil {
			t.Fatal("result = nil, ожидался паттерн")
		}
		if !(result.Burstiness > 0.5) {
			t.Errorf("burstiness = %v, ожидалось > 0.5", result.Burstiness)
		}
	})
}
