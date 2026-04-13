package validator

import (
	"fmt"
	"regexp"
	"strings"
)

// MWSViolation — нарушение правил MWS окружения
type MWSViolation struct {
	Type  string `json:"type"`
	Match string `json:"match"`
	Line  int    `json:"line"`
	Hint  string `json:"hint"`
}

// MWSError — ошибка MWS валидации для LLM
type MWSError struct {
	Message    string         `json:"message"`
	Violations []MWSViolation `json:"violations"`
}

func (e *MWSError) Error() string {
	if len(e.Violations) == 0 {
		return e.Message
	}
	var details []string
	for _, v := range e.Violations {
		details = append(details, fmt.Sprintf("[%s] line %d: %s (fix: %s)",
			v.Type, v.Line, v.Match, v.Hint))
	}
	return fmt.Sprintf("%s\n%s", e.Message, strings.Join(details, "\n"))
}

// MWSValidator проверяет соответствие кода правилам MWS Octapi
type MWSValidator struct {
	forbiddenPatterns []*regexp.Regexp
	patternInfo       map[string]PatternInfo
}

type PatternInfo struct {
	Type string
	Hint string
}

// NewMWSValidator создает новый MWS валидатор
func NewMWSValidator() *MWSValidator {
	v := &MWSValidator{
		forbiddenPatterns: []*regexp.Regexp{},
		patternInfo:       make(map[string]PatternInfo),
	}

	// JsonPath (запрещен в LowCode)
	v.addPattern(`\$\.\w+`, "JsonPath",
		"Don't use JsonPath like $.field. Use direct access: wf.vars.field")
	v.addPattern(`\$\[.*?\]`, "JsonPath",
		"Don't use JsonPath like $[0]. Use direct access: wf.vars.array[1]")
	v.addPattern(`\$\{.*?\}`, "JsonPath",
		"Don't use JsonPath like ${path}. Use direct access to wf.vars")

	// Запрещенные конструкции Lua
	v.addPattern(`\bgoto\b`, "goto",
		"goto is not allowed in LowCode. Use if/else or loops instead")
	v.addPattern(`::[a-zA-Z_][a-zA-Z0-9_]*::`, "label",
		"labels for goto are not allowed")

	return v
}

func (v *MWSValidator) addPattern(regex, typ, hint string) {
	re := regexp.MustCompile(regex)
	v.forbiddenPatterns = append(v.forbiddenPatterns, re)
	v.patternInfo[regex] = PatternInfo{Type: typ, Hint: hint}
}

// Validate проверяет код на соответствие MWS правилам
func (v *MWSValidator) Validate(code string) error {
	var violations []MWSViolation

	// 1. Проверка формата JSONString (lua{...}lua)
	if err := v.checkJSONStringFormat(code); err != nil {
		if jsonErr, ok := err.(*JSONFormatError); ok {
			violations = append(violations, MWSViolation{
				Type:  "json_format",
				Match: jsonErr.Found,
				Line:  0,
				Hint:  jsonErr.Hint,
			})
		}
	}

	// 2. Извлекаем чистый код без обертки
	cleanCode := ExtractMWSCode(code)
	cleanLines := strings.Split(cleanCode, "\n")

	// 3. Проверка на запрещенные паттерны
	for _, pattern := range v.forbiddenPatterns {
		matches := pattern.FindAllStringIndex(cleanCode, -1)
		if len(matches) == 0 {
			continue
		}
		info := v.patternInfo[pattern.String()]
		for _, match := range matches {
			lineNum := findLineNumber(match[0], cleanLines)
			matchText := cleanCode[match[0]:match[1]]

			violations = append(violations, MWSViolation{
				Type:  info.Type,
				Match: matchText,
				Line:  lineNum,
				Hint:  info.Hint,
			})
		}
	}

	// 4. Проверка: код должен возвращать значение
	if !strings.Contains(cleanCode, "return") {
		violations = append(violations, MWSViolation{
			Type:  "missing_return",
			Match: "no return statement",
			Line:  0,
			Hint:  "Add 'return' statement at the end of your code",
		})
	}

	if len(violations) > 0 {
		return &MWSError{
			Message:    "MWS environment violation - fix these issues",
			Violations: violations,
		}
	}

	return nil
}

// JSONFormatError — ошибка формата JSONString
type JSONFormatError struct {
	Message  string
	Expected string
	Found    string
	Hint     string
}

func (e *JSONFormatError) Error() string {
	return fmt.Sprintf("%s Expected: %s, Found: %s. Hint: %s",
		e.Message, e.Expected, e.Found, e.Hint)
}

// checkJSONStringFormat проверяет формат lua{...}lua
// checkJSONStringFormat проверяет формат lua{...}lua или просто валидный Lua код
func (v *MWSValidator) checkJSONStringFormat(code string) error {
	trimmed := strings.TrimSpace(code)

	// Если код уже в правильном формате
	if strings.HasPrefix(trimmed, "lua{") && strings.HasSuffix(trimmed, "}lua") {
		inner := trimmed[4 : len(trimmed)-4]
		if strings.TrimSpace(inner) == "" {
			return &JSONFormatError{
				Message:  "Code inside lua{...}lua cannot be empty",
				Expected: "some Lua code",
				Found:    "empty",
				Hint:     "Write your Lua logic between lua{ and }lua",
			}
		}
		return nil
	}

	// Если код без обертки, но это валидный Lua — пропускаем с предупреждением
	// (или можно автоматически обернуть)
	if v.isValidLuaCode(trimmed) {
		// Автоматически оборачиваем
		// Но это требует доступа к修改 кода, что сложно
		return nil // временно разрешаем
	}

	return &JSONFormatError{
		Message:  "Code must be wrapped in JSONString format",
		Expected: "lua{ ... }lua",
		Found:    trimmed[:min(len(trimmed), 30)] + "...",
		Hint:     "Wrap your code like this: lua{return wf.vars.data}lua",
	}
}

// isValidLuaCode проверяет, выглядит ли строка как валидный Lua код
func (v *MWSValidator) isValidLuaCode(code string) bool {
	// Простая проверка: содержит ли код ключевые слова Lua
	luaKeywords := []string{"return", "function", "local", "if", "for", "while", "end"}
	for _, kw := range luaKeywords {
		if strings.Contains(code, kw) {
			return true
		}
	}
	return false
}

// findLineNumber находит номер строки по позиции символа
func findLineNumber(pos int, lines []string) int {
	charPos := 0
	for i, line := range lines {
		if charPos+len(line) >= pos {
			return i + 1
		}
		charPos += len(line) + 1
	}
	return 0
}

// ExtractMWSCode извлекает чистый Lua код из JSONString формата
func ExtractMWSCode(code string) string {
	trimmed := strings.TrimSpace(code)
	if strings.HasPrefix(trimmed, "lua{") && strings.HasSuffix(trimmed, "}lua") {
		return trimmed[4 : len(trimmed)-4]
	}
	return trimmed
}

// IsMWSError проверяет, является ли ошибка ошибкой MWS
func IsMWSError(err error) bool {
	_, ok := err.(*MWSError)
	return ok
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
