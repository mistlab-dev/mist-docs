<template>
  <div class="editor-page">
    <!-- 顶部导航栏 -->
    <div class="editor-header">
      <div class="header-left">
        <el-button @click="router.push('/docs')" text size="small">
          <el-icon><ArrowLeft /></el-icon> {{ t("docEditor.back") }}
        </el-button>
        <el-input v-model="title" class="title-input" @blur="saveTitle" />
        <div class="doc-badges">
          <el-tag size="small" effect="plain" round>{{ doc?.type === 'sheet' ? t('common.sheet') : t('common.doc') }}</el-tag>
          <el-tag size="small" effect="plain" round type="info">v{{ doc?.version || 1 }}</el-tag>
          <el-tag v-if="collabUsers.length" size="small" effect="plain" round type="success">
            <svg class="tag-icon" viewBox="0 0 16 16" fill="currentColor"><circle cx="4" cy="8" r="3"/><circle cx="12" cy="8" r="3" opacity="0.5"/></svg>
            {{ t("docEditor.collabUsers", [collabUsers.length + 1]) }}
          </el-tag>
          <div v-if="collabUsers.length" class="collab-avatars">
            <el-tooltip v-for="u in collabUsers" :key="u.id" :content="u.name" placement="bottom">
              <span class="collab-avatar" :style="{ background: u.color }">{{ u.name?.charAt(0) || '?' }}</span>
            </el-tooltip>
          </div>
        </div>
      </div>
      <div class="header-right">
        <span v-if="saveStatus === 'saving'" class="save-indicator saving">{{ t('docEditor.saving') }}</span>
        <span v-else-if="saveStatus === 'saved'" class="save-indicator saved">{{ t('docEditor.saved') }}</span>
        <span v-else-if="saveStatus === 'error'" class="save-indicator error">{{ t('docEditor.saveFailed') }}</span>

        <el-button v-if="doc?.locked_by && doc?.locked_by !== currentUserId" size="small" type="warning" disabled>
          <svg class="btn-icon" viewBox="0 0 20 20" fill="currentColor"><path d="M10 2a4 4 0 00-4 4v2H5a1 1 0 00-1 1v8a1 1 0 001 1h10a1 1 0 001-1V9a1 1 0 00-1-1h-1V6a4 4 0 00-4-4zm2 6H8V6a2 2 0 114 0v2z"/></svg>
          {{ t("docEditor.locked") }}
        </el-button>
        <el-button v-else-if="doc?.locked_by === currentUserId" size="small" type="warning" @click="unlockDoc">
          <svg class="btn-icon" viewBox="0 0 20 20" fill="currentColor"><path d="M10 2a4 4 0 00-4 4v2H5a1 1 0 00-1 1v8a1 1 0 001 1h10a1 1 0 001-1V9a1 1 0 00-1-1h-1V6a4 4 0 00-4-4zm2 6H8V6a2 2 0 114 0v2zm-2 4a1 1 0 011 1v2a1 1 0 11-2 0v-2a1 1 0 011-1z"/></svg>
          {{ t("docEditor.unlockBtn") }}
        </el-button>
        <el-button v-else size="small" @click="lockDoc">
          <svg class="btn-icon" viewBox="0 0 20 20" fill="currentColor"><path d="M5 8V6a5 5 0 0110 0v2h1a1 1 0 011 1v8a1 1 0 01-1 1H4a1 1 0 01-1-1V9a1 1 0 011-1h1zm2-2a3 3 0 016 0v2H7V6z"/></svg>
          <span class="btn-label">{{ t('docEditor.lockBtn') }}</span>
        </el-button>

        <el-button type="primary" size="small" @click="manualSave" :loading="saving">
          <svg class="btn-icon" viewBox="0 0 20 20" fill="currentColor"><path d="M7 3a1 1 0 00-1 1v2H4a1 1 0 000 2h5a1 1 0 000-2H8V4h8v3a1 1 0 102 0V4a1 1 0 00-1-1H7zM5 10a1 1 0 00-1 1v5a1 1 0 001 1h10a1 1 0 001-1v-5a1 1 0 10-2 0v4H6v-4a1 1 0 00-1-1z"/></svg>
          {{ t("common.save") }}
        </el-button>

        <el-dropdown @command="handleMore">
          <el-button size="small">
            <svg class="btn-icon" viewBox="0 0 20 20" fill="currentColor"><circle cx="5" cy="10" r="1.5"/><circle cx="10" cy="10" r="1.5"/><circle cx="15" cy="10" r="1.5"/></svg>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="share">
                <svg class="menu-icon" viewBox="0 0 20 20" fill="currentColor"><path d="M15 8a3 3 0 10-2.977-2.63l-4.94 2.47a3 3 0 100 4.247l4.959 2.479A3 3 0 1015 12a3 3 0 00-2.965 2.574l-4.96-2.48a3.013 3.013 0 000-2.188l4.96-2.48A3 3 0 1015 8z"/></svg>
                {{ t("docEditor.share") }}
              </el-dropdown-item>
              <el-dropdown-item command="move">
                <svg class="menu-icon" viewBox="0 0 20 20" fill="currentColor"><path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z"/></svg>
                {{ t("docEditor.moveMenu") }}
              </el-dropdown-item>
              <el-dropdown-item v-if="isAdmin" command="watermark">
                <svg class="menu-icon" viewBox="0 0 20 20" fill="currentColor"><path d="M10 2a8 8 0 100 16 8 8 0 000-16zm0 2a6 6 0 016 6h-2a4 4 0 00-4-4V4z"/></svg>
                {{ watermarkOn ? t('docEditor.watermarkOn') : t('docEditor.watermarkOff') }}
              </el-dropdown-item>
              <el-dropdown-item command="comments">
                <svg class="menu-icon" viewBox="0 0 20 20" fill="currentColor"><path d="M2 5a2 2 0 012-2h7a2 2 0 012 2v4a2 2 0 01-2 2H6l-3 3V5z"/></svg>
                {{ t("docEditor.commentsMenu") }}
              </el-dropdown-item>
              <el-dropdown-item command="stats">
                <svg class="menu-icon" viewBox="0 0 20 20" fill="currentColor"><path d="M2 11a1 1 0 011-1h2a1 1 0 011 1v5a1 1 0 01-1 1H3a1 1 0 01-1-1v-5zm6-4a1 1 0 011-1h2a1 1 0 011 1v9a1 1 0 01-1 1H9a1 1 0 01-1-1V7zm6-3a1 1 0 011-1h2a1 1 0 011 1v12a1 1 0 01-1 1h-2a1 1 0 01-1-1V4z"/></svg>
                {{ t("docEditor.statsMenu") }}
              </el-dropdown-item>
              <el-dropdown-item command="versions" divided>
                <svg class="menu-icon" viewBox="0 0 20 20" fill="currentColor"><path d="M4 4a2 2 0 012-2h8a2 2 0 012 2v12a2 2 0 01-2 2H6a2 2 0 01-2-2V4zm2 0v12h8V4H6z"/></svg>
                {{ t("docEditor.versionHistory") }}
              </el-dropdown-item>
              <el-dropdown-item command="export" divided>
                <svg class="menu-icon" viewBox="0 0 20 20" fill="currentColor"><path d="M3 17a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1zm3.293-7.707a1 1 0 011.414 0L9 10.586V3a1 1 0 112 0v7.586l1.293-1.293a1 1 0 111.414 1.414l-3 3a1 1 0 01-1.414 0l-3-3a1 1 0 010-1.414z"/></svg>
                {{ t("docEditor.exportMenu") }}
              </el-dropdown-item>
              <el-dropdown-item v-if="doc?.type === 'doc'" command="save-template">
                <svg class="menu-icon" viewBox="0 0 20 20" fill="currentColor"><path d="M4 4a2 2 0 012-2h8a2 2 0 012 2v12a2 2 0 01-2 2H6a2 2 0 01-2-2V4zm2 0v12h8V4H6zm1 3h6v2H7V7zm0 4h4v2H7v-2z"/></svg>
                {{ t("docEditor.saveAsTemplate") }}
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </div>

        <!-- TipTap 工具栏 -->
    <div v-if="editor" class="toolbar">
      <div class="tb-group">
        <button class="tb-btn" :class="{active: editor.isActive('bold')}" @click="editor.chain().focus().toggleBold().run()" :title="t('docEditor.toolbar.bold')">
          <svg viewBox="0 0 20 20" fill="currentColor"><path d="M6 4h5.5a3.5 3.5 0 012.5 6 3.5 3.5 0 01-2.5 6H6V4zm2 2v3h3.5a1.5 1.5 0 000-3H8zm0 5v3h3.5a1.5 1.5 0 000-3H8z"/></svg>
        </button>
        <button class="tb-btn" :class="{active: editor.isActive('italic')}" @click="editor.chain().focus().toggleItalic().run()" :title="t('docEditor.toolbar.italic')">
          <svg viewBox="0 0 20 20" fill="currentColor"><path d="M8 4h7v2h-2.5l-2 8H13v2H6v-2h2.5l2-8H8V4z"/></svg>
        </button>
        <button class="tb-btn" :class="{active: editor.isActive('strike')}" @click="editor.chain().focus().toggleStrike().run()" :title="t('docEditor.toolbar.strikethrough')">
          <svg viewBox="0 0 20 20" fill="currentColor"><path d="M4 9h12v2H4V9zm4-3a2 2 0 114 0h2a4 4 0 10-8 0h2zm4 8a2 2 0 11-4 0H4a4 4 0 108 0h-2z"/></svg>
        </button>
        <button class="tb-btn" :class="{active: editor.isActive('underline')}" @click="editor.chain().focus().toggleUnderline().run()" :title="t('docEditor.toolbar.underline')">
          <svg viewBox="0 0 20 20" fill="currentColor"><path d="M6 4v5a4 4 0 008 0V4h2v5a6 6 0 01-12 0V4h2zm-2 13h12v2H4v-2z"/></svg>
        </button>
      </div>

      <div class="tb-sep"></div>

      <div class="tb-group">
        <button class="tb-btn" :class="{active: editor.isActive('heading', {level:1})}" @click="editor.chain().focus().toggleHeading({level:1}).run()" :title="t('docEditor.toolbar.heading1')">H1</button>
        <button class="tb-btn" :class="{active: editor.isActive('heading', {level:2})}" @click="editor.chain().focus().toggleHeading({level:2}).run()" :title="t('docEditor.toolbar.heading2')">H2</button>
        <button class="tb-btn" :class="{active: editor.isActive('heading', {level:3})}" @click="editor.chain().focus().toggleHeading({level:3}).run()" :title="t('docEditor.toolbar.heading3')">H3</button>
      </div>

      <div class="tb-sep"></div>

      <div class="tb-group">
        <button class="tb-btn" :class="{active: editor.isActive('bulletList')}" @click="editor.chain().focus().toggleBulletList().run()" :title="t('docEditor.toolbar.bulletList')">
          <svg viewBox="0 0 20 20" fill="currentColor"><circle cx="4" cy="5" r="1.5"/><circle cx="4" cy="10" r="1.5"/><circle cx="4" cy="15" r="1.5"/><rect x="8" y="4" width="10" height="2" rx="1"/><rect x="8" y="9" width="10" height="2" rx="1"/><rect x="8" y="14" width="10" height="2" rx="1"/></svg>
        </button>
        <button class="tb-btn" :class="{active: editor.isActive('orderedList')}" @click="editor.chain().focus().toggleOrderedList().run()" :title="t('docEditor.toolbar.orderedList')">
          <svg viewBox="0 0 20 20" fill="currentColor"><text x="2" y="7" font-size="6" font-weight="bold">1</text><text x="2" y="12" font-size="6" font-weight="bold">2</text><text x="2" y="17" font-size="6" font-weight="bold">3</text><rect x="8" y="4" width="10" height="2" rx="1"/><rect x="8" y="9" width="10" height="2" rx="1"/><rect x="8" y="14" width="10" height="2" rx="1"/></svg>
        </button>
        <button class="tb-btn" :class="{active: editor.isActive('taskList')}" @click="editor.chain().focus().toggleTaskList().run()" :title="t('docEditor.toolbar.taskList')">
          <svg viewBox="0 0 20 20" fill="currentColor"><rect x="2" y="4" width="5" height="5" rx="1" stroke="currentColor" fill="none" stroke-width="1.5"/><path d="M3.5 6.5L5 8l3-3.5" stroke="currentColor" fill="none" stroke-width="1.5"/><rect x="10" y="5" width="8" height="2" rx="1"/><rect x="2" y="11" width="5" height="5" rx="1" stroke="currentColor" fill="none" stroke-width="1.5"/><rect x="10" y="12.5" width="8" height="2" rx="1"/></svg>
        </button>
        <button class="tb-btn" :class="{active: editor.isActive('blockquote')}" @click="editor.chain().focus().toggleBlockquote().run()" :title="t('docEditor.toolbar.blockquote')">
          <svg viewBox="0 0 20 20" fill="currentColor"><path d="M4 4h4v4H6l-1 3H3l1-3V4zm8 0h4v4h-2l-1 3h-2l1-3V4z"/></svg>
        </button>
      </div>

      <div class="tb-sep"></div>

      <div class="tb-group">
        <button class="tb-btn" :class="{active: editor.isActive('codeBlock')}" @click="toggleCodeBlock" :title="t('docEditor.toolbar.codeBlock')">
          <svg viewBox="0 0 20 20" fill="currentColor"><path d="M6.707 4.293a1 1 0 010 1.414L3.414 9l3.293 3.293a1 1 0 01-1.414 1.414l-4-4a1 1 0 010-1.414l4-4a1 1 0 011.414 0zm6.586 0a1 1 0 011.414 0l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414-1.414L16.586 9l-3.293-3.293a1 1 0 010-1.414z"/></svg>
        </button>
        <button class="tb-btn" :class="{active: editor.isActive('code')}" @click="editor.chain().focus().toggleCode().run()" :title="t('docEditor.toolbar.inlineCode')">
          <svg viewBox="0 0 20 20" fill="currentColor"><path d="M7.4 4.3L2.7 9l4.7 4.7-1.4 1.4L0 9l6-6 1.4 1.3zm5.2 0L17.3 9l-4.7 4.7 1.4 1.4L20 9l-6-6-1.4 1.3z"/></svg>
        </button>
      </div>

      <div class="tb-sep"></div>

      <div class="tb-group">
        <button class="tb-btn" :class="{active: editor.isActive('link')}" @click="insertLink" :title="t('docEditor.toolbar.link')">
          <svg viewBox="0 0 20 20" fill="currentColor"><path d="M12.586 4.586a2 2 0 112.828 2.828l-3.879 3.879a2 2 0 01-2.828 0 1 1 0 00-1.414 1.414 4 4 0 005.656 0l3.879-3.879a4 4 0 00-5.656-5.656L8.12 5.464a1 1 0 001.414 1.414l3.052-3.292z"/><path d="M7.414 15.414a2 2 0 11-2.828-2.828l3.879-3.879a2 2 0 012.828 0 1 1 0 001.414-1.414 4 4 0 00-5.656 0L3.172 11.17a4 4 0 005.656 5.656l2.828-2.828a1 1 0 10-1.414-1.414l-2.828 2.83z"/></svg>
        </button>
        <button class="tb-btn" @click="triggerImageUpload" :title="t('docEditor.toolbar.image')">
          <svg viewBox="0 0 20 20" fill="currentColor"><path d="M4 3a2 2 0 00-2 2v10a2 2 0 002 2h12a2 2 0 002-2V5a2 2 0 00-2-2H4zm12 12H4l4-6 3 4 2-3 3 5z"/><circle cx="13" cy="7" r="2"/></svg>
        </button>
        <button class="tb-btn" @click="showMediaLib = true" :title="t('docEditor.mediaLibTitle')">
          <svg viewBox="0 0 20 20" fill="currentColor"><path d="M3 3a1 1 0 011-1h12a1 1 0 011 1v2.586l2.707-2.707a1 1 0 011.414 1.414L16.414 7H19a1 1 0 010 2h-4a1 1 0 01-1-1V5H5v10h4a1 1 0 010 2H5a1 1 0 01-1-1V3z"/></svg>
        </button>
        <button class="tb-btn" @click="insertTable" :title="t('docEditor.toolbar.insertTable')">
          <svg viewBox="0 0 20 20" fill="currentColor"><path d="M3 4h14a1 1 0 011 1v10a1 1 0 01-1 1H3a1 1 0 01-1-1V5a1 1 0 011-1zm1 2v3h5V6H4zm0 5v3h5v-3H4zm7-5v3h5V6h-5zm0 5v3h5v-3h-5z"/></svg>
        </button>
        <button class="tb-btn" @click="editor.chain().focus().setHorizontalRule().run()" :title="t('docEditor.toolbar.hr')">
          <svg viewBox="0 0 20 20" fill="currentColor"><rect x="2" y="9" width="16" height="2" rx="1"/></svg>
        </button>
      </div>

      <!-- 表格操作（仅选中表格时显示） -->
      <template v-if="editor.isActive('table')">
        <div class="tb-sep"></div>
        <div class="tb-group">
          <button class="tb-btn" @click="editor.chain().focus().addRowBefore().run()" :title="t('docEditor.toolbar.addRowBefore')">
            <svg viewBox="0 0 20 20" fill="currentColor"><rect x="3" y="3" width="14" height="14" rx="1" fill="none" stroke="currentColor" stroke-width="1.5"/><line x1="10" y1="5" x2="10" y2="15" stroke="currentColor" stroke-width="1.5"/><path d="M7 8h6M10 5v6" stroke="currentColor" stroke-width="1.5"/></svg>
          </button>
          <button class="tb-btn" @click="editor.chain().focus().addRowAfter().run()" :title="t('docEditor.toolbar.addRowAfter')">
            <svg viewBox="0 0 20 20" fill="currentColor"><rect x="3" y="3" width="14" height="14" rx="1" fill="none" stroke="currentColor" stroke-width="1.5"/><line x1="10" y1="5" x2="10" y2="15" stroke="currentColor" stroke-width="1.5"/><path d="M7 12h6M10 9v6" stroke="currentColor" stroke-width="1.5"/></svg>
          </button>
          <button class="tb-btn" @click="editor.chain().focus().deleteRow().run()" :title="t('docEditor.toolbar.deleteRow')" class-name="danger">
            <svg viewBox="0 0 20 20" fill="#f56c6c"><rect x="3" y="3" width="14" height="14" rx="1" fill="none" stroke="#f56c6c" stroke-width="1.5"/><line x1="10" y1="3" x2="10" y2="17" stroke="#f56c6c" stroke-width="1.5"/><line x1="6" y1="10" x2="14" y2="10" stroke="#f56c6c" stroke-width="2"/></svg>
          </button>
          <button class="tb-btn" @click="editor.chain().focus().mergeCells().run()" :title="t('docEditor.toolbar.mergeCells')">{{ t('docEditor.toolbar.merge') }}</button>
          <button class="tb-btn" @click="editor.chain().focus().splitCell().run()" :title="t('docEditor.toolbar.splitCell')">{{ t('docEditor.toolbar.split') }}</button>
          <button class="tb-btn" @click="editor.chain().focus().deleteTable().run()" :title="t('docEditor.toolbar.deleteTable')" style="color:#f56c6c">
            <svg viewBox="0 0 20 20" fill="currentColor"><path d="M4.707 3.293a1 1 0 00-1.414 1.414L8.586 10l-5.293 5.293a1 1 0 001.414 1.414L10 11.414l5.293 5.293a1 1 0 001.414-1.414L11.414 10l5.293-5.293a1 1 0 00-1.414-1.414L10 8.586 4.707 3.293z"/></svg>
          </button>
        </div>
      </template>

      <div style="flex:1"></div>

      <div class="tb-group">
        <button class="tb-btn" @click="editor.chain().focus().undo().run()" :title="t('docEditor.toolbar.undo')">
          <svg viewBox="0 0 20 20" fill="currentColor"><path d="M3 8l4-4v3h6a3 3 0 010 6H9v-2h4a1 1 0 000-2H7v3L3 8z"/></svg>
        </button>
        <button class="tb-btn" @click="editor.chain().focus().redo().run()" :title="t('docEditor.toolbar.redo')">
          <svg viewBox="0 0 20 20" fill="currentColor"><path d="M17 8l-4-4v3H7a3 3 0 000 6h4v-2H7a1 1 0 010-2h6v3l4-4z"/></svg>
        </button>
      </div>
    </div>

        <input type="file" ref="imageInput" style="display:none" accept="image/*" @change="handleImageUpload" />

    <!-- 文档编辑器 -->
    <div v-if="doc?.type === 'doc' && editor" class="editor-body with-outline">
      <editor-content :editor="editor as any" class="tiptap-editor" @click="handleEditorClick" />
      <!-- 大纲导航 -->
      <div class="outline-panel" :class="{ collapsed: outlineCollapsed }">
        <div class="outline-header" @click="outlineCollapsed = !outlineCollapsed">
          <span class="outline-title">{{ t('docEditor.outline') }}</span>
          <el-icon :size="14" class="outline-toggle"><ArrowRight v-if="outlineCollapsed" /><ArrowLeft v-else /></el-icon>
        </div>
        <div v-if="!outlineCollapsed && !outlineItems.length" class="outline-empty">{{ t('docEditor.noOutline') }}</div>
        <div v-if="!outlineCollapsed" class="outline-list">
          <div
            v-for="(item, i) in outlineItems" :key="i"
            class="outline-item"
            :class="['outline-h' + item.level, { active: outlineActiveId === item.id }]"
            @click="scrollToHeading(item.id)"
          >{{ item.text }}</div>
        </div>
      </div>
    </div>

    <!-- 表格编辑器 -->
    <div v-else-if="doc?.type === 'sheet'" class="editor-body sheet-body">
      <SheetEditor ref="sheetRef" :initial-data="sheetData" @change="onSheetChange" />
    </div>

    <!-- 水印层 -->
    <div v-if="watermarkOn" class="watermark-layer">
      <div v-for="i in 40" :key="i" class="watermark-text">{{ currentUser }} · {{ formatTime(new Date().toISOString()) }}</div>
    </div>

    <!-- 版本回退确认 -->
    <el-dialog v-model="versionDialog.show" :title="t('docEditor.versionHistory')" width="560px" :fullscreen="windowWidth < 768">
      <el-timeline style="max-height:400px;overflow-y:auto">
        <el-timeline-item
          v-for="v in versions" :key="v.version"
          :timestamp="formatTime(v.created_at) + ' · ' + (v.created_by_name || t('common.unknown'))"
          :type="v.version === versionDialog.version ? 'primary' : ''"
          placement="top"
        >
          <div style="display:flex;align-items:center;gap:8px">
            <span> {{ t('docEditor.versionLabel', [v.version]) }} v.version }}</span>
            <span v-if="v.version === doc?.version" style="color:#409eff;font-size:12px">{{ t('common.current') }}</span>
            <el-button v-if="v.version !== doc?.version" size="small" text type="primary" @click="previewVersion(v.version)">{{ t('docEditor.previewBtn') }}</el-button>
            <el-button v-if="v.version !== doc?.version" size="small" text type="primary" @click="openDiff(v.version)">{{ t('docEditor.diffBtn') }}</el-button>
            <el-button v-if="v.version !== doc?.version" size="small" text type="warning" @click="selectRestoreVersion(v.version)">{{ t('docEditor.restoreBtn') }}</el-button>
          </div>
        </el-timeline-item>
      </el-timeline>
      <template #footer>
        <el-button @click="versionDialog.show = false">{{ t('common.close') }}</el-button>
      </template>
    </el-dialog>

    <!-- 版本预览弹窗 -->
    <el-dialog v-model="previewDialog.show" :title="t('docEditor.previewVersion', [previewDialog.version])" width="700px" :fullscreen="windowWidth < 768">
      <div v-if="previewDialog.loading" style="text-align:center;padding:40px">
        <el-icon class="is-loading" :size="24"><Loading /></el-icon>
        <p style="margin-top:12px;color:#909399">{{ t('common.loading') }}</p>
      </div>
      <div v-else class="preview-content" v-html="previewDialog.html"></div>
      <template #footer>
        <el-button @click="previewDialog.show = false">{{ t('common.close') }}</el-button>
        <el-button type="primary" @click="selectRestoreVersion(previewDialog.version); previewDialog.show = false">{{ t('docEditor.restoreVersion') }}</el-button>
      </template>
    </el-dialog>

    <!-- 统计弹窗 -->
    <el-dialog v-model="showStats" :title="t('docEditor.docStats')" width="580px" :fullscreen="windowWidth < 768">
      <div v-if="stats" class="stats-section">
        <!-- 核心指标 -->
        <div class="stats-grid">
          <div class="stat-item">
            <div class="stat-value">{{ stats.word_count?.toLocaleString() }}</div>
            <div class="stat-label">{{ t('docEditor.statWordCount') }}</div>
          </div>
          <div class="stat-item">
            <div class="stat-value">{{ stats.char_count?.toLocaleString() }}</div>
            <div class="stat-label">{{ t('docEditor.statCharCount') }}</div>
          </div>
          <div class="stat-item">
            <div class="stat-value">{{ t("docEditor.statReadTime", [stats.reading_time]) }}</div>
            <div class="stat-label">{{ t("docEditor.statReadTimeLabel") }}</div>
          </div>
          <div class="stat-item">
            <div class="stat-value">{{ stats.edit_count }}</div>
            <div class="stat-label">{{ t('docEditor.statEditCount') }}</div>
          </div>
          <div class="stat-item">
            <div class="stat-value">{{ stats.contributors }}</div>
            <div class="stat-label">{{ t('docEditor.statContributors') }}</div>
          </div>
          <div class="stat-item">
            <div class="stat-value">{{ stats.comment_count }}</div>
            <div class="stat-label">{{ t('docEditor.statCommentCount') }}</div>
          </div>
        </div>

        <!-- 文档结构 -->
        <div class="stats-subsection">
          <div class="stats-subtitle">{{ t('docEditor.docStructure') }}</div>
          <div class="stats-structure">
            <span v-if="stats.headings" :title="t('common.title')">📑 {{ t("docEditor.statHeadings", [stats.headings]) }}</span>
            <span v-if="stats.paragraphs" :title="t('docEditor.statParagraphs')">¶ {{ t("docEditor.statParagraphs", [stats.paragraphs]) }}</span>
            <span v-if="stats.images" :title="t('docEditor.toolbar.image')">🖼 {{ t("docEditor.statImages", [stats.images]) }}</span>
            <span v-if="stats.links" :title="t('docEditor.toolbar.link')">🔗 {{ t("docEditor.statLinks", [stats.links]) }}</span>
            <span v-if="stats.tables" :title="t('docEditor.toolbar.insertTable')">📊 {{ t("docEditor.statTables", [stats.tables]) }}</span>
            <span v-if="stats.code_blocks" :title="t('docEditor.toolbar.codeBlock')">💻 {{ t("docEditor.statCodeBlocks", [stats.code_blocks]) }}</span>
          </div>
        </div>

        <!-- 贡献者列表 -->
        <div v-if="stats.contributor_list?.length" class="stats-subsection">
          <div class="stats-subtitle">{{ t("docEditor.contributorsTitle") }}</div>
          <div class="stats-contributors">
            <el-tag v-for="u in stats.contributor_list" :key="u.id" size="small" type="info" style="margin:2px 4px">{{ u.name }}</el-tag>
          </div>
        </div>

        <!-- 时间信息 -->
        <div v-if="stats.first_edit || stats.file_size" class="stats-subsection">
          <div class="stats-subtitle">{{ t("docEditor.timeInfoTitle") }}</div>
          <div class="stats-meta">
            <span v-if="stats.first_edit">📅 {{ t("docEditor.createdAt", [stats.first_edit]) }}</span>
            <span v-if="stats.last_edit">✏️ {{ t("docEditor.lastEditAt", [stats.last_edit]) }}</span>
            <span v-if="stats.file_size">💾 {{ formatFileSize(stats.file_size) }}</span>
          </div>
        </div>

        <!-- 编辑活动图 -->
        <div v-if="stats.daily_edits?.length" class="stats-subsection">
          <div class="stats-subtitle">{{ t("docEditor.editActivity") }}</div>
          <div class="activity-chart">
            <div v-for="(d, i) in stats.daily_edits" :key="i" class="activity-bar"
                 :style="{ height: Math.min(d.count * 20, 60) + 'px' }"
                 :title="d.date + ': ' + d.count + ' ' + t('common.count')">
            </div>
          </div>
        </div>

        <!-- 活跃时段 -->
        <div v-if="stats.hourly_edits" class="stats-subsection">
          <div class="stats-subtitle">{{ t("docEditor.activePeriod") }}</div>
          <div class="hourly-chart">
            <div v-for="(count, h) in stats.hourly_edits" :key="h" class="hourly-cell"
                 :class="{ active: count > 0 }"
                 :style="{ opacity: count > 0 ? Math.min(count / Math.max(...stats.hourly_edits.filter((v: number) => v > 0) || [1]), 1) * 0.8 + 0.2 : 0.1 }"
                 :title="h + ':00 — ' + count + ' ' + t('common.count')">
              {{ h }}
            </div>
          </div>
          <div class="hourly-labels"><span v-for="h in t('docEditor.hourLabels')" :key="h">{{ h }}</span></div>
        </div>
      </div>
    </el-dialog>

    <!-- 版本对比弹窗 -->
    <el-dialog v-model="showDiff" :title="t('docEditor.versionDiff')" width="700px" :fullscreen="windowWidth < 768">
      <div class="diff-toolbar">
        <span>{{ t("docEditor.diffVersion") }} </span>
        <el-select v-model="diffOld" size="small" style="width:120px">
          <el-option v-for="v in versions" :key="v.version" :label="'v' + v.version" :value="v.version" />
        </el-select>
        <span style="margin:0 8px">→</span>
        <el-select v-model="diffNew" size="small" style="width:120px">
          <el-option v-for="v in versions" :key="v.version" :label="'v' + v.version" :value="v.version" />
        </el-select>
        <el-button size="small" type="primary" @click="loadDiff" :loading="diffLoading">{{ t("docEditor.diffBtn") }}</el-button>
      </div>
      <div v-if="diffHtml" class="diff-content" v-html="diffHtml" />
    </el-dialog>

    <!-- 链接弹窗 -->
    <el-dialog v-model="linkDialog.show" :title="t('docEditor.insertLinkTitle')" width="480px">
      <el-form label-width="60px">
        <el-form-item :label="t('docEditor.linkTextLabel')"><el-input v-model="linkDialog.text" :placeholder="t('docEditor.linkTextPlaceholder')" /></el-form-item>
        <el-form-item :label="t('docEditor.linkUrlLabel')"><el-input v-model="linkDialog.url" :placeholder="t('docEditor.linkUrlPlaceholder')" @input="searchDocsForLink" /></el-form-item>
        <div v-if="linkDialog.results.length" class="link-search-results">
          <div v-for="d in linkDialog.results" :key="d.id" class="link-search-item" @click="selectDocLink(d)">
            <span class="link-doc-title">{{ d.title }}</span>
            <span class="link-doc-type">{{ d.type === 'doc' ? t('common.doc') : t('common.sheet') }}</span>
          </div>
        </div>
      </el-form>
      <template #footer>
        <el-button @click="linkDialog.show = false">{{ t('common.cancel') }}</el-button>
        <el-button v-if="editor?.isActive('link')" type="danger" @click="removeLink">{{ t('docEditor.removeLinkBtn') }}</el-button>
        <el-button type="primary" @click="confirmLink">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- 代码语言弹窗 -->
    <el-dialog v-model="codeLangDialog.show" :title="t('docEditor.codeLangTitle')" width="320px">
      <el-select v-model="codeLangDialog.lang" :placeholder="t('docEditor.selectLangPlaceholder')" style="width:100%">
        <el-option-group :label="t('docEditor.popularLangs')">
          <el-option v-for="l in popularLangs" :key="l" :label="l" :value="l" />
        </el-option-group>
        <el-option-group :label="t('docEditor.otherLangs')">
          <el-option v-for="l in otherLangs" :key="l" :label="l" :value="l" />
        </el-option-group>
      </el-select>
      <template #footer>
        <el-button @click="codeLangDialog.show = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="confirmCodeLang">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- 媒体库弹窗 -->
    <el-dialog v-model="showMediaLib" :title="t('docEditor.mediaLibTitle')" width="700px" :fullscreen="windowWidth < 768">
      <div class="media-toolbar">
        <el-radio-group v-model="mediaFilter" size="small" @change="loadMedia">
          <el-radio-button value="">{{ t('docEditor.mediaAll') }}</el-radio-button>
          <el-radio-button value="image">{{ t('docEditor.mediaImages') }}</el-radio-button>
          <el-radio-button value="document">{{ t('docEditor.mediaFiles') }}</el-radio-button>
        </el-radio-group>
      </div>
      <div v-if="mediaLoading" style="text-align:center;padding:40px"><el-skeleton :rows="4" animated /></div>
      <div v-else-if="!mediaItems.length" style="text-align:center;padding:40px;color:#c0c4cc">{{ t('docEditor.mediaEmpty') }}</div>
      <div v-else class="media-grid">
        <div v-for="item in mediaItems" :key="item.name" class="media-item" @click="insertMedia(item)">
          <div class="media-preview">
            <img v-if="item.type === 'image'" :src="item.url" :alt="item.name" />
            <div v-else class="media-file-icon">{{ item.type === 'document' ? '📄' : '📦' }}</div>
          </div>
          <div class="media-name" :title="item.name">{{ item.name }}</div>
          <div class="media-size">{{ formatFileSize(item.size) }}</div>
        </div>
      </div>
    </el-dialog>

    <!-- 标签 -->
    <div v-if="docTags.length" class="doc-tags-bar">
      <el-tag
        v-for="tag in docTags" :key="tag.id"
        :color="tag.color" effect="dark" size="small"
        closable @close="removeTag(tag.id)"
        style="margin-right:6px"
      >{{ tag.name }}</el-tag>
      <el-button size="small" text @click="showTagDialog = true">{{ t("docEditor.addTagBtn") }}</el-button>
    </div>

    <!-- 标签管理弹窗 -->
    <el-dialog v-model="showTagDialog" :title="t('docEditor.manageTagsTitle')" width="400px">
      <div v-if="allTags.length" style="margin-bottom:12px">
        <p style="color:#999;font-size:13px;margin-bottom:8px">{{ t("docEditor.clickToAddTag") }}</p>
        <el-tag
          v-for="tag in allTags" :key="tag.id"
          :color="tag.color" effect="dark" size="small"
          :class="{ 'tag-disabled': docTagIds.includes(tag.id) }"
          style="margin:0 6px 6px 0;cursor:pointer"
          @click="addTag(tag.id)"
        >{{ tag.name }}</el-tag>
      </div>
      <div style="display:flex;gap:8px">
        <el-input v-model="newTagName" :placeholder="t('docEditor.newTagNamePlaceholder')" size="small" style="flex:1" />
        <el-color-picker v-model="newTagColor" size="small" />
        <el-button size="small" type="primary" @click="createAndAddTag">{{ t('common.create') }}</el-button>
      </div>
      <template #footer>
        <el-button @click="showTagDialog = false">{{ t('common.finish') }}</el-button>
      </template>
    </el-dialog>

    <!-- 移动文档弹窗 -->
    <el-dialog v-model="showMoveDialog" :title="t('docEditor.moveDocTitle')" width="400px">
      <p style="color:#999;margin-bottom:12px">{{ t("docEditor.moveDocDesc") }}</p>
      <el-tree
        :data="folderTree"
        :props="{ label: 'name', children: 'children', value: 'id' }"
        node-key="id"
        highlight-current
        default-expand-all
        @node-click="moveTarget = $event.id"
      >
        <template #default="{ node, data }">
          <span :style="{ color: moveTarget === data.id ? '#409eff' : '' }">
            <svg style="width:16px;height:16px;vertical-align:-2px;margin-right:4px" viewBox="0 0 20 20" fill="currentColor"><path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z"/></svg>{{ data.name }}
          </span>
        </template>
      </el-tree>
      <template #footer>
        <el-button @click="showMoveDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="moveDoc" :disabled="!moveTarget">{{ t('docEditor.moveMenu') }}</el-button>
      </template>
    </el-dialog>

    <!-- 导出弹窗 -->
    <el-dialog v-model="showExportDialog" :title="t('docEditor.exportTitle')" width="420">
      <div class="export-options">
        <div class="export-option" @click="handleExport('pdf'); showExportDialog = false">
          <div class="export-icon"><svg viewBox="0 0 20 20" fill="currentColor" width="24" height="24"><path d="M4 4a2 2 0 012-2h8a2 2 0 012 2v12a2 2 0 01-2 2H6a2 2 0 01-2-2V4zm2 0v12h8V4H6zm1 3h6v2H7V7zm0 4h4v2H7v-2z"/></svg></div>
          <div class="export-info">
            <div class="export-name">PDF</div>
            <div class="export-desc">{{ t("docEditor.exportPdfDesc") }}</div>
          </div>
        </div>
        <div class="export-option" @click="handleExport('html'); showExportDialog = false">
          <div class="export-icon">🌐</div>
          <div class="export-info">
            <div class="export-name">HTML</div>
            <div class="export-desc">{{ t("docEditor.exportHtmlDesc") }}</div>
          </div>
        </div>
        <div class="export-option" @click="handleExport('markdown'); showExportDialog = false">
          <div class="export-icon"><svg viewBox="0 0 20 20" fill="currentColor" width="24" height="24"><path d="M4 4a2 2 0 012-2h8a2 2 0 012 2v12a2 2 0 01-2 2H6a2 2 0 01-2-2V4zm2 0v12h8V4H6zm1 2h6v1H7V6zm0 2h6v1H7V8zm0 2h4v1H7v-1z"/></svg></div>
          <div class="export-info">
            <div class="export-name">Markdown</div>
            <div class="export-desc">{{ t("docEditor.exportTxtDesc") }}</div>
          </div>
        </div>
        <div class="export-option" @click="handleExport('docx'); showExportDialog = false">
          <div class="export-icon">📘</div>
          <div class="export-info">
            <div class="export-name">Word</div>
            <div class="export-desc">{{ t("docEditor.exportDocxDesc") }}</div>
          </div>
        </div>
        <div class="export-option" @click="handleExport('txt'); showExportDialog = false">
          <div class="export-icon">📃</div>
          <div class="export-info">
            <div class="export-name">{{ t("docEditor.exportPlainName") }}</div>
            <div class="export-desc">{{ t("docEditor.exportPlainDesc") }}</div>
          </div>
        </div>
      </div>
    </el-dialog>

    <!-- 分享/权限弹窗 (Google Docs 风格) -->
    <el-dialog v-model="showShareDialog" :title="t('docEditor.shareDialogTitle')" width="560" destroy-on-close @opened="loadCollaborators">
      <!-- 添加人员 -->
      <div class="share-add-row">
        <el-autocomplete
          v-model="targetSearch"
          :fetch-suggestions="searchTargets"
          :placeholder="t('docEditor.addPeoplePlaceholder')"
          style="flex:1"
          @select="onTargetSelect"
          clearable
        >
          <template #default="{ item }">
            <div class="target-option">
              <div class="target-avatar" :style="{ background: item.type === 'department' ? '#e6f7ff' : '#f0f0f0' }">
                <span v-if="item.type === 'department'"><svg style="width:16px;height:16px;vertical-align:-2px" viewBox="0 0 20 20" fill="currentColor"><path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z"/></svg></span>
                <span v-else><svg style="width:16px;height:16px;vertical-align:-2px" viewBox="0 0 20 20" fill="currentColor"><path d="M10 10a4 4 0 100-8 4 4 0 000 8zm0 2c-4.418 0-8 1.79-8 4v2h16v-2c0-2.21-3.582-4-8-4z"/></svg></span>
              </div>
              <div class="target-info">
                <div class="target-name">{{ item.name }}</div>
                <div class="target-sub">{{ item.type === 'department' ? t('common.department') : item.username }}</div>
              </div>
            </div>
          </template>
        </el-autocomplete>
        <el-select v-model="newRole" style="width:120px">
          <el-option :label="t('docEditor.sharePermViewer')" value="viewer" />
          <el-option :label="t('docEditor.sharePermEditor')" value="editor" />
        </el-select>
        <el-button type="primary" @click="addCollaborator" :disabled="!selectedTarget">{{ t('docEditor.addPeopleBtn') }}</el-button>
      </div>

      <!-- 协作者列表 -->
      <div class="collaborators-list">
        <div v-if="collaboratorsLoading" style="text-align:center;padding:20px;color:#909399">{{ t('common.loading') }}</div>
        <div v-for="c in collaborators" :key="c.id" class="collaborator-item">
          <div class="collab-left">
            <div class="collab-avatar" :style="{ background: c.target_type === 'department' ? '#e6f7ff' : '#f0f0f0' }">
              <span v-if="c.target_type === 'department'"><svg style="width:16px;height:16px;vertical-align:-2px" viewBox="0 0 20 20" fill="currentColor"><path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z"/></svg></span>
              <span v-else-if="c.target_name">{{ c.target_name.charAt(0) }}</span>
              <span v-else><svg style="width:16px;height:16px;vertical-align:-2px" viewBox="0 0 20 20" fill="currentColor"><path d="M10 10a4 4 0 100-8 4 4 0 000 8zm0 2c-4.418 0-8 1.79-8 4v2h16v-2c0-2.21-3.582-4-8-4z"/></svg></span>
            </div>
            <div class="collab-info">
              <div class="collab-name">
                {{ c.target_name }}
                <el-tag v-if="c.role === 'owner'" size="small" type="" effect="plain" style="margin-left:6px">{{ t('docEditor.ownerTag') }}</el-tag>
                <el-tag v-if="c.inherited" size="small" type="info" effect="plain" style="margin-left:6px">{{ t('docEditor.inheritedTag') }}</el-tag>
              </div>
              <div v-if="c.target_type === 'department'" class="collab-sub">{{ t('common.department') }}</div>
            </div>
          </div>
          <div v-if="c.role !== 'owner'" class="collab-right">
            <el-dropdown trigger="click" @command="(role: string) => updateCollaborator(c.id, role)">
              <span class="role-dropdown">
                {{ roleLabel(c.role) }}
                <el-icon><ArrowDown /></el-icon>
              </span>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="viewer" :class="{ active: c.role === 'viewer' }">{{ t('docEditor.sharePermViewer') }}</el-dropdown-item>
                  <el-dropdown-item command="editor" :class="{ active: c.role === 'editor' }">{{ t('docEditor.sharePermEditor') }}</el-dropdown-item>
                  <el-dropdown-item command="admin" :class="{ active: c.role === 'admin' }">{{ t('docEditor.sharePermAdmin') }}</el-dropdown-item>
                  <el-dropdown-item divided command="remove" style="color:#f56c6c">{{ t('common.remove') }}</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </div>
      </div>

      <el-divider />

      <!-- 链接分享 -->
      <div class="link-share-section">
        <div class="link-share-header">
          <span style="font-weight:500">{{ t('docEditor.linkShareLabel') }}</span>
          <el-switch v-model="linkShareEnabled" @change="toggleLinkShare" />
        </div>
        <div v-if="linkShareEnabled" class="link-share-body">
          <el-select v-model="shareForm.expiresIn" size="small" style="width:120px;margin-right:8px">
            <el-option :label="t('docEditor.expireNever')" :value="0" />
            <el-option :label="t('docEditor.expire24h')" :value="24" />
            <el-option :label="t('docEditor.expire7d')" :value="168" />
            <el-option :label="t('docEditor.expire30d')" :value="720" />
          </el-select>
          <el-input v-model="shareForm.password" :placeholder="t('docEditor.sharePasswordPlaceholder')" size="small" style="width:140px;margin-right:8px" />
          <el-button size="small" type="primary" @click="createShare">{{ t('docEditor.generateLink') }}</el-button>
        </div>
        <div v-if="shareResult" class="share-link-result">
          <el-input :model-value="shareResult.share_url" readonly size="small">
            <template #append>
              <el-button @click="copyShareUrl" size="small">{{ t('docEditor.copyBtn') }}</el-button>
            </template>
          </el-input>
        </div>
        <!-- 已有分享链接 -->
        <div v-if="existingShares.length > 0" class="existing-shares">
          <div v-for="s in existingShares" :key="s.id" class="share-link-item">
            <span class="share-link-token"><svg style="width:14px;height:14px;vertical-align:-2px" viewBox="0 0 20 20" fill="currentColor"><path d="M12.586 4.586a2 2 0 112.828 2.828l-3.879 3.879a2 2 0 01-2.828 0 1 1 0 00-1.414 1.414 4 4 0 005.656 0l3.879-3.879a4 4 0 00-5.656-5.656L8.12 5.464a1 1 0 001.414 1.414l3.052-3.292z"/></svg> {{ s.token }}</span>
            <span v-if="s.has_password" class="share-link-badge"><svg style="width:14px;height:14px;vertical-align:-2px" viewBox="0 0 20 20" fill="currentColor"><path d="M10 2a4 4 0 00-4 4v2H5a1 1 0 00-1 1v8a1 1 0 001 1h10a1 1 0 001-1V9a1 1 0 00-1-1h-1V6a4 4 0 00-4-4zm2 6H8V6a2 2 0 114 0v2z"/></svg></span>
            <span v-if="s.expired" class="share-link-badge expired">{{ t('docEditor.shareExpired') }}</span>
            <span class="share-link-count">{{ s.access_count }}  {{ t('docEditor.shareAccessCount', [s.access_count]) }}</span>
            <el-button link type="danger" size="small" @click="deleteShare(s.id)">{{ t('common.delete') }}</el-button>
          </div>
        </div>
      </div>

      <template #footer>
        <el-button @click="showShareDialog = false; shareResult = null">{{ t('common.finish') }}</el-button>
      </template>
    </el-dialog>

    <!-- 评论面板 -->
    <el-drawer v-model="showComments" :title="t('docEditor.commentsTitle')" size="400px">
      <div class="comment-input">
        <div style="position:relative">
          <el-input v-model="newComment" type="textarea" :rows="3" :placeholder="t('docEditor.commentPlaceholder')" @input="onCommentInput" />
          <div v-if="mentionList.length" class="mention-dropdown">
            <div v-for="u in mentionList" :key="u.id" class="mention-item" @click="selectMention(u)">
              {{ u.name }} ({{ u.username }})
            </div>
          </div>
        </div>
        <el-button type="primary" size="small" @click="submitComment" :disabled="!newComment.trim()" style="margin-top:8px">{{ t('docEditor.sendBtn') }}</el-button>
      </div>
      <div class="comment-list">
        <div v-for="c in comments" :key="c.id" class="comment-item">
          <div class="comment-header">
            <strong>{{ c.user_name }}</strong>
            <span class="comment-time">{{ formatTime(c.created_at) }}</span>
          </div>
          <div class="comment-content">{{ c.content }}</div>
          <div class="comment-actions">
            <el-button link size="small" @click="replyTo(c)">{{ t('docEditor.replyBtn') }}</el-button>
            <el-button v-if="c.user_id === currentUserId" link type="danger" size="small" @click="deleteComment(c.id)">{{ t('common.delete') }}</el-button>
          </div>
          <!-- 回复 -->
          <div v-for="r in getReplies(c.id)" :key="r.id" class="comment-reply">
            <div class="comment-header">
              <strong>{{ r.user_name }}</strong>
              <span class="comment-time">{{ formatTime(r.created_at) }}</span>
            </div>
            <div class="comment-content">{{ r.content }}</div>
            <div class="comment-actions">
              <el-button v-if="r.user_id === currentUserId" link type="danger" size="small" @click="deleteComment(r.id)">{{ t('common.delete') }}</el-button>
            </div>
          </div>
        </div>
        <div v-if="!comments.length" class="no-data">{{ t('docEditor.noComments') }}</div>
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, watch, nextTick, defineAsyncComponent } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Editor, EditorContent } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import Underline from '@tiptap/extension-underline'
import TaskList from '@tiptap/extension-task-list'
import TaskItem from '@tiptap/extension-task-item'
import Placeholder from '@tiptap/extension-placeholder'
import Image from '@tiptap/extension-image'
import Link from '@tiptap/extension-link'
import { Table } from '@tiptap/extension-table'
import { TableRow } from '@tiptap/extension-table-row'
import { TableCell } from '@tiptap/extension-table-cell'
import { TableHeader } from '@tiptap/extension-table-header'
import CodeBlockLowlight from '@tiptap/extension-code-block-lowlight'
import Collaboration from '@tiptap/extension-collaboration'
import CollaborationCursor from '@tiptap/extension-collaboration-cursor'
import { common, createLowlight } from 'lowlight'
import 'highlight.js/styles/github-dark.min.css'
import * as Y from 'yjs'
import { MistWSProvider, type CollabUser } from '@/utils/collab'
import http from '@/utils/http'
import teamApi from '@/utils/team-api'
const SheetEditor = defineAsyncComponent(() => import('@/components/SheetEditor.vue'))

