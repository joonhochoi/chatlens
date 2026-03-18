package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Settings struct {
	LeaderName     string   `json:"leaderName"`
	IgnoreKeywords []string `json:"ignoreKeywords"`
	LLMProvider    string   `json:"llmProvider"`    // "ollama" | "gemini" | "openai"
	APIKey         string   `json:"apiKey"`
	OllamaModel    string   `json:"ollamaModel"`
	EmbeddingModel string   `json:"embeddingModel"` // Ollama embedding model name
	SearchTopK     int      `json:"searchTopK"`     // 검색 결과 수 (3~8)

	// 청킹 전략
	MaxChunkMessages  int     `json:"maxChunkMessages"`  // 청크당 최대 메시지 수 (기본 40)
	ChunkOverlap      int     `json:"chunkOverlap"`      // 인접 청크 간 오버랩 메시지 수 (기본 3)
	UseLeaderMicro    bool    `json:"useLeaderMicro"`    // 리더 발언 마이크로 청킹
	UseSemanticChunk  bool    `json:"useSemanticChunk"`  // 의미 기반 추가 분할
	SemanticThreshold float64 `json:"semanticThreshold"` // 의미 유사도 임계값 (기본 0.65)
}

func defaultSettings() Settings {
	return Settings{
		LeaderName:        "",
		IgnoreKeywords:    []string{"사진", "동영상", "이모티콘"},
		LLMProvider:       "ollama",
		APIKey:            "",
		OllamaModel:       "llama3:latest",
		EmbeddingModel:    "nomic-embed-text",
		SearchTopK:        5,
		MaxChunkMessages:  40,
		ChunkOverlap:      3,
		UseLeaderMicro:    false,
		UseSemanticChunk:  false,
		SemanticThreshold: 0.65,
	}
}

func configDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(exe)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

func Load() (Settings, error) {
	path, err := configPath()
	if err != nil {
		return defaultSettings(), err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return defaultSettings(), nil
	}
	if err != nil {
		return defaultSettings(), err
	}

	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return defaultSettings(), err
	}
	return s, nil
}

func Save(s Settings) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// DBPath returns the path for the SQLite database file.
func DBPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "chat_data.db"), nil
}
