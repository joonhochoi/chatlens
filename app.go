package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"chatlens/internal/config"
	"chatlens/internal/db"
	"chatlens/internal/embedder"
	"chatlens/internal/llm"
	"chatlens/internal/parser"
	"chatlens/internal/search"

	selfupdate "github.com/creativeprojects/go-selfupdate"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// AppVersion 은 빌드 시 ldflags로 주입됩니다.
// 예: -ldflags "-X main.AppVersion=0.0.2"
var AppVersion = "dev"

type App struct {
	ctx      context.Context
	database *sql.DB
	embedder *embedder.Embedder
}

// SearchResult is the response returned to the frontend after a search.
type SearchResult struct {
	Summary string        `json:"summary"`
	Sources []SourceChunk `json:"sources"`
}

type SourceChunk struct {
	Text      string  `json:"text"`
	IsLeader  bool    `json:"isLeader"`
	StartTime string  `json:"startTime"`
	DateKey   string  `json:"dateKey"` // "YYYY-MM-DD" for chat history navigation
	Score     float64 `json:"score"`
}

// ChatMessage is a single message returned to the chat history view.
type ChatMessage struct {
	Time    string `json:"time"`    // "HH:MM"
	Speaker string `json:"speaker"`
	Content string `json:"content"`
}

// KeywordHit is a single message returned by keyword search.
type KeywordHit struct {
	Time    string `json:"time"`    // "HH:MM"
	Date    string `json:"date"`    // "2025년 11월 1일" — display label
	DateKey string `json:"dateKey"` // "2025-11-01" — for chat history navigation
	Speaker string `json:"speaker"`
	Content string `json:"content"`
}

// KeywordResult is the paginated response for keyword search.
type KeywordResult struct {
	Hits    []KeywordHit `json:"hits"`
	Total   int          `json:"total"`
	HasMore bool         `json:"hasMore"`
}

// UpdateInfo is the result of an update check.
type UpdateInfo struct {
	Available  bool   `json:"available"`
	Version    string `json:"version"`
	ReleaseURL string `json:"releaseUrl"`
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// DB 초기화
	dbPath, err := config.DBPath()
	if err != nil {
		runtime.EventsEmit(ctx, "app:error", map[string]any{"error": "DB 경로 오류: " + err.Error()})
		return
	}
	database, err := db.Open(dbPath)
	if err != nil {
		runtime.EventsEmit(ctx, "app:error", map[string]any{"error": "DB 초기화 오류: " + err.Error()})
		return
	}
	a.database = database

	// ONNX 임베더 초기화 (실행파일 옆 onnx/ 폴더에서 로드)
	onnxDir, err := onnxDirPath()
	if err != nil {
		runtime.EventsEmit(ctx, "app:error", map[string]any{"error": "ONNX 경로 오류: " + err.Error()})
		return
	}
	emb, err := embedder.New(onnxDir)
	if err != nil {
		runtime.EventsEmit(ctx, "app:error", map[string]any{
			"error": fmt.Sprintf("임베딩 모델 로드 실패: %v\n\n실행파일 옆 onnx/ 폴더에 model.onnx, tokenizer.json, onnxruntime.dll 이 있어야 합니다.", err),
		})
		return
	}
	a.embedder = emb

	// 자동 업데이트 체크 (백그라운드)
	go func() {
		settings, err := config.Load()
		if err != nil || !settings.AutoUpdate || AppVersion == "dev" {
			return
		}
		info, err := checkLatestUpdate()
		if err != nil || !info.Available {
			return
		}
		runtime.EventsEmit(ctx, "update:available", info)
	}()
}

// --- 설정 ---

func (a *App) GetSettings() (config.Settings, error) {
	return config.Load()
}

func (a *App) SaveSettings(s config.Settings) error {
	return config.Save(s)
}

// --- LLM ---

// CheckOllama returns true if Ollama is running on localhost:11434.
func (a *App) CheckOllama() bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:11434/api/tags")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// --- 파일 임포트 ---

