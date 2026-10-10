package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// writeReport — запись docs/VALIDATION_REPORT.md (writeReport в metrics.ts).
func writeReport(matching matchingResults, testing testingResults, performance performanceResults) {
	total := 0
	passed := 0
	for _, s := range sections {
		total += len(s.Checks)
		for _, c := range s.Checks {
			if c.Ok {
				passed++
			}
		}
	}

	lines := []string{}
	lines = append(lines, `# Отчёт о валидации решения «ФСП · платформа ИТ-вакансий»`)
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Дата прогона: %s", runAt.Format("02.01.2006, 15:04:05")))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("**Итог: %d / %d проверок пройдено**", passed, total))
	lines = append(lines, "")
	lines = append(lines, "## Сводка по целям (docs/VALIDATION.md)")
	lines = append(lines, "")
	lines = append(lines, "| Метрика | Цель | Факт | Статус |")
	lines = append(lines, "|---------|------|------|--------|")
	lines = append(lines, fmt.Sprintf("| Precision@5 | ≥ 70%% | %.0f%% | %s |",
		matching.P5*100, passFail(matching.P5 >= 0.7)))
	lines = append(lines, fmt.Sprintf("| Precision@10 | — | %.0f%% | — |", matching.P10*100))
	lines = append(lines, fmt.Sprintf("| MRR | — | %.2f | — |", matching.MrrAvg))
	lines = append(lines, fmt.Sprintf("| nDCG@10 | — | %.3f | — |", matching.NDCG))
	lines = append(lines, fmt.Sprintf("| Поиск (avg) | < 500 мс | %d мс | %s |",
		int(matching.SearchMS+0.5), passFail(matching.SearchMS < 500)))
	lines = append(lines, fmt.Sprintf("| API (avg) | < 200 мс | %d мс | %s |",
		int(performance.AvgAll+0.5), passFail(performance.AvgAll < 200)))
	lines = append(lines, fmt.Sprintf("| Frontend (документ) | < 3000 мс | %d мс | %s |",
		int(performance.FrontendMS+0.5), passFail(performance.FrontendMS < 3000)))
	deltaScore := abs(testing.Strong.Score - testing.Strong2.Score)
	lines = append(lines, fmt.Sprintf("| Test-retest (Δscore) | ≤ 10 | %v | %s |",
		fmtNum(deltaScore), passFail(deltaScore <= 10)))
	thetaDelta := testing.Strong.Theta - testing.Weak.Theta
	lines = append(lines, fmt.Sprintf("| Дискриминативность (θ) | ≥ 2 | %.2f | %s |",
		thetaDelta, passFail(thetaDelta >= 2)))
	lines = append(lines, "")

	for _, section := range sections {
		lines = append(lines, "## "+section.Title)
		lines = append(lines, "")
		lines = append(lines, "| Проверка | Результат | Статус |")
		lines = append(lines, "|----------|-----------|--------|")
		for _, c := range section.Checks {
			lines = append(lines, fmt.Sprintf("| %s | %s | %s |", c.Name, c.Detail, mark(c.Ok)))
		}
		lines = append(lines, "")
		if section.Extra != "" {
			lines = append(lines, section.Extra)
			lines = append(lines, "")
		}
	}

	lines = append(lines, "---")
	lines = append(lines, "")
	lines = append(lines, "Как воспроизвести:")
	lines = append(lines, "")
	lines = append(lines, "```bash")
	lines = append(lines, "docker compose up -d --build                  # полный стек")
	lines = append(lines, "cd backend && go run ./cmd/metrics            # этот отчёт")
	lines = append(lines, "```")
	lines = append(lines, "")

	path := reportPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "report:", err)
		return
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "report:", err)
		return
	}

	fmt.Printf("\nОтчёт: %s\n", path)
	fmt.Printf("Проверок: %d/%d PASS\n", passed, total)
}

func passFail(ok bool) string {
	if ok {
		return "PASS"
	}
	return "FAIL"
}

func mark(ok bool) string {
	if ok {
		return "✅ PASS"
	}
	return "❌ FAIL"
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
