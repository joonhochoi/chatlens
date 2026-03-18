package db

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenAndSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	database, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer database.Close()

	// 스키마가 적용됐는지 확인
	_, err = database.Exec(`INSERT INTO processed_files (file_name, file_hash) VALUES ('test.txt', 'abc123')`)
	if err != nil {
		t.Fatalf("insert processed_files: %v", err)
	}
}

func TestIsFileProcessed(t *testing.T) {
	dir := t.TempDir()
	database, _ := Open(filepath.Join(dir, "test.db"))
	defer database.Close()

	exists, err := IsFileProcessed(database, "deadbeef")
	if err != nil || exists {
		t.Fatalf("expected false, got exists=%v err=%v", exists, err)
	}

	database.Exec(`INSERT INTO processed_files (file_name, file_hash) VALUES ('f.txt', 'deadbeef')`)
	exists, err = IsFileProcessed(database, "deadbeef")
	if err != nil || !exists {
		t.Fatalf("expected true, got exists=%v err=%v", exists, err)
	}
}

func TestInsertChunks(t *testing.T) {
	dir := t.TempDir()
	database, _ := Open(filepath.Join(dir, "test.db"))
	defer database.Close()

	now := time.Now()
	rows := []ChunkRow{
		{Text: "A: hello\nB: world", IsLeader: true, StartTime: now, EndTime: now.Add(time.Minute)},
		{Text: "C: bye", IsLeader: false, StartTime: now.Add(time.Hour), EndTime: now.Add(time.Hour)},
	}
	// 384-dim zero vectors (multilingual-e5-small)
	emb := make([][]float32, 2)
	for i := range emb {
		emb[i] = make([]float32, 384)
	}

	if err := InsertChunks(database, "hash1", "chat.txt", rows, emb, nil); err != nil {
		t.Fatalf("InsertChunks: %v", err)
	}

	var count int
	database.QueryRow(`SELECT COUNT(*) FROM chunks`).Scan(&count)
	if count != 2 {
		t.Errorf("expected 2 chunks, got %d", count)
	}

	database.QueryRow(`SELECT COUNT(*) FROM chunk_vectors`).Scan(&count)
	if count != 2 {
		t.Errorf("expected 2 vectors, got %d", count)
	}
}

func TestGetProcessedFileNames(t *testing.T) {
	dir := t.TempDir()
	database, _ := Open(filepath.Join(dir, "test.db"))
	defer database.Close()

	names, err := GetProcessedFileNames(database)
	if err != nil || len(names) != 0 {
		t.Fatalf("expected empty, got %v err=%v", names, err)
	}

	database.Exec(`INSERT INTO processed_files (file_name, file_hash) VALUES ('a.txt', 'h1')`)
	database.Exec(`INSERT INTO processed_files (file_name, file_hash) VALUES ('b.txt', 'h2')`)
	names, _ = GetProcessedFileNames(database)
	if len(names) != 2 {
		t.Errorf("expected 2 names, got %v", names)
	}
}

func TestDuplicateInsertIgnored(t *testing.T) {
	dir := t.TempDir()
	database, _ := Open(filepath.Join(dir, "test.db"))
	defer database.Close()

	// INSERT OR IGNORE — 같은 hash 두 번 넣어도 에러 없어야 함
	for i := 0; i < 2; i++ {
		database.Exec(`INSERT OR IGNORE INTO processed_files (file_name, file_hash) VALUES ('f.txt', 'same_hash')`)
	}
	var count int
	database.QueryRow(`SELECT COUNT(*) FROM processed_files`).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}
}

func init() {
	// 이 패키지를 빌드할 때 sqlite-vec.h를 찾을 수 있도록 CGO_CFLAGS가 설정되어야 합니다.
	// go test ./internal/db/... 실행 시:
	//   CGO_ENABLED=1 CGO_CFLAGS="-I/c/msys64/mingw64/include" go test ./internal/db/...
	_ = os.Getenv // 환경 변수 패키지 import 방지용 dummy
}