// ImportFiles starts processing the given file paths asynchronously.
// Progress is sent via Wails events: "import:progress", "import:done", "import:error".
func (a *App) ImportFiles(paths []string) error {
	if a.database == nil {
		return fmt.Errorf("DB가 초기화되지 않았습니다")
	}
	if a.embedder == nil {
		return fmt.Errorf("임베더가 초기화되지 않았습니다")
	}

	go func() {
		settings, err := config.Load()
		if err != nil {
			runtime.EventsEmit(a.ctx, "import:error", map[string]any{"error": err.Error()})
			return
		}

		// 파일명 기준 오름차순 정렬
		sorted := make([]string, len(paths))
		copy(sorted, paths)
		sort.Slice(sorted, func(i, j int) bool {
			return filepath.Base(sorted[i]) < filepath.Base(sorted[j])
		})

		totalChunks := 0
		for idx, path := range sorted {
			baseName := filepath.Base(path)
			filePct := (idx * 100) / len(sorted)

			// 1. SHA256 해시 계산
			runtime.EventsEmit(a.ctx, "import:progress", map[string]any{
				"file":    baseName,
				"percent": filePct,
				"status":  "해시 계산 중...",
			})
			hash, err := sha256File(path)
			if err != nil {
				runtime.EventsEmit(a.ctx, "import:error", map[string]any{
					"error": fmt.Sprintf("%s 해시 오류: %v", baseName, err),
				})
				return
			}

			// 2. 중복 체크
			exists, err := db.IsFileProcessed(a.database, hash)
			if err != nil {
				runtime.EventsEmit(a.ctx, "import:error", map[string]any{
					"error": fmt.Sprintf("%s DB 오류: %v", baseName, err),
				})
				return
			}
			if exists {
				runtime.EventsEmit(a.ctx, "import:progress", map[string]any{
					"file":    baseName,
					"percent": filePct,
					"status":  "이미 처리된 파일 — 건너뜀",
				})
				continue
			}

			// 3. 파싱
			runtime.EventsEmit(a.ctx, "import:progress", map[string]any{
				"file":    baseName,
				"percent": filePct,
				"status":  "파싱 중...",
			})
			msgs, err := parser.ParseFile(path, settings.IgnoreKeywords)
			if err != nil {
				runtime.EventsEmit(a.ctx, "import:error", map[string]any{
					"error": fmt.Sprintf("%s 파싱 오류: %v", baseName, err),
				})
				return
			}

			// 4. 메시지 수준 중복 제거 (겹치는 내용 제거)
			runtime.EventsEmit(a.ctx, "import:progress", map[string]any{
				"file":    baseName,
				"percent": filePct,
				"status":  "중복 메시지 확인 중...",
			})
			dedupSources := make([]db.MessageForDedup, len(msgs))
			for i, m := range msgs {
				dedupSources[i] = db.MessageForDedup{
					Timestamp: m.Timestamp,
					Speaker:   m.Speaker,
					Content:   m.Content,
				}
			}
			newDedups, err := db.FilterNewMessages(a.database, dedupSources)
			if err != nil {
				runtime.EventsEmit(a.ctx, "import:error", map[string]any{
					"error": fmt.Sprintf("%s 중복 확인 오류: %v", baseName, err),
				})
				return
			}
			if len(newDedups) == 0 {
				runtime.EventsEmit(a.ctx, "import:progress", map[string]any{
					"file":    baseName,
					"percent": filePct,
					"status":  "새 메시지 없음 — 건너뜀",
				})
				continue
			}
			// 새 메시지만 parser.Message 슬라이스로 재구성
			newMsgSet := make(map[string]struct{}, len(newDedups))
			for _, d := range newDedups {
				key := d.Timestamp.UTC().Format("20060102150405") + "|" + d.Speaker + "|" + d.Content
				newMsgSet[key] = struct{}{}
			}
			var newMsgs []parser.Message
			for _, m := range msgs {
				key := m.Timestamp.UTC().Format("20060102150405") + "|" + m.Speaker + "|" + m.Content
				if _, ok := newMsgSet[key]; ok {
					newMsgs = append(newMsgs, m)
				}
			}

			// 5. 청킹
			chunkOpts := parser.ChunkOptions{
				LeaderName:        settings.LeaderName,
				MaxMessages:       settings.MaxChunkMessages,
				OverlapMessages:   settings.ChunkOverlap,
				UseLeaderMicro:    settings.UseLeaderMicro,
				UseSemanticSplit:  settings.UseSemanticChunk,
				SemanticThreshold: settings.SemanticThreshold,
			}
			if settings.UseSemanticChunk {
				chunkOpts.Embedder = a.embedder
			}
			chunks := parser.Chunkify(newMsgs, chunkOpts)
			if len(chunks) == 0 {
				continue
			}

			// 6. 청크 텍스트 수집
			texts := make([]string, len(chunks))
			for i, c := range chunks {
				texts[i] = c.Text
			}

			// 7. 임베딩 (배치)
			runtime.EventsEmit(a.ctx, "import:progress", map[string]any{
				"file":    baseName,
				"percent": filePct,
				"status":  fmt.Sprintf("임베딩 중... (0 / %d)", len(chunks)),
			})
			embeddings, err := a.embedder.EmbedBatch(texts, func(done, total int) {
				runtime.EventsEmit(a.ctx, "import:progress", map[string]any{
					"file":    baseName,
					"percent": filePct,
					"status":  fmt.Sprintf("임베딩 중... (%d / %d)", done, total),
				})
			})
			if err != nil {
				runtime.EventsEmit(a.ctx, "import:error", map[string]any{
					"error": fmt.Sprintf("%s 임베딩 오류: %v", baseName, err),
				})
				return
			}

			// 8. DB 저장 (청크 + 메시지 해시 한 트랜잭션)
			runtime.EventsEmit(a.ctx, "import:progress", map[string]any{
				"file":    baseName,
				"percent": filePct,
				"status":  "DB 저장 중...",
			})
			rows := make([]db.ChunkRow, len(chunks))
			for i, c := range chunks {
				rows[i] = db.ChunkRow{
					Text:      c.Text,
					IsLeader:  c.IsLeader,
					StartTime: c.StartTime,
					EndTime:   c.EndTime,
				}
			}
			if err := db.InsertChunks(a.database, hash, baseName, rows, embeddings, newDedups); err != nil {
				runtime.EventsEmit(a.ctx, "import:error", map[string]any{
					"error": fmt.Sprintf("%s DB 저장 오류: %v", baseName, err),
				})
				return
			}

			totalChunks += len(chunks)
		}

		runtime.EventsEmit(a.ctx, "import:done", map[string]any{
			"total": totalChunks,
		})
	}()
	return nil
}

