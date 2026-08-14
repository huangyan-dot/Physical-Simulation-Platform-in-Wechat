// pages/profile/profile.js
import { logout } from '../../utils/auth';

const app = getApp();

Page({
  data: {
    userInfo: {},
    roleText: {
      student: '学生',
      teacher: '教师',
      admin: '管理员'
    }
  },

  onShow() {
    if (typeof this.getTabBar === 'function' && this.getTabBar()) {
      this.getTabBar().setData({ selected: 3 });
    }
    this.setData({
      userInfo: app.globalData.userInfo || {}
    });
  },

  goToProgress() {
    wx.navigateTo({ url: '/pages/progress/progress' });
  },

  goToClass() {
    wx.switchTab({ url: '/pages/class/class' });
  },

  showAbout() {
    wx.showModal({
      title: '关于',
      content: '大学生物理实验 3D 模拟微信小程序\n版本：v1.0.0',
      showCancel: false
    });
  },

  async handleLogout() {
    const res = await wx.showModal({
      title: '确认退出',
      content: '退出后需要重新登录',
      confirmColor: '#e74c3c'
    });
    if (res.confirm) {
      logout();
    }
  }
});