const lowlight = createLowlight(common)

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const { t } = useI18n()
const docId = route.params.id as string

const doc = ref<any>(null)
const title = ref('')
const versions = ref<any[]>([])
const editor = ref<Editor | null>(null)
const sheetData = ref('{}')
const sheetRef = ref<InstanceType<typeof SheetEditor> | null>(null)
const imageInput = ref<HTMLInputElement | null>(null)
const saving = ref(false)
const saveStatus = ref('') // '', 'saving', 'saved', 'error'
let saveTimer: any = null
let autoSaveTimer: any = null
let dataLoaded = false // 防止加载数据前自动保存空内容

// 分享 & 协作
const showShareDialog = ref(false)
const _watermarkToggle = ref(false)
const isAdmin = computed(() => auth.isAdmin)
// 普通用户强制水印，管理员可手动开关
const watermarkOn = computed(() => !isAdmin.value || _watermarkToggle.value)
function toggleWatermark() { _watermarkToggle.value = !_watermarkToggle.value }
const shareForm = reactive({ password: '', expiresIn: 0 })
const shareResult = ref<any>(null)
const existingShares = ref<any[]>([])
const linkShareEnabled = ref(false)

// 协作者
const collaborators = ref<any[]>([])
const collaboratorsLoading = ref(false)
const targetSearch = ref('')
const selectedTarget = ref<any>(null)
const newRole = ref('viewer')

