// pages/experiments/michelson/michelson.js
import { request } from '../../../utils/request';

Page({
  data: {
    levelId: null,
    config: {
      wavelength_nm: 632.8
    },
    mirrorDistUM: 31.64, // μm，对应 N=100 @ 632.8nm
    fringeCount: 100,
    readings: [],
    submitting: false,
    canvasWidth: 0,
    canvasHeight: 0,
    pixelRatio: 1
  },

  _canvas: null,
  _ctx: null,
  _rafId: 0,

  onLoad(options) {
    const levelId = options.levelId;
    this.setData({ levelId });
    this.initCanvas();
    if (levelId) {
      this.loadExperimentConfig(levelId);
    } else {
      this.generateReadings();
    }
  },

  onReady() {
    const query = wx.createSelectorQuery().in(this);
    query.select('#michCanvas')
      .fields({ node: true, size: true })
      .exec((res) => {
        if (!res[0]) {
          console.error('[michelson] canvas 节点获取失败');
          return;
        }
        const canvas = res[0].node;
        const ctx = canvas.getContext('2d');
        this._canvas = canvas;
        this._ctx = ctx;
        this.startAnimation();
      });
  },

  onUnload() {
    if (this._rafId && this._canvas) {
      this._canvas.cancelAnimationFrame(this._rafId);
    }
    this._canvas = null;
    this._ctx = null;
  },

  initCanvas() {
    const sysInfo = wx.getSystemInfoSync();
    const width = sysInfo.windowWidth;
    const height = Math.min(width, 480);
    this.setData({
      canvasWidth: width,
      canvasHeight: height,
      pixelRatio: sysInfo.pixelRatio || 1
    });
  },

  async loadExperimentConfig(levelId) {
    try {
      const res = await request({
        url: `/experiments/${levelId}`,
        method: 'GET'
      });
      const data = res.data || res || {};
      if (data.config && data.config.wavelength_nm) {
        this.setData({ 'config.wavelength_nm': data.config.wavelength_nm });
      }
    } catch (err) {
      console.warn('[michelson] 后端不可用，使用默认配置');
    }
    this.generateReadings();
  },

  // 生成理论读数
  generateReadings() {
    const { wavelength_nm, mirrorDistUM } = this.data;
    // N = 2d / λ
    // d = mirrorDistUM * 1e-6 m, λ = wavelength_nm * 1e-9 m
    const N = Math.round((2 * mirrorDistUM * 1e-6) / (wavelength_nm * 1e-9));
    this.setData({ fringeCount: N });

    // 生成 3 组读数（不同 d 值）
    const multipliers = [0.5, 1.0, 1.5];
    const readings = multipliers.map((m, i) => {
      const d = mirrorDistUM * m;
      const n = Math.round((2 * d * 1e-6) / (wavelength_nm * 1e-9));
      return { idx: i + 1, n: n, mirror_um: d.toFixed(2) };
    });
    this.setData({ readings });
  },

  startAnimation() {
    const canvas = this._canvas;
    if (!canvas) return;
    const loop = () => {
      this.drawScene();
      this._rafId = canvas.requestAnimationFrame(loop);
    };
    loop();
  },

  // 绘制迈克尔逊干涉仪光路 + 干涉条纹
  drawScene() {
    const canvas = this._canvas;
    const ctx = this._ctx;
    if (!canvas || !ctx) return;

    const { canvasWidth, canvasHeight, pixelRatio, config, mirrorDistUM } = this.data;
    canvas.width = canvasWidth * pixelRatio;
    canvas.height = canvasHeight * pixelRatio;
    ctx.setTransform(pixelRatio, 0, 0, pixelRatio, 0, 0);

    const w = canvasWidth;
    const h = canvasHeight;

    // 背景
    ctx.fillStyle = '#0a0a0a';
    ctx.fillRect(0, 0, w, h);

    // ---- 上半部分：光路图 ----
    const lightY = h * 0.12;
    const splitterX = w * 0.4;
    const splitterY = lightY + h * 0.12;

    // 光源
    ctx.fillStyle = '#f1c40f';
    ctx.beginPath();
    ctx.arc(w * 0.1, splitterY, 8, 0, Math.PI * 2);
    ctx.fill();
    ctx.fillStyle = 'rgba(241, 196, 15, 0.8)';
    ctx.font = '10px sans-serif';
    ctx.textAlign = 'center';
    ctx.fillText('激光', w * 0.1, splitterY - 15);

    // 入射光路
    ctx.strokeStyle = 'rgba(241, 196, 15, 0.6)';
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(w * 0.1 + 8, splitterY);
    ctx.lineTo(splitterX - 5, splitterY);
    ctx.stroke();

    // 分束器（45° 线）
    ctx.strokeStyle = '#3498db';
    ctx.lineWidth = 3;
    ctx.beginPath();
    ctx.moveTo(splitterX - 12, splitterY - 12);
    ctx.lineTo(splitterX + 12, splitterY + 12);
    ctx.stroke();
    ctx.fillStyle = 'rgba(52, 152, 219, 0.3)';
    ctx.beginPath();
    ctx.moveTo(splitterX - 12, splitterY - 12);
    ctx.lineTo(splitterX + 12, splitterY + 12);
    ctx.lineTo(splitterX + 8, splitterY + 16);
    ctx.lineTo(splitterX - 16, splitterY - 8);
    ctx.fill();

    // 光路 1：分束器 -> 反射镜 M1（上方）
    const m1Y = lightY + 5;
    ctx.strokeStyle = 'rgba(241, 196, 15, 0.6)';
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(splitterX, splitterY - 12);
    ctx.lineTo(splitterX, m1Y + 5);
    ctx.stroke();
    // M1
    ctx.strokeStyle = '#ecf0f1';
    ctx.lineWidth = 3;
    ctx.beginPath();
    ctx.moveTo(splitterX - 15, m1Y);
    ctx.lineTo(splitterX + 15, m1Y);
    ctx.stroke();
    // M1 标注
    ctx.fillStyle = 'rgba(255,255,255,0.5)';
    ctx.font = '9px sans-serif';
    ctx.fillText('M1(固定)', splitterX, m1Y - 8);

    // 光路 2：分束器 -> 反射镜 M2（右方，可移动）
    const m2X = w * 0.7 + (mirrorDistUM * 0.3); // 随 d 微移
    ctx.strokeStyle = 'rgba(241, 196, 15, 0.6)';
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(splitterX + 12, splitterY);
    ctx.lineTo(m2X - 5, splitterY);
    ctx.stroke();
    // M2
    ctx.strokeStyle = '#ecf0f1';
    ctx.lineWidth = 3;
    ctx.beginPath();
    ctx.moveTo(m2X, splitterY - 15);
    ctx.lineTo(m2X, splitterY + 15);
    ctx.stroke();
    ctx.fillStyle = 'rgba(255,255,255,0.5)';
    ctx.font = '9px sans-serif';
    ctx.textAlign = 'center';
    ctx.fillText('M2(可动)', m2X, splitterY + 28);

    // 返回光路（M1 -> 分束器 -> 观察屏）
    ctx.strokeStyle = 'rgba(231, 76, 60, 0.4)';
    ctx.lineWidth = 1.5;
    ctx.setLineDash([4, 4]);
    // M1 返回
    ctx.beginPath();
    ctx.moveTo(splitterX, m1Y + 5);
    ctx.lineTo(splitterX, splitterY);
    ctx.stroke();
    // M2 返回
    ctx.beginPath();
    ctx.moveTo(m2X - 5, splitterY);
    ctx.lineTo(splitterX + 12, splitterY);
    ctx.stroke();
    // 到观察屏
    const screenX = splitterX;
    const screenY = h * 0.45;
    ctx.beginPath();
    ctx.moveTo(splitterX, splitterY + 12);
    ctx.lineTo(screenX, screenY - 10);
    ctx.stroke();
    ctx.setLineDash([]);

    // ---- 下半部分：干涉条纹（同心圆环）----
    const ringCX = w / 2;
    const ringCY = h * 0.72;
    const maxR = Math.min(w, h) * 0.22;

    // 条纹随时间动画（条纹冒出/陷入）
    const t = Date.now() / 1000;
    const phase = (t * 0.5) % 1; // 0~1 循环

    // 波长 -> 颜色
    const lambda = config.wavelength_nm;
    let color;
    if (lambda < 450) color = '#4040ff';
    else if (lambda < 500) color = '#00aaff';
    else if (lambda < 570) color = '#00ff00';
    else if (lambda < 590) color = '#ffff00';
    else if (lambda < 620) color = '#ff8800';
    else color = '#ff0000';

    // 绘制同心干涉环
    const numRings = 15;
    for (let i = 0; i < numRings; i++) {
      const r = maxR * (i + phase) / numRings;
      if (r < 2) continue;
      const alpha = Math.max(0.1, 0.9 - i * 0.05);
      ctx.strokeStyle = color;
      ctx.globalAlpha = alpha;
      ctx.lineWidth = 2;
      ctx.beginPath();
      ctx.arc(ringCX, ringCY, r, 0, Math.PI * 2);
      ctx.stroke();
    }
    ctx.globalAlpha = 1;

    // 中心点
    ctx.fillStyle = color;
    ctx.globalAlpha = 0.8;
    ctx.beginPath();
    ctx.arc(ringCX, ringCY, 3, 0, Math.PI * 2);
    ctx.fill();
    ctx.globalAlpha = 1;

    // 标注
    ctx.fillStyle = 'rgba(255, 255, 255, 0.5)';
    ctx.font = '10px sans-serif';
    ctx.textAlign = 'left';
    ctx.fillText('分束器', splitterX + 15, splitterY - 15);
    ctx.fillText('观察屏', screenX - 30, screenY);
    ctx.textAlign = 'center';
    ctx.fillText(`λ = ${config.wavelength_nm} nm`, ringCX, ringCY + maxR + 18);
  },

  onWavelengthChange(e) {
    this.setData({ 'config.wavelength_nm': e.detail.value });
    this.generateReadings();
  },

  onMirrorDistChange(e) {
    this.setData({ mirrorDistUM: e.detail.value });
    this.generateReadings();
  },

  onNInput(e) {
    const index = parseInt(e.currentTarget.dataset.index, 10);
    this.setData({
      [`readings[${index}].n`]: parseInt(e.detail.value, 10) || 0
    });
  },

  onMirrorInput(e) {
    const index = parseInt(e.currentTarget.dataset.index, 10);
    this.setData({
      [`readings[${index}].mirror_um`]: e.detail.value
    });
  },

  async submitReadings() {
    const { levelId, readings, config } = this.data;
    const payload = readings.map(r => ({
      n: parseInt(r.n, 10) || 0,
      mirror_m: (parseFloat(r.mirror_um) || 0) * 1e-6 // μm -> m
    }));

    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit',
        method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 6,
          experiment: 'michelson',
          readings: payload,
          user_config: {
            wavelength_nm: config.wavelength_nm
          }
        }
      });

      const data = res.data || res || {};
      wx.showModal({
        title: data.passed ? '恭喜过关' : '提交结果',
        content: `得分：${data.score}，最佳：${data.best_score}`,
        showCancel: false,
        success: () => {
          if (data.passed) wx.navigateBack();
        }
      });
    } catch (err) {
      console.error('[michelson] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally {
      this.setData({ submitting: false });
    }
  }
});
