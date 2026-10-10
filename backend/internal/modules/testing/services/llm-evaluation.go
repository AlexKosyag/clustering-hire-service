package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// LLMEvaluationCriteria — критерий оценки.
type LLMEvaluationCriteria struct {
	Name        string  `json:"name"`
	Weight      float64 `json:"weight"`
	Description *string `json:"description,omitempty"`
}

// LLMEvaluationResult — результат LLM-оценки.
type LLMEvaluationResult struct {
	Score      float64            `json:"score"` // 0-100
	Breakdown  map[string]float64 `json:"breakdown"`
	Strengths  []string           `json:"strengths"`
	Weaknesses []string           `json:"weaknesses"`
	Feedback   string             `json:"feedback"`
	Provider   string             `json:"provider"`
	LatencyMs  int64              `json:"latencyMs"`
}

// LLMEvaluationRequest — вход оценки.
type LLMEvaluationRequest struct {
	Question        string
	CandidateAnswer string
	Criteria        []LLMEvaluationCriteria
	ReferenceAnswer *string
	KeyConcepts     []string
	MaxTokens       int
}

// LLMEvaluation — LLM-оценка развёрнутых ответов (OpenAI-совместимый API).
// При отсутствии ключа или ошибке — детерминированная keyword-эвристика.
type LLMEvaluation struct {
	httpClient *http.Client
	apiKey     string
	apiURL     string
	model      string
}

// NewLLMEvaluation — конструктор (cfg.OpenAIAPIKey / OpenAIAPIURL / OpenAIModel;
// в TS — process.env.OPENAI_*).
func NewLLMEvaluation(apiKey, apiURL, model string) *LLMEvaluation {
	s := &LLMEvaluation{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		apiKey:     apiKey,
		apiURL:     apiURL,
		model:      model,
	}
	if s.apiKey != "" {
		fmt.Printf("[LLMEvaluation] LLM Evaluation enabled — model=%s\n", s.model)
	} else {
		fmt.Printf("[LLMEvaluation] LLM Evaluation: OPENAI_API_KEY not set, using keyword fallback\n")
	}
	return s
}

// Available — настроен ли LLM.
func (s *LLMEvaluation) Available() bool { return s.apiKey != "" }

