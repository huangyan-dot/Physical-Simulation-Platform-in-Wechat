// app.js
import { request } from './utils/request';
import { getToken, setToken, removeToken, setUserInfo, getUserInfo } from './utils/storage';
import { API_BASE_URL } from './utils/config';

App({
  globalData: {
    userInfo: null,
    token: null,
    // 后端地址统一在 utils/config.js 中维护（真机调试需改为电脑局域网 IP）
    apiBaseUrl: API_BASE_URL,
    isLogin: false,
    systemInfo: null
  },

  onLaunch() {
    console.log('[App] onLaunch');
    this.initSystemInfo();
    this.checkLoginStatus();
  },

  onShow() {
    console.log('[App] onShow');
  },

  // 初始化系统信息
  initSystemInfo() {
    const systemInfo = wx.getSystemInfoSync();
    this.globalData.systemInfo = systemInfo;
    console.log('[App] systemInfo:', systemInfo);
  },

  // 检查登录态
  checkLoginStatus() {
    const token = getToken();
    const userInfo = getUserInfo();
    if (token && userInfo) {
      this.globalData.token = token;
      this.globalData.userInfo = userInfo;
      this.globalData.isLogin = true;
      console.log('[App] 已登录:', userInfo);
    } else {
      console.log('[App] 未登录');
    }
  },

  // 设置登录态
  setLoginState(token, userInfo) {
    setToken(token);
    setUserInfo(userInfo);
    this.globalData.token = token;
    this.globalData.userInfo = userInfo;
    this.globalData.isLogin = true;
  },

  // 清除登录态
  clearLoginState() {
    removeToken();
    this.globalData.token = null;
    this.globalData.userInfo = null;
    this.globalData.isLogin = false;
  },

  // 全局登录方法
  async login() {
    try {
      const wxLoginRes = await wx.login();
      console.log('[App] wx.login code:', wxLoginRes.code);

      const res = await request({
        url: '/auth/login',
        method: 'POST',
        data: { code: wxLoginRes.code }
      });

      const { token, user } = res;
      this.setLoginState(token, user);

      // 如果是首次登录，跳转到补全信息页
      if (user.need_complete) {
        wx.navigateTo({ url: '/pages/login/login?mode=complete' });
      }

      return { success: true, user };
    } catch (err) {
      console.error('[App] 登录失败:', err);
      return { success: false, error: err };
    }
  }
});
