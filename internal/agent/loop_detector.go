package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"sync"
)

const (
	DefaultLoopWindowSize = 10
	DefaultLoopMaxRepeats = 4
)

// ToolExecutionSignature represents an observed tool interaction (name, input, and result).
type ToolExecutionSignature struct {
	ToolName string
	Input    string
	Output   string
	Hash     string
}

// LoopDetector detects cyclic / spinning tool calls to prevent runaway agent loops
// and token exhaustion when the model gets stuck repeating failed operations.
type LoopDetector struct {
	mu                    sync.Mutex
	windowSize            int
	maxRepeats            int
	window                []ToolExecutionSignature
	hasLoop               bool
	loopTool              string
	loopCount             int
	consecutiveInputHash  string
	consecutiveInputCount int
	consecutiveInputTool  string
}

// NewLoopDetector initializes a LoopDetector with configurable sliding window and repeat threshold.
func NewLoopDetector(windowSize, maxRepeats int) *LoopDetector {
	if windowSize <= 0 {
		windowSize = DefaultLoopWindowSize
	}
	if maxRepeats <= 0 {
		maxRepeats = DefaultLoopMaxRepeats
	}
	return &LoopDetector{
		windowSize: windowSize,
		maxRepeats: maxRepeats,
		window:     make([]ToolExecutionSignature, 0, windowSize),
	}
}

// ComputeSignature calculates a stable hex-encoded SHA-256 hash for a tool execution.
func ComputeSignature(toolName, input, output string) string {
	h := sha256.New()
	io.WriteString(h, toolName)
	io.WriteString(h, "\x00")
	io.WriteString(h, input)
	io.WriteString(h, "\x00")
	io.WriteString(h, output)
	io.WriteString(h, "\x00")
	return hex.EncodeToString(h.Sum(nil))
}

// ComputeInputHash calculates a stable hex-encoded SHA-256 hash for a tool input.
func ComputeInputHash(toolName, input string) string {
	h := sha256.New()
	io.WriteString(h, toolName)
	io.WriteString(h, "\x00")
	io.WriteString(h, input)
	return hex.EncodeToString(h.Sum(nil))
}

// Record adds a tool interaction to the sliding window and evaluates whether
// the agent has entered a repetitive loop.
// Returns (isLoop, repeatCount, offendingToolName).
func (d *LoopDetector) Record(toolName, input, output string) (bool, int, string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	hash := ComputeSignature(toolName, input, output)
	sig := ToolExecutionSignature{
		ToolName: toolName,
		Input:    input,
		Output:   output,
		Hash:     hash,
	}

	// 1. Consecutive identical input loop check
	inputHash := ComputeInputHash(toolName, input)
	if inputHash == d.consecutiveInputHash {
		d.consecutiveInputCount++
	} else {
		d.consecutiveInputHash = inputHash
		d.consecutiveInputCount = 1
		d.consecutiveInputTool = toolName
	}

	if d.consecutiveInputCount > d.maxRepeats {
		d.hasLoop = true
		d.loopTool = toolName
		d.loopCount = d.consecutiveInputCount
		return true, d.consecutiveInputCount, toolName
	}

	d.window = append(d.window, sig)
	if len(d.window) > d.windowSize {
		d.window = d.window[len(d.window)-d.windowSize:]
	}

	// Count occurrences of signatures within current sliding window
	counts := make(map[string]int)
	toolNames := make(map[string]string)
	for _, entry := range d.window {
		counts[entry.Hash]++
		toolNames[entry.Hash] = entry.ToolName
	}

	maxCount := 0
	loopTool := ""
	for h, count := range counts {
		if count > maxCount {
			maxCount = count
			loopTool = toolNames[h]
		}
	}

	if maxCount > d.maxRepeats {
		d.hasLoop = true
		d.loopTool = loopTool
		d.loopCount = maxCount
		return true, maxCount, loopTool
	}

	return false, maxCount, ""
}

// HasLoop checks if a loop has been flagged.
func (d *LoopDetector) HasLoop() (bool, string, int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.hasLoop, d.loopTool, d.loopCount
}

// Reset clears the sliding window history.
func (d *LoopDetector) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.window = d.window[:0]
	d.hasLoop = false
	d.loopTool = ""
	d.loopCount = 0
	d.consecutiveInputHash = ""
	d.consecutiveInputCount = 0
	d.consecutiveInputTool = ""
}

// FormatLoopWarning creates a descriptive diagnostic message when a loop is detected.
func FormatLoopWarning(toolName string, repeatCount int) string {
	return fmt.Sprintf("Agent circuit breaker triggered: tool '%s' was invoked with identical input and output %d times within recent steps without progress. Halting turn to prevent runaway token spend.", toolName, repeatCount)
}
