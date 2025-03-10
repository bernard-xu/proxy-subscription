<template>
  <el-config-provider :locale="zhCn">
    <div class="app-container">
      <el-container>
        <el-header height="60px">
          <div class="header-content">
            <h1>NekoRay 配置管理</h1>
          </div>
        </el-header>
        <el-container>
          <el-aside width="200px">
            <el-menu
              router
              :default-active="$route.path"
              class="el-menu-vertical"
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
                <span>节点列表</span>
              </el-menu-item>
              <el-menu-item index="/settings">
                <el-icon><Setting /></el-icon>
                <span>设置</span>
              </el-menu-item>
            </el-menu>
          </el-aside>
          <el-main>
            <router-view />
          </el-main>
        </el-container>
      </el-container>
    </div>
  </el-config-provider>
</template>

<script setup lang="ts">
import { HomeFilled, Document, Connection, Setting } from '@element-plus/icons-vue';
import { ElConfigProvider } from 'element-plus';
import zhCn from 'element-plus/es/locale/lang/zh-cn'
</script>

<style>
.app-container {
  height: 100vh;
  width: 100vw;
}

.el-header {
  background-color: #409EFF;
  color: white;
  display: flex;
  align-items: center;
}

.header-content {
  width: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.el-aside {
  background-color: #f5f7fa;
}

.el-menu-vertical {
  height: calc(100vh - 60px);
  width: 100%;
}

.el-main {
  padding: 5px;
}
</style>
