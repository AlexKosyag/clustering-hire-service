package services

import (
	"math"
	"regexp"
	"unicode/utf16"
)

// Хелперы с семантикой JavaScript (TS-исходники считают в double и UTF-16):
// формулы IRT/CAT и LCG-генераторы обязаны совпадать с TS бит в бит.

// JSRound — Math.round из JS (половина вверх, для NaN — NaN).
func JSRound(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}
	return math.Floor(x + 0.5)
}

// JSStringLen — String.length из JS (число UTF-16 code units, не байтов).
func JSStringLen(s string) int {
	return len(utf16.Encode([]rune(s)))
}

// jsUTF16 — строка как последовательность UTF-16 code units (charCodeAt в JS).
func jsUTF16(s string) []uint16 {
	return utf16.Encode([]rune(s))
}

// jsWordsRe — \s из JS-регэкспа (для split(/\s+/) и /[-•*\d]/).
var jsWordsRe = regexp.MustCompile(`\s+`)

// JSSplitWhitespace — s.split(/\s+/) из JS: сегменты между совпадениями,
// включая пустые крайние (" a b " → ["", "a", "b", ""]).
func JSSplitWhitespace(s string) []string {
	locs := jsWordsRe.FindAllStringIndex(s, -1)
	if len(locs) == 0 {
		return []string{s}
	}
	out := make([]string, 0, len(locs)+1)
	prev := 0
	for _, loc := range locs {
		out = append(out, s[prev:loc[0]])
		prev = loc[1]
	}
	out = append(out, s[prev:])
	return out
}

// LCGState — состояние детерминированного LCG из TS:
// state = (state * 1103515245 + 12345) & 0x7fffffff.
// ВНИМАНИЕ: умножение выполняется в double — как в JS (число теряет точность
// выше 2^53), поэтому результат совпадает с TS бит в бит.
type LCGState struct {
	state float64
}

// NewLCG — создание генератора с сидом seed (Math.imul-подобная точность JS).
func NewLCG(seed int) *LCGState { return &LCGState{state: float64(seed)} }

// Next — следующее состояние (0 … 2^31−1), как в JS.
func (l *LCGState) Next() int {
	d := l.state*1103515245 + 12345 // две IEEE-754 операции — как в JS
	l.state = float64(int64(d) % 0x80000000)
	return int(l.state)
}

// Float — случайное [0, 1), как `state / 0x7fffffff` в TS.
func (l *LCGState) Float() float64 { return l.state / 0x7fffffff }
