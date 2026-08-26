import type { Extension } from "@codemirror/state";

// Language grammars are loaded on demand: each entry dynamically imports its
// @codemirror/lang-* package so the editor core stays small and unused
// grammars are never downloaded. Results are cached per language id (see
// resolveLanguageExtension).
const L = {
  javascript: (opts?: { typescript?: boolean; jsx?: boolean }): Promise<Extension> =>
    import("@codemirror/lang-javascript").then((m) => m.javascript(opts)),
  go: (): Promise<Extension> => import("@codemirror/lang-go").then((m) => m.go()),
  python: (): Promise<Extension> => import("@codemirror/lang-python").then((m) => m.python()),
  rust: (): Promise<Extension> => import("@codemirror/lang-rust").then((m) => m.rust()),
  json: (): Promise<Extension> => import("@codemirror/lang-json").then((m) => m.json()),
  html: (): Promise<Extension> => import("@codemirror/lang-html").then((m) => m.html()),
  markdown: (): Promise<Extension> => import("@codemirror/lang-markdown").then((m) => m.markdown()),
  cpp: (): Promise<Extension> => import("@codemirror/lang-cpp").then((m) => m.cpp()),
  sql: (): Promise<Extension> => import("@codemirror/lang-sql").then((m) => m.sql()),
  php: (): Promise<Extension> => import("@codemirror/lang-php").then((m) => m.php()),
  css: (): Promise<Extension> => import("@codemirror/lang-css").then((m) => m.css()),
  less: (): Promise<Extension> => import("@codemirror/lang-less").then((m) => m.less()),
  sass: (): Promise<Extension> => import("@codemirror/lang-sass").then((m) => m.sass()),
  java: (): Promise<Extension> => import("@codemirror/lang-java").then((m) => m.java()),
  xml: (): Promise<Extension> => import("@codemirror/lang-xml").then((m) => m.xml()),
  vue: (): Promise<Extension> => import("@codemirror/lang-vue").then((m) => m.vue()),
};

export interface CommentTokens {
  singleLine?: string;
  multiLineStart?: string;
  multiLineEnd?: string;
}

export interface LanguageMeta {
  id: string;
  name: string;
  extensions: string[];
  filenames?: string[];
  comment: CommentTokens;
  indent: string;
  stickyHeaders?: string[];
  getExtension: () => Promise<Extension>;
}

