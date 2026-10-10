package candidates

// Порт backend/src/modules/candidates/pdf-parsing.service.ts (docs 07 §4).
//
// Текст из PDF извлекается github.com/ledongthuc/pdf (в TS — pdf-parse, динамический
// import); вся regex-логика ниже перенесена дословно. RE2 не поддерживает lookaround,
// поэтому секции «опыт/образование» и «о себе» эмулируются вручную с сохранением
// семантики ленивых квантификаторов и lookaround-терминаторов JS-регулярок.

import (
	"bytes"
	"errors"
	"io"
	"log"
	"regexp"
	"strings"
	"unicode"

	"github.com/ledongthuc/pdf"
)

// pdfText — извлечение plain-text из PDF-буфера (Reader.GetPlainText).
func pdfText(buffer []byte) (string, error) {
	reader, err := pdf.NewReader(bytes.NewReader(buffer), int64(len(buffer)))
	if err != nil {
		return "", err
	}
	plain, err := reader.GetPlainText()
	if err != nil {
		return "", err
	}
	b, err := io.ReadAll(plain)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ErrPDFUnreadable — «Не удалось прочитать PDF файл» (throw new Error в TS).
var ErrPDFUnreadable = errors.New("Не удалось прочитать PDF файл")

// ParsedResume — ParsedResume из pdf-parsing.service.ts.
// Ключи с undefined-значениями Prisma/JSON.stringify опускает (omitempty).
type ParsedResume struct {
	Name       *string           `json:"name,omitempty"`
	Email      *string           `json:"email,omitempty"`
	Phone      *string           `json:"phone,omitempty"`
	City       *string           `json:"city,omitempty"`
	Skills     []string          `json:"skills"`
	Experience []ExperienceEntry `json:"experience"`
	Education  []EducationEntry  `json:"education"`
	Summary    *string           `json:"summary,omitempty"`
	RawText    string            `json:"rawText"`
}

// ExperienceEntry — запись опыта (заполняется только description).
type ExperienceEntry struct {
	Title       *string `json:"title,omitempty"`
	Company     *string `json:"company,omitempty"`
	Period      *string `json:"period,omitempty"`
	Description *string `json:"description,omitempty"`
}

// EducationEntry — запись об образовании (заполняется только institution).
type EducationEntry struct {
	Institution *string `json:"institution,omitempty"`
	Degree      *string `json:"degree,omitempty"`
	Year        *string `json:"year,omitempty"`
}

// TECH_SKILLS — известные технологии для извлечения (дословно из TS, включая дубль gRPC).
var TECH_SKILLS = []string{
	// Языки
	"JavaScript", "TypeScript", "Python", "Java", "Go", "Golang", "C\\+\\+", "C#", "Ruby", "PHP",
	"Swift", "Kotlin", "Rust", "Scala", "Elixir", "Clojure", "Haskell", "Lua", "R",
	// Frontend
	"React", "Vue", "Angular", "Next\\.js", "Nuxt", "Svelte", "HTML", "CSS", "SASS", "LESS",
	"Redux", "MobX", "Zustand", "jQuery", "Bootstrap", "Tailwind",
	// Backend
	"Node\\.js", "Express", "NestJS", "Django", "Flask", "FastAPI", "Spring", "Laravel",
	"Ruby on Rails", "ASP\\.NET", "Gin", "Echo", "Fiber",
	// Базы данных
	"PostgreSQL", "MySQL", "MongoDB", "Redis", "Elasticsearch", "Cassandra", "DynamoDB",
	"SQLite", "MariaDB", "ClickHouse", "TimescaleDB", "InfluxDB",
	// DevOps
	"Docker", "Kubernetes", "K8s", "Terraform", "Ansible", "Jenkins", "GitLab CI",
	"GitHub Actions", "AWS", "GCP", "Azure", "Yandex Cloud", "Heroku",
	// Мессенджеры
	"Kafka", "RabbitMQ", "ActiveMQ", "NATS", "Pulsar",
	// Прочее
	"GraphQL", "gRPC", "REST", "SOAP", "WebSocket", "gRPC",
	"Git", "Linux", "Bash", "CI/CD", "Microservices",
	// ML/Data
	"TensorFlow", "PyTorch", "scikit-learn", "Pandas", "NumPy", "Jupyter",
	"Machine Learning", "Deep Learning", "NLP", "Computer Vision",
	// Мобильные
	"React Native", "Flutter", "iOS", "Android", "Xamarin",
}

// SKILL_PATTERN — \b(techs)\b c флагом "gi".
var SKILL_PATTERN = regexp.MustCompile(`(?i)\b(` + strings.Join(TECH_SKILLS, "|") + `)\b`)

// skillNormalizations — карта нормализации (порядок как в TS).
var skillNormalizations = []struct {
	re *regexp.Regexp
	to string
}{
	{regexp.MustCompile(`(?i)node\.?js`), "Node.js"},
	{regexp.MustCompile(`(?i)react\.?js`), "React"},
	{regexp.MustCompile(`(?i)vue\.?js`), "Vue.js"},
	{regexp.MustCompile(`(?i)next\.?js`), "Next.js"},
	{regexp.MustCompile(`(?i)postgres`), "PostgreSQL"},
	{regexp.MustCompile(`(?i)mongo`), "MongoDB"},
	{regexp.MustCompile(`(?i)k8s`), "Kubernetes"},
	{regexp.MustCompile(`(?i)golang`), "Go"},
	{regexp.MustCompile(`(?i)c\+\+`), "C++"},
	{regexp.MustCompile(`(?i)c#`), "C#"},
	{regexp.MustCompile(`(?i)machine learning`), "Machine Learning"},
}

// EMAIL_PATTERN — /[\w.-]+@[\w.-]+\.\w{2,}/.
var EMAIL_PATTERN = regexp.MustCompile(`[\w.-]+@[\w.-]+\.\w{2,}`)

// PHONE_PATTERNS — русский и международный форматы.
var PHONE_PATTERNS = []*regexp.Regexp{
	regexp.MustCompile(`(\+7|8)[\s-]?\(?\d{3}\)?[\s-]?\d{3}[\s-]?\d{2}[\s-]?\d{2}`), // +7 XXX XXX XX XX
	regexp.MustCompile(`\d{3}[\s-]\d{3}[\s-]\d{2}[\s-]\d{2}`),                       // XXX XXX XX XX
}

// CITY_KEYWORDS — города для substring-сверки (порядок как в TS).
var CITY_KEYWORDS = []string{
	"Москва", "Санкт-Петербург", "Питер", "СПб", "Новосибирск", "Екатеринбург",
	"Казань", "Нижний Новгород", "Челябинск", "Самара", "Омск", "Ростов-на-Дону",
	"Уфа", "Красноярск", "Пермь", "Воронеж", "Краснодар", "Сочи", "Владивосток",
	"Minsk", "Almaty", "Tashkent", "Bishkek", "Dushanbe", "Astana",
}

var (
	// ФИ из 2-3 заглавных слов (кириллица/латиница), без цифр.
	NAME_RU_PATTERN = regexp.MustCompile(`^[А-ЯЁ][а-яё]+\s+[А-ЯЁ][а-яё]+(?:\s+[А-ЯЁ][а-яё]+)?$`)
	NAME_EN_PATTERN = regexp.MustCompile(`^[A-Z][a-z]+\s+[A-Z][a-z]+(?:\s+[A-Z][a-z]+)?$`)
)

// SUMMARY_KEYWORDS — ключевые слова секции «о себе» (альтернативы regex).
var SUMMARY_KEYWORDS = []string{"о себе", "about", "summary", "профиль", "objective"}

// ParseResume — парсинг PDF-буфера и извлечение полей резюме (parseResume).
func ParseResume(buffer []byte) (*ParsedResume, error) {
	rawText, err := pdfText(buffer)
	if err != nil {
		log.Printf("[PdfParsing] PDF parsing failed: %s", err.Error())
		return nil, ErrPDFUnreadable
	}

	res := &ParsedResume{
		Skills:     extractSkills(rawText),
		Experience: extractExperience(rawText),
		Education:  extractEducation(rawText),
		RawText:    sliceRunes(rawText, 5000), // Limit raw text size
	}
	if m := EMAIL_PATTERN.FindString(rawText); m != "" {
		v := m
		res.Email = &v
	}
	for _, p := range PHONE_PATTERNS {
		if m := p.FindString(rawText); m != "" {
			v := m
			res.Phone = &v
			break
		}
	}
	for _, city := range CITY_KEYWORDS {
		if strings.Contains(strings.ToLower(rawText), strings.ToLower(city)) {
			v := city
			res.City = &v
			break
		}
	}
	if v := extractName(rawText); v != nil {
		res.Name = v
	}
	if v := extractSummary(rawText); v != nil {
		res.Summary = v
	}
	return res, nil
}

// extractSkills — SKILL_PATTERN + нормализация + дедупликация, лимит 30.
func extractSkills(text string) []string {
	matches := SKILL_PATTERN.FindAllString(text, -1)
	seen := make(map[string]bool, len(matches))
	skills := make([]string, 0, len(matches))
	for _, m := range matches {
		normalized := strings.TrimSpace(m)
		for _, n := range skillNormalizations {
			if n.re.MatchString(normalized) {
				normalized = n.to
				break
			}
		}
		if !seen[normalized] {
			seen[normalized] = true
			skills = append(skills, normalized)
		}
		if len(skills) == 30 { // Limit to 30 skills
			break
		}
	}
	return skills
}

// findSection — эмуляция JS-regex /(?:a|b)[\s\S]*?(?=(?:x|y|$))/i на RE2.
// Возвращает подстроку от начала ключевого слова до ближайшей позиции lookaround-терминатора.
func findSection(text string, startAlts, endAlts []string) (string, bool) {
	// strings.ToLower меняет длину только для экзотических Unicode-символов —
	// для текста резюме смещения совпадают.
	lower := strings.ToLower(text)
	start, kwLen := -1, 0
	for _, alt := range startAlts {
		if i := strings.Index(lower, alt); i >= 0 && (start < 0 || i < start) {
			start, kwLen = i, len(alt)
		}
	}
	if start < 0 {
		return "", false
	}
	// На позиции start JS-regex перебирает альтернативы по порядку.
	for _, alt := range startAlts {
		if strings.HasPrefix(lower[start:], alt) {
			kwLen = len(alt)
			break
		}
	}
	// Ленивый [\s\S]*? расширяется до первой позиции, где срабатывает lookaround
	// (включая конец строки — $).
	for k := start + kwLen; k <= len(text); k++ {
		if k == len(text) {
			return text[start:k], true
		}
		for _, alt := range endAlts {
			if strings.HasPrefix(lower[k:], alt) {
				return text[start:k], true
			}
		}
	}
	return text[start:], true
}

// splitLines — section.split(/\n\n|\n(?=[A-ZА-Я])/) (без флага "i").
func splitLines(section string) []string {
	runes := []rune(section)
	parts := make([]string, 0, 8)
	last, i := 0, 0
	for i < len(runes) {
		if runes[i] == '\n' {
			if i+1 < len(runes) && runes[i+1] == '\n' {
				parts = append(parts, string(runes[last:i]))
				i += 2
				last = i
				continue
			}
			if i+1 < len(runes) && isUpperAZ(runes[i+1]) {
				parts = append(parts, string(runes[last:i]))
				i++
				last = i
				continue
			}
		}
		i++
	}
	return append(parts, string(runes[last:]))
}

// isUpperAZ — [A-ZА-Я] (Ё не входит, как и в JS-классе).
func isUpperAZ(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'А' && r <= 'Я')
}

