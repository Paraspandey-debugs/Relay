import { useState, useEffect, useRef, useCallback, useMemo } from 'react';
import {
  Plus, Pause, Play, Trash2, DownloadCloud,
  Activity, FolderOpen, X, Zap, HardDrive,
  Clock, Layers, ArrowDownCircle, Wifi, Settings, Save
} from 'lucide-react';
import './App.css';

/* ═══════════════════════════════════════════════════════
   Types
   ═══════════════════════════════════════════════════════ */
interface Progress {
  downloaded: number;
  total: number;
  speed_bps: number;
  eta: number;
  workers: number;
  retries: number;
}

interface Download {
  id: string;
  url: string;
  destination: string;
  status: string;
  progress: Progress;
  error?: string;
  created_at?: string;
  started_at?: string;
  completed_at?: string;
}

interface SSEEvent {
  type: string;
  id: string;
  status: string;
  progress?: Progress;
  error?: string;
  at: string;
}

/* ═══════════════════════════════════════════════════════
   Helpers
   ═══════════════════════════════════════════════════════ */
const API = '/api';

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

function formatSpeed(bps: number): string {
  if (!bps || bps <= 0) return '—';
  return formatBytes(bps) + '/s';
}

function formatETA(nanos: number): string {
  if (!nanos || nanos <= 0) return '—';
  const seconds = Math.floor(nanos / 1e9);
  if (seconds > 3600) return Math.floor(seconds / 3600) + 'h ' + Math.floor((seconds % 3600) / 60) + 'm';
  if (seconds > 60) return Math.floor(seconds / 60) + 'm ' + (seconds % 60) + 's';
  return seconds + 's';
}

function filename(path: string): string {
  return path.split('/').pop() || path;
}

/* ═══════════════════════════════════════════════════════
   Speed Graph (Canvas)
   ═══════════════════════════════════════════════════════ */
function SpeedGraph({ history, peak }: { history: number[]; peak: number }) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    const container = containerRef.current;
    if (!canvas || !container) return;

    const rect = container.getBoundingClientRect();
    const dpr = window.devicePixelRatio || 1;
    canvas.width = rect.width * dpr;
    canvas.height = rect.height * dpr;

    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    ctx.scale(dpr, dpr);

    const w = rect.width;
    const h = rect.height;
    const data = history;
    const maxVal = peak > 0 ? peak * 1.2 : 1000000;

    // Clear
    ctx.clearRect(0, 0, w, h);

    // Grid lines
    ctx.strokeStyle = 'rgba(255,255,255,0.04)';
    ctx.lineWidth = 1;
    for (let i = 1; i < 4; i++) {
      const y = (h / 4) * i;
      ctx.beginPath();
      ctx.moveTo(0, y);
      ctx.lineTo(w, y);
      ctx.stroke();
    }

    if (data.length < 2) return;

    // Build path
    const points: [number, number][] = data.map((v, i) => {
      const x = (i / (data.length - 1)) * w;
      const y = h - (v / maxVal) * h;
      return [x, Math.max(2, Math.min(h - 2, y))];
    });

    // Area fill gradient
    const grad = ctx.createLinearGradient(0, 0, 0, h);
    grad.addColorStop(0, 'rgba(99, 102, 241, 0.35)');
    grad.addColorStop(0.5, 'rgba(99, 102, 241, 0.1)');
    grad.addColorStop(1, 'rgba(99, 102, 241, 0)');

    ctx.beginPath();
    ctx.moveTo(points[0][0], h);
    for (const [x, y] of points) ctx.lineTo(x, y);
    ctx.lineTo(points[points.length - 1][0], h);
    ctx.closePath();
    ctx.fillStyle = grad;
    ctx.fill();

    // Line
    const lineGrad = ctx.createLinearGradient(0, 0, w, 0);
    lineGrad.addColorStop(0, '#6366f1');
    lineGrad.addColorStop(1, '#a78bfa');

    ctx.beginPath();
    ctx.moveTo(points[0][0], points[0][1]);
    for (let i = 1; i < points.length; i++) {
      const [x, y] = points[i];
      const [px, py] = points[i - 1];
      const cx = (px + x) / 2;
      ctx.bezierCurveTo(cx, py, cx, y, x, y);
    }
    ctx.strokeStyle = lineGrad;
    ctx.lineWidth = 2;
    ctx.stroke();

    // Current value dot
    if (points.length > 0) {
      const last = points[points.length - 1];
      ctx.beginPath();
      ctx.arc(last[0], last[1], 3, 0, Math.PI * 2);
      ctx.fillStyle = '#a78bfa';
      ctx.fill();
      ctx.beginPath();
      ctx.arc(last[0], last[1], 6, 0, Math.PI * 2);
      ctx.fillStyle = 'rgba(167, 139, 250, 0.2)';
      ctx.fill();
    }
  }, [history, peak]);

  return (
    <div className="graph-canvas-container" ref={containerRef}>
      <canvas ref={canvasRef} />
    </div>
  );
}

