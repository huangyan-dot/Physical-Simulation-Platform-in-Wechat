// utils/request.js - 统一请求封装
import { getToken, removeToken, removeUserInfo } from './storage';

// 默认基础 URL，可被 app.globalData.apiBaseUrl 覆盖
const DEFAULT_BASE_URL = 'http://localhost:8080';

function getAppInstance() {
  return getApp({ allowDefault: true });
}

/**
 * 统一请求方法
 * @param {Object} options
 * @param {string} options.url - 接口路径，例如 /auth/login
 * @param {string} options.method - HTTP 方法
 * @param {Object} options.data - 请求数据
 * @param {boolean} options.needAuth - 是否需要携带 token，默认 true
 * @param {boolean} options.showLoading - 是否显示 loading，默认 false
 */
export function request(options) {
  const {
    url,
    method = 'GET',
    data = {},
    needAuth = true,
    showLoading = false
  } = options;

  const app = getAppInstance();
  const baseUrl = app && app.globalData.apiBaseUrl ? app.globalData.apiBaseUrl : DEFAULT_BASE_URL;
  const fullUrl = url.startsWith('http') ? url : `${baseUrl}${url}`;

  if (showLoading) {
    wx.showLoading({ title: '加载中...', mask: true });
  }

  return new Promise((resolve, reject) => {
    const header = {
      'Content-Type': 'application/json'
    };

    if (needAuth) {
      const token = getToken();
      if (token) {
        header.Authorization = `Bearer ${token}`;
      }
    }

    wx.request({
      url: fullUrl,
      method,
      data,
      header,
      timeout: 15000,
      success(res) {
        const { statusCode, data: responseData } = res;

        if (statusCode >= 200 && statusCode < 300) {
          resolve(responseData);
        } else if (statusCode === 401) {
          handleAuthError();
          reject(new Error(responseData.message || '登录已过期，请重新登录'));
        } else if (statusCode === 403) {
          reject(new Error(responseData.message || '没有权限执行此操作'));
        } else {
          reject(new Error(responseData.message || `请求失败: ${statusCode}`));
        }
      },
      fail(err) {
        console.error('[request] 请求失败:', err);
        reject(new Error(err.errMsg || '网络请求失败'));
      },
      complete() {
        if (showLoading) {
          wx.hideLoading();
        }
      }
    });
  });
}

// 处理认证失败
function handleAuthError() {
  removeToken();
  removeUserInfo();
  const app = getAppInstance();
  if (app) {
    app.globalData.token = null;
    app.globalData.userInfo = null;
    app.globalData.isLogin = false;
  }
  wx.showToast({ title: '登录已过期', icon: 'none' });
  setTimeout(() => {
    wx.reLaunch({ url: '/pages/login/login' });
  }, 1500);
}

// 上传文件（预留）
export function uploadFile(url, filePath, formData = {}) {
  const app = getAppInstance();
  const baseUrl = app && app.globalData.apiBaseUrl ? app.globalData.apiBaseUrl : DEFAULT_BASE_URL;
  const fullUrl = url.startsWith('http') ? url : `${baseUrl}${url}`;
  const token = getToken();

  return new Promise((resolve, reject) => {
    wx.uploadFile({
      url: fullUrl,
      filePath,
      name: 'file',
      formData,
      header: token ? { Authorization: `Bearer ${token}` } : {},
      success(res) {
        const data = JSON.parse(res.data);
        if (res.statusCode >= 200 && res.statusCode < 300) {
          resolve(data);
        } else {
          reject(new Error(data.message || '上传失败'));
        }
      },
      fail(err) {
        reject(new Error(err.errMsg || '上传失败'));
      }
    });
  });
}
