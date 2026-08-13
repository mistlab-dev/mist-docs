import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import 'element-plus/theme-chalk/dark/css-vars.css'
import './styles/dark.css'
import {
  ArrowDown, Bell, Clock, DataAnalysis, Delete, Document, Download,
  Files, Folder, List, Loading, Monitor, Moon, MoreFilled,
  OfficeBuilding, Operation, Plus, QuestionFilled, Search,
  Star, StarFilled, Sunny, Upload, User, WarningFilled,
} from '@element-plus/icons-vue'
import App from './App.vue'
import router from './router'
import i18n from './i18n'

const app = createApp(App)
const pinia = createPinia()

// Register only the icons actually used in templates
const ICONS = {
  ArrowDown, Bell, Clock, DataAnalysis, Delete, Document, Download,
  Files, Folder, List, Loading, Monitor, Moon, MoreFilled,
  OfficeBuilding, Operation, Plus, QuestionFilled, Search,
  Star, StarFilled, Sunny, Upload, User, WarningFilled,
}
for (const [key, component] of Object.entries(ICONS)) {
  app.component(key, component)
}

app.use(pinia)
app.use(router)
app.use(i18n)
app.use(ElementPlus)

app.mount('#app')
