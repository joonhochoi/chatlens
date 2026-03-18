import { useState, useEffect } from 'react';
import { GetSettings, SaveSettings, CheckOllama } from '../../wailsjs/go/main/App';
import { config } from '../../wailsjs/go/models';
type Settings = config.Settings;

export default function SettingsPage() {
  const [settings, setSettings] = useState<Settings>({
    leaderName: '',
    ignoreKeywords: ['사진', '동영상', '이모티콘'],
    llmProvider: 'ollama',
    apiKey: '',
    ollamaModel: 'llama3:latest',
    embeddingModel: 'nomic-embed-text',
    searchTopK: 5,
    maxChunkMessages: 40,
    chunkOverlap: 3,
    useLeaderMicro: false,
    useSemanticChunk: false,
    semanticThreshold: 0.65,
  });
  const [keywordInput, setKeywordInput] = useState('');
  const [ollamaStatus, setOllamaStatus] = useState<'unknown' | 'connected' | 'disconnected'>('unknown');
  const [saveMsg, setSaveMsg] = useState('');

  useEffect(() => {
    GetSettings().then(s => setSettings(s as Settings));
  }, []);

  function handleSave() {
    SaveSettings(settings).then(() => {
      setSaveMsg('저장됐습니다.');
      setTimeout(() => setSaveMsg(''), 2000);
    });
  }

  function handleCheckOllama() {
    setOllamaStatus('unknown');
    CheckOllama().then(ok => setOllamaStatus(ok ? 'connected' : 'disconnected'));
  }

  function addKeyword() {
    const kw = keywordInput.trim();
    if (!kw || settings.ignoreKeywords.includes(kw)) return;
    setSettings(s => ({ ...s, ignoreKeywords: [...s.ignoreKeywords, kw] }));
    setKeywordInput('');
  }

  function removeKeyword(kw: string) {
    setSettings(s => ({ ...s, ignoreKeywords: s.ignoreKeywords.filter(k => k !== kw) }));
  }

  return (
    <div className="page">
      <h2>설정</h2>

      <div className="form-group">
        <label>리더(타겟 화자) 이름</label>
        <input
          type="text"
          placeholder="예: 방장 리더"
          value={settings.leaderName}
          onChange={e => setSettings(s => ({ ...s, leaderName: e.target.value }))}
        />
        <span className="hint">채팅 파일에 표시되는 화자명과 정확히 일치해야 합니다.</span>
        <label className="toggle-label" style={{ marginTop: 12 }}>
          <input
            type="checkbox"
            checked={settings.useLeaderMicro}
            disabled={!settings.leaderName.trim()}
            onChange={e => setSettings(s => ({ ...s, useLeaderMicro: e.target.checked }))}
          />
          <span>리더 발언 마이크로 청킹</span>
          {!settings.leaderName.trim() && <span className="hint" style={{ marginLeft: 8 }}>(리더 이름 입력 후 활성화)</span>}
        </label>
        <span className="hint">리더의 발언 세션마다 별도 청크로 분리 — 리더 검색 정확도 향상</span>
      </div>

      <div className="form-group">
        <label>청킹 전략</label>
        <div className="chunking-strategy-box">
          <div className="chunking-row">
            <span className="chunking-label">최대 청크 크기</span>
            <div className="radio-group">
              {[20, 30, 40, 60].map(n => (
                <label key={n} className="radio-label">
                  <input
                    type="radio"
                    name="maxChunkMessages"
                    checked={settings.maxChunkMessages === n}
                    onChange={() => setSettings(s => ({ ...s, maxChunkMessages: n }))}
                  />
                  {n}개
                </label>
              ))}
            </div>
            <span className="hint">청크당 최대 메시지 수. 작을수록 검색 정확도↑ 청크 수↑</span>
          </div>

          <div className="chunking-row">
            <span className="chunking-label">청크 오버랩</span>
            <div className="radio-group">
              {[{ v: 0, label: '없음' }, { v: 3, label: '3개' }, { v: 5, label: '5개' }].map(({ v, label }) => (
                <label key={v} className="radio-label">
                  <input
                    type="radio"
                    name="chunkOverlap"
                    checked={settings.chunkOverlap === v}
                    onChange={() => setSettings(s => ({ ...s, chunkOverlap: v }))}
                  />
                  {label}
                </label>
              ))}
            </div>
            <span className="hint">인접 청크 경계에서 공유할 메시지 수 — 경계 근처 검색 품질 개선</span>
          </div>

          <div className="chunking-row">
            <label className="toggle-label">
              <input
                type="checkbox"
                checked={settings.useSemanticChunk}
                onChange={e => setSettings(s => ({ ...s, useSemanticChunk: e.target.checked }))}
              />
              <span>의미 기반 분할</span>
            </label>
            <span className="hint">주제가 바뀌는 지점을 임베딩으로 감지해 추가 분할. 임포트 속도가 느려집니다.</span>
            {settings.useSemanticChunk && (
              <div style={{ marginTop: 8 }}>
                <span className="chunking-label">유사도 임계값</span>
                <div className="radio-group" style={{ marginTop: 4 }}>
                  {[0.55, 0.60, 0.65, 0.70, 0.75].map(v => (
                    <label key={v} className="radio-label">
                      <input
                        type="radio"
                        name="semanticThreshold"
                        checked={Math.abs(settings.semanticThreshold - v) < 0.001}
                        onChange={() => setSettings(s => ({ ...s, semanticThreshold: v }))}
                      />
                      {v.toFixed(2)}
                    </label>
                  ))}
                </div>
                <span className="hint">낮을수록 더 자주 분할. 0.65 권장</span>
              </div>
            )}
          </div>
        </div>
        <span className="hint" style={{ marginTop: 8, display: 'block' }}>
          ⚠️ 청킹 전략 변경 시 기존 데이터를 전체 삭제 후 재임포트해야 적용됩니다.
        </span>
      </div>

      <div className="form-group">
        <label>무시할 키워드</label>
        <div className="chips">
          {settings.ignoreKeywords.map(kw => (
            <span key={kw} className="chip">
              {kw}
              <button onClick={() => removeKeyword(kw)}>×</button>
            </span>
          ))}
        </div>
        <div className="keyword-input-row">
          <input
            type="text"
            placeholder="키워드 입력 후 추가"
            value={keywordInput}
            onChange={e => setKeywordInput(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && addKeyword()}
          />
          <button className="btn-small" onClick={addKeyword}>추가</button>
        </div>
      </div>

      <div className="form-group">
        <label>검색 결과 수</label>
        <div className="radio-group">
          {[3, 4, 5, 6, 7, 8].map(n => (
            <label key={n} className="radio-label">
              <input
                type="radio"
                name="searchTopK"
                value={n}
                checked={settings.searchTopK === n}
                onChange={() => setSettings(s => ({ ...s, searchTopK: n }))}
              />
              {n}개
            </label>
          ))}
        </div>
        <span className="hint">LLM에 전달할 참고 대화 수. 많을수록 더 넓은 맥락을 참고하지만 응답이 느려질 수 있습니다.</span>
      </div>

      <div className="form-group">
        <label>LLM 설정</label>
        <div className="radio-group">
          {['ollama', 'gemini', 'openai'].map(p => (
            <label key={p} className="radio-label">
              <input
                type="radio"
                name="llmProvider"
                value={p}
                checked={settings.llmProvider === p}
                onChange={() => setSettings(s => ({ ...s, llmProvider: p }))}
              />
              {p === 'ollama' ? 'Ollama (로컬)' : p === 'gemini' ? 'Google Gemini' : 'OpenAI'}
            </label>
          ))}
        </div>
      </div>

      {settings.llmProvider === 'ollama' && (
        <div className="form-group">
          <label>Ollama 모델명</label>
          <div className="inline-row">
            <input
              type="text"
              value={settings.ollamaModel}
              onChange={e => setSettings(s => ({ ...s, ollamaModel: e.target.value }))}
            />
            <button className="btn-small" onClick={handleCheckOllama}>연결 테스트</button>
            {ollamaStatus === 'connected' && <span className="status ok">✓ 연결됨</span>}
            {ollamaStatus === 'disconnected' && <span className="status err">✗ 연결 안됨</span>}
          </div>
        </div>
      )}

      {settings.llmProvider !== 'ollama' && (
        <div className="form-group">
          <label>API 키</label>
          <input
            type="password"
            placeholder="API 키 입력"
            value={settings.apiKey}
            onChange={e => setSettings(s => ({ ...s, apiKey: e.target.value }))}
          />
        </div>
      )}

      <div className="actions">
        <button className="btn-primary" onClick={handleSave}>저장</button>
        {saveMsg && <span className="save-msg">{saveMsg}</span>}
      </div>
    </div>
  );
}
