import { useState, useEffect, useRef } from 'react';
import { GetChatDates, GetMessagesByDate, GetSettings } from '../../wailsjs/go/main/App';
import { main } from '../../wailsjs/go/models';
type ChatMessage = main.ChatMessage;

interface ChatPageProps {
  initialDate?: string;  // "YYYY-MM-DD"
  initialTime?: string;  // "HH:MM" — 이 시간 근처로 스크롤
  onDateConsumed?: () => void;
}

const WEEKDAYS = ['일', '월', '화', '수', '목', '금', '토'];

function getWeekday(dateStr: string): string {
  const d = new Date(dateStr);
  return WEEKDAYS[d.getDay()];
}

export default function ChatPage({ initialDate, initialTime, onDateConsumed }: ChatPageProps) {
  const [dates, setDates] = useState<string[]>([]);
  const [selectedDate, setSelectedDate] = useState('');
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [loading, setLoading] = useState(false);
  const [leaderName, setLeaderName] = useState('');
  const [highlightTime, setHighlightTime] = useState<string | undefined>();

  const [selectedYear, setSelectedYear] = useState('');
  const [selectedMonth, setSelectedMonth] = useState('');

  const scrollRef = useRef<HTMLDivElement>(null);
  const pendingScrollRef = useRef<string | undefined>(); // HH:MM to scroll to after load

  useEffect(() => {
    GetSettings().then(s => setLeaderName(s.leaderName || ''));
    GetChatDates().then(d => setDates(d || []));
  }, []);

  // When initialDate/initialTime arrive (navigation from search)
  useEffect(() => {
    if (!initialDate || dates.length === 0) return;
    if (!dates.includes(initialDate)) return;

    const [y, m] = initialDate.split('-');
    setSelectedYear(y);
    setSelectedMonth(m);
    setSelectedDate(initialDate);
    if (initialTime) {
      pendingScrollRef.current = initialTime;
    }
    onDateConsumed?.();
  }, [initialDate, dates]);

  // Set default year/month when dates first load
  useEffect(() => {
    if (dates.length === 0 || selectedYear) return;
    const [y, m] = dates[dates.length - 1].split('-');
    setSelectedYear(y);
    setSelectedMonth(m);
  }, [dates]);

  // Load messages when selectedDate changes
  useEffect(() => {
    if (!selectedDate) return;
    setLoading(true);
    setHighlightTime(undefined);
    GetMessagesByDate(selectedDate)
      .then(msgs => setMessages(msgs || []))
      .finally(() => setLoading(false));
  }, [selectedDate]);

  // Scroll to target time after messages render
  useEffect(() => {
    if (!messages.length) return;

    const target = pendingScrollRef.current;
    if (target) {
      pendingScrollRef.current = undefined;
      // Wait one animation frame for DOM to update
      requestAnimationFrame(() => {
        const container = scrollRef.current;
        if (!container) return;

        const allMsgs = container.querySelectorAll<HTMLElement>('[data-msg-time]');
        let found: HTMLElement | null = null;
        for (const el of Array.from(allMsgs)) {
          if ((el.dataset.msgTime ?? '') >= target) {
            found = el;
            break;
          }
        }

        if (found) {
          found.scrollIntoView({ block: 'center', behavior: 'smooth' });
          const t = found.dataset.msgTime;
          setHighlightTime(t);
          setTimeout(() => setHighlightTime(undefined), 2500);
        } else {
          container.scrollTo({ top: container.scrollHeight });
        }
      });
    } else {
      scrollRef.current?.scrollTo({ top: 0 });
    }
  }, [messages]);

  const years = [...new Set(dates.map(d => d.split('-')[0]))].sort();
  const months = [...new Set(
    dates.filter(d => d.startsWith(selectedYear + '-')).map(d => d.split('-')[1])
  )].sort();
  const daysInMonth = dates.filter(d => d.startsWith(`${selectedYear}-${selectedMonth}-`));

  function handleYearChange(y: string) {
    setSelectedYear(y);
    const firstMonth = dates.find(d => d.startsWith(y + '-'))?.split('-')[1] ?? '';
    setSelectedMonth(firstMonth);
    setSelectedDate('');
    setMessages([]);
  }

  function handleMonthChange(m: string) {
    setSelectedMonth(m);
    setSelectedDate('');
    setMessages([]);
  }

  const isEmpty = dates.length === 0;

  // 선택된 날짜를 한국어로 표시
  const selectedDateLabel = selectedDate
    ? `${parseInt(selectedDate.split('-')[0])}년 ${parseInt(selectedDate.split('-')[1])}월 ${parseInt(selectedDate.split('-')[2])}일 (${getWeekday(selectedDate)})`
    : null;

  return (
    <div className="page chat-page">
      <h2>채팅 기록</h2>

      {isEmpty ? (
        <div className="search-hint">
          <p>아직 가져온 채팅 파일이 없습니다.</p>
          <p className="hint">업로드 탭에서 채팅 파일을 가져오면 여기서 날짜별로 확인할 수 있습니다.</p>
        </div>
      ) : (
        <>
          {/* ── 날짜 선택 영역 ── */}
          <div className="chat-date-picker">
            <div className="date-picker-row">
              <select
                className="date-select"
                value={selectedYear}
                onChange={e => handleYearChange(e.target.value)}
              >
                {years.map(y => <option key={y} value={y}>{y}년</option>)}
              </select>
              <select
                className="date-select"
                value={selectedMonth}
                onChange={e => handleMonthChange(e.target.value)}
              >
                {months.map(m => <option key={m} value={m}>{parseInt(m)}월</option>)}
              </select>

              {/* 선택된 날짜 표시 */}
              {selectedDateLabel && (
                <div className="selected-date-label">
                  {selectedDateLabel}
                  {!loading && messages.length > 0 && (
                    <span className="msg-count-badge">{messages.length.toLocaleString()}개</span>
                  )}
                </div>
              )}
            </div>

            <div className="day-buttons">
              {daysInMonth.map(date => {
                const day = parseInt(date.split('-')[2]);
                return (
                  <button
                    key={date}
                    className={`day-btn ${selectedDate === date ? 'active' : ''}`}
                    onClick={() => setSelectedDate(date)}
                  >
                    {day}
                  </button>
                );
              })}
              {daysInMonth.length === 0 && (
                <span className="hint">이 월에 대화 기록이 없습니다.</span>
              )}
            </div>
          </div>

          {/* ── 메시지 목록 ── */}
          <div className="chat-messages" ref={scrollRef}>
            {loading && (
              <div className="spinner-area">
                <div className="spinner" />
                <p>불러오는 중...</p>
              </div>
            )}

            {!loading && !selectedDate && (
              <div className="search-hint">
                <p>위에서 날짜를 선택하면 그날의 대화 기록이 표시됩니다.</p>
              </div>
            )}

            {!loading && selectedDate && messages.length === 0 && (
              <div className="search-hint"><p>해당 날짜의 메시지가 없습니다.</p></div>
            )}

            {!loading && messages.map((msg, i) => {
              const isLeader = !!(leaderName && msg.speaker.includes(leaderName));
              const isHighlighted = highlightTime === msg.time;
              return (
                <div
                  key={i}
                  data-msg-time={msg.time}
                  className={[
                    'chat-msg',
                    isLeader ? 'chat-msg-leader' : '',
                    isHighlighted ? 'chat-msg-highlight' : '',
                  ].filter(Boolean).join(' ')}
                >
                  <div className="chat-msg-header">
                    <span className={isLeader ? 'chat-speaker-leader' : 'chat-speaker'}>
                      {msg.speaker}
                    </span>
                    <span className="chat-time">{msg.time}</span>
                  </div>
                  <div className="chat-content">{msg.content}</div>
                </div>
              );
            })}
          </div>
        </>
      )}
    </div>
  );
}
