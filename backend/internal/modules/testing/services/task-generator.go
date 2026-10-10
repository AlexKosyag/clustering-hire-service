package services

import (
	"encoding/json"
	"strconv"
	"strings"

	"fsp-backend/internal/model"
)

// TaskType — тип мультиформатного задания V2.
type TaskType = string

const (
	TaskMultipleChoice TaskType = "multiple_choice"
	TaskCodeExecution  TaskType = "code_execution"
	TaskLLMOpen        TaskType = "llm_open"
	TaskCodeReview     TaskType = "code_review"
	TaskSystemDesign   TaskType = "system_design"
)

// SampleTest — открытый тестовый случай code_execution.
type SampleTest struct {
	Input          string `json:"input"`
	ExpectedOutput string `json:"expectedOutput"`
}

// Criterion — критерий LLM-оценки.
type Criterion struct {
	Name        string  `json:"name"`
	Weight      float64 `json:"weight"`
	Description *string `json:"description,omitempty"`
}

// MultipleChoiceTask — блок multiple_choice.
type MultipleChoiceTask struct {
	Question string          `json:"question"`
	Options  json.RawMessage `json:"options"`
	// Explanation в TS-типе объявлен, но не заполняется (см. createMultipleChoiceTask).
	Explanation *string `json:"explanation,omitempty"`
}

// CodeExecutionTask — блок code_execution.
type CodeExecutionTask struct {
	Question         string       `json:"question"`
	StarterCode      string       `json:"starterCode"`
	Language         string       `json:"language"` // javascript | python | go
	SampleTests      []SampleTest `json:"sampleTests"`
	HiddenTestsCount int          `json:"hiddenTestsCount"`
}

// LLMOpenTask — блок llm_open.
type LLMOpenTask struct {
	Question           string      `json:"question"`
	EvaluationCriteria []Criterion `json:"evaluationCriteria"`
	KeyConcepts        []string    `json:"keyConcepts"`
	ReferenceAnswer    *string     `json:"referenceAnswer,omitempty"`
}

// CodeReviewTask — блок code_review.
type CodeReviewTask struct {
	Question         string   `json:"question"`
	BuggyCode        string   `json:"buggyCode"`
	Language         string   `json:"language"`
	ExpectedBugCount int      `json:"expectedBugCount"`
	BugDescriptions  []string `json:"bugDescriptions"`
}

// SystemDesignTask — блок system_design.
type SystemDesignTask struct {
	Question           string      `json:"question"`
	Constraints        []string    `json:"constraints"`
	EvaluationCriteria []Criterion `json:"evaluationCriteria"`
}

// TestTask — мультиформатное задание V2 (первый/текущий/следующий task в API).
type TestTask struct {
	ID               string              `json:"id"`
	Type             TaskType            `json:"type"`
	Difficulty       float64             `json:"difficulty"` // 0-1
	Points           int                 `json:"points"`
	Title            string              `json:"title"`
	Description      string              `json:"description"`
	TimeLimitSeconds int                 `json:"timeLimitSeconds"`
	MultipleChoice   *MultipleChoiceTask `json:"multipleChoice,omitempty"`
	CodeExecution    *CodeExecutionTask  `json:"codeExecution,omitempty"`
	LLMOpen          *LLMOpenTask        `json:"llmOpen,omitempty"`
	CodeReview       *CodeReviewTask     `json:"codeReview,omitempty"`
	SystemDesign     *SystemDesignTask   `json:"systemDesign,omitempty"`
}

// TaskDistribution — распределение типов заданий по грейду (проценты).
type TaskDistribution struct {
	MultipleChoice int `json:"multipleChoice"`
	CodeExecution  int `json:"codeExecution"`
	LLMOpen        int `json:"llmOpen"`
	CodeReview     int `json:"codeReview"`
	SystemDesign   int `json:"systemDesign"`
}

