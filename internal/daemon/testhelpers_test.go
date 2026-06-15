package daemon

import (
	"context"
)

type mockEmbedEngine struct {
	modelPresent bool
	embedResult  [][]float32
	embedErr     error
	embedCalled  int
}

func (m *mockEmbedEngine) IsModelPresent() bool {
	return m.modelPresent
}

func (m *mockEmbedEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	m.embedCalled++
	if m.embedErr != nil {
		return nil, m.embedErr
	}
	result := make([][]float32, len(texts))
	for i := range texts {
		if i < len(m.embedResult) {
			result[i] = m.embedResult[i]
		} else {
			result[i] = make([]float32, 384)
		}
	}
	return result, nil
}
