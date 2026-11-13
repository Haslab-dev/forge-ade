import React, { useState, useEffect, useRef, useCallback } from 'react';
import {
  ChevronLeft,
  ChevronRight,
  Play,
  Pause,
  RotateCcw,
  Maximize2,
  Minimize2,
  Printer,
  Presentation,
  Video,
  Layers,
  Download,
  Loader2,
  CheckCircle2,
  Film,
  Sparkles,
  X,
  FolderOpen,
  StopCircle,
  Copy
} from 'lucide-react';
import { ApiBridge } from '../../services/apiBridge';
import { showToast } from '../../lib/toast';

export interface DeckMotionViewerProps {
  filePath: string;
  fileName: string;
  htmlContent: string;
  onClose?: () => void;
}

type ViewerMode = 'deck' | 'motion' | 'auto';

export const DeckMotionViewer: React.FC<DeckMotionViewerProps> = ({
  filePath,
  fileName,
  htmlContent,
  onClose
}) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const iframeRef = useRef<HTMLIFrameElement>(null);

  // Detect mode: deck (slides) or motion (hyperframes)
  const isDeck = htmlContent.includes('class="slide') || htmlContent.includes('deck-stage') || fileName.includes('deck') || fileName.includes('slide');
  const isMotion = htmlContent.includes('data-composition-id') || htmlContent.includes('__timelines') || fileName.includes('motion') || fileName.includes('hyperframe');

  const [mode] = useState<ViewerMode>(isMotion && !isDeck ? 'motion' : 'deck');

  // Deck State
  const [activeSlide, setActiveSlide] = useState(0);
  const [slideCount, setSlideCount] = useState(1);
  const [slideLabels, setSlideLabels] = useState<string[]>([]);
  const [isThumbnailsOpen, setIsThumbnailsOpen] = useState(false);

  // Motion State
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(8);
  const [isPlaying, setIsPlaying] = useState(false);
  const [playbackSpeed, setPlaybackSpeed] = useState(1);

  // Fullscreen state
  const [isFullscreen, setIsFullscreen] = useState(false);

  // Video Export & Download State
  const [isExportModalOpen, setIsExportModalOpen] = useState(false);
  const [isRendering, setIsRendering] = useState(false);
  const [renderProgress, setRenderProgress] = useState('');
  const [renderedOutputPath, setRenderedOutputPath] = useState<string | null>(null);

  // In-Browser Recording State
  const [isRecording, setIsRecording] = useState(false);
  const [recordingSeconds, setRecordingSeconds] = useState(0);
  const mediaRecorderRef = useRef<MediaRecorder | null>(null);
  const recordingTimerRef = useRef<number | null>(null);

  // Listen for PostMessage from Iframe
  useEffect(() => {
    const handleMessage = (e: MessageEvent) => {
      if (!e.data || typeof e.data !== 'object') return;

      // Slides protocol (od:slide-state, od:deck-ready)
      if (e.data.type === 'od:slide-state') {
        if (typeof e.data.active === 'number') setActiveSlide(e.data.active);
        if (typeof e.data.count === 'number') setSlideCount(e.data.count);
        if (Array.isArray(e.data.labels)) setSlideLabels(e.data.labels);
      } else if (e.data.type === 'od:deck-ready') {
        if (typeof e.data.count === 'number') setSlideCount(e.data.count);
      }

      // Motion protocol (od:motion-state, od:motion-ready)
      if (e.data.type === 'od:motion-state') {
        if (typeof e.data.time === 'number') setCurrentTime(e.data.time);
        if (typeof e.data.duration === 'number') setDuration(e.data.duration);
        if (typeof e.data.isPlaying === 'boolean') setIsPlaying(e.data.isPlaying);
      } else if (e.data.type === 'od:motion-ready') {
        if (typeof e.data.duration === 'number') setDuration(e.data.duration);
      }
    };

    window.addEventListener('message', handleMessage);
    return () => window.removeEventListener('message', handleMessage);
  }, []);

  // PostMessage dispatch helpers
  const sendDeckCommand = useCallback((action: 'next' | 'prev' | 'first' | 'last' | 'go', index?: number) => {
    iframeRef.current?.contentWindow?.postMessage({
      type: 'od:slide',
      action,
      index
    }, '*');
  }, []);

  const sendMotionCommand = useCallback((action: 'play' | 'pause' | 'restart' | 'seek', time?: number) => {
    iframeRef.current?.contentWindow?.postMessage({
      type: 'od:motion-control',
      action,
      time
    }, '*');
  }, []);

  // Keyboard navigation
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const activeEl = document.activeElement;
      if (activeEl && (activeEl.tagName === 'INPUT' || activeEl.tagName === 'TEXTAREA')) return;

      if (mode === 'deck') {
        if (e.key === 'ArrowRight' || e.key === ' ') {
          e.preventDefault();
          sendDeckCommand('next');
        } else if (e.key === 'ArrowLeft') {
          e.preventDefault();
          sendDeckCommand('prev');
        }
      } else if (mode === 'motion') {
        if (e.key === ' ') {
          e.preventDefault();
          sendMotionCommand(isPlaying ? 'pause' : 'play');
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [mode, isPlaying, sendDeckCommand, sendMotionCommand]);

  // Fullscreen toggle
  const toggleFullscreen = () => {
    if (!containerRef.current) return;
    if (!document.fullscreenElement) {
      void containerRef.current.requestFullscreen();
      setIsFullscreen(true);
    } else {
      void document.exitFullscreen();
      setIsFullscreen(false);
    }
  };

  // Print / PDF Trigger
  const handlePrint = () => {
    iframeRef.current?.contentWindow?.print();
  };

  const formatTime = (seconds: number) => {
    const m = Math.floor(seconds / 60);
    const s = Math.floor(seconds % 60);
    const ms = Math.floor((seconds % 1) * 100);
    return `${m < 10 ? '0' : ''}${m}:${s < 10 ? '0' : ''}${s}.${ms < 10 ? '0' : ''}${ms}`;
  };

  // Stop Browser Recording
  const stopBrowserRecording = useCallback(() => {
    if (recordingTimerRef.current) {
      clearInterval(recordingTimerRef.current);
      recordingTimerRef.current = null;
    }
    if (mediaRecorderRef.current && mediaRecorderRef.current.state !== 'inactive') {
      mediaRecorderRef.current.stop();
    }
  }, []);

  // Clean up recording on unmount
  useEffect(() => {
    return () => {
      if (recordingTimerRef.current) clearInterval(recordingTimerRef.current);
      if (mediaRecorderRef.current && mediaRecorderRef.current.state !== 'inactive') {
        mediaRecorderRef.current.stop();
      }
    };
  }, []);

  // Studio Quality Render via HyperFrames Engine & Bundled FFmpeg
  const handleRenderHeadless = async () => {
    try {
      setIsRendering(true);
      setRenderProgress('Initializing HyperFrames engine & FFmpeg...');
      setRenderedOutputPath(null);

      const resolvedInput = filePath || fileName;
      const defaultOut = resolvedInput.replace(/\.(html|deck|motion)$/, '') + '.mp4';

      setRenderProgress('Rendering 60fps frames with GPU acceleration...');

      // Dynamically resolve render_motion.js: check workspace plugins, .forge/plugins, or global ~/.forge-ade/plugins
      const cmd = `export PATH="$HOME/.bun/bin:/opt/homebrew/bin:/usr/local/bin:$PATH"; RENDER_TOOL="$HOME/.forge-ade/plugins/open-design/tools/render_motion.js"; if [ -f "plugins/open-design/tools/render_motion.js" ]; then RENDER_TOOL="plugins/open-design/tools/render_motion.js"; elif [ -f ".forge/plugins/open-design/tools/render_motion.js" ]; then RENDER_TOOL=".forge/plugins/open-design/tools/render_motion.js"; fi; node "$RENDER_TOOL" --input="${resolvedInput}" --output="${defaultOut}"`;
      const execCwd = filePath && filePath.includes('/') ? filePath.substring(0, filePath.lastIndexOf('/')) : '';
      const res = await ApiBridge.executeCommand(cmd, execCwd);

      const isSuccess = res.exitCode === 0 && (res.stdout?.includes('"success": true') || !res.stderr?.includes('Error:'));
      if (isSuccess) {
        setRenderedOutputPath(defaultOut);
        setRenderProgress('');
        showToast(`Video rendered to ${defaultOut.split('/').pop()}!`, 'success');
      } else {
        throw new Error(res.stderr || res.stdout || 'Rendering engine exited with error.');
      }
    } catch (err: any) {
      console.error('Headless render error:', err);
      showToast(`Render failed: ${err?.message || 'Check terminal output'}`, 'error');
      setRenderProgress('');
    } finally {
      setIsRendering(false);
    }
  };

  // Real-Time Browser Tab Recording via MediaRecorder
  const handleStartBrowserRecording = async () => {
    try {
      if (!navigator.mediaDevices?.getDisplayMedia) {
        showToast('Screen/Tab capture is not supported in this environment.', 'warn');
        return;
      }

      setIsExportModalOpen(false);

      const stream = await navigator.mediaDevices.getDisplayMedia({
        video: { displaySurface: 'browser' },
        audio: true
      });

      const mimeType = MediaRecorder.isTypeSupported('video/mp4;codecs=avc1')
        ? 'video/mp4;codecs=avc1'
        : MediaRecorder.isTypeSupported('video/webm;codecs=vp9')
          ? 'video/webm;codecs=vp9'
          : 'video/webm';

      const recorder = new MediaRecorder(stream, { mimeType });
      const chunks: Blob[] = [];

      recorder.ondataavailable = (e) => {
        if (e.data.size > 0) chunks.push(e.data);
      };

      recorder.onstop = () => {
        const ext = mimeType.includes('mp4') ? 'mp4' : 'webm';
        const blob = new Blob(chunks, { type: mimeType });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        const base = fileName.replace(/\.(html|deck|motion)$/, '');
        a.download = `${base}-recording.${ext}`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);

        stream.getTracks().forEach((t) => t.stop());
        setIsRecording(false);
        if (recordingTimerRef.current) clearInterval(recordingTimerRef.current);
        showToast('Video recording saved and downloaded!', 'success');
      };

      mediaRecorderRef.current = recorder;
      setIsRecording(true);
      setRecordingSeconds(0);

      // Seek to 0 and start playback
      sendMotionCommand('restart');
      recorder.start(100);

      const startTime = Date.now();
      recordingTimerRef.current = window.setInterval(() => {
        const elapsed = (Date.now() - startTime) / 1000;
        setRecordingSeconds(elapsed);
        if (elapsed >= duration + 0.5) {
          stopBrowserRecording();
        }
      }, 100);

      // If user stops sharing from browser chrome
      stream.getVideoTracks()[0].onended = () => {
        stopBrowserRecording();
      };
    } catch (err: any) {
      if (err.name !== 'NotAllowedError') {
        console.error('Browser recording error:', err);
        showToast(`Recording error: ${err.message}`, 'error');
      }
      setIsRecording(false);
    }
  };

  return (
    <div
      ref={containerRef}
      className="flex-1 flex flex-col h-full bg-[#0a0a0d] text-foreground select-none overflow-hidden relative"
    >
      {/* Top Viewer Control Bar */}
      <div className="h-11 min-h-11 px-4 border-b border-border/40 bg-card/60 backdrop-blur-md flex items-center justify-between text-xs">
        {/* Left: Mode Badge & Title */}
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-1.5 px-2.5 py-1 rounded-md bg-primary/15 text-primary border border-primary/25 font-mono text-[11px] font-medium">
            {mode === 'deck' ? <Presentation className="w-3.5 h-3.5" /> : <Video className="w-3.5 h-3.5" />}
            <span>{mode === 'deck' ? 'OPEN DESIGN // SLIDES' : 'HYPERFRAMES // MOTION'}</span>
          </div>
          <span className="text-foreground-secondary font-medium truncate max-w-[200px]" title={fileName}>
            {fileName}
          </span>
          <span className="text-foreground-muted font-mono text-[11px] hidden sm:inline">1920×1080 (16:9)</span>
        </div>

        {/* Center: Slide Switcher or Playhead Readout */}
        {mode === 'deck' ? (
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => sendDeckCommand('prev')}
              disabled={activeSlide <= 0}
              className="p-1 rounded hover:bg-surface-hover transition-colors disabled:opacity-30 disabled:pointer-events-none"
              title="Previous slide (←)"
            >
              <ChevronLeft className="w-4 h-4" />
            </button>
            <div className="font-mono text-xs px-2 py-0.5 rounded bg-surface border border-border/40">
              <span className="font-semibold text-primary">{String(activeSlide + 1).padStart(2, '0')}</span>
              <span className="text-foreground-muted"> / {String(slideCount).padStart(2, '0')}</span>
            </div>
            <button
              type="button"
              onClick={() => sendDeckCommand('next')}
              disabled={activeSlide >= slideCount - 1}
              className="p-1 rounded hover:bg-surface-hover transition-colors disabled:opacity-30 disabled:pointer-events-none"
              title="Next slide (→ / Space)"
            >
              <ChevronRight className="w-4 h-4" />
            </button>
          </div>
        ) : (
          <div className="flex items-center gap-3 font-mono text-xs">
            <button
              type="button"
              onClick={() => sendMotionCommand(isPlaying ? 'pause' : 'play')}
              className="p-1.5 rounded-full bg-primary text-primary-foreground hover:opacity-90 transition-opacity"
              title="Play / Pause (Space)"
            >
              {isPlaying ? <Pause className="w-3.5 h-3.5" /> : <Play className="w-3.5 h-3.5" />}
            </button>
            <button
              type="button"
              onClick={() => sendMotionCommand('restart')}
              className="p-1 rounded hover:bg-surface-hover transition-colors"
              title="Restart (↺)"
            >
              <RotateCcw className="w-3.5 h-3.5 text-foreground-muted" />
            </button>
            <div className="text-primary font-medium tracking-wider">
              {formatTime(currentTime)} <span className="text-foreground-muted font-normal">/ {formatTime(duration)}</span>
            </div>
          </div>
        )}

        {/* Right: Actions (Download Video, Thumbnails, PDF Print, Fullscreen, Close) */}
        <div className="flex items-center gap-2">
          {mode === 'deck' && (
            <>
              <button
                type="button"
                onClick={() => setIsThumbnailsOpen(!isThumbnailsOpen)}
                className={`flex items-center gap-1.5 px-2 py-1 rounded text-xs transition-colors ${
                  isThumbnailsOpen ? 'bg-primary/20 text-primary font-medium' : 'hover:bg-surface-hover text-foreground-muted hover:text-foreground'
                }`}
                title="Toggle Slide Thumbnails Rail"
              >
                <Layers className="w-3.5 h-3.5" />
                <span>Slides</span>
              </button>
              <button
                type="button"
                onClick={handlePrint}
                className="flex items-center gap-1.5 px-2 py-1 rounded text-xs hover:bg-surface-hover text-foreground-muted hover:text-foreground transition-colors"
                title="Export or Print as PDF"
              >
                <Printer className="w-3.5 h-3.5" />
                <span>PDF</span>
              </button>
            </>
          )}

          {mode === 'motion' && (
            <button
              type="button"
              onClick={() => setIsExportModalOpen(true)}
              className="flex items-center gap-1.5 px-2.5 py-1 rounded bg-primary text-primary-foreground hover:bg-primary/90 transition-all font-medium text-xs shadow-sm shadow-primary/20"
              title="Download Motion Video (MP4 / WebM)"
            >
              <Download className="w-3.5 h-3.5" />
              <span>Download Video</span>
            </button>
          )}

          <button
            type="button"
            onClick={toggleFullscreen}
            className="p-1 rounded hover:bg-surface-hover text-foreground-muted hover:text-foreground transition-colors"
            title={isFullscreen ? 'Exit Fullscreen (Esc)' : 'Enter Presenter Fullscreen (F)'}
          >
            {isFullscreen ? <Minimize2 className="w-3.5 h-3.5" /> : <Maximize2 className="w-3.5 h-3.5" />}
          </button>

          {onClose && (
            <button
              type="button"
              onClick={onClose}
              className="px-2 py-1 rounded text-xs hover:bg-surface-hover text-foreground-muted hover:text-foreground transition-colors ml-1"
            >
              Editor
            </button>
          )}
        </div>
      </div>

      {/* Main Presentation Surface */}
      <div className="flex-1 flex overflow-hidden relative">
        {/* Left Slide Thumbnails Rail (when enabled) */}
        {mode === 'deck' && isThumbnailsOpen && (
          <div className="w-56 border-r border-border/40 bg-card/40 flex flex-col overflow-y-auto p-3 gap-2 z-20">
            <div className="text-[11px] font-mono uppercase tracking-wider text-foreground-muted mb-1 px-1">
              Slides ({slideCount})
            </div>
            {Array.from({ length: slideCount }).map((_, idx) => (
              <button
                key={idx}
                type="button"
                onClick={() => sendDeckCommand('go', idx)}
                className={`p-2 rounded-lg border text-left transition-all flex flex-col gap-1 cursor-pointer ${
                  activeSlide === idx
                    ? 'border-primary bg-primary/10 shadow-sm'
                    : 'border-border/30 bg-surface/30 hover:bg-surface/70 hover:border-border/60'
                }`}
              >
                <div className="flex items-center justify-between text-[11px] font-mono">
                  <span className={activeSlide === idx ? 'text-primary font-bold' : 'text-foreground-muted'}>
                    № {String(idx + 1).padStart(2, '0')}
                  </span>
                  {activeSlide === idx && <span className="w-1.5 h-1.5 rounded-full bg-primary" />}
                </div>
                <div className="text-xs font-medium text-foreground truncate">
                  {slideLabels[idx] || `Slide ${idx + 1}`}
                </div>
              </button>
            ))}
          </div>
        )}

        {/* Scaled Presentation Iframe Container */}
        <div className="flex-1 flex items-center justify-center p-4 bg-[#08080a] overflow-hidden relative">
          {/* Active In-Browser Recording Overlay Badge */}
          {isRecording && (
            <div className="absolute top-6 left-1/2 -translate-x-1/2 z-30 flex items-center gap-3 px-4 py-2 rounded-full bg-red-950/80 border border-red-500/60 backdrop-blur-md shadow-2xl text-xs font-mono">
              <span className="w-2.5 h-2.5 rounded-full bg-red-500 animate-ping" />
              <span className="text-red-200 font-bold">RECORDING VIDEO</span>
              <span className="text-red-300">
                {formatTime(recordingSeconds)} / {formatTime(duration)}
              </span>
              <button
                type="button"
                onClick={stopBrowserRecording}
                className="flex items-center gap-1 px-2.5 py-0.5 rounded-full bg-red-600 hover:bg-red-500 text-white font-medium text-[11px] transition-colors ml-1"
              >
                <StopCircle className="w-3 h-3" />
                <span>Stop & Save</span>
              </button>
            </div>
          )}

          <iframe
            ref={iframeRef}
            srcDoc={htmlContent}
            title={fileName}
            sandbox="allow-scripts allow-same-origin allow-modals"
            className="w-full h-full border-0 rounded shadow-2xl bg-black"
          />
        </div>
      </div>

      {/* Bottom Timeline Bar for Motion */}
      {mode === 'motion' && (
        <div className="h-12 border-t border-border/40 bg-card/60 backdrop-blur-md px-6 flex items-center gap-4 z-20">
          <button
            type="button"
            onClick={() => sendMotionCommand(isPlaying ? 'pause' : 'play')}
            className="text-xs font-mono font-medium text-primary hover:underline"
          >
            {isPlaying ? 'PAUSE' : 'PLAY'}
          </button>

          {/* Time Scrubber */}
          <input
            type="range"
            min="0"
            max={duration}
            step="0.01"
            value={currentTime}
            onChange={(e) => {
              const val = parseFloat(e.target.value);
              setCurrentTime(val);
              sendMotionCommand('seek', val);
            }}
            className="flex-1 h-1.5 bg-surface rounded-lg appearance-none cursor-pointer accent-primary"
          />

          <div className="flex items-center gap-3 text-xs font-mono text-foreground-muted">
            <span>{formatTime(currentTime)} / {formatTime(duration)}</span>
            <select
              value={playbackSpeed}
              onChange={(e) => setPlaybackSpeed(parseFloat(e.target.value))}
              className="bg-surface border border-border/40 rounded px-1.5 py-0.5 text-[11px] text-foreground"
            >
              <option value="0.5">0.5x</option>
              <option value="1">1.0x</option>
              <option value="1.5">1.5x</option>
              <option value="2">2.0x</option>
            </select>

            {/* Quick Export in Timeline */}
            <button
              type="button"
              onClick={() => setIsExportModalOpen(true)}
              className="flex items-center gap-1 px-2 py-0.5 rounded bg-surface hover:bg-surface-hover border border-border/40 text-foreground-secondary hover:text-foreground text-[11px] font-medium transition-colors ml-1"
              title="Export / Download Video"
            >
              <Download className="w-3 h-3 text-primary" />
              <span>Export</span>
            </button>
          </div>
        </div>
      )}

      {/* Download Motion Video Modal */}
      {isExportModalOpen && (
        <div className="absolute inset-0 z-50 bg-black/75 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="w-full max-w-lg bg-[#111116] border border-border/60 rounded-xl shadow-2xl overflow-hidden flex flex-col">
            {/* Modal Header */}
            <div className="h-12 px-5 border-b border-border/40 flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Film className="w-4 h-4 text-primary" />
                <span className="font-medium text-sm text-foreground">Download Motion Video</span>
              </div>
              <button
                type="button"
                onClick={() => setIsExportModalOpen(false)}
                className="p-1 rounded hover:bg-surface text-foreground-muted hover:text-foreground transition-colors"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            {/* Modal Body */}
            <div className="p-5 flex flex-col gap-4 text-xs">
              <div className="flex items-center justify-between p-3 rounded-lg bg-surface/40 border border-border/40 font-mono text-[11px]">
                <div>
                  <div className="text-foreground-muted">Composition</div>
                  <div className="text-foreground font-medium truncate max-w-[220px]">{fileName}</div>
                </div>
                <div className="text-right">
                  <div className="text-foreground-muted">Resolution & Length</div>
                  <div className="text-primary font-semibold">1920×1080 · {duration}s</div>
                </div>
              </div>

              {/* Method 1: Studio Quality 60fps MP4 (HyperFrames Render) */}
              <div className="p-4 rounded-xl border border-primary/30 bg-primary/5 flex flex-col gap-2.5 relative">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Sparkles className="w-4 h-4 text-primary" />
                    <span className="font-semibold text-sm text-foreground">Studio MP4 Render</span>
                  </div>
                  <span className="px-2 py-0.5 rounded-full bg-primary/20 text-primary border border-primary/30 text-[10px] font-mono font-medium">
                    RECOMMENDED · 60 FPS
                  </span>
                </div>
                <p className="text-foreground-muted leading-relaxed">
                  Deterministic frame-by-frame rendering with bundled HyperFrames & FFmpeg. Produces a buttery-smooth 60fps MP4 with zero dropped frames.
                </p>

                {isRendering ? (
                  <div className="flex items-center gap-2.5 p-3 rounded-lg bg-card/80 border border-border/60 text-foreground font-mono text-[11px]">
                    <Loader2 className="w-4 h-4 animate-spin text-primary shrink-0" />
                    <div className="flex-1 truncate">{renderProgress || 'Rendering video...'}</div>
                  </div>
                ) : renderedOutputPath ? (
                  <div className="flex flex-col gap-2 p-3 rounded-lg bg-emerald-950/40 border border-emerald-500/40">
                    <div className="flex items-center gap-2 text-emerald-400 font-medium">
                      <CheckCircle2 className="w-4 h-4 shrink-0" />
                      <span>Render Complete! Saved to workspace</span>
                    </div>
                    <div className="font-mono text-[11px] text-foreground-muted truncate">
                      {renderedOutputPath}
                    </div>
                    <div className="flex items-center gap-2 mt-1">
                      <button
                        type="button"
                        onClick={() => void ApiBridge.openInFinder(renderedOutputPath)}
                        className="flex items-center gap-1.5 px-3 py-1 rounded bg-surface hover:bg-surface-hover border border-border/60 text-foreground font-medium text-xs transition-colors"
                      >
                        <FolderOpen className="w-3.5 h-3.5 text-primary" />
                        <span>Show in Finder</span>
                      </button>
                      <button
                        type="button"
                        onClick={() => {
                          void navigator.clipboard.writeText(renderedOutputPath);
                          showToast('Path copied to clipboard!', 'info');
                        }}
                        className="flex items-center gap-1.5 px-3 py-1 rounded bg-surface hover:bg-surface-hover border border-border/60 text-foreground-muted hover:text-foreground text-xs transition-colors"
                      >
                        <Copy className="w-3.5 h-3.5" />
                        <span>Copy Path</span>
                      </button>
                    </div>
                  </div>
                ) : (
                  <button
                    type="button"
                    onClick={handleRenderHeadless}
                    className="flex items-center justify-center gap-2 py-2 px-4 rounded-lg bg-primary hover:bg-primary/90 text-primary-foreground font-medium transition-all shadow-md shadow-primary/20 cursor-pointer"
                  >
                    <Download className="w-4 h-4" />
                    <span>Render 60fps MP4 Video</span>
                  </button>
                )}
              </div>

              {/* Method 2: Real-time In-Browser Screen/Tab Recording */}
              <div className="p-4 rounded-xl border border-border/40 bg-surface/20 flex flex-col gap-2.5">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Film className="w-4 h-4 text-foreground-secondary" />
                    <span className="font-semibold text-sm text-foreground">Direct Browser Capture</span>
                  </div>
                  <span className="px-2 py-0.5 rounded-full bg-surface text-foreground-muted border border-border/40 text-[10px] font-mono">
                    INSTANT · WEBM/MP4
                  </span>
                </div>
                <p className="text-foreground-muted leading-relaxed">
                  Captures playback live using your browser's MediaRecorder. Starts automatically from 0:00 and downloads when the video finishes.
                </p>
                <button
                  type="button"
                  onClick={handleStartBrowserRecording}
                  className="flex items-center justify-center gap-2 py-2 px-4 rounded-lg bg-surface hover:bg-surface-hover border border-border/60 text-foreground font-medium transition-colors cursor-pointer"
                >
                  <Film className="w-4 h-4 text-primary" />
                  <span>Record from Browser Tab</span>
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
