<template>
  <div class="proxy-container">
    <div class="page-header">
      <h2>节点列表</h2>
      <div class="filter-actions">
        <el-button type="primary" @click="openAddDialog">
          <el-icon><Plus /></el-icon>
          添加自定义节点
        </el-button>
        <el-button @click="openImportDialog">
          <el-icon><Upload /></el-icon>
          一键导入
        </el-button>
        <el-select
          v-model="selectedSubscription"
          placeholder="选择分组过滤"
          clearable
          @change="handleSubscriptionChange"
        >
          <el-option label="自定义节点" :value="CUSTOM_GROUP_ID" />
          <el-option
            v-for="sub in subscriptionStore.subscriptions"
            :key="sub.id!"
            :label="sub.name"
            :value="sub.id"
          />
        </el-select>
        <el-input v-model="searchQuery" placeholder="搜索节点" clearable class="search-input">
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
      </div>
    </div>

    <el-card v-if="proxyStore.loading" class="loading-card">
      <el-skeleton :rows="10" animated />
    </el-card>

    <el-empty v-else-if="!filteredProxies.length" description="暂无节点" />

    <el-table v-else :data="filteredProxies" style="width: 100%" border stripe>
      <el-table-column label="名称" min-width="200" show-overflow-tooltip>
        <template #default="scope">
          {{ scope.row.display_name || scope.row.name }}
        </template>
      </el-table-column>
      <el-table-column prop="type" label="类型" width="100">
        <template #default="scope">
          <el-tag :type="getProxyTypeTag(scope.row.type)">
            {{ scope.row.type.toUpperCase() }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="server" label="服务器" min-width="150" show-overflow-tooltip />
      <el-table-column prop="port" label="端口" width="100" />
      <el-table-column label="分组" min-width="150">
        <template #default="scope">
          <el-tag v-if="scope.row.is_custom" type="success">自定义节点</el-tag>
          <span v-else>{{ scope.row.subscription_name || getSubscriptionName(scope.row.subscription_id) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="220" fixed="right">
        <template #default="scope">
          <el-button-group>
            <el-button size="small" @click="showProxyDetail(scope.row)">
              <el-icon><View /></el-icon>
              详情
            </el-button>
            <el-button size="small" @click="openEditDialog(scope.row)">
              <el-icon><Edit /></el-icon>
              编辑
            </el-button>
            <el-button v-if="scope.row.is_custom" size="small" type="danger" @click="deleteCustomProxy(scope.row)">
              <el-icon><Delete /></el-icon>
              删除
            </el-button>
          </el-button-group>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="detailDialogVisible" title="节点详情" width="600px">
      <div v-if="selectedProxy">
        <el-descriptions :column="1" border>
          <el-descriptions-item label="名称">
            {{ selectedProxy.display_name || selectedProxy.name }}
          </el-descriptions-item>
          <el-descriptions-item v-if="selectedProxy.display_name" label="原始名称">
            {{ selectedProxy.name }}
          </el-descriptions-item>
          <el-descriptions-item label="分组">
            {{ selectedProxy.is_custom ? '自定义节点' : selectedProxy.subscription_name }}
          </el-descriptions-item>
          <el-descriptions-item label="类型">{{ selectedProxy.type.toUpperCase() }}</el-descriptions-item>
          <el-descriptions-item label="服务器">{{ selectedProxy.server }}</el-descriptions-item>
          <el-descriptions-item label="端口">{{ selectedProxy.port }}</el-descriptions-item>
          <el-descriptions-item v-if="selectedProxy.uuid" label="UUID">{{ selectedProxy.uuid }}</el-descriptions-item>
          <el-descriptions-item v-if="selectedProxy.password" label="密码">{{ selectedProxy.password }}</el-descriptions-item>
          <el-descriptions-item v-if="selectedProxy.method" label="加密方式">{{ selectedProxy.method }}</el-descriptions-item>
          <el-descriptions-item v-if="selectedProxy.network" label="传输协议">{{ selectedProxy.network }}</el-descriptions-item>
          <el-descriptions-item v-if="selectedProxy.path" label="路径">{{ selectedProxy.path }}</el-descriptions-item>
          <el-descriptions-item v-if="selectedProxy.host" label="主机名">{{ selectedProxy.host }}</el-descriptions-item>
          <el-descriptions-item v-if="selectedProxy.tls !== undefined" label="TLS">
            {{ selectedProxy.tls ? '启用' : '禁用' }}
          </el-descriptions-item>
          <el-descriptions-item v-if="selectedProxy.sni" label="SNI">{{ selectedProxy.sni }}</el-descriptions-item>
          <el-descriptions-item v-if="selectedProxy.alpn" label="ALPN">{{ selectedProxy.alpn }}</el-descriptions-item>
        </el-descriptions>
      </div>
    </el-dialog>

    <el-dialog v-model="formDialogVisible" :title="editingProxyId ? '编辑节点' : '添加自定义节点'" width="680px">
      <el-form ref="proxyFormRef" :model="proxyForm" :rules="proxyRules" label-width="110px">
        <el-form-item label="节点名称" prop="name">
          <el-input v-model="proxyForm.name" />
        </el-form-item>
        <el-form-item label="类型" prop="type">
          <el-select v-model="proxyForm.type">
            <el-option label="VMess" value="vmess" />
            <el-option label="VLESS" value="vless" />
            <el-option label="Shadowsocks" value="ss" />
            <el-option label="Trojan" value="trojan" />
            <el-option label="TUIC" value="tuic" />
            <el-option label="AnyTLS" value="anytls" />
            <el-option label="Hysteria2" value="hysteria2" />
          </el-select>
        </el-form-item>
        <el-form-item label="服务器" prop="server">
          <el-input v-model="proxyForm.server" />
        </el-form-item>
        <el-form-item label="端口" prop="port">
          <el-input-number v-model="proxyForm.port" :min="1" :max="65535" controls-position="right" />
        </el-form-item>

        <template v-if="proxyForm.type === 'vmess' || proxyForm.type === 'vless'">
          <el-form-item label="UUID" prop="uuid">
            <el-input v-model="proxyForm.uuid" />
          </el-form-item>
          <el-form-item label="传输协议">
            <el-select v-model="proxyForm.network">
              <el-option label="tcp" value="tcp" />
              <el-option label="ws" value="ws" />
              <el-option label="grpc" value="grpc" />
            </el-select>
          </el-form-item>
          <el-form-item label="TLS">
            <el-switch v-model="proxyForm.tls" />
          </el-form-item>
          <el-form-item label="路径">
            <el-input v-model="proxyForm.path" />
          </el-form-item>
          <el-form-item label="主机名">
            <el-input v-model="proxyForm.host" />
          </el-form-item>
          <template v-if="proxyForm.type === 'vless'">
            <el-form-item label="SNI">
              <el-input v-model="proxyForm.sni" />
            </el-form-item>
            <el-form-item label="ALPN">
              <el-input v-model="proxyForm.alpn" />
            </el-form-item>
          </template>
        </template>

        <template v-if="proxyForm.type === 'ss'">
          <el-form-item label="加密方式" prop="method">
            <el-input v-model="proxyForm.method" placeholder="例如 aes-256-gcm" />
          </el-form-item>
          <el-form-item label="密码" prop="password">
            <el-input v-model="proxyForm.password" />
          </el-form-item>
          <el-form-item label="插件">
            <el-input v-model="proxyForm.plugin" />
          </el-form-item>
          <el-form-item label="插件参数">
            <el-input v-model="proxyForm.plugin_opts" />
          </el-form-item>
        </template>

        <template v-if="proxyForm.type === 'trojan'">
          <el-form-item label="密码" prop="password">
            <el-input v-model="proxyForm.password" />
          </el-form-item>
          <el-form-item label="SNI">
            <el-input v-model="proxyForm.sni" />
          </el-form-item>
          <el-form-item label="ALPN">
            <el-input v-model="proxyForm.alpn" placeholder="例如 h2,http/1.1" />
          </el-form-item>
          <el-form-item label="跳过证书验证">
            <el-switch v-model="proxyForm.allow_insecure" />
          </el-form-item>
        </template>

        <template v-if="proxyForm.type === 'tuic' || proxyForm.type === 'anytls' || proxyForm.type === 'hysteria2'">
          <el-form-item v-if="proxyForm.type === 'tuic'" label="UUID" prop="uuid">
            <el-input v-model="proxyForm.uuid" />
          </el-form-item>
          <el-form-item label="密码" prop="password">
            <el-input v-model="proxyForm.password" />
          </el-form-item>
          <el-form-item label="SNI">
            <el-input v-model="proxyForm.sni" />
          </el-form-item>
          <el-form-item label="ALPN">
            <el-input v-model="proxyForm.alpn" placeholder="例如 h3" />
          </el-form-item>
          <el-form-item label="跳过证书验证">
            <el-switch v-model="proxyForm.allow_insecure" />
          </el-form-item>
        </template>
      </el-form>
      <template #footer>
        <el-button @click="formDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingProxy" @click="saveCustomProxy">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="importDialogVisible" title="一键导入节点" width="680px">
      <el-input
        v-model="importLink"
        type="textarea"
        :rows="6"
        placeholder="粘贴 vmess://、vless://、tuic://、anytls:// 或 hysteria2:// 链接"
      />
      <template #footer>
        <el-button @click="importDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingProxy" @click="importCustomProxy">导入</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue';
import { ElMessage, ElMessageBox, type FormInstance } from 'element-plus';
import { Delete, Edit, Plus, Search, Upload, View } from '@element-plus/icons-vue';
import { useSubscriptionStore } from '@/stores/subscription';
import { useProxyStore } from '@/stores/proxy';
import { type Proxy } from '@/api';

const CUSTOM_GROUP_ID = -1;
const subscriptionStore = useSubscriptionStore();
const proxyStore = useProxyStore();

const selectedSubscription = ref<number | null>(null);
const searchQuery = ref('');
const detailDialogVisible = ref(false);
const selectedProxy = ref<Proxy | null>(null);
const formDialogVisible = ref(false);
const importDialogVisible = ref(false);
const importLink = ref('');
const savingProxy = ref(false);
const editingProxyId = ref<number | null>(null);
const proxyFormRef = ref<FormInstance>();

const createEmptyProxy = (): Proxy => ({
  subscription_id: 0,
  is_custom: true,
  name: '',
  type: 'vmess',
  server: '',
  port: 443,
  uuid: '',
  password: '',
  method: '',
  network: 'tcp',
  path: '',
  host: '',
  tls: false,
  sni: '',
  alpn: '',
  plugin: '',
  plugin_opts: '',
  allow_insecure: false,
  rawConfig: '',
});

const proxyForm = ref<Proxy>(createEmptyProxy());

const proxyRules = {
  name: [{ required: true, message: '请输入节点名称', trigger: 'blur' }],
  type: [{ required: true, message: '请选择节点类型', trigger: 'change' }],
  server: [{ required: true, message: '请输入服务器', trigger: 'blur' }],
  port: [{ required: true, message: '请输入端口', trigger: 'change' }],
  uuid: [{ required: true, message: '请输入 UUID', trigger: 'blur' }],
  method: [{ required: true, message: '请输入加密方式', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
};

const filteredProxies = computed(() => {
  let result = proxyStore.proxies;

  if (selectedSubscription.value === CUSTOM_GROUP_ID) {
    result = result.filter(proxy => proxy.is_custom);
  } else if (selectedSubscription.value) {
    result = result.filter(proxy => proxy.subscription_id === selectedSubscription.value);
  }

  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase();
    result = result.filter(proxy => {
      const displayName = (proxy.display_name || proxy.name || '').toLowerCase();
      const name = (proxy.name || '').toLowerCase();
      const server = (proxy.server || '').toLowerCase();
      return displayName.includes(query) || name.includes(query) || server.includes(query);
    });
  }

  return result;
});

const getSubscriptionName = (subscriptionId: number) => {
  const subscription = subscriptionStore.getSubscriptionById(subscriptionId);
  return subscription ? subscription.name : '未知订阅';
};

const getProxyTypeTag = (type: string) => {
  switch (type.toLowerCase()) {
    case 'vmess':
      return 'primary';
    case 'vless':
      return 'danger';
    case 'ss':
      return 'success';
    case 'trojan':
      return 'warning';
    case 'tuic':
      return 'info';
    case 'anytls':
      return 'primary';
    case 'hysteria2':
      return 'success';
    default:
      return 'info';
  }
};

const showProxyDetail = (proxy: Proxy) => {
  selectedProxy.value = proxy;
  detailDialogVisible.value = true;
};

const openAddDialog = () => {
  editingProxyId.value = null;
  proxyForm.value = createEmptyProxy();
  formDialogVisible.value = true;
};

const openImportDialog = () => {
  importLink.value = '';
  importDialogVisible.value = true;
};

const openEditDialog = (proxy: Proxy) => {
  editingProxyId.value = proxy.id ?? null;
  proxyForm.value = { ...createEmptyProxy(), ...proxy };
  formDialogVisible.value = true;
};

const saveCustomProxy = async () => {
  if (!proxyFormRef.value) return;
  await proxyFormRef.value.validate();

  savingProxy.value = true;
  try {
    if (editingProxyId.value) {
      await proxyStore.updateProxy(editingProxyId.value, proxyForm.value);
      ElMessage.success('节点已更新');
    } else {
      await proxyStore.addProxy(proxyForm.value);
      ElMessage.success('自定义节点已添加');
    }
    formDialogVisible.value = false;
  } finally {
    savingProxy.value = false;
  }
};

const decodeBase64 = (value: string) => {
  const normalized = value.replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(value.length / 4) * 4, '=');
  return decodeURIComponent(
    Array.from(atob(normalized))
      .map(char => `%${char.charCodeAt(0).toString(16).padStart(2, '0')}`)
      .join(''),
  );
};

const parseImportedProxy = (rawLink: string): Proxy => {
  const link = rawLink.trim();
  if (link.startsWith('vmess://')) {
    const config = JSON.parse(decodeBase64(link.slice('vmess://'.length)));
    return {
      ...createEmptyProxy(),
      type: 'vmess',
      name: String(config.ps || config.name || config.add || 'VMess Node'),
      server: String(config.add || ''),
      port: Number(config.port || 0),
      uuid: String(config.id || ''),
      network: String(config.net || 'tcp'),
      path: String(config.path || ''),
      host: String(config.host || ''),
      tls: config.tls === 'tls',
      rawConfig: JSON.stringify(config),
    };
  }

  if (link.startsWith('vless://')) {
    const parsedUrl = new URL(link);
    const params = parsedUrl.searchParams;
    const security = params.get('security') || '';
    const rawConfig = Object.fromEntries(params.entries());
    return {
      ...createEmptyProxy(),
      type: 'vless',
      name: decodeURIComponent(parsedUrl.hash.replace(/^#/, '')) || parsedUrl.hostname,
      server: parsedUrl.hostname,
      port: Number(parsedUrl.port || 443),
      uuid: decodeURIComponent(parsedUrl.username),
      network: params.get('type') || 'tcp',
      path: params.get('path') || '',
      host: params.get('host') || params.get('authority') || '',
      tls: security === 'tls' || security === 'reality',
      sni: params.get('sni') || params.get('servername') || '',
      alpn: params.get('alpn') || '',
      allow_insecure: ['1', 'true'].includes(params.get('allowInsecure') || params.get('skip-cert-verify') || ''),
      rawConfig: JSON.stringify(rawConfig),
    };
  }

  if (link.startsWith('tuic://') || link.startsWith('anytls://') || link.startsWith('hysteria2://') || link.startsWith('hy2://')) {
    const parsedUrl = new URL(link);
    const params = parsedUrl.searchParams;
    const type = parsedUrl.protocol.replace(':', '') === 'hy2' ? 'hysteria2' : parsedUrl.protocol.replace(':', '');
    const rawConfig = Object.fromEntries(params.entries());
    const password = type === 'tuic' ? decodeURIComponent(parsedUrl.password) : decodeURIComponent(parsedUrl.username);
    return {
      ...createEmptyProxy(),
      type,
      name: decodeURIComponent(parsedUrl.hash.replace(/^#/, '')) || parsedUrl.hostname,
      server: parsedUrl.hostname,
      port: Number(parsedUrl.port || 443),
      uuid: type === 'tuic' ? decodeURIComponent(parsedUrl.username) : '',
      password,
      tls: true,
      sni: params.get('sni') || params.get('servername') || '',
      alpn: params.get('alpn') || '',
      allow_insecure: ['1', 'true'].includes(params.get('allowInsecure') || params.get('skip-cert-verify') || params.get('insecure') || ''),
      rawConfig: JSON.stringify(rawConfig),
    };
  }

  throw new Error('仅支持 vmess://、vless://、tuic://、anytls:// 和 hysteria2:// 链接');
};

const importCustomProxy = async () => {
  savingProxy.value = true;
  try {
    const importedProxy = parseImportedProxy(importLink.value);
    await proxyStore.addProxy(importedProxy);
    importDialogVisible.value = false;
    ElMessage.success('节点已导入');
  } catch (error: any) {
    ElMessage.error(error.message || '导入失败');
  } finally {
    savingProxy.value = false;
  }
};

const deleteCustomProxy = async (proxy: Proxy) => {
  if (!proxy.id) return;
  await ElMessageBox.confirm(`确定删除自定义节点「${proxy.name}」吗？`, '删除确认', {
    type: 'warning',
  });
  await proxyStore.deleteProxy(proxy.id);
  ElMessage.success('自定义节点已删除');
};

const handleSubscriptionChange = () => {};

onMounted(async () => {
  await subscriptionStore.fetchSubscriptions();
  await proxyStore.fetchProxies();
});

watch(
  () => subscriptionStore.subscriptions,
  () => {
    proxyStore.fetchProxies();
  },
  { deep: true },
);
</script>

<style scoped>
.proxy-container {
  max-width: 1200px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  margin-bottom: 20px;
}

.filter-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.filter-actions .el-select {
  width: 180px;
}

.search-input {
  width: 200px;
}

.loading-card {
  padding: 20px;
  margin-bottom: 15px;
}

@media (max-width: 760px) {
  .page-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .filter-actions {
    align-items: stretch;
    flex-wrap: wrap;
    width: 100%;
  }

  .filter-actions .el-button,
  .filter-actions .el-select,
  .search-input {
    width: 100%;
  }
}
</style>
