// pages/login/login.js
import { wxLogin } from '../../utils/auth';
import { request } from '../../utils/request';
import { setToken, setUserInfo } from '../../utils/storage';
import { MOCK_CLASSES } from '../../utils/mock';

const app = getApp();

Page({
  data: {
    mode: 'login', // login | complete
    loading: false,
    redirect: '',
    form: {
      name: '',
      student_no: '',
      class_id: null
    },
    classIndex: 0,
    classList: []
  },

  onLoad(options) {
    const mode = options.mode || 'login';
    const redirect = options.redirect || '';
    this.setData({ mode, redirect });

    if (mode === 'complete') {
      this.loadClasses();
    }
  },

  // 微信登录
  async handleLogin() {
    if (this.data.loading) return;
    this.setData({ loading: true });

    try {
      const data = await wxLogin();
      const { user } = data;

      if (user.need_complete) {
        this.setData({ mode: 'complete', loading: false });
        this.loadClasses();
      } else {
        wx.showToast({ title: '登录成功', icon: 'success' });
        this.goBack();
      }
    } catch (err) {
      console.error('[login] 登录失败:', err);
      wx.showToast({ title: err.message || '登录失败', icon: 'none' });
      this.setData({ loading: false });
    }
  },

  // 加载班级列表
  async loadClasses() {
    try {
      const res = await request({
        url: '/classes',
        method: 'GET',
        showLoading: false
      });
      this.setData({
        classList: res.data || res || []
      });
    } catch (err) {
      console.warn('[login] 后端不可用，使用 mock 班级');
      this.setData({ classList: MOCK_CLASSES });
    }
  },

  // 输入框变化
  onInput(e) {
    const { field } = e.currentTarget.dataset;
    this.setData({
      [`form.${field}`]: e.detail.value
    });
  },

  // 班级选择
  onClassChange(e) {
    const index = parseInt(e.detail.value, 10);
    const selectedClass = this.data.classList[index];
    this.setData({
      classIndex: index,
      'form.class_id': selectedClass.id
    });
  },

  // 保存补全信息
  async handleComplete() {
    const { form } = this.data;
    if (!form.name.trim()) {
      wx.showToast({ title: '请输入姓名', icon: 'none' });
      return;
    }
    if (!form.student_no.trim()) {
      wx.showToast({ title: '请输入学号', icon: 'none' });
      return;
    }

    this.setData({ loading: true });
    try {
      const user = app.globalData.userInfo;
      const res = await request({
        url: `/users/${user.id}`,
        method: 'PUT',
        data: {
          name: form.name.trim(),
          student_no: form.student_no.trim(),
          class_id: form.class_id
        }
      });

      // 更新本地用户信息
      const updatedUser = { ...user, ...res.data, need_complete: false };
      setUserInfo(updatedUser);
      app.globalData.userInfo = updatedUser;

      wx.showToast({ title: '保存成功', icon: 'success' });
      this.goBack();
    } catch (err) {
      console.error('[login] 补全信息失败:', err);
      wx.showToast({ title: err.message || '保存失败', icon: 'none' });
      this.setData({ loading: false });
    }
  },

  // 模拟登录（无后端时前端调试用）
  handleMockLogin() {
    const mockToken = 'mock_token_dev_' + Date.now();
    const mockUser = {
      id: 1,
      name: '测试同学',
      student_no: '20240001',
      role: 'student',
      need_complete: false
    };

    setToken(mockToken);
    setUserInfo(mockUser);
    app.globalData.token = mockToken;
    app.globalData.userInfo = mockUser;
    app.globalData.isLogin = true;

    wx.showToast({ title: '模拟登录成功', icon: 'success' });
    this.goBack();
  },

  // 返回目标页
  goBack() {
    const { redirect } = this.data;
    if (redirect) {
      wx.redirectTo({ url: decodeURIComponent(redirect) });
    } else {
      wx.switchTab({ url: '/pages/index/index' });
    }
  }
});
