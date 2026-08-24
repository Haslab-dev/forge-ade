import React from "react";
import {
  IconBrandGolang,
  IconBrandJavascript,
  IconBrandPython,
  IconBrandReact,
  IconBrandRust,
  IconBrandTypescript,
  IconBrandHtml5,
  IconBrandCss3,
  IconDatabase,
  IconFileCode,
  IconFileText,
  IconFileZip,
  IconJson,
  IconMarkdown,
  IconPhoto,
  IconTerminal2,
  IconCode,
} from "@tabler/icons-react";

/**
 * Explorer/tab icon for a filename. One consistent outline style (Tabler)
 * with a per-type accent color, so the tree reads as a set rather than
 * a mix of brand logos.
 */
export function getFileIcon(filename: string, className = "size-4"): React.ReactNode {
  const ext = filename.split(".").pop()?.toLowerCase();

  switch (ext) {
    // Languages
    case "ts":
      return <IconBrandTypescript className={`${className} text-blue-400`} />;
    case "tsx":
      return <IconBrandReact className={`${className} text-cyan-300`} />;
    case "js":
    case "mjs":
    case "cjs":
      return <IconBrandJavascript className={`${className} text-yellow-400`} />;
    case "jsx":
      return <IconBrandReact className={`${className} text-cyan-300`} />;
    case "go":
      return <IconBrandGolang className={`${className} text-cyan-400`} />;
    case "py":
      return <IconBrandPython className={`${className} text-blue-400`} />;
    case "rs":
      return <IconBrandRust className={`${className} text-orange-400`} />;
    case "zig":
    case "zon":
      return <IconFileCode className={`${className} text-orange-300`} />;

    // Web
    case "html":
    case "htm":
      return <IconBrandHtml5 className={`${className} text-orange-400`} />;
    case "css":
    case "scss":
    case "less":
      return <IconBrandCss3 className={`${className} text-sky-400`} />;

    // Data / config
    case "json":
      return <IconJson className={`${className} text-amber-300`} />;
    case "toml":
    case "yaml":
    case "yml":
    case "ini":
    case "conf":
    case "env":
      return <IconFileCode className={`${className} text-gray-400`} />;
    case "sql":
    case "db":
    case "sqlite":
      return <IconDatabase className={`${className} text-amber-300`} />;

    // Docs & assets
    case "md":
    case "mdx":
      return <IconMarkdown className={`${className} text-sky-200`} />;
    case "png":
    case "jpg":
    case "jpeg":
    case "gif":
    case "webp":
    case "svg":
    case "ico":
    case "avif":
      return <IconPhoto className={`${className} text-purple-300`} />;
    case "zip":
    case "tar":
    case "gz":
    case "tgz":
    case "xz":
    case "7z":
    case "rar":
      return <IconFileZip className={`${className} text-amber-300`} />;

    // Shell
    case "sh":
    case "bash":
    case "zsh":
    case "fish":
      return <IconTerminal2 className={`${className} text-emerald-300`} />;

    default:
      if (filename.startsWith(".") || /config|rc$/.test(filename.toLowerCase())) {
        return <IconCode className={`${className} text-gray-400`} />;
      }
      return <IconFileText className={`${className} text-gray-400`} />;
  }
}
