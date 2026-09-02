import React, { useState } from 'react';
import { ZoomIn, ZoomOut, RotateCw, Download, Maximize2, X } from 'lucide-react';

interface ImageViewerProps {
  filePath: string;
  fileName: string;
}

export const ImageViewer: React.FC<ImageViewerProps> = ({ filePath, fileName }) => {
  const [zoom, setZoom] = useState(100);
  const [rotation, setRotation] = useState(0);
  const [isFullscreen, setIsFullscreen] = useState(false);

  const isSvg = fileName.endsWith('.svg');
  const imageUrl = `/api/workspace/file?path=${encodeURIComponent(filePath)}`;

  return (
    <div className={`flex flex-col h-full bg-[#f0f0f0] dark:bg-[#1a1a1a] ${isFullscreen ? 'fixed inset-0 z-50' : ''}`}>
      {/* Toolbar */}
      <div className="h-[35px] min-h-[35px] bg-white dark:bg-[#252526] border-b border-[#e2e8f0] dark:border-[#1e1e1e] flex items-center justify-between px-3 select-none">
        <div className="flex items-center gap-2 text-xs text-[#64748b] dark:text-[#9ca3af]">
          <span className="font-medium text-[#0f172a] dark:text-white">{fileName}</span>
          <span className="text-[10px] px-1.5 py-0.5 rounded bg-[#f1f5f9] dark:bg-[#333] font-mono">
            {isSvg ? 'SVG' : 'IMG'}
          </span>
        </div>
        <div className="flex items-center gap-1">
          <button onClick={() => setZoom(z => Math.max(25, z - 25))} className="p-1 hover:bg-[#e2e8f0] dark:hover:bg-[#333] rounded transition-colors" title="Zoom Out">
            <ZoomOut className="w-3.5 h-3.5" />
          </button>
          <span className="text-[10px] font-mono text-[#64748b] dark:text-[#9ca3af] w-10 text-center">{zoom}%</span>
          <button onClick={() => setZoom(z => Math.min(400, z + 25))} className="p-1 hover:bg-[#e2e8f0] dark:hover:bg-[#333] rounded transition-colors" title="Zoom In">
            <ZoomIn className="w-3.5 h-3.5" />
          </button>
          <div className="w-px h-3.5 bg-[#e2e8f0] dark:bg-[#444] mx-1" />
          <button onClick={() => setRotation(r => r + 90)} className="p-1 hover:bg-[#e2e8f0] dark:hover:bg-[#333] rounded transition-colors" title="Rotate">
            <RotateCw className="w-3.5 h-3.5" />
          </button>
          <button onClick={() => setZoom(100)} className="p-1 hover:bg-[#e2e8f0] dark:hover:bg-[#333] rounded transition-colors text-[10px] font-mono" title="Reset">
            1:1
          </button>
          <button onClick={() => setIsFullscreen(f => !f)} className="p-1 hover:bg-[#e2e8f0] dark:hover:bg-[#333] rounded transition-colors" title="Toggle Fullscreen">
            {isFullscreen ? <X className="w-3.5 h-3.5" /> : <Maximize2 className="w-3.5 h-3.5" />}
          </button>
        </div>
      </div>

      {/* Image Display */}
      <div className="flex-1 flex items-center justify-center overflow-auto p-4" onClick={() => setIsFullscreen(false)}>
        <div
          style={{
            transform: `scale(${zoom / 100}) rotate(${rotation}deg)`,
            transition: 'transform 0.2s ease'
          }}
          className="origin-center"
        >
          {isSvg ? (
            <img
              src={imageUrl}
              alt={fileName}
              className="max-w-full max-h-full object-contain"
              style={{ maxWidth: '80vw', maxHeight: '70vh' }}
            />
          ) : (
            <img
              src={imageUrl}
              alt={fileName}
              className="max-w-full max-h-full object-contain shadow-lg rounded"
              style={{ maxWidth: '80vw', maxHeight: '70vh' }}
              onError={(e) => {
                (e.target as HTMLImageElement).style.display = 'none';
              }}
            />
          )}
        </div>
      </div>

      {/* Status Bar */}
      <div className="h-[22px] min-h-[22px] bg-[#f1f5f9] dark:bg-[#181818] border-t border-[#e2e8f0] dark:border-[#282828] px-3 flex items-center justify-between text-[10px] text-[#64748b] dark:text-[#94a3b8] font-mono select-none">
        <span>{fileName}</span>
        <span>{zoom}% · {rotation}°</span>
      </div>
    </div>
  );
};
