import React, { useState, useEffect, useRef } from 'react';
import { 
  ZoomIn, 
  ZoomOut, 
  RotateCcw, 
  Maximize, 
  FileCode, 
  Image as ImageIcon,
  Copy,
  Check,
  Download
} from 'lucide-react';
import { ApiBridge } from '../../services/apiBridge';

interface ImagePreviewProps {
  filePath: string;
  fileName: string;
  rawContent?: string;
}

export const ImagePreview: React.FC<ImagePreviewProps> = ({ filePath, fileName, rawContent }) => {
  const [base64Data, setBase64Data] = useState<string>('');
  const [svgContent, setSvgContent] = useState<string>('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [zoom, setZoom] = useState(1);
  const [dimensions, setDimensions] = useState<{ width: number; height: number } | null>(null);
  const [fileSize, setFileSize] = useState<string>('');
  const [svgViewMode, setSvgViewMode] = useState<'preview' | 'code'>('preview');
  const [copied, setCopied] = useState(false);

  const isSvg = fileName.toLowerCase().endsWith('.svg');

  // Determine MIME type
  const getMimeType = (name: string): string => {
    const ext = name.split('.').pop()?.toLowerCase();
    switch (ext) {
      case 'png': return 'image/png';
      case 'jpg':
      case 'jpeg': return 'image/jpeg';
      case 'gif': return 'image/gif';
      case 'webp': return 'image/webp';
      case 'ico': return 'image/x-icon';
      case 'icns': return 'image/x-icns';
      case 'svg': return 'image/svg+xml';
      case 'bmp': return 'image/bmp';
      default: return 'image/png';
    }
  };

  useEffect(() => {
    let isMounted = true;
    const loadImage = async () => {
      setLoading(true);
      setError(null);
      try {
        if (isSvg) {
          if (rawContent && rawContent.includes('<svg')) {
            if (isMounted) setSvgContent(rawContent);
          } else {
            const text = await ApiBridge.readFile(filePath);
            if (isMounted) setSvgContent(text || '');
          }
        }
        
        // Fetch Base64 data for image rendering
        const b64 = await ApiBridge.readFileBase64(filePath);
        if (isMounted) {
          if (b64) {
            setBase64Data(b64);
            const sizeBytes = Math.round((b64.length * 3) / 4);
            if (sizeBytes > 1024 * 1024) {
              setFileSize(`${(sizeBytes / (1024 * 1024)).toFixed(2)} MB`);
            } else if (sizeBytes > 1024) {
              setFileSize(`${(sizeBytes / 1024).toFixed(1)} KB`);
            } else {
              setFileSize(`${sizeBytes} B`);
            }
          } else if (isSvg && rawContent) {
            // SVG can render directly from XML string
            setBase64Data(btoa(rawContent));
          } else {
            setError('Could not decode image content from disk.');
          }
        }
      } catch (err: any) {
        if (isMounted) setError(err?.message || 'Failed to load image');
      } finally {
        if (isMounted) setLoading(false);
      }
    };

    loadImage();
    return () => { isMounted = false; };
  }, [filePath, fileName, rawContent, isSvg]);

  const mime = getMimeType(fileName);
  const imageSrc = isSvg && svgContent
    ? `data:image/svg+xml;utf8,${encodeURIComponent(svgContent)}`
    : base64Data
    ? `data:${mime};base64,${base64Data}`
    : '';

  const handleZoomIn = () => setZoom(prev => Math.min(5, prev + 0.25));
  const handleZoomOut = () => setZoom(prev => Math.max(0.1, prev - 0.25));
  const handleResetZoom = () => setZoom(1);

  const handleCopy = () => {
    if (imageSrc) {
      navigator.clipboard.writeText(imageSrc);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  return (
    <div className="flex-1 flex flex-col h-full bg-[#f8fafc] dark:bg-[#141416] select-none overflow-hidden font-sans">
      
      {/* Top Toolbar */}
      <div className="h-9 min-h-[36px] bg-white dark:bg-[#1c1c1f] border-b border-[#e2e8f0] dark:border-[#2b2b2b] px-4 flex items-center justify-between text-xs text-[#64748b] dark:text-[#9ca3af]">
        
        {/* Left: Image specs */}
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-1.5 font-medium text-[#0f172a] dark:text-white">
            <ImageIcon className="w-4 h-4 text-[#8b5cf6]" />
            <span className="font-mono text-xs">{fileName}</span>
          </div>

          {dimensions && (
            <span className="font-mono text-[11px] px-2 py-0.5 rounded bg-[#f1f5f9] dark:bg-[#28282b] text-[#334155] dark:text-[#cbd5e1]">
              {dimensions.width} × {dimensions.height} px
            </span>
          )}

          {fileSize && (
            <span className="font-mono text-[11px] text-[#64748b] dark:text-[#94a3b8]">
              {fileSize}
            </span>
          )}
        </div>

        {/* Right: Controls (Zoom, Copy, Toggle SVG code) */}
        <div className="flex items-center gap-2">
          {isSvg && (
            <div className="bg-[#f1f5f9] dark:bg-[#28282b] p-0.5 rounded-lg flex items-center text-[11px] mr-2">
              <button
                type="button"
                onClick={() => setSvgViewMode('preview')}
                className={`px-2 py-0.5 rounded transition-colors cursor-pointer ${
                  svgViewMode === 'preview'
                    ? 'bg-white dark:bg-[#181818] text-[#0f172a] dark:text-white font-semibold shadow-2xs'
                    : 'text-[#64748b] dark:text-[#9ca3af]'
                }`}
              >
                Preview
              </button>
              <button
                type="button"
                onClick={() => setSvgViewMode('code')}
                className={`px-2 py-0.5 rounded transition-colors cursor-pointer ${
                  svgViewMode === 'code'
                    ? 'bg-white dark:bg-[#181818] text-[#0f172a] dark:text-white font-semibold shadow-2xs'
                    : 'text-[#64748b] dark:text-[#9ca3af]'
                }`}
              >
                XML Source
              </button>
            </div>
          )}

          <div className="flex items-center gap-1 bg-[#f1f5f9] dark:bg-[#28282b] p-0.5 rounded-lg">
            <button
              type="button"
              onClick={handleZoomOut}
              className="p-1 hover:bg-white dark:hover:bg-[#181818] rounded transition-colors cursor-pointer"
              title="Zoom Out"
            >
              <ZoomOut className="w-3.5 h-3.5" />
            </button>
            <span className="font-mono text-[11px] px-1.5 min-w-[45px] text-center font-semibold text-[#0f172a] dark:text-white">
              {Math.round(zoom * 100)}%
            </span>
            <button
              type="button"
              onClick={handleZoomIn}
              className="p-1 hover:bg-white dark:hover:bg-[#181818] rounded transition-colors cursor-pointer"
              title="Zoom In"
            >
              <ZoomIn className="w-3.5 h-3.5" />
            </button>
            <button
              type="button"
              onClick={handleResetZoom}
              className="p-1 hover:bg-white dark:hover:bg-[#181818] rounded transition-colors cursor-pointer"
              title="Reset Zoom (100%)"
            >
              <RotateCcw className="w-3.5 h-3.5" />
            </button>
          </div>

          <button
            type="button"
            onClick={handleCopy}
            className="p-1.5 hover:bg-[#f1f5f9] dark:hover:bg-[#28282b] rounded text-[#64748b] dark:text-[#9ca3af] hover:text-[#0f172a] dark:hover:text-white transition-colors cursor-pointer"
            title="Copy Data URL"
          >
            {copied ? <Check className="w-3.5 h-3.5 text-[#16a34a]" /> : <Copy className="w-3.5 h-3.5" />}
          </button>
        </div>

      </div>

      {/* Main Preview Area */}
      <div className="flex-1 overflow-auto p-6 flex items-center justify-center relative">
        {loading ? (
          <div className="text-center text-xs text-[#9ca3af] animate-pulse">
            Loading image {fileName}...
          </div>
        ) : error ? (
          <div className="p-4 rounded-xl bg-[#fee2e2] dark:bg-[#450a0a] text-[#dc2626] dark:text-[#fca5a5] text-xs max-w-md text-center">
            {error}
          </div>
        ) : isSvg && svgViewMode === 'code' ? (
          <textarea
            readOnly
            value={svgContent}
            className="w-full h-full p-4 font-mono text-xs bg-white dark:bg-[#181818] border border-[#e2e8f0] dark:border-[#2b2b2b] rounded-xl text-[#0f172a] dark:text-[#e2e8f0] resize-none focus:outline-none"
          />
        ) : (
          /* Checkerboard Canvas Container for Transparency */
          <div 
            className="relative p-4 rounded-xl border border-[#e2e8f0] dark:border-[#2b2b2b] shadow-md transition-transform"
            style={{
              backgroundImage: `
                linear-gradient(45deg, #e2e8f0 25%, transparent 25%), 
                linear-gradient(-45deg, #e2e8f0 25%, transparent 25%), 
                linear-gradient(45deg, transparent 75%, #e2e8f0 75%), 
                linear-gradient(-45deg, transparent 75%, #e2e8f0 75%)
              `,
              backgroundSize: '16px 16px',
              backgroundPosition: '0 0, 0 8px, 8px -8px, -8px 0px'
            }}
          >
            <img
              src={imageSrc}
              alt={fileName}
              onLoad={(e) => {
                const img = e.currentTarget;
                setDimensions({ width: img.naturalWidth, height: img.naturalHeight });
              }}
              style={{
                transform: `scale(${zoom})`,
                transformOrigin: 'center center',
                transition: 'transform 0.1s ease-out',
                maxWidth: zoom <= 1 ? '100%' : 'none',
                maxHeight: zoom <= 1 ? '70vh' : 'none'
              }}
              className="object-contain rounded select-none shadow-xs"
            />
          </div>
        )}
      </div>

    </div>
  );
};
