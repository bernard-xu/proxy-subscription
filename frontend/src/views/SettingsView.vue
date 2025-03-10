<template>
  <div class="settings-container">
    <h2>设置</h2>
    
    <el-card class="settings-card">
      <template #header>
        <div class="card-header">
          <h3>基本设置</h3>
        </div>
      </template>
      
      <el-form :model="settings" label-width="120px">
        <el-form-item label="自动刷新">
          <el-switch v-model="settings.autoRefresh" />
          <span class="setting-description">启用后，将按设定的间隔自动刷新订阅</span>
        </el-form-item>
        
        <el-form-item label="刷新间隔" v-if="settings.autoRefresh">
          <el-input-number v-model="settings.refreshInterval" :min="1" :max="24" />
          <span class="setting-unit">小时</span>
          <span class="setting-description">订阅自动刷新的时间间隔</span>
        </el-form-item>
        
        <el-form-item label="默认订阅格式">
          <el-select v-model="settings.defaultFormat" style="width: 200px">
            <el-option label="Base64" value="base64" />
            <el-option label="Clash" value="clash" />
            <el-option label="JSON" value="json" />
          </el-select>
          <span class="setting-description">合并订阅的默认输出格式</span>
        </el-form-item>
        
        <el-divider />
        
        <el-form-item>
          <el-button type="primary" @click="saveSettings" :loading="saving">保存设置</el-button>
          <el-button @click="resetSettings">重置</el-button>
        </el-form-item>
      </el-form>
    </el-card>
    
    <el-card class="settings-card">
      <template #header>
        <div class="card-header">
          <h3>系统信息</h3>
        </div>
      </template>
      
      <el-descriptions :column="1" border>
        <el-descriptions-item label="版本">1.0.0</el-descriptions-item>
        <el-descriptions-item label="后端状态">
          <el-tag :type="backendStatus === 'connected' ? 'success' : 'danger'">
            {{ backendStatus === 'connected' ? '已连接' : '未连接' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="数据库">SQLite</el-descriptions-item>
        <el-descriptions-item label="订阅数量">{{ subscriptionCount }}</el-descriptions-item>
        <el-descriptions-item label="节点数量">{{ proxyCount }}</el-descriptions-item>
      </el-descriptions>
    </el-card>
    
    <el-card class="settings-card">
      <template #header>
        <div class="card-header">
          <h3>关于</h3>
        </div>
      </template>
      
      <div class="about-content">
        <p>NekoRay 配置管理工具是一个用于管理代理订阅的Web应用，可以帮助您整合多个订阅源并提供统一的访问接口。</p>
        <p>主要功能：</p>
        <ul>
          <li>支持多种订阅格式（V2Ray, Shadowsocks, Trojan等）</li>
          <li>自动解析和更新订阅内容</li>
          <li>提供合并后的订阅链接，支持多种格式输出</li>
          <li>可与NekoBox等客户端配合使用</li>
        </ul>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue';
import { ElMessage } from 'element-plus';
import { useSubscriptionStore } from '@/stores/subscription';
import { useProxyStore } from '@/stores/proxy';
import api from '@/api';

const subscriptionStore = useSubscriptionStore();
const proxyStore = useProxyStore();

// 设置
const settings = reactive({
  autoRefresh: false,
  refreshInterval: 6,
  defaultFormat: 'base64'
});

// 状态
const saving = ref(false);
const backendStatus = ref('connecting');

// 统计信息
const subscriptionCount = computed(() => subscriptionStore.subscriptions.length);
const proxyCount = computed(() => proxyStore.proxies.length);

// 保存设置
const saveSettings = async () => {
  saving.value = true;
  
  try {
    // 这里应该调用API保存设置
    // 由于我们还没有实现设置API，这里只是模拟
    await new Promise(resolve => setTimeout(resolve, 500));
    
    // 保存到本地存储
    localStorage.setItem('settings', JSON.stringify(settings));
    
    ElMessage.success('设置保存成功');
  } catch (error) {
    ElMessage.error('设置保存失败');
  } finally {
    saving.value = false;
  }
};

// 重置设置
const resetSettings = () => {
  settings.autoRefresh = false;
  settings.refreshInterval = 6;
  settings.defaultFormat = 'base64';
};

// 检查后端连接状态
const checkBackendStatus = async () => {
  try {
    await api.get('/subscriptions', { timeout: 3000 });
    backendStatus.value = 'connected';
  } catch (error) {
    backendStatus.value = 'disconnected';
  }
};

// 加载设置
const loadSettings = () => {
  const savedSettings = localStorage.getItem('settings');
  if (savedSettings) {
    try {
      const parsed = JSON.parse(savedSettings);
      settings.autoRefresh = parsed.autoRefresh ?? false;
      settings.refreshInterval = parsed.refreshInterval ?? 6;
      settings.defaultFormat = parsed.defaultFormat ?? 'base64';
    } catch (error) {
      console.error('加载设置失败:', error);
    }
  }
};

// 初始化
onMounted(() => {
  loadSettings();
  checkBackendStatus();
  
  if (subscriptionStore.subscriptions.length === 0) {
    subscriptionStore.fetchSubscriptions();
  }
  
  if (proxyStore.proxies.length === 0) {
    proxyStore.fetchProxies();
  }
});
</script>

<style scoped>
.settings-container {
  max-width: 800px;
  margin: 0 auto;
}

.settings-card {
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-header h3 {
  margin: 0;
}

.setting-description {
  margin-left: 10px;
  color: #909399;
  font-size: 13px;
}

.setting-unit {
  margin: 0 10px;
}

.about-content {
  line-height: 1.6;
}
</style>