// GetImportedFiles returns a list of already-processed file names.
func (a *App) GetImportedFiles() ([]string, error) {
	if a.database == nil {
		return []string{}, nil
	}
	return db.GetProcessedFileNames(a.database)
}

// DeleteAllData removes all imported chunks, vectors, file records, and message hashes.
func (a *App) DeleteAllData() error {
	if a.database == nil {
		return fmt.Errorf("DB가 초기화되지 않았습니다")
	}
	return db.DeleteAllData(a.database)
}

// OpenFileDialog opens a native file picker and returns selected file paths.
func (a *App) OpenFileDialog() ([]string, error) {
	return runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "채팅 파일 선택",
		Filters: []runtime.FileFilter{
			{DisplayName: "텍스트 파일 (*.txt)", Pattern: "*.txt"},
			{DisplayName: "모든 파일", Pattern: "*"},
		},
	})
}

// --- 검색 ---

const systemPrompt = `당신은 채팅 기록 분석 전문가입니다.
제공된 대화 내용을 바탕으로 사용자의 질문에 한국어로 답변하세요.
대화에 없는 내용은 추측하지 말고, 대화에서 찾은 내용만 답변하세요.
답변은 간결하고 명확하게 작성하세요.`

// Search performs a vector similarity search with optional date range filtering.
// startDate and endDate are "YYYY-MM-DD" strings; empty string means no limit.
func (a *App) Search(query string, startDate string, endDate string) (SearchResult, error) {
	if a.database == nil || a.embedder == nil {
		return SearchResult{}, fmt.Errorf("앱이 아직 초기화되지 않았습니다")
	}

	// 1. 설정 로드 (topK, LLM 설정 포함)
	settings, err := config.Load()
	if err != nil {
		return SearchResult{}, fmt.Errorf("설정 로드 오류: %w", err)
	}

	// 2. 질문 임베딩
	queryVec, err := a.embedder.EmbedQuery(query)
	if err != nil {
		return SearchResult{}, fmt.Errorf("질문 임베딩 오류: %w", err)
	}

	// 3. 벡터 검색 + 가중치 재정렬
	topK := settings.SearchTopK
	if topK < 3 || topK > 8 {
		topK = 5
	}
	candidates, err := search.TopChunks(a.database, queryVec, topK, startDate, endDate)
	if err != nil {
		return SearchResult{}, fmt.Errorf("벡터 검색 오류: %w", err)
	}
	if len(candidates) == 0 {
		return SearchResult{
			Summary: "관련 대화를 찾지 못했습니다. 채팅 파일을 먼저 가져오세요.",
			Sources: []SourceChunk{},
		}, nil
	}

	// 4. LLM 요약
	llmClient, err := llm.New(settings)
	if err != nil {
		return SearchResult{}, fmt.Errorf("LLM 초기화 오류: %w", err)
	}

	userPrompt := buildUserPrompt(query, candidates)
	summary, err := llmClient.Chat(context.Background(), systemPrompt, userPrompt)
	if err != nil {
		return SearchResult{}, fmt.Errorf("LLM 오류: %w", err)
	}

	// 5. 결과 조합
	sources := make([]SourceChunk, len(candidates))
	for i, c := range candidates {
		sources[i] = SourceChunk{
			Text:      c.Text,
			IsLeader:  c.IsLeader,
			StartTime: c.StartTime.Format("2006-01-02 15:04"),
			DateKey:   c.StartTime.Format("2006-01-02"),
			Score:     c.FinalScore,
		}
	}
	return SearchResult{Summary: summary, Sources: sources}, nil
}

