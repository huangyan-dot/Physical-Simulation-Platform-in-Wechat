// pages/login/login.js
import { wxLogin, wxLoginDev } from '../../utils/auth';
import { request } from '../../utils/request';
import { setToken, setUserInfo } from '../../utils/storage';

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
      console.error('[login] 班级列表加载失败:', err);
      this.setData({ classList: [] });
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

      // 更新本地用户信息（res.data||res：兼容后端返回裸 user 对象或 {data:user}）
      const updatedUser = { ...user, ...(res.data || res), need_complete: false };
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

  // 开发者登录（dev 后门）：直接发 dev_ code 到后端，无需配置微信 appid/secret。
  // 上线前删除此方法及对应 wxml 按钮。
  async handleDevLogin(e) {
    const role = e.currentTarget.dataset.role;
    const codeMap = { student: 'dev_student', teacher: 'dev_teacher', admin: 'dev_admin' };
    const code = codeMap[role] || 'dev_student';

    this.setData({ loading: true });
    try {
      const data = await wxLoginDev(code);
      const { token, user } = data;
      setToken(token);
      setUserInfo(user);
      if (app) {
        app.globalData.token = token;
        app.globalData.userInfo = user;
        app.globalData.isLogin = true;
      }
      wx.showToast({ title: '登录成功', icon: 'success' });
      this.goBack();
    } catch (err) {
      console.error('[login] dev 登录失败:', err);
      wx.showToast({ title: err.message || '登录失败', icon: 'none' });
      this.setData({ loading: false });
    }
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
