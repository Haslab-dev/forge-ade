import React from "react";
import {
  IconBrandGolang,
  IconBrandJavascript,
  IconBrandPython,
  IconBrandRust,
  IconBrandHtml5,
  IconBrandCss3,
  IconJson,
  IconFileText,
  IconMarkdown,
  IconCode,
} from "@tabler/icons-react";

export function getFileIcon(filename: string, className = "size-4", muted = false): React.ReactNode {
  const ext = filename.split(".").pop()?.toLowerCase();
  // Muted mode: real per-extension glyphs without the colorful branding —
  // used by dense chrome surfaces (Review list) to keep the palette calm.
  if (muted) {
    const tone = "text-foreground-subtle";
    switch (ext) {
      case "go":
        return <IconBrandGolang className={`${className} ${tone}`} />;
      case "js":
      case "jsx":
      case "ts":
      case "tsx":
        return <IconBrandJavascript className={`${className} ${tone}`} />;
      case "py":
        return <IconBrandPython className={`${className} ${tone}`} />;
      case "rs":
        return <IconBrandRust className={`${className} ${tone}`} />;
      case "html":
      case "htm":
        return <IconBrandHtml5 className={`${className} ${tone}`} />;
      case "css":
        return <IconBrandCss3 className={`${className} ${tone}`} />;
      case "json":
        return <IconJson className={`${className} ${tone}`} />;
      case "md":
        return <IconMarkdown className={`${className} ${tone}`} />;
      default:
        if (filename.startsWith(".") || filename.includes("config")) {
          return <IconCode className={`${className} ${tone}`} />;
        }
        return <IconFileText className={`${className} ${tone}`} />;
    }
  }
  
  switch (ext) {
    case "go":
      return <IconBrandGolang className={`${className} text-cyan-400`} />;
    case "js":
    case "jsx":
      return <IconBrandJavascript className={`${className} text-yellow-400`} />;
    case "ts":
    case "tsx":
      return <IconBrandJavascript className={`${className} text-blue-400`} />;
    case "py":
      return <IconBrandPython className={`${className} text-blue-500`} />;
    case "rs":
      return <IconBrandRust className={`${className} text-orange-500`} />;
    case "html":
    case "htm":
      return <IconBrandHtml5 className={`${className} text-orange-400`} />;
    case "css":
      return <IconBrandCss3 className={`${className} text-blue-500`} />;
    case "json":
      return <IconJson className={`${className} text-yellow-500`} />;
    case "md":
      return <IconMarkdown className={`${className} text-blue-300`} />;
    default:
      if (filename.startsWith(".") || filename.includes("config")) {
        return <IconCode className={`${className} text-gray-400`} />;
      }
      return <IconFileText className={`${className} text-gray-300`} />;
  }
}