func buildUserPrompt(query string, chunks []search.Candidate) string {
	var sb strings.Builder
	sb.WriteString("다음 대화 기록을 참고하여 질문에 답해주세요.\n\n[대화 기록]\n")
	for i, c := range chunks {
		sb.WriteString(fmt.Sprintf("--- 대화 %d (%s) ---\n%s\n\n",
			i+1, c.StartTime.Format("2006-01-02 15:04"), c.Text))
	}
	sb.WriteString("[질문]\n")
	sb.WriteString(query)
	return sb.String()
}

// --- 채팅 기록 ---

// GetChatDates returns the list of distinct dates ("YYYY-MM-DD") for which messages exist.
func (a *App) GetChatDates() ([]string, error) {
	if a.database == nil {
		return []string{}, nil
	}
	return db.GetChatDates(a.database)
}

// GetMessagesByDate returns all messages stored for the given date ("YYYY-MM-DD").
func (a *App) GetMessagesByDate(date string) ([]ChatMessage, error) {
	if a.database == nil {
		return []ChatMessage{}, nil
	}
	msgs, err := db.GetMessagesByDate(a.database, date)
	if err != nil {
		return nil, err
	}
	result := make([]ChatMessage, len(msgs))
	for i, m := range msgs {
		result[i] = ChatMessage{
			Time:    m.Timestamp.Format("15:04"),
			Speaker: m.Speaker,
			Content: m.Content,
		}
	}
	return result, nil
}

