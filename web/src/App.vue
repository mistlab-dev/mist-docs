<template>
  <router-view />
</template>

<style>
/* iOS Safari 100vh fix */
:root {
  --vh: 1vh;
}
body {
  font-family: 'Noto Sans SC', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
}

/* Dark mode */
html.dark {
  --el-bg-color: #1a1a2e;
  --el-bg-color-overlay: #16213e;
  --el-bg-color-page: #0f0f23;
  --el-text-color-primary: #e0e0e0;
  --el-text-color-regular: #c0c0c0;
  --el-text-color-secondary: #909399;
  --el-border-color: #363660;
  --el-border-color-light: #2a2a4a;
  --el-fill-color-blank: #1a1a2e;
  --el-color-white: #1a1a2e;
  background-color: #0f0f23;
  color: #e0e0e0;
}

html.dark body {
  background-color: #0f0f23;
  color: #e0e0e0;
}

html.dark .editor-page { background: #1a1a2e; }
html.dark .editor-header { background: #16213e; border-color: #363660; }
html.dark .ProseMirror { background: #1a1a2e; color: #e0e0e0; }
html.dark .ProseMirror h1, html.dark .ProseMirror h2, html.dark .ProseMirror h3 { color: #e0e0e0; }
html.dark .ProseMirror code { background: #2a2a4a; color: #e06c75; }
html.dark .ProseMirror pre { background: #0f0f23; }
html.dark .ProseMirror blockquote { border-color: #363660; color: #909399; }
html.dark .ProseMirror table { border-color: #363660; }
html.dark .ProseMirror th { background: #16213e; color: #e0e0e0; border-color: #363660; }
html.dark .ProseMirror td { border-color: #363660; }
html.dark .ProseMirror a { color: #61afef; }

html.dark .docs-page { background: #0f0f23; }
html.dark .doc-card { background: #1a1a2e; border-color: #363660; }
html.dark .doc-card:hover { border-color: #409eff; }
html.dark .doc-card.selected { background: #16213e; }
html.dark .batch-bar { background: #16213e; border-color: #363660; color: #61afef; }
html.dark .list-header { background: #16213e; color: #e0e0e0; }
html.dark .empty-state { color: #909399; }
html.dark .toolbar { background: #16213e; border-color: #363660; }
html.dark .title-input .el-input__wrapper { background: #1a1a2e; }
html.dark .header-actions .el-input__wrapper { background: #1a1a2e; }

html.dark .sheet-container { background: #1a1a2e; color: #e0e0e0; }
html.dark .sheet-table th { background: #16213e; color: #e0e0e0; border-color: #363660; }
html.dark .sheet-table td { background: #1a1a2e; color: #e0e0e0; border-color: #363660; }
html.dark .sheet-table td.selected { background: #2a2a4a; }
html.dark .sheet-table td.editing { background: #16213e; }
html.dark .formula-bar { background: #16213e; border-color: #363660; color: #e0e0e0; }
html.dark .ribbon { background: #1e1e34; border-color: #363660; }
html.dark .rb-btn { color: #ccc; }
html.dark .rb-btn:hover:not(:disabled) { background: #2a2a4a; border-color: #4a4a7a; }
html.dark .rb-btn.active { background: #2a2a4a; border-color: #5a5aaa; color: #8ab4f8; }
html.dark .rb-sep { background: #363660; }
html.dark .rb-vsep { background: #363660; }
html.dark .ribbon-section-label { color: #777; }
html.dark .cell { background: #1a1a2e; color: #e0e0e0; }
html.dark .cell.sel { background: #2a2a4a !important; }
html.dark .col-hdr { background: linear-gradient(180deg, #1e1e34, #16213e); color: #999; border-color: #363660; }
html.dark .row-hdr { background: linear-gradient(90deg, #1e1e34, #16213e); color: #999; border-color: #363660; }
html.dark .corner-cell { background: #16213e; border-color: #363660; }
html.dark .grid-area { background: #0f0f23; }
html.dark .docs-page .sidebar { background: #1a1a2e; border-color: #363660; }
html.dark .section-item { color: #aaa; }
html.dark .section-item:hover, html.dark .section-item.active { background: #2a2a4a; color: #8ab4f8; }
html.dark .tree-header { background: #16213e; border-color: #363660; color: #ccc; }
html.dark .list-row:hover { background: #1e1e34; }
html.dark .list-header-row { background: #16213e; border-color: #363660; color: #909399; }

html.dark .login-container { background: #0f0f23; }
html.dark .login-card { background: #1a1a2e; }
html.dark .login-card h2 { color: #e0e0e0; }
html.dark .login-card .title { color: #e0e0e0; }
html.dark .login-card .subtitle { color: #909399; }

html.dark .el-table { --el-table-bg-color: #1a1a2e; --el-table-tr-bg-color: #1a1a2e; --el-table-header-bg-color: #16213e; --el-table-row-hover-bg-color: #16213e; --el-table-border-color: #363660; }
html.dark .el-dialog { background: #1a1a2e; }
html.dark .el-card { background: #1a1a2e; border-color: #363660; }

/* Print */
@media print {
  .editor-header, .editor-footer, .el-aside, .topbar, .sidebar,
  .doc-tags-bar, .card-actions, .batch-bar, .toolbar, .menu-btn { display: none !important; }
  .editor-page { height: auto !important; }
  .ProseMirror { padding: 0 !important; overflow: visible !important; }
  body { background: #fff !important; color: #000 !important; }
}
</style>
