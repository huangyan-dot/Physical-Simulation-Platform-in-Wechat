// pages/experiments/oscilloscope/oscilloscope.js
import { request } from '../../../utils/request';

Page({
  data: {
    levelId: null,
    mode: 'CH1',
    modes: ['CH1', 'CH2', '李萨如'],
    ch1: { A: 2.0, f: 50, phi: 0 },
    ch2: { A: 1.5, f: 50, phi: Math.PI / 4 },
    readings: {
      ch1_f: '50',
      ch1_A: '2.0',
      ch2_f: '50',
      ch2_A: '1.5'
    },
    submitting: false,
    canvasWidth: 0,
    canvasHeight: 0,
    pixelRatio: 1
  },

  // 缓存的 canvas 节点
  _canvas: null,
  _ctx: null,
  _rafId: 0,

  onLoad(options) {
    const levelId = options.levelId;
    const assignmentId = options.assignmentId || null;
    this.setData({ levelId, assignmentId });
    this.initCanvas();
    if (levelId) {
      this.loadExperimentConfig(levelId);
    }
  },

  // 加载实验配置（契约 §9：:id 传 level_id）
  async loadExperimentConfig(levelId) {
    try {
      const data = await request({
        url: `/experiments/${levelId}`,
        method: 'GET'
      });
      const channels = (data.config && data.config.channels) || {};
      const patch = {};
      if (channels.CH1) {
        patch.ch1 = { ...this.data.ch1, ...channels.CH1 };
        patch['readings.ch1_f'] = String(channels.CH1.f);
        patch['readings.ch1_A'] = String(channels.CH1.A);
      }
      if (channels.CH2) {
        patch.ch2 = { ...this.data.ch2, ...channels.CH2 };
        patch['readings.ch2_f'] = String(channels.CH2.f);
        patch['readings.ch2_A'] = String(channels.CH2.A);
      }
      if (Object.keys(patch).length) this.setData(patch);
    } catch (err) {
      console.error('[oscilloscope] 加载实验配置失败，使用页面默认参数:', err);
      wx.showToast({ title: '实验参数加载失败，使用默认值', icon: 'none' });
    }
  },

  onReady() {
    // 缓存 canvas 节点
    const query = wx.createSelectorQuery().in(this);
    query.select('#scopeCanvas')
      .fields({ node: true, size: true })
      .exec((res) => {
        if (!res[0]) {
          console.error('[oscilloscope] canvas 节点获取失败');
          return;
        }
        const canvas = res[0].node;
        this._canvas = canvas;
        this._ctx = canvas.getContext('2d');
        this.startAnimation();
      });
  },

  onUnload() {
    if (this._rafId) {
      this._canvas && this._canvas.cancelAnimationFrame(this._rafId);
    }
    this._canvas = null;
    this._ctx = null;
  },

  initCanvas() {
    const sysInfo = wx.getSystemInfoSync();
    const width = sysInfo.windowWidth;
    const height = Math.min(width * 0.6, 420);
    this.setData({
      canvasWidth: width,
      canvasHeight: height,
      pixelRatio: sysInfo.pixelRatio || 1
    });
  },

  // 开始动画循环（使用 canvas.requestAnimationFrame）
  startAnimation() {
    const canvas = this._canvas;
    if (!canvas) return;

    const loop = () => {
      this.drawScope();
      this._rafId = canvas.requestAnimationFrame(loop);
    };
    loop();
  },

  // 绘制示波器屏幕
  drawScope() {
    const canvas = this._canvas;
    const ctx = this._ctx;
    if (!canvas || !ctx) return;

    const { canvasWidth, canvasHeight, pixelRatio, mode } = this.data;
    const t = Date.now() / 1000;

    canvas.width = canvasWidth * pixelRatio;
    canvas.height = canvasHeight * pixelRatio;
    // 用 setTransform 重置变换，避免 scale 累积
    ctx.setTransform(pixelRatio, 0, 0, pixelRatio, 0, 0);

    const w = canvasWidth;
    const h = canvasHeight;

    // 示波器屏幕背景
    ctx.fillStyle = '#0a1a0a';
    ctx.fillRect(0, 0, w, h);

    // 网格
    ctx.strokeStyle = 'rgba(0, 255, 0, 0.15)';
    ctx.lineWidth = 1;
    const gridSize = 40;
    for (let x = 0; x <= w; x += gridSize) {
      ctx.beginPath();
      ctx.moveTo(x, 0);
      ctx.lineTo(x, h);
      ctx.stroke();
    }
    for (let y = 0; y <= h; y += gridSize) {
      ctx.beginPath();
      ctx.moveTo(0, y);
      ctx.lineTo(w, y);
      ctx.stroke();
    }

    // 坐标轴
    ctx.strokeStyle = 'rgba(0, 255, 0, 0.3)';
    ctx.beginPath();
    ctx.moveTo(0, h / 2);
    ctx.lineTo(w, h / 2);
    ctx.stroke();

    if (mode === '李萨如') {
      this.drawLissajous(ctx, w, h, t);
    } else {
      this.drawWaveform(ctx, w, h, t);
    }
  },

  // 绘制时域波形
  drawWaveform(ctx, w, h, t) {
    const { ch1, ch2, mode } = this.data;
    const signal = mode === 'CH1' ? ch1 : ch2;
    const amplitudeScale = h / 8;
    const timeScale = w / 0.04;

    ctx.strokeStyle = '#00ff00';
    ctx.lineWidth = 2;
    ctx.beginPath();

    for (let px = 0; px <= w; px += 2) {
      const time = px / timeScale + t;
      const value = signal.A * Math.sin(2 * Math.PI * signal.f * time + signal.phi);
      const py = h / 2 - value * amplitudeScale;
      if (px === 0) {
        ctx.moveTo(px, py);
      } else {
        ctx.lineTo(px, py);
      }
    }
    ctx.stroke();
  },

  // 绘制李萨如图形
  drawLissajous(ctx, w, h, t) {
    const { ch1, ch2 } = this.data;
    const scale = Math.min(w, h) / 6;
    const duration = 2 / Math.min(ch1.f, ch2.f || 1);
    const step = duration / 400;

    ctx.strokeStyle = '#00ff00';
    ctx.lineWidth = 2;
    ctx.beginPath();

    let first = true;
    for (let time = 0; time <= duration; time += step) {
      const x = ch1.A * Math.sin(2 * Math.PI * ch1.f * time + ch1.phi + t);
      const y = ch2.A * Math.sin(2 * Math.PI * ch2.f * time + ch2.phi + t);
      const px = w / 2 + x * scale;
      const py = h / 2 - y * scale;
      if (first) {
        ctx.moveTo(px, py);
        first = false;
      } else {
        ctx.lineTo(px, py);
      }
    }
    ctx.stroke();
  },

  // 参数输入
  onParamInput(e) {
    const { ch, field } = e.currentTarget.dataset;
    const value = parseFloat(e.detail.value);
    if (isNaN(value)) return;

    this.setData({
      [`${ch}.${field}`]: value
    });

    const readingField = `${ch}_${field}`;
    if (this.data.readings[readingField] !== undefined) {
      this.setData({
        [`readings.${readingField}`]: e.detail.value
      });
    }
  },

  // 切换显示模式
  onModeChange(e) {
    this.setData({ mode: e.currentTarget.dataset.mode });
  },

  onReadingInput(e) {
    const { field } = e.currentTarget.dataset;
    this.setData({
      [`readings.${field}`]: e.detail.value
    });
  },

  // 提交读数
  async submitReadings() {
    const { levelId, assignmentId, readings } = this.data;
    this.setData({ submitting: true });

    try {
      const url = assignmentId ? `/assignments/${assignmentId}/submit` : '/progress/submit';
      const data = await request({
        url,
        method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 2,
          experiment: 'oscilloscope',
          readings: [
            { channel: 'CH1', f: parseFloat(readings.ch1_f) || 0, A: parseFloat(readings.ch1_A) || 0 },
            { channel: 'CH2', f: parseFloat(readings.ch2_f) || 0, A: parseFloat(readings.ch2_A) || 0 }
          ]
        }
      });

      wx.showModal({
        title: data.passed ? '恭喜过关' : '提交结果',
        content: `得分：${data.score}，最佳：${data.best_score}`,
        showCancel: false,
        success: () => {
          if (data.passed) wx.navigateBack();
        }
      });
    } catch (err) {
      console.error('[oscilloscope] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally {
      this.setData({ submitting: false });
    }
  }
});