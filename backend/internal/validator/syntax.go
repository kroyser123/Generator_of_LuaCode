package validator

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

type SyntaxError struct {
	Message string
	Line    int
	Column  int
}

func (e *SyntaxError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("syntax error at line %d: %s", e.Line, e.Message)
	}
	return fmt.Sprintf("syntax error: %s", e.Message)
}

type SyntaxValidator struct{}

func NewSyntaxValidator() *SyntaxValidator {
	return &SyntaxValidator{}
}

func (v *SyntaxValidator) Validate(code string) error {
	cleanCode := ExtractMWSCode(code)

	L := lua.NewState()
	defer L.Close()

	// Только компиляция, без выполнения
	_, err := L.LoadString(cleanCode)
	if err != nil {
		return &SyntaxError{
			Message: err.Error(),
			Line:    extractLineFromError(err),
		}
	}
	return nil
}

func extractLineFromError(err error) int {
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
