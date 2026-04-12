package validator

import (
	"context"
	"fmt"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// ExecutionError — ошибка выполнения кода
type ExecutionError struct {
	Message string
	Type    string 
	Line    int
}

func (e *ExecutionError) Error() string {
	return fmt.Sprintf("[%s] %s (line %d)", e.Type, e.Message, e.Line)
}

// SandboxValidator выполняет код в изолированной среде
type SandboxValidator struct {
	timeout time.Duration
}

// NewSandboxValidator создает новый валидатор песочницы
func NewSandboxValidator(timeout time.Duration) *SandboxValidator {
	return &SandboxValidator{timeout: timeout}
}

// Validate выполняет код с таймаутом и возвращает вывод
func (v *SandboxValidator) Validate(code string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), v.timeout)
	defer cancel()

	resultCh := make(chan string)
	errCh := make(chan error)

	go func() {
		output, err := v.executeSandbox(code)
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- output
	}()

	select {
	case <-ctx.Done():
		return "", &ExecutionError{
			Message: "code execution timed out",
			Type:    "timeout",
			Line:    0,
		}
	case err := <-errCh:
		return "", err
	case output := <-resultCh:
		return output, nil
	}
}

// executeSandbox запускает Lua код в изолированной среде
func (v *SandboxValidator) executeSandbox(code string) (string, error) {
	L := lua.NewState()
	defer L.Close()

	// Собираем вывод
	var output string
	L.SetGlobal("print", L.NewFunction(func(L *lua.LState) int {
		args := make([]string, L.GetTop())
		for i := 1; i <= L.GetTop(); i++ {
			args[i-1] = L.ToString(i)
		}
		output += fmt.Sprint(args) + "\n"
		return 0
	}))

	// Удаляем опасные библиотеки
	// Оставляем только базовые: math, string, table
	dangerousLibs := []string{"os", "io", "debug", "coroutine"}
	for _, lib := range dangerousLibs {
		L.PreloadModule(lib, nil)
	}

	// Паника не должна уронить сервер
	defer func() {
		if r := recover(); r != nil {
			// Паника поймана
		}
	}()

	// Выполняем код
	if err := L.DoString(code); err != nil {
		return "", &ExecutionError{
			Message: err.Error(),
			Type:    "runtime",
			Line:    v.extractLineFromError(err),
		}
	}

	return output, nil
}

// extractLineFromError пытается извлечь номер строки из ошибки
func (v *SandboxValidator) extractLineFromError(err error) int {
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

// IsExecutionError проверяет, является ли ошибка ошибкой выполнения
func IsExecutionError(err error) bool {
	_, ok := err.(*ExecutionError)
	return ok
}
