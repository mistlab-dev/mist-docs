package handler

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strconv"
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
	id       string
	target   string
	external bool // true -> TargetMode="External" (hyperlinks); false -> in-package target
}

type docxConverter struct {
	body       strings.Builder
	rels       []docxRel
	images     []docxImage
	drawingIdx int
	relIdx     int
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
			case "img":
				dc.processImage(c, next)
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
					dc.rels = append(dc.rels, docxRel{id: next.relID, target: href, external: true})
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
				case "img":
					dc.processImage(child, next)
					continue
				case "a":
					for _, attr := range child.Attr {
						if attr.Key == "href" {
							h := strings.TrimSpace(attr.Val)
							if safeHref(h) {
								next.href = h
								next.relID = dc.nextRelID()
								dc.rels = append(dc.rels, docxRel{id: next.relID, target: next.href, external: true})
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

// docxTableWidthTwips is the text width of an A4 page with default margins,
// split evenly across tblGrid columns (tblW stays 100% so Word reflows).
const docxTableWidthTwips = 9000

// docxMaxTableSpan clamps colspan/rowspan values. Document HTML is user
// content, so an unbounded colspan="50000" would allocate tens of thousands
// of grid columns and blow up the exported XML; 64 matches the practical
// upper limit of real-world tables.
const docxMaxTableSpan = 64

// docxTableCell is one td/th node together with its parsed colspan/rowspan
// (both clamped to >= 1 by docxParseSpan).
type docxTableCell struct {
	node    *golanghtml.Node
	colspan int
	rowspan int
}

// docxSlot is one grid column of one row. kind selects how it is emitted:
//
//	"cell"  — a real cell starts here (cell != nil); w:gridSpan covers its
//	          columns, w:vMerge w:val="restart" starts a rowspan
//	"cont"  — column covered by a rowspan started above → <w:vMerge/> cell
//	"span"  — column covered by a colspan of the same row's cell → no w:tc
//	          (w:gridSpan already covers it)
//	"empty" — ragged-row gap → empty <w:tc>
type docxSlot struct {
	kind string
	cell *docxTableCell
}

const (
	docxSlotCell  = "cell"
	docxSlotCont  = "cont"
	docxSlotSpan  = "span"
	docxSlotEmpty = "empty"
)

// docxTableRow is one tr with its grid fully resolved: slots holds exactly
// one entry per grid column this row spans, so every row's slot count equals
// its span sum — the conservation invariant Word's CT_Tbl relies on.
type docxTableRow struct {
	slots  []docxSlot // ordered by grid column
	isHead bool       // tr inside thead, or contains at least one th
}

// docxTableLayout is the pre-scan result for one <table>. rowspan cannot be
// resolved in a single pass (a later row must know which columns earlier rows
// still occupy), so processTable scans the whole table first and then emits.
type docxTableLayout struct {
	rows []*docxTableRow
	cols int // real grid columns, i.e. max slot count over all rows
}

// docxParseSpan reads a colspan/rowspan attribute. Missing, empty, malformed
// or non-positive values fall back to 1 (the HTML default); values above
// docxMaxTableSpan are clamped (export-path resource guard).
func docxParseSpan(n *golanghtml.Node, attr string) int {
	for _, a := range n.Attr {
		if a.Key == attr {
			if v, err := strconv.Atoi(strings.TrimSpace(a.Val)); err == nil && v > 1 {
				if v > docxMaxTableSpan {
					return docxMaxTableSpan
				}
				return v
			}
			break
		}
	}
	return 1
}

// docxScanTable lays out the whole table using the standard HTML table
// algorithm: walk every row's columns left to right, skip columns already
// held by a rowspan from an earlier row, and pad any columns this row must
// still cover (including trailing held ones) with continuation slots so each
// row's span sum stays equal to the grid width. Returns the rows in document
// order plus the resulting grid column count.
func docxScanTable(n *golanghtml.Node) docxTableLayout {
	type rowCells struct {
		row   *docxTableRow
		cells []*docxTableCell
	}
	var rows []rowCells
	var collect func(*golanghtml.Node, bool)
	collect = func(node *golanghtml.Node, inHeader bool) {
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != golanghtml.ElementNode {
				continue
			}
			switch c.Data {
			case "thead":
				collect(c, true)
			case "tbody", "tfoot":
				collect(c, inHeader)
			case "tr":
				row := &docxTableRow{isHead: inHeader}
				var cells []*docxTableCell
				for cell := c.FirstChild; cell != nil; cell = cell.NextSibling {
					if cell.Type != golanghtml.ElementNode || (cell.Data != "td" && cell.Data != "th") {
						continue
					}
					if cell.Data == "th" {
						row.isHead = true
					}
					cells = append(cells, &docxTableCell{
						node:    cell,
						colspan: docxParseSpan(cell, "colspan"),
						rowspan: docxParseSpan(cell, "rowspan"),
					})
				}
				rows = append(rows, rowCells{row: row, cells: cells})
			}
		}
	}
	collect(n, false)

	// active[j] tracks the rowspan started above that still covers grid
	// column j: start=true only on the cell's first grid column (the column
	// that must emit the <w:vMerge/> continuation, with the cell's gridSpan);
	// covered further columns are start=false and emit no w:tc at all —
	// Word models a colspan+rowspan continuation as ONE merged tc, not N.
	// The remaining counter lives here (not on docxTableCell.rowspan) so the
	// scan never mutates cell data the emitter still needs to read.
	type activeSpan struct {
		cell  *docxTableCell
		left  int
		start bool
	}
	var active []activeSpan
	ensure := func(total int) {
		for len(active) < total {
			active = append(active, activeSpan{})
		}
	}
	type placement struct {
		cell *docxTableCell
		col  int
	}
	layout := docxTableLayout{}
	for _, rc := range rows {
		// Place this row's own cells: each starts at the next free column,
		// skipping columns held by rowspans from above.
		var placed []placement
		cursor := 0
		for _, cell := range rc.cells {
			for cursor < len(active) && active[cursor].cell != nil {
				cursor++
			}
			ensure(cursor + cell.colspan)
			placed = append(placed, placement{cell: cell, col: cursor})
			cursor += cell.colspan
		}
		// The row must cover at least the columns its own cells span, plus any
		// held columns further right (trailing rowspan continuations).
		total := len(active)
		for _, p := range placed {
			if end := p.col + p.cell.colspan; end > total {
				total = end
			}
		}
		ensure(total)
		slots := make([]docxSlot, total)
		for i := range slots {
			slots[i] = docxSlot{kind: docxSlotEmpty}
		}
		for _, p := range placed {
			// First grid column of the cell emits the w:tc (with gridSpan);
			// further columns of the same cell are span slots — no extra w:tc.
			slots[p.col] = docxSlot{kind: docxSlotCell, cell: p.cell}
			for j := p.col + 1; j < p.col+p.cell.colspan; j++ {
				slots[j] = docxSlot{kind: docxSlotSpan}
			}
		}
		// Columns held by a rowspan from above but not claimed by this row's
		// own cells become continuation slots. Only the rowspan's FIRST grid
		// column emits a <w:vMerge/> tc (carrying the origin cell's gridSpan);
		// the columns it spans emit nothing (kind stays "span").
		for j := range slots {
			if slots[j].kind == docxSlotEmpty && active[j].cell != nil {
				if active[j].start {
					slots[j] = docxSlot{kind: docxSlotCont, cell: active[j].cell}
				} else {
					slots[j] = docxSlot{kind: docxSlotSpan}
				}
			}
		}
		rc.row.slots = slots
		if total > layout.cols {
			layout.cols = total
		}

		// Retire rowspans ending with this row, then register this row's own
		// still-pending rowspans for the rows below.
		for j := range active {
			if active[j].cell == nil {
				continue
			}
			active[j].left--
			if active[j].left <= 1 {
				active[j] = activeSpan{}
			}
		}
		for _, p := range placed {
			if p.cell.rowspan > 1 {
				for j := p.col; j < p.col+p.cell.colspan; j++ {
					ensure(j + 1)
					active[j] = activeSpan{cell: p.cell, left: p.cell.rowspan, start: j == p.col}
				}
			}
		}
	}
	if layout.cols < 1 {
		layout.cols = 1
	}
	// Rows placed before a later row widened the grid are short: pad them to
	// layout.cols with empty slots so every row's span sum equals the grid
	// width (the conservation invariant Word's table model requires).
	for _, rc := range rows {
		for len(rc.row.slots) < layout.cols {
			rc.row.slots = append(rc.row.slots, docxSlot{kind: docxSlotEmpty})
		}
	}
	layout.rows = make([]*docxTableRow, 0, len(rows))
	for _, rc := range rows {
		// An entirely empty <tr> has no cells and holds no continuation
		// columns; emitting it would produce <w:tr></w:tr> with a span sum
		// of 0, which strict readers reject. Drop it.
		if len(rc.row.slots) == 0 {
			continue
		}
		layout.rows = append(layout.rows, rc.row)
	}
	return layout
}

// docxGridWidths distributes docxTableWidthTwips across cols. Widths the HTML
// author supplied via <colgroup>/<col> (style="width:…" or width="…", px or %)
// take priority; any column without an explicit width gets an equal share of
// the remaining twips, and the sum always equals docxTableWidthTwips (the
// last column absorbs the rounding remainder).
func docxGridWidths(n *golanghtml.Node, cols int) []int {
	widths := make([]int, cols)
	explicit := make([]float64, cols) // twips; 0 = no author width
	idx := 0
	var walk func(*golanghtml.Node)
	walk = func(node *golanghtml.Node) {
		for c := node.FirstChild; c != nil && idx < cols; c = c.NextSibling {
			if c.Type != golanghtml.ElementNode {
				continue
			}
			switch c.Data {
			case "colgroup":
				walk(c)
			case "col":
				raw := ""
				for _, a := range c.Attr {
					switch a.Key {
					case "style":
						for _, decl := range strings.Split(a.Val, ";") {
							kv := strings.SplitN(decl, ":", 2)
							if len(kv) == 2 && strings.EqualFold(strings.TrimSpace(kv[0]), "width") {
								raw = strings.TrimSpace(kv[1])
							}
						}
					case "width":
						if raw == "" {
							raw = strings.TrimSpace(a.Val)
						}
					}
				}
				if v, _ := docxParseWidth(raw); v > 0 {
					explicit[idx] = v // docxParseWidth already scales to twips
				}
				idx++
			}
		}
	}
	walk(n)

	// Fallback share for columns without an explicit width: the remaining
	// twips split evenly across them (equal division, the #51 behaviour).
	unset, remain := 0, float64(docxTableWidthTwips)
	for i := 0; i < cols; i++ {
		if explicit[i] > 0 {
			remain -= explicit[i]
		} else {
			unset++
		}
	}
	if remain < 0 {
		remain = 0
	}
	share := 0.0
	if unset > 0 {
		share = remain / float64(unset)
	}
	acc := 0
	for i := 0; i < cols; i++ {
		w := share
		if explicit[i] > 0 {
			w = explicit[i]
		}
		wi := int(w + 0.5)
		if wi < 1 {
			wi = 1
		}
		widths[i] = wi
		acc += wi
	}
	// Reconcile rounding so the gridCol widths sum to docxTableWidthTwips.
	diff := docxTableWidthTwips - acc
	for i := cols - 1; i >= 0 && diff != 0; i-- {
		if diff > 0 {
			widths[i]++
			diff--
		} else if widths[i] > 1 {
			widths[i]--
			diff++
		}
	}
	return widths
}

// docxParseWidth parses a CSS/HTML width token into twips. Percentages are
// scaled against docxTableWidthTwips; bare numbers and px use the CSS 96dpi
// conversion (1px = 15 twips). ok reports whether the token parsed at all —
// callers only need the width, so a failed parse simply yields 0.
func docxParseWidth(raw string) (w float64, ok bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, false
	}
	if strings.HasSuffix(s, "%") {
		if v, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(s, "%")), 64); err == nil && v > 0 {
			return v * float64(docxTableWidthTwips) / 100, true
		}
		return 0, false
	}
	s = strings.TrimSuffix(s, "px")
	if v, err := strconv.ParseFloat(s, 64); err == nil && v > 0 {
		return v * 15, false // 96dpi: 1440 twips / 96 px
	}
	return 0, false
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
	// CT_Tbl requires <w:tblGrid> between tblPr and the first row; Word and
	// strict readers (python-docx) reject or "repair" tables without it.
	layout := docxScanTable(n)
	cols := layout.cols
	if cols < 1 {
		cols = 1
	}
	widths := docxGridWidths(n, cols)
	dc.body.WriteString(`<w:tblGrid>`)
	for _, w := range widths {
		fmt.Fprintf(&dc.body, `<w:gridCol w:w="%d"/>`, w)
	}
	dc.body.WriteString(`</w:tblGrid>`)

	for _, row := range layout.rows {
		dc.body.WriteString(`<w:tr>`)
		for _, slot := range row.slots {
			dc.emitSlot(slot, row.isHead)
		}
		dc.body.WriteString(`</w:tr>`)
	}
	dc.body.WriteString(`</w:tbl>`)
	// Word requires an empty paragraph after a table to permit cursor placement
	dc.openParagraph("", 0, "")
	dc.closeParagraph()
}

