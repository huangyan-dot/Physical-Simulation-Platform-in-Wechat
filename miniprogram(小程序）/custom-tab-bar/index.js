// custom-tab-bar/index.js
Component({
  data: {
    selected: 0,
    color: '#7f8c8d',
    selectedColor: '#3498db',
    list: []
  },

  lifetimes: {
    attached() {
      this.updateTabsByRole();
    }
  },

  pageLifetimes: {
    show() {
      this.updateTabsByRole();
    }
  },

  methods: {
    updateTabsByRole() {
      const app = getApp();
      const role = (app && app.globalData && app.globalData.userInfo && app.globalData.userInfo.role) || 'student';
      const studentTabs = [
        { pagePath: '/pages/index/index', text: '实验', icon: '📚', iconActive: '📖' },
        { pagePath: '/pages/class/class', text: '班级', icon: '🏫', iconActive: '🏫' },
        { pagePath: '/pages/assignment/list/list', text: '作业', icon: '📝', iconActive: '📋' },
        { pagePath: '/pages/profile/profile', text: '我的', icon: '👤', iconActive: '👤' }
      ];
      const teacherTabs = [
        { pagePath: '/pages/index/index', text: '实验', icon: '📚', iconActive: '📖' },
        { pagePath: '/pages/class/class', text: '班级管理', icon: '🏫', iconActive: '🏫' },
        { pagePath: '/pages/assignment/list/list', text: '作业管理', icon: '📝', iconActive: '📋' },
        { pagePath: '/pages/profile/profile', text: '我的', icon: '👤', iconActive: '👤' }
      ];
      this.setData({ list: role === 'teacher' ? teacherTabs : studentTabs });
    },

    switchTab(e) {
      const { path, index } = e.currentTarget.dataset;
      this.setData({ selected: index });
      wx.switchTab({ url: path });
    }
  }
});
