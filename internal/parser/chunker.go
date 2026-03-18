package parser

import (
	"math"
	"strings"
	"time"
)

const (
	chunkGap        = 30 * time.Minute
	leaderSessionGap = 5 * time.Minute
	leaderRunGap    = 5 * time.Minute // 리더 발언 런 사이 간격 임계값
)

// SemanticEmbedder embeds a passage for semantic similarity comparison.
type SemanticEmbedder interface {
	EmbedPassage(text string) ([]float32, error)
}

// ChunkOptions controls the chunking behaviour.
type ChunkOptions struct {
	LeaderName        string
	MaxMessages       int     // 청크당 최대 메시지 수 (0 = 무제한)
	OverlapMessages   int     // 인접 청크 간 오버랩 메시지 수 (0 = 없음)
	UseLeaderMicro    bool    // 리더 발언 마이크로 청킹 (전략 3)
	UseSemanticSplit  bool    // 의미 기반 추가 분할 (전략 4)
	SemanticThreshold float64 // 코사인 유사도 임계값 (전략 4)
	Embedder          SemanticEmbedder
}

// Chunk represents a group of related messages.
type Chunk struct {
	Messages  []Message
	Text      string
	IsLeader  bool
	StartTime time.Time
	EndTime   time.Time
}

// Chunkify converts messages into chunks using the provided options.
//
// Processing order:
//  1. Time-based + max-size splitting (전략 1+2 기반)
//  2. Leader micro-chunking (전략 3, optional)
//  3. Semantic splitting (전략 4, optional)
//  4. Overlap injection (전략 1)
func Chunkify(messages []Message, opts ChunkOptions) []Chunk {
	if len(messages) == 0 {
		return nil
	}

	maxMsgs := opts.MaxMessages
	if maxMsgs <= 0 {
		maxMsgs = 999999
	}

	// Phase 1: 시간 간격 + 최대 크기 기반 분할
	segs := timeChunk(messages, opts.LeaderName, maxMsgs)

	// Phase 2: 리더 마이크로 청킹
	if opts.UseLeaderMicro && opts.LeaderName != "" {
		segs = leaderMicroSplit(segs, opts.LeaderName)
	}

	// Phase 3: 의미 기반 분할
	if opts.UseSemanticSplit && opts.Embedder != nil && opts.SemanticThreshold > 0 {
		segs = semanticSplit(segs, opts.Embedder, opts.SemanticThreshold)
	}

	// Phase 4: 오버랩 적용
	if opts.OverlapMessages > 0 {
		segs = applyOverlap(segs, opts.OverlapMessages)
	}

	return buildChunks(segs, opts.LeaderName)
}

// ── Phase 1: 시간 기반 + 최대 크기 분할 ─────────────────────────────────────

func timeChunk(messages []Message, leaderName string, maxMsgs int) [][]Message {
	var segs [][]Message
	cur := []Message{messages[0]}
	leaderIn := speakerIsLeader(messages[0].Speaker, leaderName)

	for i := 1; i < len(messages); i++ {
		prev := messages[i-1]
		msg := messages[i]
		gap := msg.Timestamp.Sub(prev.Timestamp)

		split := false
		switch {
		case gap >= chunkGap:
			split = true
		case leaderIn && gap >= leaderSessionGap:
			split = true
		case len(cur) >= maxMsgs:
			split = true
		}

		if split {
			segs = append(segs, cur)
			cur = []Message{msg}
			leaderIn = speakerIsLeader(msg.Speaker, leaderName)
		} else {
			if speakerIsLeader(msg.Speaker, leaderName) {
				leaderIn = true
			}
			cur = append(cur, msg)
		}
	}
	segs = append(segs, cur)
	return segs
}

// ── Phase 2: 리더 마이크로 청킹 ─────────────────────────────────────────────

const leaderMicroMinSize = 8 // 이보다 작은 세그먼트는 분할 시도 안 함

func leaderMicroSplit(segs [][]Message, leaderName string) [][]Message {
	var result [][]Message
	for _, seg := range segs {
		if !segHasLeader(seg, leaderName) || len(seg) < leaderMicroMinSize {
			result = append(result, seg)
			continue
		}
		splits := findLeaderRunBoundaries(seg, leaderName)
		if len(splits) == 0 {
			result = append(result, seg)
			continue
		}
		prev := 0
		for _, sp := range splits {
			if sp > prev {
				result = append(result, seg[prev:sp])
			}
			prev = sp
		}
		if prev < len(seg) {
			result = append(result, seg[prev:])
		}
	}
	return result
}

