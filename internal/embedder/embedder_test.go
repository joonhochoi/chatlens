package embedder

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

// onnxDir returns the path to the onnx/ directory (project root level).
func onnxDir(t *testing.T) string {
	t.Helper()
	// package is at internal/embedder/, project root is two levels up
	dir, _ := filepath.Abs(filepath.Join("..", "..", "onnx"))
	if _, err := os.Stat(filepath.Join(dir, "model.onnx")); os.IsNotExist(err) {
		t.Skip("onnx/model.onnx not found — skipping ONNX tests")
	}
	return dir
}

// ── Tokenizer tests (no ONNX required) ────────────────────────────────────

func TestTokenizerLoad(t *testing.T) {
	tokPath := filepath.Join("..", "..", "onnx", "tokenizer.json")
	if _, err := os.Stat(tokPath); os.IsNotExist(err) {
		t.Skip("tokenizer.json not found")
	}
	tok, err := loadTokenizer(tokPath, 128)
	if err != nil {
		t.Fatalf("loadTokenizer: %v", err)
	}
	if len(tok.vocab) == 0 {
		t.Error("vocab is empty")
	}
	t.Logf("vocab size: %d", len(tok.vocab))
}

func TestTokenizerEncode_Korean(t *testing.T) {
	tokPath := filepath.Join("..", "..", "onnx", "tokenizer.json")
	if _, err := os.Stat(tokPath); os.IsNotExist(err) {
		t.Skip("tokenizer.json not found")
	}
	tok, err := loadTokenizer(tokPath, 128)
	if err != nil {
		t.Fatalf("loadTokenizer: %v", err)
	}

	inputIDs, attnMask, tokenTypeIDs := tok.Encode("passage: 안녕하세요 테스트입니다")

	if len(inputIDs) != 128 {
		t.Errorf("inputIDs len = %d, want 128", len(inputIDs))
	}
	if inputIDs[0] != int64(idCLS) {
		t.Errorf("inputIDs[0] = %d, want CLS=%d", inputIDs[0], idCLS)
	}
	// attention mask should have at least a few real tokens
	var realTokens int
	for _, m := range attnMask {
		realTokens += int(m)
	}
	if realTokens < 3 {
		t.Errorf("too few real tokens: %d", realTokens)
	}
	t.Logf("real tokens: %d, tokenTypeIDs[0]: %d", realTokens, tokenTypeIDs[0])
}

func TestTokenizerEncode_English(t *testing.T) {
	tokPath := filepath.Join("..", "..", "onnx", "tokenizer.json")
	if _, err := os.Stat(tokPath); os.IsNotExist(err) {
		t.Skip("tokenizer.json not found")
	}
	tok, _ := loadTokenizer(tokPath, 128)
	inputIDs, attnMask, _ := tok.Encode("query: what is the best apartment in Seoul?")
	var real int
	for _, m := range attnMask {
		real += int(m)
	}
	if real < 5 {
		t.Errorf("too few tokens for English: %d", real)
	}
	t.Logf("English tokens: %d, first 5 ids: %v", real, inputIDs[:5])
}

// ── ONNX inference tests (require onnx/ directory) ────────────────────────

func TestEmbedderNew(t *testing.T) {
	emb, err := New(onnxDir(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer emb.Close()
}

func TestEmbedPassage_Dim(t *testing.T) {
	emb, err := New(onnxDir(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer emb.Close()

	vec, err := emb.EmbedPassage("안녕하세요 테스트입니다")
	if err != nil {
		t.Fatalf("EmbedPassage: %v", err)
	}
	if len(vec) != Dim {
		t.Errorf("dim = %d, want %d", len(vec), Dim)
	}
	// Check L2 norm ≈ 1.0
	var sum float64
	for _, v := range vec {
		sum += float64(v) * float64(v)
	}
	norm := math.Sqrt(sum)
	if math.Abs(norm-1.0) > 0.01 {
		t.Errorf("L2 norm = %.4f, want ≈ 1.0", norm)
	}
	t.Logf("embedding dim=%d, norm=%.6f, first3=%v", len(vec), norm, vec[:3])
}

func TestEmbedQuery_SimilarToPassage(t *testing.T) {
	emb, err := New(onnxDir(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer emb.Close()

	// 유사한 텍스트는 높은 코사인 유사도를 가져야 함
	query := "부동산 전망이 어떻게 되나요"
	passage := "올해 부동산 시장은 상승세를 보이고 있습니다"
	irrelevant := "오늘 날씨가 맑고 기온이 높습니다"

	qVec, _ := emb.EmbedQuery(query)
	pVec, _ := emb.EmbedPassage(passage)
	iVec, _ := emb.EmbedPassage(irrelevant)

	simRelevant := cosineSim(qVec, pVec)
	simIrrelevant := cosineSim(qVec, iVec)
	t.Logf("sim(query, relevant)=%.4f, sim(query, irrelevant)=%.4f", simRelevant, simIrrelevant)

	if simRelevant <= simIrrelevant {
		t.Errorf("relevant similarity %.4f should be > irrelevant %.4f", simRelevant, simIrrelevant)
	}
}

func cosineSim(a, b []float32) float32 {
	var dot float32
	for i := range a {
		dot += a[i] * b[i]
	}
	return dot // vectors are already L2-normalised
}