// Evaluate — оценка ответа кандидата (LLM + fallback).
func (s *LLMEvaluation) Evaluate(ctx context.Context, req LLMEvaluationRequest) (*LLMEvaluationResult, error) {
	t0 := time.Now()

	if s.Available() {
		result, err := s.evaluateWithLLM(ctx, req, t0)
		if err == nil {
			return result, nil
		}
		fmt.Printf("[LLMEvaluation] LLM evaluation failed: %s, using fallback\n", err.Error())
	}

	// Fallback: keyword matching
	return s.evaluateWithKeywords(req, t0), nil
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// evaluateWithLLM — оценка через LLM (OpenAI API).
func (s *LLMEvaluation) evaluateWithLLM(ctx context.Context, req LLMEvaluationRequest, t0 time.Time) (*LLMEvaluationResult, error) {
	criteriaLines := make([]string, 0, len(req.Criteria))
	for _, c := range req.Criteria {
		desc := c.Name
		if c.Description != nil {
			desc = *c.Description
		}
		criteriaLines = append(criteriaLines, fmt.Sprintf("- %s (weight: %s): %s", c.Name, jsNumber(c.Weight), desc))
	}
	criteriaText := strings.Join(criteriaLines, "\n")

	keyConceptsText := ""
	if len(req.KeyConcepts) > 0 {
		keyConceptsText = "\nKey concepts to look for: " + strings.Join(req.KeyConcepts, ", ")
	}

	referenceText := ""
	if req.ReferenceAnswer != nil {
		referenceText = "\nReference answer (for context, not exact match required):\n" + *req.ReferenceAnswer
	}

	prompt := fmt.Sprintf(`You are an expert technical interviewer evaluating a candidate's answer.

QUESTION:
%s

CANDIDATE'S ANSWER:
%s

EVALUATION CRITERIA:
%s%s%s

Evaluate the answer based on the criteria. Consider:
1. Technical accuracy
2. Completeness
3. Clarity and structure
4. Practical examples (if applicable)

Return ONLY a JSON object with this exact structure:
{
  "score": <0-100 overall score>,
  "breakdown": { "<criterion_name>": <0-100 score>, ... },
  "strengths": ["<strength 1>", "<strength 2>"],
  "weaknesses": ["<weakness 1>", "<weakness 2>"],
  "feedback": "<2-3 sentence constructive feedback>"
}`, req.Question, req.CandidateAnswer, criteriaText, keyConceptsText, referenceText)

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 1000
	}

	body := map[string]any{
		"model": s.model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a technical assessment expert. Always respond with valid JSON."},
			{"role": "user", "content": prompt},
		},
		"temperature": 0.3,
		"max_tokens":  maxTokens,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, s.apiURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+s.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var parsedResp openAIResponse
	if err := json.Unmarshal(raw, &parsedResp); err != nil {
		return nil, err
	}
	if len(parsedResp.Choices) == 0 {
		return nil, fmt.Errorf("LLM response has no choices")
	}
	content := parsedResp.Choices[0].Message.Content

	var parsed struct {
		Score      float64            `json:"score"`
		Breakdown  map[string]float64 `json:"breakdown"`
		Strengths  []string           `json:"strengths"`
		Weaknesses []string           `json:"weaknesses"`
		Feedback   string             `json:"feedback"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, err
	}

	breakdown := parsed.Breakdown
	if breakdown == nil {
		breakdown = map[string]float64{}
	}
	strengths := parsed.Strengths
	if strengths == nil {
		strengths = []string{}
	}
	weaknesses := parsed.Weaknesses
	if weaknesses == nil {
		weaknesses = []string{}
	}

	return &LLMEvaluationResult{
		Score:      math.Max(0, math.Min(100, parsed.Score)),
		Breakdown:  breakdown,
		Strengths:  strengths,
		Weaknesses: weaknesses,
		Feedback:   parsed.Feedback,
		Provider:   "openai:" + s.model,
		LatencyMs:  time.Since(t0).Milliseconds(),
	}, nil
}

var structureRe = regexp.MustCompile(`[-•*\d]`)

// evaluateWithKeywords — fallback: keyword-эвристика (docs 06 §4.6).
func (s *LLMEvaluation) evaluateWithKeywords(req LLMEvaluationRequest, t0 time.Time) *LLMEvaluationResult {
	answerLower := strings.ToLower(req.CandidateAnswer)
	breakdown := map[string]float64{}

	// Check for key concepts
	foundConcepts := []string{}
	missingConcepts := []string{}

	for _, concept := range req.KeyConcepts {
		if strings.Contains(answerLower, strings.ToLower(concept)) {
			foundConcepts = append(foundConcepts, concept)
		} else {
			missingConcepts = append(missingConcepts, concept)
		}
	}

	// Score based on concept coverage
	totalConcepts := len(req.KeyConcepts)
	conceptScore := 50.0
	if totalConcepts > 0 {
		conceptScore = float64(len(foundConcepts)) / float64(totalConcepts) * 100
	}

	// Score based on answer length (very rough)
	wordCount := len(JSSplitWhitespace(req.CandidateAnswer))
	lengthScore := math.Min(100, float64(wordCount)*2)

	// Score based on structure (has bullet points, numbers, etc.)
	structureScore := 40.0
	if structureRe.MatchString(req.CandidateAnswer) {
		structureScore = 70
	}

	// Calculate per-criterion scores
	for _, criterion := range req.Criteria {
		switch strings.ToLower(criterion.Name) {
		case "accuracy", "точность":
			breakdown[criterion.Name] = conceptScore
		case "completeness", "полнота":
			breakdown[criterion.Name] = conceptScore*0.8 + lengthScore*0.2
		case "structure", "структура":
			breakdown[criterion.Name] = structureScore
		default:
			breakdown[criterion.Name] = (conceptScore + lengthScore) / 2
		}
	}

	// Weighted overall score
	totalWeight := 0.0
	for _, c := range req.Criteria {
		totalWeight += c.Weight
	}
	overallScore := 0.0
	if totalWeight > 0 {
		for _, criterion := range req.Criteria {
			value, ok := breakdown[criterion.Name]
			if !ok {
				value = 50
			}
			overallScore += value * (criterion.Weight / totalWeight)
		}
	} else {
		overallScore = conceptScore
	}

	missing := "Good coverage!"
	if len(missingConcepts) > 0 {
		missing = "Missing: " + strings.Join(missingConcepts[:min3(len(missingConcepts))], ", ")
	}

	return &LLMEvaluationResult{
		Score:      JSRound(overallScore),
		Breakdown:  breakdown,
		Strengths:  foundConcepts[:min3(len(foundConcepts))],
		Weaknesses: missingConcepts[:min3(len(missingConcepts))],
		Feedback:   fmt.Sprintf("Found %d/%d key concepts. %s", len(foundConcepts), totalConcepts, missing),
		Provider:   "keyword-fallback",
		LatencyMs:  time.Since(t0).Milliseconds(),
	}
}

func min3(n int) int {
	if n < 3 {
		return n
	}
	return 3
}

// jsNumber — представление числа как в JS template literal (0.4 → "0.4").
func jsNumber(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}