// 评论
const showComments = ref(false)
const comments = ref<any[]>([])
const newComment = ref('')
const mentionList = ref<any[]>([])
const allUsers = ref<any[]>([])

// 加载用户列表用于 @提及
async function loadMentionUsers() {
  if (allUsers.value.length) return
  try {
    const { data } = await teamApi.get('/search-targets', { params: { q: '' } })
    allUsers.value = data.data || []
  } catch {}
}

function onCommentInput() {
  const text = newComment.value
  const atIdx = text.lastIndexOf('@')
  if (atIdx < 0) { mentionList.value = []; return }
  const query = text.slice(atIdx + 1).toLowerCase()
  if (query.includes(' ') || query.includes('\n')) { mentionList.value = []; return }
  mentionList.value = allUsers.value.filter((u: any) =>
    u.name?.toLowerCase().includes(query) || u.username?.toLowerCase().includes(query)
  ).slice(0, 5)
  loadMentionUsers()
}

function selectMention(u: any) {
  const text = newComment.value
  const atIdx = text.lastIndexOf('@')
  newComment.value = text.slice(0, atIdx) + '@' + u.name + ' '
  mentionList.value = []
}
const commentCount = ref(0)
const outlineItems = ref<{text: string; level: number; id: string}[]>([])
const outlineCollapsed = ref(false)
const outlineActiveId = ref('')

