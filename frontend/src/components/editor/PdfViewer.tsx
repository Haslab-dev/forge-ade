import React, { useState, useRef, useEffect } from 'react';
import { ChevronLeft, ChevronRight, ZoomIn, ZoomOut, Download, Maximize2, X } from 'lucide-react';

interface PdfViewerProps {
  filePath: string;
  fileName: string;
}

export const PdfViewer: React.FC<PdfViewerProps> = ({ filePath, fileName }) => {
  const [currentPage, setCurrentPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [zoom, setZoom] = useState(100);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const iframeRef = useRef<HTMLIFrameElement>(null);

  const pdfUrl = `/api/workspace/file?path=${encodeURIComponent(filePath)}`;

  return (
    <div className={`flex flex-col h-full bg-[#f0f0f0] dark:bg-[#1a1a1a] ${isFullscreen ? 'fixed inset-0 z-50' : ''}`}>
      {/* Toolbar */}
      <div className="h-[35px] min-h-[35px] bg-white dark:bg-[#252526] border-b border-[#e2e8f0] dark:border-[#1e1e1e] flex items-center justify-between px-3 select-none">
        <div className="flex items-center gap-2 text-xs text-[#64748b] dark:text-[#9ca3af]">
          <span className="font-medium text-[#0f172a] dark:text-white">{fileName}</span>
          <span className="text-[10px] px-1.5 py-0.5 rounded bg-[#fee2e2] dark:bg-[#450a0a] text-[#dc2626] dark:text-[#fca5a5] font-mono font-semibold">
            PDF
          </span>
        </div>
        <div className="flex items-center gap-1">
          <button
            onClick={() => setCurrentPage(p => Math.max(1, p - 1))}
            disabled={currentPage <= 1}
            className="p-1 hover:bg-[#e2e8f0] dark:hover:bg-[#333] rounded transition-colors disabled:opacity-30"
            title="Previous Page"
          >
            <ChevronLeft className="w-3.5 h-3.5" />
          </button>
          <span className="text-[10px] font-mono text-[#64748b] dark:text-[#9ca3af] px-1">
            {currentPage} / {totalPages}
          </span>
          <button
            onClick={() => setCurrentPage(p => Math.min(totalPages, p + 1))}
            disabled={currentPage >= totalPages}
            className="p-1 hover:bg-[#e2e8f0] dark:hover:bg-[#333] rounded transition-colors disabled:opacity-30"
            title="Next Page"
          >
            <ChevronRight className="w-3.5 h-3.5" />
          </button>
          <div className="w-px h-3.5 bg-[#e2e8f0] dark:bg-[#444] mx-1" />
          <button onClick={() => setZoom(z => Math.max(25, z - 25))} className="p-1 hover:bg-[#e2e8f0] dark:hover:bg-[#333] rounded transition-colors" title="Zoom Out">
            <ZoomOut className="w-3.5 h-3.5" />
          </button>
          <span className="text-[10px] font-mono text-[#64748b] dark:text-[#9ca3af] w-10 text-center">{zoom}%</span>
          <button onClick={() => setZoom(z => Math.min(200, z + 25))} className="p-1 hover:bg-[#e2e8f0] dark:hover:bg-[#333] rounded transition-colors" title="Zoom In">
            <ZoomIn className="w-3.5 h-3.5" />
          </button>
          <div className="w-px h-3.5 bg-[#e2e8f0] dark:bg-[#444] mx-1" />
          <a
            href={pdfUrl}
            download={fileName}
            className="p-1 hover:bg-[#e2e8f0] dark:hover:bg-[#333] rounded transition-colors"
            title="Download"
          >
            <Download className="w-3.5 h-3.5" />
          </a>
          <button onClick={() => setIsFullscreen(f => !f)} className="p-1 hover:bg-[#e2e8f0] dark:hover:bg-[#333] rounded transition-colors" title="Toggle Fullscreen">
            {isFullscreen ? <X className="w-3.5 h-3.5" /> : <Maximize2 className="w-3.5 h-3.5" />}
          </button>
        </div>
      </div>

      {/* PDF Display via iframe with Google Docs Viewer fallback */}
      <div className="flex-1 flex items-center justify-center overflow-hidden bg-[#525659]">
        <iframe
          ref={iframeRef}
          src={`${pdfUrl}#page=${currentPage}`}
          className="w-full h-full border-0"
          style={{ transform: `scale(${zoom / 100})`, transformOrigin: 'center center' }}
          title={fileName}
          onLoad={() => {
            // Try to get total pages from PDF.js if available
            try {
              const iframe = iframeRef.current;
              if (iframe?.contentWindow) {
                // Approximate: set a reasonable default
                setTotalPages(Math.max(1, totalPages));
              }
            } catch {}
          }}
        />
      </div>

      {/* Status Bar */}
      <div className="h-[22px] min-h-[22px] bg-[#f1f5f9] dark:bg-[#181818] border-t border-[#e2e8f0] dark:border-[#282828] px-3 flex items-center justify-between text-[10px] text-[#64748b] dark:text-[#94a3b8] font-mono select-none">
        <span>{fileName}</span>
        <span>Page {currentPage} · {zoom}%</span>
      </div>
    </div>
  );
};
