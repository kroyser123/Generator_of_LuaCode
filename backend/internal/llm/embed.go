package llm

import (
	"crypto/md5"
	"encoding/binary"
	"math/rand"
)

// Embedder — интерфейс для получения эмбеддингов
type Embedder interface {
	Embed(text string) ([]float32, error)
}

// MockEmbedder — заглушка (пока не придет ML)
type MockEmbedder struct {
	dim int
}

// NewMockEmbedder создает новую заглушку
func NewMockEmbedder(dim int) *MockEmbedder {
	return &MockEmbedder{dim: dim}
}

// Embed возвращает детерминированный псевдо-случайный вектор
func (e *MockEmbedder) Embed(text string) ([]float32, error) {
	// Детерминированный рандом на основе текста
	hash := md5.Sum([]byte(text))
	seed := int64(binary.BigEndian.Uint64(hash[:8]))
	r := rand.New(rand.NewSource(seed))

	vec := make([]float32, e.dim)
	for i := range vec {
		vec[i] = r.Float32()
	}
	return vec, nil
}

// RealEmbedder — будет реализован ML-инженером
// Потом заменим MockEmbedder на RealEmbedder, который вызывает эмбеддинг сервер
type RealEmbedder struct {
	endpoint string
}

func NewRealEmbedder(endpoint string) *RealEmbedder {
	return &RealEmbedder{endpoint: endpoint}
}

func (e *RealEmbedder) Embed(text string) ([]float32, error) {
	// TODO: ML-инженер реализует вызов к эмбеддинг серверу
	// POST /embed -> {vector: [...]}
	return nil, nil
}