// --- 키워드 검색 ---

// SearchKeyword performs a simple text search over stored messages.
// offset is the number of results to skip (for load-more pagination).
// startDate and endDate are "YYYY-MM-DD" strings; empty string means no limit.
func (a *App) SearchKeyword(keyword string, offset int, startDate string, endDate string) (KeywordResult, error) {
	if a.database == nil {
		return KeywordResult{}, fmt.Errorf("DB가 초기화되지 않았습니다")
	}
	if keyword == "" {
		return KeywordResult{}, nil
	}

	msgs, total, err := db.SearchMessages(a.database, keyword, offset, startDate, endDate)
	if err != nil {
		return KeywordResult{}, err
	}

	hits := make([]KeywordHit, len(msgs))
	for i, m := range msgs {
		hits[i] = KeywordHit{
			Time:    m.Timestamp.Format("15:04"),
			Date:    fmt.Sprintf("%d년 %d월 %d일", m.Timestamp.Year(), int(m.Timestamp.Month()), m.Timestamp.Day()),
			DateKey: m.Timestamp.Format("2006-01-02"),
			Speaker: m.Speaker,
			Content: m.Content,
		}
	}
	return KeywordResult{
		Hits:    hits,
		Total:   total,
		HasMore: offset+len(msgs) < total,
	}, nil
}

// --- 업데이트 ---

// CheckUpdate 는 GitHub Releases 에서 새 버전을 확인합니다.
func (a *App) CheckUpdate() (UpdateInfo, error) {
	return checkLatestUpdate()
}

// ApplyUpdate 는 새 버전을 다운로드하고 실행 파일을 교체한 뒤 앱을 재시작합니다.
func (a *App) ApplyUpdate() error {
	updater, err := selfupdate.NewUpdater(selfupdate.Config{})
	if err != nil {
		return fmt.Errorf("업데이터 초기화 오류: %w", err)
	}
	latest, found, err := updater.DetectLatest(context.Background(), selfupdate.ParseSlug("joonhochoi/chatlens"))
	if err != nil {
		return fmt.Errorf("업데이트 확인 오류: %w", err)
	}
	if !found {
		return fmt.Errorf("업데이트를 찾을 수 없습니다")
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("실행파일 경로 오류: %w", err)
	}
	if err := updater.UpdateTo(context.Background(), latest, exe); err != nil {
		return fmt.Errorf("업데이트 적용 오류: %w", err)
	}

	// 새 바이너리로 재시작
	go func() {
		time.Sleep(500 * time.Millisecond)
		cmd := exec.Command(exe)
		cmd.Start() //nolint:errcheck
		os.Exit(0)
	}()
	return nil
}

func checkLatestUpdate() (UpdateInfo, error) {
	if AppVersion == "dev" {
		return UpdateInfo{}, fmt.Errorf("개발 버전에서는 업데이트를 확인할 수 없습니다")
	}
	updater, err := selfupdate.NewUpdater(selfupdate.Config{})
	if err != nil {
		return UpdateInfo{}, err
	}
	latest, found, err := updater.DetectLatest(context.Background(), selfupdate.ParseSlug("joonhochoi/chatlens"))
	if err != nil {
		return UpdateInfo{}, err
	}
	if !found || !latest.GreaterThan(AppVersion) {
		return UpdateInfo{Available: false}, nil
	}
	return UpdateInfo{
		Available:  true,
		Version:    latest.Version(),
		ReleaseURL: fmt.Sprintf("https://github.com/joonhochoi/chatlens/releases/tag/v%s", latest.Version()),
	}, nil
}

// --- 헬퍼 ---

// onnxDirPath returns the path to the onnx/ folder next to the executable.
func onnxDirPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), "onnx"), nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
