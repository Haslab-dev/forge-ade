import React, { useState, useRef, useEffect, ReactNode } from "react";

interface ResizableSplitProps {
  left: ReactNode;
  right: ReactNode;
  initialLeftWidth?: number;
  minLeftWidth?: number;
  maxLeftWidth?: number;
  collapsed?: boolean;
  collapsedWidth?: number;
}

export function ResizableSplit({
  left,
  right,
  initialLeftWidth = 260,
  minLeftWidth = 180,
  maxLeftWidth = 600,
  collapsed = false,
  collapsedWidth = 48,
}: ResizableSplitProps) {
  const [leftWidth, setLeftWidth] = useState(initialLeftWidth);
  const [isDragging, setIsDragging] = useState(false);
  const isDraggingRef = useRef(false);
  const startXRef = useRef(0);
  const startWidthRef = useRef(initialLeftWidth);

  const currentWidth = collapsed ? collapsedWidth : leftWidth;

  const handleMouseDown = (e: React.MouseEvent) => {
    if (collapsed) return;
    e.preventDefault();
    isDraggingRef.current = true;
    setIsDragging(true);
    startXRef.current = e.clientX;
    startWidthRef.current = leftWidth;

    document.addEventListener("mousemove", handleMouseMove);
    document.addEventListener("mouseup", handleMouseUp);
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
  };

  const handleMouseMove = (e: MouseEvent) => {
    if (!isDraggingRef.current) return;
    const deltaX = e.clientX - startXRef.current;
    let newWidth = startWidthRef.current + deltaX;
    if (newWidth < minLeftWidth) newWidth = minLeftWidth;
    if (newWidth > maxLeftWidth) newWidth = maxLeftWidth;
    setLeftWidth(newWidth);
  };

  const handleMouseUp = () => {
    isDraggingRef.current = false;
    setIsDragging(false);
    document.removeEventListener("mousemove", handleMouseMove);
    document.removeEventListener("mouseup", handleMouseUp);
    document.body.style.cursor = "default";
    document.body.style.userSelect = "auto";
  };

  return (
    <div className="flex h-full w-full overflow-hidden select-none relative">
      {/* Overlay to catch all moves and prevent iframe capture */}
      {isDragging && (
        <div className="fixed inset-0 z-50 cursor-col-resize bg-transparent" />
      )}

      {/* Left Panel */}
      <div style={{ width: `${currentWidth}px` }} className="h-full shrink-0 overflow-hidden transition-[width] duration-200 ease-in-out">
        {left}
      </div>

      {/* Resize Handle */}
      {!collapsed && (
        <div
          onMouseDown={handleMouseDown}
          className="w-1.5 h-full cursor-col-resize bg-[var(--color-border)] hover:bg-blue-500/80 transition-colors shrink-0 z-10"
          title="Drag to resize horizontally"
        />
      )}

      {/* Right Panel */}
      <div className="flex-1 h-full min-w-0 overflow-hidden">
        {right}
      </div>
    </div>
  );
}
