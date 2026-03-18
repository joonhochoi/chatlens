import { useState, useEffect } from 'react';
import { Search, GetSettings } from '../../wailsjs/go/main/App';
import { main } from '../../wailsjs/go/models';
type SearchResult = main.SearchResult;

const SEARCH_STEPS = ['질문 임베딩 중...', 'LLM 응답 대기 중...'];

// 청크 텍스트를 줄별로 파싱해 리더 발언자 이름을 강조해 렌더링
function ChunkText({ text, leaderName }: { text: string; leaderName: string }) {
  const lines = text.split('\n');
  return (
    <div className="chunk-text">
      {lines.map((line, i) => {
        const sepIdx = line.indexOf(': ');
        if (!leaderName || sepIdx === -1) {
          return <div key={i} className="msg-line"><span className="msg-content">{line}</span></div>;
        }
        const speaker = line.substring(0, sepIdx);
        const content = line.substring(sepIdx + 2);
        const isLeader = speaker.includes(leaderName);
        return (
          <div key={i} className="msg-line">
            <span className={isLeader ? 'msg-speaker-leader' : 'msg-speaker'}>{speaker}</span>
            <span className="msg-sep">: </span>
            <span className="msg-content">{content}</span>
          </div>
        );
      })}
    </div>
  );
}

export default function SearchPage({ onGoToSettings, onGoToChat }: {
  onGoToSettings: () => void;
  onGoToChat: (date: string, time?: string) => void;
}) {
  const [query, setQuery] = useState('');
  const [result, setResult] = useState<SearchResult | null>(null);
  const [isSearching, setIsSearching] = useState(false);
  const [stepIdx, setStepIdx] = useState(0);
  const [error, setError] = useState('');
  const [leaderName, setLeaderName] = useState('');
  const [startDate, setStartDate] = useState('');
  const [endDate, setEndDate] = useState('');

  useEffect(() => {
    GetSettings().then(s => setLeaderName(s.leaderName || ''));
  }, []);

  function handleSearch() {
    if (!query.trim() || isSearching) return;
    setIsSearching(true);
    setError('');
    setResult(null);
    setStepIdx(0);

    // 진행 단계 시뮬레이션 (1.5초 후 LLM 대기 메시지로 전환)
    const timer = setTimeout(() => setStepIdx(1), 1500);

    Search(query.trim(), startDate, endDate)
      .then(r => setResult(r as SearchResult))
      .catch(e => {
        const msg = String(e);
        if (msg.includes('LLM') || msg.includes('ollama') || msg.includes('API')) {
          setError(msg + '\n\n설정 화면에서 LLM 제공자와 API 키를 확인하세요.');
        } else {
          setError(msg);
        }
      })
      .finally(() => {
        clearTimeout(timer);
        setIsSearching(false);
      });
  }

  const isLLMError = error.includes('설정 화면');

  return (
    <div className="page">
      <h2>대화 검색</h2>

      <div className="search-bar">
        <input
          type="text"
          placeholder="예: 추천하는 액티브 ETF는 어떤 것들이 있나요?"
          value={query}
          onChange={e => setQuery(e.target.value)}
          onKeyDown={e => e.key === 'Enter' && handleSearch()}
          disabled={isSearching}
        />
        <button
          className="btn-primary"
          onClick={handleSearch}
          disabled={isSearching || !query.trim()}
        >
          {isSearching ? '검색 중...' : '검색'}
        </button>
      </div>

      <div className="date-range-row">
        <span className="date-range-label">날짜 범위</span>
        <input
          type="date"
          className="date-input"
          value={startDate}
          onChange={e => setStartDate(e.target.value)}
          disabled={isSearching}
        />
        <span className="date-range-sep">~</span>
        <input
          type="date"
          className="date-input"
          value={endDate}
          onChange={e => setEndDate(e.target.value)}
          disabled={isSearching}
        />
        {(startDate || endDate) && (
          <button className="btn-small" onClick={() => { setStartDate(''); setEndDate(''); }}>
            전체
          </button>
        )}
      </div>

      {isSearching && (
        <div className="spinner-area">
          <div className="spinner" />
          <p>{SEARCH_STEPS[stepIdx]}</p>
        </div>
      )}

      {error && (
        <div className="error-box">
          <strong>검색 오류</strong>
          <p style={{ marginTop: 8, whiteSpace: 'pre-wrap' }}>{error}</p>
          {isLLMError && (
            <button className="btn-small" style={{ marginTop: 12 }} onClick={onGoToSettings}>
              ⚙️ 설정으로 이동
            </button>
          )}
        </div>
      )}

      {result && (
        <div className="result-area">
          <div className="summary-box">
            <h3>요약 답변</h3>
            <p className="summary-text">{result.summary}</p>
          </div>

          {result.sources.length > 0 && (
            <div className="sources">
              <h3>참고한 원본 대화 ({result.sources.length}개)</h3>
              {result.sources.map((chunk, i) => (
                <div key={i} className={`chunk-card ${chunk.isLeader ? 'leader' : ''}`}>
                  <div className="chunk-meta">
                    {chunk.startTime && <span className="chunk-time">{chunk.startTime}</span>}
                    {chunk.dateKey && (
                      <button
                        className="btn-goto-chat"
                        onClick={() => {
                          // startTime 형식: "YYYY-MM-DD HH:MM" → 시간 부분만 추출
                          const time = chunk.startTime?.split(' ')[1];
                          onGoToChat(chunk.dateKey, time);
                        }}
                        title={`${chunk.startTime} 대화 기록 보기`}
                      >
                        💬 기록 보기
                      </button>
                    )}
                    {chunk.isLeader && <span className="leader-badge">리더 발언 포함</span>}
                    <span className="chunk-score">유사도 {(chunk.score * 100 / 1.5).toFixed(0)}%</span>
                  </div>
                  <ChunkText text={chunk.text} leaderName={leaderName} />
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {!isSearching && !error && !result && (
        <div className="search-hint">
          <p>채팅 기록에서 자연어로 검색할 수 있습니다.</p>
          <p className="hint">예시 질문: "하이닉스에 대해 어떻게 생각하나요?" · "지방창생은 어떤 내용인가요?" · "국장과 미장은 어떤 비율로 투자해야할까요?"</p>
        </div>
      )}
    </div>
  );
}