// emitSlot writes the w:tc(s) for one grid column of one row. CT_Tc requires
// at least one block-level child, so every emitted tc ends with <w:p/>.
//
//	cell  → <w:tc><w:tcPr>…gridSpan…vMerge…</w:tcPr><w:p/></w:tc>
//	cont  → ONE continuation tc per merged region: <w:vMerge/> plus the
//	        origin cell's gridSpan when it covered more than one column
//	span  → nothing; a gridSpan (real cell or continuation) already covers
//	        this column
//	empty → <w:tc><w:tcPr><w:tcW …/></w:tcPr><w:p/></w:tc>
func (dc *docxConverter) emitSlot(slot docxSlot, isRowHead bool) {
	switch slot.kind {
	case docxSlotSpan:
		return
	case docxSlotCont:
		dc.body.WriteString(`<w:tc><w:tcPr>`)
		if slot.cell != nil && slot.cell.colspan > 1 {
			fmt.Fprintf(&dc.body, `<w:gridSpan w:val="%d"/>`, slot.cell.colspan)
		}
		dc.body.WriteString(`<w:vMerge/><w:tcW w:w="0" w:type="auto"/></w:tcPr><w:p/></w:tc>`)
		return
	case docxSlotEmpty:
		dc.body.WriteString(`<w:tc><w:tcPr><w:tcW w:w="0" w:type="auto"/></w:tcPr><w:p/></w:tc>`)
		return
	}
	cell := slot.cell
	isTh := isRowHead || cell.node.Data == "th"
	dc.body.WriteString(`<w:tc><w:tcPr>`)
	if cell.colspan > 1 {
		fmt.Fprintf(&dc.body, `<w:gridSpan w:val="%d"/>`, cell.colspan)
	}
	if cell.rowspan > 1 {
		dc.body.WriteString(`<w:vMerge w:val="restart"/>`)
	}
	dc.body.WriteString(`<w:tcW w:w="0" w:type="auto"/>`)
	if isTh {
		dc.body.WriteString(`<w:shd w:val="clear" w:color="auto" w:fill="F2F2F2"/>`)
	}
	dc.body.WriteString(`</w:tcPr>`)

	dc.openParagraph("", 0, "")
	dc.processInline(cell.node, docxInlineState{bold: isTh})
	dc.closeParagraph()
	dc.body.WriteString(`</w:tc>`)
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
					if node.Data == "img" {
						dc.processImage(node, docxInlineState{})
					} else {
						dc.processInline(node, docxInlineState{})
					}
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
	case "img":
		// TipTap saves images as block-level <img> siblings of <p> (inline: false).
		// Render in its own paragraph.
		dc.openParagraph("", 0, "")
		dc.processImage(c, docxInlineState{})
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
		"ul", "ol", "table", "pre", "div", "section", "blockquote", "img",
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
			case "img":
				// TipTap blockquote content is block+: an inline:false Image is
				// a block child here. Render it as a quoted paragraph so it is
				// never silently dropped (PR #50 review B1 prime).
				dc.openParagraphEx("", indent, blockquoteShade, "")
				dc.processImage(c, docxInlineState{})
				dc.closeParagraph()
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
            xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
            xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing">
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
		if rel.external {
			docRels.WriteString(fmt.Sprintf(`
  <Relationship Id="%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="%s" TargetMode="External"/>`,
				docxXMLEscape(rel.id), docxXMLEscape(rel.target)))
		} else {
			docRels.WriteString(fmt.Sprintf(`
  <Relationship Id="%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="%s"/>`,
				docxXMLEscape(rel.id), docxXMLEscape(rel.target)))
		}
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
	contentTypes := docxContentTypeXML
	if len(dc.images) > 0 {
		// Insert image <Default> entries before </Types>.
		insert := ""
		seen := map[string]bool{}
		for _, img := range dc.images {
			if !seen[img.mediaType] {
				seen[img.mediaType] = true
				insert += fmt.Sprintf("\n  <Default Extension=\"%s\" ContentType=\"%s\"/>", docxImageExtension(img.mediaType), img.mediaType)
			}
		}
		contentTypes = strings.Replace(docxContentTypeXML, "</Types>", insert+"\n</Types>", 1)
	}
	parts := []partEntry{
		{"[Content_Types].xml", contentTypes},
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

	for _, img := range dc.images {
		w, err := zw.Create("word/media/" + img.name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(img.data); err != nil {
			return nil, err
		}
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}

	return zipBuf.Bytes(), nil
}
