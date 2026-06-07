package wa

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// URLRegex matches HTTP/HTTPS URLs in text.
var URLRegex = regexp.MustCompile(`https?://[^\s<>"]+`)

type LinkPreview struct {
	URL         string
	Title       string
	Description string
}

func FetchLinkPreview(ctx context.Context, text string) *LinkPreview {
	match := URLRegex.FindString(text)
	if match == "" {
		return nil
	}

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", match, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "WhatsApp/2")

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil
	}
	html := string(body)

	title := extractMeta(html, "og:title")
	if title == "" {
		title = extractTag(html, "title")
	}
	desc := extractMeta(html, "og:description")
	if desc == "" {
		desc = extractMeta(html, "description")
	}

	if title == "" && desc == "" {
		return nil
	}
	return &LinkPreview{URL: match, Title: title, Description: desc}
}

func extractMeta(html, property string) string {
	// Look for <meta property="X" content="Y"> or <meta name="X" content="Y">
	patterns := []string{
		`<meta[^>]*property="` + regexp.QuoteMeta(property) + `"[^>]*content="([^"]*)"`,
		`<meta[^>]*name="` + regexp.QuoteMeta(property) + `"[^>]*content="([^"]*)"`,
		`<meta[^>]*content="([^"]*)"[^>]*property="` + regexp.QuoteMeta(property) + `"`,
		`<meta[^>]*content="([^"]*)"[^>]*name="` + regexp.QuoteMeta(property) + `"`,
	}
	for _, p := range patterns {
		re := regexp.MustCompile(p)
		m := re.FindStringSubmatch(html)
		if len(m) > 1 {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

func extractTag(html, tag string) string {
	re := regexp.MustCompile(`<` + regexp.QuoteMeta(tag) + `[^>]*>([^<]*)<`)
	m := re.FindStringSubmatch(html)
	if len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}