/* ═══════════════════════════════════════════════════════
   Chunk Map
   ═══════════════════════════════════════════════════════ */
function ChunkMap({ downloaded, total, workers }: { downloaded: number; total: number; workers: number }) {
  const numBlocks = 40;
  if (total <= 0) return null;

  const ratio = downloaded / total;
  const completedBlocks = Math.floor(ratio * numBlocks);
  const activeBlocks = Math.min(workers, numBlocks - completedBlocks);

  const blocks = [];
  for (let i = 0; i < numBlocks; i++) {
    let cls = 'chunk-block empty';
    if (i < completedBlocks) cls = 'chunk-block done';
    else if (i < completedBlocks + activeBlocks) cls = 'chunk-block active';
    blocks.push(<div key={i} className={cls} />);
  }

  return <div className="chunk-map">{blocks}</div>;
}

/* ═══════════════════════════════════════════════════════
   Main App
   ═══════════════════════════════════════════════════════ */
export default function App() {
  const [downloads, setDownloads] = useState<Download[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [tab, setTab] = useState<'all' | 'active' | 'queued' | 'done'>('all');
  const [urlInput, setUrlInput] = useState('');
  const [destInput, setDestInput] = useState('');
  const [toast, setToast] = useState<{ msg: string; type: 'error' | 'success' } | null>(null);
  const [connected, setConnected] = useState(false);
  const [speedHistory, setSpeedHistory] = useState<number[]>([]);
  const [peakSpeed, setPeakSpeed] = useState(0);

  const [browserEntries, setBrowserEntries] = useState<{ name: string; path: string; is_dir: boolean }[]>([]);
  const [showBrowser, setShowBrowser] = useState(false);

  // ─── Settings state ───
  const [showSettings, setShowSettings] = useState(false);
  const [daemonConfig, setDaemonConfig] = useState<{
    concurrency: number;
    workers: number;
    state_path: string;
    log_path: string;
    api_port: number;
    headless: boolean;
    open_web: boolean;
    theme: string;
    refresh_ms: number;
    cleanup: boolean;
  } | null>(null);
  const [editConcurrency, setEditConcurrency] = useState(3);
  const [editWorkers, setEditWorkers] = useState(4);
  const [savingConfig, setSavingConfig] = useState(false);

  const toastTimer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const downloadsRef = useRef(downloads);
  downloadsRef.current = downloads;

  // ─── Toast helper ───
  const showToast = useCallback((msg: string, type: 'error' | 'success' = 'error') => {
    setToast({ msg, type });
    if (toastTimer.current) clearTimeout(toastTimer.current);
    toastTimer.current = setTimeout(() => setToast(null), 5000);
  }, []);

  // ─── Fetch full state ───
  const fetchAll = useCallback(async () => {
    try {
      const res = await fetch(`${API}/downloads`);
      if (!res.ok) throw new Error('Failed');
      const data: Download[] = await res.json();
      setDownloads(data || []);
      setConnected(true);
    } catch {
      setConnected(false);
    }
  }, []);

  // ─── SSE for real-time updates ───
  useEffect(() => {
    fetchAll();
    let sse: EventSource | null = null;

    const connect = () => {
      sse = new EventSource(`${API}/events`);

      sse.addEventListener('message', (e) => {
        try {
          const evt: SSEEvent = JSON.parse(e.data);
          setConnected(true);

          setDownloads(prev => {
            if (evt.type === 'removed') {
              return prev.filter(d => d.id !== evt.id);
            }

            const idx = prev.findIndex(d => d.id === evt.id);
            if (idx >= 0) {
              const updated = [...prev];
              updated[idx] = {
                ...updated[idx],
                status: evt.status,
                ...(evt.progress ? { progress: evt.progress } : {}),
                ...(evt.error ? { error: evt.error } : {}),
              };
              return updated;
            }

            // New download — fetch full state
            fetchAll();
            return prev;
          });
        } catch { /* skip */ }
      });

      sse.onerror = () => {
        setConnected(false);
        sse?.close();
        setTimeout(connect, 3000);
      };
    };

    connect();
    return () => { sse?.close(); };
  }, [fetchAll]);

  // ─── Fetch daemon config ───
  const fetchConfig = useCallback(async () => {
    try {
      const res = await fetch(`${API}/config`);
      if (!res.ok) throw new Error('Failed');
      const data = await res.json();
      setDaemonConfig(data);
      setEditConcurrency(data.concurrency);
      setEditWorkers(data.workers);
    } catch { /* ignore */ }
  }, []);

  useEffect(() => {
    fetchConfig();
  }, [fetchConfig]);

  // ─── Save daemon config ───
  const saveConfig = async () => {
    setSavingConfig(true);
    try {
      const res = await fetch(`${API}/config`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          concurrency: editConcurrency,
          workers: editWorkers,
        }),
      });
      if (!res.ok) throw new Error(await res.text());
      showToast('Settings applied', 'success');
      fetchConfig();
    } catch (err: unknown) {
      showToast('Failed to save: ' + (err instanceof Error ? err.message : String(err)));
    } finally {
      setSavingConfig(false);
    }
  };

  // ─── Speed history tracker ───
  useEffect(() => {
    const interval = setInterval(() => {
      const totalSpeed = downloadsRef.current
        .filter(d => d.status === 'downloading')
        .reduce((sum, d) => sum + (d.progress?.speed_bps || 0), 0);

      setSpeedHistory(prev => {
        const next = [...prev, totalSpeed].slice(-60);
        return next;
      });
      setPeakSpeed(prev => Math.max(prev, totalSpeed));
    }, 1000);

    return () => clearInterval(interval);
  }, []);

  // ─── Browser ───
  const fetchBrowser = async (path: string) => {
    try {
      const res = await fetch(`${API}/browser?path=${encodeURIComponent(path)}`);
      if (res.ok) {
        const data = await res.json();
        setBrowserEntries(data.entries || []);
      }
    } catch { /* ignore */ }
  };

  // ─── Actions ───
  const addDownload = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!urlInput || !destInput) return;
    try {
      const res = await fetch(`${API}/downloads`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url: urlInput, destination: destInput }),
      });
      if (!res.ok) throw new Error(await res.text());
      setUrlInput('');
      setDestInput('');
      showToast('Download added', 'success');
      fetchAll();
    } catch (err: unknown) {
      showToast('Failed: ' + (err instanceof Error ? err.message : String(err)));
    }
  };

  const action = async (id: string, act: string) => {
    try {
      const url = act === 'delete' ? `${API}/downloads/${id}` : `${API}/downloads/${id}/${act}`;
      const method = act === 'delete' ? 'DELETE' : 'POST';
      const res = await fetch(url, { method });
      if (!res.ok) throw new Error(await res.text());
      if (act === 'delete' && selectedId === id) setSelectedId(null);
      fetchAll();
    } catch (err: unknown) {
      showToast(`Failed to ${act}: ` + (err instanceof Error ? err.message : String(err)));
    }
  };

  // ─── Derived state ───
  const activeCount = downloads.filter(d => d.status === 'downloading').length;
  const queuedCount = downloads.filter(d => d.status === 'queued' || d.status === 'paused').length;
  const doneCount = downloads.filter(d => d.status === 'completed').length;
  const totalSpeed = downloads.filter(d => d.status === 'downloading').reduce((s, d) => s + (d.progress?.speed_bps || 0), 0);
  const totalDownloaded = downloads.reduce((s, d) => s + (d.progress?.downloaded || 0), 0);

  const selected = useMemo(() => downloads.find(d => d.id === selectedId) || null, [downloads, selectedId]);

  const filtered = useMemo(() => {
    if (tab === 'all') return downloads;
    if (tab === 'active') return downloads.filter(d => d.status === 'downloading');
    if (tab === 'queued') return downloads.filter(d => d.status === 'queued' || d.status === 'paused' || d.status === 'errored');
    return downloads.filter(d => d.status === 'completed');
  }, [downloads, tab]);

  return (
    <div className="app">
      {/* ─── Header ─── */}
      <header className="header">
        <div className="header-left">
          <span className="logo">Relay</span>
          <span className="version-badge">v1.0.0</span>
          <div className={`connection-dot ${connected ? '' : 'disconnected'}`} />
        </div>
        <div className="header-stats">
          <div className="stat-pill">
            <span className="stat-dot active" />
            <span className="stat-value">{activeCount}</span> active
          </div>
          <div className="stat-pill">
            <span className="stat-dot queued" />
            <span className="stat-value">{queuedCount}</span> queued
          </div>
          <div className="stat-pill">
            <span className="stat-dot done" />
            <span className="stat-value">{doneCount}</span> done
          </div>
          <div className="stat-pill">
            <span className="stat-dot speed" />
            <span className="stat-value">{formatSpeed(totalSpeed)}</span>
          </div>
          <button
            className={`settings-toggle ${showSettings ? 'active' : ''}`}
            onClick={() => setShowSettings(!showSettings)}
            title="Daemon Settings"
          >
            <Settings size={16} />
          </button>
        </div>
      </header>

      {/* ─── Add Bar ─── */}
      <form className="add-bar" onSubmit={addDownload}>
        <input
          type="url"
          placeholder="Paste download URL..."
          value={urlInput}
          onChange={e => setUrlInput(e.target.value)}
          required
        />
        <div className="input-group">
          <input
            type="text"
            placeholder="Save to path..."
            value={destInput}
            onChange={e => {
              setDestInput(e.target.value);
              const dir = e.target.value.substring(0, e.target.value.lastIndexOf('/') + 1) || e.target.value;
              fetchBrowser(dir);
              setShowBrowser(true);
            }}
            onFocus={e => {
              setShowBrowser(true);
              const dir = e.target.value.substring(0, e.target.value.lastIndexOf('/') + 1) || e.target.value;
              fetchBrowser(dir);
            }}
            onBlur={() => setTimeout(() => setShowBrowser(false), 200)}
            required
          />
          {showBrowser && browserEntries.length > 0 && (
            <div className="browser-dropdown">
              {browserEntries.map((entry, i) => (
                <div
                  key={i}
                  className="browser-item"
                  onClick={() => {
                    let fname = 'download.bin';
                    try { fname = new URL(urlInput).pathname.split('/').pop() || fname; } catch { /* ok */ }
                    setDestInput(entry.path + (entry.path.endsWith('/') ? '' : '/') + fname);
                    setShowBrowser(false);
                  }}
                >
                  <FolderOpen size={14} />
                  <span>{entry.name}/</span>
                </div>
              ))}
            </div>
          )}
        </div>
        <button type="submit" className="btn-add"><Plus size={16} /> Add</button>
      </form>

      {/* ─── Main Grid ─── */}
      <div className="main">
        {/* Left: Downloads List */}
        <div className="downloads-panel">
          <div className="section-header">
            <span className="section-title">Downloads ({filtered.length})</span>
            <div className="tab-row">
              {(['all', 'active', 'queued', 'done'] as const).map(t => (
                <button
                  key={t}
                  className={`tab-btn ${tab === t ? 'active' : ''}`}
                  onClick={() => setTab(t)}
                >
                  {t.charAt(0).toUpperCase() + t.slice(1)}
                </button>
              ))}
            </div>
          </div>

          {filtered.length === 0 ? (
            <div className="empty-list">
              <DownloadCloud size={40} />
              <p>No downloads{tab !== 'all' ? ` in "${tab}"` : ''}. Add one above.</p>
            </div>
          ) : (
            <div className="download-list">
              {filtered.map(d => {
                const pct = d.progress?.total ? (d.progress.downloaded / d.progress.total) * 100 : 0;
                return (
                  <div
                    key={d.id}
                    className={`dl-item ${d.status} ${selectedId === d.id ? 'selected' : ''}`}
                    onClick={() => setSelectedId(d.id)}
                  >
                    <div className={`dl-status-indicator ${d.status}`} />
                    <div className="dl-info">
                      <span className="dl-name">{filename(d.destination)}</span>
                      <div className="dl-meta">
                        {d.status === 'downloading' && <span>{formatSpeed(d.progress?.speed_bps || 0)}</span>}
                        {d.status === 'downloading' && d.progress?.total > 0 && <span>{pct.toFixed(0)}%</span>}
                        {d.status === 'completed' && <span>{formatBytes(d.progress?.total || d.progress?.downloaded || 0)}</span>}
                        {d.status === 'queued' && <span>Waiting</span>}
                        {d.status === 'paused' && <span>Paused</span>}
                        {d.status === 'errored' && <span style={{ color: 'var(--error)' }}>Error</span>}
                      </div>
                    </div>
                    {(d.status === 'downloading' || d.status === 'completed') && (
                      <div className="dl-progress-mini">
                        <div className="dl-progress-mini-fill" style={{ width: `${d.status === 'completed' ? 100 : pct}%` }} />
                      </div>
                    )}
                    <div className="dl-actions">
                      {d.status === 'downloading' && (
                        <button className="btn-icon" onClick={e => { e.stopPropagation(); action(d.id, 'pause'); }}>
                          <Pause size={14} />
                        </button>
                      )}
                      {(d.status === 'paused' || d.status === 'errored') && (
                        <button className="btn-icon" onClick={e => { e.stopPropagation(); action(d.id, 'resume'); }}>
                          <Play size={14} />
                        </button>
                      )}
                      <button className="btn-icon danger" onClick={e => { e.stopPropagation(); action(d.id, 'delete'); }}>
                        <Trash2 size={14} />
                      </button>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>

        {/* Right: Sidebar */}
        <div className="sidebar">
          {/* Speed Graph */}
          <div className="graph-panel">
            <div className="graph-header">
              <span className="graph-title"><Activity size={12} /> Network Activity</span>
              <span className="graph-speed">{formatSpeed(totalSpeed)}</span>
            </div>
            <SpeedGraph history={speedHistory} peak={peakSpeed} />
            <div className="graph-stats-row">
              <div className="graph-stat">
                <span className="graph-stat-label">Peak</span>
                <span className="graph-stat-value">{formatSpeed(peakSpeed)}</span>
              </div>
              <div className="graph-stat">
                <span className="graph-stat-label">Downloaded</span>
                <span className="graph-stat-value">{formatBytes(totalDownloaded)}</span>
              </div>
              <div className="graph-stat">
                <span className="graph-stat-label">Workers</span>
                <span className="graph-stat-value">
                  {downloads.filter(d => d.status === 'downloading').reduce((s, d) => s + (d.progress?.workers || 0), 0)}
                </span>
              </div>
            </div>
          </div>

          {/* Settings Panel */}
          {showSettings && (
            <div className="settings-panel">
              <div className="settings-header">
                <span className="settings-title"><Settings size={12} /> Daemon Settings</span>
                <button className="settings-close" onClick={() => setShowSettings(false)}><X size={14} /></button>
              </div>
              <div className="settings-body">
                <div className="settings-info">
                  These settings are applied immediately and persisted to disk.
                  Some values (state path, log path, API port) require a daemon restart to take effect.
                </div>
                <div className="settings-field">
                  <label className="settings-label">Max Concurrent Downloads</label>
                  <input
                    type="number"
                    className="settings-input"
                    min={1}
                    max={20}
                    value={editConcurrency}
                    onChange={e => setEditConcurrency(Math.max(1, parseInt(e.target.value) || 1))}
                  />
                  <span className="settings-hint">Controls how many downloads run in parallel. Lower to reduce server load.</span>
                </div>
                <div className="settings-field">
                  <label className="settings-label">Default Workers Per Download</label>
                  <input
                    type="number"
                    className="settings-input"
                    min={0}
                    max={8}
                    value={editWorkers}
                    onChange={e => setEditWorkers(Math.max(0, Math.min(8, parseInt(e.target.value) || 0)))}
                  />
                  <span className="settings-hint">Parallel connections per download (0 = use built-in default). Capped at 8.</span>
                </div>
                {daemonConfig && (
                  <div className="settings-readonly">
                    <div className="settings-ro-row"><span>State Path</span><code>{daemonConfig.state_path}</code></div>
                    <div className="settings-ro-row"><span>Log Path</span><code>{daemonConfig.log_path || '(stderr)'}</code></div>
                    <div className="settings-ro-row"><span>API Port</span><code>{daemonConfig.api_port}</code></div>
                    <div className="settings-ro-row"><span>Headless</span><code>{daemonConfig.headless ? 'yes' : 'no'}</code></div>
                  </div>
                )}
                <button
                  className="settings-save-btn"
                  onClick={saveConfig}
                  disabled={savingConfig}
                >
                  <Save size={14} /> {savingConfig ? 'Saving...' : 'Apply Settings'}
                </button>
              </div>
            </div>
          )}

          {/* Detail Panel */}
          <div className="detail-panel">
            {!selected ? (
              <div className="detail-empty">
                <Layers size={36} />
                <p>Select a download to view details</p>
              </div>
            ) : (
              <>
                <div className="detail-header">
                  <span className="detail-filename">{filename(selected.destination)}</span>
                  <span className={`detail-badge ${selected.status}`}>{selected.status}</span>
                </div>

                {/* Progress */}
                <div className={`detail-progress-section ${selected.status}`}>
                  <div className="detail-progress-header">
                    <span className="size">
                      {formatBytes(selected.progress?.downloaded || 0)} / {selected.progress?.total ? formatBytes(selected.progress.total) : 'Unknown'}
                    </span>
                    <span className="pct">
                      {selected.progress?.total ? ((selected.progress.downloaded / selected.progress.total) * 100).toFixed(1) + '%' : '—'}
                    </span>
                  </div>
                  <div className="detail-progress-bar">
                    <div
                      className="detail-progress-fill"
                      style={{ width: `${selected.status === 'completed' ? 100 : (selected.progress?.total ? (selected.progress.downloaded / selected.progress.total) * 100 : 0)}%` }}
                    />
                  </div>
                  {selected.status === 'downloading' && selected.progress?.total > 0 && (
                    <ChunkMap
                      downloaded={selected.progress.downloaded}
                      total={selected.progress.total}
                      workers={selected.progress?.workers || 1}
                    />
                  )}
                </div>

                {/* Stats */}
                <div className="detail-stats-grid">
                  <div className="detail-stat">
                    <span className="detail-stat-label"><Zap size={10} /> Speed</span>
                    <span className="detail-stat-value">{formatSpeed(selected.progress?.speed_bps || 0)}</span>
                  </div>
                  <div className="detail-stat">
                    <span className="detail-stat-label"><Clock size={10} /> ETA</span>
                    <span className="detail-stat-value">{formatETA(selected.progress?.eta || 0)}</span>
                  </div>
                  <div className="detail-stat">
                    <span className="detail-stat-label"><Wifi size={10} /> Workers</span>
                    <span className="detail-stat-value">{selected.progress?.workers || 0}</span>
                  </div>
                  <div className="detail-stat">
                    <span className="detail-stat-label"><HardDrive size={10} /> Size</span>
                    <span className="detail-stat-value">{selected.progress?.total ? formatBytes(selected.progress.total) : '—'}</span>
                  </div>
                </div>

                {/* URL */}
                <div className="detail-url">{selected.url}</div>

                {/* Error */}
                {selected.error && (
                  <div className="detail-error">{selected.error}</div>
                )}

                {/* Action Buttons */}
                <div className="detail-actions-bar">
                  {selected.status === 'downloading' && (
                    <button className="detail-action-btn" onClick={() => action(selected.id, 'pause')}>
                      <Pause size={14} /> Pause
                    </button>
                  )}
                  {(selected.status === 'paused' || selected.status === 'errored') && (
                    <button className="detail-action-btn primary" onClick={() => action(selected.id, 'resume')}>
                      <Play size={14} /> Resume
                    </button>
                  )}
                  <button className="detail-action-btn danger" onClick={() => action(selected.id, 'delete')}>
                    <Trash2 size={14} /> Remove
                  </button>
                </div>
              </>
            )}
          </div>
        </div>
      </div>

      {/* Toast */}
      {toast && (
        <div className={`toast ${toast.type}`}>
          {toast.type === 'error' ? <X size={16} /> : <ArrowDownCircle size={16} />}
          {toast.msg}
          <button className="toast-close" onClick={() => setToast(null)}><X size={14} /></button>
        </div>
      )}
    </div>
  );
}
