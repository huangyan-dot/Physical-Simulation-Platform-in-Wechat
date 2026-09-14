// pages/experiments/thermal-expansion/thermal-expansion.js
import { request } from '../../../utils/request';

const ALPHA_TRUE = 1.7e-5; // 铜
const T0 = 20.0; // 参考温度

Page({
  data: {
    levelId: null,
    initialLengthCM: 50, // cm
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
    q.select('#teCanvas').fields({ node: true, size: true }).exec((res) => {
      if (!res[0]) { console.error('[thermal-expansion] canvas 获取失败'); return; }
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
      if (data.config && data.config.initial_length_m) {
        this.setData({ initialLengthCM: Math.round(data.config.initial_length_m * 100) });
      }
    } catch (err) { console.warn('[thermal-expansion] 后端不可用'); }
    this.generateReadings();
  },

  generateReadings() {
    const L0 = this.data.initialLengthCM / 100; // m
    const temps = [20, 30, 40, 50, 60, 70, 80];
    const readings = temps.map((tc, i) => {
      const deltaT = tc - T0;
      const deltaL_um = ALPHA_TRUE * L0 * deltaT * 1e6; // µm
      const noise = (Math.random() - 0.5) * 0.5;
      return {
        set_num: i + 1,
        temp_c: tc.toFixed(0),
        delta_length_um: (deltaL_um + noise).toFixed(1)
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
    const { canvasWidth: w, canvasHeight: h, pixelRatio: pr } = this.data;
    canvas.width = w * pr; canvas.height = h * pr;
    ctx.setTransform(pr, 0, 0, pr, 0, 0);

    ctx.fillStyle = '#0d1117'; ctx.fillRect(0, 0, w, h);

    // 加热管（水平放置的金属棒 + 加热套）
    const tubeY = h * 0.2;
    const tubeX = w * 0.1, tubeW = w * 0.55, tubeH = 30;

    // 加热套（外层）
    ctx.fillStyle = 'rgba(231, 76, 60, 0.15)';
    ctx.fillRect(tubeX - 5, tubeY - 5, tubeW + 10, tubeH + 10);
    ctx.strokeStyle = 'rgba(231, 76, 60, 0.4)'; ctx.lineWidth = 1.5;
    ctx.strokeRect(tubeX - 5, tubeY - 5, tubeW + 10, tubeH + 10);

    // 金属棒
    const t = Date.now() / 1000;
    const heatPhase = (Math.sin(t * 0.5) + 1) / 2;
    const r = Math.round(180 + heatPhase * 50);
    const g = Math.round(100 + heatPhase * 30);
    const b = Math.round(50);
    ctx.fillStyle = `rgb(${r},${g},${b})`;
    ctx.fillRect(tubeX, tubeY, tubeW, tubeH);
    ctx.strokeStyle = 'rgba(255,255,255,0.3)'; ctx.lineWidth = 1;
    ctx.strokeRect(tubeX, tubeY, tubeW, tubeH);

    // 热波纹
    ctx.strokeStyle = 'rgba(231, 76, 60, 0.3)'; ctx.lineWidth = 1;
    for (let i = 0; i < 5; i++) {
      const wy = tubeY - 10 - i * 6 + Math.sin(t * 2 + i) * 3;
      ctx.beginPath();
      for (let px = tubeX; px < tubeX + tubeW; px += 4) {
        const py = wy + Math.sin((px - tubeX) * 0.1 + t * 3 + i) * 2;
        if (px === tubeX) ctx.moveTo(px, py); else ctx.lineTo(px, py);
      }
      ctx.stroke();
    }

    // 温度计
    const tempX = tubeX + tubeW * 0.5, tempY = tubeY + tubeH + 10;
    ctx.strokeStyle = 'rgba(255,255,255,0.3)'; ctx.lineWidth = 1.5;
    ctx.strokeRect(tempX - 5, tempY, 10, 50);
    const tempFill = ((t * 20) % 60 + 20) / 100; // 20-80°C 模拟
    ctx.fillStyle = '#e74c3c';
    ctx.fillRect(tempX - 4, tempY + 50 - 50 * tempFill, 8, 50 * tempFill);
    ctx.fillStyle = 'rgba(255,255,255,0.5)'; ctx.font = '8px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('T°C', tempX, tempY + 62);

    // 千分表（右侧测量伸长量）
    const dialX = tubeX + tubeW + 20, dialY = tubeY + tubeH / 2;
    ctx.strokeStyle = 'rgba(52, 152, 219, 0.6)'; ctx.lineWidth = 2;
    ctx.beginPath(); ctx.arc(dialX, dialY, 16, 0, Math.PI * 2); ctx.stroke();
    ctx.fillStyle = 'rgba(52, 152, 219, 0.1)';
    ctx.fill();
    // 指针（随热膨胀转动）
    const needleAngle = t * 0.5;
    ctx.strokeStyle = '#f1c40f'; ctx.lineWidth = 1.5;
    ctx.beginPath();
    ctx.moveTo(dialX, dialY);
    ctx.lineTo(dialX + 12 * Math.cos(needleAngle), dialY + 12 * Math.sin(needleAngle));
    ctx.stroke();
    ctx.fillStyle = 'rgba(52,152,219,0.7)'; ctx.font = '8px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('千分表', dialX, dialY + 28);

    // 连杆（金属棒 -> 千分表）
    ctx.strokeStyle = 'rgba(255,255,255,0.3)'; ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(tubeX + tubeW, tubeY + tubeH / 2);
    ctx.lineTo(dialX - 16, dialY);
    ctx.stroke();

    // 下方：ΔL-T 曲线
    const graphX = w * 0.08, graphY = h * 0.52, graphW = w * 0.84, graphH = h * 0.38;
    ctx.strokeStyle = 'rgba(255,255,255,0.2)'; ctx.lineWidth = 1;
    ctx.strokeRect(graphX, graphY, graphW, graphH);

    // 网格
    ctx.strokeStyle = 'rgba(255,255,255,0.06)'; ctx.lineWidth = 0.5;
    for (let i = 1; i < 4; i++) {
      const gy = graphY + (graphH / 4) * i;
      ctx.beginPath(); ctx.moveTo(graphX, gy); ctx.lineTo(graphX + graphW, gy); ctx.stroke();
    }

    // 线性曲线 ΔL = α·L₀·(T-T₀)
    const L0 = this.data.initialLengthCM / 100;
    const maxDeltaL = ALPHA_TRUE * L0 * 60 * 1e6; // µm at 80°C
    ctx.strokeStyle = '#e74c3c'; ctx.lineWidth = 2;
    ctx.beginPath();
    for (let px = 0; px <= graphW; px++) {
      const T = 20 + (px / graphW) * 60; // 20-80°C
      const dL = ALPHA_TRUE * L0 * (T - T0) * 1e6; // µm
      const py = graphY + graphH - (dL / maxDeltaL) * graphH * 0.9;
      if (px === 0) ctx.moveTo(graphX + px, py); else ctx.lineTo(graphX + px, py);
    }
    ctx.stroke();

    // 数据点
    ctx.fillStyle = '#f1c40f';
    for (const r of this.data.readings) {
      const tc = parseFloat(r.temp_c);
      const dL = parseFloat(r.delta_length_um);
      const px = ((tc - 20) / 60) * graphW;
      const py = graphY + graphH - (dL / maxDeltaL) * graphH * 0.9;
      if (px >= 0 && px <= graphW) {
        ctx.beginPath(); ctx.arc(graphX + px, py, 2.5, 0, Math.PI * 2); ctx.fill();
      }
    }

    ctx.fillStyle = 'rgba(255,255,255,0.5)'; ctx.font = '9px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('ΔL - T 曲线', graphX + graphW / 2, graphY - 5);
    ctx.textAlign = 'left';
    ctx.fillText('T (°C)', graphX + graphW - 35, graphY + graphH + 12);
    ctx.fillText('ΔL (µm)', graphX - 5, graphY - 3);
    ctx.textAlign = 'center';
    ctx.fillText(`L₀=${this.data.initialLengthCM}cm, α=1.7×10⁻⁵/°C`, w / 2, h - 5);
  },

  onLengthChange(e) {
    this.setData({ initialLengthCM: e.detail.value });
    this.generateReadings();
  },

  onReadingInput(e) {
    const idx = parseInt(e.currentTarget.dataset.index, 10);
    const field = e.currentTarget.dataset.field;
    this.setData({ [`readings[${idx}].${field}`]: e.detail.value });
  },

  async submitReadings() {
    const { levelId, readings, initialLengthCM } = this.data;
    const payload = readings.map(r => ({
      set_num: r.set_num,
      temp_c: parseFloat(r.temp_c) || 0,
      delta_length_um: parseFloat(r.delta_length_um) || 0
    }));
    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit', method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 20,
          experiment: 'thermal_expansion',
          readings: payload,
          user_config: { initial_length_m: initialLengthCM / 100 }
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
      console.error('[thermal-expansion] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally { this.setData({ submitting: false }); }
  }
});
