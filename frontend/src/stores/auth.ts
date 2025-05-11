import { defineStore } from 'pinia';
import axios from 'axios';
import api from '@/api';

export interface User {
  id: number;
  username: string;
  is_admin: boolean;
}

export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: null as User | null,
    token: localStorage.getItem('token') || null,
    loading: false,
  }),
  
  getters: {
    isAuthenticated: (state) => !!state.token,
    isAdmin: (state) => state.user?.is_admin || false,
  },
  
  actions: {
    async login(username: string, password: string) {
      this.loading = true;
      
      try {
        const response = await api.post('/auth/login', { username, password });
        
        if (response.data.token) {
          this.token = response.data.token;
          this.user = response.data.user;
          
          // 保存token到本地存储
          localStorage.setItem('token', response.data.token);
          
          // 设置API请求的Authorization头
          axios.defaults.headers.common['Authorization'] = `Bearer ${response.data.token}`;
          
          // 确保重新应用到请求头上
          api.defaults.headers.common['Authorization'] = `Bearer ${response.data.token}`;
          
          return response.data;
        } else {
          throw new Error('登录失败: 无效的响应');
        }
      } catch (error: any) {
        const errorMsg = error.response?.data?.error || error.message || '登录失败';
        throw new Error(errorMsg);
      } finally {
        this.loading = false;
      }
    },
    
    async logout() {
      // 清除状态
      this.token = null;
      this.user = null;
      
      // 清除本地存储
      localStorage.removeItem('token');
      
      // 清除请求头
      delete axios.defaults.headers.common['Authorization'];
    },
    
    async fetchCurrentUser() {
      if (!this.token) return null;
      
      try {
        const response = await api.get('/auth/user');
        this.user = response.data;
        return this.user;
      } catch (error) {
        // 如果获取用户信息失败，可能是token已过期
        this.logout();
        throw error;
      }
    },
    
    async changePassword(oldPassword: string, newPassword: string) {
      if (!this.token) throw new Error('未登录');
      
      try {
        const response = await api.post('/auth/change-password', {
          old_password: oldPassword,
          new_password: newPassword
        });
        return response.data;
      } catch (error: any) {
        const errorMsg = error.response?.data?.error || error.message || '修改密码失败';
        throw new Error(errorMsg);
      }
    },
    
    // 初始化认证状态
    init() {
      if (this.token) {
        // 同时设置全局axios和api实例的默认请求头
        axios.defaults.headers.common['Authorization'] = `Bearer ${this.token}`;
        api.defaults.headers.common['Authorization'] = `Bearer ${this.token}`;
        
        this.fetchCurrentUser().catch(() => this.logout());
      }
    }
  }
}); 