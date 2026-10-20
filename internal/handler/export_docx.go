package handler

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"

	golanghtml "golang.org/x/net/html"
)

// Theme fills for paragraph-level shading (OOXML hex, no leading '#').
const (
	codeBlockShade  = "F4F4F4" // light grey, approximates <pre> background
	blockquoteShade = "F0F4F8" // light blue-grey, approximates <blockquote> background
)

// OOXML constants and namespaces
const (
	docxContentTypeXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
</Types>`

	docxRootRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`

	docxStylesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:style w:type="paragraph" w:styleId="Normal" w:default="1">
    <w:name w:val="Normal"/>
    <w:rPr>
      <w:rFonts w:ascii="Calibri" w:hAnsi="Calibri" w:eastAsia="SimSun"/>
      <w:sz w:val="22"/>
      <w:szCs w:val="22"/>
    </w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading1">
    <w:name w:val="heading 1"/><w:basedOn w:val="Normal"/><w:uiPriority w:val="9"/>
    <w:pPr><w:outlineLvl w:val="0"/><w:spacing w:before="240" w:after="120"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="36"/><w:szCs w:val="36"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading2">
    <w:name w:val="heading 2"/><w:basedOn w:val="Normal"/><w:uiPriority w:val="9"/>
    <w:pPr><w:outlineLvl w:val="1"/><w:spacing w:before="200" w:after="100"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="30"/><w:szCs w:val="30"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading3">
    <w:name w:val="heading 3"/><w:basedOn w:val="Normal"/><w:uiPriority w:val="9"/>
    <w:pPr><w:outlineLvl w:val="2"/><w:spacing w:before="160" w:after="80"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="26"/><w:szCs w:val="26"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading4">
    <w:name w:val="heading 4"/><w:basedOn w:val="Normal"/><w:uiPriority w:val="9"/>
    <w:pPr><w:outlineLvl w:val="3"/><w:spacing w:before="120" w:after="60"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="24"/><w:szCs w:val="24"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading5">
    <w:name w:val="heading 5"/><w:basedOn w:val="Normal"/><w:uiPriority w:val="9"/>
    <w:pPr><w:outlineLvl w:val="4"/><w:spacing w:before="100" w:after="50"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="22"/><w:szCs w:val="22"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading6">
    <w:name w:val="heading 6"/><w:basedOn w:val="Normal"/><w:uiPriority w:val="9"/>
    <w:pPr><w:outlineLvl w:val="5"/><w:spacing w:before="100" w:after="50"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="20"/><w:szCs w:val="20"/></w:rPr>
  </w:style>
  <w:style w:type="character" w:styleId="Hyperlink">
    <w:name w:val="Hyperlink"/>
    <w:rPr>
      <w:color w:val="0563C1"/>
      <w:u w:val="single"/>
    </w:rPr>
  </w:style>
</w:styles>`
)

type docxRel struct {
	id     string
	target string
}

type docxConverter struct {
	body   strings.Builder
	rels   []docxRel
	relIdx int
}

func (dc *docxConverter) nextRelID() string {
	dc.relIdx++
	return fmt.Sprintf("rId%d", dc.relIdx+1) // rId1 is reserved for styles.xml
}

func docxXMLEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		case '\'':
			b.WriteString("&apos;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// safeHref reports whether href uses an allow-listed scheme. Only http(s)
// and mailto: links become clickable w:hyperlink relationships; everything
// else (javascript:, data:, file:, …) renders as plain text.
func safeHref(href string) bool {
	if href == "" {
		return false
	}
	lowerHref := strings.ToLower(href)
	return strings.HasPrefix(lowerHref, "http://") ||
		strings.HasPrefix(lowerHref, "https://") ||
		strings.HasPrefix(lowerHref, "mailto:")
}

type docxInlineState struct {
	bold      bool
	italic    bool
	underline bool
	strike    bool
	code      bool
	href      string
	relID     string
}

func (dc *docxConverter) openParagraph(style string, indentLevel int, prefix string) {
	dc.openParagraphEx(style, indentLevel*420, "", prefix)
}

// openParagraphEx opens a paragraph with an absolute left indent (twips) and
// an optional paragraph-level shading fill ("" disables). Shading
// approximates code-block / quote backgrounds; Word renders it as a
// full-width backdrop behind the paragraph.
func (dc *docxConverter) openParagraphEx(style string, indentTwips int, shadeFill string, prefix string) {
	dc.body.WriteString(`<w:p>`)
	hasPPr := style != "" || indentTwips > 0 || shadeFill != ""
	if hasPPr {
		dc.body.WriteString(`<w:pPr>`)
		if style != "" {
			dc.body.WriteString(`<w:pStyle w:val="` + docxXMLEscape(style) + `"/>`)
		}
		if indentTwips > 0 {
			dc.body.WriteString(fmt.Sprintf(`<w:ind w:left="%d"/>`, indentTwips))
		}
		if shadeFill != "" {
			dc.body.WriteString(`<w:shd w:val="clear" w:color="auto" w:fill="` + docxXMLEscape(shadeFill) + `"/>`)
		}
		dc.body.WriteString(`</w:pPr>`)
	}
	if prefix != "" {
		dc.writeRun(prefix, docxInlineState{})
	}
}

func (dc *docxConverter) closeParagraph() {
	dc.body.WriteString(`</w:p>`)
}

func (dc *docxConverter) writeRun(text string, st docxInlineState) {
	if text == "" {
		return
	}

	runXML := func() string {
		var r strings.Builder
		r.WriteString(`<w:r>`)
		hasRPr := st.bold || st.italic || st.underline || st.strike || st.code || st.href != ""
		if hasRPr {
			r.WriteString(`<w:rPr>`)
			if st.bold {
				r.WriteString(`<w:b/>`)
			}
			if st.italic {
				r.WriteString(`<w:i/>`)
			}
			if st.underline || st.href != "" {
				r.WriteString(`<w:u w:val="single"/>`)
			}
			if st.strike {
				r.WriteString(`<w:strike/>`)
			}
			if st.href != "" {
				r.WriteString(`<w:color w:val="0563C1"/>`)
			}
			if st.code {
				r.WriteString(`<w:rFonts w:ascii="Courier New" w:hAnsi="Courier New"/>`)
				r.WriteString(`<w:shd w:val="clear" w:color="auto" w:fill="F4F4F4"/>`)
			}
			r.WriteString(`</w:rPr>`)
		}
		r.WriteString(`<w:t xml:space="preserve">`)
		r.WriteString(docxXMLEscape(text))
		r.WriteString(`</w:t></w:r>`)
		return r.String()
	}

	if st.href != "" && st.relID != "" {
		dc.body.WriteString(`<w:hyperlink r:id="` + st.relID + `">`)
		dc.body.WriteString(runXML())
		dc.body.WriteString(`</w:hyperlink>`)
	} else {
		dc.body.WriteString(runXML())
	}
}

func (dc *docxConverter) processInline(n *golanghtml.Node, st docxInlineState) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case golanghtml.TextNode:
			dc.writeRun(c.Data, st)
		case golanghtml.ElementNode:
			next := st
			switch c.Data {
			case "strong", "b":
				next.bold = true
			case "em", "i":
				next.italic = true
			case "u":
				next.underline = true
			case "s", "strike", "del":
				next.strike = true
			case "code":
				next.code = true
			case "br":
				dc.body.WriteString(`<w:r><w:br/></w:r>`)
				continue
			case "a":
				href := ""
				for _, attr := range c.Attr {
					if attr.Key == "href" {
						href = strings.TrimSpace(attr.Val)
						break
					}
				}
				if safeHref(href) {
					next.href = href
					next.relID = dc.nextRelID()
					dc.rels = append(dc.rels, docxRel{id: next.relID, target: href})
				}
			}
			dc.processInline(c, next)
		}
	}
}

func (dc *docxConverter) processList(n *golanghtml.Node, isOrdered bool, level int) {
	idx := 1
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != golanghtml.ElementNode || c.Data != "li" {
			continue
		}

		prefix := "• "
		if isOrdered {
			prefix = fmt.Sprintf("%d. ", idx)
			idx++
		}

		// Separate direct children vs nested list children
		var nestedLists []*golanghtml.Node
		var inlineChildren []*golanghtml.Node

		for child := c.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == golanghtml.ElementNode && (child.Data == "ul" || child.Data == "ol") {
				nestedLists = append(nestedLists, child)
			} else {
				inlineChildren = append(inlineChildren, child)
			}
		}

		// Render the li item line
		dc.openParagraph("", level, prefix)
		for _, child := range inlineChildren {
			if child.Type == golanghtml.TextNode {
				dc.writeRun(child.Data, docxInlineState{})
			} else {
				next := docxInlineState{}
				switch child.Data {
				case "strong", "b":
					next.bold = true
				case "em", "i":
					next.italic = true
				case "u":
					next.underline = true
				case "s", "strike", "del":
					next.strike = true
				case "code":
					next.code = true
				case "a":
					for _, attr := range child.Attr {
						if attr.Key == "href" {
							h := strings.TrimSpace(attr.Val)
							if safeHref(h) {
								next.href = h
								next.relID = dc.nextRelID()
								dc.rels = append(dc.rels, docxRel{id: next.relID, target: next.href})
							}
							break
						}
					}
				}
				dc.processInline(child, next)
			}
		}
		dc.closeParagraph()

		// Render nested lists
		for _, nl := range nestedLists {
			dc.processList(nl, nl.Data == "ol", level+1)
		}
	}
}

func (dc *docxConverter) processTable(n *golanghtml.Node) {
	dc.body.WriteString(`<w:tbl><w:tblPr><w:tblStyle w:val="TableGrid"/><w:tblW w:w="5000" w:type="pct"/>`)
	dc.body.WriteString(`<w:tblBorders>`)
	dc.body.WriteString(`<w:top w:val="single" w:sz="4" w:space="0" w:color="D3D3D3"/>`)
	dc.body.WriteString(`<w:left w:val="single" w:sz="4" w:space="0" w:color="D3D3D3"/>`)
	dc.body.WriteString(`<w:bottom w:val="single" w:sz="4" w:space="0" w:color="D3D3D3"/>`)
	dc.body.WriteString(`<w:right w:val="single" w:sz="4" w:space="0" w:color="D3D3D3"/>`)
	dc.body.WriteString(`<w:insideH w:val="single" w:sz="4" w:space="0" w:color="E0E0E0"/>`)
	dc.body.WriteString(`<w:insideV w:val="single" w:sz="4" w:space="0" w:color="E0E0E0"/>`)
	dc.body.WriteString(`</w:tblBorders></w:tblPr>`)

	var walkRows func(*golanghtml.Node, bool)
	walkRows = func(node *golanghtml.Node, inHeader bool) {
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != golanghtml.ElementNode {
				continue
			}
			switch c.Data {
			case "thead":
				walkRows(c, true)
			case "tbody", "tfoot":
				walkRows(c, inHeader)
			case "tr":
				dc.body.WriteString(`<w:tr>`)
				for cell := c.FirstChild; cell != nil; cell = cell.NextSibling {
					if cell.Type != golanghtml.ElementNode || (cell.Data != "td" && cell.Data != "th") {
						continue
					}
					isTh := inHeader || cell.Data == "th"
					dc.body.WriteString(`<w:tc><w:tcPr><w:tcW w:w="0" w:type="auto"/>`)
					if isTh {
						dc.body.WriteString(`<w:shd w:val="clear" w:color="auto" w:fill="F2F2F2"/>`)
					}
					dc.body.WriteString(`</w:tcPr>`)

					dc.openParagraph("", 0, "")
					dc.processInline(cell, docxInlineState{bold: isTh})
					dc.closeParagraph()
					dc.body.WriteString(`</w:tc>`)
				}
				dc.body.WriteString(`</w:tr>`)
			}
		}
	}

	walkRows(n, false)
	dc.body.WriteString(`</w:tbl>`)
	// Word requires an empty paragraph after a table to permit cursor placement
	dc.openParagraph("", 0, "")
	dc.closeParagraph()
}

func (dc *docxConverter) processBlocks(n *golanghtml.Node) {
	var inlineBuf []*golanghtml.Node

	flushInline := func() {
		if len(inlineBuf) == 0 {
			return
		}
		hasContent := false
		for _, node := range inlineBuf {
			if node.Type == golanghtml.TextNode && strings.TrimSpace(node.Data) != "" {
				hasContent = true
				break
			}
			if node.Type == golanghtml.ElementNode {
				hasContent = true
				break
			}
		}
		if hasContent {
			dc.openParagraph("", 0, "")
			for _, node := range inlineBuf {
				if node.Type == golanghtml.TextNode {
					dc.writeRun(node.Data, docxInlineState{})
				} else if node.Type == golanghtml.ElementNode {
					dc.processInline(node, docxInlineState{})
				}
			}
			dc.closeParagraph()
		}
		inlineBuf = nil
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if isBlockNode(c) {
			flushInline()
			dc.processOneBlock(c)
		} else {
			inlineBuf = append(inlineBuf, c)
		}
	}
	flushInline()
}

func (dc *docxConverter) processOneBlock(c *golanghtml.Node) {
	switch c.Data {
	case "h1":
		dc.openParagraph("Heading1", 0, "")
		dc.processInline(c, docxInlineState{})
		dc.closeParagraph()
	case "h2":
		dc.openParagraph("Heading2", 0, "")
		dc.processInline(c, docxInlineState{})
		dc.closeParagraph()
	case "h3":
		dc.openParagraph("Heading3", 0, "")
		dc.processInline(c, docxInlineState{})
		dc.closeParagraph()
	case "h4":
		dc.openParagraph("Heading4", 0, "")
		dc.processInline(c, docxInlineState{})
		dc.closeParagraph()
	case "h5":
		dc.openParagraph("Heading5", 0, "")
		dc.processInline(c, docxInlineState{})
		dc.closeParagraph()
	case "h6":
		dc.openParagraph("Heading6", 0, "")
		dc.processInline(c, docxInlineState{})
		dc.closeParagraph()
	case "p":
		dc.openParagraph("", 0, "")
		dc.processInline(c, docxInlineState{})
		dc.closeParagraph()
	case "pre":
		// Code block: monospace Courier New, newlines as w:br, paragraph-level
		// light shading approximates the code-block background.
		dc.openParagraphEx("", 0, codeBlockShade, "")
		dc.processPre(c)
		dc.closeParagraph()
	case "blockquote":
		// Quote: indent approximates the blockquote bar, light shading
		// approximates the quote background. Nested blocks are re-processed
		// with an extra indent level.
		dc.processBlockquote(c, 1)
	case "ul":
		dc.processList(c, false, 1)
	case "ol":
		dc.processList(c, true, 1)
	case "table":
		dc.processTable(c)
	default:
		// Wrapper / unknown block element (<div>, <section>, <body>, …):
		// recursively process child blocks, flushing intermediate inline runs.
		dc.processBlocks(c)
	}
}

// isBlockNode reports whether n is an HTML block-level element.
func isBlockNode(n *golanghtml.Node) bool {
	if n.Type != golanghtml.ElementNode {
		return false
	}
	return isBlockElement(n.Data)
}

// isBlockElement reports whether tag is a block-level HTML element. These are
// exclusively owned by the block loop (processBlocks / processBlockquote /
// processList / processTable) and must never be consumed as inline content.
func isBlockElement(tag string) bool {
	switch tag {
	case "p", "h1", "h2", "h3", "h4", "h5", "h6",
		"ul", "ol", "table", "pre", "div", "section", "blockquote",
		"article", "header", "footer", "aside", "nav", "main", "form",
		"dl", "figure", "hr":
		return true
	}
	return false
}

// processBlockquote renders a <blockquote> at the given indent level (each
// level = 720 twips ≈ 1.27 cm / 0.5 in). A light blue-grey shading approximates
// the quote background. Direct inline content becomes an indented quote
// paragraph; nested blocks (p, ul, nested blockquote, pre …) recurse with an
// extra indent level.
func (dc *docxConverter) processBlockquote(n *golanghtml.Node, level int) {
	indent := level * 720
	var inlineBuf []*golanghtml.Node

	flushInline := func() {
		if len(inlineBuf) == 0 {
			return
		}
		hasContent := false
		for _, node := range inlineBuf {
			if node.Type == golanghtml.TextNode && strings.TrimSpace(node.Data) != "" {
				hasContent = true
				break
			}
			if node.Type == golanghtml.ElementNode {
				hasContent = true
				break
			}
		}
		if hasContent {
			dc.openParagraphEx("", indent, blockquoteShade, "")
			for _, node := range inlineBuf {
				if node.Type == golanghtml.TextNode {
					dc.writeRun(node.Data, docxInlineState{})
				} else if node.Type == golanghtml.ElementNode {
					dc.processInline(node, docxInlineState{})
				}
			}
			dc.closeParagraph()
		}
		inlineBuf = nil
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if isBlockNode(c) {
			flushInline()
			switch c.Data {
			case "p":
				dc.openParagraphEx("", indent, blockquoteShade, "")
				dc.processInline(c, docxInlineState{})
				dc.closeParagraph()
			case "blockquote":
				dc.processBlockquote(c, level+1)
			case "pre":
				// Code inside a quote: monospace + code shading, quoted indent.
				dc.openParagraphEx("", indent, codeBlockShade, "")
				dc.processPre(c)
				dc.closeParagraph()
			case "h1", "h2", "h3", "h4", "h5", "h6":
				style := "Heading" + strings.ToUpper(c.Data[1:2]) + c.Data[2:]
				dc.openParagraphEx(style, indent, blockquoteShade, "")
				dc.processInline(c, docxInlineState{})
				dc.closeParagraph()
			case "ul":
				dc.processList(c, false, level+1)
			case "ol":
				dc.processList(c, true, level+1)
			case "table":
				dc.processTable(c)
			default:
				dc.processBlocks(c)
			}
		} else {
			inlineBuf = append(inlineBuf, c)
		}
	}
	flushInline()
}

func (dc *docxConverter) processPre(n *golanghtml.Node) {
	var extractText func(*golanghtml.Node) string
	extractText = func(node *golanghtml.Node) string {
		var b strings.Builder
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == golanghtml.TextNode {
				b.WriteString(c.Data)
			} else {
				b.WriteString(extractText(c))
			}
		}
		return b.String()
	}

	raw := extractText(n)
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		if i > 0 {
			dc.body.WriteString(`<w:r><w:br/></w:r>`)
		}
		dc.writeRun(line, docxInlineState{code: true})
	}
}

// htmlToDocx converts HTML string to OOXML (.docx) zip bytes.
func htmlToDocx(htmlContent string) ([]byte, error) {
	root, err := golanghtml.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}

	// Locate <body> if present, else root
	var bodyNode *golanghtml.Node
	var findBody func(*golanghtml.Node)
	findBody = func(n *golanghtml.Node) {
		if n.Type == golanghtml.ElementNode && n.Data == "body" {
			bodyNode = n
			return
		}
		for c := n.FirstChild; c != nil && bodyNode == nil; c = c.NextSibling {
			findBody(c)
		}
	}
	findBody(root)
	if bodyNode == nil {
		bodyNode = root
	}

	dc := &docxConverter{}
	dc.processBlocks(bodyNode)

	// Ensure document has at least one paragraph so Word considers it valid
	if dc.body.Len() == 0 {
		dc.openParagraph("", 0, "")
		dc.closeParagraph()
	}

	// Build document.xml
	documentXML := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
            xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
    %s
    <w:sectPr>
      <w:pgSz w:w="11906" w:h="16838"/>
      <w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440"/>
    </w:sectPr>
  </w:body>
</w:document>`, dc.body.String())

	// Build word/_rels/document.xml.rels
	var docRels strings.Builder
	docRels.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`)
	for _, rel := range dc.rels {
		docRels.WriteString(fmt.Sprintf(`
  <Relationship Id="%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="%s" TargetMode="External"/>`,
			docxXMLEscape(rel.id), docxXMLEscape(rel.target)))
	}
	docRels.WriteString(`
</Relationships>`)

	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)

	// Deterministic order: [Content_Types].xml MUST be first
	type partEntry struct {
		name    string
		content string
	}
	parts := []partEntry{
		{"[Content_Types].xml", docxContentTypeXML},
		{"_rels/.rels", docxRootRelsXML},
		{"word/styles.xml", docxStylesXML},
		{"word/document.xml", documentXML},
		{"word/_rels/document.xml.rels", docRels.String()},
	}

	for _, p := range parts {
		w, err := zw.Create(p.name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(p.content)); err != nil {
			return nil, err
		}
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}

	return zipBuf.Bytes(), nil
}
