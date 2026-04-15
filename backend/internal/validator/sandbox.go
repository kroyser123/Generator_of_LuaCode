package validator

import (
	"context"
	"fmt"
	"strings"
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

// Run выполняет код с заданными входными данными (для самопроверки)
func (v *SandboxValidator) Run(code string, input map[string]interface{}) (string, error) {
	L := lua.NewState()
	defer L.Close()

	// Настраиваем окружение MWS Octapi
	wf := L.NewTable()
	wfVars := L.NewTable()
	wfInitVars := L.NewTable()

	// Заполняем входные данные
	for key, value := range input {
		if strings.HasPrefix(key, "wf.vars.") {
			varName := strings.TrimPrefix(key, "wf.vars.")
			L.SetField(wfVars, varName, toLuaValue(L, value))
		}
	}

	L.SetField(wf, "vars", wfVars)
	L.SetField(wf, "initVariables", wfInitVars)
	L.SetGlobal("wf", wf)

	// Добавляем _utils.array
	utils := L.NewTable()
	utilsArray := L.NewTable()
	L.SetField(utilsArray, "new", L.NewFunction(func(L *lua.LState) int {
		L.Push(L.NewTable())
		return 1
	}))
	L.SetField(utils, "array", utilsArray)
	L.SetGlobal("_utils", utils)

	// Очищаем код от обёртки
	cleanCode := ExtractMWSCode(code)

	// Добавляем принудительный return, если его нет
	if !strings.Contains(cleanCode, "return") {
		cleanCode = cleanCode + "\nreturn nil"
	}

	// Выполняем код
	if err := L.DoString(cleanCode); err != nil {
		return "", err
	}

	// Получаем результат
	result := L.Get(-1)
	return luaValueToString(result), nil
}

func (v *SandboxValidator) executeSandbox(code string) (string, error) {
	L := lua.NewState()
	defer L.Close()

	// MWS Octapi окружение
	wf := L.NewTable()
	wfVars := L.NewTable()
	wfInitVars := L.NewTable()

	L.SetField(wf, "vars", wfVars)
	L.SetField(wf, "initVariables", wfInitVars)
	L.SetGlobal("wf", wf)

	utils := L.NewTable()
	utilsArray := L.NewTable()
	L.SetField(utilsArray, "new", L.NewFunction(func(L *lua.LState) int {
		L.Push(L.NewTable())
		return 1
	}))
	L.SetField(utilsArray, "markAsArray", L.NewFunction(func(L *lua.LState) int {
		if L.GetTop() >= 1 {
			L.Push(L.Get(1))
		} else {
			L.Push(lua.LNil)
		}
		return 1
	}))
	L.SetField(utils, "array", utilsArray)
	L.SetGlobal("_utils", utils)

	// Сбор вывода
	var output strings.Builder
	L.SetGlobal("print", L.NewFunction(func(L *lua.LState) int {
		top := L.GetTop()
		for i := 1; i <= top; i++ {
			if i > 1 {
				output.WriteString("\t")
			}
			output.WriteString(L.ToString(i))
		}
		output.WriteString("\n")
		return 0
	}))

	// Безопасность
	dangerousLibs := []string{"os", "io", "debug", "coroutine", "package", "socket", "http"}
	for _, lib := range dangerousLibs {
		L.PreloadModule(lib, nil)
	}

	// Очищаем код от обёртки
	cleanCode := ExtractMWSCode(code)

	if err := L.DoString(cleanCode); err != nil {
		return "", &ExecutionError{
			Message: err.Error(),
			Type:    "runtime",
			Line:    v.extractLineFromError(err),
		}
	}

	return output.String(), nil
}

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

func toLuaValue(L *lua.LState, val interface{}) lua.LValue {
	switch v := val.(type) {
	case int:
		return lua.LNumber(v)
	case float64:
		return lua.LNumber(v)
	case string:
		return lua.LString(v)
	case []interface{}:
		tbl := L.NewTable()
		for i, item := range v {
			L.RawSetInt(tbl, i+1, toLuaValue(L, item))
		}
		return tbl
	case []string:
		tbl := L.NewTable()
		for i, item := range v {
			L.RawSetInt(tbl, i+1, lua.LString(item))
		}
		return tbl
	default:
		return lua.LNil
	}
}

func luaValueToString(val lua.LValue) string {
	switch val.Type() {
	case lua.LTNumber:
		return val.String()
	case lua.LTString:
		return val.String()
	case lua.LTTable:
		var result []string
		val.(*lua.LTable).ForEach(func(key, value lua.LValue) {
			result = append(result, luaValueToString(value))
		})
		return fmt.Sprintf("[%s]", strings.Join(result, " "))
	default:
		return val.String()
	}
}

// IsExecutionError проверяет, является ли ошибка ошибкой выполнения
func IsExecutionError(err error) bool {
	_, ok := err.(*ExecutionError)
	return ok
}
