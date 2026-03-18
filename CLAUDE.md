# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 프로젝트 개요

**ChatLens** — 수개월 치의 카카오톡 채팅 기록(.txt)을 로컬에서 분석하여 자연어 질문에 답변하는 데스크톱 앱. 100% 로컬 데이터 처리가 핵심 가치.

## 기술 스택

| 레이어 | 기술 | 비고 |
|--------|------|------|
| 데스크톱 프레임워크 | Wails v2 | Go + OS 기본 WebView |
| 백엔드 | Go 1.21+ | CGO 활성화 필수 |
| 프론트엔드 | React + TypeScript | Wails 템플릿 `react-ts` |
| 로컬 DB | SQLite + `sqlite-vec` | C 확장, CGO 빌드 필요 |
| 임베딩 | FastEmbed-Go (`intfloat/multilingual-e5-small`, ~120MB) | 최초 실행 시 자동 다운로드 |
| LLM | Ollama (기본) → Gemini/OpenAI API (폴백) | 포트 11434 자동 감지 |
| 설정 저장 | JSON 파일 (`AppData/chatlens/settings.json`) | |

## 개발 환경 전제조건

```bash
# 필수 도구 (모두 설치되어 있어야 wails build 가능)
Go 1.21+
Node.js 18+
Wails CLI v2:  go install github.com/wailsapp/wails/v2/cmd/wails@latest
MinGW-w64 (Windows):  CGO 컴파일러, sqlite-vec 빌드에 필수
  → choco install mingw  또는  scoop install gcc
```

## 개발 명령어

모든 Wails 명령어는 **반드시 프로젝트 루트** (`chatlens/`)에서 실행해야 합니다.
`frontend/` 디렉토리에서 `cd` 한 뒤 Wails 명령어를 실행하면 `wails.json`을 잘못 찾습니다.

```bash
# 개발 서버 실행 (핫 리로드, 항상 프로젝트 루트에서)
cd /path/to/chatlens && CGO_ENABLED=1 wails dev

# 프로덕션 빌드 (단일 .exe 생성)
cd /path/to/chatlens && CGO_ENABLED=1 wails build

# Go 백엔드 유닛 테스트
go test ./internal/...

# 특정 패키지만 테스트
go test ./internal/parser/... -v

# 프론트엔드 TypeScript 타입 체크 (frontend/ 디렉토리에서)
cd frontend && ./node_modules/.bin/tsc --noEmit
```

## 디렉토리 구조 (목표)

```
chatlens/
├── main.go                   # Wails 앱 진입점
├── app.go                    # Wails App 구조체, 바인딩 함수 정의
├── wails.json
├── go.mod
├── frontend/                 # React + TypeScript UI
│   ├── src/
│   │   ├── App.tsx
│   │   ├── pages/
│   │   │   ├── SettingsPage.tsx
│   │   │   ├── UploadPage.tsx
│   │   │   └── SearchPage.tsx
│   │   └── wailsjs/         # 자동 생성 (wails dev 실행 시)
├── internal/
│   ├── parser/              # 채팅 파싱 + 청킹
│   │   ├── parser.go
│   │   └── parser_test.go
│   ├── embedder/            # FastEmbed-Go 래퍼
│   │   └── embedder.go
│   ├── db/                  # SQLite + sqlite-vec CRUD
│   │   ├── db.go
│   │   └── schema.go
│   ├── search/              # 벡터 검색 + 가중치 재정렬
│   │   └── search.go
│   ├── llm/                 # LLM 클라이언트 (Ollama / API)
│   │   └── llm.go
│   └── config/              # 설정 로드/저장
│       └── config.go
└── chat_log/                # 샘플 데이터 (개발용)
```

## 핵심 Go 구조체

```go
// 파싱된 단일 메시지
type Message struct {
    Timestamp time.Time
    Speaker   string
    Content   string
}

// 청킹된 대화 묶음
type Chunk struct {
    ID        int64
    Messages  []Message
    Text      string    // 임베딩용 직렬화 텍스트 (화자: 내용 형식)
    IsLeader  bool
    StartTime time.Time
    EndTime   time.Time
    FileHash  string    // 중복 방지용
}

// 설정 (settings.json에 저장)
type Settings struct {
    LeaderName      string   `json:"leaderName"`
    IgnoreKeywords  []string `json:"ignoreKeywords"`
    LLMProvider     string   `json:"llmProvider"`    // "ollama" | "gemini" | "openai"
    APIKey          string   `json:"apiKey"`
    OllamaModel     string   `json:"ollamaModel"`    // e.g. "gemma3:4b"
}
```

## DB 스키마

```sql
-- 처리된 파일 목록 (중복 업로드 방지)
CREATE TABLE processed_files (
    id        INTEGER PRIMARY KEY,
    file_name TEXT NOT NULL,
    file_hash TEXT NOT NULL UNIQUE,   -- SHA256
    imported_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 청크 텍스트 + 메타데이터
CREATE TABLE chunks (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    text       TEXT NOT NULL,
    is_leader  INTEGER NOT NULL DEFAULT 0,   -- 0 or 1
    start_time DATETIME,
    end_time   DATETIME,
    file_hash  TEXT
);

-- sqlite-vec 가상 테이블 (벡터 저장)
CREATE VIRTUAL TABLE chunk_vectors USING vec0(
    chunk_id INTEGER PRIMARY KEY,
    embedding FLOAT[384]    -- multilingual-e5-small 차원수
);
```

