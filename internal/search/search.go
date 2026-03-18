// Package search implements vector similarity search over the chunk_vectors table.
package search

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"chatlens/internal/db"
)

const leaderBoost = 1.5 // IsLeader 청크에 곱하는 가중치

// Candidate is a search result candidate before re-ranking.
type Candidate struct {
	ID         int64
	Text       string
	IsLeader   bool
	StartTime  time.Time
	Similarity float64 // cosine similarity (1 = identical)
	FinalScore float64 // after leader boost
}

// TopChunks searches for the most similar chunks to queryVec, applies leader
// weighting, and returns the top topK results.
// startDate and endDate are optional "YYYY-MM-DD" strings; empty means no filter.
func TopChunks(database *sql.DB, queryVec []float32, topK int, startDate, endDate string) ([]Candidate, error) {
	if topK < 1 {
		topK = 5
	}
	knnCandidates := topK * 3 // 재정렬 여유를 위해 3배 후보 확보

	vecBytes := db.Float32ToBytes(queryVec)

	var whereParts []string
	var whereArgs []interface{}
	if startDate != "" {
		whereParts = append(whereParts, "date(c.start_time) >= ?")
		whereArgs = append(whereArgs, startDate)
	}
	if endDate != "" {
		whereParts = append(whereParts, "date(c.start_time) <= ?")
		whereArgs = append(whereArgs, endDate)
	}
	whereSQL := ""
	if len(whereParts) > 0 {
		whereSQL = "WHERE " + strings.Join(whereParts, " AND ")
	}

	querySQL := fmt.Sprintf(`
		SELECT c.id, c.text, c.is_leader, c.start_time,
		       (1.0 - vec_distance_cosine(v.embedding, ?)) AS similarity
		FROM chunk_vectors v
		JOIN chunks c ON c.id = v.chunk_id
		%s
		ORDER BY vec_distance_cosine(v.embedding, ?) ASC
		LIMIT ?
	`, whereSQL)

	args := []interface{}{vecBytes}
	args = append(args, whereArgs...)
	args = append(args, vecBytes, knnCandidates)

	rows, err := database.Query(querySQL, args...)
	if err != nil {
		return nil, fmt.Errorf("knn query: %w", err)
	}
	defer rows.Close()

	var candidates []Candidate
	for rows.Next() {
		var c Candidate
		var isLeaderInt int
		var startTimeStr string
		if err := rows.Scan(&c.ID, &c.Text, &isLeaderInt, &startTimeStr, &c.Similarity); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		c.IsLeader = isLeaderInt == 1
		c.StartTime, _ = time.Parse(time.RFC3339, startTimeStr)
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	// IsLeader 가중치 적용 후 재정렬
	for i := range candidates {
		score := candidates[i].Similarity
		if candidates[i].IsLeader {
			score *= leaderBoost
		}
		candidates[i].FinalScore = score
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].FinalScore > candidates[j].FinalScore
	})

	if len(candidates) > topK {
		candidates = candidates[:topK]
	}
	return candidates, nil
}
