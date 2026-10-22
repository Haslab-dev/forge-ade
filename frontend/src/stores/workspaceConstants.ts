import { CustomSlashCommand, AgentHookConfig, BrowserUseSettings, ComputerUseSettings } from '../types';

export const DEFAULT_SLASH_COMMANDS: CustomSlashCommand[] = [
  {
    id: 'cmd-review',
    name: 'review',
    description: 'Perform a comprehensive code review focusing on bugs, security, and performance.',
    promptTemplate: 'Perform a comprehensive code review of the recently modified files. Identify potential bugs, security issues, and edge cases, and propose concrete improvements.',
    scope: 'workspace',
    enabled: true
  },
  {
    id: 'cmd-test',
    name: 'test',
    description: 'Analyze code coverage, run tests, and generate missing unit tests.',
    promptTemplate: 'Run the project test suite, analyze test output, and generate tests for uncovered critical edge cases in the active files.',
    scope: 'workspace',
    enabled: true
  },
  {
    id: 'cmd-lint',
    name: 'lint',
    description: 'Fix syntax, lint, and formatting errors in the current workspace.',
    promptTemplate: 'Check for compiler errors, lint warnings, and formatting issues. Fix any detected problems while preserving the existing logic.',
    scope: 'workspace',
    enabled: true
  },
  {
    id: 'cmd-plan',
    name: 'plan',
    description: 'Formulate an architectural implementation plan before modifying files.',
    promptTemplate: 'Carefully analyze the requested feature, identify all affected files and packages, outline architectural considerations, and provide a numbered step-by-step implementation plan before making edits.',
    scope: 'workspace',
    enabled: true
  },
  {
    id: 'cmd-commit',
    name: 'commit',
    description: 'Generate conventional commit messages and staged diff summaries.',
    promptTemplate: 'Inspect the current git status and staged changes. Summarize the changes following conventional commit syntax (feat, fix, refactor, docs) with clear bullet points.',
    scope: 'workspace',
    enabled: true
  }
];

export const DEFAULT_HOOKS: AgentHookConfig[] = [
  {
    id: 'hook-pre-turn',
    name: 'Pre-turn Environment Check',
    event: 'pre_turn',
    command: 'git status --porcelain',
    enabled: false,
    timeout: 5
  },
  {
    id: 'hook-post-file-write',
    name: 'Auto-format on File Write',
    event: 'post_file_write',
    command: 'go fmt ./... 2>/dev/null || prettier --write "$FORGE_FILE" 2>/dev/null',
    enabled: true,
    timeout: 10
  },
  {
    id: 'hook-post-turn',
    name: 'Post-turn Lint & Typecheck',
    event: 'post_turn',
    command: 'npm run lint --if-present 2>/dev/null || go vet ./... 2>/dev/null',
    enabled: false,
    timeout: 15
  },
  {
    id: 'hook-pre-commit',
    name: 'Pre-commit Smoke Test',
    event: 'pre_commit',
    command: 'npm test -- --bail 2>/dev/null || go test -short ./... 2>/dev/null',
    enabled: false,
    timeout: 30
  }
];

export const DEFAULT_BROWSER_SETTINGS: BrowserUseSettings = {
  enabled: true,
  headless: true,
  searchProvider: 'google',
  searchApiKey: '',
  remoteCdpUrl: '',
  viewport: '1280x800',
  maxExtractLength: 50000,
  allowJavaScript: false
};

export const DEFAULT_COMPUTER_SETTINGS: ComputerUseSettings = {
  permissionMode: 'confirm_dangerous',
  defaultShell: '/bin/zsh',
  commandTimeoutSeconds: 30,
  allowClipboard: true,
  screenCaptureScale: 1.0,
  terminalSandbox: true
};