## 아키텍처 및 데이터 흐름

### 업로드 파이프라인

```
채팅 파일 선택 (UI)
    → Go: SHA256 해시 계산 → processed_files 중복 체크
    → Go: 정규식 파싱 → []Message
    → Go: 시간 기반 청킹 → []Chunk
    → FastEmbed-Go: Chunk.Text → []float32 벡터
    → SQLite: chunks 테이블 INSERT
    → sqlite-vec: chunk_vectors 테이블 INSERT
    → Wails Event: "progress" 이벤트로 진행률 전송
```

### 검색 파이프라인

```
자연어 질문 (UI)
    → FastEmbed-Go: 질문 → []float32 벡터
    → sqlite-vec: KNN 쿼리로 상위 10개 후보 검색
    → Go: IsLeader=true 청크에 ×1.5 가중치 후 재정렬 → 상위 3개 선택
    → llm.Client: 프롬프트 + 컨텍스트 전송
    → UI: 요약 답변 + 원본 대화 표시
```

### 채팅 파일 파싱 규칙

파일 형식:
```
저장한 날짜 : 2026. 3. 15. 오후 3:34

2025년 11월 19일 수요일
2025. 11. 19. 오전 10:44, 화자이름 : 메시지 내용
2025. 11. 19. 오전 10:44, 화자이름 : 멀티라인
다음 줄도 같은 메시지
2025. 11. 19. 오전 10:45: 시스템 메시지 (입장/퇴장)
```

파싱 정규식 패턴:
```
^\d{4}\. \d{1,2}\. \d{1,2}\. (오전|오후) \d{1,2}:\d{2}, .+ : .+$   // 일반 메시지
^\d{4}\. \d{1,2}\. \d{1,2}\. (오전|오후) \d{1,2}:\d{2}: .+$        // 시스템 메시지 (무시)
```

파싱 규칙:
- 파일 상단 비규격 라인(헤더)은 첫 메시지 타임스탬프 감지 전까지 무시
- 다음 타임스탬프 패턴이 나올 때까지 현재 메시지에 이어 붙임 (멀티라인)
- 시스템 메시지(콤마 없이 콜론) 무시
- 무시 키워드(예: `사진`)만 있는 메시지는 저장하지 않음

### 청킹 규칙

- **30분 규칙:** 마지막 메시지 이후 30분 이상 경과 → 새 청크
- **5분 규칙:** 리더가 발언을 시작한 후, 리더의 연속 발언이 5분 이상 끊기면 → 새 청크 시작 (리더 발언 세션 종료)
- **IsLeader 플래그:** 청크 내에 `Settings.LeaderName`을 포함하는 Speaker가 있으면 `true`
- 청크 직렬화 텍스트 포맷: `"화자: 내용\n화자: 내용\n..."` (타임스탬프 제외, 임베딩 품질 향상)

### 가중치 적용 방식

sqlite-vec KNN 쿼리는 SQL 내 조건 분기를 지원하지 않으므로, 후처리로 재정렬:
```go
// 상위 10개 후보 검색 후 Go에서 재정렬
for _, r := range candidates {
    score := r.Similarity
    if r.IsLeader {
        score *= 1.5
    }
    r.FinalScore = score
}
sort.Slice(candidates, func(i, j int) bool {
    return candidates[i].FinalScore > candidates[j].FinalScore
})
return candidates[:3]
```

## Wails IPC 설계 원칙

- **모든 무거운 작업은 goroutine으로 실행** — 파싱, 임베딩, DB 저장 모두 비동기
- **진행률은 Wails Events로 전송** — `runtime.EventsEmit(ctx, "progress", payload)`
- **IPC 바인딩 함수는 `app.go`에 집중** — `App` 구조체의 메서드로 노출
- **에러는 Go 반환값으로 처리** — `(result, error)` 패턴 유지, panic 금지

```go
// app.go IPC 바인딩 예시
func (a *App) ImportFiles(paths []string) error { ... }
func (a *App) Search(query string) (SearchResult, error) { ... }
func (a *App) GetSettings() (Settings, error) { ... }
func (a *App) SaveSettings(s Settings) error { ... }
func (a *App) CheckOllama() bool { ... }
```

## 주요 설계 결정 및 주의사항

- **설정 파일 경로:** `os.UserConfigDir()/chatlens/settings.json` (플랫폼별 AppData)
- **DB 파일 경로:** `os.UserConfigDir()/chatlens/chat_data.db`
- **FastEmbed 모델:** `intfloat/multilingual-e5-small` (384차원, ~120MB) — 한국어 지원, 배포 용량 균형
- **LLM 감지 순서:** 앱 시작 시 `localhost:11434/api/tags` 호출 → 성공하면 Ollama 사용, 실패하면 API 키 입력 요구
- **여러 파일 처리:** 파일명 내 날짜 기준 오름차순 정렬 후 순차 처리
- **중복 업로드:** SHA256 해시로 `processed_files` 체크, 이미 있으면 스킵
- **CGO 의존성:** sqlite-vec 사용으로 인해 `CGO_ENABLED=1` 필수. 크로스 컴파일 불가 (각 OS에서 별도 빌드)
- **LLM 프롬프트:** 반드시 한국어로 작성. 시스템 프롬프트에 "한국어로 답변하라" 명시
