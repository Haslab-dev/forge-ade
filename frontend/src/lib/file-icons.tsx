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

export function getFileIcon(filename: string, className = "size-4"): React.ReactNode {
  const ext = filename.split(".").pop()?.toLowerCase();
  
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