// 提取大纲
function updateOutline() {
  if (!editor.value) return
  const items: {text: string; level: number; id: string}[] = []
  const doc = editor.value.state.doc
  doc.descendants((node: any, pos: number) => {
    if (node.type.name === 'heading') {
      const id = 'heading-' + pos
      items.push({ text: node.textContent, level: node.attrs.level, id })
    }
  })
  outlineItems.value = items
}

// 滚动时高亮当前标题
function onEditorScroll() {
  if (!outlineItems.value.length) return
  const container = document.querySelector('.tiptap-editor')
  if (!container) return
  const headings = container.querySelectorAll('h1, h2, h3, h4, h5, h6')
  let activeId = ''
  for (const h of headings) {
    const rect = h.getBoundingClientRect()
    const containerRect = container.getBoundingClientRect()
    if (rect.top - containerRect.top <= 60) {
      activeId = outlineItems.value.find(o => o.text === h.textContent)?.id || ''
    }
  }
  outlineActiveId.value = activeId
}

function scrollToHeading(id: string) {
  const el = document.querySelector(`[data-heading="${id}"]`) ||
    [...document.querySelectorAll('.tiptap-editor h1, .tiptap-editor h2, .tiptap-editor h3')]
      .find(el => el.textContent === outlineItems.value.find(o => o.id === id)?.text)
  if (el) el.scrollIntoView({ behavior: 'smooth', block: 'center' })
}
const replyParent = ref('')
const currentUserId = ref('')
const currentUser = ref('')

// 协同编辑
const collabStatus = ref<'connecting' | 'connected' | 'disconnected'>('disconnected')
const collabUsers = ref<CollabUser[]>([])
const remoteCursors = ref<any[]>([])
let ydoc: Y.Doc | null = null
let wsProvider: MistWSProvider | null = null

// 移动文档
const showMoveDialog = ref(false)
const moveTarget = ref('')
const folderTree = ref<any[]>([])

