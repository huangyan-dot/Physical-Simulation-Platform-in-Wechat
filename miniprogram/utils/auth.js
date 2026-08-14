// utils/auth.js - 登录与权限相关
import { request } from './request';
import { getToken, setToken, setUserInfo, removeToken, removeUserInfo } from './storage';

function getAppInstance() {
  return getApp({ allowDefault: true });
}

/**
 * 微信登录并换取后端 token。
 * @param {Object} opts
 * @param {string} opts.role - 'student' | 'teacher'
 * @param {string} opts.inviteCode - 教师邀请码（role=teacher 时必填）
 * 开发阶段：wx.login 失败时自动降级为后端开发后门（dev_student / dev_teacher）。
 */
export function wxLogin(opts = {}) {
  const { role = 'student', inviteCode = '' } = opts;
  return new Promise((resolve, reject) => {
    wx.login({
      success(res) {
        if (res.code) {
          request({
            url: '/auth/login',
            method: 'POST',
            needAuth: false,
            data: { code: res.code, role, invite_code: inviteCode }
          })
            .then((data) => {
              const { token, user } = data;
              saveLoginState(token, user);
              resolve(data);
            })
            .catch(reject);
        } else {
          reject(new Error('获取微信登录 code 失败'));
        }
      },
      fail(err) {
        // wx.login 不可用（多见于未配 AppID 的开发者工具）：退回后端开发后门。
        const devCode = role === 'teacher' ? 'dev_teacher' : 'dev_student';
        console.warn(`[auth] wx.login 失败，尝试开发后门 ${devCode}:`, err && err.errMsg);
        request({
          url: '/auth/login',
          method: 'POST',
          needAuth: false,
          data: { code: devCode, role, invite_code: inviteCode }
        })
          .then((data) => {
            const { token, user } = data;
            saveLoginState(token, user);
            resolve(data);
          })
          .catch((e) => {
            reject(new Error(`登录失败：微信登录不可用，开发后门也失败（${e.message}）`));
          });
      }
    });
  });
}

function saveLoginState(token, user) {
  setToken(token);
  setUserInfo(user);
  const app = getAppInstance();
  if (app) {
    app.globalData.token = token;
    app.globalData.userInfo = user;
    app.globalData.isLogin = true;
  }
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
