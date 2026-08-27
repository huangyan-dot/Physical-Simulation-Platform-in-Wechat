// pages/experiments/pn-junction/pn-junction.js
import { request } from '../../../utils/request';

const K_TRUE = 1.381e-23; // 玻尔兹曼常数 J/K
const T = 300.0;           // 环境温度 K
const E_CHARGE = 1.602e-19; // 元电荷 C
const N_FACTOR = 1.0;       // 理想因子
const I0 = 1e-12;           // 反向饱和电流 A

Page({
  data: {
    levelId: null,
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
    q.select('#pnCanvas').fields({ node: true, size: true }).exec((res) => {
      if (!res[0]) { console.error('[pn-junction] canvas 获取失败'); return; }
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
    // 生成 7 组 V-I 数据：V 从 0.30 到 0.60，步进 0.05
    const readings = [];
    for (let i = 0; i < 7; i++) {
      const V = 0.30 + i * 0.05;
      const I_A = I0 * Math.exp(E_CHARGE * V / (N_FACTOR * K_TRUE * T));
      const noise = (Math.random() - 0.5) * I_A * 0.02; // 1% 噪声
      readings.push({
        set_num: i + 1,
        voltage_v: V.toFixed(3),
        current_ua: ((I_A + noise) * 1e6).toFixed(2)
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
    const { canvasWidth: w, canvasHeight: h, pixelRatio: pr, readings } = this.data;
    canvas.width = w * pr; canvas.height = h * pr;
    ctx.setTransform(pr, 0, 0, pr, 0, 0);

    ctx.fillStyle = '#0d1117'; ctx.fillRect(0, 0, w, h);

    // 上半部分：PN 结符号 + 电路
    const cy = h * 0.2;
    ctx.strokeStyle = 'rgba(255,255,255,0.4)'; ctx.lineWidth = 2;

    // 电源
    ctx.fillStyle = '#f1c40f'; ctx.font = 'bold 12px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('V', 30, cy + 5);

    // 连线
    ctx.beginPath();
    ctx.moveTo(30, cy + 10);
    ctx.lineTo(30, cy + 30);
    ctx.lineTo(w * 0.35, cy + 30);
    ctx.stroke();

    // PN 结二极管符号（三角 + 横线）
    const diodeX = w * 0.4, diodeY = cy + 30;
    const dSize = 12;
    ctx.fillStyle = '#3498db';
    ctx.beginPath();
    ctx.moveTo(diodeX, diodeY - dSize);
    ctx.lineTo(diodeX + dSize * 1.5, diodeY);
    ctx.lineTo(diodeX, diodeY + dSize);
    ctx.closePath();
    ctx.fill();
    ctx.strokeStyle = '#3498db'; ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(diodeX + dSize * 1.5, diodeY - dSize);
    ctx.lineTo(diodeX + dSize * 1.5, diodeY + dSize);
    ctx.stroke();

    // P、N 标注
    ctx.fillStyle = 'rgba(255,255,255,0.5)'; ctx.font = '9px sans-serif';
    ctx.fillText('P', diodeX + 5, diodeY - dSize - 5);
    ctx.fillText('N', diodeX + dSize * 1.5 + 5, diodeY - dSize - 5);

    // 右侧连线
    ctx.strokeStyle = 'rgba(255,255,255,0.4)'; ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(diodeX + dSize * 1.5, diodeY);
    ctx.lineTo(w - 30, diodeY);
    ctx.lineTo(w - 30, cy + 5);
    ctx.stroke();
    ctx.fillStyle = '#f1c40f';
    ctx.fillText('A', w - 30, cy + 18); // 电流表

    // 电流方向箭头（闪烁）
    const t = Date.now() / 400;
    const arrowAlpha = (Math.sin(t) + 1) / 2 * 0.5 + 0.3;
    ctx.fillStyle = `rgba(231, 76, 60, ${arrowAlpha})`;
    ctx.font = '10px sans-serif';
    ctx.fillText('→ I', diodeX + dSize * 2 + 10, diodeY + 4);

    // 下半部分：I-V 曲线图 + ln(I)-V 曲线
    const graphX = w * 0.1, graphY = h * 0.42, graphW = w * 0.38, graphH = h * 0.48;
    const graph2X = w * 0.55, graph2Y = h * 0.42, graph2W = w * 0.38, graph2H = h * 0.48;

    // I-V 曲线
    ctx.strokeStyle = 'rgba(255,255,255,0.2)'; ctx.lineWidth = 1;
    ctx.strokeRect(graphX, graphY, graphW, graphH);
    ctx.strokeStyle = '#e74c3c'; ctx.lineWidth = 2;
    ctx.beginPath();
    for (let px = 0; px <= graphW; px++) {
      const V = 0.2 + (px / graphW) * 0.5; // 0.2V - 0.7V
      const I = I0 * Math.exp(E_CHARGE * V / (N_FACTOR * K_TRUE * T));
      // 映射 I 到 graphH（对数缩放）
      const lnI = Math.log(I * 1e6); // µA 的 ln 值，范围约 ln(0.1)~ln(1e6)
      const lnIMin = Math.log(0.1), lnIMax = Math.log(1e6);
      const py = graphY + graphH - ((lnI - lnIMin) / (lnIMax - lnIMin)) * graphH;
      if (px === 0) ctx.moveTo(graphX + px, py); else ctx.lineTo(graphX + px, py);
    }
    ctx.stroke();

    // 绘制数据点
    ctx.fillStyle = '#f1c40f';
    for (const r of readings) {
      const V = parseFloat(r.voltage_v);
      const I_A = parseFloat(r.current_ua) * 1e-6;
      const lnI = Math.log(parseFloat(r.current_ua));
      const lnIMin = Math.log(0.1), lnIMax = Math.log(1e6);
      const px = ((V - 0.2) / 0.5) * graphW;
      const py = graphY + graphH - ((lnI - lnIMin) / (lnIMax - lnIMin)) * graphH;
      ctx.beginPath(); ctx.arc(graphX + px, py, 2.5, 0, Math.PI * 2); ctx.fill();
    }

    ctx.fillStyle = 'rgba(255,255,255,0.5)'; ctx.font = '9px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('I-V 特性曲线', graphX + graphW / 2, graphY - 5);
    ctx.textAlign = 'left';
    ctx.fillText('V (V)', graphX + graphW - 25, graphY + graphH + 12);
    ctx.fillText('I (µA)', graphX - 5, graphY - 3);

    // ln(I)-V 曲线
    ctx.strokeStyle = 'rgba(255,255,255,0.2)'; ctx.lineWidth = 1;
    ctx.strokeRect(graph2X, graph2Y, graph2W, graph2H);
    ctx.strokeStyle = '#3498db'; ctx.lineWidth = 2;
    ctx.beginPath();
    for (let px = 0; px <= graph2W; px++) {
      const V = 0.2 + (px / graph2W) * 0.5;
      const I = I0 * Math.exp(E_CHARGE * V / (N_FACTOR * K_TRUE * T));
      const lnI = Math.log(I * 1e6); // ln(µA)
      const lnIMin = Math.log(0.1), lnIMax = Math.log(1e6);
      const py = graph2Y + graph2H - ((lnI - lnIMin) / (lnIMax - lnIMin)) * graph2H;
      if (px === 0) ctx.moveTo(graph2X + px, py); else ctx.lineTo(graph2X + px, py);
    }
    ctx.stroke();

    // 数据点
    ctx.fillStyle = '#f1c40f';
    for (const r of readings) {
      const V = parseFloat(r.voltage_v);
      const lnI = Math.log(parseFloat(r.current_ua));
      const lnIMin = Math.log(0.1), lnIMax = Math.log(1e6);
      const px = ((V - 0.2) / 0.5) * graph2W;
      const py = graph2Y + graph2H - ((lnI - lnIMin) / (lnIMax - lnIMin)) * graph2H;
      ctx.beginPath(); ctx.arc(graph2X + px, py, 2.5, 0, Math.PI * 2); ctx.fill();
    }

    ctx.fillStyle = 'rgba(255,255,255,0.5)'; ctx.font = '9px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('ln(I)-V 线性回归', graph2X + graph2W / 2, graph2Y - 5);
    ctx.textAlign = 'left';
    ctx.fillText('V (V)', graph2X + graph2W - 25, graph2Y + graph2H + 12);
    ctx.fillText('ln(I)', graph2X - 5, graph2Y - 3);

    // 底部参数
    ctx.fillStyle = 'rgba(255,255,255,0.5)'; ctx.font = '9px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText(`T=${T}K, k=1.381×10⁻²³ J/K`, w / 2, h - 5);
  },

  onReadingInput(e) {
    const idx = parseInt(e.currentTarget.dataset.index, 10);
    const field = e.currentTarget.dataset.field;
    this.setData({ [`readings[${idx}].${field}`]: e.detail.value });
  },

  async submitReadings() {
    const { levelId, readings } = this.data;
    const payload = readings.map(r => ({
      set_num: r.set_num,
      voltage_v: parseFloat(r.voltage_v) || 0,
      current_ua: parseFloat(r.current_ua) || 0
    }));
    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit', method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 15,
          experiment: 'pn_junction',
          readings: payload,
          user_config: {}
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
      console.error('[pn-junction] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally { this.setData({ submitting: false }); }
  }
});
