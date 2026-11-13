import React, { useState, useRef, useEffect } from "react";
import { cn } from "../lib/utils";

interface ResizableSplitProps {
  direction?: "horizontal" | "vertical";
  left: React.ReactNode;
  right: React.ReactNode;
  initialLeftWidth?: number;
  minLeftWidth?: number;
  maxLeftWidth?: number;
  collapsed?: boolean;
  collapsedWidth?: number;
}

export function ResizableSplit({
  direction = "horizontal",
  left,
  right,
  initialLeftWidth = 250,
  minLeftWidth = 100,
  maxLeftWidth = 800,
  collapsed = false,
  collapsedWidth = 4,
}: ResizableSplitProps) {
  const [leftWidth, setLeftWidth] = useState(initialLeftWidth);
  const [isResizing, setIsResizing] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  const startResizing = (e: React.MouseEvent) => {
    e.preventDefault();
    setIsResizing(true);
  };

  // Keyboard resizing (WCAG 2.1.1): arrows move the split by 16px, Shift by 4px.
  const resizeByKeyboard = (e: React.KeyboardEvent) => {
    const step = e.shiftKey ? 4 : 16;
    if (e.key === "ArrowLeft" || e.key === "ArrowUp") {
      e.preventDefault();
      setLeftWidth((w) => Math.max(minLeftWidth, w - step));
    } else if (e.key === "ArrowRight" || e.key === "ArrowDown") {
      e.preventDefault();
      setLeftWidth((w) => Math.min(maxLeftWidth, w + step));
    }
  };

  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      if (!isResizing || !containerRef.current) return;
      const containerRect = containerRef.current.getBoundingClientRect();
      
      let newWidth = 0;
      if (direction === "horizontal") {
        newWidth = e.clientX - containerRect.left;
      } else {
        newWidth = e.clientY - containerRect.top;
      }

      if (newWidth >= minLeftWidth && newWidth <= maxLeftWidth) {
        setLeftWidth(newWidth);
      }
    };

    const handleMouseUp = () => {
      setIsResizing(false);
    };

    if (isResizing) {
      document.addEventListener("mousemove", handleMouseMove);
      document.addEventListener("mouseup", handleMouseUp);
    }

    return () => {
      document.removeEventListener("mousemove", handleMouseMove);
      document.removeEventListener("mouseup", handleMouseUp);
    };
  }, [isResizing, direction, minLeftWidth, maxLeftWidth]);

  return (
    <div
      ref={containerRef}
      className={cn(
        "flex h-full w-full overflow-hidden select-none",
        direction === "horizontal" ? "flex-row" : "flex-col"
      )}
    >
      {/* Left / Top Panel */}
      <div
        style={{
          width: direction === "horizontal" ? (collapsed ? collapsedWidth : leftWidth) : "100%",
          height: direction === "vertical" ? (collapsed ? collapsedWidth : leftWidth) : "100%",
          overflow: "hidden",
        }}
        className="overflow-hidden shrink-0"
      >
        {left}
      </div>

      {/* Resize Handle */}
      {!collapsed && (
        <div
          role="separator"
          aria-orientation={direction === "horizontal" ? "vertical" : "horizontal"}
          aria-label="Resize split"
          tabIndex={0}
          onKeyDown={resizeByKeyboard}
          onMouseDown={startResizing}
          className={cn(
            "resize-handle bg-border hover:bg-primary focus-visible:bg-primary z-20 shrink-0 outline-none focus-visible:outline focus-visible:outline-1 focus-visible:outline-[var(--accent-primary,#5b9dff)]",
            direction === "horizontal" ? "w-[1px] cursor-col-resize h-full" : "h-[1px] cursor-row-resize w-full"
          )}
        />
      )}

      {/* Right / Bottom Panel */}
      <div className="flex-1 overflow-hidden">{right}</div>
    </div>
  );
}
