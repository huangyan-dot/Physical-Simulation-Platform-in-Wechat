// pages/experiments/oil-drop/oil-drop.js
import { request } from '../../../utils/request';

const e_charge = 1.602e-19;
const eta = 1.83e-5;
const rho_oil = 850.0;
const g = 9.8;
const plateDist = 0.006;

Page({
  data: {
    levelId: null,
    voltage: 300,
    chargeStr: '0.00',
    nValue: '-',
    readings: [],
    submitting: false,
    canvasWidth: 0, canvasHeight: 0, pixelRatio: 1
  },
  _canvas: null, _ctx: null, _rafId: 0,

  onLoad(options) {
    this.setData({ levelId: options.levelId });
    this.initCanvas();
    this.generateReadings();
  },

  onReady() {
    const q = wx.createSelectorQuery().in(this);
    q.select('#odCanvas').fields({ node: true, size: true }).exec((res) => {
      if (!res[0]) { console.error('[oil-drop] canvas 获取失败'); return; }
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

  generateReadings() {
    // 生成 4 组理论数据
    const distM = 0.002;
    const readings = [];
    for (let n = 1; n <= 4; n++) {
      const q = n * e_charge;
      const tFall = 10 + n * 3;
      const vTerm = distM / tFall;
      const radius = Math.sqrt(distM / (2 * rho_oil * g * tFall * tFall));
      const V = (6 * Math.PI * eta * radius * vTerm * plateDist) / q;
      readings.push({
        idx: n, voltage: V.toFixed(1), distance: '2.0', time: tFall.toFixed(1)
      });
    }
    this.setData({ readings });
    this.updateCurrent();
  },

  updateCurrent() {
    // 根据当前电压反算一个示意电荷
    const V = this.data.voltage;
    const tFall = 15;
    const distM = 0.002;
    const vTerm = distM / tFall;
    const radius = Math.sqrt(distM / (2 * rho_oil * g * tFall * tFall));
    const q = (6 * Math.PI * eta * radius * vTerm * plateDist) / V;
    const n = Math.round(q / e_charge);
    this.setData({ chargeStr: (q / 1e-19).toFixed(2), nValue: n > 0 ? n : '-' });
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
    const { canvasWidth: w, canvasHeight: h, pixelRatio: pr, voltage } = this.data;
    canvas.width = w * pr; canvas.height = h * pr;
    ctx.setTransform(pr, 0, 0, pr, 0, 0);

    ctx.fillStyle = '#111827'; ctx.fillRect(0, 0, w, h);

    // 平行极板
    const plateW = w * 0.5;
    const plateX = (w - plateW) / 2;
    const topY = h * 0.15, botY = h * 0.7;
    const plateH = 4;

    // 上极板（+）
    ctx.fillStyle = '#e74c3c';
    ctx.fillRect(plateX, topY, plateW, plateH);
    ctx.fillStyle = 'rgba(255,255,255,0.5)';
    ctx.font = '10px sans-serif'; ctx.textAlign = 'left';
    ctx.fillText('+ ' + voltage + 'V', plateX, topY - 5);

    // 下极板（-）
    ctx.fillStyle = '#3498db';
    ctx.fillRect(plateX, botY, plateW, plateH);
    ctx.fillText('−', plateX, botY + 15);

    // 喷雾孔
    ctx.fillStyle = '#7f8c8d';
    ctx.fillRect(plateX + plateW * 0.8, topY - 15, 6, 15);

    // 油滴动画
    const t = Date.now() / 1000;
    const cx = plateX + plateW * 0.5;
    // 油滴悬浮位置（受电压影响）
    const dropY = topY + plateH + (botY - topY - plateH) * (0.5 + 0.3 * Math.sin(t * 0.5));
    const dropR = 6;

    // 油滴
    const grad = ctx.createRadialGradient(cx - 2, dropY - 2, 1, cx, dropY, dropR);
    grad.addColorStop(0, '#f39c12');
    grad.addColorStop(1, '#d35400');
    ctx.fillStyle = grad;
    ctx.beginPath(); ctx.arc(cx, dropY, dropR, 0, Math.PI * 2); ctx.fill();

    // 电场线
    ctx.strokeStyle = 'rgba(231, 76, 60, 0.2)'; ctx.lineWidth = 1;
    ctx.setLineDash([3, 5]);
    for (let i = 1; i < 5; i++) {
      const lx = plateX + plateW * i / 5;
      ctx.beginPath(); ctx.moveTo(lx, topY + plateH); ctx.lineTo(lx, botY); ctx.stroke();
    }
    ctx.setLineDash([]);

    // 观察孔（右侧）
    ctx.strokeStyle = 'rgba(255,255,255,0.3)';
    ctx.beginPath(); ctx.arc(plateX + plateW + 15, (topY + botY) / 2, 8, 0, Math.PI * 2); ctx.stroke();

    // 标注
    ctx.fillStyle = 'rgba(255,255,255,0.5)'; ctx.font = '9px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('密立根油滴仪', w / 2, h - 8);
  },

  onVoltageChange(e) {
    this.setData({ voltage: e.detail.value });
    this.updateCurrent();
  },

  onInput(e) {
    const idx = parseInt(e.currentTarget.dataset.index, 10);
    const field = e.currentTarget.dataset.field;
    this.setData({ [`readings[${idx}].${field}`]: e.detail.value });
  },

  async submitReadings() {
    const { levelId, readings } = this.data;
    const payload = readings.map(r => ({
      voltage_v: parseFloat(r.voltage) || 0,
      distance_m: (parseFloat(r.distance) || 0) * 1e-3,
      time_s: parseFloat(r.time) || 0
    }));
    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit', method: 'POST',
        data: { level_id: parseInt(levelId, 10) || 10, experiment: 'oil_drop', readings: payload }
      });
      const data = res.data || res || {};
      wx.showModal({
        title: data.passed ? '恭喜过关' : '提交结果',
        content: `得分：${data.score}，最佳：${data.best_score}`,
        showCancel: false,
        success: () => { if (data.passed) wx.navigateBack(); }
      });
    } catch (err) {
      console.error('[oil-drop] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally { this.setData({ submitting: false }); }
  }
});
