import axios from 'axios';

// 动态获取API URL，根据当前访问地址构建
const getApiUrl = () => {
  const { protocol, hostname } = window.location;
  const port = import.meta.env.VITE_API_PORT || window.location.port || '8080';
  
  // 如果是开发环境，使用默认的localhost:8080
  if (import.meta.env.DEV) {
    return 'http://localhost:8080/api';
  }
  
  // 在生产环境中，使用当前访问的域名和协议
  return `${protocol}//${hostname}${port ? `:${port}` : ''}/api`;
};

const API_URL = getApiUrl();

// 获取存储的token
const storedToken = localStorage.getItem('token');

const api = axios.create({
  baseURL: API_URL,
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
    // 如果存在token，则添加到默认请求头
    ...(storedToken ? { 'Authorization': `Bearer ${storedToken}` } : {})
  },
});

// 请求拦截器，确保每次请求都带上最新的token
api.interceptors.request.use(config => {
  const token = localStorage.getItem('token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// 响应拦截器，统一处理错误信息
api.interceptors.response.use(
  (response) => {
    // 成功响应直接返回
    return response;
  },
  (error) => {
    // 处理错误响应
    if (error.response) {
      // 服务器返回了错误响应
      const { status, data } = error.response;
      
      // 从响应中提取错误信息
      let errorMessage = '操作失败';
      
      if (data) {
        // 优先使用 error 字段
        if (data.error) {
          errorMessage = data.error;
        } else if (data.message) {
          errorMessage = data.message;
        } else if (typeof data === 'string') {
          errorMessage = data;
        }
      }
      
      // 根据状态码提供默认错误信息
      if (!data || (!data.error && !data.message)) {
        switch (status) {
          case 400:
            errorMessage = '请求参数错误';
            break;
          case 401:
            errorMessage = '未授权，请重新登录';
            break;
          case 403:
            errorMessage = '访问被拒绝';
            break;
          case 404:
            errorMessage = '资源不存在';
            break;
          case 500:
            errorMessage = '服务器内部错误';
            break;
          default:
            errorMessage = `请求失败 (${status})`;
        }
      }
      
      // 将错误信息设置到 error.message，方便前端使用
      error.message = errorMessage;
    } else if (error.request) {
      // 请求已发出但没有收到响应
      error.message = '网络错误，请检查网络连接';
    } else {
      // 请求配置出错
      error.message = error.message || '请求配置错误';
    }
    
    return Promise.reject(error);
  }
);

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
  // 去掉API_URL末尾的'/api'以获取基础URL
  const baseUrl = API_URL.replace(/\/api$/, '');
  return `${baseUrl}/api/merged?format=${format}`;
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