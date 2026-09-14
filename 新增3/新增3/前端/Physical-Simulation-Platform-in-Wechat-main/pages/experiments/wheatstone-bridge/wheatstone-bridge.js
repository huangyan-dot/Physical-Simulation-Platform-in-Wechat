// pages/experiments/wheatstone-bridge/wheatstone-bridge.js
import { request } from '../../../utils/request';

// 比率臂档位（对应电阻箱常见倍率）
const RATIO_OPTIONS = [0.1, 1, 10, 100, 1000];

Page({
  data: {
    levelId: null,
    ratioIndex: 1,       // 默认 R1/R2 = 1
    ratioDisplay: '1',
    r3Ohm: 500,
    rxTheoretical: '500.0',
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
    q.select('#wbCanvas').fields({ node: true, size: true }).exec((res) => {
      if (!res[0]) { console.error('[wheatstone-bridge] canvas 获取失败'); return; }
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
      if (data.config && data.config.ratio_r1_r2) {
        const idx = RATIO_OPTIONS.indexOf(data.config.ratio_r1_r2);
        if (idx >= 0) {
          this.setData({ ratioIndex: idx, ratioDisplay: String(RATIO_OPTIONS[idx]) });
        }
      }
    } catch (err) { console.warn('[wheatstone-bridge] 后端不可用'); }
    this.generateReadings();
  },

  generateReadings() {
    const ratio = RATIO_OPTIONS[this.data.ratioIndex];
    const r3 = this.data.r3Ohm;
    const rx = ratio * r3;
    this.setData({
      ratioDisplay: String(ratio),
      rxTheoretical: rx.toFixed(1)
    });

    const readings = [];
    for (let i = 1; i <= 5; i++) {
      // 理论值 + 微小扰动模拟测量
      const noise = (Math.random() - 0.5) * 2; // ±1 Ω
      readings.push({
        set_num: i,
        r3_ohm: String(r3),
        rx_ohm: (rx + noise).toFixed(1)
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
    const { canvasWidth: w, canvasHeight: h, pixelRatio: pr, ratioIndex } = this.data;
    canvas.width = w * pr; canvas.height = h * pr;
    ctx.setTransform(pr, 0, 0, pr, 0, 0);

    ctx.fillStyle = '#0d1117'; ctx.fillRect(0, 0, w, h);

    const cx = w / 2, cy = h * 0.38;
    const size = Math.min(w, h) * 0.3;

    // 菱形电桥电路
    // 四个顶点：上(A)，右(B)，下(C)，左(D)
    const ax = cx, ay = cy - size;
    const bx = cx + size, by = cy;
    const dx = cx - size, dy = cy;
    const ex_cx = cx, ey_cy = cy + size;

    // 电源（上端 A -> 下端 C，经过 B 和 D）
    ctx.strokeStyle = 'rgba(255,255,255,0.4)'; ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(ax, ay); ctx.lineTo(bx, by); // A->B (R1)
    ctx.lineTo(ex_cx, ey_cy);               // B->C (R3)
    ctx.moveTo(ax, ay); ctx.lineTo(dx, dy); // A->D (R2)
    ctx.lineTo(ex_cx, ey_cy);               // D->C (Rx)
    ctx.stroke();

    // R1 标签
    ctx.fillStyle = '#e74c3c'; ctx.font = 'bold 12px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('R1', (ax + bx) / 2 + 12, (ay + by) / 2 - 5);
    // R2 标签
    ctx.fillText('R2', (ax + dx) / 2 - 12, (ay + dy) / 2 - 5);
    // R3 标签
    ctx.fillStyle = '#3498db';
    ctx.fillText('R3', (bx + ex_cx) / 2 + 12, (by + ey_cy) / 2 + 5);
    // Rx 标签
    ctx.fillText('Rx', (dx + ex_cx) / 2 - 12, (dy + ey_cy) / 2 + 5);

    // 检流计 G（B-D 连线中间）
    const gx = (bx + dx) / 2, gy = (by + dy) / 2;
    ctx.strokeStyle = 'rgba(46, 204, 113, 0.6)'; ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(bx, by); ctx.lineTo(gx, gy); ctx.lineTo(dx, dy);
    ctx.stroke();
    ctx.fillStyle = '#2ecc71';
    ctx.beginPath(); ctx.arc(gx, gy, 10, 0, Math.PI * 2); ctx.fill();
    ctx.fillStyle = '#fff'; ctx.font = 'bold 10px sans-serif';
    ctx.fillText('G', gx, gy + 4);

    // 电源符号
    ctx.fillStyle = '#f1c40f'; ctx.font = 'bold 14px sans-serif';
    ctx.fillText('E', ax, ay - 12);

    // 平衡指示（闪烁动画）
    const t = Date.now() / 500;
    const balanced = Math.sin(t) > 0;
    ctx.fillStyle = balanced ? 'rgba(46, 204, 113, 0.8)' : 'rgba(231, 76, 60, 0.4)';
    ctx.font = '11px sans-serif'; ctx.textAlign = 'left';
    ctx.fillText(balanced ? '● 电桥平衡' : '○ 调节中...', 16, h - 40);

    // 比率显示
    const ratio = RATIO_OPTIONS[ratioIndex];
    ctx.fillStyle = 'rgba(255,255,255,0.6)'; ctx.font = '11px sans-serif';
    ctx.fillText(`R1/R2 = ${ratio}`, 16, h - 24);
    ctx.textAlign = 'right';
    ctx.fillText(`Rx = ${ratio} × R3`, w - 16, h - 24);
  },

  onRatioChange(e) {
    const idx = parseInt(e.detail.value, 10);
    this.setData({ ratioIndex: idx });
    this.generateReadings();
  },

  onReadingInput(e) {
    const idx = parseInt(e.currentTarget.dataset.index, 10);
    const field = e.currentTarget.dataset.field;
    this.setData({ [`readings[${idx}].${field}`]: e.detail.value });
  },

  async submitReadings() {
    const { levelId, readings, ratioIndex } = this.data;
    const ratio = RATIO_OPTIONS[ratioIndex];
    const payload = readings.map(r => ({
      set_num: r.set_num,
      known_r3_ohm: parseFloat(r.r3_ohm) || 0,
      rx_ohm: parseFloat(r.rx_ohm) || 0
    }));
    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit', method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 13,
          experiment: 'wheatstone_bridge',
          readings: payload,
          user_config: { ratio_r1_r2: ratio }
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
      console.error('[wheatstone-bridge] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally { this.setData({ submitting: false }); }
  }
});
