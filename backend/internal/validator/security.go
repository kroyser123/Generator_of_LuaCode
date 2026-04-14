package validator

import (
	"fmt"
	"regexp"
	"strings"
)

// SecurityViolation — структура для детализации нарушения безопасности
type SecurityViolation struct {
	Pattern string `json:"pattern"`
	Match   string `json:"match"`
	Line    int    `json:"line,omitempty"`
}

// SecurityError — ошибка безопасности с деталями
type SecurityError struct {
	Message    string
	Violations []SecurityViolation
}

func (e *SecurityError) Error() string {
	if len(e.Violations) == 0 {
		return e.Message
	}
	var details []string
	for _, v := range e.Violations {
		details = append(details, v.Pattern)
	}
	return fmt.Sprintf("%s: %s", e.Message, strings.Join(details, ", "))
}

// SecurityValidator проверяет наличие опасных функций
type SecurityValidator struct {
	forbiddenPatterns []*regexp.Regexp
	patternNames      map[string]string
}

// NewSecurityValidator создает новый валидатор безопасности
func NewSecurityValidator() *SecurityValidator {
	patterns := []struct {
		regex string
		name  string
	}{
		// Файловая система
		{`io\.open\s*\(`, "io.open() - file access"},
		{`io\.popen\s*\(`, "io.popen() - command execution"},
		{`io\.lines\s*\(`, "io.lines() - file reading"},
		{`os\.remove\s*\(`, "os.remove() - file deletion"},
		{`os\.rename\s*\(`, "os.rename() - file rename"},
		{`os\.tmpname\s*\(`, "os.tmpname() - temp file"},

		// Выполнение команд
		{`os\.execute\s*\(`, "os.execute() - shell command"},
		{`os\.exit\s*\(`, "os.exit() - process termination"},
		{`os\.getenv\s*\(`, "os.getenv() - env variable access"},

		// Загрузка кода
		{`dofile\s*\(`, "dofile() - file execution"},
		{`loadfile\s*\(`, "loadfile() - file loading"},
		{`load\s*\(`, "load() - dynamic code loading"},
		{`loadstring\s*\(`, "loadstring() - dynamic code loading"},

		// Опасные require
		{`require\s*\(\s*['"]os['"]\s*\)`, "require('os') - os module"},
		{`require\s*\(\s*['"]io['"]\s*\)`, "require('io') - io module"},
		{`require\s*\(\s*['"]debug['"]\s*\)`, "require('debug') - debug module"},

		// Доступ к отладке
		{`debug\.`, "debug.* - debug access"},

		// Сетевые вызовы
		{`socket\.`, "socket.* - network access"},
		{`http\.`, "http.* - HTTP client"},
		{`ftp\.`, "ftp.* - FTP client"},

		// Raw доступ
		{`rawget\s*\(`, "rawget() - bypass metatable"},
		{`rawset\s*\(`, "rawset() - bypass metatable"},
		{`rawlen\s*\(`, "rawlen() - raw length access"},
		{`rawequal\s*\(`, "rawequal() - raw equality"},

		// Package манипуляции
		{`package\.loadlib\s*\(`, "package.loadlib() - load C library"},
		{`package\.cpath`, "package.cpath - modify C library path"},
		{`package\.path`, "package.path - modify Lua module path"},

		// Metatable манипуляции
		{`getmetatable\s*\(\s*_G`, "getmetatable(_G) - access global metatable"},
		{`setmetatable\s*\(\s*_G`, "setmetatable(_G) - modify global metatable"},

		// Альтернативные способы вызова (через квадратные скобки)
		{`os\[['"]execute['"]\]`, "os['execute']() - shell command"},
		{`io\[['"]open['"]\]`, "io['open']() - file access"},
		{`_G\[['"]os['"]\]`, "_G['os'] - os access"},

		// ===== ДОБАВЛЕНО: JsonPath =====
		{`\$\.\w+`, "JsonPath - use direct access wf.vars.field"},
		{`\$\[.*?\]`, "JsonPath - use direct access wf.vars.array[1]"},
		{`\$\{.*?\}`, "JsonPath - use direct access to wf.vars"},
	}

	v := &SecurityValidator{
		forbiddenPatterns: make([]*regexp.Regexp, 0, len(patterns)),
		patternNames:      make(map[string]string),
	}

	for _, p := range patterns {
		re := regexp.MustCompile(p.regex)
		v.forbiddenPatterns = append(v.forbiddenPatterns, re)
		v.patternNames[p.regex] = p.name
	}

	return v
}

// Validate проверяет код на опасные функции
func (v *SecurityValidator) Validate(code string) error {
	var violations []SecurityViolation
	lines := strings.Split(code, "\n")

	for _, pattern := range v.forbiddenPatterns {
		if matches := pattern.FindAllStringIndex(code, -1); len(matches) > 0 {
			for _, match := range matches {
				lineNum := 1
				charPos := 0
				for i, line := range lines {
					if charPos+len(line) >= match[0] {
						lineNum = i + 1
						break
					}
					charPos += len(line) + 1
				}

				matchText := code[match[0]:match[1]]
				violations = append(violations, SecurityViolation{
					Pattern: v.patternNames[pattern.String()],
					Match:   matchText,
					Line:    lineNum,
				})
			}
		}
	}

	if len(violations) > 0 {
		return &SecurityError{
			Message:    "security violation: forbidden functions detected",
			Violations: violations,
		}
	}

	return nil
}

// IsSecurityError проверяет, является ли ошибка ошибкой безопасности
func IsSecurityError(err error) bool {
	_, ok := err.(*SecurityError)
	return ok
}

// GetViolations возвращает все нарушения из ошибки безопасности
func GetViolations(err error) []SecurityViolation {
	if secErr, ok := err.(*SecurityError); ok {
		return secErr.Violations
	}
	return nil
}
