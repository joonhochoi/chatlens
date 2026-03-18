# ChatLens

메신저 채팅 기록을 로컬에서 분석해 자연어로 검색하는 Windows/macOS 데스크톱 앱.

임베딩 모델을 앱에 내장하여 인덱싱과 검색 과정에서 채팅 내용이 외부로 전송되지 않습니다.
LLM은 로컬 Ollama 또는 Gemini/OpenAI API를 선택해 사용할 수 있습니다.

현재 지원 메신저 채팅 기록 : 카카오톡(폰/PC 내보내기 txt파일)

이 프로젝트는 Claude Code로 작업되었으며, 상세 개발 계획에서 시작해서 어느정도까지 프로그램을 구현할 수 있는가를 테스트 하는 목적과 RAG의 기본 원리를 조금이나마 구현하면서 느껴보기 위해 작업되었습니다.

---

## 주요 기능

- **AI 검색 (RAG)** — 자연어 질문으로 관련 대화 청크를 검색하고 LLM이 요약 답변 생성 (LLM 호출 시에만 외부 API 사용)
- **키워드 검색** — SQL `LIKE` 기반 원문 메시지 직접 검색, 날짜 범위 필터 지원
- **채팅 기록 뷰어** — 날짜별 전체 대화 열람, 검색 결과에서 해당 시간으로 바로 이동
- **리더 중심 검색** — 특정 화자 발언에 가중치(×1.5)를 부여해 검색 정확도 향상
- **중복 방지** — 기간이 겹치는 파일을 추가해도 메시지 단위 해시로 중복 임베딩 없음

---

## 기술 스택

| 레이어 | 기술 |
|--------|------|
| 데스크톱 프레임워크 | [Wails v2](https://wails.io) (Go + WebView2) |
| 백엔드 | Go 1.23, CGO |
| 프론트엔드 | React + TypeScript |
| 로컬 임베딩 | ONNX Runtime 1.17.0 + `intfloat/multilingual-e5-small` INT8 |
| 벡터 DB | SQLite + [sqlite-vec](https://github.com/asg017/sqlite-vec) |
| LLM | Ollama (로컬) / Gemini / OpenAI |

---

## 설치 및 실행

### 1. 릴리즈 다운로드

[Releases](../../releases) 페이지에서 운영체제에 맞는 압축 파일을 내려받아 원하는 폴더에 압축 해제합니다.

```
chatlens-windows/
├── chatlens.exe
└── onnx/
    ├── model.onnx
    ├── tokenizer.json
    └── onnxruntime.dll
```

> **macOS**: `chatlens.app`을 Applications 폴더에 복사하거나 바로 실행합니다.
> 처음 실행 시 Gatekeeper 경고가 뜨면 터미널에서 `xattr -cr chatlens.app` 을 실행하세요.

### 2. 첫 실행

1. **설정(⚙️)** 탭에서 LLM 설정을 완료합니다.
   - 로컬 Ollama를 쓴다면 Ollama가 실행 중인지 확인 (`ollama serve`)
   - Gemini / OpenAI를 쓴다면 API 키를 입력합니다.
2. **리더 이름**을 설정합니다 (검색 가중치 적용 대상 화자).

### 3. 채팅 파일 가져오기

카카오톡 앱에서 채팅방 → 설정 → 대화 내보내기로 `.txt` 파일을 추출한 뒤, **업로드(📂)** 탭에서 파일을 선택합니다.

- 모바일 내보내기 / PC 내보내기 포맷 모두 지원
- 여러 파일 동시 선택 가능, 기간이 겹쳐도 중복 없이 처리

### 4. 검색

| 탭 | 설명 |
|----|------|
| 🤖 AI 검색 | 자연어 질문 입력 → LLM 요약 답변 + 관련 대화 출처 |
| 🔍 키워드 | 특정 단어/문장 직접 검색, 날짜 범위 지정 가능 |
| 💬 기록 | 날짜별 전체 대화 열람 |

---

## 데이터 저장 위치

실행 파일과 같은 폴더에 자동 생성됩니다.

```
(실행 파일 폴더)/
├── settings.json    — 사용자 설정
└── chat_data.db     — 임베딩 벡터 및 메시지 DB
```

임베딩·검색 과정에서 채팅 내용은 외부로 전송되지 않습니다. LLM으로 Gemini/OpenAI를 사용하는 경우 요약 요청 시 관련 대화 청크가 해당 API로 전송됩니다.

---

## 소스 빌드

### 필수 도구

```bash
# Go 1.21+
https://go.dev/dl/

# Node.js 18+
https://nodejs.org/

# Wails CLI v2
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# MinGW-w64 (Windows — CGO 컴파일러)
# MSYS2 사용 시:
pacman -S mingw-w64-x86_64-gcc mingw-w64-x86_64-sqlite3
# 또는 Chocolatey:
choco install mingw
```

> **macOS**: Xcode Command Line Tools(`xcode-select --install`)만 있으면 CGO가 동작합니다.

### ONNX 모델 파일 준비

프로젝트 루트에 `onnx/` 디렉토리를 만들고 아래 파일을 배치합니다.

| 파일 | 출처 |
|------|------|
| `model.onnx` | HuggingFace [`intfloat/multilingual-e5-small`](https://huggingface.co/intfloat/multilingual-e5-small) — ONNX INT8 변환본 |
| `tokenizer.json` | 동일 모델 페이지의 `tokenizer.json` |
| `onnxruntime.dll` (Windows) | [ONNX Runtime v1.17.0](https://github.com/microsoft/onnxruntime/releases/tag/v1.17.0) Windows x64 |
| `libonnxruntime.dylib` (macOS) | 동일 릴리즈 페이지 macOS 패키지 |

### 빌드

```bash
# Windows (프로젝트 루트에서)
export PATH="/c/msys64/mingw64/bin:$PATH"
CGO_ENABLED=1 wails build

# macOS
CGO_ENABLED=1 wails build                        # amd64
CGO_ENABLED=1 wails build -platform darwin/universal  # 유니버셜 (arm64 + amd64)
```

빌드 결과: `build/bin/chatlens.exe` (Windows) / `build/bin/chatlens.app` (macOS)

빌드 후 `onnx/` 폴더를 실행 파일 옆에 복사합니다.

```bash
# Windows
cp -r onnx build/bin/

# macOS
cp -r onnx build/bin/chatlens.app/Contents/MacOS/
```

### 개발 서버 (핫 리로드)

```bash
# Windows
CGO_ENABLED=1 wails dev

# 프론트엔드 타입 체크
cd frontend && ./node_modules/.bin/tsc --noEmit
```

---

## 상세 문서

기술 아키텍처, 파싱 규칙, 청킹 전략, DB 스키마 등 상세 내용은 [detail_description.md](./detail_description.md)를 참고하세요.