// 标签
const docTags = ref<any[]>([])
const allTags = ref<any[]>([])
const docTagIds = ref<string[]>([])
const showTagDialog = ref(false)
const newTagName = ref('')
const newTagColor = ref('#409eff')

async function loadDocTags() {
  try {
    const { data } = await teamApi.get(`/documents/${docId}/tags`)
    docTags.value = data || []
    docTagIds.value = docTags.value.map((t: any) => t.id)
  } catch {}
}

async function loadAllTags() {
  try {
    const { data } = await teamApi.get('/tags')
    allTags.value = data || []
  } catch {}
}

async function addTag(tagId: string) {
  if (docTagIds.value.includes(tagId)) return
  const newIds = [...docTagIds.value, tagId]
  await teamApi.put(`/documents/${docId}/tags`, { tag_ids: newIds })
  await loadDocTags()
}

async function removeTag(tagId: string) {
  const newIds = docTagIds.value.filter(id => id !== tagId)
  await teamApi.put(`/documents/${docId}/tags`, { tag_ids: newIds })
  await loadDocTags()
}

async function createAndAddTag() {
  if (!newTagName.value.trim()) return
  try {
    await teamApi.post('/tags', { name: newTagName.value, color: newTagColor.value })
    await loadAllTags()
    // Find the new tag and add it
    const created = allTags.value.find((t: any) => t.name === newTagName.value.trim())
    if (created) await addTag(created.id)
    newTagName.value = ''
  } catch {}
}

watch(showTagDialog, (v) => { if (v) loadAllTags() })

// 文档统计
const showStats = ref(false)
const stats = ref<any>(null)

const showExportDialog = ref(false)

async function loadAndShowStats() {
  try {
    const res = await teamApi.get(`/documents/${docId}/stats`)
    stats.value = res.data?.data || res.data
    showStats.value = true
  } catch {}
}

// 版本对比
const showDiff = ref(false)
const diffOld = ref(0)
const diffNew = ref(0)
const diffHtml = ref('')
const diffLoading = ref(false)
const windowWidth = ref(window.innerWidth)

// 锁定
async function lockDoc() {
  try {
    await teamApi.post(`/documents/${docId}/lock`)
    if (doc.value) doc.value.locked_by = currentUserId.value
    ElMessage.success(t('docEditor.lockSuccess'))
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.error || t('docEditor.lockFailed'))
  }
}

async function unlockDoc() {
  try {
    await teamApi.post(`/documents/${docId}/unlock`)
    if (doc.value) doc.value.locked_by = ''
    ElMessage.success(t('docEditor.unlockSuccess'))
  } catch { ElMessage.error(t('docEditor.unlockFailed')) }
}

function openDiff(ver: number) {
  diffOld.value = ver
  diffNew.value = doc.value?.version || ver
  showDiff.value = true
  loadDiff()
}

async function loadDiff() {
  if (!diffOld.value || !diffNew.value) return
  diffLoading.value = true
  try {
    const [oldResp, newResp] = await Promise.all([
      fetch(`/api/teams/${auth.currentTeamId}/documents/${docId}/versions/${diffOld.value}/content`, { headers: authHeader() }),
      fetch(`/api/teams/${auth.currentTeamId}/documents/${docId}/versions/${diffNew.value}/content`, { headers: authHeader() }),
    ])
    let oldText = await oldResp.text()
    let newText = await newResp.text()
    // Check for API error responses
    const checkError = (text: string, label: string): string | null => {
      try {
        const obj = JSON.parse(text)
        if (obj.error) return `${label}: ${obj.error}`
      } catch {}
      return null
    }
    const oldErr = checkError(oldText, t('docEditor.diffVersion') + '(old)')
    const newErr = checkError(newText, t('docEditor.diffVersion') + '(new)')
    if (oldErr || newErr) {
      diffHtml.value = '<div style="font-size:14px;line-height:1.8">' +
        (oldErr ? '<p style="color:#f56c6c"><svg style="width:16px;height:16px;vertical-align:-2px" viewBox="0 0 20 20" fill="currentColor"><path d="M10 10a4 4 0 100-8 4 4 0 000 8zm0 2c-4.418 0-8 1.79-8 4v2h16v-2c0-2.21-3.582-4-8-4z"/></svg> ' + oldErr + '</p>' : '') +
        (newErr ? '<p style="color:#f56c6c"><svg style="width:16px;height:16px;vertical-align:-2px" viewBox="0 0 20 20" fill="currentColor"><path d="M10 10a4 4 0 100-8 4 4 0 000 8zm0 2c-4.418 0-8 1.79-8 4v2h16v-2c0-2.21-3.582-4-8-4z"/></svg> ' + newErr + '</p>' : '') +
        t('docEditor.oldVersionDecryptHint') ? '<p style="color:#909399">' + t('docEditor.oldVersionDecryptHint') + '</p>' : '' +
        '</div>'
    } else {
      diffHtml.value = simpleDiff(oldText, newText)
    }
  } catch { diffHtml.value = '<p style="color:#f56c6c">' + t('docEditor.loadFailed') + '</p>' }
  diffLoading.value = false
}

function authHeader(): Record<string, string> {
  const token = localStorage.getItem('token')
  return token ? { Authorization: `Bearer ${token}` } : {}
}

function simpleDiff(oldHtml: string, newHtml: string): string {
  // 检测是否为 JSON（sheet 类型）
  const isJson = (s: string) => { try { const p = JSON.parse(s); return Array.isArray(p) } catch { return false } }
  if (isJson(oldHtml) && isJson(newHtml)) {
    return sheetDiff(oldHtml, newHtml)
  }
  const stripTags = (s: string) => s.replace(/<[^>]+>/g, '').split(/\s+/).filter(Boolean)
  const oldWords = stripTags(oldHtml)
  const newWords = stripTags(newHtml)
  const oldSet = new Set(oldWords)
  const newSet = new Set(newWords)
  let html = '<div style="font-size:14px;line-height:1.8">'
  // Removed (in old but not in new)
  const removed = oldWords.filter(w => !newSet.has(w))
  const added = newWords.filter(w => !oldSet.has(w))
  if (removed.length === 0 && added.length === 0) {
    html += '<p style="color:#67c23a"><svg style="width:16px;height:16px;vertical-align:-2px" viewBox="0 0 20 20" fill="currentColor"><path d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"/></svg> ' + t('docEditor.twoVersionsSame') + '</p>'
  } else {
    if (removed.length) html += '<p><strong style="color:#f56c6c">' + t('docEditor.diffRemoved', [removed.length]) + '：</strong></p><p>' + removed.slice(0, 50).map(w => `<span style="background:#fde2e2;color:#f56c6c;padding:1px 3px;border-radius:3px">${w}</span>`).join(' ') + (removed.length > 50 ? ' ...' : '') + '</p>'
    if (added.length) html += '<p><strong style="color:#67c23a">' + t('docEditor.diffAdded', [added.length]) + '：</strong></p><p>' + added.slice(0, 50).map(w => `<span style="background:#e1f3d8;color:#67c23a;padding:1px 3px;border-radius:3px">${w}</span>`).join(' ') + (added.length > 50 ? ' ...' : '') + '</p>'
  }
  html += '</div>'
  return html
}

function sheetDiff(oldJson: string, newJson: string): string {
  try {
    const oldSheets: any[] = JSON.parse(oldJson)
    const newSheets: any[] = JSON.parse(newJson)
    let html = '<div style="font-size:14px;line-height:1.8">'
    const maxSheets = Math.max(oldSheets.length, newSheets.length)
    let hasDiff = false
    for (let si = 0; si < maxSheets; si++) {
      const os = oldSheets[si], ns = newSheets[si]
      const name = ns?.name || os?.name || `Sheet${si + 1}`
      if (!os) { html += '<p><svg style="width:16px;height:16px;vertical-align:-2px" viewBox="0 0 20 20" fill="currentColor"><path d="M4 4a2 2 0 012-2h8a2 2 0 012 2v12a2 2 0 01-2 2H6a2 2 0 01-2-2V4z"/></svg> <strong>' + name + '</strong>: <span style="color:#67c23a">' + t('docEditor.newSheetAdded') + '</span></p>'; hasDiff = true; continue }
      if (!ns) { html += '<p><svg style="width:16px;height:16px;vertical-align:-2px" viewBox="0 0 20 20" fill="currentColor"><path d="M4 4a2 2 0 012-2h8a2 2 0 012 2v12a2 2 0 01-2 2H6a2 2 0 01-2-2V4z"/></svg> <strong>' + name + '</strong>: <span style="color:#f56c6c">' + t('docEditor.sheetDeleted') + '</span></p>'; hasDiff = true; continue }
      const oldRows = os.rows || [], newRows = ns.rows || []
      const maxR = Math.max(oldRows.length, newRows.length)
      for (let ri = 0; ri < maxR; ri++) {
        const or_ = oldRows[ri], nr_ = newRows[ri]
        const maxC = Math.max(or_?.cells?.length || 0, nr_?.cells?.length || 0)
        for (let ci = 0; ci < maxC; ci++) {
          const oc = or_?.cells?.[ci], nc = nr_?.cells?.[ci]
          const ov = oc?.value ?? oc?.formula ?? '', nv = nc?.value ?? nc?.formula ?? ''
          if (ov !== nv) {
            hasDiff = true
            const colName = String.fromCharCode(65 + ci)
            const cellRef = `${colName}${ri + 1}`
            html += '<p><svg style="width:16px;height:16px;vertical-align:-2px" viewBox="0 0 20 20" fill="currentColor"><path d="M4 4a2 2 0 012-2h8a2 2 0 012 2v12a2 2 0 01-2 2H6a2 2 0 01-2-2V4z"/></svg> ' + name + ' ' + cellRef + ': '
            if (ov) html += `<span style="background:#fde2e2;color:#f56c6c;padding:1px 3px;border-radius:3px">${String(ov).substring(0, 50)}</span>`
            html += ' → '
            if (nv) html += `<span style="background:#e1f3d8;color:#67c23a;padding:1px 3px;border-radius:3px">${String(nv).substring(0, 50)}</span>`
            if (!ov) html += '<span style="color:#67c23a">' + t('docEditor.diffNew') + '</span>'
            if (!nv) html += '<span style="color:#f56c6c">' + t('common.delete') + '</span>'
            html += '</p>'
          }
        }
      }
    }
    if (!hasDiff) html += '<p style="color:#67c23a"><svg style="width:16px;height:16px;vertical-align:-2px" viewBox="0 0 20 20" fill="currentColor"><path d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"/></svg> ' + t('docEditor.twoVersionsSame') + '</p>'
    html += '</div>'
    return html
  } catch {
    return '<p style="color:#f56c6c">' + t('docEditor.dataParseFailed') + '</p>'
  }
}

// 链接弹窗
const linkDialog = reactive({ show: false, text: '', url: '', results: [] as {id: string; title: string; type: string}[] })
let linkSearchTimer: ReturnType<typeof setTimeout> | null = null

function searchDocsForLink() {
  clearTimeout(linkSearchTimer!)
  const q = linkDialog.url.trim()
  if (!q || q.startsWith('http://') || q.startsWith('https://') || q.startsWith('/')) {
    linkDialog.results = []
    return
  }
  linkSearchTimer = setTimeout(async () => {
    try {
      const { data } = await teamApi.get('/documents/search', { params: { q } })
      linkDialog.results = (data.data || []).slice(0, 5).map((d: any) => ({ id: d.id, title: d.title, type: d.type }))
    } catch { linkDialog.results = [] }
  }, 300)
}

function selectDocLink(d: {id: string; title: string; type: string}) {
  linkDialog.url = `${window.location.origin}/docs/${d.id}`
  if (!linkDialog.text) linkDialog.text = d.title
  linkDialog.results = []
}
// 代码语言弹窗
const codeLangDialog = reactive({ show: false, lang: 'plaintext' })
// 版本回退弹窗
const versionDialog = reactive({ show: false, version: 0, loading: false })
const popularLangs = ['plaintext', 'javascript', 'typescript', 'python', 'go', 'java', 'bash', 'sql', 'html', 'css', 'json', 'yaml', 'markdown']
const otherLangs = ['c', 'cpp', 'csharp', 'rust', 'ruby', 'php', 'swift', 'kotlin', 'scala', 'lua', 'perl', 'r', 'dockerfile', 'nginx', 'xml', 'diff']

// 媒体库
const showMediaLib = ref(false)
const mediaFilter = ref('')
const mediaItems = ref<{name: string; url: string; size: number; type: string}[]>([])
const mediaLoading = ref(false)

async function loadMedia() {
  mediaLoading.value = true
  try {
    const params: any = { limit: 50 }
    if (mediaFilter.value) params.type = mediaFilter.value
    const { data } = await teamApi.get('/media', { params })
    mediaItems.value = data.data || []
  } catch { mediaItems.value = [] }
  mediaLoading.value = false
}

function insertMedia(item: {name: string; url: string; size: number; type: string}) {
  if (item.type === 'image') {
    editor.value?.chain().focus().setImage({ src: item.url }).run()
  } else {
    editor.value?.chain().focus().setLink({ href: item.url }).run()
  }
  showMediaLib.value = false
  ElMessage.success(t('common.inserted'))
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
}

watch(showMediaLib, (v) => { if (v) loadMedia() })

// Comment real-time polling
let commentPollTimer: ReturnType<typeof setInterval> | null = null
watch(showComments, (v) => {
  if (v) {
    loadComments()
    commentPollTimer = setInterval(loadComments, 10000)
  } else {
    if (commentPollTimer) { clearInterval(commentPollTimer); commentPollTimer = null }
  }
})

