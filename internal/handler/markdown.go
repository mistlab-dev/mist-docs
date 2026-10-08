package handler

import (
	"bufio"
	"bytes"
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	ghtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// mdRefs resolves the local references of one Markdown file during import.
// A nil *mdRefs (single-file import) leaves remote links alone and turns
// local images, which cannot be found, into a visible note.
type mdRefs struct {
	// image returns the URL to use for a local image reference, or ok=false
	// when the image is not part of the import.
	image func(ref string) (url string, ok bool)
	// link returns the in-app address of another imported document, or
	// ok=false to keep the original link.
	link func(ref string) (url string, ok bool)
	// missing records local images that could not be imported.
	missing []string
}

func (r *mdRefs) resolveImage(ref string) (string, bool) {
	if isRemoteRef(ref) {
		return ref, true
	}
	if r != nil && r.image != nil {
		if u, ok := r.image(ref); ok {
			return u, true
		}
	}
	if r != nil {
		r.missing = append(r.missing, ref)
	}
	return "", false
}

func (r *mdRefs) resolveLink(ref string) string {
	if r == nil || r.link == nil || isRemoteRef(ref) || strings.HasPrefix(ref, "#") {
		return ref
	}
	if u, ok := r.link(ref); ok {
		return u
	}
	return ref
}

func isRemoteRef(ref string) bool {
	l := strings.ToLower(strings.TrimSpace(ref))
	return strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") ||
		strings.HasPrefix(l, "//") || strings.HasPrefix(l, "data:image/") ||
		strings.HasPrefix(l, "mailto:")
}

var (
	// Obsidian-style embeds: ![[pic.png]] or ![[pic.png|300]]
	wikiEmbedRe = regexp.MustCompile(`!\[\[([^\]|\n]+?)(?:\|[^\]\n]*)?\]\]`)
	imgTagRe    = regexp.MustCompile(`(?i)<img\b[^>]*>`)
	imgAttrRe   = regexp.MustCompile(`(?i)\b(src|alt)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
	brTagRe     = regexp.MustCompile(`(?i)^\s*<br\s*/?>\s*$`)
)

// splitFrontMatter removes a leading YAML front matter block (Hexo, Hugo,
// Jekyll, many note apps) and returns its title, if any.
func splitFrontMatter(md string) (body, title string) {
	md = strings.TrimPrefix(md, "\ufeff")
	if !strings.HasPrefix(md, "---\n") && !strings.HasPrefix(md, "---\r\n") {
		return md, ""
	}
	rest := md[strings.Index(md, "\n")+1:]
	end := -1
	for off := 0; off < len(rest); {
		nl := strings.IndexByte(rest[off:], '\n')
		line := rest[off:]
		if nl >= 0 {
			line = rest[off : off+nl]
		}
		if t := strings.TrimRight(line, "\r "); t == "---" || t == "..." {
			end = off
			break
		}
		if nl < 0 {
			break
		}
		off += nl + 1
	}
	if end < 0 {
		return md, ""
	}
	for _, line := range strings.Split(rest[:end], "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "title:") {
			title = strings.Trim(strings.TrimSpace(line[len("title:"):]), `"'`)
		}
	}
	body = rest[end:]
	if nl := strings.IndexByte(body, '\n'); nl >= 0 {
		body = body[nl+1:]
	} else {
		body = ""
	}
	return body, title
}

// renderMarkdown converts Markdown to the HTML the document editor reads:
// GitHub-style tables, task lists, strikethrough and fenced code; raw HTML is
// dropped except <img> and <br>. refs rewrites local images and links.
func renderMarkdown(md string, refs *mdRefs) (out string, title string) {
	md, title = splitFrontMatter(strings.ReplaceAll(md, "\r\n", "\n"))
	md = wikiEmbedRe.ReplaceAllStringFunc(md, func(m string) string {
		name := strings.TrimSpace(wikiEmbedRe.FindStringSubmatch(m)[1])
		return "![](<" + name + ">)"
	})

	gm := goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.CJK),
		goldmark.WithParserOptions(parser.WithASTTransformers(util.Prioritized(&mdRefTransformer{refs: refs}, 100))),
		goldmark.WithRendererOptions(renderer.WithNodeRenderers(util.Prioritized(&mdEditorRenderer{refs: refs}, 1))),
	)
	var buf bytes.Buffer
	if err := gm.Convert([]byte(md), &buf); err != nil {
		return textToHTML(md), title
	}
	return strings.TrimSpace(buf.String()), title
}

