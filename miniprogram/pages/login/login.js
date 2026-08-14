// pages/login/login.js
import { wxLogin } from '../../utils/auth';
import { request } from '../../utils/request';
import { setToken, setUserInfo } from '../../utils/storage';

const app = getApp();

Page({
  data: {
    mode: 'login', // login | complete
    role: 'student', // student | teacher
    inviteCode: '',
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

  // 选择角色
  onRoleChange(e) {
    this.setData({ role: e.currentTarget.dataset.role });
  },

  // 邀请码输入
  onInviteInput(e) {
    this.setData({ inviteCode: e.detail.value });
  },

  // 微信登录（带角色）
  async handleLogin() {
    if (this.data.loading) return;
    const { role, inviteCode } = this.data;

    if (role === 'teacher' && !inviteCode.trim()) {
      wx.showToast({ title: '请填写教师邀请码', icon: 'none' });
      return;
    }

    this.setData({ loading: true });

    try {
      const data = await wxLogin({
        role,
        inviteCode: inviteCode.trim()
      });
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
        classList: Array.isArray(res) ? res : []
      });
    } catch (err) {
      console.error('[login] 加载班级失败:', err);
      this.setData({ classList: [] });
      wx.showToast({ title: '班级列表加载失败，可稍后在个人页补充', icon: 'none' });
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
      const updated = await request({
        url: `/users/${user.id}`,
        method: 'PUT',
        data: {
          name: form.name.trim(),
          student_no: form.student_no.trim(),
          class_id: form.class_id
        }
      });

      const updatedUser = { ...user, ...updated, need_complete: false };
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

  // 开发登录：走后端开发后门，需后端可用
  async handleMockLogin() {
    if (this.data.loading) return;
    const { role, inviteCode } = this.data;
    const devMap = { student: 'dev_student', teacher: 'dev_teacher', admin: 'dev_admin' };
    const devCode = devMap[role] || 'dev_student';

    this.setData({ loading: true });

    try {
      const res = await request({
        url: '/auth/login',
        method: 'POST',
        needAuth: false,
        data: { code: devCode, role, invite_code: inviteCode.trim() },
        showLoading: false
      });

      const { token, user } = res;
      setToken(token);
      setUserInfo(user);
      app.globalData.token = token;
      app.globalData.userInfo = user;
      app.globalData.isLogin = true;

      wx.showToast({ title: '开发登录成功', icon: 'success' });
      this.setData({ loading: false });
      this.goBack();
    } catch (err) {
      console.error('[login] 后端开发后门不可用:', err);
      this.setData({ loading: false });
      wx.showModal({
        title: '无法连接后端',
        content: `${err.message || '请求失败'}\n\n请确认后端已启动（start_backend.bat），且 utils/config.js 中 apiBaseUrl 指向后端地址。`,
        showCancel: false
      });
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
