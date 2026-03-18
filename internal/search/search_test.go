package search

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"chatlens/internal/db"
	"chatlens/internal/embedder"
	"chatlens/internal/parser"
)

func onnxDir(t *testing.T) string {
	t.Helper()
	dir, _ := filepath.Abs(filepath.Join("..", "..", "onnx"))
	if _, err := os.Stat(filepath.Join(dir, "model.onnx")); os.IsNotExist(err) {
		t.Skip("onnx/model.onnx not found — skipping ONNX integration tests")
	}
	return dir
}

// TestSearchPipeline는 임베더 + DB + 검색 전체 파이프라인을 통합 테스트합니다.
func TestSearchPipeline(t *testing.T) {
	emb, err := embedder.New(onnxDir(t))
	if err != nil {
		t.Fatalf("embedder.New: %v", err)
	}
	defer emb.Close()

	// 임시 DB 생성
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer database.Close()

	// 샘플 청크 삽입
	texts := []string{
		"채상욱 리더: 3기 신도시 분양은 무조건 받는 게 좋습니다. LH 청약플러스도 꼭 확인하세요.",
		"채상욱 리더: 배당소득 분리과세 원안이 유리합니다. 과세 탈레반 세력이 밀리고 있어요.",
		"홍길동: 오늘 날씨가 정말 좋네요. 산책이라도 다녀올까요?",
		"채상욱 리더: 수출기업 ETF 지금 진입해도 괜찮을지 신중하게 판단해야 합니다.",
	}
	rows := make([]db.ChunkRow, len(texts))
	for i, txt := range texts {
		rows[i] = db.ChunkRow{
			Text:      txt,
			IsLeader:  i != 2, // 날씨 청크만 non-leader
			StartTime: time.Now().Add(time.Duration(i) * time.Hour),
			EndTime:   time.Now().Add(time.Duration(i)*time.Hour + 5*time.Minute),
		}
	}

	embeddings, err := emb.EmbedBatch(texts, nil)
	if err != nil {
		t.Fatalf("EmbedBatch: %v", err)
	}
	if err := db.InsertChunks(database, "testhash", "test.txt", rows, embeddings, nil); err != nil {
		t.Fatalf("InsertChunks: %v", err)
	}

	// 부동산 관련 질문 검색
	queryVec, err := emb.EmbedQuery("신도시 청약 어떻게 해야 하나요")
	if err != nil {
		t.Fatalf("EmbedQuery: %v", err)
	}

	candidates, err := TopChunks(database, queryVec, 5, "", "")
	if err != nil {
		t.Fatalf("TopChunks: %v", err)
	}
	if len(candidates) == 0 {
		t.Fatal("no candidates returned")
	}

	t.Logf("Top %d results:", len(candidates))
	for i, c := range candidates {
		t.Logf("  [%d] score=%.4f isLeader=%v text=%q", i+1, c.FinalScore, c.IsLeader, c.Text[:min(60, len(c.Text))])
	}

	// 신도시 청크가 날씨 청크보다 높은 순위여야 함
	topText := candidates[0].Text
	if topText == texts[2] { // 날씨 청크가 1위면 실패
		t.Error("irrelevant weather chunk should not be top result")
	}
}

// TestSearchRealFile은 실제 샘플 파일로 전체 파이프라인을 테스트합니다.
func TestSearchRealFile(t *testing.T) {
	samplePath := filepath.Join("..", "..", "chat_log", "Talk_2026.3.15 14_39-1.txt")
	if _, err := os.Stat(samplePath); os.IsNotExist(err) {
		t.Skip("sample file not found")
	}

	emb, err := embedder.New(onnxDir(t))
	if err != nil {
		t.Fatalf("embedder.New: %v", err)
	}
	defer emb.Close()

	database, err := db.Open(filepath.Join(t.TempDir(), "real.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer database.Close()

	// 파싱 + 청킹
	msgs, err := parser.ParseFile(samplePath, []string{"사진", "동영상", "이모티콘"})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	chunks := parser.Chunkify(msgs, parser.ChunkOptions{LeaderName: "채상욱 리더", MaxMessages: 40, OverlapMessages: 3})
	t.Logf("chunks: %d", len(chunks))

	texts := make([]string, len(chunks))
	rows := make([]db.ChunkRow, len(chunks))
	for i, c := range chunks {
		texts[i] = c.Text
		rows[i] = db.ChunkRow{
			Text:      c.Text,
			IsLeader:  c.IsLeader,
			StartTime: c.StartTime,
			EndTime:   c.EndTime,
		}
	}

	// 임베딩 (첫 50청크만 — 테스트 속도 제한)
	limit := 50
	if len(texts) < limit {
		limit = len(texts)
	}
	embeddings, err := emb.EmbedBatch(texts[:limit], func(done, total int) {
		if done%10 == 0 {
			t.Logf("  embedding %d/%d", done, total)
		}
	})
	if err != nil {
		t.Fatalf("EmbedBatch: %v", err)
	}
	if err := db.InsertChunks(database, "realhash", "real.txt", rows[:limit], embeddings, nil); err != nil {
		t.Fatalf("InsertChunks: %v", err)
	}

	// 검색
	queryVec, _ := emb.EmbedQuery("리더가 부동산에 대해 뭐라고 했나요")
	candidates, err := TopChunks(database, queryVec, 5, "", "")
	if err != nil {
		t.Fatalf("TopChunks: %v", err)
	}
	t.Logf("Search results for '리더가 부동산에 대해 뭐라고 했나요':")
	for i, c := range candidates {
		t.Logf("  [%d] score=%.4f isLeader=%v\n      %s", i+1, c.FinalScore, c.IsLeader, c.Text[:min(100, len(c.Text))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