// markdownToHTML is the single-file converter used by /import.
func markdownToHTML(md string) string {
	out, _ := renderMarkdown(md, nil)
	return out
}

// mdRefTransformer rewrites link and image destinations before rendering.
type mdRefTransformer struct{ refs *mdRefs }

const missingImageAttr = "data-missing"

func (t *mdRefTransformer) Transform(doc *gast.Document, reader text.Reader, pc parser.Context) {
	_ = gast.Walk(doc, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		switch v := n.(type) {
		case *gast.Image:
			ref := string(v.Destination)
			if u, ok := t.refs.resolveImage(ref); ok {
				v.Destination = []byte(u)
			} else {
				v.SetAttributeString(missingImageAttr, []byte(ref))
			}
		case *gast.Link:
			v.Destination = []byte(t.refs.resolveLink(string(v.Destination)))
		}
		return gast.WalkContinue, nil
	})
}

// mdEditorRenderer adapts goldmark's output to the editor (TipTap):
// task lists use data-type="taskList"/"taskItem", missing local images
// become a short note instead of a broken picture, and the only raw HTML
// kept is <img> (common in exported notes) and <br>.
type mdEditorRenderer struct{ refs *mdRefs }

func (r *mdEditorRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(gast.KindList, r.renderList)
	reg.Register(gast.KindListItem, r.renderListItem)
	reg.Register(extast.KindTaskCheckBox, r.renderTaskCheckBox)
	reg.Register(gast.KindImage, r.renderImage)
	reg.Register(gast.KindRawHTML, r.renderRawHTML)
	reg.Register(gast.KindHTMLBlock, r.renderHTMLBlock)
}

func taskCheckBox(li gast.Node) *extast.TaskCheckBox {
	if li == nil {
		return nil
	}
	fc := li.FirstChild()
	if fc == nil {
		return nil
	}
	if cb, ok := fc.FirstChild().(*extast.TaskCheckBox); ok {
		return cb
	}
	return nil
}

func isTaskList(list gast.Node) bool {
	for c := list.FirstChild(); c != nil; c = c.NextSibling() {
		if taskCheckBox(c) == nil {
			return false
		}
	}
	return list.FirstChild() != nil
}

func (r *mdEditorRenderer) renderList(w util.BufWriter, src []byte, node gast.Node, entering bool) (gast.WalkStatus, error) {
	n := node.(*gast.List)
	tag := "ul"
	if n.IsOrdered() {
		tag = "ol"
	}
	if entering {
		switch {
		case !n.IsOrdered() && isTaskList(n):
			_, _ = w.WriteString(`<ul data-type="taskList">`)
		case n.IsOrdered() && n.Start != 1:
			_, _ = w.WriteString("<ol start=\"" + strconv.Itoa(n.Start) + "\">")
		default:
			_, _ = w.WriteString("<" + tag + ">")
		}
		_ = w.WriteByte('\n')
	} else {
		_, _ = w.WriteString("</" + tag + ">\n")
	}
	return gast.WalkContinue, nil
}

func (r *mdEditorRenderer) renderListItem(w util.BufWriter, src []byte, n gast.Node, entering bool) (gast.WalkStatus, error) {
	if !entering {
		_, _ = w.WriteString("</li>\n")
		return gast.WalkContinue, nil
	}
	if cb := taskCheckBox(n); cb != nil && isTaskList(n.Parent()) {
		checked := "false"
		if cb.IsChecked {
			checked = "true"
		}
		_, _ = w.WriteString(`<li data-type="taskItem" data-checked="` + checked + `">`)
	} else {
		_, _ = w.WriteString("<li>")
	}
	return gast.WalkContinue, nil
}