// findLeaderRunBoundaries는 리더 발언 "런(run)" 사이 경계 인덱스를 반환합니다.
// 런: 리더 메시지들이 leaderRunGap 미만 간격으로 이어지는 묶음.
// 런과 런 사이의 중간점을 분할 위치로 사용합니다.
func findLeaderRunBoundaries(messages []Message, leaderName string) []int {
	var ldrIdx []int
	for i, m := range messages {
		if speakerIsLeader(m.Speaker, leaderName) {
			ldrIdx = append(ldrIdx, i)
		}
	}
	if len(ldrIdx) < 2 {
		return nil
	}

	var splitPoints []int
	for i := 1; i < len(ldrIdx); i++ {
		gap := messages[ldrIdx[i]].Timestamp.Sub(messages[ldrIdx[i-1]].Timestamp)
		if gap >= leaderRunGap {
			mid := (ldrIdx[i-1]+ldrIdx[i])/2 + 1
			if mid > 2 && mid < len(messages)-2 {
				splitPoints = append(splitPoints, mid)
			}
		}
	}
	return splitPoints
}

// ── Phase 3: 의미 기반 분할 ─────────────────────────────────────────────────

const semanticWindowSize = 5
const semanticMinSegSize = semanticWindowSize * 2

func semanticSplit(segs [][]Message, emb SemanticEmbedder, threshold float64) [][]Message {
	var result [][]Message
	for _, seg := range segs {
		if len(seg) < semanticMinSegSize {
			result = append(result, seg)
			continue
		}
		sub := semanticSplitSeg(seg, emb, threshold)
		result = append(result, sub...)
	}
	return result
}

func semanticSplitSeg(messages []Message, emb SemanticEmbedder, threshold float64) [][]Message {
	numWindows := len(messages) / semanticWindowSize
	if numWindows < 2 {
		return [][]Message{messages}
	}

	vecs := make([][]float32, numWindows)
	for i := 0; i < numWindows; i++ {
		start := i * semanticWindowSize
		end := start + semanticWindowSize
		if end > len(messages) {
			end = len(messages)
		}
		text := buildText(messages[start:end])
		v, err := emb.EmbedPassage(text)
		if err != nil {
			return [][]Message{messages} // 오류 시 분할하지 않음
		}
		vecs[i] = v
	}

	var splitPoints []int
	for i := 1; i < numWindows; i++ {
		if cosineSim(vecs[i-1], vecs[i]) < threshold {
			splitPoints = append(splitPoints, i*semanticWindowSize)
		}
	}
	if len(splitPoints) == 0 {
		return [][]Message{messages}
	}

	var result [][]Message
	prev := 0
	for _, sp := range splitPoints {
		if sp-prev >= semanticWindowSize {
			result = append(result, messages[prev:sp])
			prev = sp
		}
	}
	if prev < len(messages) {
		result = append(result, messages[prev:])
	}
	return result
}

func cosineSim(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// ── Phase 4: 오버랩 적용 ────────────────────────────────────────────────────

func applyOverlap(segs [][]Message, overlap int) [][]Message {
	if len(segs) <= 1 {
		return segs
	}
	result := make([][]Message, len(segs))
	result[0] = segs[0]
	for i := 1; i < len(segs); i++ {
		prev := segs[i-1]
		take := overlap
		if take > len(prev) {
			take = len(prev)
		}
		tail := prev[len(prev)-take:]
		merged := make([]Message, 0, take+len(segs[i]))
		merged = append(merged, tail...)
		merged = append(merged, segs[i]...)
		result[i] = merged
	}
	return result
}

// ── 공통 헬퍼 ───────────────────────────────────────────────────────────────

func buildChunks(segs [][]Message, leaderName string) []Chunk {
	chunks := make([]Chunk, 0, len(segs))
	for _, seg := range segs {
		if len(seg) > 0 {
			chunks = append(chunks, finishChunk(seg, leaderName))
		}
	}
	return chunks
}

func finishChunk(messages []Message, leaderName string) Chunk {
	return Chunk{
		Messages:  messages,
		Text:      buildText(messages),
		IsLeader:  segHasLeader(messages, leaderName),
		StartTime: messages[0].Timestamp,
		EndTime:   messages[len(messages)-1].Timestamp,
	}
}

func segHasLeader(messages []Message, leaderName string) bool {
	if leaderName == "" {
		return false
	}
	for _, m := range messages {
		if speakerIsLeader(m.Speaker, leaderName) {
			return true
		}
	}
	return false
}

func speakerIsLeader(speaker, leaderName string) bool {
	return leaderName != "" && strings.Contains(speaker, leaderName)
}

// buildText serializes messages to "화자: 내용\n..." (타임스탬프 제외).
func buildText(messages []Message) string {
	var sb strings.Builder
	for _, m := range messages {
		sb.WriteString(m.Speaker)
		sb.WriteString(": ")
		sb.WriteString(m.Content)
		sb.WriteByte('\n')
	}
	return strings.TrimRight(sb.String(), "\n")
}
