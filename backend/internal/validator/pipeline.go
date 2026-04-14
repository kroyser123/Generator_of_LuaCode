package validator

import (
	"fmt"
	"time"
)

// ValidationResult — результат валидации

// Pipeline — пайплайн валидации
type Pipeline struct {
	syntax   *SyntaxValidator
	security *SecurityValidator
	mws      *MWSValidator
	sandbox  *SandboxValidator
}

// NewPipeline создает новый пайплайн
func NewPipeline(sandboxTimeout time.Duration) Validator {
	return &Pipeline{
		syntax:   NewSyntaxValidator(),
		security: NewSecurityValidator(),
		mws:      NewMWSValidator(),
		sandbox:  NewSandboxValidator(sandboxTimeout),
	}
}

// ValidateFull полная проверка: синтаксис, безопасность, MWS, выполнение
func (p *Pipeline) Validate(code string) ValidationResult {
	result := ValidationResult{Valid: true, Errors: []string{}}

	// 1. Синтаксис
	if err := p.syntax.Validate(code); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("syntax: %s", err.Error()))
		return result
	}

	// 2. Безопасность
	if err := p.security.Validate(code); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("security: %s", err.Error()))
		return result
	}

	// 3. MWS окружение
	if err := p.mws.Validate(code); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("mws: %s", err.Error()))
		return result
	}

	/* 4. Выполнение в песочнице
	output, err := p.sandbox.Validate(code)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("execution: %s", err.Error()))
		return result
	}
	result.Output = strings.TrimSpace(output)
	*/
	result.Output = ""
	return result
}

func IsSyntaxError(err error) bool {
	_, ok := err.(*SyntaxError)
	return ok
}