async function loadDoc() {
  const { data } = await teamApi.get(`/documents/${docId}/content`)
  doc.value = data.data
  title.value = doc.value?.title || ''
  if (doc.value?.type === 'sheet') {
    sheetData.value = doc.value?.content || '{}'
  }
  // 数据加载完成，允许自动保存
  nextTick(() => { dataLoaded = true })
}

// === 文档移动 ===
async function loadFolderTree() {
  try {
    const { data } = await teamApi.get('/folders/tree')
    folderTree.value = data || []
  } catch {}
}

async function moveDoc() {
  if (!moveTarget.value) return
  try {
    await teamApi.put(`/documents/${docId}`, { title: title.value, folder_id: moveTarget.value })
    ElMessage.success(t('docEditor.moveSuccess'))
    showMoveDialog.value = false
    if (doc.value) doc.value.folder_id = moveTarget.value
  } catch { ElMessage.error(t('docEditor.moveFailed')) }
}

watch(showMoveDialog, (v) => { if (v) loadFolderTree() })

async function loadVersions() {
  const { data } = await teamApi.get(`/documents/${docId}/versions`)
  versions.value = (data.data || [])
}

function initEditor(initialContent: string) {
  // Try to use Yjs collaboration
  const token = localStorage.getItem('token')
  const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${wsProtocol}//${window.location.host}/ws/docs/${docId}?token=${token}`

  // Use collab mode for doc type when authenticated
  const useCollab = !!token && doc.value?.type === 'doc'

  if (useCollab) {
    try {
      ydoc = new Y.Doc()

      // Setup WS provider
      wsProvider = new MistWSProvider(wsUrl, ydoc)
      wsProvider.onStatus = (status) => { collabStatus.value = status }
      wsProvider.onUserJoin = (user) => { collabUsers.value = [...collabUsers.value, user] }
      wsProvider.onUserLeave = (userId) => {
        collabUsers.value = collabUsers.value.filter(u => u.id !== userId)
        remoteCursors.value = remoteCursors.value.filter(c => c.userId !== userId)
      }
      wsProvider.onAwareness = (data: any) => {
        // Handle incoming cursor/selection awareness from other users
        if (data?.userId && data.userId !== currentUserId.value) {
          const existing = remoteCursors.value.find(c => c.userId === data.userId)
          if (existing) {
            Object.assign(existing, data)
          } else {
            remoteCursors.value.push(data)
          }
        }
      }
      wsProvider.onClients = (users) => { collabUsers.value = users.filter((u: CollabUser) => u.id !== currentUserId.value) }
      wsProvider.bind()

      const userColors = ['#e06c75', '#e5c07b', '#98c379', '#56b6c2', '#61afef', '#c678dd', '#d19a66']
      const userColorIdx = currentUserId.value.split('').reduce((a, c) => a + c.charCodeAt(0), 0) % userColors.length

      editor.value = new Editor({
        extensions: [
          StarterKit.configure({
            codeBlock: false,
          }),
          Underline,
          TaskList,
          TaskItem.configure({ nested: true }),
          Placeholder.configure({ placeholder: t('docEditor.placeholder') }),
          Image.configure({ inline: false, allowBase64: true, HTMLAttributes: { class: 'editor-image' } }),
          Link.configure({ openOnClick: false, HTMLAttributes: { class: 'editor-link', target: '_blank', rel: 'noopener' } }),
          Table.configure({ resizable: true }),
          TableRow,
          TableCell,
          TableHeader,
          CodeBlockLowlight.configure({ lowlight }),
          Collaboration.configure({
            document: ydoc,
          }),
          CollaborationCursor.configure({
            provider: { awareness: null } as any,
            user: {
              name: auth.user?.display_name || auth.user?.username || t('docEditor.collabAnonymous'),
              color: userColors[userColorIdx],
              fallbackColor: userColors[userColorIdx],
            },
          }),
        ],
        editorProps: {
          attributes: { class: 'prose prose-lg focus:outline-none max-w-none' },
          handlePaste: (_view: any, event: ClipboardEvent) => {
            const items = event.clipboardData?.items
            if (items) {
              for (let i = 0; i < items.length; i++) {
                if (items[i].type.indexOf('image') >= 0) {
                  event.preventDefault()
                  const file = items[i].getAsFile()
                  if (file) uploadImageFile(file)
                  return true
                }
              }
            }
            return false
          },
          handleDrop: (_view: any, event: DragEvent) => {
            const files = event.dataTransfer?.files
            if (files) {
              for (let i = 0; i < files.length; i++) {
                if (files[i].type.indexOf('image') >= 0) {
                  event.preventDefault()
                  uploadImageFile(files[i])
                  return true
                }
              }
            }
            return false
          },
        },
        onUpdate: () => {
          scheduleAutoSave()
          updateOutline()
        },
        onSelectionUpdate: ({ editor: ed }) => {
          // Broadcast cursor position via awareness
          if (!wsProvider || !ed) return
          const { from, to } = ed.state.selection
          wsProvider.sendAwareness({
            userId: currentUserId.value,
            userName: auth.user?.display_name || t('docEditor.collabAnonymous'),
            color: userColors[userColorIdx],
            cursor: { from, to },
            timestamp: Date.now(),
          })
        },
      })

      // Key fix: after sync, if Yjs has no content but we have HTML,
      // populate the editor with HTML. This seeds the Y.Doc via Collaboration.
      const syncHandler = (synced: boolean) => {
        if (!synced) return
        nextTick(() => {
          const yXmlFragment = ydoc!.getXmlFragment('default')
          // Check if Yjs is truly empty — just check length
          if (yXmlFragment.length === 0 && initialContent && initialContent !== '{}') {
            console.log('[Collab] Yjs empty after sync, seeding with HTML content')
            editor.value?.commands.setContent(initialContent)
          }
          wsProvider!.onSynced = null
        })
      }
      // If already synced (fast path), check immediately.
      // Otherwise, wait for sync callback.
      if (wsProvider.isSynced()) {
        syncHandler(true)
      } else {
        wsProvider.onSynced = syncHandler
      }

      return
    } catch (e) {
      console.warn('Collab mode failed, falling back to local:', e)
      ydoc?.destroy()
      ydoc = null
      wsProvider?.destroy()
      wsProvider = null
    }
  }

  // Fallback: local mode (no collab)
  editor.value = new Editor({
    content: initialContent || '',
    extensions: [
      StarterKit.configure({ codeBlock: false }),
      Underline,
      TaskList,
      TaskItem.configure({ nested: true }),
      Placeholder.configure({ placeholder: t('docEditor.placeholder') }),
      Image.configure({ inline: false, allowBase64: true, HTMLAttributes: { class: 'editor-image' } }),
      Link.configure({ openOnClick: false, HTMLAttributes: { class: 'editor-link', target: '_blank', rel: 'noopener' } }),
      Table.configure({ resizable: true }),
      TableRow,
      TableCell,
      TableHeader,
      CodeBlockLowlight.configure({ lowlight }),
    ],
    editorProps: {
      attributes: { class: 'prose prose-lg focus:outline-none max-w-none' },
      handlePaste: (_view: any, event: ClipboardEvent) => {
        const items = event.clipboardData?.items
        if (items) {
          for (let i = 0; i < items.length; i++) {
            if (items[i].type.indexOf('image') >= 0) {
              event.preventDefault()
              const file = items[i].getAsFile()
              if (file) uploadImageFile(file)
              return true
            }
          }
        }
        return false
      },
      handleDrop: (_view: any, event: DragEvent) => {
        const files = event.dataTransfer?.files
        if (files) {
          for (let i = 0; i < files.length; i++) {
            if (files[i].type.indexOf('image') >= 0) {
              event.preventDefault()
              uploadImageFile(files[i])
              return true
            }
          }
        }
        return false
      },
    },
    onUpdate: () => {
      scheduleAutoSave()
      updateOutline()
    },
  })
}

// 图片上传
function triggerImageUpload() { imageInput.value?.click() }

async function handleImageUpload(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  await uploadImageFile(file)
  ;(e.target as HTMLInputElement).value = ''
}

async function uploadImageFile(file: File) {
  try {
    const formData = new FormData()
    formData.append('file', file)
    const { data } = await teamApi.post('/upload', formData, { headers: { 'Content-Type': 'multipart/form-data' } })
    const url = data.data?.url || data.data?.path || data.data
    editor.value?.chain().focus().setImage({ src: url }).run()
    ElMessage.success(t('docEditor.imageUploaded'))
  } catch {
    const reader = new FileReader()
    reader.onload = () => { editor.value?.chain().focus().setImage({ src: reader.result as string }).run() }
    reader.readAsDataURL(file)
  }
}

// 链接
function insertLink() {
  linkDialog.text = editor.value?.state.selection.content()?.content.firstChild?.text || ''
  linkDialog.url = editor.value?.getAttributes('link').href || ''
  linkDialog.show = true
}
function confirmLink() {
  if (!linkDialog.url) { linkDialog.show = false; return }
  editor.value?.chain().focus().extendMarkRange('link').setLink({ href: linkDialog.url }).run()
  linkDialog.show = false
}
function removeLink() {
  editor.value?.chain().focus().unsetLink().run()
  linkDialog.show = false
}

// Handle clicks on links inside editor — navigate to internal doc links
function handleEditorClick(e: MouseEvent) {
  const target = (e.target as HTMLElement).closest('a.editor-link, .tiptap a[href]')
  if (!target) return
  const href = (target as HTMLAnchorElement).href
  if (!href) return
  // Internal doc link: /docs/:id
  const match = href.match(/\/docs\/([a-f0-9-]+)$/)
  if (match) {
    e.preventDefault()
    router.push(`/docs/${match[1]}`)
  }
}

// 表格
function insertTable() { editor.value?.chain().focus().insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run() }

// 代码块
function toggleCodeBlock() {
  if (editor.value?.isActive('codeBlock')) {
    editor.value.chain().focus().toggleCodeBlock().run()
  } else {
    codeLangDialog.lang = 'plaintext'
    codeLangDialog.show = true
  }
}
function confirmCodeLang() {
  editor.value?.chain().focus().toggleCodeBlock({ language: codeLangDialog.lang }).run()
  codeLangDialog.show = false
}

// 保存
function scheduleAutoSave() {
  if (!dataLoaded) return // 数据未加载完不保存
  clearTimeout(autoSaveTimer)
  autoSaveTimer = setTimeout(doSave, 1500)
}

async function doSave() {
  if (!dataLoaded) { console.warn('[SAVE] blocked: dataLoaded=false'); return }
  let content = ''
  if (doc.value?.type === 'sheet') {
    const refVal = sheetRef.value
    console.log('[SAVE] sheetRef.value type:', typeof refVal, 'keys:', refVal ? Object.keys(refVal) : 'null')
    console.log('[SAVE] getData exists:', typeof refVal?.getData)
    content = refVal?.getData?.() || '{}'
    console.log('[SAVE] sheet content len:', content.length, 'isEmpty:', content === '{}')
  } else if (editor.value) {
    content = editor.value.getHTML()
  }
  if (!content || content === '{}') { console.warn('[SAVE] blocked: no content or empty'); return }
  saving.value = true
  saveStatus.value = 'saving'
  try {
    await teamApi.put(`/documents/${docId}/content`, { content })
    saveStatus.value = 'saved'
    clearTimeout(saveTimer)
    saveTimer = setTimeout(() => { saveStatus.value = '' }, 3000)
  } catch (e) {
    console.error('保存失败', e)
    saveStatus.value = 'error'
  }
  saving.value = false
}

async function manualSave() {
  clearTimeout(autoSaveTimer)
  await doSave()
  ElMessage.success(t('docEditor.docSaved'))
}

async function saveTitle() {
  if (!title.value || title.value === doc.value?.title) return
  await teamApi.put(`/documents/${docId}`, { title: title.value })
  doc.value.title = title.value
}

