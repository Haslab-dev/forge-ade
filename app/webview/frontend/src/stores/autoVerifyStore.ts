import { create } from 'zustand';
import { ExecuteCommandSync } from '../lib/wails';
import { showToast } from '../lib/toast';

export interface AutoVerifyResult {
  command: string;
  exitCode: number;
  output: string;
  timestamp: number;
}

export interface AutoVerifyState {
  enabled: boolean;
  customCommand: string;
  isVerifying: boolean;
  lastResult: AutoVerifyResult | null;
  retryCounts: Record<string, number>;

  setEnabled: (enabled: boolean) => void;
  toggleEnabled: () => void;
  setCustomCommand: (cmd: string) => void;
  resetRetries: (sessionId: string) => void;
  detectCommand: (workspacePath: string) => Promise<string>;
  runVerification: (
    sessionId: string,
    workspacePath: string,
    onFailure?: (failurePrompt: string) => void
  ) => Promise<{ success: boolean; output: string; command: string }>;
}

const STORAGE_KEY_ENABLED = 'forge:auto-verify:enabled';
const STORAGE_KEY_CMD = 'forge:auto-verify:cmd';

function loadInitialState(): { enabled: boolean; customCommand: string } {
  let enabled = false;
  let customCommand = '';
  try {
    const rawEnabled = localStorage.getItem(STORAGE_KEY_ENABLED);
    if (rawEnabled !== null) enabled = rawEnabled === 'true';
    const rawCmd = localStorage.getItem(STORAGE_KEY_CMD);
    if (rawCmd) customCommand = rawCmd;
  } catch {}
  return { enabled, customCommand };
}

export const useAutoVerifyStore = create<AutoVerifyState>((set, get) => {
  const initial = loadInitialState();

  return {
    enabled: initial.enabled,
    customCommand: initial.customCommand,
    isVerifying: false,
    lastResult: null,
    retryCounts: {},

    setEnabled: (enabled: boolean) => {
      set({ enabled });
      try {
        localStorage.setItem(STORAGE_KEY_ENABLED, String(enabled));
      } catch {}
    },

    toggleEnabled: () => {
      const next = !get().enabled;
      get().setEnabled(next);
      if (next) {
        showToast('Auto-verification enabled. Tests will run after changes.', 'info');
      } else {
        showToast('Auto-verification disabled.', 'info');
      }
    },

    setCustomCommand: (cmd: string) => {
      set({ customCommand: cmd });
      try {
        localStorage.setItem(STORAGE_KEY_CMD, cmd);
      } catch {}
    },

    resetRetries: (sessionId: string) => {
      set(state => {
        const next = { ...state.retryCounts };
        delete next[sessionId];
        return { retryCounts: next };
      });
    },

    detectCommand: async (workspacePath: string) => {
      const custom = get().customCommand.trim();
      if (custom) return custom;
      if (!workspacePath) return '';

      try {
        const detectScript =
          'if [ -f package.json ]; then if command -v bun >/dev/null 2>&1; then echo -n "bun test"; else echo -n "npm test"; fi; ' +
          'elif [ -f go.mod ]; then echo -n "go test ./..."; ' +
          'elif [ -f Cargo.toml ]; then echo -n "cargo test"; ' +
          'elif [ -f pytest.ini ] || [ -f pyproject.toml ]; then echo -n "pytest"; fi';
        const res = await ExecuteCommandSync(detectScript, workspacePath);
        return (res?.stdout || '').trim();
      } catch {
        return '';
      }
    },

    runVerification: async (sessionId: string, workspacePath: string, onFailure) => {
      if (!get().enabled) {
        return { success: true, output: '', command: '' };
      }

      const retries = get().retryCounts[sessionId] || 0;
      if (retries >= 2) {
        showToast(
          'Auto-verification paused: 2 consecutive self-correction attempts reached.',
          'warn'
        );
        return { success: false, output: 'Max retries reached', command: '' };
      }

      const cmd = await get().detectCommand(workspacePath);
      if (!cmd) {
        return { success: true, output: 'No test suite detected', command: '' };
      }

      set({ isVerifying: true });
      try {
        const res = await ExecuteCommandSync(cmd, workspacePath);
        const exitCode = typeof res?.exitCode === 'number' ? res.exitCode : (res ? 0 : 1);
        const combinedOutput = ((res?.stdout || '') + '\n' + (res?.stderr || '')).trim();

        const result: AutoVerifyResult = {
          command: cmd,
          exitCode,
          output: combinedOutput,
          timestamp: Date.now()
        };
        set({ lastResult: result });

        if (exitCode === 0) {
          get().resetRetries(sessionId);
          showToast(`✓ Auto-verify passed: ${cmd}`, 'success');
          return { success: true, output: combinedOutput, command: cmd };
        }

        // Verification failed -> trigger self-correction
        set(state => ({
          retryCounts: {
            ...state.retryCounts,
            [sessionId]: retries + 1
          }
        }));

        showToast(
          `⚠️ Auto-verify failed (${exitCode}): triggering self-correction...`,
          'warn'
        );

        if (onFailure) {
          const trimmedOutput = combinedOutput.length > 2000
            ? combinedOutput.slice(-2000)
            : combinedOutput;

          const prompt =
            `⚠️ **Auto-Verification Failed**\n` +
            `The test command \`${cmd}\` failed with exit code ${exitCode}.\n\n` +
            `**Test Output:**\n` +
            `\`\`\`\n${trimmedOutput}\n\`\`\`\n\n` +
            `Please analyze the test failure above, inspect recent changes, and fix the issues so the tests pass.`;

          onFailure(prompt);
        }

        return { success: false, output: combinedOutput, command: cmd };
      } catch (err: any) {
        showToast(`Auto-verify error: ${err?.message || 'Command failed'}`, 'error');
        return { success: false, output: err?.message || 'Error', command: cmd };
      } finally {
        set({ isVerifying: false });
      }
    }
  };
});
