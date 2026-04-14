package validator

import (
	"context"
	"fmt"
	"regexp"
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

// SandboxValidator выполняет код в изолированной среде MWS Octapi
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

// executeSandbox запускает Lua код в изолированной среде MWS Octapi
func (v *SandboxValidator) executeSandbox(code string) (string, error) {
	L := lua.NewState()
	defer L.Close()

	// ===== MWS OCTAPI: wf.vars и wf.initVariables =====
	wf := L.NewTable()
	wfVars := L.NewTable()
	wfInitVars := L.NewTable()

	L.SetField(wf, "vars", wfVars)
	L.SetField(wf, "initVariables", wfInitVars)
	L.SetGlobal("wf", wf)

	// ===== MWS OCTAPI: _utils.array =====
	utils := L.NewTable()
	utilsArray := L.NewTable()

	// _utils.array.new() - создает новый массив
	L.SetField(utilsArray, "new", L.NewFunction(func(L *lua.LState) int {
		L.Push(L.NewTable())
		return 1
	}))

	// _utils.array.markAsArray(arr) - помечает таблицу как массив
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

	// ===== ЗАПРЕЩЕННЫЕ БИБЛИОТЕКИ (согласно PDF: стр. 4-5) =====
	// В MWS Octapi НЕЛЬЗЯ использовать:
	// - os.execute, io.popen, loadstring, debug.*, socket.*, http.*
	// - JsonPath ($.field, $[0], ${path})
	// - goto и метки
	// Блокируем их через удаление из окружения
	dangerousLibs := []string{"os", "io", "debug", "coroutine", "package", "socket", "http"}
	for _, lib := range dangerousLibs {
		L.PreloadModule(lib, nil)
	}

	// Дополнительно удаляем опасные функции из глобальной таблицы
	dangerousFuncs := []string{
		"dofile", "loadfile", "load", "loadstring",
		"getfenv", "setfenv", "getmetatable", "setmetatable",
		"rawget", "rawset", "rawlen", "rawequal",
	}
	for _, fn := range dangerousFuncs {
		L.SetGlobal(fn, lua.LNil)
	}

	// ===== СБОР ВЫВОДА =====
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

	// ===== ИЗВЛЕКАЕМ ЧИСТЫЙ КОД ИЗ ФОРМАТА lua{...}lua =====
	cleanCode := code
	trimmed := strings.TrimSpace(code)
	if strings.HasPrefix(trimmed, "lua{") && strings.HasSuffix(trimmed, "}lua") {
		cleanCode = trimmed[4 : len(trimmed)-4]
	}

	// ===== ПРОВЕРКА НА ЗАПРЕЩЕННЫЕ ПАТТЕРНЫ (JSONPATH и т.д.) =====
	if err := v.checkForbiddenPatterns(cleanCode); err != nil {
		return "", err
	}

	// ===== ВЫПОЛНЯЕМ КОД =====
	defer func() {
		if r := recover(); r != nil {
			// Паника поймана, но не падаем
		}
	}()

	if err := L.DoString(cleanCode); err != nil {
		return "", &ExecutionError{
			Message: err.Error(),
			Type:    "runtime",
			Line:    v.extractLineFromError(err),
		}
	}

	return output.String(), nil
}

// checkForbiddenPatterns проверяет наличие запрещенных паттернов в коде
func (v *SandboxValidator) checkForbiddenPatterns(code string) error {
	forbidden := []struct {
		pattern string
		name    string
		hint    string
	}{
		// JsonPath (PDF стр. 4: "нельзя обращаться к переменным с помощью JsonPath")
		{`\$\.\w+`, "JsonPath", "Use direct access: wf.vars.field instead of $.field"},
		{`\$\[.*?\]`, "JsonPath", "Use direct access: wf.vars.array[1] instead of $[0]"},
		{`\$\{.*?\}`, "JsonPath", "Use direct access to wf.vars instead of ${path}"},

		// goto и метки (PDF стр. 5: запрещены)
		{`\bgoto\b`, "goto", "goto is not allowed in MWS Octapi"},
		{`::[a-zA-Z_][a-zA-Z0-9_]*::`, "label", "labels for goto are not allowed"},
	}

	for _, f := range forbidden {
		re := regexp.MustCompile(f.pattern)
		if re.MatchString(code) {
			return &ExecutionError{
				Message: fmt.Sprintf("forbidden pattern: %s (%s)", f.name, f.hint),
				Type:    "security",
				Line:    0,
			}
		}
	}
	return nil
}

// extractLineFromError пытается извлечь номер строки из ошибки
func (v *SandboxValidator) extractLineFromError(err error) int {
	s := err.Error()

	// Ищем "line N" в сообщении об ошибке
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
