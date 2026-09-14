// pages/experiments/diffraction-grating/diffraction-grating.js
import { request } from '../../../utils/request';

const LAMBDA = 589.3e-9; // 钠光波长 (m)

Page({
  data: {
    levelId: null,
    gratingLines: 300, // 线/mm
    gratingConstStr: '3.33',
    readings: [],
    submitting: false,
    canvasWidth: 0, canvasHeight: 0, pixelRatio: 1
  },
  _canvas: null, _ctx: null, _rafId: 0,

  onLoad(options) {
    this.setData({ levelId: options.levelId });
    this.initCanvas();
    if (options.levelId) {
      this.loadExperimentConfig(options.levelId);
    } else {
      this.generateReadings();
    }
  },

  onReady() {
    const query = wx.createSelectorQuery().in(this);
    query.select('#dgCanvas').fields({ node: true, size: true }).exec((res) => {
      if (!res[0]) { console.error('[diffraction-grating] canvas 节点获取失败'); return; }
      this._canvas = res[0].node;
      this._ctx = this._canvas.getContext('2d');
      this.startAnimation();
    });
  },

  onUnload() {
    if (this._rafId && this._canvas) this._canvas.cancelAnimationFrame(this._rafId);
    this._canvas = null; this._ctx = null;
  },

  initCanvas() {
    const sys = wx.getSystemInfoSync();
    const w = sys.windowWidth;
    this.setData({ canvasWidth: w, canvasHeight: Math.min(w, 420), pixelRatio: sys.pixelRatio || 1 });
  },

  async loadExperimentConfig(levelId) {
    try {
      const res = await request({ url: `/experiments/${levelId}`, method: 'GET' });
      const data = res.data || res || {};
      if (data.config && data.config.grating_const_m) {
        const lines = Math.round(1 / (data.config.grating_const_m * 1000));
        this.setData({ gratingLines: lines });
      }
    } catch (err) { console.warn('[diffraction-grating] 后端不可用'); }
    this.generateReadings();
  },

  generateReadings() {
    const d = 1 / (this.data.gratingLines * 1000); // m
    this.setData({ gratingConstStr: (d * 1e6).toFixed(2) });

    const readings = [];
    for (let k = -2; k <= 2; k++) {
      if (k === 0) continue;
      const sinTheta = Math.abs(k) * LAMBDA / d;
      if (sinTheta >= 1) continue; // 超出衍射范围
      const theta = Math.asin(sinTheta);
      readings.push({
        order: k,
        angle_deg: (theta * 180 / Math.PI).toFixed(2),
        angle_rad: theta,
        sin_theta: sinTheta.toFixed(4)
      });
    }
    this.setData({ readings });
  },

  startAnimation() {
    const canvas = this._canvas;
    if (!canvas) return;
    const loop = () => { this.drawScene(); this._rafId = canvas.requestAnimationFrame(loop); };
    loop();
  },

  drawScene() {
    const canvas = this._canvas, ctx = this._ctx;
    if (!canvas || !ctx) return;
    const { canvasWidth: w, canvasHeight: h, pixelRatio: pr, gratingLines } = this.data;
    canvas.width = w * pr; canvas.height = h * pr;
    ctx.setTransform(pr, 0, 0, pr, 0, 0);

    ctx.fillStyle = '#0a0a0a'; ctx.fillRect(0, 0, w, h);

    const cx = w / 2, cy = h * 0.5;

    // 光栅（竖线）
    const gratingX = cx;
    const gratingH = h * 0.4;
    ctx.strokeStyle = 'rgba(100, 149, 237, 0.4)';
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(gratingX, cy - gratingH / 2);
    ctx.lineTo(gratingX, cy + gratingH / 2);
    ctx.stroke();
    // 光栅刻线
    const numLines = Math.min(gratingLines / 50, 20);
    for (let i = 1; i <= numLines; i++) {
      const offset = i * 2;
      ctx.strokeStyle = `rgba(100, 149, 237, ${0.3 - i * 0.01})`;
      ctx.beginPath();
      ctx.moveTo(gratingX + offset, cy - gratingH / 2 + i);
      ctx.lineTo(gratingX + offset, cy + gratingH / 2 - i);
      ctx.stroke();
      ctx.beginPath();
      ctx.moveTo(gratingX - offset, cy - gratingH / 2 + i);
      ctx.lineTo(gratingX - offset, cy + gratingH / 2 - i);
      ctx.stroke();
    }

    // 光源（左侧）
    ctx.fillStyle = '#ffff00';
    ctx.beginPath(); ctx.arc(w * 0.1, cy, 6, 0, Math.PI * 2); ctx.fill();

    // 入射光
    ctx.strokeStyle = 'rgba(255, 255, 0, 0.5)'; ctx.lineWidth = 2;
    ctx.beginPath(); ctx.moveTo(w * 0.1 + 6, cy); ctx.lineTo(gratingX, cy); ctx.stroke();

    // 衍射光路
    const d = 1 / (gratingLines * 1000);
    const colors = { '-2': '#ff6600', '-1': '#ffaa00', '1': '#aaff00', '2': '#00ff66' };
    for (let k = -2; k <= 2; k++) {
      if (k === 0) continue;
      const sinTheta = Math.abs(k) * LAMBDA / d;
      if (sinTheta >= 1) continue;
      const theta = Math.asin(sinTheta);
      const len = w * 0.35;
      const endX = gratingX + Math.sin(theta) * len * Math.sign(k);
      const endY = cy - Math.cos(theta) * len;
      const color = colors[String(k)] || '#ffff00';
      ctx.strokeStyle = color; ctx.lineWidth = 1.5;
      ctx.beginPath(); ctx.moveTo(gratingX, cy); ctx.lineTo(endX, endY); ctx.stroke();

      // 衍射光斑
      ctx.fillStyle = color;
      ctx.beginPath(); ctx.arc(endX, endY, 4, 0, Math.PI * 2); ctx.fill();

      // 级次标注
      ctx.fillStyle = 'rgba(255,255,255,0.6)';
      ctx.font = '10px sans-serif'; ctx.textAlign = 'center';
      ctx.fillText(`k=${k > 0 ? '+' : ''}${k}`, endX, endY - 8);
    }

    // 中央亮纹（k=0）
    ctx.fillStyle = '#ffff00';
    ctx.beginPath(); ctx.arc(gratingX + w * 0.3, cy, 5, 0, Math.PI * 2); ctx.fill();
    ctx.fillStyle = 'rgba(255,255,255,0.6)';
    ctx.font = '10px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('k=0', gratingX + w * 0.3, cy - 8);

    // 光栅标注
    ctx.fillStyle = 'rgba(255,255,255,0.5)'; ctx.font = '10px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('光栅', gratingX, cy + gratingH / 2 + 15);
    ctx.fillText(`${gratingLines} 线/mm`, gratingX, cy + gratingH / 2 + 28);

    // 光源标注
    ctx.textAlign = 'left';
    ctx.fillText('钠光', w * 0.1 - 12, cy - 12);
    ctx.fillText('589.3nm', w * 0.1 - 12, cy + 20);
  },

  onGratingChange(e) {
    this.setData({ gratingLines: e.detail.value });
    this.generateReadings();
  },

  onAngleInput(e) {
    const idx = parseInt(e.currentTarget.dataset.index, 10);
    const deg = parseFloat(e.detail.value) || 0;
    const rad = deg * Math.PI / 180;
    this.setData({
      [`readings[${idx}].angle_deg`]: e.detail.value,
      [`readings[${idx}].angle_rad`]: rad,
      [`readings[${idx}].sin_theta`]: Math.sin(rad).toFixed(4)
    });
  },

  async submitReadings() {
    const { levelId, readings, gratingLines } = this.data;
    const d = 1 / (gratingLines * 1000); // m
    const payload = readings.map(r => ({
      order: r.order,
      angle_rad: parseFloat(r.angle_deg || 0) * Math.PI / 180
    }));
    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit', method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 9,
          experiment: 'diffraction_grating',
          readings: payload,
          user_config: { grating_const_m: d }
        }
      });
      const data = res.data || res || {};
      wx.showModal({
        title: data.passed ? '恭喜过关' : '提交结果',
        content: `得分：${data.score}，最佳：${data.best_score}`,
        showCancel: false,
        success: () => { if (data.passed) wx.navigateBack(); }
      });
    } catch (err) {
      console.error('[diffraction-grating] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally { this.setData({ submitting: false }); }
  }
});
