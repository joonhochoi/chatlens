# ChatLens

카카오톡 채팅 기록을 로컬에서 분석하여 자연어로 검색하는 데스크톱 RAG 애플리케이션.

---

## 목차

1. [프로젝트 개요](#1-프로젝트-개요)
2. [주요 기능](#2-주요-기능)
3. [기술 스택](#3-기술-스택)
4. [아키텍처 및 데이터 흐름](#4-아키텍처-및-데이터-흐름)
5. [기술 상세 설명](#5-기술-상세-설명)
6. [설정 항목](#6-설정-항목)
7. [디렉토리 구조](#7-디렉토리-구조)
8. [DB 스키마](#8-db-스키마)
9. [배포 구조](#9-배포-구조)
10. [개발 환경 구성](#10-개발-환경-구성)
11. [빌드](#11-빌드)
12. [개발 명령어](#12-개발-명령어)
13. [설계 결정 기록](#13-설계-결정-기록)

---

## 1. 프로젝트 개요

ChatLens는 수개월치 카카오톡 단체 채팅 기록(`.txt` 내보내기 파일)을 로컬 PC에서 완전히 처리하여, 자연어 질문으로 대화 내용을 검색·요약할 수 있는 **Windows 데스크톱 RAG(Retrieval-Augmented Generation) 애플리케이션**입니다.

### 핵심 가치

- **100% 로컬 데이터 처리** — 채팅 내용이 외부 서버로 전송되지 않습니다. 임베딩 모델이 실행 파일과 함께 배포되어 인터넷 없이 동작합니다.
- **LLM 유연성** — 로컬 Ollama부터 Gemini, OpenAI까지 동일한 인터페이스로 교체 가능합니다.
- **리더(특정 화자) 중심 검색** — 단체 채팅에서 타겟 화자의 발언에 가중치를 부여해 검색 정확도를 높입니다.
- **단일 폴더 배포** — `chatlens.exe` + `onnx/` 폴더만 있으면 별도 설치 없이 실행됩니다.

---

## 2. 주요 기능

### 채팅 파일 임포트

- 카카오톡 내보내기 `.txt` 파일을 여러 개 동시에 선택해 처리
- SHA256 해시 기반 파일 중복 체크 + `(타임스탬프, 화자, 내용)` 해시 기반 **메시지 수준 중복 제거**
- 기간이 겹치는 파일을 추가해도 중복 없이 신규 메시지만 임베딩
- 실시간 진행률 표시 (파일별 파싱 → 중복 확인 → 임베딩 → DB 저장)

### 🤖 AI 검색 (RAG)

- 한국어 질문을 임베딩하여 벡터 유사도 기반으로 관련 대화 청크 검색
- 리더 발언 청크에 ×1.5 점수 가중치 적용 후 상위 N개 선택 (N은 설정 가능, 3~8개)
- LLM이 선택된 청크를 컨텍스트로 활용해 자연어 요약 답변 생성
- 검색 결과에서 "💬 기록 보기" 버튼으로 해당 날짜·시간의 원문 대화로 바로 이동

### 🔍 키워드 검색

- messages 테이블에 대한 SQL `LIKE` 검색으로 원문 메시지를 직접 조회
- 최신 메시지 우선, 10개씩 "더 보기" 페이지네이션
- 일치 단어 amber 색상 하이라이트, 총 매칭 수 표시
- 리더 발언 구분 + "💬 기록 보기" 버튼

### 채팅 기록 뷰어

- 날짜별 전체 대화 기록 탐색 (연/월 드롭다운 + 일자 버튼 선택)
- 선택된 날짜를 "2025년 11월 1일 (토)" 형태로 메시지 수와 함께 표시
- AI 검색 / 키워드 검색의 "💬 기록 보기"로 해당 시간 메시지로 자동 스크롤 + 보라색 하이라이트
- 리더 닉네임을 amber 색상으로 강조 표시

### 데이터 관리

- 전체 데이터 삭제 (설정 변경 후 재임포트 시 활용)
- 처리된 파일 목록 표시

### 설정

- 리더(타겟 화자) 이름 설정
- 무시할 키워드 설정 (사진, 동영상 등)
- 청킹 전략 세부 조정 (최대 크기, 오버랩, 리더 마이크로 청킹, 의미 기반 분할)
- 검색 결과 수 조정
- LLM 제공자 및 모델 설정

---

## 3. 기술 스택

| 레이어 | 기술 | 버전 | 역할 |
|--------|------|------|------|
| **데스크톱 프레임워크** | [Wails v2](https://wails.io) | v2.11.0 | Go 백엔드 + OS 기본 WebView2 브리지 |
| **백엔드** | Go | 1.23 | 파싱, 임베딩, 검색, IPC |
| **프론트엔드** | React + TypeScript | 18 / 5 | UI 컴포넌트 |
| **로컬 임베딩** | ONNX Runtime | 1.17.0 | `intfloat/multilingual-e5-small` INT8 추론 |
| **임베딩 모델** | multilingual-e5-small (INT8 ONNX) | — | 384차원, 100개 언어, 한국어 지원 |
| **토크나이저** | 자체 구현 (Unigram/SentencePiece) | — | 표준 Go 라이브러리 미지원으로 직접 구현 |
| **벡터 DB** | SQLite + [sqlite-vec](https://github.com/asg017/sqlite-vec) | v0.1.6 | 임베딩 저장 및 KNN 검색 |
| **LLM** | Ollama / Gemini / OpenAI | — | 검색 결과 요약 (`gemini-3.1-flash-lite-preview` 기본) |
| **설정 저장** | JSON 파일 | — | `settings.json` (실행 파일 옆) |

### 주요 Go 의존성

```
github.com/asg017/sqlite-vec-go-bindings  — sqlite-vec CGO 바인딩
github.com/mattn/go-sqlite3               — SQLite CGO 드라이버
github.com/wailsapp/wails/v2              — Wails 프레임워크
github.com/yalue/onnxruntime_go           — ONNX Runtime Go 바인딩
```

> **CGO 필수**: `sqlite-vec`와 `go-sqlite3` 모두 C 확장이므로 `CGO_ENABLED=1`과 GCC(MinGW-w64) 없이는 빌드되지 않습니다.

---

## 4. 아키텍처 및 데이터 흐름

### 전체 구조

```
┌─────────────────────────────────────────────────────────────┐
│                        chatlens.exe                         │
│                                                             │
│  ┌────────────────┐     Wails IPC      ┌────────────────┐  │
│  │  React + TS    │ ←────────────────→ │   Go Backend   │  │
│  │  (WebView2)    │   JS ↔ Go 바인딩   │   (app.go)     │  │
│  └────────────────┘                    └───────┬────────┘  │
│                                                │            │
│          ┌─────────────────────────────────────┤            │
│          ↓                   ↓                 ↓            │
│  ┌───────────────┐  ┌──────────────┐  ┌──────────────┐    │
│  │parser package │  │  embedder    │  │  db package  │    │
│  │(파싱 + 청킹)  │  │  (ONNX)      │  │(SQLite+vec)  │    │
│  └───────────────┘  └──────────────┘  └──────────────┘    │
│                                                             │
│  ┌───────────────┐  ┌──────────────┐                       │
│  │search package │  │ llm package  │                       │
│  │(KNN + 재정렬) │  │(Ollama 등)   │                       │
│  └───────────────┘  └──────────────┘                       │
└─────────────────────────────────────────────────────────────┘
        ↕                      ↕
  onnx/ (모델 파일)    settings.json / chat_data.db
```

### 업로드 파이프라인

```
사용자가 .txt 파일 선택
        │
        ▼
① SHA256 해시 계산
   → processed_files 테이블에서 파일 단위 중복 체크
        │
        ▼
② ParseFile()
   → 정규식으로 메시지 파싱 (타임스탬프, 화자, 내용)
   → 멀티라인, 시스템 메시지, 무시 키워드 처리
        │
        ▼
③ FilterNewMessages()
   → (타임스탬프|화자|내용) SHA256 해시로 메시지 수준 중복 제거
   → 이미 처리된 메시지 제외
        │
        ▼
④ Chunkify()  — 4단계 청킹 파이프라인
   ├─ Phase 1: 30분 갭 / 리더 5분 갭 / 최대 크기 분할 [항상]
   ├─ Phase 2: 리더 발언 런(run) 단위 마이크로 청킹   [설정 시]
   ├─ Phase 3: 윈도우 임베딩 기반 의미 분할           [설정 시]
   └─ Phase 4: 인접 청크 오버랩 적용                  [설정 시]
        │
        ▼
⑤ EmbedBatch()
   → 각 청크 텍스트 → 384차원 float32 벡터
   → multilingual-e5-small, "passage: " prefix 사용
        │
        ▼
⑥ InsertChunks()  — 단일 SQLite 트랜잭션
   ├─ chunks 테이블 (텍스트, IsLeader, 시간 범위)
   ├─ chunk_vectors 테이블 (임베딩 벡터)
   ├─ message_hashes 테이블 (중복 방지 해시)
   └─ messages 테이블 (개별 메시지 — 채팅 기록 뷰어용)

→ Wails Events: "import:progress" / "import:done" / "import:error"
```

### 키워드 검색 파이프라인

```
사용자 키워드 입력
        │
        ▼
① SearchMessages(keyword, offset)
   → messages 테이블: WHERE content LIKE '%keyword%'
   → ORDER BY timestamp DESC
   → LIMIT 10 OFFSET offset
        │
        ▼
② 총 매칭 수 COUNT(*) 쿼리 (같은 조건)
        │
        ▼
③ KeywordResult 반환
   → { hits: KeywordHit[], total: int, hasMore: bool }
   → 각 hit: { time, date, dateKey, speaker, content }
        │
        ▼
④ UI: 키워드 부분 amber 하이라이트 + "💬 기록 보기" 버튼
   → 더 보기: offset += 10 로 누적 로드
```

### AI 검색 파이프라인 (RAG)

```
사용자 자연어 질문
        │
        ▼
① 설정 로드 (SearchTopK, LLM 설정 등)
        │
        ▼
② EmbedQuery()
   → 질문 → 384차원 float32 벡터
   → "query: " prefix로 비대칭 임베딩
        │
        ▼
③ sqlite-vec KNN
   → vec_distance_cosine으로 상위 (TopK × 3)개 후보 검색
        │
        ▼
④ Go 레이어 재정렬
   → IsLeader=true 청크에 ×1.5 점수 부여
   → 내림차순 정렬 후 상위 TopK개 선택
        │
        ▼
⑤ LLM Chat()
   → 시스템 프롬프트 + 선택된 청크 컨텍스트 + 사용자 질문
   → 한국어 요약 답변 생성
        │
        ▼
⑥ SearchResult 반환
   → { summary: string, sources: SourceChunk[] }
```

---

## 5. 기술 상세 설명

### 5.1 카카오톡 채팅 파싱

두 가지 내보내기 포맷을 모두 지원하며, 동일한 파싱 루프에서 자동 감지합니다.

#### 포맷 A — 모바일(앱) 내보내기

```
저장한 날짜 : 2026. 3. 15. 오후 3:34

2025년 11월 19일 수요일
2025. 11. 19. 오전 10:44, 화자이름 : 메시지 내용
2025. 11. 19. 오전 10:44, 화자이름 : 멀티라인 메시지
다음 줄도 같은 메시지
2025. 11. 19. 오전 10:45: 시스템 메시지 (입장/퇴장)
```

```go
// 일반 메시지: 콤마(,)로 타임스탬프와 화자 구분
msgPattern = `^(\d{4})\. (\d{1,2})\. (\d{1,2})\. (오전|오후) (\d{1,2}):(\d{2}), (.+?) : (.+)$`

// 시스템 메시지: 콤마 없이 콜론
sysPattern = `^\d{4}\. \d{1,2}\. \d{1,2}\. (오전|오후) \d{1,2}:\d{2}: .+$`
```

#### 포맷 B — PC(카카오톡 PC 앱) 내보내기

```
[소통방] 채상욱의 머니버스 회원전용 소통방 님과 카카오톡 대화
저장한 날짜 : 2026-03-17 19:51:00

--------------- 2025년 11월 1일 토요일 ---------------
[윤쭈쭈] [오전 10:16] https://youtu.be/...
[채상욱 리더] [오전 11:10] 옹 어서오세용
착한 물고기 742810님이 들어왔습니다.
메시지가 삭제되었습니다.
```

```go
// 날짜 구분선: 날짜 컨텍스트 업데이트 (연/월/일 추출)
pcDatePattern = `^-+\s+(\d{4})년\s+(\d{1,2})월\s+(\d{1,2})일`

// 일반 메시지: [닉네임] [오전/오후 HH:MM] 내용
pcMsgPattern = `^\[(.+?)\] \[(오전|오후) (\d{1,2}):(\d{2})\] (.*)$`

// 시스템 메시지: 입장/퇴장/삭제
pcSysPattern = `님이 (들어왔습니다|나갔습니다)\.$|^메시지가 삭제되었습니다\.$`
```

PC 포맷은 날짜 구분선에서 연/월/일을 추적하고, 개별 메시지에서 시간만 결합하여 `time.Time`을 구성합니다.

**공통 처리 규칙:**

| 라인 유형 | 처리 방식 |
|-----------|---------|
| 일반 메시지 (A/B) | `Message{Timestamp, Speaker, Content}` 구조체 생성 |
| 멀티라인 | 다음 메시지 패턴 전까지 `Content`에 `\n`으로 이어붙임 |
| 시스템 메시지 | 현재 메시지 플러시 후 무시 |
| 파일 헤더 | 첫 메시지 이전 모든 줄 무시 |
| 무시 키워드 | 내용이 해당 키워드 단독인 메시지 버림 |

**오전/오후 12시 엣지 케이스:**
- `오전 12:xx` → 0시 (자정)
- `오후 12:xx` → 12시 (정오)

### 5.2 청킹 전략

청킹은 파싱된 메시지 배열을 의미 있는 묶음(`Chunk`)으로 나누는 핵심 단계입니다. 384차원 벡터 하나가 청크 전체를 표현하므로, 청크 품질이 검색 정확도에 직결됩니다.

```go
func Chunkify(messages []Message, opts ChunkOptions) []Chunk
```

4단계 파이프라인으로 동작합니다:

#### Phase 1: 시간 기반 + 최대 크기 분할 (항상 적용)

```
규칙 A (30분 갭):  연속 메시지 사이 간격 ≥ 30분 → 새 청크
규칙 B (5분 리더): 리더 발언이 있는 청크에서 간격 ≥ 5분 → 리더 세션 종료
규칙 C (최대 크기): 현재 청크 메시지 수 ≥ MaxMessages → 강제 분할
```

규칙 C는 리더가 짧은 간격으로 오랜 시간 발언할 때 하나의 청크에 너무 많은 내용이 쌓이는 문제를 방지합니다.

#### Phase 2: 리더 마이크로 청킹 (선택)

리더의 발언 "런(run)" — `leaderRunGap(5분)` 미만 간격으로 이어지는 리더 메시지 묶음 — 경계를 찾아 추가 분할합니다. 두 리더 런 사이의 중간점을 분할 위치로 사용합니다.

```
[참가자들의 대화]  ─ 일반 청크
     ↓ 분할 경계
[리더: 발언 A-1]
[리더: 발언 A-2]  ─ 리더 런 A 청크
     ↓ 분할 경계 (5분 이상 간격)
[리더: 발언 B-1]
[리더: 발언 B-2]  ─ 리더 런 B 청크
```

#### Phase 3: 의미 기반 추가 분할 (선택)

5개 메시지씩 슬라이딩 윈도우를 만들어 각각 임베딩하고, 인접 윈도우 간 코사인 유사도가 임계값 미만이면 주제 전환으로 판단해 분할합니다.

```
[메시지 1~5]  → 벡터 A
[메시지 6~10] → 벡터 B
cosine_similarity(A, B) = 0.83  → 같은 주제, 유지

[메시지 11~15] → 벡터 C
cosine_similarity(B, C) = 0.41  → 주제 전환! → 경계 삽입
```

이 단계는 임포트 시 청킹 자체에 임베딩 연산이 추가로 발생하므로 속도가 느려집니다.

#### Phase 4: 청크 오버랩 (선택)

청크 경계에서 이전 청크의 마지막 N개 메시지를 다음 청크 앞에 복사합니다.

```
청크 i:    [m1  m2  m3  m4  m5]
청크 i+1:  [m4  m5  m6  m7  m8]   ← overlap=2 적용
```

경계 근처 대화가 양쪽 청크에서 검색될 수 있어, 분할 위치가 부정확해도 정보 손실을 줄입니다.

**청크 직렬화 포맷:**

```
화자이름: 메시지 내용
화자이름: 메시지 내용
...
```

타임스탬프는 의도적으로 제외합니다 — 임베딩이 시간 정보보다 의미 내용에 집중하도록 합니다. 시간 정보는 DB에 `start_time`, `end_time`으로 별도 저장합니다.

### 5.3 로컬 임베딩 (ONNX)

#### 모델 선택 과정

| 검토 모델 | 결과 |
|-----------|------|
| fastembed-go (anush008) | ❌ Unigram 토크나이저 미지원, 저장소 archived |
| sugarme/tokenizer | ❌ `createUnigram` 호출 시 `NotImplementedError` panic |
| LaBSE ONNX quantized | ❌ 450MB로 배포 크기 과대 |
| **multilingual-e5-small INT8** | ✅ 113MB, 한국어 포함 100개 언어, 384차원 → 채택 |

#### Unigram 토크나이저 자체 구현

`multilingual-e5-small`은 XLM-RoBERTa 기반으로 **SentencePiece Unigram** 방식을 사용합니다. Go 생태계에 이를 지원하는 라이브러리가 없어 직접 구현했습니다 (`internal/embedder/tokenizer.go`).

구현 내용:
1. HuggingFace `tokenizer.json` 파싱 (250,002개 vocab 항목 로드)
2. **Metaspace 사전 토크나이징**: 공백 → `▁` 변환 후 공백 기준 분리
3. **Viterbi 알고리즘**: 각 단어를 최적 서브워드 시퀀스로 분할 (로그 확률 최대화)
4. 특수 토큰 삽입: `<s>`(CLS=0) + 토큰들 + `</s>`(SEP=2)
5. MaxLength=512 트런케이션 / `<pad>`(1) 패딩

#### ONNX Runtime 추론 파이프라인

```
텍스트 입력 (+ "passage:" 또는 "query:" prefix)
        │
        ▼
unigramTokenizer.Encode()
  → input_ids      [1, 512]  int64
  → attention_mask [1, 512]  int64   (패딩 위치 = 0)
  → token_type_ids [1, 512]  int64   (XLM-RoBERTa: 전부 0)
        │
        ▼
ort.DynamicAdvancedSession.Run()   (ONNX Runtime v1.17.0)
  → last_hidden_state [1, 512, 384]  float32
        │
        ▼
meanPool()
  → attention_mask 가중 평균 (패딩 토큰 제외)
  → [384]  float32
        │
        ▼
l2Normalize()
  → L2 정규화 (코사인 유사도 = 내적)
  → [384]  float32  (단위 벡터)
```

**비대칭 임베딩 (Asymmetric Embedding):**

`multilingual-e5` 모델은 학습 시 문서와 쿼리에 서로 다른 prefix를 사용했습니다:

```go
func (e *Embedder) EmbedPassage(text string) ([]float32, error) {
    return e.embed("passage: " + text)  // 청크 저장 시
}
func (e *Embedder) EmbedQuery(query string) ([]float32, error) {
    return e.embed("query: " + query)   // 검색 시
}
```

동일한 prefix를 쓰면 성능이 저하됩니다.

#### 배포 파일 구성

```
onnx/
├── model.onnx       (113 MB) — INT8 양자화 ONNX 모델
├── tokenizer.json   ( 17 MB) — HuggingFace 토크나이저 설정 (vocab 포함)
└── onnxruntime.dll  ( 11 MB) — Microsoft ONNX Runtime 1.17.0 (Windows x64)
```

실행 경로 해석: `os.Executable()` → `filepath.Dir(exe)` + `/onnx/`

### 5.4 벡터 데이터베이스 (sqlite-vec)

표준 SQLite에 `sqlite-vec` C 확장을 로드하여 벡터 연산을 추가합니다.

```go
func init() {
    sqlite_vec.Auto()  // sqlite-vec 확장 자동 등록
}
```

벡터는 little-endian `float32` 바이트 배열로 직렬화해 BLOB으로 저장합니다:

```go
func float32SliceToBytes(vec []float32) []byte {
    buf := make([]byte, len(vec)*4)
    for i, v := range vec {
        binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
    }
    return buf
}
```

**CGO 빌드 요구사항 (Windows):**

```bash
CGO_ENABLED=1
CGO_CFLAGS="-I/c/msys64/mingw64/include"  # sqlite3.h 위치
GCC: MinGW-w64 (pacman -S mingw-w64-x86_64-sqlite3)
```

### 5.5 벡터 검색 및 재정렬

**KNN 쿼리 (sqlite-vec):**

```sql
SELECT c.id, c.text, c.is_leader, c.start_time,
       (1.0 - vec_distance_cosine(v.embedding, ?)) AS similarity
FROM chunk_vectors v
JOIN chunks c ON c.id = v.chunk_id
ORDER BY vec_distance_cosine(v.embedding, ?) ASC
LIMIT ?   -- TopK × 3개 후보 확보
```

`vec_distance_cosine(a, b) = 1 - cosine_similarity(a, b)` 이므로 `1.0 - distance`로 유사도(0~1)를 구합니다. SQL은 벡터 정렬만 담당하고, 조건부 가중치는 sqlite-vec 내에서 지원되지 않아 Go에서 처리합니다.

**Go 레이어 재정렬:**

```go
const leaderBoost = 1.5

for i := range candidates {
    score := candidates[i].Similarity
    if candidates[i].IsLeader {
        score *= leaderBoost  // 리더 발언 50% 보너스
    }
    candidates[i].FinalScore = score
}
sort.Slice(candidates, func(i, j int) bool {
    return candidates[i].FinalScore > candidates[j].FinalScore
})
return candidates[:topK]
```

### 5.6 LLM 통합

`llm.Client` 인터페이스 하나로 세 제공자를 통일합니다:

```go
type Client interface {
    Chat(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}
```

| 제공자 | 엔드포인트 | 기본 모델 |
|--------|-----------|---------|
| **Ollama** | `http://localhost:11434/api/chat` | `llama3:latest` (설정 가능) |
| **Gemini** | `generativelanguage.googleapis.com/v1beta/models/gemini-3.1-flash-lite-preview` | `gemini-3.1-flash-lite-preview` |
| **OpenAI** | `https://api.openai.com/v1/chat/completions` | `gpt-4o-mini` |

**시스템 프롬프트:**

```
당신은 채팅 기록 분석 전문가입니다.
제공된 대화 내용을 바탕으로 사용자의 질문에 한국어로 답변하세요.
대화에 없는 내용은 추측하지 말고, 대화에서 찾은 내용만 답변하세요.
답변은 간결하고 명확하게 작성하세요.
```

**사용자 프롬프트 구성:**

```
대화 기록:
--- 대화 1 (2025-11-19 10:44) ---
화자: 내용
화자: 내용
...

질문: 사용자 질문 내용
```

### 5.7 Wails IPC 설계

Wails v2는 `app.go`의 `App` 구조체 메서드를 JavaScript 함수로 자동 노출합니다.

**IPC 바인딩 함수:**

```go
// 비동기 (goroutine 실행, Events로 진행상황 전달)
func (a *App) ImportFiles(paths []string) error

// 검색
func (a *App) Search(query string) (SearchResult, error)

// 채팅 기록 뷰어
func (a *App) GetChatDates() ([]string, error)               // ["2025-11-01", ...]
func (a *App) GetMessagesByDate(date string) ([]ChatMessage, error)

// 키워드 검색
func (a *App) SearchKeyword(keyword string, offset int) (KeywordResult, error)

// 설정
func (a *App) GetSettings() (config.Settings, error)
func (a *App) SaveSettings(s config.Settings) error
func (a *App) CheckOllama() bool

// 파일 관리
func (a *App) GetImportedFiles() ([]string, error)
func (a *App) DeleteAllData() error
func (a *App) OpenFileDialog() ([]string, error)
```

**비동기 진행률 패턴:**

```go
// Go → JavaScript 이벤트
runtime.EventsEmit(ctx, "import:progress", map[string]any{
    "file":    "파일명.txt",
    "percent": 42,
    "status":  "임베딩 중... (21 / 50)",
})
runtime.EventsEmit(ctx, "import:done",  map[string]any{"total": 1234})
runtime.EventsEmit(ctx, "import:error", map[string]any{"error": "..."})
```

```typescript
// React에서 수신
useEffect(() => {
    EventsOn('import:progress', (data: ProgressPayload) => { ... });
    EventsOn('import:done',     (data: DonePayload)     => { ... });
    EventsOn('import:error',    (data: { error: string }) => { ... });
    return () => { EventsOff('import:progress'); ... };
}, []);
```

**파일 및 DB 경로:**

모든 데이터 파일은 실행 파일 옆에 위치합니다:

```go
func configDir() (string, error) {
    exe, _ := os.Executable()
    return filepath.Dir(exe), nil
}
// settings.json → exe 옆
// chat_data.db  → exe 옆
```

---

## 6. 설정 항목

`settings.json`에 저장되며, 설정 화면에서 관리합니다.

| 항목 | 키 | 기본값 | 설명 |
|------|-----|--------|------|
| 리더 이름 | `leaderName` | `""` | 타겟 화자의 채팅 표시명 (`strings.Contains` 부분 매칭) |
| 무시 키워드 | `ignoreKeywords` | `["사진","동영상","이모티콘"]` | 해당 내용만 있는 메시지 무시 |
| LLM 제공자 | `llmProvider` | `"ollama"` | `"ollama"` / `"gemini"` / `"openai"` |
| API 키 | `apiKey` | `""` | Gemini/OpenAI 용 |
| Ollama 모델 | `ollamaModel` | `"llama3:latest"` | Ollama 사용 시 모델명 |
| 검색 결과 수 | `searchTopK` | `5` | LLM에 전달할 청크 수 (3~8) |
| **최대 청크 크기** | `maxChunkMessages` | `40` | 청크당 최대 메시지 수 |
| **청크 오버랩** | `chunkOverlap` | `3` | 인접 청크 간 공유 메시지 수 (0=없음) |
| **리더 마이크로 청킹** | `useLeaderMicro` | `false` | 리더 발언 런별 분리 (리더 이름 설정 필요) |
| **의미 기반 분할** | `useSemanticChunk` | `false` | 주제 전환 감지 추가 분할 |
| **유사도 임계값** | `semanticThreshold` | `0.65` | 의미 분할 코사인 유사도 기준 (낮을수록 더 자주 분할) |

> ⚠️ **청킹 전략 변경 시**: 이미 저장된 데이터는 이전 전략으로 처리되었습니다. 업로드 화면의 "전체 데이터 삭제" 후 재임포트해야 적용됩니다.

---

## 7. 디렉토리 구조

```
chatlens/
├── main.go                          # Wails 앱 진입점
├── app.go                           # IPC 바인딩 (App 구조체 메서드)
├── wails.json                       # Wails 빌드 설정
├── go.mod / go.sum
│
├── frontend/                        # React + TypeScript UI
│   ├── src/
│   │   ├── App.tsx                  # 루트 컴포넌트, 페이지 라우팅, 글로벌 에러 배너
│   │   ├── App.css                  # 전역 스타일 (다크 테마)
│   │   └── pages/
│   │       ├── UploadPage.tsx       # 파일 업로드, 임포트, 전체 삭제
│   │       ├── SearchPage.tsx       # AI(RAG) 자연어 검색, 결과 표시, 채팅 기록 바로가기
│   │       ├── KeywordPage.tsx      # SQL LIKE 키워드 검색, 10개씩 페이지네이션
│   │       ├── ChatPage.tsx         # 날짜별 채팅 기록 뷰어
│   │       └── SettingsPage.tsx     # 모든 설정 관리
│   └── wailsjs/                     # 자동 생성 Go↔JS 바인딩
│       └── go/
│           ├── models.ts            # Go 구조체 → TypeScript 타입
│           └── main/
│               ├── App.js           # IPC 함수 구현
│               └── App.d.ts         # TypeScript 타입 선언
│
├── internal/
│   ├── parser/
│   │   ├── parser.go                # 카카오톡 .txt 파싱 (정규식, 멀티라인, 오전/오후)
│   │   ├── chunker.go               # 4단계 청킹 파이프라인 (ChunkOptions)
│   │   └── parser_test.go           # 파싱 + 청킹 단위 테스트 (12개)
│   │
│   ├── embedder/
│   │   ├── embedder.go              # ONNX 추론, 평균 풀링, L2 정규화
│   │   └── tokenizer.go             # Unigram/SentencePiece 자체 구현
│   │
│   ├── db/
│   │   ├── schema.go                # SQLite 스키마 정의 (5개 테이블)
│   │   ├── db.go                    # CRUD, 메시지 중복 제거, 채팅 기록 조회, 전체 삭제
│   │   └── db_test.go
│   │
│   ├── search/
│   │   ├── search.go                # KNN 쿼리 + 리더 가중치 재정렬
│   │   └── search_test.go           # 통합 테스트 (ONNX 파일 필요)
│   │
│   ├── llm/
│   │   └── llm.go                   # Ollama / Gemini / OpenAI 클라이언트 통합
│   │
│   └── config/
│       └── config.go                # 설정 로드/저장, 파일 경로 관리
│
└── onnx/                            # 임베딩 모델 파일 (빌드 시 배포 폴더에 복사)
    ├── model.onnx                   # multilingual-e5-small INT8 (113 MB)
    ├── tokenizer.json               # 토크나이저 설정 (17 MB)
    └── onnxruntime.dll              # ONNX Runtime 1.17.0 (11 MB)
```

---

## 8. DB 스키마

```sql
-- 임포트된 파일 목록 (파일 단위 중복 방지)
CREATE TABLE IF NOT EXISTS processed_files (
    id          INTEGER PRIMARY KEY,
    file_name   TEXT NOT NULL,
    file_hash   TEXT NOT NULL UNIQUE,  -- SHA256 (파일 전체)
    imported_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 대화 청크 텍스트 및 메타데이터
CREATE TABLE IF NOT EXISTS chunks (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    text        TEXT NOT NULL,              -- "화자: 내용\n..." 직렬화
    is_leader   INTEGER NOT NULL DEFAULT 0, -- 1: 리더 발언 포함
    start_time  DATETIME,                   -- 청크 첫 메시지 시각
    end_time    DATETIME,                   -- 청크 마지막 메시지 시각
    file_hash   TEXT                        -- processed_files 참조
);

-- sqlite-vec 벡터 저장소 (384차원 float32)
CREATE VIRTUAL TABLE IF NOT EXISTS chunk_vectors USING vec0(
    chunk_id  INTEGER PRIMARY KEY,
    embedding FLOAT[384]
);

-- 메시지 수준 중복 방지
-- (기간 겹치는 다른 파일 추가 시에도 동일 메시지 재임베딩 방지)
CREATE TABLE IF NOT EXISTS message_hashes (
    msg_hash TEXT PRIMARY KEY  -- SHA256(RFC3339타임스탬프|화자|내용)
);

-- 개별 메시지 저장 (채팅 기록 뷰어용)
-- 임포트 시 청크와 동일 트랜잭션에서 저장
CREATE TABLE IF NOT EXISTS messages (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME NOT NULL,
    speaker   TEXT NOT NULL,
    content   TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_messages_date ON messages(date(timestamp));
```

---

## 9. 배포 구조

```
배포 폴더/
├── chatlens.exe         (~ 13 MB)   -- Go 백엔드 + React UI 내장
└── onnx/
    ├── model.onnx       (~113 MB)   -- multilingual-e5-small INT8
    ├── tokenizer.json   (~ 17 MB)   -- Unigram 토크나이저
    └── onnxruntime.dll  (~ 11 MB)   -- ONNX Runtime 1.17.0

총 배포 크기: ~154 MB
```

최초 실행 후 자동 생성되는 파일:

```
배포 폴더/
├── settings.json    -- 사용자 설정
└── chat_data.db     -- SQLite DB (청크 텍스트 + 임베딩 벡터)
```

> `chat_data.db`는 청크 수에 따라 수백 MB까지 커질 수 있습니다. 384차원 float32 벡터 하나가 1.5 KB이며, 청크 1만 개 기준 약 15~50 MB입니다.

---

## 10. 개발 환경 구성

### 필수 도구

```bash
# Go 1.21 이상
https://go.dev/dl/

# Node.js 18 이상
https://nodejs.org/

# Wails CLI v2
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# MinGW-w64 (Windows, CGO 컴파일러)
choco install mingw
# 또는
scoop install gcc

# SQLite 헤더 파일 (MSYS2 사용 시)
pacman -S mingw-w64-x86_64-sqlite3
```

### ONNX 모델 파일 준비

`onnx/` 디렉토리에 아래 파일을 배치합니다:

| 파일 | 출처 |
|------|------|
| `model.onnx` | Hugging Face: `intfloat/multilingual-e5-small` → ONNX 변환 (INT8 권장) |
| `tokenizer.json` | 동일 모델 페이지의 `tokenizer.json` |
| `onnxruntime.dll` | [Microsoft ONNX Runtime v1.17.0](https://github.com/microsoft/onnxruntime/releases/tag/v1.17.0) Windows x64 |

---

## 11. 빌드

> 모든 명령어는 **프로젝트 루트 (`chatlens/`)에서 실행**합니다.

```bash
# PATH 설정 (MinGW + Go + Node 포함)
export PATH="/c/msys64/mingw64/bin:/c/Go/bin:/c/Users/$USER/go/bin:/c/Program Files/nodejs:$PATH"

# 프로덕션 빌드
CGO_ENABLED=1 CGO_CFLAGS="-I/c/msys64/mingw64/include" wails build

# ONNX 파일 배포 폴더에 복사
mkdir -p build/bin/onnx
cp onnx/* build/bin/onnx/
```

빌드 결과물: `build/bin/chatlens.exe` + `build/bin/onnx/`

---

## 12. 개발 명령어

```bash
# 개발 서버 (핫 리로드)
CGO_ENABLED=1 CGO_CFLAGS="-I/c/msys64/mingw64/include" wails dev

# Go 전체 테스트
go test ./internal/...

# 파서 테스트만 (CGO 불필요, 빠름)
go test ./internal/parser/... -v

# 검색 통합 테스트 (onnx/ 파일 필요, ONNX 추론 포함)
go test ./internal/search/... -v -run TestSearchPipeline

# TypeScript 타입 체크
cd frontend && ./node_modules/.bin/tsc --noEmit

# 프론트엔드 개발 서버 (단독)
cd frontend && npm run dev
```

---

## 13. 설계 결정 기록

### 왜 fastembed-go 대신 ONNX Runtime을 직접 사용했나?

`anush008/fastembed-go`는 2024년에 archived 되었으며, 다국어 지원 모델(XLM-RoBERTa 계열)이 사용하는 Unigram/SentencePiece 토크나이저를 지원하지 않습니다 — 코드에 명시적으로 "Unigram not supported"라고 주석처리되어 있습니다. `sugarme/tokenizer`도 `createUnigram` 호출 시 `NotImplementedError` panic이 발생합니다.

한국어를 제대로 지원하는 로컬 임베딩을 위해 Viterbi Unigram 토크나이저를 직접 구현하고 `yalue/onnxruntime_go`로 ONNX 추론을 연결했습니다.

### 왜 sqlite-vec인가?

- 기존 SQLite와 동일한 파일 하나에 텍스트와 벡터를 함께 저장
- 별도 벡터 DB 프로세스(Qdrant, Chroma, Weaviate 등) 불필요 → 단일 폴더 배포 유지
- CGO 바인딩(`sqlite-vec-go-bindings`)이 공식 지원됨
- 소규모~중규모 채팅 데이터에 충분한 성능 (수만 청크 기준 검색 < 100ms)

### 왜 IsLeader 가중치를 Go에서 후처리로 적용하나?

`sqlite-vec`의 KNN 쿼리는 단순 벡터 거리 기반 정렬만 지원합니다. SQL 내에서 `is_leader` 컬럼에 따른 조건부 가중치 ORDER BY가 불가능하기 때문에, 3배 많은 후보(TopK × 3)를 가져온 뒤 Go에서 재정렬합니다.

### 왜 청킹이 중요한가?

384차원 벡터 하나가 청크 전체를 표현해야 합니다. 청크가 너무 크면(여러 주제가 섞이면) 어느 주제도 제대로 표현되지 않아 검색 정확도가 급락합니다. 시간 기반 분할만으로는 리더가 30분 미만 간격으로 1시간 이야기할 경우 모든 내용이 하나의 청크에 들어갑니다. 최대 크기 제한과 리더 마이크로 청킹이 이 문제를 해결합니다.

### IsLeader 플래그는 언제 설정되나?

임포트 시점에 `settings.LeaderName`을 읽어 청킹 단계에서 결정되고 DB에 저장됩니다. 따라서 **리더 이름을 먼저 설정한 후 임포트**해야 검색 가중치가 정상 동작합니다. 설정 변경 후에는 전체 데이터 삭제 + 재임포트가 필요합니다.

### 왜 채팅 기록 뷰어에 청크 대신 개별 메시지를 사용하나?

청크는 여러 메시지를 하나의 텍스트 블록으로 직렬화한 것이라 개별 타임스탬프 정보가 소실됩니다. 날짜별 전체 대화를 시간 순으로 보려면 원래 메시지 단위가 필요합니다. `messages` 테이블은 임포트 트랜잭션 안에서 청크/해시와 함께 저장되므로 추가 비용 없이 개별 메시지 조회가 가능합니다.

### 왜 모바일/PC 포맷을 별도 함수 대신 같은 루프에서 처리하나?

포맷 감지를 위한 전처리 패스를 추가하지 않아도 두 패턴이 서로 겹치지 않으므로 한 루프에서 순서대로 시도해도 안전합니다. 실제로는 파일 하나가 두 포맷을 동시에 사용하는 경우가 없지만, 단일 루프 구조는 코드가 단순하고 이론적으로 혼합 포맷도 처리 가능합니다.

### 왜 키워드 검색은 chunks 대신 messages 테이블을 사용하나?

청크는 여러 메시지를 하나의 텍스트 블록으로 합친 것이라 특정 단어가 누가 언제 발언했는지 추적하기 어렵습니다. `messages` 테이블은 메시지 단위로 `timestamp`, `speaker`, `content`가 분리 저장되어 있어 정확한 발화자와 시각을 함께 반환할 수 있습니다. `idx_messages_date` 인덱스로 날짜 범위 쿼리도 빠르게 처리됩니다.

### 왜 페이지 전환 시 컴포넌트를 언마운트하지 않나? (always-mount 패턴)

React에서 컴포넌트를 언마운트하면 그 안의 모든 state가 사라집니다. 검색 결과를 보다가 채팅 기록을 확인하고 돌아왔을 때 결과가 유지되도록 하려면 두 가지 방법이 있습니다: (1) 상위 컴포넌트로 state 끌어올리기, (2) 모든 페이지를 항상 마운트된 상태로 유지하되 `display: none`으로 숨기기. 후자가 state 공유 없이 각 페이지의 독립성을 유지하면서 코드를 단순하게 만듭니다. `display: contents`는 레이아웃에 영향 없이 활성 페이지를 표시합니다.
