<template>
  <el-config-provider :locale="zhCn">
    <div id="app">
      <el-container v-if="$route.name !== 'login'">
        <el-header height="60px">
          <AppHeader />
        </el-header>
        <el-container>
          <el-aside width="220px">
            <el-menu
              router
              default-active="$route.path"
              class="side-menu"
              :collapse="false"
            >
              <el-menu-item index="/">
                <el-icon><HomeFilled /></el-icon>
                <span>首页</span>
              </el-menu-item>
              <el-menu-item index="/subscriptions">
                <el-icon><Document /></el-icon>
                <span>订阅管理</span>
              </el-menu-item>
              <el-menu-item index="/proxies">
                <el-icon><Connection /></el-icon>
                <span>节点管理</span>
              </el-menu-item>
              <el-menu-item index="/settings">
                <el-icon><Setting /></el-icon>
                <span>系统设置</span>
              </el-menu-item>
            </el-menu>
          </el-aside>
          <el-main>
            <router-view></router-view>
          </el-main>
        </el-container>
      </el-container>
      
      <!-- 登录页面使用不同布局 -->
      <div v-else class="login-layout">
        <router-view></router-view>
      </div>
    </div>
  </el-config-provider>
</template>

<script setup lang="ts">
import { ElConfigProvider } from 'element-plus';
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import { onMounted } from 'vue'
import { useAuthStore } from './stores/auth'
import AppHeader from './components/AppHeader.vue'
import { HomeFilled, Document, Connection, Setting } from '@element-plus/icons-vue';

const authStore = useAuthStore()

onMounted(() => {
  // 初始化认证状态
  authStore.init()
})
</script>

<style>
html, body, #app, .el-container {
  width: 100vw;
  height: 100%;
  margin: 0;
  padding: 0;
}

#app {
  font-family: 'Helvetica Neue', Helvetica, 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', Arial, sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  height: 100vh;
}

.el-header {
  padding: 0;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  z-index: 1000;
}

.el-aside {
  background-color: #f5f7fa;
  border-right: 1px solid #e6e6e6;
  height: calc(100vh - 60px);
  overflow-y: auto;
}

.side-menu {
  height: 100%;
  border-right: none;
}

.el-main {
  background-color: #fff;
  padding: 20px;
  overflow-y: auto;
  width: 100vw;
}

.login-layout {
  width: 100vw;
  height: 100vh;
}
</style>
