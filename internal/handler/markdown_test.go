package handler

import (
	"strings"
	"testing"
)

func TestRenderMarkdownEditorHTML(t *testing.T) {
	md := "---\ntitle: 上线说明\ntags: [a]\n---\n# 标题\n\n第一行\n第二行\n\n" +
		"- [ ] 备份\n- [x] 通知\n\n1. 一\n2. 二\n\n" +
		"| 项 | 值 |\n|---|---|\n| a | b |\n\n" +
		"```bash\necho <hi>\n```\n\n" +
		"![架构](images/a.png) ![远程](https://example.com/x.png) ![[b.png]]\n\n" +
		"<p align=\"center\"><img src=\"images/c.png\" width=300 alt=\"c\"></p>\n\n" +
		"<script>alert(1)</script>\n\n[下一篇](next.md#part) [外链](https://example.com) [坏](javascript:alert(1))\n"
	refs := &mdRefs{
		image: func(ref string) (string, bool) {
			if ref == "images/a.png" || ref == "images/c.png" {
				return "/api/media/t/" + strings.TrimPrefix(ref, "images/") + "?sig=1", true
			}
			return "", false
		},
		link: func(ref string) (string, bool) {
			if ref == "next.md#part" {
				return "/docs/abc", true
			}
			return "", false
		},
	}
	out, title := renderMarkdown(md, refs)
	if title != "上线说明" {
		t.Fatalf("title = %q", title)
	}
	for _, want := range []string{
		"<h1>标题</h1>", "第一行第二行",
		`<ul data-type="taskList">`, `<li data-type="taskItem" data-checked="false">`, `data-checked="true"`,
		"<ol>", "<table>", "<th>项</th>", `<code class="language-bash">echo &lt;hi&gt;`,
		`<img src="/api/media/t/a.png?sig=1" alt="架构">`, `<img src="https://example.com/x.png" alt="远程">`,
		`<img src="/api/media/t/c.png?sig=1" alt="c">`, "图片没有导入：b.png",
		`<a href="/docs/abc">下一篇</a>`, `<a href="https://example.com">外链</a>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	for _, bad := range []string{"<script", "javascript:", "tags:", "raw HTML omitted"} {
		if strings.Contains(out, bad) {
			t.Errorf("unexpected %q in\n%s", bad, out)
		}
	}
	if len(refs.missing) != 1 || refs.missing[0] != "b.png" {
		t.Errorf("missing = %v", refs.missing)
	}
}

func TestMarkdownToHTMLSingleFile(t *testing.T) {
	out := markdownToHTML("# 计划\n\n- 第一步\n- 第二步\n\n```\n<script>x</script>\n```\n\n![图](a.png)")
	for _, want := range []string{"<h1>计划</h1>", "<li>第一步</li>", "&lt;script&gt;", "图片没有导入：a.png"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
}
