package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"golang.org/x/net/html"
)

var (
	docIDFromURLPattern = regexp.MustCompile(`/document/d/([a-zA-Z0-9_-]+)`)
	commentIDPattern    = regexp.MustCompile(`^cmnt\d+$`)
	commentRefIDPattern = regexp.MustCompile(`^cmnt_ref\d+$`)
	slugPattern         = regexp.MustCompile(`[^a-z0-9]+`)
	whitespaceOnly      = regexp.MustCompile(`^\s*$`)
)

type authStatus struct {
	AuthMethod string   `json:"auth_method"`
	TokenValid bool     `json:"token_valid"`
	Scopes     []string `json:"scopes"`
	User       string   `json:"user"`
}

type fileMetadata struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	MimeType     string `json:"mimeType"`
	ModifiedTime string `json:"modifiedTime"`
	WebViewLink  string `json:"webViewLink"`
}

type commentsPage struct {
	Comments      []driveComment `json:"comments"`
	NextPageToken string         `json:"nextPageToken"`
}

type driveComment struct {
	ID                string              `json:"id"`
	Content           string              `json:"content"`
	CreatedTime       string              `json:"createdTime"`
	ModifiedTime      string              `json:"modifiedTime"`
	Deleted           bool                `json:"deleted"`
	Resolved          bool                `json:"resolved"`
	Anchor            string              `json:"anchor"`
	QuotedFileContent quotedFileContent   `json:"quotedFileContent"`
	Author            driveAuthor         `json:"author"`
	Replies           []driveCommentReply `json:"replies"`
}

type quotedFileContent struct {
	Value string `json:"value"`
}

