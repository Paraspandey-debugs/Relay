import React, { useState, useEffect } from 'react';
import { Play, Pause, X, Plus, Activity, DownloadCloud, Folder } from 'lucide-react';
import './App.css';

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
}

const API_URL = 'http://localhost:8080/api/downloads';

function App() {
  const [downloads, setDownloads] = useState<Download[]>([]);
  const [urlInput, setUrlInput] = useState('');
  const [destInput, setDestInput] = useState('');
  const [error, setError] = useState<string | null>(null);

  const [browserEntries, setBrowserEntries] = useState<{name: string, path: string, is_dir: boolean}[]>([]);
  const [showBrowser, setShowBrowser] = useState(false);

  useEffect(() => {
    fetchDownloads();
    const interval = setInterval(fetchDownloads, 1000);
    return () => clearInterval(interval);
  }, []);

  const fetchDownloads = async () => {
    try {
      const res = await fetch(API_URL);
      if (!res.ok) throw new Error('Failed to fetch downloads');
      const data = await res.json();
      setDownloads(data || []);
      setError(null);
    } catch (err: any) {
      setError("Cannot connect to Relay API at :8080. Is it running?");
    }
  };

  const fetchBrowserPath = async (path: string) => {
    try {
      const res = await fetch(`http://localhost:8080/api/browser?path=${encodeURIComponent(path)}`);
      if (res.ok) {
        const data = await res.json();
        setBrowserEntries(data.entries || []);
      }
    } catch (err) {
      console.error(err);
    }
  };

  const handleDestChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const val = e.target.value;
    setDestInput(val);
    
    const dirPart = val.substring(0, val.lastIndexOf('/') + 1) || val;
    fetchBrowserPath(dirPart);
    setShowBrowser(true);
  };

  const handleBrowserSelect = (path: string) => {
    let filename = '';
    try {
      if (urlInput) {
        const urlObj = new URL(urlInput);
        filename = urlObj.pathname.split('/').pop() || 'download.bin';
        if (!filename) filename = 'download.bin';
      }
    } catch(e) { filename = 'download.bin'; }
    
    setDestInput(path + (path.endsWith('/') ? '' : '/') + filename);
    setShowBrowser(false);
  };

  const addDownload = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!urlInput || !destInput) return;
    
    try {
      const res = await fetch(API_URL, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url: urlInput, destination: destInput }),
      });
      if (!res.ok) {
        const text = await res.text();
        throw new Error(text);
      }
      setUrlInput('');
      setDestInput('');
      fetchDownloads();
    } catch (err: any) {
      setError("Failed to add download: " + err.message);
    }
  };

  const handleAction = async (id: string, action: string) => {
    try {
      let url = `${API_URL}/${id}`;
      let method = 'DELETE';
      
      if (action !== 'delete') {
        url += `/${action}`;
        method = 'POST';
      }

      const res = await fetch(url, { method });
      if (!res.ok) {
        const text = await res.text();
        throw new Error(text);
      }
      fetchDownloads();
    } catch (err: any) {
      setError(`Failed to ${action}: ` + err.message);
    }
  };

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const formatETA = (nanos: number) => {
    if (nanos <= 0) return '-';
    const seconds = Math.floor(nanos / 1000000000);
    if (seconds > 3600) return Math.floor(seconds / 3600) + 'h ' + Math.floor((seconds % 3600) / 60) + 'm';
    if (seconds > 60) return Math.floor(seconds / 60) + 'm ' + (seconds % 60) + 's';
    return seconds + 's';
  };

  return (
    <div className="app-container">
      <header>
        <div className="logo-section">
          <h1>Relay</h1>
          <p>Next-generation parallel download manager</p>
        </div>
        <div style={{display: 'flex', gap: '1rem', color: 'var(--text-muted)'}}>
          <div style={{display: 'flex', alignItems: 'center', gap: '0.5rem'}}><Activity size={18}/> {downloads.filter(d => d.status === 'downloading').length} Active</div>
          <div style={{display: 'flex', alignItems: 'center', gap: '0.5rem'}}><DownloadCloud size={18}/> {downloads.length} Total</div>
        </div>
      </header>

      {error && (
        <div className="error-toast">
          {error}
          <button className="btn-icon" onClick={() => setError(null)}><X size={16}/></button>
        </div>
      )}

      <form className="add-download-card" onSubmit={addDownload}>
        <input 
          type="url" 
          placeholder="https://example.com/file.zip" 
          value={urlInput}
          onChange={(e) => setUrlInput(e.target.value)}
          required
        />
        <div className="input-group">
          <input 
            type="text" 
            placeholder="/path/to/destination/file.zip" 
            value={destInput}
            onChange={handleDestChange}
            onFocus={(e) => {
               setShowBrowser(true);
               const val = e.target.value;
               const dirPart = val.substring(0, val.lastIndexOf('/') + 1) || val;
               fetchBrowserPath(dirPart);
            }}
            onBlur={() => setTimeout(() => setShowBrowser(false), 200)}
            required
          />
          {showBrowser && browserEntries.length > 0 && (
            <div className="browser-dropdown">
              {browserEntries.map((entry, idx) => (
                <div 
                  key={idx} 
                  className="browser-item"
                  onClick={() => handleBrowserSelect(entry.path)}
                >
                  <Folder size={16} className="folder-icon" />
                  <span>{entry.name}/</span>
                </div>
              ))}
            </div>
          )}
        </div>
        <button type="submit"><Plus size={18}/> Add Download</button>
      </form>

      <div className="downloads-grid">
        {downloads.length === 0 ? (
          <div className="empty-state">
            <DownloadCloud size={48} style={{opacity: 0.5, marginBottom: '1rem'}}/>
            <h3>No downloads yet</h3>
            <p>Add a new download using the form above.</p>
          </div>
        ) : (
          downloads.map(d => {
            const progressRatio = d.progress?.total ? (d.progress.downloaded / d.progress.total) * 100 : 0;
            const filename = d.destination.split('/').pop() || d.destination;
            
            return (
              <div key={d.id} className={`download-card status-${d.status}`}>
                <div className="card-header">
                  <div className="file-info">
                    <span className="file-name" title={d.destination}>{filename}</span>
                    <span className="file-url" title={d.url}>{d.url}</span>
                  </div>
                  <div className="card-actions">
                    <span className={`badge ${d.status}`}>{d.status}</span>
                    {d.status === 'downloading' && (
                      <button className="btn-icon" onClick={() => handleAction(d.id, 'pause')}><Pause size={18}/></button>
                    )}
                    {(d.status === 'paused' || d.status === 'errored') && (
                      <button className="btn-icon" onClick={() => handleAction(d.id, 'resume')}><Play size={18}/></button>
                    )}
                    <button className="btn-icon delete" onClick={() => handleAction(d.id, 'delete')}><X size={18}/></button>
                  </div>
                </div>

                <div className="progress-section">
                  <div className="stats-row">
                    <span>{formatBytes(d.progress?.downloaded || 0)} / {d.progress?.total ? formatBytes(d.progress.total) : 'Unknown'}</span>
                    <span>{progressRatio.toFixed(1)}%</span>
                  </div>
                  <div className="progress-bar-bg">
                    <div className="progress-bar-fill" style={{ width: `${progressRatio}%` }}></div>
                  </div>
                  <div className="stats-row" style={{ marginTop: '0.25rem' }}>
                    <span>{d.status === 'downloading' ? `${formatBytes(d.progress?.speed_bps || 0)}/s` : '-'}</span>
                    <span>ETA: {d.status === 'downloading' ? formatETA(d.progress?.eta || 0) : '-'}</span>
                  </div>
                </div>
                {d.error && <div style={{color: 'var(--error)', fontSize: '0.85rem', marginTop: '0.5rem'}}>{d.error}</div>}
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}

export default App;
