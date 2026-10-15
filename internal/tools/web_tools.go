package tools

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	ignoredTagRegexes = func() []*regexp.Regexp {
		tags := []string{"script", "style", "noscript", "svg", "iframe", "header", "footer", "nav", "aside"}
		res := make([]*regexp.Regexp, len(tags))
		for i, tag := range tags {
			res[i] = regexp.MustCompile(fmt.Sprintf(`(?is)<%s[^>]*>.*?</%s>`, tag, tag))
		}
		return res
	}()
	tagTitleRegex      = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	tagH1Regex         = regexp.MustCompile(`(?is)<h1[^>]*>(.*?)</h1>`)
	tagH2Regex         = regexp.MustCompile(`(?is)<h2[^>]*>(.*?)</h2>`)
	tagH3Regex         = regexp.MustCompile(`(?is)<h3[^>]*>(.*?)</h3>`)
	tagH4Regex         = regexp.MustCompile(`(?is)<h[4-6][^>]*>(.*?)</h[4-6]>`)
	tagLiRegex         = regexp.MustCompile(`(?is)<li[^>]*>(.*?)</li>`)
	tagPRegex          = regexp.MustCompile(`(?is)<p[^>]*>(.*?)</p>`)
	tagAnchorRegex     = regexp.MustCompile(`(?is)<a\s+[^>]*href=["']([^"']+)["'][^>]*>(.*?)</a>`)
	tagCodeRegex       = regexp.MustCompile(`(?is)<pre[^>]*><code[^>]*>(.*?)</code></pre>`)
	tagInlineCodeRegex = regexp.MustCompile(`(?is)<code[^>]*>(.*?)</code>`)
	tagBrRegex         = regexp.MustCompile(`(?i)<br\s*/?>`)
	tagStripAnyRegex   = regexp.MustCompile(`(?s)<[^>]+>`)
	multiNewlineRegex  = regexp.MustCompile(`\n{3,}`)
	multiSpaceRegex    = regexp.MustCompile(`[ \t]+`)

	// DuckDuckGo result scrapers
	ddgResultRegex = regexp.MustCompile(`(?is)<a[^>]*class="[^"]*result__snippet[^"]*"[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
	ddgTitleRegex  = regexp.MustCompile(`(?is)<h2[^>]*class="[^"]*result__title[^"]*"[^>]*>\s*<a[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
	ddgLiteRegex   = regexp.MustCompile(`(?is)<td[^>]*class="result-snippet"[^>]*>(.*?)</td>`)
	ddgLiteLink    = regexp.MustCompile(`(?is)<a[^>]*class="result-link"[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
)

func webFetchTool() ToolSpec {
	return ToolSpec{
		Name:        "web_fetch",
		Description: "Fast HTTP web fetch and text/markdown extractor. Retrieves URL content, extracts clean markdown text without spinning up a browser. Use this tool for reading web documentation, API references, articles, and GitHub pages.",
		Cost:        "medium",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "Full HTTP or HTTPS URL to fetch (e.g. https://docs.example.com/api)",
				},
				"max_length": map[string]any{
					"type":        "integer",
					"description": "Max characters of content to return (default 25000, max 60000)",
				},
			},
			"required": []string{"url"},
		},
		Handler: handleWebFetch,
	}
}

func handleWebFetch(ctx context.Context, args map[string]any) (any, error) {
	rawURL := strings.TrimSpace(argString(args, "url"))
	if rawURL == "" {
		return nil, fmt.Errorf("url parameter is required")
	}
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Host == "" {
		return nil, fmt.Errorf("invalid URL %q: %v", rawURL, err)
	}

	maxLen := argInt(args, "max_length", 25000)
	if maxLen <= 0 {
		maxLen = 25000
	}
	if maxLen > 60000 {
		maxLen = 60000
	}

	client := &http.Client{
		Timeout: 20 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 6 {
				return fmt.Errorf("too many redirects (%d)", len(via))
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", parsedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,text/plain;q=0.8,*/*;q=0.5")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	// Read up to 5MB
	limitReader := io.LimitReader(resp.Body, 5*1024*1024)
	bodyBytes, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, fmt.Errorf("reading response body failed: %w", err)
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	bodyStr := string(bodyBytes)

	var title string
	var markdown string

	if strings.Contains(contentType, "json") || strings.Contains(contentType, "text/plain") {
		markdown = bodyStr
		title = parsedURL.Path
	} else {
		// HTML conversion
		if m := tagTitleRegex.FindStringSubmatch(bodyStr); len(m) > 1 {
			title = strings.TrimSpace(html.UnescapeString(stripHTML(m[1])))
		}
		markdown = htmlToCleanMarkdown(bodyStr)
	}

	truncated := false
	if len(markdown) > maxLen {
		markdown = markdown[:maxLen] + "\n\n... (content truncated at character limit)"
		truncated = true
	}

	return toolResult(map[string]any{
		"url":         resp.Request.URL.String(),
		"status_code": resp.StatusCode,
		"title":       title,
		"truncated":   truncated,
		"content":     markdown,
	}), nil
}

func htmlToCleanMarkdown(rawHTML string) string {
	// 1. Remove non-content blocks
	s := rawHTML
	for _, re := range ignoredTagRegexes {
		s = re.ReplaceAllString(s, " ")
	}

	// 2. Convert pre/code blocks
	s = tagCodeRegex.ReplaceAllStringFunc(s, func(match string) string {
		inner := tagCodeRegex.FindStringSubmatch(match)
		if len(inner) > 1 {
			return "\n```\n" + html.UnescapeString(stripHTML(inner[1])) + "\n```\n"
		}
		return match
	})
	s = tagInlineCodeRegex.ReplaceAllStringFunc(s, func(match string) string {
		inner := tagInlineCodeRegex.FindStringSubmatch(match)
		if len(inner) > 1 {
			return "`" + html.UnescapeString(stripHTML(inner[1])) + "`"
		}
		return match
	})

	// 3. Headings
	s = tagH1Regex.ReplaceAllString(s, "\n\n# $1\n\n")
	s = tagH2Regex.ReplaceAllString(s, "\n\n## $1\n\n")
	s = tagH3Regex.ReplaceAllString(s, "\n\n### $1\n\n")
	s = tagH4Regex.ReplaceAllString(s, "\n\n#### $1\n\n")

	// 4. Links: convert <a href="url">text</a> -> [text](url)
	s = tagAnchorRegex.ReplaceAllStringFunc(s, func(match string) string {
		m := tagAnchorRegex.FindStringSubmatch(match)
		if len(m) > 2 {
			linkHref := strings.TrimSpace(m[1])
			linkText := strings.TrimSpace(stripHTML(m[2]))
			if linkText != "" && !strings.HasPrefix(linkHref, "javascript:") {
				return fmt.Sprintf(" [%s](%s) ", linkText, linkHref)
			}
		}
		return " "
	})

	// 5. Structure & Breaks
	s = tagLiRegex.ReplaceAllString(s, "\n- $1")
	s = tagBrRegex.ReplaceAllString(s, "\n")
	s = tagPRegex.ReplaceAllString(s, "\n\n$1\n\n")

	// 6. Strip any residual HTML tags
	s = tagStripAnyRegex.ReplaceAllString(s, " ")

	// 7. Unescape HTML entities
	s = html.UnescapeString(s)

	// 8. Normalize spacing
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(multiSpaceRegex.ReplaceAllString(line, " "))
		if trimmed != "" {
			lines = append(lines, trimmed)
		} else if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
	}
	clean := strings.Join(lines, "\n")
	return multiNewlineRegex.ReplaceAllString(clean, "\n\n")
}

func stripHTML(s string) string {
	return strings.TrimSpace(tagStripAnyRegex.ReplaceAllString(s, " "))
}

func webSearchTool() ToolSpec {
	return ToolSpec{
		Name:        "web_search",
		Description: "Fast web search engine. Returns top search results with titles, links, and snippets without launching a heavy browser. Use this tool for quick answers, documentation discovery, and web research.",
		Cost:        "medium",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Keywords or question to search",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Number of results to return (default 5, max 15)",
				},
			},
			"required": []string{"query"},
		},
		Handler: handleWebSearch,
	}
}

type SearchResultItem struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

func handleWebSearch(ctx context.Context, args map[string]any) (any, error) {
	query := strings.TrimSpace(argString(args, "query"))
	if query == "" {
		return nil, fmt.Errorf("query parameter is required")
	}
	limit := argInt(args, "limit", 5)
	if limit <= 0 {
		limit = 5
	}
	if limit > 15 {
		limit = 15
	}

	results, err := searchDuckDuckGo(ctx, query, limit)
	if err != nil || len(results) == 0 {
		// Fallback to DuckDuckGo Lite endpoint
		liteResults, liteErr := searchDuckDuckGoLite(ctx, query, limit)
		if liteErr == nil && len(liteResults) > 0 {
			results = liteResults
		} else if err != nil {
			return nil, fmt.Errorf("search failed: %w", err)
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Search results for %q:\n\n", query))
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("%d. [%s](%s)\n   %s\n\n", i+1, r.Title, r.URL, r.Snippet))
	}

	return toolResult(map[string]any{
		"query":        query,
		"count":        len(results),
		"results":      results,
		"summary_text": strings.TrimSpace(sb.String()),
	}), nil
}

func searchDuckDuckGo(ctx context.Context, query string, limit int) ([]SearchResultItem, error) {
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, "POST", searchURL, strings.NewReader("q="+url.QueryEscape(query)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, err
	}
	bodyStr := string(bodyBytes)

	var results []SearchResultItem
	titleMatches := ddgTitleRegex.FindAllStringSubmatch(bodyStr, -1)
	snippetMatches := ddgResultRegex.FindAllStringSubmatch(bodyStr, -1)

	for i, tm := range titleMatches {
		if len(results) >= limit {
			break
		}
		if len(tm) > 2 {
			linkURL := tm[1]
			// DuckDuckGo redirects through /l/?kh=-1&uddg=...
			if strings.Contains(linkURL, "uddg=") {
				if parsed, err := url.Parse(linkURL); err == nil {
					if target := parsed.Query().Get("uddg"); target != "" {
						linkURL = target
					}
				}
			}
			titleText := html.UnescapeString(stripHTML(tm[2]))
			snippetText := ""
			if i < len(snippetMatches) && len(snippetMatches[i]) > 2 {
				snippetText = html.UnescapeString(stripHTML(snippetMatches[i][2]))
			}
			if titleText != "" && linkURL != "" {
				results = append(results, SearchResultItem{
					Title:   titleText,
					URL:     linkURL,
					Snippet: snippetText,
				})
			}
		}
	}
	return results, nil
}

func searchDuckDuckGoLite(ctx context.Context, query string, limit int) ([]SearchResultItem, error) {
	searchURL := "https://lite.duckduckgo.com/lite/"
	form := url.Values{}
	form.Set("q", query)

	req, err := http.NewRequestWithContext(ctx, "POST", searchURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, err
	}
	bodyStr := string(bodyBytes)

	links := ddgLiteLink.FindAllStringSubmatch(bodyStr, -1)
	snippets := ddgLiteRegex.FindAllStringSubmatch(bodyStr, -1)

	var results []SearchResultItem
	for i, l := range links {
		if len(results) >= limit {
			break
		}
		if len(l) > 2 {
			linkURL := l[1]
			if strings.Contains(linkURL, "uddg=") {
				if parsed, err := url.Parse(linkURL); err == nil {
					if target := parsed.Query().Get("uddg"); target != "" {
						linkURL = target
					}
				}
			}
			titleText := html.UnescapeString(stripHTML(l[2]))
			snippetText := ""
			if i < len(snippets) && len(snippets[i]) > 1 {
				snippetText = html.UnescapeString(stripHTML(snippets[i][1]))
			}
			if titleText != "" && linkURL != "" {
				results = append(results, SearchResultItem{
					Title:   titleText,
					URL:     linkURL,
					Snippet: snippetText,
				})
			}
		}
	}
	return results, nil
}
