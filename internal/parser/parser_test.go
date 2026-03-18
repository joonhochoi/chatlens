package parser

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// helper: write a temp file with the given content and return its path.
func writeTmp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "chat*.txt")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(content)
	f.Close()
	return f.Name()
}

// ── ParseFile tests ───────────────────────────────────────────────────────────

func TestParseFile_BasicMessages(t *testing.T) {
	content := `저장한 날짜 : 2026. 3. 15. 오후 3:34

2025년 7월 22일 화요일
2025. 7. 22. 오전 8:33, 채상욱 리더 : 안녕하세요
2025. 7. 22. 오전 8:34, 홍길동 : 반갑습니다
`
	path := writeTmp(t, content)
	msgs, err := ParseFile(path, nil)
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Speaker != "채상욱 리더" {
		t.Errorf("speaker[0] = %q", msgs[0].Speaker)
	}
	if msgs[0].Content != "안녕하세요" {
		t.Errorf("content[0] = %q", msgs[0].Content)
	}
	if msgs[1].Speaker != "홍길동" {
		t.Errorf("speaker[1] = %q", msgs[1].Speaker)
	}
}

func TestParseFile_Multiline(t *testing.T) {
	content := `2025. 7. 22. 오전 8:33, 채상욱 리더 : 첫 번째 줄
두 번째 줄도 같은 메시지
세 번째 줄
2025. 7. 22. 오전 8:35, 홍길동 : 다음 메시지
`
	path := writeTmp(t, content)
	msgs, err := ParseFile(path, nil)
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2, got %d", len(msgs))
	}
	expected := "첫 번째 줄\n두 번째 줄도 같은 메시지\n세 번째 줄"
	if msgs[0].Content != expected {
		t.Errorf("multiline content = %q, want %q", msgs[0].Content, expected)
	}
}

func TestParseFile_SystemMessagesIgnored(t *testing.T) {
	content := `2025. 7. 22. 오전 8:33: 우드워커님이 들어왔습니다.
2025. 7. 22. 오전 8:34, 채상욱 리더 : 어서오세요
2025. 7. 22. 오전 8:35: 박애용님이 나갔습니다.
`
	path := writeTmp(t, content)
	msgs, err := ParseFile(path, nil)
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1, got %d", len(msgs))
	}
}

func TestParseFile_IgnoreKeywords(t *testing.T) {
	content := `2025. 7. 22. 오전 8:33, 채상욱 리더 : 사진
2025. 7. 22. 오전 8:34, 채상욱 리더 : 이모티콘
2025. 7. 22. 오전 8:35, 채상욱 리더 : 정상 메시지
2025. 7. 22. 오전 8:36, 채상욱 리더 : 동영상
`
	path := writeTmp(t, content)
	ignore := []string{"사진", "이모티콘", "동영상"}
	msgs, err := ParseFile(path, ignore)
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1, got %d: %v", len(msgs), msgs)
	}
	if msgs[0].Content != "정상 메시지" {
		t.Errorf("content = %q", msgs[0].Content)
	}
}

func TestParseFile_AmPmEdgeCases(t *testing.T) {
	// 오전 12시 → 0시 (자정), 오후 12시 → 12시 (정오), 오후 1시 → 13시
	content := `2025. 7. 22. 오전 12:00, A : 자정
2025. 7. 22. 오후 12:00, B : 정오
2025. 7. 22. 오후 1:00, C : 오후1시
`
	path := writeTmp(t, content)
	msgs, err := ParseFile(path, nil)
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3, got %d", len(msgs))
	}
	if msgs[0].Timestamp.Hour() != 0 {
		t.Errorf("오전 12시 → hour %d, want 0", msgs[0].Timestamp.Hour())
	}
	if msgs[1].Timestamp.Hour() != 12 {
		t.Errorf("오후 12시 → hour %d, want 12", msgs[1].Timestamp.Hour())
	}
	if msgs[2].Timestamp.Hour() != 13 {
		t.Errorf("오후 1시 → hour %d, want 13", msgs[2].Timestamp.Hour())
	}
}