func (r *mdEditorRenderer) renderTaskCheckBox(w util.BufWriter, src []byte, node gast.Node, entering bool) (gast.WalkStatus, error) {
	// Rendered as the list item's data-checked attribute. A stray one
	// outside a task list keeps its meaning as text.
	if entering && !isTaskList(node.Parent().Parent().Parent()) {
		if node.(*extast.TaskCheckBox).IsChecked {
			_, _ = w.WriteString("[x] ")
		} else {
			_, _ = w.WriteString("[ ] ")
		}
	}
	return gast.WalkContinue, nil
}

func nodePlainText(n gast.Node, src []byte) string {
	var b strings.Builder
	_ = gast.Walk(n, func(c gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		switch t := c.(type) {
		case *gast.Text:
			b.Write(t.Segment.Value(src))
		case *gast.String:
			b.Write(t.Value)
		}
		return gast.WalkContinue, nil
	})
	return b.String()
}

func writeImg(w util.BufWriter, src, alt string) {
	_, _ = w.WriteString(`<img src="` + html.EscapeString(src) + `" alt="` + html.EscapeString(alt) + `">`)
}

func writeMissingImg(w util.BufWriter, ref string) {
	_, _ = w.WriteString(`<em>［图片没有导入：` + html.EscapeString(ref) + `］</em>`)
}

func (r *mdEditorRenderer) renderImage(w util.BufWriter, src []byte, node gast.Node, entering bool) (gast.WalkStatus, error) {
	if !entering {
		return gast.WalkContinue, nil
	}
	n := node.(*gast.Image)
	if v, ok := n.AttributeString(missingImageAttr); ok {
		writeMissingImg(w, string(v.([]byte)))
		return gast.WalkSkipChildren, nil
	}
	dest := string(n.Destination)
	if ghtml.IsDangerousURL([]byte(dest)) {
		return gast.WalkSkipChildren, nil
	}
	writeImg(w, dest, nodePlainText(n, src))
	return gast.WalkSkipChildren, nil
}

// rawImages keeps <img> tags (with src resolved like Markdown images) and
// <br> from a piece of raw HTML; everything else is dropped.
func (r *mdEditorRenderer) rawImages(w util.BufWriter, raw string) {
	if brTagRe.MatchString(raw) {
		_, _ = w.WriteString("<br>")
		return
	}
	for _, tag := range imgTagRe.FindAllString(raw, -1) {
		var srcAttr, alt string
		for _, m := range imgAttrRe.FindAllStringSubmatch(tag, -1) {
			val := html.UnescapeString(m[2] + m[3] + m[4])
			if strings.EqualFold(m[1], "src") {
				srcAttr = val
			} else {
				alt = val
			}
		}
		if srcAttr == "" {
			continue
		}
		u, ok := r.refs.resolveImage(srcAttr)
		if !ok {
			writeMissingImg(w, srcAttr)
			continue
		}
		if ghtml.IsDangerousURL([]byte(u)) {
			continue
		}
		writeImg(w, u, alt)
	}
}

func (r *mdEditorRenderer) renderRawHTML(w util.BufWriter, src []byte, node gast.Node, entering bool) (gast.WalkStatus, error) {
	if !entering {
		return gast.WalkSkipChildren, nil
	}
	n := node.(*gast.RawHTML)
	var b strings.Builder
	for i := 0; i < n.Segments.Len(); i++ {
		s := n.Segments.At(i)
		b.Write(s.Value(src))
	}
	r.rawImages(w, b.String())
	return gast.WalkSkipChildren, nil
}

func (r *mdEditorRenderer) renderHTMLBlock(w util.BufWriter, src []byte, node gast.Node, entering bool) (gast.WalkStatus, error) {
	if !entering {
		return gast.WalkContinue, nil
	}
	n := node.(*gast.HTMLBlock)
	var b strings.Builder
	for i := 0; i < n.Lines().Len(); i++ {
		l := n.Lines().At(i)
		b.Write(l.Value(src))
	}
	if n.HasClosure() {
		b.Write(n.ClosureLine.Value(src))
	}
	var inner bytes.Buffer
	bw := bufio.NewWriter(&inner)
	r.rawImages(bw, b.String())
	_ = bw.Flush()
	if inner.Len() > 0 {
		_, _ = w.WriteString("<p>")
		_, _ = w.Write(inner.Bytes())
		_, _ = w.WriteString("</p>\n")
	}
	return gast.WalkContinue, nil
}