// extractExperience — секция «опыт/experience», до 5 записей, описание ≤300.
func extractExperience(text string) []ExperienceEntry {
	entries := []ExperienceEntry{}
	section, ok := findSection(text,
		[]string{"опыт", "experience", "работа", "занятость"},
		[]string{"образование", "education", "навыки", "skills"})
	if !ok {
		return entries
	}
	for i, line := range splitLines(section) {
		if i >= 5 { // lines.slice(0, 5)
			break
		}
		trimmed := strings.TrimSpace(line)
		if len([]rune(trimmed)) > 20 {
			description := sliceRunes(trimmed, 300)
			entries = append(entries, ExperienceEntry{Description: &description})
		}
	}
	return entries
}

// extractEducation — секция «образование/education», до 3 записей, institution ≤200.
func extractEducation(text string) []EducationEntry {
	entries := []EducationEntry{}
	section, ok := findSection(text,
		[]string{"образование", "education", "университет", "институт", "university"},
		[]string{"опыт", "experience", "навыки", "skills"})
	if !ok {
		return entries
	}
	for i, line := range splitLines(section) {
		if i >= 3 { // lines.slice(0, 3)
			break
		}
		trimmed := strings.TrimSpace(line)
		if len([]rune(trimmed)) > 10 {
			institution := sliceRunes(trimmed, 200)
			entries = append(entries, EducationEntry{Institution: &institution})
		}
	}
	return entries
}

