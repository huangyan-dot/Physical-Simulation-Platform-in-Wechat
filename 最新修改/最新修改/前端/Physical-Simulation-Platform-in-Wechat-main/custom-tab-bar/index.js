// custom-tab-bar/index.js
Component({
  data: {
    selected: 0,
    color: '#7f8c8d',
    selectedColor: '#3498db',
    list: [
      { pagePath: '/pages/index/index', text: '实验', icon: '📚', iconActive: '📖' },
      { pagePath: '/pages/class/class', text: '班级', icon: '🏫', iconActive: '🏫' },
      { pagePath: '/pages/progress/progress', text: '成绩', icon: '📊', iconActive: '📈' },
      { pagePath: '/pages/profile/profile', text: '我的', icon: '👤', iconActive: '👤' }
    ]
  },

  methods: {
    switchTab(e) {
      const { path, index } = e.currentTarget.dataset;
      this.setData({ selected: index });
      wx.switchTab({ url: path });
    }
  }
});