// TaskQuestion — вход createTaskFromQuestion (вопрос банка).
type TaskQuestion struct {
	ID            string
	QuestionType  string
	Text          string
	CodeSnippet   *string
	Options       json.RawMessage // raw JSON options вопроса (nil = отсутствуют)
	CorrectAnswer *string
	Difficulty    float64
	PoolTag       *string
}

// TaskGenerator — генерация мультиформатных заданий (V2).
type TaskGenerator struct {
	distributions map[string]TaskDistribution
}

// NewTaskGenerator — конструктор (распределение по грейдам — docs 06 §5).
func NewTaskGenerator() *TaskGenerator {
	return &TaskGenerator{
		distributions: map[string]TaskDistribution{
			"JUNIOR":      {MultipleChoice: 40, CodeExecution: 30, LLMOpen: 20, CodeReview: 10, SystemDesign: 0},
			"MIDDLE":      {MultipleChoice: 20, CodeExecution: 35, LLMOpen: 25, CodeReview: 15, SystemDesign: 5},
			"MIDDLE_PLUS": {MultipleChoice: 15, CodeExecution: 35, LLMOpen: 25, CodeReview: 15, SystemDesign: 10},
			"SENIOR":      {MultipleChoice: 10, CodeExecution: 35, LLMOpen: 25, CodeReview: 15, SystemDesign: 15},
			"LEAD":        {MultipleChoice: 5, CodeExecution: 30, LLMOpen: 30, CodeReview: 15, SystemDesign: 20},
			"PRINCIPAL":   {MultipleChoice: 5, CodeExecution: 25, LLMOpen: 30, CodeReview: 20, SystemDesign: 20},
		},
	}
}

// GetDistribution — распределение заданий для грейда (fallback — MIDDLE).
func (g *TaskGenerator) GetDistribution(grade model.Grade) TaskDistribution {
	if d, ok := g.distributions[string(grade)]; ok {
		return d
	}
	return g.distributions["MIDDLE"]
}

// GetPoints — баллы за тип задания с учётом сложности (информационные: итоговый
// score считает ScoringService, не сумма points).
func (g *TaskGenerator) GetPoints(taskType TaskType, difficulty float64) int {
	basePoints := map[TaskType]float64{
		TaskMultipleChoice: 10,
		TaskCodeExecution:  25,
		TaskLLMOpen:        20,
		TaskCodeReview:     30,
		TaskSystemDesign:   40,
	}

	// Higher difficulty = more points
	multiplier := 0.5 + difficulty*1.0 // 0.5x to 1.5x
	return int(JSRound(basePoints[taskType] * multiplier))
}

// CreateTaskFromQuestion — шаблон задания из вопроса банка.
// BUG-REF: T6 — слоты типов строятся от totalTasks = 20, тогда как фактический
// тест — 15 заданий; тип задания выбирается по позиции (index % shuffled.length),
// а не по questionType вопроса.
func (g *TaskGenerator) CreateTaskFromQuestion(question TaskQuestion, grade model.Grade, index int) *TestTask {
	distribution := g.GetDistribution(grade)
	totalTasks := 20 // Default test size (см. BUG-REF: T6)

	// Determine task type based on distribution and index
	taskType := g.selectTaskType(distribution, index, totalTasks)

	switch taskType {
	case TaskCodeExecution:
		return g.createCodeExecutionTask(question, index)
	case TaskLLMOpen:
		return g.createLLMOpenTask(question, index)
	case TaskCodeReview:
		return g.createCodeReviewTask(question, index)
	case TaskSystemDesign:
		return g.createSystemDesignTask(question, index)
	case TaskMultipleChoice:
		return g.createMultipleChoiceTask(question, index)
	default:
		return g.createMultipleChoiceTask(question, index)
	}
}