// extractName — ФИ в первых 5 непустых строках.
func extractName(text string) *string {
	checked := 0
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		checked++
		if checked > 5 { // lines.slice(0, 5)
			break
		}
		if NAME_RU_PATTERN.MatchString(trimmed) || NAME_EN_PATTERN.MatchString(trimmed) {
			return &trimmed
		}
	}
	return nil
}

// extractSummary — /(?:о себе|about|summary|профиль|objective)[\s:]*\n([\s\S]{20,500}?)(?=\n\n|\n[A-ZА-Я]|$)/i.
func extractSummary(text string) *string {
	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		for _, kw := range SUMMARY_KEYWORDS {
			kwr := []rune(kw)
			if !hasFoldPrefix(runes[i:], kwr) {
				continue
			}
			if out, ok := trySummary(runes, i+len(kwr)); ok {
				return &out
			}
		}
	}
	return nil
}

// trySummary — [\s:]*\n (жадный с backtracking → последний допустимый \n),
// затем ленивое {20,500} с lookaround (?=\n\n|\n[A-ZА-Я]|$).
func trySummary(runes []rune, j int) (string, bool) {
	p := -1
	for k := j; k < len(runes); k++ {
		if runes[k] == '\n' {
			p = k // backtracking жадной [\s:]* находит наибольший допустимый \n
			continue
		}
		if !unicode.IsSpace(runes[k]) && runes[k] != ':' {
			break
		}
	}
	if p < 0 {
		return "", false
	}
	start := p + 1
	maxLen := 500
	if len(runes)-start < maxLen {
		maxLen = len(runes) - start
	}
	for l := 20; l <= maxLen; l++ { // ленивый квантификатор — с наименьшей длины
		end := start + l
		if end == len(runes) { // $ — конец строки
			return strings.TrimSpace(string(runes[start:end])), true
		}
		if runes[end] == '\n' && end+1 < len(runes) {
			if runes[end+1] == '\n' || isUpperAZ(runes[end+1]) {
				return strings.TrimSpace(string(runes[start:end])), true
			}
		}
	}
	return "", false
}

// hasFoldPrefix — префикс без учёта регистра (как /keyword/i).
func hasFoldPrefix(runes, prefix []rune) bool {
	if len(runes) < len(prefix) {
		return false
	}
	for i, r := range prefix {
		if !strings.EqualFold(string(runes[i]), string(r)) {
			return false
		}
	}
	return true
}

// sliceRunes — slice(0, n) по code points (в TS — по UTF-16; для BMP-текста совпадает).
func sliceRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// sliceRunesPtr — обрезка указателя (nil → nil).
func sliceRunesPtr(s *string, n int) *string {
	if s == nil {
		return nil
	}
	out := sliceRunes(*s, n)
	return &out
}