type driveAuthor struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"emailAddress"`
}

type driveCommentReply struct {
	ID           string      `json:"id"`
	Content      string      `json:"content"`
	CreatedTime  string      `json:"createdTime"`
	ModifiedTime string      `json:"modifiedTime"`
	Deleted      bool        `json:"deleted"`
	Action       string      `json:"action"`
	Author       driveAuthor `json:"author"`
}

func main() {
	docInput := flag.String("doc", "", "Google Doc URL or raw document ID")
	outputPath := flag.String("output", "", "Output markdown path. Defaults to ./<doc-name>.comments.md")
	flag.Parse()

	if strings.TrimSpace(*docInput) == "" {
		exitf("missing required --doc value")
	}

	docID, err := parseDocID(*docInput)
	if err != nil {
		exitf("%v", err)
	}

	if err := ensureGWSAuth(); err != nil {
		exitf("%v", err)
	}

	meta, err := getFileMetadata(docID)
	if err != nil {
		exitf("fetch file metadata: %v", err)
	}

	if meta.MimeType != "application/vnd.google-apps.document" {
		exitf("file %s is not a Google Doc (mimeType=%s)", docID, meta.MimeType)
	}

	htmlPath, err := exportDocHTML(docID)
	if err != nil {
		exitf("export doc HTML: %v", err)
	}
	defer os.Remove(htmlPath)

	cleanHTMLPath, err := cleanExportedHTML(htmlPath)
	if err != nil {
		exitf("clean exported HTML: %v", err)
	}
	defer os.Remove(cleanHTMLPath)

	bodyMarkdown, err := convertHTMLToMarkdown(cleanHTMLPath)
	if err != nil {
		exitf("convert HTML to Markdown: %v", err)
	}
	bodyMarkdown = normalizeMarkdown(bodyMarkdown)

	comments, err := listComments(docID)
	if err != nil {
		exitf("fetch comments: %v", err)
	}

	finalMarkdown := buildMarkdown(meta, *docInput, bodyMarkdown, comments)

	targetPath := strings.TrimSpace(*outputPath)
	if targetPath == "" {
		targetPath = filepath.Join(".", defaultOutputName(meta.Name))
	}

	if targetPath == "-" {
		if _, err := os.Stdout.Write([]byte(finalMarkdown)); err != nil {
			exitf("write stdout: %v", err)
		}
		return
	}

	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		exitf("resolve output path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		exitf("create output directory: %v", err)
	}
	if err := os.WriteFile(absPath, []byte(finalMarkdown), 0o644); err != nil {
		exitf("write output: %v", err)
	}

	fmt.Printf("Saved Markdown with comments to %s\n", absPath)
}

func parseDocID(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", errors.New("empty doc input")
	}
	if matches := docIDFromURLPattern.FindStringSubmatch(trimmed); len(matches) == 2 {
		return matches[1], nil
	}
	if strings.Contains(trimmed, "/") || strings.Contains(trimmed, "?") || strings.Contains(trimmed, "#") {
		return "", fmt.Errorf("could not parse Google Doc ID from %q", trimmed)
	}
	return trimmed, nil
}

func ensureGWSAuth() error {
	statusOutput, err := runCommand("gws", "auth", "status")
	if err != nil {
		return fmt.Errorf("gws auth status failed: %w", err)
	}

	var status authStatus
	if err := unmarshalFirstJSONObject(statusOutput, &status); err != nil {
		return fmt.Errorf("parse gws auth status: %w", err)
	}

	if status.AuthMethod == "" || status.AuthMethod == "none" || !status.TokenValid {
		return errors.New("gws is not authenticated for this machine. Run `gws auth setup` once, then `gws auth login`, and retry")
	}

	if !hasAnyScope(status.Scopes, "https://www.googleapis.com/auth/drive", "https://www.googleapis.com/auth/drive.readonly", "https://www.googleapis.com/auth/drive.file") {
		return errors.New("gws token is missing Drive scopes required for Google Doc export/comments. Re-run `gws auth login` with Drive access and retry")
	}

	return nil
}

func hasAnyScope(scopes []string, wanted ...string) bool {
	scopeSet := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		scopeSet[scope] = struct{}{}
	}
	for _, want := range wanted {
		if _, ok := scopeSet[want]; ok {
			return true
		}
	}
	return false
}

func getFileMetadata(docID string) (fileMetadata, error) {
	params := map[string]any{
		"fileId": docID,
		"fields": "id,name,mimeType,modifiedTime,webViewLink",
	}
	output, err := runGWSJSON("drive", "files", "get", params)
	if err != nil {
		return fileMetadata{}, err
	}
	var meta fileMetadata
	if err := json.Unmarshal(output, &meta); err != nil {
		return fileMetadata{}, err
	}
	return meta, nil
}

func exportDocHTML(docID string) (string, error) {
	tempFile, err := os.CreateTemp("", "gws-doc-export-*.html")
	if err != nil {
		return "", err
	}
	tempPath := tempFile.Name()
	tempFile.Close()

	params := map[string]any{
		"fileId":   docID,
		"mimeType": "text/html",
	}
	rawParams, err := json.Marshal(params)
	if err != nil {
		return "", err
	}

	_, err = runCommand("gws", "drive", "files", "export", "--params", string(rawParams), "--output", tempPath)
	if err != nil {
		os.Remove(tempPath)
		return "", err
	}
	return tempPath, nil
}

func cleanExportedHTML(path string) (string, error) {
	rawHTML, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	doc, err := html.Parse(bytes.NewReader(rawHTML))
	if err != nil {
		return "", err
	}

	body := findElement(doc, "body")
	if body == nil {
		return "", errors.New("exported HTML has no <body>")
	}

	walkPostOrder(body, func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}

		switch n.Data {
		case "style", "script":
			removeNode(n)
			return
		case "img":
			if strings.HasPrefix(attr(n, "src"), "data:") {
				replaceNodeWithText(n, "Embedded image omitted from Google export.")
				return
			}
		case "a":
			id := attr(n, "id")
			href := attr(n, "href")
			if commentIDPattern.MatchString(id) {
				removeCommentBlock(n)
				return
			}
			if commentRefIDPattern.MatchString(id) || commentIDPattern.MatchString(strings.TrimPrefix(href, "#")) {
				removeCommentRef(n)
				return
			}
		case "span":
			unwrapNode(n)
			return
		}

		stripAttrs(n)
	})

	removeEmptyNodes(body)

	tempFile, err := os.CreateTemp("", "gws-doc-clean-*.html")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	if err := html.Render(tempFile, doc); err != nil {
		return "", err
	}

	return tempFile.Name(), nil
}

func convertHTMLToMarkdown(path string) (string, error) {
	output, err := runCommand("pandoc", "-f", "html", "-t", "gfm", "--wrap=none", path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func listComments(docID string) ([]driveComment, error) {
	comments := make([]driveComment, 0, 16)
	pageToken := ""

	for {
		params := map[string]any{
			"fileId":   docID,
			"fields":   "comments,nextPageToken",
			"pageSize": 100,
		}
		if pageToken != "" {
			params["pageToken"] = pageToken
		}

		output, err := runGWSJSON("drive", "comments", "list", params)
		if err != nil {
			return nil, err
		}

		var page commentsPage
		if err := json.Unmarshal(output, &page); err != nil {
			return nil, err
		}

		comments = append(comments, page.Comments...)
		if page.NextPageToken == "" {
			return comments, nil
		}
		pageToken = page.NextPageToken
	}
}

func buildMarkdown(meta fileMetadata, input string, bodyMarkdown string, comments []driveComment) string {
	var b strings.Builder

	source := meta.WebViewLink
	if strings.TrimSpace(source) == "" {
		source = strings.TrimSpace(input)
	}

	fmt.Fprintf(&b, "Source: %s\n", source)
	fmt.Fprintf(&b, "Document ID: `%s`\n", meta.ID)
	if modified := formatTimestamp(meta.ModifiedTime); modified != "" {
		fmt.Fprintf(&b, "Last modified: %s\n", modified)
	}
	b.WriteString("\n")
	b.WriteString(strings.TrimSpace(bodyMarkdown))
	b.WriteString("\n\n## Comments\n\n")

	if len(comments) == 0 {
		b.WriteString("No comments.\n")
		return b.String()
	}

	for i, comment := range comments {
		fmt.Fprintf(&b, "### Comment %d\n\n", i+1)
		fmt.Fprintf(&b, "Author: %s\n", displayAuthor(comment.Author))
		if created := formatTimestamp(comment.CreatedTime); created != "" {
			fmt.Fprintf(&b, "Created: %s\n", created)
		}
		if modified := formatTimestamp(comment.ModifiedTime); modified != "" {
			fmt.Fprintf(&b, "Modified: %s\n", modified)
		}
		fmt.Fprintf(&b, "Status: %s\n", commentStatus(comment))
		fmt.Fprintf(&b, "Comment ID: `%s`\n", comment.ID)
		if comment.Anchor != "" && strings.TrimSpace(comment.QuotedFileContent.Value) == "" {
			fmt.Fprintf(&b, "Anchor: `%s`\n", comment.Anchor)
		}
		b.WriteString("\n")

		if quoted := strings.TrimSpace(comment.QuotedFileContent.Value); quoted != "" {
			b.WriteString("Quoted text:\n")
			b.WriteString(asBlockquote(quoted))
			b.WriteString("\n")
		}

		if body := strings.TrimSpace(comment.Content); body != "" {
			b.WriteString(body)
			b.WriteString("\n\n")
		} else if comment.Deleted {
			b.WriteString("_Deleted comment._\n\n")
		}

		if len(comment.Replies) > 0 {
			b.WriteString("Replies:\n")
			for _, reply := range comment.Replies {
				b.WriteString("- ")
				b.WriteString(displayAuthor(reply.Author))
				if created := formatTimestamp(reply.CreatedTime); created != "" {
					b.WriteString(" - ")
					b.WriteString(created)
				}
				if reply.Action != "" {
					b.WriteString(" - action: ")
					b.WriteString(reply.Action)
				}
				b.WriteString(": ")
				replyText := strings.TrimSpace(reply.Content)
				if replyText == "" && reply.Deleted {
					replyText = "[deleted]"
				}
				b.WriteString(replyText)
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
	}

	return strings.TrimSpace(b.String()) + "\n"
}

func commentStatus(comment driveComment) string {
	switch {
	case comment.Deleted:
		return "deleted"
	case comment.Resolved:
		return "resolved"
	default:
		return "open"
	}
}

func asBlockquote(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for i, line := range lines {
		lines[i] = "> " + strings.TrimSpace(line)
	}
	return strings.Join(lines, "\n") + "\n"
}

func defaultOutputName(title string) string {
	slug := strings.ToLower(strings.TrimSpace(title))
	slug = slugPattern.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "google-doc"
	}
	return slug + ".comments.md"
}

func normalizeMarkdown(input string) string {
	input = strings.ReplaceAll(input, "\u00a0", " ")

	var b strings.Builder
	for _, r := range input {
		if unicode.Is(unicode.Co, r) {
			continue
		}
		b.WriteRune(r)
	}

	lines := strings.Split(b.String(), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func displayAuthor(author driveAuthor) string {
	if strings.TrimSpace(author.DisplayName) != "" {
		return author.DisplayName
	}
	if strings.TrimSpace(author.Email) != "" {
		return author.Email
	}
	return "Unknown author"
}

func formatTimestamp(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	return t.UTC().Format("2006-01-02 15:04 MST")
}

func runGWSJSON(service, resource, method string, params map[string]any) ([]byte, error) {
	rawParams, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	output, err := runCommand("gws", service, resource, method, "--params", string(rawParams))
	if err != nil {
		return nil, err
	}
	return extractFirstJSONObject(output)
}

func runCommand(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func unmarshalFirstJSONObject(input []byte, dest any) error {
	data, err := extractFirstJSONObject(input)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func extractFirstJSONObject(input []byte) ([]byte, error) {
	for i, b := range input {
		if b == '{' || b == '[' {
			return input[i:], nil
		}
	}
	return nil, fmt.Errorf("no JSON object found in command output: %s", strings.TrimSpace(string(input)))
}

func walkPostOrder(n *html.Node, fn func(*html.Node)) {
	for child := n.FirstChild; child != nil; {
		next := child.NextSibling
		walkPostOrder(child, fn)
		child = next
	}
	fn(n)
}

func findElement(n *html.Node, tag string) *html.Node {
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if found := findElement(child, tag); found != nil {
			return found
		}
	}
	return nil
}

func removeCommentBlock(anchor *html.Node) {
	target := nearestAncestor(anchor, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "div"
	})
	if target == nil {
		target = nearestAncestor(anchor, func(n *html.Node) bool {
			return n.Type == html.ElementNode && (n.Data == "p" || n.Data == "li")
		})
	}
	if target == nil {
		target = anchor
	}
	removeNode(target)
}

func removeCommentRef(anchor *html.Node) {
	target := nearestAncestor(anchor, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "sup"
	})
	if target == nil {
		target = anchor
	}
	removeNode(target)
}

func nearestAncestor(n *html.Node, match func(*html.Node) bool) *html.Node {
	for current := n; current != nil; current = current.Parent {
		if match(current) {
			return current
		}
	}
	return nil
}

func removeEmptyNodes(root *html.Node) {
	walkPostOrder(root, func(n *html.Node) {
		if n == root || n.Type != html.ElementNode {
			return
		}
		switch n.Data {
		case "p", "div", "sup", "span":
			if !nodeHasMeaningfulContent(n) {
				removeNode(n)
			}
		}
	})
}

func nodeHasMeaningfulContent(n *html.Node) bool {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		switch child.Type {
		case html.TextNode:
			if !whitespaceOnly.MatchString(child.Data) {
				return true
			}
		case html.ElementNode:
			if child.Data == "img" || child.Data == "a" || child.Data == "table" || child.Data == "ul" || child.Data == "ol" || strings.HasPrefix(child.Data, "h") {
				return true
			}
			if nodeHasMeaningfulContent(child) {
				return true
			}
		}
	}
	return false
}

func replaceNodeWithText(n *html.Node, text string) {
	if n.Parent == nil {
		return
	}
	textNode := &html.Node{Type: html.TextNode, Data: text}
	n.Parent.InsertBefore(textNode, n)
	n.Parent.RemoveChild(n)
}

func unwrapNode(n *html.Node) {
	if n.Parent == nil {
		return
	}
	for child := n.FirstChild; child != nil; {
		next := child.NextSibling
		n.RemoveChild(child)
		n.Parent.InsertBefore(child, n)
		child = next
	}
	n.Parent.RemoveChild(n)
}

func removeNode(n *html.Node) {
	if n != nil && n.Parent != nil {
		n.Parent.RemoveChild(n)
	}
}

func stripAttrs(n *html.Node) {
	if n.Type != html.ElementNode {
		return
	}
	keep := make([]html.Attribute, 0, len(n.Attr))
	for _, attr := range n.Attr {
		switch attr.Key {
		case "href", "src", "alt", "title", "colspan", "rowspan":
			keep = append(keep, attr)
		}
	}
	n.Attr = keep
}

func attr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