func (g *TaskGenerator) selectTaskType(distribution TaskDistribution, index, totalTasks int) TaskType {
	// Calculate how many tasks of each type
	counts := map[TaskType]int{
		TaskMultipleChoice: int(JSRound(float64(distribution.MultipleChoice) / 100 * float64(totalTasks))),
		TaskCodeExecution:  int(JSRound(float64(distribution.CodeExecution) / 100 * float64(totalTasks))),
		TaskLLMOpen:        int(JSRound(float64(distribution.LLMOpen) / 100 * float64(totalTasks))),
		TaskCodeReview:     int(JSRound(float64(distribution.CodeReview) / 100 * float64(totalTasks))),
		TaskSystemDesign:   int(JSRound(float64(distribution.SystemDesign) / 100 * float64(totalTasks))),
	}

	// Distribute tasks round-robin based on counts
	types := []TaskType{}
	for i := 0; i < counts[TaskMultipleChoice]; i++ {
		types = append(types, TaskMultipleChoice)
	}
	for i := 0; i < counts[TaskCodeExecution]; i++ {
		types = append(types, TaskCodeExecution)
	}
	for i := 0; i < counts[TaskLLMOpen]; i++ {
		types = append(types, TaskLLMOpen)
	}
	for i := 0; i < counts[TaskCodeReview]; i++ {
		types = append(types, TaskCodeReview)
	}
	for i := 0; i < counts[TaskSystemDesign]; i++ {
		types = append(types, TaskSystemDesign)
	}

	// Shuffle deterministically based on index
	shuffled := g.shuffleDeterministic(types, index)

	if len(shuffled) == 0 {
		return TaskMultipleChoice
	}
	return shuffled[index%len(shuffled)]
}

func (g *TaskGenerator) shuffleDeterministic(array []TaskType, seed int) []TaskType {
	result := make([]TaskType, len(array))
	copy(result, array)
	rng := NewLCG(seed)
	for i := len(result) - 1; i > 0; i-- {
		state := rng.Next()
		j := state % (i + 1)
		result[i], result[j] = result[j], result[i]
	}
	return result
}

func (g *TaskGenerator) createMultipleChoiceTask(question TaskQuestion, index int) *TestTask {
	if isJSONNullOrAbsent(question.Options) {
		// Вопрос банка без вариантов (coding/open_text/scenario): показываем как
		// развернутый ответ, иначе задача становилась null и прохождение вставало.
		return g.createLLMOpenTask(question, index)
	}

	return &TestTask{
		ID:               question.ID,
		Type:             TaskMultipleChoice,
		Difficulty:       question.Difficulty,
		Points:           g.GetPoints(TaskMultipleChoice, question.Difficulty),
		Title:            "Вопрос " + itoa(index+1),
		Description:      question.Text,
		TimeLimitSeconds: 120,
		MultipleChoice: &MultipleChoiceTask{
			Question: question.Text,
			// BUG-REF: TS отдаёт options вопроса как есть (включая isCorrect) —
			// в отличие от V1 viewQuestion, который вырезает isCorrect.
			Options: question.Options,
		},
	}
}

func (g *TaskGenerator) createCodeExecutionTask(question TaskQuestion, index int) *TestTask {
	starterCode := "// Ваш код здесь\nfunction solution(input) {\n  // TODO\n}\n"
	if question.CodeSnippet != nil {
		starterCode = *question.CodeSnippet
	}
	return &TestTask{
		ID:               question.ID,
		Type:             TaskCodeExecution,
		Difficulty:       question.Difficulty,
		Points:           g.GetPoints(TaskCodeExecution, question.Difficulty),
		Title:            "Задача с кодом " + itoa(index+1),
		Description:      question.Text,
		TimeLimitSeconds: 300, // 5 minutes
		CodeExecution: &CodeExecutionTask{
			Question:         question.Text,
			StarterCode:      starterCode,
			Language:         g.detectLanguage(question.CodeSnippet),
			SampleTests:      []SampleTest{{Input: "test", ExpectedOutput: "test"}}, // Will be customized
			HiddenTestsCount: 5,
		},
	}
}

