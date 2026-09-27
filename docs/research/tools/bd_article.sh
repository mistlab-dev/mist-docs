#!/bin/bash
# usage: bd_article.sh <url> [maxchars]
u="$1"; mc="${2:-7000}"
openclaw browser navigate "$u" >/dev/null 2>&1
sleep 6
openclaw browser evaluate --fn "() => { const sel=['article','.Post-RichTextContainer','.RichText','#content','.content','.article-content','.post_text','.d_post_content','.markdown-body','.blog-content-box','.htmledit_views','main'];[...document.querySelectorAll('script,style,nav,footer,header,aside')].forEach(e=>e.remove());for(const s of sel){const el=document.querySelector(s);if(el&&el.innerText&&el.innerText.length>500) return el.innerText.slice(0,$mc);}return document.body.innerText.slice(0,$mc);}"
