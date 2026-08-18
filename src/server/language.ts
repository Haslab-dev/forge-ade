import path from "path";

export interface CommentProperties {
  singleLineStart?: string;
  singleLineEnd?: string;
  multiLineStart?: string;
  multiLineEnd?: string;
  multiLinePrefix?: string;
}

export interface TreeSitterProperties {
  grammar?: string;
  grammarFn?: string;
  query?: string;
  stickyHeaders?: string[];
  codeGlance?: {
    include: string[];
    exclude: string[];
  };
}

export interface LanguageDefinition {
  id: string;
  name: string;
  extensions: string[];
  filenames: string[];
  comment: CommentProperties;
  indent: string;
  treeSitter: TreeSitterProperties;
  lspLanguageId: string;
}

const DEFAULT_CODE_GLANCE = {
  include: ["source_file", "function_item", "function_declaration", "class_declaration", "impl_item"],
  exclude: ["source_file"],
};

export const LANGUAGES: Record<string, LanguageDefinition> = {
  rust: {
    id: "rust",
    name: "Rust",
    extensions: ["rs"],
    filenames: ["Cargo.lock"],
    comment: { singleLineStart: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "    ",
    treeSitter: {
      grammar: "tree-sitter-rust",
      grammarFn: "tree_sitter_rust",
      query: "rust",
      stickyHeaders: ["function_item", "impl_item", "trait_item", "struct_item", "enum_item", "mod_item"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "rust",
  },
  go: {
    id: "go",
    name: "Go",
    extensions: ["go"],
    filenames: ["go.mod", "go.sum"],
    comment: { singleLineStart: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "\t",
    treeSitter: {
      grammar: "tree-sitter-go",
      grammarFn: "tree_sitter_go",
      query: "go",
      stickyHeaders: ["function_declaration", "method_declaration", "type_declaration"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "go",
  },
  typescript: {
    id: "typescript",
    name: "TypeScript",
    extensions: ["ts", "mts", "cts"],
    filenames: ["tsconfig.json"],
    comment: { singleLineStart: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "  ",
    treeSitter: {
      grammar: "tree-sitter-typescript",
      grammarFn: "tree_sitter_typescript",
      query: "typescript",
      stickyHeaders: ["function_declaration", "class_declaration", "interface_declaration", "enum_declaration", "method_definition"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "typescript",
  },
  typescriptreact: {
    id: "typescriptreact",
    name: "TypeScript React (TSX)",
    extensions: ["tsx"],
    filenames: [],
    comment: { singleLineStart: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "  ",
    treeSitter: {
      grammar: "tree-sitter-tsx",
      grammarFn: "tree_sitter_tsx",
      query: "tsx",
      stickyHeaders: ["function_declaration", "class_declaration", "interface_declaration", "lexical_declaration"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "typescriptreact",
  },
  javascript: {
    id: "javascript",
    name: "JavaScript",
    extensions: ["js", "mjs", "cjs"],
    filenames: ["package.json"],
    comment: { singleLineStart: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "  ",
    treeSitter: {
      grammar: "tree-sitter-javascript",
      grammarFn: "tree_sitter_javascript",
      query: "javascript",
      stickyHeaders: ["function_declaration", "class_declaration", "method_definition"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "javascript",
  },
  javascriptreact: {
    id: "javascriptreact",
    name: "JavaScript React (JSX)",
    extensions: ["jsx"],
    filenames: [],
    comment: { singleLineStart: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "  ",
    treeSitter: {
      grammar: "tree-sitter-javascript",
      grammarFn: "tree_sitter_javascript",
      query: "jsx",
      stickyHeaders: ["function_declaration", "class_declaration"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "javascriptreact",
  },
  python: {
    id: "python",
    name: "Python",
    extensions: ["py", "pyi", "pyw"],
    filenames: ["Pipfile", "requirements.txt", "pyproject.toml"],
    comment: { singleLineStart: "#", multiLineStart: '"""', multiLineEnd: '"""' },
    indent: "    ",
    treeSitter: {
      grammar: "tree-sitter-python",
      grammarFn: "tree_sitter_python",
      query: "python",
      stickyHeaders: ["function_definition", "class_definition", "async_function_definition"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "python",
  },
  zig: {
    id: "zig",
    name: "Zig",
    extensions: ["zig", "zon"],
    filenames: ["build.zig", "build.zig.zon", "app.zon"],
    comment: { singleLineStart: "//" },
    indent: "    ",
    treeSitter: {
      grammar: "tree-sitter-zig",
      grammarFn: "tree_sitter_zig",
      query: "zig",
      stickyHeaders: ["FnProto", "ContainerDecl", "VarDecl"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "zig",
  },
  cpp: {
    id: "cpp",
    name: "C++",
    extensions: ["cpp", "hpp", "cxx", "hxx", "c++", "h++", "cc", "hh", "C", "H", "ino"],
    filenames: ["CMakeLists.txt"],
    comment: { singleLineStart: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "  ",
    treeSitter: {
      grammar: "tree-sitter-cpp",
      grammarFn: "tree_sitter_cpp",
      query: "cpp",
      stickyHeaders: ["function_definition", "class_specifier", "struct_specifier", "namespace_definition"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "cpp",
  },
  c: {
    id: "c",
    name: "C",
    extensions: ["c", "h"],
    filenames: ["Makefile"],
    comment: { singleLineStart: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "  ",
    treeSitter: {
      grammar: "tree-sitter-c",
      grammarFn: "tree_sitter_c",
      query: "c",
      stickyHeaders: ["function_definition", "struct_specifier", "union_specifier", "enum_specifier"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "c",
  },
  csharp: {
    id: "csharp",
    name: "C#",
    extensions: ["cs", "csx"],
    filenames: [],
    comment: { singleLineStart: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "    ",
    treeSitter: {
      grammar: "tree-sitter-c-sharp",
      grammarFn: "tree_sitter_c_sharp",
      query: "c_sharp",
      stickyHeaders: ["class_declaration", "method_declaration", "struct_declaration", "interface_declaration"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "csharp",
  },
  java: {
    id: "java",
    name: "Java",
    extensions: ["java"],
    filenames: ["pom.xml", "build.gradle"],
    comment: { singleLineStart: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "    ",
    treeSitter: {
      grammar: "tree-sitter-java",
      grammarFn: "tree_sitter_java",
      query: "java",
      stickyHeaders: ["class_declaration", "method_declaration", "interface_declaration", "enum_declaration"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "java",
  },
  kotlin: {
    id: "kotlin",
    name: "Kotlin",
    extensions: ["kt", "kts"],
    filenames: ["build.gradle.kts"],
    comment: { singleLineStart: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "    ",
    treeSitter: {
      grammar: "tree-sitter-kotlin",
      grammarFn: "tree_sitter_kotlin",
      query: "kotlin",
      stickyHeaders: ["class_declaration", "function_declaration"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "kotlin",
  },
  swift: {
    id: "swift",
    name: "Swift",
    extensions: ["swift"],
    filenames: ["Package.swift"],
    comment: { singleLineStart: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "    ",
    treeSitter: {
      grammar: "tree-sitter-swift",
      grammarFn: "tree_sitter_swift",
      query: "swift",
      stickyHeaders: ["function_declaration", "class_declaration", "struct_declaration", "extension_declaration"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "swift",
  },
  dart: {
    id: "dart",
    name: "Dart",
    extensions: ["dart"],
    filenames: ["pubspec.yaml"],
    comment: { singleLineStart: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "  ",
    treeSitter: {
      grammar: "tree-sitter-dart",
      grammarFn: "tree_sitter_dart",
      query: "dart",
      stickyHeaders: ["class_definition", "method_signature", "function_signature"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "dart",
  },
  json: {
    id: "json",
    name: "JSON",
    extensions: ["json", "jsonc", "json5", "geojson"],
    filenames: [".eslintrc", ".prettierrc"],
    comment: { singleLineStart: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "  ",
    treeSitter: {
      grammar: "tree-sitter-json",
      grammarFn: "tree_sitter_json",
      query: "json",
      stickyHeaders: ["object", "pair"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "json",
  },
  toml: {
    id: "toml",
    name: "TOML",
    extensions: ["toml"],
    filenames: ["Cargo.toml", "settings.toml", "keymaps.toml"],
    comment: { singleLineStart: "#" },
    indent: "  ",
    treeSitter: {
      grammar: "tree-sitter-toml",
      grammarFn: "tree_sitter_toml",
      query: "toml",
      stickyHeaders: ["table", "table_array_element"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "toml",
  },
  yaml: {
    id: "yaml",
    name: "YAML",
    extensions: ["yaml", "yml"],
    filenames: [".gitlab-ci.yml", "docker-compose.yml"],
    comment: { singleLineStart: "#" },
    indent: "  ",
    treeSitter: {
      grammar: "tree-sitter-yaml",
      grammarFn: "tree_sitter_yaml",
      query: "yaml",
      stickyHeaders: ["block_mapping_pair"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "yaml",
  },
  html: {
    id: "html",
    name: "HTML",
    extensions: ["html", "htm", "xhtml"],
    filenames: ["index.html"],
    comment: { multiLineStart: "<!--", multiLineEnd: "-->" },
    indent: "  ",
    treeSitter: {
      grammar: "tree-sitter-html",
      grammarFn: "tree_sitter_html",
      query: "html",
      stickyHeaders: ["element"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "html",
  },
  xml: {
    id: "xml",
    name: "XML",
    extensions: ["xml", "svg", "xsl", "xslt", "rss", "dtd", "wsdl", "csproj", "fsproj", "vbproj"],
    filenames: [],
    comment: { multiLineStart: "<!--", multiLineEnd: "-->" },
    indent: "  ",
    treeSitter: {
      grammar: "tree-sitter-xml",
      grammarFn: "tree_sitter_xml",
      query: "xml",
      stickyHeaders: ["element"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "xml",
  },
  css: {
    id: "css",
    name: "CSS",
    extensions: ["css", "pcss", "postcss"],
    filenames: [],
    comment: { multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "  ",
    treeSitter: {
      grammar: "tree-sitter-css",
      grammarFn: "tree_sitter_css",
      query: "css",
      stickyHeaders: ["rule_set", "media_statement"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "css",
  },
  scss: {
    id: "scss",
    name: "SCSS / Sass",
    extensions: ["scss", "sass"],
    filenames: [],
    comment: { singleLineStart: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "  ",
    treeSitter: {
      grammar: "tree-sitter-scss",
      grammarFn: "tree_sitter_scss",
      query: "scss",
      stickyHeaders: ["rule_set"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "scss",
  },
  less: {
    id: "less",
    name: "Less",
    extensions: ["less", "styl"],
    filenames: [],
    comment: { singleLineStart: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "  ",
    treeSitter: {
      grammar: "tree-sitter-less",
      grammarFn: "tree_sitter_less",
      query: "less",
      stickyHeaders: ["rule_set"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "less",
  },
  markdown: {
    id: "markdown",
    name: "Markdown",
    extensions: ["md", "markdown", "mdx"],
    filenames: ["README.md", "CHANGELOG.md", "RFC.md"],
    comment: { multiLineStart: "<!--", multiLineEnd: "-->" },
    indent: "  ",
    treeSitter: {
      grammar: "tree-sitter-markdown",
      grammarFn: "tree_sitter_markdown",
      query: "markdown",
      stickyHeaders: ["atx_heading", "setext_heading", "section"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "markdown",
  },
  sql: {
    id: "sql",
    name: "SQL",
    extensions: ["sql"],
    filenames: [],
    comment: { singleLineStart: "--", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "  ",
    treeSitter: {
      grammar: "tree-sitter-sql",
      grammarFn: "tree_sitter_sql",
      query: "sql",
      stickyHeaders: ["statement"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "sql",
  },
  php: {
    id: "php",
    name: "PHP",
    extensions: ["php", "phtml", "php3", "php4", "php5", "phps"],
    filenames: [],
    comment: { singleLineStart: "//", multiLineStart: "/*", multiLineEnd: "*/" },
    indent: "    ",
    treeSitter: {
      grammar: "tree-sitter-php",
      grammarFn: "tree_sitter_php",
      query: "php",
      stickyHeaders: ["function_definition", "class_declaration", "method_declaration"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "php",
  },
  vue: {
    id: "vue",
    name: "Vue",
    extensions: ["vue"],
    filenames: [],
    comment: { multiLineStart: "<!--", multiLineEnd: "-->" },
    indent: "  ",
    treeSitter: {
      grammar: "tree-sitter-vue",
      grammarFn: "tree_sitter_vue",
      query: "vue",
      stickyHeaders: ["template_element", "script_element", "style_element"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "vue",
  },
  svelte: {
    id: "svelte",
    name: "Svelte",
    extensions: ["svelte"],
    filenames: [],
    comment: { multiLineStart: "<!--", multiLineEnd: "-->" },
    indent: "  ",
    treeSitter: {
      grammar: "tree-sitter-svelte",
      grammarFn: "tree_sitter_svelte",
      query: "svelte",
      stickyHeaders: ["element"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "svelte",
  },
  shellscript: {
    id: "shellscript",
    name: "Shell Script",
    extensions: ["sh", "bash", "zsh", "fish", "ksh"],
    filenames: [".bashrc", ".zshrc", ".bash_profile", ".profile"],
    comment: { singleLineStart: "#" },
    indent: "  ",
    treeSitter: {
      grammar: "tree-sitter-bash",
      grammarFn: "tree_sitter_bash",
      query: "bash",
      stickyHeaders: ["function_definition"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "shellscript",
  },
  dockerfile: {
    id: "dockerfile",
    name: "Dockerfile",
    extensions: ["dockerfile"],
    filenames: ["Dockerfile", "Containerfile"],
    comment: { singleLineStart: "#" },
    indent: "  ",
    treeSitter: {
      grammar: "tree-sitter-dockerfile",
      grammarFn: "tree_sitter_dockerfile",
      query: "dockerfile",
      stickyHeaders: ["from_instruction"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "dockerfile",
  },
  lua: {
    id: "lua",
    name: "Lua",
    extensions: ["lua"],
    filenames: ["init.lua"],
    comment: { singleLineStart: "--", multiLineStart: "--[[", multiLineEnd: "]]" },
    indent: "  ",
    treeSitter: {
      grammar: "tree-sitter-lua",
      grammarFn: "tree_sitter_lua",
      query: "lua",
      stickyHeaders: ["function_declaration", "local_function"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "lua",
  },
  ruby: {
    id: "ruby",
    name: "Ruby",
    extensions: ["rb", "rake", "gemspec"],
    filenames: ["Gemfile", "Rakefile"],
    comment: { singleLineStart: "#", multiLineStart: "=begin", multiLineEnd: "=end" },
    indent: "  ",
    treeSitter: {
      grammar: "tree-sitter-ruby",
      grammarFn: "tree_sitter_ruby",
      query: "ruby",
      stickyHeaders: ["method", "class", "module"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "ruby",
  },
  makefile: {
    id: "makefile",
    name: "Makefile",
    extensions: ["mk", "mak"],
    filenames: ["Makefile", "makefile", "GNUMakefile"],
    comment: { singleLineStart: "#" },
    indent: "\t",
    treeSitter: {
      grammar: "tree-sitter-make",
      grammarFn: "tree_sitter_make",
      query: "make",
      stickyHeaders: ["rule"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "makefile",
  },
  diff: {
    id: "diff",
    name: "Diff",
    extensions: ["diff", "patch"],
    filenames: [],
    comment: {},
    indent: " ",
    treeSitter: {
      grammar: "tree-sitter-diff",
      grammarFn: "tree_sitter_diff",
      query: "diff",
      stickyHeaders: ["file_change"],
      codeGlance: DEFAULT_CODE_GLANCE,
    },
    lspLanguageId: "diff",
  },
};

/**
 * Resolves a language definition from a file path using extensions and filenames,
 * matching Lapce's `language_id_from_path` logic.
 */
export function getLanguageFromPath(filePath: string): LanguageDefinition | null {
  const filename = path.basename(filePath);
  const ext = path.extname(filePath).replace(/^\./, "").toLowerCase();

  // 1. Exact filename match (e.g. Dockerfile, Cargo.toml, package.json, app.zon)
  for (const lang of Object.values(LANGUAGES)) {
    if (lang.filenames.some((f) => f.toLowerCase() === filename.toLowerCase())) {
      return lang;
    }
  }

  // 2. Extension match
  if (ext) {
    for (const lang of Object.values(LANGUAGES)) {
      if (lang.extensions.includes(ext)) {
        return lang;
      }
    }
  }

  return null;
}

/**
 * Returns the LSP language ID from a file path, matching Lapce's `language_id_from_path`.
 */
export function languageIdFromPath(filePath: string): string {
  const lang = getLanguageFromPath(filePath);
  return lang ? lang.lspLanguageId : "plaintext";
}
