// Package embedder provides local text embedding using the
// intfloat/multilingual-e5-small ONNX model (INT8 quantized, 384-dim).
//
// Distribution layout (relative to the executable):
//
//	onnx/
//	  model.onnx          – INT8 quantized ONNX model (~113 MB)
//	  tokenizer.json      – HuggingFace tokenizer.json (~17 MB)
//	  onnxruntime.dll     – Microsoft ONNX Runtime shared library (~60 MB)
package embedder

import (
	"fmt"
	"math"
	"path/filepath"

	ort "github.com/yalue/onnxruntime_go"
)

const (
	// MaxLength is the maximum token sequence length (including CLS/SEP).
	MaxLength = 512
	// Dim is the embedding dimension of multilingual-e5-small.
	Dim = 384
)

// Embedder holds the ONNX session and tokenizer for multilingual-e5-small.
type Embedder struct {
	session   *ort.DynamicAdvancedSession
	tokenizer *unigramTokenizer
}

// New initialises the embedder from the given onnxDir directory, which must
// contain model.onnx, tokenizer.json, and (on Windows) onnxruntime.dll.
func New(onnxDir string) (*Embedder, error) {
	dllPath := filepath.Join(onnxDir, "onnxruntime.dll")
	modelPath := filepath.Join(onnxDir, "model.onnx")
	tokPath := filepath.Join(onnxDir, "tokenizer.json")

	// Set the ONNX Runtime DLL path before initialising the environment.
	ort.SetSharedLibraryPath(dllPath)
	if err := ort.InitializeEnvironment(); err != nil {
		return nil, fmt.Errorf("onnxruntime init: %w", err)
	}

	session, err := ort.NewDynamicAdvancedSession(
		modelPath,
		[]string{"input_ids", "attention_mask", "token_type_ids"},
		[]string{"last_hidden_state"},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create onnx session: %w", err)
	}

	tok, err := loadTokenizer(tokPath, MaxLength)
	if err != nil {
		session.Destroy()
		return nil, fmt.Errorf("load tokenizer: %w", err)
	}

	return &Embedder{session: session, tokenizer: tok}, nil
}

// Close releases the ONNX session.
func (e *Embedder) Close() {
	if e.session != nil {
		e.session.Destroy()
	}
	ort.DestroyEnvironment()
}

// EmbedPassage embeds a document passage ("passage: " prefix).
func (e *Embedder) EmbedPassage(text string) ([]float32, error) {
	return e.embed("passage: " + text)
}

// EmbedQuery embeds a search query ("query: " prefix).
func (e *Embedder) EmbedQuery(query string) ([]float32, error) {
	return e.embed("query: " + query)
}

// EmbedBatch embeds multiple passages with an optional progress callback.
func (e *Embedder) EmbedBatch(texts []string, progress func(done, total int)) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i, t := range texts {
		vec, err := e.EmbedPassage(t)
		if err != nil {
			return nil, fmt.Errorf("embed[%d]: %w", i, err)
		}
		results[i] = vec
		if progress != nil {
			progress(i+1, len(texts))
		}
	}
	return results, nil
}

// embed is the internal function that tokenizes, runs ONNX inference,
// applies mean pooling, and L2-normalises the result.
func (e *Embedder) embed(text string) ([]float32, error) {
	inputIDs, attnMask, tokenTypeIDs := e.tokenizer.Encode(text)

	seqLen := int64(MaxLength)
	shape := ort.NewShape(1, seqLen)

	tInputIDs, err := ort.NewTensor(shape, inputIDs)
	if err != nil {
		return nil, err
	}
	defer tInputIDs.Destroy()

	tAttnMask, err := ort.NewTensor(shape, attnMask)
	if err != nil {
		return nil, err
	}
	defer tAttnMask.Destroy()

	tTokenType, err := ort.NewTensor(shape, tokenTypeIDs)
	if err != nil {
		return nil, err
	}
	defer tTokenType.Destroy()

	// Output shape: [1, seqLen, Dim]
	outShape := ort.NewShape(1, seqLen, Dim)
	tOutput, err := ort.NewEmptyTensor[float32](outShape)
	if err != nil {
		return nil, err
	}
	defer tOutput.Destroy()

	if err := e.session.Run(
		[]ort.ArbitraryTensor{tInputIDs, tAttnMask, tTokenType},
		[]ort.ArbitraryTensor{tOutput},
	); err != nil {
		return nil, fmt.Errorf("onnx run: %w", err)
	}

	hidden := tOutput.GetData() // flat slice: [1 * seqLen * Dim]
	embedding := meanPool(hidden, attnMask, int(seqLen), Dim)
	l2Normalize(embedding)
	return embedding, nil
}

// meanPool computes the attention-weighted mean over the sequence dimension.
// hidden is flat [seqLen * dim], attnMask is [seqLen].
func meanPool(hidden []float32, attnMask []int64, seqLen, dim int) []float32 {
	result := make([]float32, dim)
	var maskSum float64
	for t := 0; t < seqLen; t++ {
		if attnMask[t] == 0 {
			continue
		}
		maskSum++
		base := t * dim
		for d := 0; d < dim; d++ {
			result[d] += hidden[base+d]
		}
	}
	if maskSum > 0 {
		inv := float32(1.0 / maskSum)
		for d := range result {
			result[d] *= inv
		}
	}
	return result
}

// l2Normalize divides the vector by its L2 norm in-place.
func l2Normalize(v []float32) {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	norm := float32(math.Sqrt(sum))
	if norm == 0 {
		return
	}
	for i := range v {
		v[i] /= norm
	}
}
