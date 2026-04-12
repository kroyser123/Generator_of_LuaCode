package llm

import (
	"strings"
)

// RAG — интерфейс для поиска похожих примеров
type RAG interface {
	FindSimilar(prompt string, limit int) ([]string, error)
}

// MockRAG — заглушка (поиск по ключевым словам)
type MockRAG struct {
	examples []string
}

// NewMockRAG создает новую заглушку
func NewMockRAG() *MockRAG {
	return &MockRAG{
		examples: []string{
			`return wf.vars.emails[#wf.vars.emails]`,
			`return wf.vars.try_count_n + 1`,
			`local result = _utils.array.new()
for _, item in ipairs(wf.vars.items) do
    if item.value ~= nil then
        table.insert(result, item)
    end
end
return result`,
			`local function calculate_sum(arr)
    local sum = 0
    for _, v in ipairs(arr) do
        sum = sum + v
    end
    return sum
end
return calculate_sum(wf.vars.numbers)`,
		},
	}
}

// FindSimilar ищет похожие примеры по ключевым словам
func (r *MockRAG) FindSimilar(prompt string, limit int) ([]string, error) {
	var results []string
	lowerPrompt := strings.ToLower(prompt)

	for _, ex := range r.examples {
		// Простой поиск по ключевым словам
		if r.matchExample(lowerPrompt, ex) {
			results = append(results, ex)
		}
		if len(results) >= limit {
			break
		}
	}

	// Если ничего не нашли, берем первые примеры
	if len(results) == 0 && len(r.examples) > 0 {
		for i := 0; i < limit && i < len(r.examples); i++ {
			results = append(results, r.examples[i])
		}
	}

	return results, nil
}

func (r *MockRAG) matchExample(prompt, example string) bool {
	keywords := map[string][]string{
		"email":  {"email", "emails", "mail"},
		"array":  {"array", "list", "массив", "список"},
		"filter": {"filter", "фильтр", "очист"},
		"count":  {"count", "счет", "попыток", "счетчик"},
		"sum":    {"sum", "сумм", "сложение"},
		"last":   {"last", "последн", "последний"},
	}

	for key, words := range keywords {
		for _, word := range words {
			if strings.Contains(prompt, word) && strings.Contains(example, key) {
				return true
			}
		}
	}
	return false
}

// RealRAG — будет реализован ML-инженером через pgvector
type RealRAG struct {
	pgvectorEndpoint string
}

func NewRealRAG(endpoint string) *RealRAG {
	return &RealRAG{pgvectorEndpoint: endpoint}
}

func (r *RealRAG) FindSimilar(prompt string, limit int) ([]string, error) {
	// TODO: ML-инженер реализует:
	// 1. Вызов эмбеддинг сервера для векторизации prompt
	// 2. Поиск по pgvector (ORDER BY embedding <-> query LIMIT limit)
	// 3. Возврат кода примеров
	return nil, nil
}
