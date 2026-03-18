package parser

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

// Message represents a single parsed chat message.
type Message struct {
	Timestamp time.Time
	Speaker   string
	Content   string
}

// ── 모바일 포맷 패턴 ────────────────────────────────────────────────────────
// 예: 2025. 7. 22. 오전 8:33, 화자이름 : 내용
var msgPattern = regexp.MustCompile(
	`^(\d{4})\. (\d{1,2})\. (\d{1,2})\. (오전|오후) (\d{1,2}):(\d{2}), (.+?) : (.+)$`,
)

// 예: 2025. 7. 22. 오전 8:33: 시스템 메시지
var sysPattern = regexp.MustCompile(
	`^\d{4}\. \d{1,2}\. \d{1,2}\. (오전|오후) \d{1,2}:\d{2}: .+$`,
)

// ── PC 포맷 패턴 ─────────────────────────────────────────────────────────────
// 날짜 구분선: --------------- 2025년 11월 1일 토요일 ---------------
var pcDatePattern = regexp.MustCompile(
	`^-+\s+(\d{4})년\s+(\d{1,2})월\s+(\d{1,2})일`,
)

// 메시지: [닉네임] [오전/오후 HH:MM] 내용
var pcMsgPattern = regexp.MustCompile(
	`^\[(.+?)\] \[(오전|오후) (\d{1,2}):(\d{2})\] (.*)$`,
)

// 시스템 메시지: "xxx님이 들어왔습니다.", "xxx님이 나갔습니다.", "메시지가 삭제되었습니다."
var pcSysPattern = regexp.MustCompile(
	`님이 (들어왔습니다|나갔습니다)\.$|^메시지가 삭제되었습니다\.$`,
)

// ParseFile reads a KakaoTalk chat export file and returns parsed messages.
// Supports both mobile export format and PC export format.
// ignoreKeywords: messages whose content (after trimming) consists solely of
// one of these keywords are skipped.
func ParseFile(path string, ignoreKeywords []string) ([]Message, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var messages []Message
	var current *Message
	var pcYear, pcMonth, pcDay int // current date context for PC format

	flush := func() {
		if current != nil {
			if msg := finalize(current, ignoreKeywords); msg != nil {
				messages = append(messages, *msg)
			}
			current = nil
		}
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()

		// ── PC 날짜 구분선 ──────────────────────────────────────────────────
		if m := pcDatePattern.FindStringSubmatch(line); m != nil {
			flush()
			fmt.Sscan(m[1], &pcYear)
			fmt.Sscan(m[2], &pcMonth)
			fmt.Sscan(m[3], &pcDay)
			continue
		}

		// ── PC 시스템 메시지 ────────────────────────────────────────────────
		if pcSysPattern.MatchString(line) {
			flush()
			continue
		}

		// ── 모바일 시스템 메시지 ────────────────────────────────────────────
		if sysPattern.MatchString(line) {
			flush()
			continue
		}

		// ── PC 일반 메시지 ──────────────────────────────────────────────────
		if m := pcMsgPattern.FindStringSubmatch(line); m != nil {
			flush()
			ts, err := parsePCTimestamp(pcYear, pcMonth, pcDay, m[2], m[3], m[4])
			if err != nil {
				current = nil
				continue
			}
			current = &Message{
				Timestamp: ts,
				Speaker:   m[1],
				Content:   m[5],
			}
			continue
		}

		// ── 모바일 일반 메시지 ──────────────────────────────────────────────
		if m := msgPattern.FindStringSubmatch(line); m != nil {
			flush()
			ts, err := parseTimestamp(m[1], m[2], m[3], m[4], m[5], m[6])
			if err != nil {
				current = nil
				continue
			}
			current = &Message{
				Timestamp: ts,
				Speaker:   m[7],
				Content:   m[8],
			}
			continue
		}

		// ── 연속 줄 (멀티라인 메시지) ───────────────────────────────────────
		if current != nil {
			current.Content += "\n" + line
		}
		// current == nil이면 첫 메시지 이전 헤더 줄 → 무시
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}

	flush()
	return messages, nil
}

// parsePCTimestamp builds a time.Time from PC format (date tracked separately, time in message).
func parsePCTimestamp(year, month, day int, ampm, hour, minute string) (time.Time, error) {
	var h, mi int
	fmt.Sscan(hour, &h)
	fmt.Sscan(minute, &mi)
	if ampm == "오전" {
		if h == 12 {
			h = 0
		}
	} else {
		if h != 12 {
			h += 12
		}
	}
	if year == 0 {
		return time.Time{}, fmt.Errorf("PC format: date separator not yet seen")
	}
	return time.Date(year, time.Month(month), day, h, mi, 0, 0, time.Local), nil
}

// parseTimestamp converts the matched regex groups into a time.Time.
// KakaoTalk uses Korean 오전(AM)/오후(PM) notation.
func parseTimestamp(year, month, day, ampm, hour, minute string) (time.Time, error) {
	var y, mo, d, h, mi int
	fmt.Sscan(year, &y)
	fmt.Sscan(month, &mo)
	fmt.Sscan(day, &d)
	fmt.Sscan(hour, &h)
	fmt.Sscan(minute, &mi)

	// Korean 12-hour clock edge cases:
	//   오전 12:xx → 0:xx (midnight)
	//   오후 12:xx → 12:xx (noon)
	//   오후  1:xx → 13:xx
	if ampm == "오전" {
		if h == 12 {
			h = 0
		}
	} else { // 오후
		if h != 12 {
			h += 12
		}
	}

	loc := time.Local
	return time.Date(y, time.Month(mo), d, h, mi, 0, 0, loc), nil
}

// finalize trims the content and checks ignore keywords.
// Returns nil if the message should be discarded.
func finalize(m *Message, ignoreKeywords []string) *Message {
	m.Content = strings.TrimRight(m.Content, "\r\n ")
	trimmed := strings.TrimSpace(m.Content)
	for _, kw := range ignoreKeywords {
		if trimmed == kw {
			return nil
		}
	}
	if trimmed == "" {
		return nil
	}
	return m
}
