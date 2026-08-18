import fs from "fs";
import path from "path";
import os from "os";
import { LanguageDefinition, getLanguageFromPath, LANGUAGES } from "./language";

export interface HighlightQueryInfo {
  languageId: string;
  queryPath: string;
  content: string;
}

export interface GrammarInfo {
  languageId: string;
  name: string;
  path: string;
  source: "user" | "system" | "builtin";
}

export interface HighlightToken {
  line: number;
  startCol: number;
  endCol: number;
  scope: string; // e.g. "keyword", "function", "string", "comment", "type", "variable"
}

export class SyntaxManager {
  private userGrammarDir: string;
  private systemGrammarDirs: string[];
  private queryDirs: string[];
  private queryCache = new Map<string, string>();

  constructor() {
    const home = os.homedir();
    // $LOCAL_DATA_DIR/grammars & $SETTINGS_DIR/queries/$LANGUAGE/highlights.scm (Lapce Spec)
    const localDataDir = process.env.LOCAL_DATA_DIR || path.join(home, ".local", "share", "forge-ade");
    const settingsDir = process.env.SETTINGS_DIR || path.join(home, ".config", "forge-ade");

    this.userGrammarDir = path.join(localDataDir, "grammars");
    this.systemGrammarDirs = [
      "/usr/local/lib/tree-sitter",
      "/usr/lib/tree-sitter",
      "/opt/homebrew/lib",
    ];
    this.queryDirs = [
      path.join(settingsDir, "queries"),
      path.join(localDataDir, "queries"),
      path.join(process.cwd(), "queries"),
    ];

    this.ensureDirs();
  }

  private ensureDirs() {
    try {
      if (!fs.existsSync(this.userGrammarDir)) {
        fs.mkdirSync(this.userGrammarDir, { recursive: true });
      }
    } catch {}
  }

  /**
   * Find available Tree-sitter grammars in order:
   * 1. User provided grammar ($LOCAL_DATA_DIR/grammars)
   * 2. System provided grammar
   * 3. Built-in grammar
   */
  public findGrammar(langId: string): GrammarInfo | null {
    const lang = LANGUAGES[langId];
    if (!lang || !lang.treeSitter.grammar) return null;

    const grammarBaseName = lang.treeSitter.grammar;
    const dllExt = process.platform === "darwin" ? ".dylib" : process.platform === "win32" ? ".dll" : ".so";
    const dllPrefix = process.platform === "win32" ? "" : "lib";

    const candidateFiles = [
      `${grammarBaseName}${dllExt}`,
      `${dllPrefix}${grammarBaseName}${dllExt}`,
      `${grammarBaseName}.wasm`,
    ];

    // 1. User provided grammar
    for (const file of candidateFiles) {
      const userPath = path.join(this.userGrammarDir, file);
      if (fs.existsSync(userPath)) {
        return {
          languageId: langId,
          name: grammarBaseName,
          path: userPath,
          source: "user",
        };
      }
    }

    // 2. System provided grammar
    for (const sysDir of this.systemGrammarDirs) {
      for (const file of candidateFiles) {
        const sysPath = path.join(sysDir, file);
        if (fs.existsSync(sysPath)) {
          return {
            languageId: langId,
            name: grammarBaseName,
            path: sysPath,
            source: "system",
          };
        }
      }
    }

    // 3. Built-in grammar indicator
    return {
      languageId: langId,
      name: grammarBaseName,
      path: `builtin://${grammarBaseName}`,
      source: "builtin",
    };
  }

  /**
   * Load highlights.scm query for the specified language.
   * Checks $SETTINGS_DIR/queries/$LANGUAGE/highlights.scm first, then fallback defaults.
   */
  public getHighlightQuery(langId: string): HighlightQueryInfo | null {
    const lang = LANGUAGES[langId];
    if (!lang) return null;

    const queryFolder = lang.treeSitter.query || langId;
    if (this.queryCache.has(queryFolder)) {
      return {
        languageId: langId,
        queryPath: "cached",
        content: this.queryCache.get(queryFolder)!,
      };
    }

    for (const qDir of this.queryDirs) {
      const queryFile = path.join(qDir, queryFolder, "highlights.scm");
      if (fs.existsSync(queryFile)) {
        try {
          const content = fs.readFileSync(queryFile, "utf-8");
          this.queryCache.set(queryFolder, content);
          return {
            languageId: langId,
            queryPath: queryFile,
            content,
          };
        } catch {}
      }
    }

    // Default embedded highlight queries for core languages
    const defaultQuery = this.getDefaultHighlightQuery(queryFolder);
    if (defaultQuery) {
      this.queryCache.set(queryFolder, defaultQuery);
      return {
        languageId: langId,
        queryPath: `embedded://${queryFolder}/highlights.scm`,
        content: defaultQuery,
      };
    }

    return null;
  }

  /**
   * Return sticky header AST node names for the language.
   */
  public getStickyHeaders(langId: string): string[] {
    const lang = LANGUAGES[langId];
    return lang?.treeSitter.stickyHeaders || [];
  }

  /**
   * Tokenize content for syntax highlighting and symbol outlines.
   */
  public tokenize(filePath: string, content: string): HighlightToken[] {
    const lang = getLanguageFromPath(filePath);
    if (!lang) return [];

    const tokens: HighlightToken[] = [];
    const lines = content.split("\n");

    const keywordPatterns = [
      { regex: /\b(func|fn|function|def|class|struct|enum|interface|type|impl|trait|mod|package|import|export|from|return|if|else|switch|case|for|while|loop|match|break|continue|async|await|const|let|var|val|mut|pub|private|protected|public)\b/g, scope: "keyword" },
      { regex: /\b(true|false|nil|null|None|undefined|Some|None|Ok|Err)\b/g, scope: "constant" },
      { regex: /\b(i8|i16|i32|i64|u8|u16|u32|u64|isize|usize|f32|f64|bool|char|str|String|int|string|boolean|any|void)\b/g, scope: "type" },
      { regex: /(["'`])(?:(?=(\\?))\2[\s\S])*?\1/g, scope: "string" },
      { regex: /\b\d+(?:\.\d+)?(?:[eE][+-]?\d+)?\b/g, scope: "number" },
    ];

    lines.forEach((line, idx) => {
      const lineNum = idx + 1;

      // Comment token
      if (lang.comment.singleLineStart) {
        const commentIdx = line.indexOf(lang.comment.singleLineStart);
        if (commentIdx >= 0) {
          tokens.push({
            line: lineNum,
            startCol: commentIdx,
            endCol: line.length,
            scope: "comment",
          });
        }
      }

      // Keyword & constant tokens
      for (const p of keywordPatterns) {
        let match: RegExpExecArray | null;
        p.regex.lastIndex = 0;
        while ((match = p.regex.exec(line)) !== null) {
          tokens.push({
            line: lineNum,
            startCol: match.index,
            endCol: match.index + match[0].length,
            scope: p.scope,
          });
        }
      }
    });

    return tokens;
  }

  private getDefaultHighlightQuery(langName: string): string {
    return `; Tree-sitter highlight query for ${langName}
(comment) @comment
(string_literal) @string
(number_literal) @number
(keyword) @keyword
(type_identifier) @type
(function_item name: (identifier) @function)
(function_declaration name: (identifier) @function)
(method_declaration name: (identifier) @function.method)
(call_expression function: (identifier) @function.call)
(variable_declarator name: (identifier) @variable)
`;
  }
}
