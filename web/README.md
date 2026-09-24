# mist-docs 前端

## 技术栈

- Vue 3 + Vite
- Element Plus
- TipTap (富文本文档)
- 自研 SheetEditor（表格：公式、图表、数据透视；保存后刷新，不走 Yjs）
- Yjs（仅富文本文档，WebSocket 地址 `/ws/teams/:team_id/docs/:doc_id`，token 为 `mist-docs-token`）
- Pinia (状态管理)

## 开发

```bash
cd web
pnpm install
pnpm dev
```
