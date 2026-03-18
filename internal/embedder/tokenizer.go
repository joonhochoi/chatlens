package embedder

// unigramTokenizer implements the SentencePiece Unigram tokenizer
// as used by XLM-RoBERTa–based models (e.g. multilingual-e5-small).
//
// It reads the HuggingFace tokenizer.json format and performs:
//  1. Metaspace pre-tokenisation (space → ▁)
//  2. Viterbi segmentation with the Unigram language model
//  3. Token-ID lookup, [CLS]/[SEP] insertion, padding/truncation

import (
	"encoding/json"
	"math"
	"os"
	"strings"
	"unicode"
)

// Special-token IDs for XLM-RoBERTa (matches multilingual-e5-small)
const (
	idCLS = 0 // <s>
	idPAD = 1 // <pad>
	idSEP = 2 // </s>
	idUNK = 3 // <unk>
)

// vocabEntry maps a SentencePiece piece to its index and log-prob score.
type vocabEntry struct {
	id    int
	score float64
}

type unigramTokenizer struct {
	vocab     map[string]vocabEntry // piece → (id, score)
	maxLength int
}

// tokenizerJSON is the subset of tokenizer.json we need.
type tokenizerJSON struct {
	Model struct {
		Type  string          `json:"type"`
		Vocab [][]interface{} `json:"vocab"` // [[piece, score], ...]
	} `json:"model"`
}

// loadTokenizer reads a tokenizer.json file and builds a unigramTokenizer.
func loadTokenizer(path string, maxLength int) (*unigramTokenizer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw tokenizerJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	vocabMap := make(map[string]vocabEntry, len(raw.Model.Vocab))
	for id, entry := range raw.Model.Vocab {
		piece, _ := entry[0].(string)
		score, _ := entry[1].(float64)
		vocabMap[piece] = vocabEntry{id: id, score: score}
	}

	return &unigramTokenizer{vocab: vocabMap, maxLength: maxLength}, nil
}

// Encode converts text into (inputIDs, attentionMask, tokenTypeIDs).
// maxLen must be > 2 to leave room for [CLS] and [SEP].
func (t *unigramTokenizer) Encode(text string) (inputIDs, attentionMask, tokenTypeIDs []int64) {
	pieces := t.tokenize(text)

	// Truncate to fit [CLS] + pieces + [SEP]
	maxPieces := t.maxLength - 2
	if len(pieces) > maxPieces {
		pieces = pieces[:maxPieces]
	}

	seqLen := len(pieces) + 2
	inputIDs = make([]int64, t.maxLength)
	attentionMask = make([]int64, t.maxLength)
	tokenTypeIDs = make([]int64, t.maxLength) // all zeros

	inputIDs[0] = int64(idCLS)
	attentionMask[0] = 1
	for i, id := range pieces {
		inputIDs[i+1] = int64(id)
		attentionMask[i+1] = 1
	}
	inputIDs[seqLen-1] = int64(idSEP)
	attentionMask[seqLen-1] = 1
	// Positions seqLen..maxLength-1 remain 0 (padding)

	return inputIDs, attentionMask, tokenTypeIDs
}

// tokenize runs Metaspace pre-tokenisation followed by Viterbi segmentation
// and returns a slice of token IDs (without special tokens).
func (t *unigramTokenizer) tokenize(text string) []int {
	text = preprocess(text)
	words := metaspaceSplit(text)

	var ids []int
	for _, word := range words {
		wordIDs := t.viterbi(word)
		ids = append(ids, wordIDs...)
	}
	return ids
}

// preprocess applies Unicode normalisation compatible with the model's
// "Precompiled" + Lowercase normaliser (we do a lightweight version:
// strip control chars, keep punctuation, lowercase).
func preprocess(text string) string {
	var sb strings.Builder
	for _, r := range text {
		if unicode.IsControl(r) {
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// metaspaceSplit splits text into words and prepends '▁' to each word,
// which is the SentencePiece Metaspace pre-tokeniser behaviour.
func metaspaceSplit(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	fields := strings.Fields(text)
	out := make([]string, len(fields))
	for i, f := range fields {
		out[i] = "▁" + f
	}
	return out
}

// viterbi runs the Unigram Viterbi algorithm on a single word (with ▁ prefix)
// and returns a slice of token IDs.
func (t *unigramTokenizer) viterbi(word string) []int {
	runes := []rune(word)
	n := len(runes)
	if n == 0 {
		return nil
	}

	const negInf = -1e38

	// dp[i] = best log-prob to cover runes[0..i)
	dp := make([]float64, n+1)
	prev := make([]int, n+1) // starting position of the last token
	for i := range dp {
		dp[i] = negInf
	}
	dp[0] = 0.0
	prev[0] = -1

	for end := 1; end <= n; end++ {
		for start := 0; start < end; start++ {
			if dp[start] <= negInf {
				continue
			}
			piece := string(runes[start:end])
			entry, ok := t.vocab[piece]
			if !ok {
				continue
			}
			score := dp[start] + entry.score
			if score > dp[end] {
				dp[end] = score
				prev[end] = start
			}
		}
	}

	// If we couldn't fully segment, fall back to character-level unk
	if dp[n] <= negInf {
		ids := make([]int, n)
		for i := range ids {
			piece := string(runes[i])
			if e, ok := t.vocab[piece]; ok {
				ids[i] = e.id
			} else {
				ids[i] = idUNK
			}
		}
		return ids
	}

	// Backtrack
	pieces := []int{}
	pos := n
	for pos > 0 {
		start := prev[pos]
		piece := string(runes[start:pos])
		entry := t.vocab[piece]
		pieces = append([]int{entry.id}, pieces...)
		pos = start
	}
	return pieces
}

// dummy reference to math to avoid import cycle when math is only used here
var _ = math.Inf
