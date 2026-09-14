// app.js
import { request } from './utils/request';
import { getToken, setToken, removeToken, setUserInfo, getUserInfo } from './utils/storage';

App({
  globalData: {
    userInfo: null,
    token: null,
    lanBaseUrl: 'http://192.168.10.5:8080', // 真机调试用：电脑当前局域网 IP（电脑换网络后要同步改）
    apiBaseUrl: 'http://localhost:8080', // 由 initSystemInfo 按运行环境覆盖；上线前改为真实域名
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
    // 模拟器（platform=devtools）跑在本机：localhost 直达后端，电脑换网络/IP 变化都不受影响；
    // 真机则必须走电脑的局域网 IP（lanBaseUrl）
    this.globalData.apiBaseUrl = systemInfo.platform === 'devtools'
      ? 'http://localhost:8080'
      : this.globalData.lanBaseUrl;
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
