import { useState, useEffect } from 'react';
import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime';
import { GetImportedFiles } from '../wailsjs/go/main/App';
import SettingsPage from './pages/SettingsPage';
import UploadPage from './pages/UploadPage';
import SearchPage from './pages/SearchPage';
import KeywordPage from './pages/KeywordPage';
import ChatPage from './pages/ChatPage';
import './App.css';

type Page = 'upload' | 'search' | 'keyword' | 'chat' | 'settings';

export default function App() {
  const [page, setPage] = useState<Page>('upload');
  const [appError, setAppError] = useState('');
  const [chatDate, setChatDate] = useState<string | undefined>();
  const [chatTime, setChatTime] = useState<string | undefined>();

  useEffect(() => {
    EventsOn('app:error', (data: { error: string }) => setAppError(data.error));
    // 이미 임포트된 데이터가 있으면 AI 검색 화면으로 시작
    GetImportedFiles().then(files => {
      if (files && files.length > 0) setPage('search');
    });
    return () => EventsOff('app:error');
  }, []);

  function goToChat(date?: string, time?: string) {
    setChatDate(date);
    setChatTime(time);
    setPage('chat');
  }

  return (
    <div id="app-root">
      <nav className="sidebar">
        <div className="app-title">ChatLens</div>
        <button
          className={`nav-btn ${page === 'upload' ? 'active' : ''}`}
          onClick={() => setPage('upload')}
        >
          📂 업로드
        </button>
        <button
          className={`nav-btn ${page === 'search' ? 'active' : ''}`}
          onClick={() => setPage('search')}
        >
          🤖 AI 검색
        </button>
        <button
          className={`nav-btn ${page === 'keyword' ? 'active' : ''}`}
          onClick={() => setPage('keyword')}
        >
          🔍 키워드
        </button>
        <button
          className={`nav-btn ${page === 'chat' ? 'active' : ''}`}
          onClick={() => goToChat()}
        >
          💬 기록
        </button>
        <button
          className={`nav-btn ${page === 'settings' ? 'active' : ''}`}
          onClick={() => setPage('settings')}
        >
          ⚙️ 설정
        </button>
        <div className="app-version">v0.0.2</div>
      </nav>

      <main className="content">
        {appError && (
          <div className="app-error-banner">
            <span className="app-error-icon">⚠️</span>
            <div>
              <strong>초기화 오류</strong>
              <pre className="app-error-msg">{appError}</pre>
            </div>
            <button className="app-error-close" onClick={() => setAppError('')}>✕</button>
          </div>
        )}
        {/* 모든 페이지를 항상 마운트 — CSS show/hide로 상태 보존 */}
        <div style={{ display: page === 'upload' ? 'contents' : 'none' }}>
          <UploadPage onGoToSearch={() => setPage('search')} onGoToSettings={() => setPage('settings')} />
        </div>
        <div style={{ display: page === 'search' ? 'contents' : 'none' }}>
          <SearchPage onGoToSettings={() => setPage('settings')} onGoToChat={goToChat} />
        </div>
        <div style={{ display: page === 'keyword' ? 'contents' : 'none' }}>
          <KeywordPage onGoToChat={goToChat} />
        </div>
        <div style={{ display: page === 'chat' ? 'contents' : 'none' }}>
          <ChatPage
            initialDate={chatDate}
            initialTime={chatTime}
            onDateConsumed={() => { setChatDate(undefined); setChatTime(undefined); }}
          />
        </div>
        <div style={{ display: page === 'settings' ? 'contents' : 'none' }}>
          <SettingsPage />
        </div>
      </main>
    </div>
  );
}