function formatTime(t: string): string {
  if (!t) return ''
  const d = new Date(t)
  return d.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

function handleVersion(ver: number) {
  versionDialog.version = ver
  versionDialog.show = true
}

const previewDialog = reactive({ show: false, version: 0, html: '', loading: false })

async function previewVersion(ver: number) {
  previewDialog.version = ver
  previewDialog.html = ''
  previewDialog.loading = true
  previewDialog.show = true
  try {
    const resp = await fetch(`/api/teams/${auth.currentTeamId}/documents/${docId}/versions/${ver}/content`, { headers: authHeader() })
    const text = await resp.text()
    try {
      const obj = JSON.parse(text)
      if (obj.error) { ElMessage.error(obj.error); previewDialog.show = false; return }
    } catch {}
    previewDialog.html = text || '<p style="color:#999;text-align:center">' + t('common.emptyDoc') + '</p>'
  } catch (e: any) {
    ElMessage.error(t('docEditor.versionPreviewFailed'))
    previewDialog.show = false
  }
  previewDialog.loading = false
}

function selectRestoreVersion(ver: number) {
  ElMessageBox.confirm(
    t('common.restoreConfirmMsg', [ver]),
    t('common.restoreConfirm'),
    { type: 'warning', confirmButtonText: t('common.restore'), cancelButtonText: t('common.cancel') }
  ).then(async () => {
    versionDialog.loading = true
    try {
      await teamApi.post(`/documents/${docId}/restore`, { version: ver })
      ElMessage.success(t('common.restoreSuccess', [ver]))
      versionDialog.show = false
      await loadDoc()
      await loadVersions()
      if (doc.value?.type === 'doc' && editor.value) {
        const content = doc.value?.content || ''
        editor.value.commands.setContent(content === '{}' ? '' : content)
      } else if (doc.value?.type === 'sheet') {
        sheetData.value = doc.value?.content || '{}'
      }
    } catch (e: any) {
      ElMessage.error(e?.response?.data?.error || t('common.restoreFailed'))
    }
    versionDialog.loading = false
  }).catch(() => {})
}

function onSheetChange() { console.log('[SAVE] onSheetChange triggered, dataLoaded:', dataLoaded); scheduleAutoSave() }

// === 分享 & 协作 ===
const permRoleMap: any = { read: 'viewer', write: 'editor', admin: t('docEditor.permRoleMap.admin') }
function roleLabel(role: string) {
  const m: any = { viewer: t('docEditor.permRoleMap.viewer'), editor: t('docEditor.permRoleMap.editor'), admin: t('docEditor.permRoleMap.admin'), owner: t('docEditor.permRoleMap.owner') }
  return m[role] || role
}

async function loadCollaborators() {
  collaboratorsLoading.value = true
  try {
    const [collabRes, shareRes] = await Promise.all([
      teamApi.get(`/documents/${docId}/collaborators`),
      teamApi.get(`/documents/${docId}/shares`),
    ])
    collaborators.value = collabRes.data.data || []
    existingShares.value = shareRes.data.data || []
    linkShareEnabled.value = existingShares.value.length > 0
  } finally {
    collaboratorsLoading.value = false
  }
}

function searchTargets(query: string, cb: any) {
  if (!query) { cb([]); return }
  teamApi.get('/search-targets', { params: { q: query } }).then(({ data }) => {
    const items = (data.data || []).map((t: any) => ({ ...t, value: t.display }))
    cb(items)
  }).catch(() => cb([]))
}

function onTargetSelect(item: any) {
  selectedTarget.value = item
}

async function addCollaborator() {
  if (!selectedTarget.value) return
  await teamApi.post(`/documents/${docId}/collaborators`, {
    target_type: selectedTarget.value.type,
    target_id: selectedTarget.value.id,
    role: newRole.value,
  })
  ElMessage.success(t('docEditor.collabAdded'))
  selectedTarget.value = null
  targetSearch.value = ''
  loadCollaborators()
}

async function updateCollaborator(id: string, role: string) {
  if (role === 'remove') {
    await ElMessageBox.confirm(t('docEditor.removeCollabConfirm'), t('docEditor.removeConfirmTitle'))
    await teamApi.delete(`/collaborators/${id}`)
    ElMessage.success(t('docEditor.collabRemoved'))
  } else {
    await teamApi.put(`/collaborators/${id}`, { role })
    ElMessage.success(t('docEditor.collabUpdated'))
  }
  loadCollaborators()
}

function toggleLinkShare(val: boolean) {
  if (!val && existingShares.value.length > 0) {
    // 取消时删除所有分享链接
    ElMessageBox.confirm(t('docEditor.disableLinkShareConfirm'), t('common.confirm') + '').then(async () => {
      for (const s of existingShares.value) {
        await teamApi.delete(`/shares/${s.id}`)
      }
      existingShares.value = []
    }).catch(() => { linkShareEnabled.value = true })
  }
}

async function createShare() {
  const { data } = await teamApi.post(`/documents/${docId}/share`, shareForm)
  shareResult.value = data
  ElMessage.success(t('docEditor.shareLinkGenerated'))
  loadCollaborators()
}

function copyShareUrl() {
  if (!shareResult.value) return
  const url = `${window.location.origin}${shareResult.value.share_url}`
  navigator.clipboard.writeText(url)
  ElMessage.success(t('docEditor.copiedToClipboard'))
}

async function deleteShare(id: string) {
  await teamApi.delete(`/shares/${id}`)
  ElMessage.success(t('docEditor.shareDeleted'))
  loadCollaborators()
}

// === 导出 ===
function handleMore(cmd: string) {
  switch (cmd) {
    case 'share': showShareDialog.value = true; break
    case 'move': showMoveDialog.value = true; break
    case 'watermark': toggleWatermark(); break
    case 'comments': showComments.value = true; break
    case 'stats': loadAndShowStats(); break
    case 'versions':
      if (versions.value.length) handleVersion(versions.value[0].version)
      break
    case 'export':
      showExportDialog.value = true
      break
    case 'save-template':
      saveAsTemplate()
      break
  }
}

async function saveAsTemplate() {
  const html = editor.value?.getHTML() || ''
  if (!html || html === '<p></p>') { ElMessage.warning(t('docEditor.contentEmpty')); return }
  const { value: name } = await ElMessageBox.prompt(t('docEditor.templateNamePrompt'), t('docEditor.templateSaveTitle'), {
    inputValue: doc.value?.title || '',
    confirmButtonText: t('common.save'),
    cancelButtonText: t('common.cancel'),
  }).catch(() => ({ value: '' }))
  if (!name) return
  try {
    await teamApi.post('/templates', { name, type: 'doc', content: html, is_public: false })
    ElMessage.success(t('docEditor.templateSaved'))
  } catch { ElMessage.error(t('docEditor.templateSaveFailed')) }
}

async function handleExport(format: string) {
  try {
    if (format === 'pdf') {
      // PDF: 前端生成，支持中文
      const html2pdf = (await import('html2pdf.js')).default
      const editorEl = document.querySelector('.ProseMirror') as HTMLElement
      if (!editorEl) { ElMessage.error(t('docEditor.exportFailed')); return }
      // Clone and wrap for PDF
      const wrapper = document.createElement('div')
      wrapper.style.cssText = 'padding:20px;font-family:"Noto Sans SC",sans-serif;font-size:14px;line-height:1.8;color:#333'
      wrapper.innerHTML = `<h1 style="text-align:center;font-size:22px;margin-bottom:16px">${doc.value?.title || ''}</h1>` + editorEl.innerHTML
      const opt = {
        margin: [10, 10, 10, 10] as [number, number, number, number],
        filename: `${doc.value?.title || 'export'}.pdf`,
        image: { type: 'jpeg' as const, quality: 0.95 },
        html2canvas: { scale: 2, useCORS: true },
        jsPDF: { unit: 'mm', format: 'a4', orientation: 'portrait' as const },
      }
      await html2pdf().set(opt).from(wrapper).save()
      ElMessage.success(t('docEditor.pdfExportSuccess'))
      return
    }
    const token = localStorage.getItem('token')
    const resp = await fetch(`/api/teams/${auth.currentTeamId}/documents/${docId}/export?format=${format}`, {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    })
    if (!resp.ok) { ElMessage.error(t('docEditor.exportFailed')); return }
    const blob = await resp.blob()
    const cd = resp.headers.get('Content-Disposition') || ''
    const match = cd.match(/filename="?([^"]+)"?/)
    const filename = match ? match[1] : `${doc.value?.title || 'export'}.${format === 'markdown' ? 'md' : format}`
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url; a.download = filename; a.click()
    URL.revokeObjectURL(url)
    ElMessage.success(t('docEditor.exportSuccess'))
  } catch (e) { console.error(e); ElMessage.error(t('docEditor.exportFailed')) }
}

// === 评论 ===
async function loadComments() {
  const { data } = await teamApi.get(`/documents/${docId}/comments`)
  comments.value = data.data || []
  commentCount.value = comments.value.length
}

async function submitComment() {
  if (!newComment.value.trim()) return
  await teamApi.post(`/documents/${docId}/comments`, {
    content: newComment.value,
    parent_id: replyParent.value,
  })
  newComment.value = ''
  replyParent.value = ''
  loadComments()
}

function replyTo(c: any) {
  replyParent.value = c.id
  newComment.value = `@${c.user_name} `
}

async function deleteComment(id: string) {
  await teamApi.delete(`/comments/${id}`)
  loadComments()
}

function getReplies(parentId: string) {
  return comments.value.filter(c => c.parent_id === parentId)
}

onMounted(async () => {
  await loadDoc()
  await loadVersions()
  await loadDocTags()
  // Get current user ID
  try {
    const { data: me } = await http.get('/auth/me')
    currentUserId.value = me.data?.id || ''
    currentUser.value = me.data?.name || ''
  } catch {}
  // Load comment count
  loadComments()
  if (doc.value?.type === 'doc') {
    const content = doc.value?.content || ''
    initEditor(content === '{}' ? '' : content)
  }
  // Attach scroll listener for outline tracking
  nextTick(() => {
    const container = document.querySelector('.tiptap-editor')
    if (container) container.addEventListener('scroll', onEditorScroll, { passive: true })
  })
})

onUnmounted(() => {
  doSave().catch(() => {})
  clearTimeout(autoSaveTimer)
  if (commentPollTimer) clearInterval(commentPollTimer)
  wsProvider?.destroy()
  ydoc?.destroy()
  editor.value?.destroy()
  document.removeEventListener('keydown', handleGlobalKeydown)
  const container = document.querySelector('.tiptap-editor')
  if (container) container.removeEventListener('scroll', onEditorScroll)
})

function handleGlobalKeydown(e: KeyboardEvent) {
  const mod = e.ctrlKey || e.metaKey
  // Ctrl+S 保存
  if (mod && e.key === 's') {
    e.preventDefault()
    manualSave()
  }
  // Ctrl+P 导出 PDF
  if (mod && e.key === 'p') {
    e.preventDefault()
    handleExport('pdf')
  }
  // Ctrl+Shift+E 导出 HTML
  if (mod && e.shiftKey && e.key === 'E') {
    e.preventDefault()
    handleExport('html')
  }
  // Ctrl+Shift+S 分享
  if (mod && e.shiftKey && e.key === 'S') {
    e.preventDefault()
    showShareDialog.value = true
  }
  // Ctrl+K 插入链接（TipTap 已内置，这里处理无选中文本的情况）
  // Ctrl+/ 显示快捷键帮助
  if (mod && e.key === '/') {
    e.preventDefault()
    ElMessage({
      message: t('docEditor.shortcutsHelp'),
      duration: 4000,
    })
  }
}

// Register global shortcut
document.addEventListener('keydown', handleGlobalKeydown)
</script>

