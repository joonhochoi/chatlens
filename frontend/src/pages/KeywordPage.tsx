import { useState, useRef } from 'react';
import { SearchKeyword, GetSettings } from '../../wailsjs/go/main/App';
import { main } from '../../wailsjs/go/models';
import { useEffect } from 'react';
type KeywordHit = main.KeywordHit;

// 검색어를 강조 표시하는 컴포넌트
function Highlighted({ text, keyword }: { text: string; keyword: string }) {
  if (!keyword) return <>{text}</>;
  const parts = text.split(new RegExp(`(${keyword.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')})`, 'gi'));
  return (
    <>
      {parts.map((part, i) =>
        part.toLowerCase() === keyword.toLowerCase()
          ? <mark key={i} className="kw-highlight">{part}</mark>
          : <span key={i}>{part}</span>
      )}
    </>
  );
}

interface KeywordPageProps {
  onGoToChat: (date: string, time?: string) => void;
}

export default function KeywordPage({ onGoToChat }: KeywordPageProps) {
  const [query, setQuery] = useState('');
  const [hits, setHits] = useState<KeywordHit[]>([]);
  const [total, setTotal] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(false);
  const [searched, setSearched] = useState(false);
  const [leaderName, setLeaderName] = useState('');
  const [startDate, setStartDate] = useState('');
  const [endDate, setEndDate] = useState('');
  const lastKeyword = useRef('');
  const lastStartDate = useRef('');
  const lastEndDate = useRef('');

  useEffect(() => {
    GetSettings().then(s => setLeaderName(s.leaderName || ''));
  }, []);

  function doSearch(keyword: string, newOffset: number, sd: string, ed: string) {
    if (!keyword.trim()) return;
    setLoading(true);
    SearchKeyword(keyword.trim(), newOffset, sd, ed)
      .then(r => {
        if (newOffset === 0) {
          setHits(r.hits || []);
        } else {
          setHits(prev => [...prev, ...(r.hits || [])]);
        }
        setTotal(r.total);
        setHasMore(r.hasMore);
        setOffset(newOffset + (r.hits?.length ?? 0));
        setSearched(true);
      })
      .finally(() => setLoading(false));
  }

  function handleSearch() {
    if (!query.trim() || loading) return;
    lastKeyword.current = query.trim();
    lastStartDate.current = startDate;
    lastEndDate.current = endDate;
    setOffset(0);
    setHits([]);
    doSearch(query.trim(), 0, startDate, endDate);
  }

  function handleLoadMore() {
    doSearch(lastKeyword.current, offset, lastStartDate.current, lastEndDate.current);
  }

  return (
    <div className="page">
      <h2>키워드 검색</h2>

      <div className="search-bar">
        <input
          type="text"
          placeholder="검색할 단어나 문장을 입력하세요"
          value={query}
          onChange={e => setQuery(e.target.value)}
          onKeyDown={e => e.key === 'Enter' && handleSearch()}
          disabled={loading}
        />
        <button
          className="btn-primary"
          onClick={handleSearch}
          disabled={loading || !query.trim()}
        >
          {loading && offset === 0 ? '검색 중...' : '검색'}
        </button>
      </div>

      <div className="date-range-row">
        <span className="date-range-label">날짜 범위</span>
        <input
          type="date"
          className="date-input"
          value={startDate}
          onChange={e => setStartDate(e.target.value)}
          disabled={loading}
        />
        <span className="date-range-sep">~</span>
        <input
          type="date"
          className="date-input"
          value={endDate}
          onChange={e => setEndDate(e.target.value)}
          disabled={loading}
        />
        {(startDate || endDate) && (
          <button className="btn-small" onClick={() => { setStartDate(''); setEndDate(''); }}>
            전체
          </button>
        )}
      </div>

      {searched && (
        <p className="kw-total">
          총 <strong>{total.toLocaleString()}</strong>개 메시지 중{' '}
          <strong>{hits.length.toLocaleString()}</strong>개 표시 중
        </p>
      )}

      {searched && hits.length === 0 && !loading && (
        <div className="search-hint"><p>일치하는 메시지가 없습니다.</p></div>
      )}

      <div className="kw-results">
        {hits.map((hit, i) => {
          const isLeader = !!(leaderName && hit.speaker.includes(leaderName));
          return (
            <div key={i} className={`kw-card ${isLeader ? 'leader' : ''}`}>
              <div className="kw-meta">
                <span className="kw-date">{hit.date}</span>
                <span className="kw-time">{hit.time}</span>
                <button
                  className="btn-goto-chat"
                  onClick={() => onGoToChat(hit.dateKey, hit.time)}
                  title="이날 채팅 기록 보기"
                >
                  💬 기록 보기
                </button>
                {isLeader && <span className="leader-badge">리더</span>}
              </div>
              <div className="kw-msg">
                <span className={isLeader ? 'chat-speaker-leader' : 'chat-speaker'}>
                  {hit.speaker}
                </span>
                <span className="msg-sep">: </span>
                <span className="kw-content">
                  <Highlighted text={hit.content} keyword={lastKeyword.current} />
                </span>
              </div>
            </div>
          );
        })}
      </div>

      {hasMore && (
        <div className="kw-more-row">
          <button
            className="btn-small"
            onClick={handleLoadMore}
            disabled={loading}
          >
            {loading ? '불러오는 중...' : `다음 10개 더 보기`}
          </button>
        </div>
      )}

      {!searched && (
        <div className="search-hint">
          <p>채팅 메시지에서 특정 단어나 문장을 직접 찾습니다.</p>
          <p className="hint">최신 메시지부터 10개씩 표시됩니다. AI 요약 없이 원문 그대로 검색합니다.</p>
        </div>
      )}
    </div>
  );
}
