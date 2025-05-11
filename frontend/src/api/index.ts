import axios from 'axios';

const API_URL = 'http://localhost:8080/api';

const api = axios.create({
  baseURL: API_URL,
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
});

export interface Subscription {
  id?: number;
  name: string;
  url: string;
  type: string;
  enabled: boolean;
  lastUpdated?: string;
  createdAt?: string;
  updatedAt?: string;
  valid_proxy_count?: number;
}

export interface Proxy {
  id?: number;
  subscription_id: number;
  name: string;
  type: string;
  server: string;
  port: number;
  uuid?: string;
  password?: string;
  method?: string;
  network?: string;
  path?: string;
  host?: string;
  tls?: boolean;
  sni?: string;
  alpn?: string;
  rawConfig?: string;
}

// 订阅相关API
export const subscriptionApi = {
  getAll: () => api.get<Subscription[]>('/subscriptions'),
  getById: (id: number) => api.get<Subscription>(`/subscriptions/${id}`),
  create: (subscription: Subscription) => api.post<Subscription>('/subscriptions', subscription),
  update: (id: number, subscription: Subscription) => api.put<Subscription>(`/subscriptions/${id}`, subscription),
  delete: (id: number) => api.delete(`/subscriptions/${id}`),
  refresh: (id: number) => api.post(`/subscriptions/${id}/refresh`),
};

// 代理节点相关API
export const proxyApi = {
  getAll: (subscriptionId?: number) => {
    const params = subscriptionId ? { subscription_id: subscriptionId } : {};
    return api.get<Proxy[]>('/proxies', { params });
  },
  getById: (id: number) => api.get<Proxy>(`/proxies/${id}`),
};

// 获取合并订阅链接
export const getMergedSubscriptionUrl = (format: string = 'base64') => {
  return `${API_URL}/merged?format=${format}`;
};

// 设置相关API
export const settingsApi = {
  getSettings: () => api.get('/settings'),
  saveSettings: (settings: {
    autoRefresh: boolean;
    refreshInterval: number;
    defaultFormat: string;
  }) => api.post('/settings', settings),
};

export default api;