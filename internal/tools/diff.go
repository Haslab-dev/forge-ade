package tools

import (
	"fmt"
	"strings"
)

// unifiedDiff returns a compact line-based unified diff (2 context lines)
// between oldText and newText, so write/edit tool results show the model
// exactly what changed instead of echoing whole files. Inputs whose line-count
// product exceeds the cap fall back to a one-line summary — exact LCS on huge
// files costs O(n*m) memory and agent edits are usually small.
func unifiedDiff(oldText, newText string) string {
	oldLines := splitLines(oldText)
	newLines := splitLines(newText)
	if len(oldLines)*len(newLines) > 1_000_000 {
		return fmt.Sprintf("%d lines -> %d lines (exact diff skipped: too large)", len(oldLines), len(newLines))
	}
	ops := lcsOps(oldLines, newLines)

	type dl struct {
		op   byte
		text string
	}
	aligned := make([]dl, 0, len(ops))
	oi, ni := 0, 0
	for _, op := range ops {
		switch op {
		case ' ':
			aligned = append(aligned, dl{' ', oldLines[oi]})
			oi++
			ni++
		case '-':
			aligned = append(aligned, dl{'-', oldLines[oi]})
			oi++
		case '+':
			aligned = append(aligned, dl{'+', newLines[ni]})
			ni++
		}
	}

	const ctx = 2
	var b strings.Builder
	for i := 0; i < len(aligned); {
		if aligned[i].op == ' ' {
			i++
			continue
		}
		// Extend while the gap to the next change is <= ctx context lines.
		start, end := i, i
		for end+1 < len(aligned) {
			j := end + 1
			for j < len(aligned) && aligned[j].op == ' ' {
				j++
			}
			if j < len(aligned) && j-(end+1) <= ctx {
				end = j
			} else {
				break
			}
		}
		hStart := start
		for hStart > 0 && aligned[hStart-1].op == ' ' && start-hStart < ctx {
			hStart--
		}
		hEnd := end
		for hEnd+1 < len(aligned) && aligned[hEnd+1].op == ' ' && hEnd-end < ctx {
			hEnd++
		}
		oldLine, newLine := 1, 1
		for k := 0; k < hStart; k++ {
			if aligned[k].op != '+' {
				oldLine++
			}
			if aligned[k].op != '-' {
				newLine++
			}
		}
		oldCount, newCount := 0, 0
		for k := hStart; k <= hEnd; k++ {
			if aligned[k].op != '+' {
				oldCount++
			}
			if aligned[k].op != '-' {
				newCount++
			}
		}
		fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", oldLine, oldCount, newLine, newCount)
		for k := hStart; k <= hEnd; k++ {
			b.WriteByte(aligned[k].op)
			b.WriteString(aligned[k].text)
			b.WriteByte('\n')
		}
		i = end + 1
	}
	out := b.String()
	if len(out) > 12<<10 {
		out = out[:12<<10] + "\n... (diff truncated)\n"
	}
	return strings.TrimRight(out, "\n")
}

// lcsOps backtracks the LCS DP table into an op sequence: ' ' equal, '-' only
// in old, '+' only in new.
func lcsOps(a, b []string) []byte {
	n, m := len(a), len(b)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}
	ops := make([]byte, 0, n+m)
	i, j := n, m
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			ops = append(ops, ' ')
			i--
			j--
		} else if dp[i-1][j] >= dp[i][j-1] {
			ops = append(ops, '-')
			i--
		} else {
			ops = append(ops, '+')
			j--
		}
	}
	for i > 0 {
		ops = append(ops, '-')
		i--
	}
	for j > 0 {
		ops = append(ops, '+')
		j--
	}
	for l, r := 0, len(ops)-1; l < r; l, r = l+1, r-1 {
		ops[l], ops[r] = ops[r], ops[l]
	}
	return ops
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
