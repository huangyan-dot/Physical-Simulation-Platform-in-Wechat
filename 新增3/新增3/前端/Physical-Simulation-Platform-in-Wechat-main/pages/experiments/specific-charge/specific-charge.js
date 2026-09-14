// pages/experiments/specific-charge/specific-charge.js
import { request } from '../../../utils/request';

const EM_TRUE = 1.759e11; // C/kg

Page({
  data: {
    levelId: null,
    bFieldMT: 5, // mT
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
    const q = wx.createSelectorQuery().in(this);
    q.select('#scCanvas').fields({ node: true, size: true }).exec((res) => {
      if (!res[0]) { console.error('[specific-charge] canvas 获取失败'); return; }
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
      if (data.config && data.config.b_field_t) {
        this.setData({ bFieldMT: Math.round(data.config.b_field_t * 1000) });
      }
    } catch (err) { console.warn('[specific-charge] 后端不可用'); }
    this.generateReadings();
  },

  generateReadings() {
    const B = this.data.bFieldMT * 1e-3; // T
    // e/m = 2V/(B²r²), 取 V=150,200,250,300,350
    const voltages = [150, 200, 250, 300, 350];
    const readings = voltages.map((V, i) => {
      const r_m = Math.sqrt(2.0 * V / (B * B * EM_TRUE));
      const r_mm = r_m * 1e3;
      const noise = (Math.random() - 0.5) * 0.5;
      return {
        set_num: i + 1,
        voltage_v: V.toFixed(0),
        radius_mm: (r_mm + noise).toFixed(2)
      };
    });
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
    const { canvasWidth: w, canvasHeight: h, pixelRatio: pr, bFieldMT } = this.data;
    canvas.width = w * pr; canvas.height = h * pr;
    ctx.setTransform(pr, 0, 0, pr, 0, 0);

    ctx.fillStyle = '#0d1117'; ctx.fillRect(0, 0, w, h);

    // 真空管（圆形区域）
    const cx = w / 2, cy = h * 0.3;
    const tubeR = Math.min(w, h) * 0.22;

    // 管壁
    ctx.strokeStyle = 'rgba(255,255,255,0.3)'; ctx.lineWidth = 2;
    ctx.beginPath(); ctx.arc(cx, cy, tubeR, 0, Math.PI * 2); ctx.stroke();
    ctx.fillStyle = 'rgba(30, 30, 50, 0.3)';
    ctx.fill();

    // 电子枪（左侧）
    const gunX = cx - tubeR * 0.6;
    ctx.fillStyle = '#f1c40f';
    ctx.fillRect(gunX - 8, cy - 3, 8, 6);
    ctx.fillStyle = 'rgba(241,196,15,0.7)'; ctx.font = '8px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('K', gunX - 4, cy - 8);

    // 电子束（圆弧轨迹，动画）
    const t = Date.now() / 1000;
    const B = bFieldMT * 1e-3;
    const V = 200; // 显示用参考电压
    const r_m = Math.sqrt(2.0 * V / (B * B * EM_TRUE));
    // 映射到像素半径
    const r_px = tubeR * 0.55;

    // 圆弧轨迹（电子束偏转路径）
    ctx.strokeStyle = '#2ecc71'; ctx.lineWidth = 2;
    ctx.beginPath();
    // 电子从枪口水平射出，在磁场中偏转成圆弧
    const startAngle = 0;
    const endAngle = Math.PI * 0.55;
    ctx.arc(gunX, cy, r_px, startAngle, endAngle);
    ctx.stroke();

    // 运动中的电子（沿圆弧移动）
    const ePhase = (t * 0.8) % 1;
    const eAngle = startAngle + ePhase * (endAngle - startAngle);
    const ex = gunX + r_px * Math.cos(eAngle);
    const ey = cy + r_px * Math.sin(eAngle);

    // 电子尾迹
    ctx.strokeStyle = 'rgba(46, 204, 113, 0.3)'; ctx.lineWidth = 1;
    for (let i = 1; i <= 5; i++) {
      const trailAngle = eAngle - i * 0.08;
      if (trailAngle < startAngle) break;
      ctx.beginPath();
      ctx.arc(gunX, cy, r_px, trailAngle, eAngle - (i - 1) * 0.08);
      ctx.stroke();
    }

    // 电子粒子
    ctx.fillStyle = '#2ecc71';
    ctx.beginPath(); ctx.arc(ex, ey, 4, 0, Math.PI * 2); ctx.fill();
    // 光晕
    ctx.fillStyle = 'rgba(46, 204, 113, 0.3)';
    ctx.beginPath(); ctx.arc(ex, ey, 8, 0, Math.PI * 2); ctx.fill();

    // 磁场标识（×表示进入纸面）
    ctx.fillStyle = 'rgba(52, 152, 219, 0.4)'; ctx.font = '14px sans-serif';
    for (let i = -2; i <= 2; i++) {
      for (let j = -1; j <= 1; j++) {
        if (i === 0 && j === 0) continue;
        const mx = cx + i * 35, my = cy + j * 35;
        if (Math.sqrt((mx - cx) ** 2 + (my - cy) ** 2) < tubeR - 15) {
          ctx.fillText('×', mx, my + 5);
        }
      }
    }
    ctx.fillStyle = 'rgba(52, 152, 219, 0.6)'; ctx.font = '8px sans-serif';
    ctx.fillText('B', cx + tubeR * 0.5, cy - tubeR * 0.5);

    // 半径标注
    ctx.strokeStyle = 'rgba(241, 196, 15, 0.5)'; ctx.lineWidth = 1;
    ctx.setLineDash([3, 3]);
    ctx.beginPath();
    ctx.moveTo(gunX, cy);
    ctx.lineTo(gunX + r_px, cy);
    ctx.stroke();
    ctx.setLineDash([]);
    ctx.fillStyle = 'rgba(241, 196, 15, 0.7)'; ctx.font = '9px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText(`r`, gunX + r_px / 2, cy - 5);

    // 下半部分：r²-V 线性图
    const graphX = w * 0.08, graphY = h * 0.58, graphW = w * 0.84, graphH = h * 0.34;
    ctx.strokeStyle = 'rgba(255,255,255,0.2)'; ctx.lineWidth = 1;
    ctx.strokeRect(graphX, graphY, graphW, graphH);

    // r² ∝ V 线性关系
    ctx.strokeStyle = '#e74c3c'; ctx.lineWidth = 2;
    ctx.beginPath();
    const maxV = 400, maxR2 = 2.0 * maxV / (B * B * EM_TRUE) * 1e6; // mm²
    for (let px = 0; px <= graphW; px++) {
      const V = (px / graphW) * maxV;
      const r2 = 2.0 * V / (B * B * EM_TRUE) * 1e6; // mm²
      const py = graphY + graphH - (r2 / maxR2) * graphH * 0.9;
      if (px === 0) ctx.moveTo(graphX + px, py); else ctx.lineTo(graphX + px, py);
    }
    ctx.stroke();

    // 数据点
    ctx.fillStyle = '#f1c40f';
    for (const r of this.data.readings) {
      const V = parseFloat(r.voltage_v);
      const r_mm = parseFloat(r.radius_mm);
      const r2 = r_mm * r_mm;
      const px = (V / maxV) * graphW;
      const py = graphY + graphH - (r2 / maxR2) * graphH * 0.9;
      if (px >= 0 && px <= graphW) {
        ctx.beginPath(); ctx.arc(graphX + px, py, 2.5, 0, Math.PI * 2); ctx.fill();
      }
    }

    ctx.fillStyle = 'rgba(255,255,255,0.5)'; ctx.font = '9px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('r² - V 线性关系', graphX + graphW / 2, graphY - 5);
    ctx.textAlign = 'left';
    ctx.fillText('V (V)', graphX + graphW - 30, graphY + graphH + 12);
    ctx.fillText('r² (mm²)', graphX - 5, graphY - 3);
    ctx.textAlign = 'center';
    ctx.fillText(`B=${bFieldMT}mT, e/m=1.759×10¹¹ C/kg`, w / 2, h - 5);
  },

  onBFieldChange(e) {
    this.setData({ bFieldMT: e.detail.value });
    this.generateReadings();
  },

  onReadingInput(e) {
    const idx = parseInt(e.currentTarget.dataset.index, 10);
    const field = e.currentTarget.dataset.field;
    this.setData({ [`readings[${idx}].${field}`]: e.detail.value });
  },

  async submitReadings() {
    const { levelId, readings, bFieldMT } = this.data;
    const B = bFieldMT * 1e-3; // T
    const payload = readings.map(r => ({
      set_num: r.set_num,
      voltage_v: parseFloat(r.voltage_v) || 0,
      radius_mm: parseFloat(r.radius_mm) || 0
    }));
    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit', method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 21,
          experiment: 'specific_charge',
          readings: payload,
          user_config: { b_field_t: B }
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
      console.error('[specific-charge] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally { this.setData({ submitting: false }); }
  }
});
