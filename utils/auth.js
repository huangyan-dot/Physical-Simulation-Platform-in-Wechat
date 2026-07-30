// utils/auth.js - 登录与权限相关
import { request } from './request';
import { getToken, setToken, setUserInfo, removeToken, removeUserInfo } from './storage';

function getAppInstance() {
  return getApp({ allowDefault: true });
}

/**
 * 微信登录并换取后端 token,接入时将url:'/auth/login'
 */
export function wxLogin() {
  return new Promise((resolve, reject) => {
    wx.login({
      success(res) {
        if (res.code) {
          request({
            url: '/login',
            method: 'POST',
            needAuth: false,
            data: { code: res.code }
          })
            .then((data) => {
              const { token, user } = data;
              setToken(token);
              setUserInfo(user);
              if (getAppInstance()) {
                getAppInstance().globalData.token = token;
                getAppInstance().globalData.userInfo = user;
                getAppInstance().globalData.isLogin = true;
              }
              resolve(data);
            })
            .catch(reject);
        } else {
          reject(new Error('获取微信登录 code 失败'));
        }
      },
      fail: reject
    });
  });
}

/**
 * 退出登录
 */
export function logout() {
  removeToken();
  removeUserInfo();
  if (getAppInstance()) {
    getAppInstance().globalData.token = null;
    getAppInstance().globalData.userInfo = null;
    getAppInstance().globalData.isLogin = false;
  }
  wx.reLaunch({ url: '/pages/login/login' });
}

/**
 * 检查是否登录，未登录跳转登录页
 */
export function checkLogin(redirectUrl = '') {
  const token = getToken();
  if (!token) {
    wx.navigateTo({
      url: `/pages/login/login${redirectUrl ? '?redirect=' + encodeURIComponent(redirectUrl) : ''}`
    });
    return false;
  }
  return true;
}

/**
 * 获取当前用户角色
 */
export function getCurrentRole() {
  const userInfo = getAppInstance() && getAppInstance().globalData.userInfo ? getAppInstance().globalData.userInfo : wx.getStorageSync('physics_user_info');
  return userInfo ? userInfo.role : null;
}

/**
 * 判断当前用户是否是指定角色
 */
export function isRole(role) {
  return getCurrentRole() === role;
}