export const LANGUAGE_REGISTRY: Record<string, LanguageMeta> = {
  rust: {
    id: "rust",
    name: "Rust",
    extensions: ["rs"],
    filenames: ["Cargo.lock"],
    comment: { singleLine: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "    ",
    stickyHeaders: ["function_item", "impl_item", "trait_item", "struct_item", "enum_item"],
    getExtension: () => L.rust(),
  },
  go: {
    id: "go",
    name: "Go",
    extensions: ["go"],
    filenames: ["go.mod", "go.sum"],
    comment: { singleLine: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "\t",
    stickyHeaders: ["function_declaration", "method_declaration", "type_declaration"],
    getExtension: () => L.go(),
  },
  typescript: {
    id: "typescript",
    name: "TypeScript",
    extensions: ["ts", "mts", "cts"],
    filenames: ["tsconfig.json"],
    comment: { singleLine: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "  ",
    stickyHeaders: ["function_declaration", "class_declaration", "interface_declaration", "enum_declaration"],
    getExtension: () => L.javascript({ typescript: true }),
  },
  typescriptreact: {
    id: "typescriptreact",
    name: "TypeScript React",
    extensions: ["tsx"],
    comment: { singleLine: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "  ",
    stickyHeaders: ["function_declaration", "class_declaration", "interface_declaration"],
    getExtension: () => L.javascript({ jsx: true, typescript: true }),
  },
  javascript: {
    id: "javascript",
    name: "JavaScript",
    extensions: ["js", "mjs", "cjs"],
    filenames: ["package.json"],
    comment: { singleLine: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "  ",
    stickyHeaders: ["function_declaration", "class_declaration"],
    getExtension: () => L.javascript(),
  },
  javascriptreact: {
    id: "javascriptreact",
    name: "JavaScript React",
    extensions: ["jsx"],
    comment: { singleLine: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "  ",
    stickyHeaders: ["function_declaration", "class_declaration"],
    getExtension: () => L.javascript({ jsx: true }),
  },
  python: {
    id: "python",
    name: "Python",
    extensions: ["py", "pyi", "pyw"],
    filenames: ["Pipfile", "requirements.txt", "pyproject.toml"],
    comment: { singleLine: "#", multiLineStart: '"""', multiLineEnd: '"""' },
    indent: "    ",
    stickyHeaders: ["function_definition", "class_definition"],
    getExtension: () => L.python(),
  },
  zig: {
    id: "zig",
    name: "Zig",
    extensions: ["zig", "zon"],
    filenames: ["build.zig", "build.zig.zon", "app.zon"],
    comment: { singleLine: "//" },
    indent: "    ",
    stickyHeaders: ["FnProto", "ContainerDecl"],
    getExtension: () => L.cpp(),
  },
  cpp: {
    id: "cpp",
    name: "C++",
    extensions: ["cpp", "hpp", "cxx", "hxx", "c++", "h++", "cc", "hh", "C", "H", "ino"],
    filenames: ["CMakeLists.txt"],
    comment: { singleLine: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "  ",
    stickyHeaders: ["function_definition", "class_specifier", "struct_specifier"],
    getExtension: () => L.cpp(),
  },
  c: {
    id: "c",
    name: "C",
    extensions: ["c", "h"],
    filenames: ["Makefile"],
    comment: { singleLine: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "  ",
    stickyHeaders: ["function_definition", "struct_specifier"],
    getExtension: () => L.cpp(),
  },
  csharp: {
    id: "csharp",
    name: "C#",
    extensions: ["cs", "csx"],
    comment: { singleLine: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "    ",
    stickyHeaders: ["class_declaration", "method_declaration"],
    getExtension: () => L.cpp(),
  },
  java: {
    id: "java",
    name: "Java",
    extensions: ["java"],
    filenames: ["pom.xml", "build.gradle"],
    comment: { singleLine: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "    ",
    stickyHeaders: ["class_declaration", "method_declaration", "interface_declaration"],
    getExtension: () => L.java(),
  },
  kotlin: {
    id: "kotlin",
    name: "Kotlin",
    extensions: ["kt", "kts"],
    filenames: ["build.gradle.kts"],
    comment: { singleLine: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "    ",
    stickyHeaders: ["class_declaration", "function_declaration"],
    getExtension: () => L.java(),
  },
  swift: {
    id: "swift",
    name: "Swift",
    extensions: ["swift"],
    filenames: ["Package.swift"],
    comment: { singleLine: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "    ",
    stickyHeaders: ["function_declaration", "class_declaration"],
    getExtension: () => L.cpp(),
  },
  dart: {
    id: "dart",
    name: "Dart",
    extensions: ["dart"],
    filenames: ["pubspec.yaml"],
    comment: { singleLine: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "  ",
    stickyHeaders: ["class_definition", "method_signature"],
    getExtension: () => L.cpp(),
  },
  json: {
    id: "json",
    name: "JSON",
    extensions: ["json", "jsonc", "json5", "geojson", "yaml", "yml", "toml", "ini", "cfg", "conf", "properties", "env"],
    filenames: [".eslintrc", ".prettierrc", ".gitignore", "settings.toml", "Cargo.toml"],
    comment: { singleLine: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "  ",
    stickyHeaders: ["object", "pair"],
    getExtension: () => L.json(),
  },
  html: {
    id: "html",
    name: "HTML",
    extensions: ["html", "htm"],
    filenames: ["index.html"],
    comment: { multiLineStart: "<!--", multiLineEnd: "-->" },
    indent: "  ",
    stickyHeaders: ["element"],
    getExtension: () => L.html(),
  },
  xml: {
    id: "xml",
    name: "XML",
    extensions: ["xml", "svg", "xsl", "xslt", "rss", "xhtml", "dtd", "wsdl", "csproj", "fsproj", "vbproj"],
    comment: { multiLineStart: "<!--", multiLineEnd: "-->" },
    indent: "  ",
    stickyHeaders: ["element"],
    getExtension: () => L.xml(),
  },
  css: {
    id: "css",
    name: "CSS",
    extensions: ["css", "pcss", "postcss"],
    comment: { multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "  ",
    stickyHeaders: ["rule_set"],
    getExtension: () => L.css(),
  },
  scss: {
    id: "scss",
    name: "SCSS / Sass",
    extensions: ["scss", "sass"],
    comment: { singleLine: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "  ",
    stickyHeaders: ["rule_set"],
    getExtension: () => L.sass(),
  },
  less: {
    id: "less",
    name: "Less",
    extensions: ["less", "styl"],
    comment: { singleLine: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "  ",
    stickyHeaders: ["rule_set"],
    getExtension: () => L.less(),
  },
  markdown: {
    id: "markdown",
    name: "Markdown",
    extensions: ["md", "markdown", "mdx"],
    filenames: ["README.md", "CHANGELOG.md", "RFC.md"],
    comment: { multiLineStart: "<!--", multiLineEnd: "-->" },
    indent: "  ",
    stickyHeaders: ["atx_heading", "section"],
    getExtension: () => L.markdown(),
  },
  sql: {
    id: "sql",
    name: "SQL",
    extensions: ["sql"],
    comment: { singleLine: "--", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "  ",
    stickyHeaders: ["statement"],
    getExtension: () => L.sql(),
  },
  php: {
    id: "php",
    name: "PHP",
    extensions: ["php", "phtml", "php3", "php4", "php5", "phps"],
    comment: { singleLine: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "    ",
    stickyHeaders: ["function_definition", "class_declaration"],
    getExtension: () => L.php(),
  },
  vue: {
    id: "vue",
    name: "Vue",
    extensions: ["vue"],
    comment: { multiLineStart: "<!--", multiLineEnd: "-->" },
    indent: "  ",
    stickyHeaders: ["template_element", "script_element"],
    getExtension: () => L.vue(),
  },
  svelte: {
    id: "svelte",
    name: "Svelte",
    extensions: ["svelte"],
    comment: { multiLineStart: "<!--", multiLineEnd: "-->" },
    indent: "  ",
    stickyHeaders: ["element"],
    getExtension: () => L.html(),
  },
  shellscript: {
    id: "shellscript",
    name: "Shell Script",
    extensions: ["sh", "bash", "zsh", "fish", "ksh"],
    filenames: [".bashrc", ".zshrc", ".profile"],
    comment: { singleLine: "#" },
    indent: "  ",
    stickyHeaders: ["function_definition"],
    getExtension: () => L.javascript(),
  },
  dockerfile: {
    id: "dockerfile",
    name: "Dockerfile",
    extensions: ["dockerfile"],
    filenames: ["Dockerfile", "Containerfile"],
    comment: { singleLine: "#" },
    indent: "  ",
    stickyHeaders: ["from_instruction"],
    getExtension: () => L.markdown(),
  },
  lua: {
    id: "lua",
    name: "Lua",
    extensions: ["lua"],
    filenames: ["init.lua"],
    comment: { singleLine: "--", multiLineStart: "--[[", multiLineEnd: "]]" },
    indent: "  ",
    stickyHeaders: ["function_declaration"],
    getExtension: () => L.python(),
  },
  ruby: {
    id: "ruby",
    name: "Ruby",
    extensions: ["rb", "rake", "gemspec"],
    filenames: ["Gemfile", "Rakefile"],
    comment: { singleLine: "#", multiLineStart: "=begin", multiLineEnd: "=end" },
    indent: "  ",
    stickyHeaders: ["method", "class"],
    getExtension: () => L.python(),
  },
};

/**
 * Resolves language metadata from a file path.
 */
export function getLanguageMeta(filePath: string): LanguageMeta | null {
  const base = filePath.split("/").pop() || filePath;
  const ext = base.includes(".") ? base.split(".").pop()?.toLowerCase() : "";

  // 1. Filename match
  for (const lang of Object.values(LANGUAGE_REGISTRY)) {
    if (lang.filenames?.some((f) => f.toLowerCase() === base.toLowerCase())) {
      return lang;
    }
  }

  // 2. Extension match
  if (ext) {
    for (const lang of Object.values(LANGUAGE_REGISTRY)) {
      if (lang.extensions.includes(ext)) {
        return lang;
      }
    }
  }

  return null;
}

/**
 * Returns CodeMirror language extension for the file path, loading the
 * grammar package on first use and caching it for subsequent files.
 */
const extensionCache = new Map<string, Promise<Extension>>();
export function resolveLanguageExtension(filePath: string): Promise<Extension> {
  const meta = getLanguageMeta(filePath);
  if (!meta) return Promise.resolve([]);
  let ext = extensionCache.get(meta.id);
  if (!ext) {
    ext = meta.getExtension().catch((err) => {
      extensionCache.delete(meta.id);
      throw err;
    });
    extensionCache.set(meta.id, ext);
  }
  return ext;
}

/**
 * Returns comment tokens for commenting commands.
 */
export function getCommentTokens(filePath: string): CommentTokens {
  const meta = getLanguageMeta(filePath);
  return meta?.comment || { singleLine: "//", multiLineStart: "/*", multiLineEnd: "*/" };
}

/**
 * Returns indent unit string (spaces or tab).
 */
export function getIndentUnit(filePath: string): string {
  const meta = getLanguageMeta(filePath);
  return meta?.indent || "  ";
}