func (g *TaskGenerator) createLLMOpenTask(question TaskQuestion, index int) *TestTask {
	return &TestTask{
		ID:               question.ID,
		Type:             TaskLLMOpen,
		Difficulty:       question.Difficulty,
		Points:           g.GetPoints(TaskLLMOpen, question.Difficulty),
		Title:            "Развернутый ответ " + itoa(index+1),
		Description:      question.Text,
		TimeLimitSeconds: 300,
		LLMOpen: &LLMOpenTask{
			Question: question.Text,
			EvaluationCriteria: []Criterion{
				{Name: "accuracy", Weight: 0.4, Description: strPtr("Точность технических утверждений")},
				{Name: "completeness", Weight: 0.3, Description: strPtr("Полнота ответа")},
				{Name: "structure", Weight: 0.2, Description: strPtr("Структурированность")},
				{Name: "examples", Weight: 0.1, Description: strPtr("Примеры кода/практики")},
			},
			KeyConcepts: g.extractKeyConcepts(question.Text),
		},
	}
}

func (g *TaskGenerator) createCodeReviewTask(question TaskQuestion, index int) *TestTask {
	buggyCode := "// Код с багами\nfunction buggy() {\n  // TODO\n}"
	if question.CodeSnippet != nil {
		buggyCode = *question.CodeSnippet
	}
	return &TestTask{
		ID:               question.ID,
		Type:             TaskCodeReview,
		Difficulty:       question.Difficulty,
		Points:           g.GetPoints(TaskCodeReview, question.Difficulty),
		Title:            "Code Review " + itoa(index+1),
		Description:      question.Text,
		TimeLimitSeconds: 240,
		CodeReview: &CodeReviewTask{
			Question:         question.Text,
			BuggyCode:        buggyCode,
			Language:         g.detectLanguage(question.CodeSnippet),
			ExpectedBugCount: 2,
			BugDescriptions:  []string{}, // Will be filled from question data
		},
	}
}

func (g *TaskGenerator) createSystemDesignTask(question TaskQuestion, index int) *TestTask {
	return &TestTask{
		ID:               question.ID,
		Type:             TaskSystemDesign,
		Difficulty:       question.Difficulty,
		Points:           g.GetPoints(TaskSystemDesign, question.Difficulty),
		Title:            "Проектирование системы " + itoa(index+1),
		Description:      question.Text,
		TimeLimitSeconds: 600, // 10 minutes
		SystemDesign: &SystemDesignTask{
			Question:    question.Text,
			Constraints: []string{},
			EvaluationCriteria: []Criterion{
				{Name: "architecture", Weight: 0.3, Description: strPtr("Выбор архитектуры")},
				{Name: "scalability", Weight: 0.25, Description: strPtr("Масштабируемость")},
				{Name: "tradeoffs", Weight: 0.25, Description: strPtr("Осознанные компромиссы")},
				{Name: "clarity", Weight: 0.2, Description: strPtr("Ясность объяснения")},
			},
		},
	}
}

func (g *TaskGenerator) detectLanguage(codeSnippet *string) string {
	if codeSnippet == nil {
		return "javascript"
	}

	lower := strings.ToLower(*codeSnippet)
	if strings.Contains(lower, "def ") || strings.Contains(lower, "import ") || strings.Contains(lower, "print(") {
		return "python"
	}
	if strings.Contains(lower, "func ") || strings.Contains(lower, "package ") || strings.Contains(lower, "fmt.") {
		return "go"
	}
	return "javascript"
}

// conceptKeywords — простая эвристика извлечения ключевых понятий.
var conceptKeywords = []string{
	"garbage collection", "memory", "concurrency", "thread", "async",
	"database", "index", "query", "transaction", "acid",
	"api", "rest", "graphql", "microservices", "monolith",
	"cache", "redis", "memcached",
	"queue", "kafka", "rabbitmq",
	"docker", "kubernetes", "container",
	"ci/cd", "devops", "deployment",
}

func (g *TaskGenerator) extractKeyConcepts(text string) []string {
	concepts := []string{}

	lower := strings.ToLower(text)
	for _, keyword := range conceptKeywords {
		if strings.Contains(lower, keyword) {
			concepts = append(concepts, keyword)
		}
	}

	return concepts
}

func itoa(n int) string { return strconv.Itoa(n) }

func strPtr(s string) *string { return &s }
