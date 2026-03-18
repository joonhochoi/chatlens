import { useState, useEffect } from 'react';
import { ImportFiles, GetImportedFiles, OpenFileDialog, DeleteAllData, GetSettings } from '../../wailsjs/go/main/App';
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';

interface ProgressPayload { file: string; percent: number; status: string; }
interface DonePayload { total: number; }

export default function UploadPage({ onGoToSearch, onGoToSettings }: { onGoToSearch: () => void; onGoToSettings: () => void }) {
  const [selectedFiles, setSelectedFiles] = useState<string[]>([]);
  const [importedFiles, setImportedFiles] = useState<string[]>([]);
  const [leaderConfigured, setLeaderConfigured] = useState<boolean | null>(null);
  const [isProcessing, setIsProcessing] = useState(false);
  const [progress, setProgress] = useState(0);
  const [currentFile, setCurrentFile] = useState('');
  const [statusMsg, setStatusMsg] = useState('');
  const [doneTotal, setDoneTotal] = useState<number | null>(null);
  const [error, setError] = useState('');
  const [deleteConfirm, setDeleteConfirm] = useState(false);
  const [deleteMsg, setDeleteMsg] = useState('');

  useEffect(() => {
    loadImportedFiles();
    GetSettings().then(s => setLeaderConfigured(!!s.leaderName?.trim()));

    EventsOn('import:progress', (data: ProgressPayload) => {
      setProgress(data.percent);
      setCurrentFile(data.file);
      setStatusMsg(data.status);
    });
    EventsOn('import:done', (data: DonePayload) => {
      setIsProcessing(false);
      setProgress(100);
      setDoneTotal(data.total);
      setSelectedFiles([]);
      loadImportedFiles();
    });
    EventsOn('import:error', (data: { error: string }) => {
      setIsProcessing(false);
      setError(data.error);
    });

    return () => {
      EventsOff('import:progress');
      EventsOff('import:done');
      EventsOff('import:error');
    };
  }, []);

  function loadImportedFiles() {
    GetImportedFiles().then(files => setImportedFiles(files || []));
  }

  function handleSelectFiles() {
    OpenFileDialog().then(paths => {
      if (paths && paths.length > 0) {
        setSelectedFiles(paths);
        setDoneTotal(null);
        setError('');
        setProgress(0);
        setStatusMsg('');
      }
    });
  }

  function handleDeleteAll() {
    if (!deleteConfirm) {
      setDeleteConfirm(true);
      return;
    }
    DeleteAllData()
      .then(() => {
        setImportedFiles([]);
        setDeleteMsg('전체 데이터가 삭제됐습니다.');
        setTimeout(() => setDeleteMsg(''), 3000);
      })
      .catch(e => setError(String(e)))
      .finally(() => setDeleteConfirm(false));
  }

  function handleImport() {
    if (selectedFiles.length === 0) return;
    setIsProcessing(true);
    setDoneTotal(null);
    setError('');
    setProgress(0);
    setStatusMsg('시작 중...');
    setCurrentFile('');
    ImportFiles(selectedFiles);
  }

  const showProgress = isProcessing || (progress > 0 && doneTotal === null && !error);

  return (
    <div className="page">
      <h2>채팅 파일 업로드</h2>

      {leaderConfigured === false && (
        <div className="setup-notice">
          <span className="setup-notice-icon">⚙️</span>
          <div className="setup-notice-body">
            <strong>임베딩 전 설정을 먼저 완료해주세요</strong>
            <p>리더 이름과 청킹 전략이 설정되지 않으면 검색 품질이 떨어질 수 있습니다.</p>
          </div>
          <button className="btn-small" onClick={onGoToSettings}>설정으로 이동</button>
        </div>
      )}

      <div className="upload-area" onClick={!isProcessing ? handleSelectFiles : undefined}
           style={isProcessing ? { cursor: 'default', opacity: 0.6 } : {}}>
        <div className="upload-icon">📂</div>
        <p>클릭하여 채팅 파일 선택 (.txt)</p>
        <span className="hint">카카오톡 내보내기 파일을 여러 개 동시에 선택할 수 있습니다.</span>
      </div>

      {selectedFiles.length > 0 && !isProcessing && (
        <div className="selected-files">
          <p className="label">선택된 파일 ({selectedFiles.length}개):</p>
          <ul>
            {selectedFiles.map(f => (
              <li key={f}>{f.split(/[/\\]/).pop()}</li>
            ))}
          </ul>
          <button className="btn-primary" onClick={handleImport}>
            데이터 처리 시작
          </button>
        </div>
      )}

      {showProgress && (
        <div className="progress-area">
          <div className="progress-bar-track">
            <div className="progress-bar-fill" style={{ width: `${progress}%` }} />
          </div>
          <p className="status-msg">
            {currentFile && <span className="progress-file">{currentFile}</span>}
            {statusMsg}
          </p>
        </div>
      )}

      {error && (
        <div className="error-box" style={{ maxWidth: 560, whiteSpace: 'pre-wrap' }}>
          <strong>오류 발생</strong>
          <p style={{ marginTop: 8 }}>{error}</p>
          <button className="btn-small" style={{ marginTop: 12 }} onClick={() => setError('')}>닫기</button>
        </div>
      )}

      {doneTotal !== null && !error && (
        <div className="done-box">
          <p>✓ 처리 완료! 총 <strong>{doneTotal}개</strong>의 대화 청크가 저장됐습니다.</p>
          <button className="btn-primary" onClick={onGoToSearch}>검색하러 가기 →</button>
        </div>
      )}

      {importedFiles.length > 0 && (
        <div className="imported-list">
          <p className="label">처리된 파일 ({importedFiles.length}개):</p>
          <ul>
            {importedFiles.map(f => (
              <li key={f} className="imported-item">✓ {f}</li>
            ))}
          </ul>
        </div>
      )}

      <div className="delete-data-section">
        {deleteMsg && <span className="save-msg">{deleteMsg}</span>}
        {!deleteConfirm ? (
          <button className="btn-danger" onClick={handleDeleteAll} disabled={isProcessing}>
            전체 데이터 삭제
          </button>
        ) : (
          <div className="delete-confirm-row">
            <span className="delete-confirm-msg">정말 삭제하시겠습니까? 재임포트가 필요합니다.</span>
            <button className="btn-danger" onClick={handleDeleteAll}>확인</button>
            <button className="btn-small" onClick={() => setDeleteConfirm(false)}>취소</button>
          </div>
        )}
      </div>
    </div>
  );
}