func TestParseFile_RealSampleFile(t *testing.T) {
	// 실제 샘플 파일이 있으면 테스트, 없으면 스킵
	path := filepath.Join("..", "..", "chat_log", "Talk_2026.3.15 14_39-1.txt")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("sample file not found, skipping integration test")
	}
	ignore := []string{"사진", "동영상", "이모티콘"}
	msgs, err := ParseFile(path, ignore)
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}
	if len(msgs) == 0 {
		t.Error("expected messages from real file, got 0")
	}
	t.Logf("parsed %d messages from sample file", len(msgs))
}

// ── Chunkify tests ────────────────────────────────────────────────────────────

func makeMsg(speaker, content string, ts time.Time) Message {
	return Message{Timestamp: ts, Speaker: speaker, Content: content}
}

func base() time.Time {
	return time.Date(2025, 7, 22, 8, 0, 0, 0, time.Local)
}

func TestChunkify_30MinGap(t *testing.T) {
	t0 := base()
	msgs := []Message{
		makeMsg("A", "msg1", t0),
		makeMsg("A", "msg2", t0.Add(29*time.Minute)),
		makeMsg("A", "msg3", t0.Add(60*time.Minute)), // 31분 차이 → 새 청크
	}
	chunks := Chunkify(msgs, ChunkOptions{})
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if len(chunks[0].Messages) != 2 {
		t.Errorf("chunk[0] should have 2 messages, got %d", len(chunks[0].Messages))
	}
	if len(chunks[1].Messages) != 1 {
		t.Errorf("chunk[1] should have 1 message, got %d", len(chunks[1].Messages))
	}
}

func TestChunkify_LeaderSession5MinGap(t *testing.T) {
	t0 := base()
	// 리더 발언 후 5분 이상 공백이 생기면 새 청크 (아직 30분은 안 됨)
	msgs := []Message{
		makeMsg("채상욱 리더", "리더 발언1", t0),
		makeMsg("채상욱 리더", "리더 발언2", t0.Add(4*time.Minute)),
		makeMsg("채상욱 리더", "리더 발언3", t0.Add(10*time.Minute)), // 6분 차이 → 리더세션 끊김
		makeMsg("홍길동", "일반 발언", t0.Add(12*time.Minute)),
	}
	chunks := Chunkify(msgs, ChunkOptions{LeaderName: "채상욱 리더"})
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if len(chunks[0].Messages) != 2 {
		t.Errorf("chunk[0] should have 2 messages, got %d", len(chunks[0].Messages))
	}
}

func TestChunkify_IsLeaderFlag(t *testing.T) {
	t0 := base()
	msgs := []Message{
		makeMsg("홍길동", "일반1", t0),
		makeMsg("홍길동", "일반2", t0.Add(1*time.Minute)),
		makeMsg("홍길동", "일반3", t0.Add(35*time.Minute)), // 34분 차이 → 새 청크
		makeMsg("채상욱 리더", "리더 발언", t0.Add(36*time.Minute)),
	}
	chunks := Chunkify(msgs, ChunkOptions{LeaderName: "채상욱 리더"})
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0].IsLeader {
		t.Error("chunk[0] should NOT be leader chunk")
	}
	if !chunks[1].IsLeader {
		t.Error("chunk[1] should be leader chunk")
	}
}

func TestChunkify_TextSerialization(t *testing.T) {
	t0 := base()
	msgs := []Message{
		makeMsg("A", "hello", t0),
		makeMsg("B", "world", t0.Add(time.Minute)),
	}
	chunks := Chunkify(msgs, ChunkOptions{})
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	want := "A: hello\nB: world"
	if chunks[0].Text != want {
		t.Errorf("Text = %q, want %q", chunks[0].Text, want)
	}
}

func TestChunkify_EmptyInput(t *testing.T) {
	chunks := Chunkify(nil, ChunkOptions{LeaderName: "리더"})
	if chunks != nil {
		t.Errorf("expected nil, got %v", chunks)
	}
}

func TestChunkify_NoLeaderName(t *testing.T) {
	// leaderName이 빈 문자열이면 IsLeader는 항상 false, 5분 규칙 미적용
	t0 := base()
	msgs := []Message{
		makeMsg("A", "msg1", t0),
		makeMsg("A", "msg2", t0.Add(6*time.Minute)), // 6분, leaderName 없으므로 청크 유지
	}
	chunks := Chunkify(msgs, ChunkOptions{})
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].IsLeader {
		t.Error("IsLeader should be false when leaderName is empty")
	}
}