<style scoped>
/* ── 顶部导航栏 ── */
.editor-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 8px 16px; border-bottom: 1px solid #e8ecf0;
  background: #fff; gap: 12px; flex-shrink: 0;
}
.header-left { display: flex; align-items: center; gap: 8px; flex: 1; min-width: 0; }
.header-right { display: flex; align-items: center; gap: 6px; flex-shrink: 0; flex-wrap: wrap; }
.title-input { flex: 1; max-width: 300px; }
.title-input :deep(.el-input__wrapper) { box-shadow: none; background: transparent; font-size: 16px; font-weight: 600; }
.title-input :deep(.el-input__wrapper:hover),
.title-input :deep(.el-input__wrapper.is-focus) { box-shadow: 0 0 0 1px #dcdfe6 inset; background: #fff; }
.doc-badges { display: flex; align-items: center; gap: 4px; }
.tag-icon { width: 12px; height: 12px; margin-right: 2px; }
.btn-icon { width: 16px; height: 16px; }
.btn-label { }
.menu-icon { width: 16px; height: 16px; margin-right: 6px; vertical-align: -3px; }

.save-indicator { font-size: 12px; color: #999; white-space: nowrap; }
.save-indicator.saving { color: #e6a23c; }
.save-indicator.saved { color: #67c23a; }
.save-indicator.error { color: #f56c6c; }

/* ── 工具栏 ── */
.toolbar {
  display: flex; align-items: center; padding: 4px 12px;
  background: #fafbfc; border-bottom: 1px solid #e8ecf0;
  gap: 2px; flex-shrink: 0; flex-wrap: wrap;
}
.tb-group { display: flex; align-items: center; gap: 1px; }
.tb-sep { width: 1px; height: 20px; background: #e0e0e0; margin: 0 6px; }
.tb-btn {
  display: flex; align-items: center; justify-content: center;
  width: 30px; height: 30px; border: none; border-radius: 4px;
  background: transparent; cursor: pointer; color: #555; font-size: 12px;
  font-weight: 600; transition: all 0.15s;
}
.tb-btn svg { width: 16px; height: 16px; }
.tb-btn:hover { background: #ecf5ff; color: #409eff; }
.tb-btn.active { background: #409eff; color: #fff; }

/* ── 编辑器主体 ── */
.editor-page { display: flex; flex-direction: column; height: calc(var(--vh, 1vh) * 100); overflow: hidden; }
.editor-body { flex: 1; display: flex; overflow: hidden; }
.editor-body.with-outline { gap: 0; }
.sheet-body { flex: 1; }

.tiptap-editor { flex: 1; padding: 24px 32px; overflow-y: auto; }
.tiptap-editor :deep(.tiptap) { outline: none; max-width: 800px; margin: 0 auto; min-height: 300px; }
.tiptap-editor :deep(.tiptap p.is-editor-empty:first-child::before) {
  content: attr(data-placeholder); color: #adb5bd; pointer-events: none;
}
.tiptap-editor :deep(.tiptap h1) { font-size: 28px; font-weight: 700; margin: 24px 0 12px; }
.tiptap-editor :deep(.tiptap h2) { font-size: 22px; font-weight: 600; margin: 20px 0 10px; }
.tiptap-editor :deep(.tiptap h3) { font-size: 18px; font-weight: 600; margin: 16px 0 8px; }
.tiptap-editor :deep(.tiptap pre) { background: #1e1e2e; color: #cdd6f4; border-radius: 8px; padding: 16px; overflow-x: auto; }
.tiptap-editor :deep(.tiptap code) { background: #f0f2f5; padding: 2px 6px; border-radius: 4px; font-size: 13px; }
.tiptap-editor :deep(.tiptap blockquote) { border-left: 4px solid #409eff; padding-left: 16px; margin: 12px 0; color: #666; }
.tiptap-editor :deep(.tiptap table) { border-collapse: collapse; width: 100%; margin: 12px 0; }
.tiptap-editor :deep(.tiptap table td), .tiptap-editor :deep(.tiptap table th) {
  border: 1px solid #dcdfe6; padding: 8px 12px; min-width: 80px;
}
.tiptap-editor :deep(.tiptap table th) { background: #f5f7fa; font-weight: 600; }

/* 大纲 */
.outline-panel {
  width: 200px; border-left: 1px solid #e8ecf0;
  overflow-y: auto; flex-shrink: 0; background: #fafbfc;
  transition: width 0.2s;
}
.outline-panel.collapsed { width: 40px; }
.outline-panel.collapsed .outline-list,
.outline-panel.collapsed .outline-empty { display: none; }
.outline-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 12px; cursor: pointer; user-select: none;
}
.outline-header:hover { background: #f0f5ff; }
.outline-title { font-size: 12px; font-weight: 600; color: #909399; text-transform: uppercase; }
.outline-toggle { color: #909399; }
.outline-list { padding: 0 12px 12px; }
.outline-empty { padding: 0 12px; font-size: 12px; color: #c0c4cc; }
.outline-item { font-size: 13px; color: #666; padding: 4px 8px; cursor: pointer; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; border-radius: 4px; transition: all 0.15s; }
.outline-item:hover { color: #409eff; background: #f0f5ff; }
.outline-item.active { color: #409eff; background: #ecf5ff; font-weight: 500; }
.outline-h1 { }
.outline-h2 { padding-left: 16px; }
.outline-h3 { padding-left: 24px; }
.outline-h4 { padding-left: 32px; }

/* 水印 */
.watermark-layer {
  position: fixed; top: 0; left: 0; width: 100vw; height: calc(var(--vh, 1vh) * 100);
  pointer-events: none; z-index: 9999; display: flex; flex-wrap: wrap;
  align-items: center; justify-content: center; gap: 80px;
  transform: rotate(-25deg); opacity: 0.06;
}
.watermark-text { font-size: 16px; color: #000; white-space: nowrap; }

/* 版本弹窗 */
.diff-toolbar { display: flex; align-items: center; gap: 8px; margin-bottom: 16px; }
.diff-content { border: 1px solid #e8ecf0; border-radius: 8px; padding: 16px; max-height: 400px; overflow-y: auto; }
.diff-content :deep(.diff-add) { background: #f0f9eb; }
.diff-content :deep(.diff-remove) { background: #fef0f0; text-decoration: line-through; }

/* 版本预览 */
.preview-content {
  border: 1px solid #e8ecf0; border-radius: 8px; padding: 24px;
  max-height: 500px; overflow-y: auto; line-height: 1.8;
}
.preview-content :deep(h1) { font-size: 24px; font-weight: 700; margin: 16px 0 8px; }
.preview-content :deep(h2) { font-size: 20px; font-weight: 600; margin: 14px 0 6px; }
.preview-content :deep(h3) { font-size: 16px; font-weight: 600; margin: 12px 0 4px; }
.preview-content :deep(p) { margin: 8px 0; }
.preview-content :deep(pre) { background: #1e1e2e; color: #cdd6f4; border-radius: 8px; padding: 12px; }
.preview-content :deep(code) { background: #f0f2f5; padding: 2px 4px; border-radius: 3px; }

/* 链接搜索 */
.link-search-results { border: 1px solid #e8ecf0; border-radius: 8px; max-height: 200px; overflow-y: auto; margin-top: 4px; }
.link-search-item { display: flex; align-items: center; justify-content: space-between; padding: 8px 12px; cursor: pointer; transition: background 0.15s; }
.link-search-item:hover { background: #f0f5ff; }
.link-doc-title { font-size: 14px; color: #303133; }
.link-doc-type { font-size: 12px; color: #909399; }

/* 媒体库 */
.media-toolbar { margin-bottom: 12px; }
.media-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(120px, 1fr)); gap: 12px; max-height: 400px; overflow-y: auto; }
.media-item { border: 1px solid #e8ecf0; border-radius: 8px; padding: 8px; cursor: pointer; transition: all 0.15s; text-align: center; }
.media-item:hover { border-color: #409eff; background: #f0f5ff; }
.media-preview { width: 100%; height: 80px; display: flex; align-items: center; justify-content: center; overflow: hidden; border-radius: 4px; background: #f5f7fa; margin-bottom: 6px; }
.media-preview img { max-width: 100%; max-height: 100%; object-fit: cover; }
.media-file-icon { font-size: 32px; }
.media-name { font-size: 12px; color: #303133; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.media-size { font-size: 11px; color: #909399; margin-top: 2px; }

/* 协作头像 */
.collab-avatars { display: inline-flex; margin-left: 6px; }
.collab-avatar { display: inline-flex; align-items: center; justify-content: center; width: 22px; height: 22px; border-radius: 50%; color: #fff; font-size: 11px; font-weight: 600; margin-left: -4px; border: 2px solid #fff; cursor: default; }

/* 协作光标 */
.collab-cursor-flag { position: absolute; pointer-events: none; z-index: 100; }
.collab-cursor-flag .flag-name { font-size: 11px; padding: 1px 6px; border-radius: 3px 3px 3px 0; color: #fff; white-space: nowrap; }
.collab-cursor-flag .flag-line { width: 2px; height: 20px; }

/* 统计 */
.stats-section { display: flex; flex-direction: column; gap: 16px; }
.stats-subsection { border-top: 1px solid #f0f0f0; padding-top: 12px; }
.stats-subtitle { font-size: 13px; color: #909399; margin-bottom: 8px; font-weight: 500; }
.stats-structure { display: flex; flex-wrap: wrap; gap: 8px; font-size: 13px; color: #606266; }
.stats-structure span { background: #f5f7fa; padding: 4px 10px; border-radius: 12px; }
.stats-contributors { display: flex; flex-wrap: wrap; gap: 4px; }
.stats-meta { display: flex; flex-wrap: wrap; gap: 16px; font-size: 13px; color: #606266; }
.stats-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 16px; text-align: center; }
.stat-item { padding: 12px 0; border-radius: 8px; background: #f8fafc; }
.stat-value { font-size: 22px; font-weight: 700; color: #303133; }
.stat-label { font-size: 12px; color: #909399; margin-top: 4px; }
.activity-chart { display: flex; align-items: flex-end; gap: 2px; height: 60px; }
.activity-bar { flex: 1; min-width: 4px; background: #409eff; border-radius: 2px 2px 0 0; transition: height 0.2s; }
.hourly-chart { display: grid; grid-template-columns: repeat(24, 1fr); gap: 2px; }
.hourly-cell { text-align: center; padding: 4px 0; font-size: 10px; border-radius: 3px; background: #e6f0ff; color: #909399; cursor: default; }
.hourly-cell.active { background: #409eff; color: #fff; }
.hourly-labels { display: flex; justify-content: space-between; font-size: 10px; color: #c0c4cc; margin-top: 2px; }
.stat-item { background: #f5f7fa; border-radius: 8px; padding: 16px; }
.stat-value { font-size: 24px; font-weight: 700; color: #303133; }
.stat-label { font-size: 13px; color: #909399; margin-top: 4px; }
.activity-chart { display: flex; align-items: flex-end; gap: 3px; height: 80px; }
.activity-bar { flex: 1; background: #409eff; border-radius: 2px 2px 0 0; min-height: 2px; }

/* 评论 */
.comment-input { margin-bottom: 16px; }
.comment-list { display: flex; flex-direction: column; gap: 12px; }
.comment-item { padding: 12px 0; border-bottom: 1px solid #f0f2f5; }
.comment-item:last-child { border-bottom: none; }
.comment-header { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
.comment-header strong { font-size: 14px; }
.comment-time { font-size: 12px; color: #999; }
.comment-content { font-size: 14px; color: #333; line-height: 1.6; }
.comment-actions { margin-top: 4px; display: flex; gap: 8px; }
.comment-reply { margin-left: 24px; padding-top: 8px; border-left: 2px solid #f0f2f5; padding-left: 12px; }
.mention-dropdown { position: absolute; top: 100%; left: 0; right: 0; background: #fff; border: 1px solid #e8ecf0; border-radius: 6px; box-shadow: 0 4px 12px rgba(0,0,0,0.1); max-height: 160px; overflow-y: auto; z-index: 10; }
.mention-item { padding: 6px 12px; cursor: pointer; font-size: 13px; }
.mention-item:hover { background: #f5f7fa; }
.no-data { text-align: center; color: #c0c4cc; padding: 24px; }

/* 标签栏 */
.doc-tags-bar { padding: 8px 16px; border-bottom: 1px solid #e8ecf0; display: flex; align-items: center; flex-wrap: wrap; gap: 4px; }
.tag-disabled { opacity: 0.4; }

/* 移动端 */
@media (max-width: 768px) {
  .editor-header { flex-wrap: wrap; padding: 8px; gap: 4px; }
  .header-left { flex: 1 1 100%; }
  .header-right { width: 100%; justify-content: flex-end; flex-wrap: wrap; gap: 4px; }
  .title-input { max-width: none; font-size: 16px; }
  .doc-badges { display: none; }
  .btn-label { display: none; }
  .toolbar { padding: 2px 4px; overflow-x: auto; flex-wrap: nowrap; -webkit-overflow-scrolling: touch; }
  .toolbar::-webkit-scrollbar { display: none; }
  .outline-panel { display: none; }
  .outline-panel.collapsed { display: none; }
  .tiptap-editor { padding: 12px 16px; }
  .tiptap-editor :deep(.tiptap) { font-size: 15px; }
  .tiptap-editor :deep(.tiptap h1) { font-size: 22px; }
  .tiptap-editor :deep(.tiptap h2) { font-size: 18px; }
  .tiptap-editor :deep(.tiptap h3) { font-size: 16px; }
  .tiptap-editor :deep(.tiptap pre) { font-size: 12px; padding: 12px; }
  .tiptap-editor :deep(.tiptap table td),
  .tiptap-editor :deep(.tiptap table th) { padding: 6px 8px; min-width: 60px; }
  /* 评论抽屉全屏 */
  .comment-list { gap: 8px; }
  .comment-item { padding: 8px 0; }
  .comment-reply { margin-left: 12px; padding-left: 8px; }
  /* 弹窗全屏 */
  :deep(.el-dialog) { margin: 8px !important; }
  :deep(.el-dialog__body) { max-height: 60vh; overflow-y: auto; }
  /* 版本预览 */
  .preview-content { padding: 12px; max-height: 50vh; }
  .diff-content { padding: 8px; max-height: 50vh; }
  /* 统计 */
  .stats-grid { grid-template-columns: repeat(2, 1fr); }
}

@media (max-width: 480px) {
  .editor-header { padding: 6px; }
  .tb-btn { padding: 4px 6px; }
  .header-right .el-button { padding: 6px 8px; }
  .stats-grid { grid-template-columns: 1fr; }
}

.export-options {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.export-option {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 12px 16px;
  border: 1px solid #e4e7ed;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.15s;
}
.export-option:hover {
  border-color: #409eff;
  background: #ecf5ff;
}
.export-icon {
  font-size: 24px;
  flex-shrink: 0;
}
.export-info {
  flex: 1;
}
.export-name {
  font-weight: 600;
  font-size: 14px;
  color: #303133;
}
.export-desc {
  font-size: 12px;
  color: #909399;
  margin-top: 2px;
}

/* 分享/协作者弹窗 */
.share-add-row {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;
}
.target-option {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 0;
}
.target-avatar {
  width: 28px; height: 28px; border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  font-size: 13px; flex-shrink: 0;
}
.target-info { line-height: 1.3; }
.target-name { font-size: 14px; color: #303133; }
.target-sub { font-size: 12px; color: #909399; }

.collaborators-list {
  max-height: 300px;
  overflow-y: auto;
}
.collaborator-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 0;
  border-bottom: 1px solid #f0f0f0;
}
.collaborator-item:last-child { border-bottom: none; }
.collab-left { display: flex; align-items: center; gap: 10px; }
.collab-avatar {
  width: 32px; height: 32px; border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  font-size: 14px; font-weight: 500; flex-shrink: 0;
  background: #f0f0f0;
}
.collab-info { line-height: 1.3; }
.collab-name { font-size: 14px; color: #303133; display: flex; align-items: center; }
.collab-sub { font-size: 12px; color: #909399; }
.collab-right { flex-shrink: 0; }
.role-dropdown {
  display: inline-flex; align-items: center; gap: 4px;
  cursor: pointer; color: #606266; font-size: 13px;
  padding: 4px 8px; border-radius: 6px; border: 1px solid #dcdfe6;
}
.role-dropdown:hover { border-color: #409eff; color: #409eff; }

.link-share-section { margin-top: 4px; }
.link-share-header {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 8px;
}
.link-share-body {
  display: flex; align-items: center;
  margin-bottom: 8px;
}
.share-link-result { margin-top: 8px; }
.existing-shares { margin-top: 8px; }
.share-link-item {
  display: flex; align-items: center; gap: 8px;
  padding: 6px 0; font-size: 13px; color: #606266;
}
.share-link-token { font-family: monospace; font-size: 12px; }
.share-link-badge { font-size: 11px; }
.share-link-badge.expired { color: #f56c6c; }
.share-link-count { color: #909399; font-size: 12px; }
</style>
