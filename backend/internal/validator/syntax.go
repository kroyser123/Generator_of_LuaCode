package validator

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

// SyntaxError — ошибка синтаксиса с деталями
type SyntaxError struct {
	Message string
	Line    int
	Column  int
}

func (e *SyntaxError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("syntax error at line %d, column %d: %s", e.Line, e.Column, e.Message)
	}
	return fmt.Sprintf("syntax error: %s", e.Message)
}

// SyntaxValidator проверяет синтаксис Lua
type SyntaxValidator struct{}

// NewSyntaxValidator создает новый валидатор синтаксиса
func NewSyntaxValidator() *SyntaxValidator {
	return &SyntaxValidator{}
}

// Validate проверяет синтаксис кода
// Возвращает nil если синтаксис корректен, иначе SyntaxError
func (v *SyntaxValidator) Validate(code string) error {
	L := lua.NewState()
	defer L.Close()

	if err := L.DoString(code); err != nil {
		// Пытаемся извлечь номер строки из ошибки
		return &SyntaxError{
			Message: err.Error(),
			Line:    extractLineFromError(err),
			Column:  0,
		}
	}
	return nil
}

// extractLineFromError пытается извлечь номер строки из ошибки gopher-lua
func extractLineFromError(err error) int {
	// gopher-lua ошибки обычно содержат строку вида "line 42:"
	s := err.Error()
	for i := 0; i < len(s)-5; i++ {
		if s[i:i+5] == "line " {
			line := 0
			for j := i + 5; j < len(s) && s[j] >= '0' && s[j] <= '9'; j++ {
				line = line*10 + int(s[j]-'0')
			}
			if line > 0 {
				return line
			}
		}
	}
	return 0
}
