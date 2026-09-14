// pages/experiments/gas-heat-ratio/gas-heat-ratio.js
import { request } from '../../../utils/request';

const GAMMA_TRUE = 1.40;

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
    q.select('#ghCanvas').fields({ node: true, size: true }).exec((res) => {
      if (!res[0]) { console.error('[gas-heat-ratio] canvas 获取失败'); return; }
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
    // γ = P1/(P1-P2), 取 P1=5.6 kPa -> P2 = P1 - P1/γ = 5.6 - 4.0 = 1.6 kPa
    const p1 = 5.6;
    const p2 = p1 - p1 / GAMMA_TRUE; // 1.6

    const readings = [];
    for (let i = 1; i <= 5; i++) {
      const noise1 = (Math.random() - 0.5) * 0.1;
      const noise2 = (Math.random() - 0.5) * 0.1;
      readings.push({
        set_num: i,
        p1_kpa: (p1 + noise1).toFixed(2),
        p2_kpa: (p2 + noise2).toFixed(2)
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
    const { canvasWidth: w, canvasHeight: h, pixelRatio: pr } = this.data;
    canvas.width = w * pr; canvas.height = h * pr;
    ctx.setTransform(pr, 0, 0, pr, 0, 0);

    ctx.fillStyle = '#0d1117'; ctx.fillRect(0, 0, w, h);

    // 储气瓶
    const bottleX = w * 0.15, bottleY = h * 0.1, bottleW = w * 0.25, bottleH = h * 0.55;
    ctx.strokeStyle = 'rgba(255,255,255,0.4)'; ctx.lineWidth = 2;
    // 瓶身（圆角矩形近似）
    ctx.beginPath();
    ctx.moveTo(bottleX, bottleY + 10);
    ctx.arcTo(bottleX, bottleY, bottleX + 10, bottleY, 10);
    ctx.lineTo(bottleX + bottleW - 10, bottleY);
    ctx.arcTo(bottleX + bottleW, bottleY, bottleX + bottleW, bottleY + 10, 10);
    ctx.lineTo(bottleX + bottleW, bottleY + bottleH - 10);
    ctx.arcTo(bottleX + bottleW, bottleY + bottleH, bottleX + bottleW - 10, bottleY + bottleH, 10);
    ctx.lineTo(bottleX + 10, bottleY + bottleH);
    ctx.arcTo(bottleX, bottleY + bottleH, bottleX, bottleY + bottleH - 10, 10);
    ctx.closePath();
    ctx.stroke();

    // 气体分子（动画）
    const t = Date.now() / 1000;
    ctx.fillStyle = 'rgba(52, 152, 219, 0.5)';
    for (let i = 0; i < 15; i++) {
      const px = bottleX + 15 + ((i * 37 + t * 20) % (bottleW - 30));
      const py = bottleY + 15 + ((i * 53 + t * 15) % (bottleH - 30));
      ctx.beginPath(); ctx.arc(px, py, 2, 0, Math.PI * 2); ctx.fill();
    }

    // 瓶口阀门
    const valveX = bottleX + bottleW / 2;
    const valveY = bottleY - 15;
    ctx.fillStyle = '#e74c3c';
    ctx.fillRect(valveX - 8, valveY, 16, 12);
    ctx.fillStyle = 'rgba(255,255,255,0.5)'; ctx.font = '8px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('阀门', valveX, valveY - 3);

    // 压力表（U型管）
    const gaugeX = bottleX + bottleW + 25, gaugeY = h * 0.15;
    ctx.strokeStyle = 'rgba(255,255,255,0.3)'; ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(gaugeX, gaugeY);
    ctx.lineTo(gaugeX, gaugeY + 60);
    ctx.lineTo(gaugeX + 30, gaugeY + 60);
    ctx.lineTo(gaugeX + 30, gaugeY);
    ctx.stroke();

    // 液柱（动画高度变化）
    const phase = (t % 4) / 4; // 0-1 循环
    let leftH, rightH;
    if (phase < 0.33) {
      // 状态1：P1 高压
      leftH = 45; rightH = 15;
    } else if (phase < 0.66) {
      // 状态2：绝热膨胀后 P0
      leftH = 25; rightH = 25;
    } else {
      // 状态3：平衡后 P2
      leftH = 35; rightH = 20;
    }
    ctx.fillStyle = 'rgba(231, 76, 60, 0.6)';
    ctx.fillRect(gaugeX + 2, gaugeY + 60 - leftH, 10, leftH);
    ctx.fillRect(gaugeX + 18, gaugeY + 60 - rightH, 10, rightH);

    ctx.fillStyle = 'rgba(255,255,255,0.5)'; ctx.font = '8px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('U型管', gaugeX + 15, gaugeY + 75);

    // 三阶段标注
    const labels = ['① 充气 P₁', '② 膨胀 P₀', '③ 平衡 P₂'];
    const colors = ['rgba(231,76,60,0.7)', 'rgba(241,196,15,0.7)', 'rgba(46,204,113,0.7)'];
    const phaseIdx = Math.floor(phase * 3);
    ctx.fillStyle = colors[phaseIdx]; ctx.font = 'bold 10px sans-serif';
    ctx.fillText(labels[phaseIdx], gaugeX + 15, gaugeY - 8);

    // 右侧：P-V 图（绝热过程示意）
    const graphX = w * 0.55, graphY = h * 0.15, graphW = w * 0.38, graphH = h * 0.5;
    ctx.strokeStyle = 'rgba(255,255,255,0.2)'; ctx.lineWidth = 1;
    ctx.strokeRect(graphX, graphY, graphW, graphH);

    // 绝热曲线 PV^γ = const
    ctx.strokeStyle = '#e74c3c'; ctx.lineWidth = 2;
    ctx.beginPath();
    for (let px = 0; px <= graphW; px++) {
      const V = 0.3 + (px / graphW) * 0.7;
      const P = 1.0 / Math.pow(V, GAMMA_TRUE);
      const py = graphY + graphH - ((P - 0.5) / 2.5) * graphH;
      if (px === 0) ctx.moveTo(graphX + px, py); else ctx.lineTo(graphX + px, py);
    }
    ctx.stroke();

    // 等温线（参考）
    ctx.strokeStyle = 'rgba(52, 152, 219, 0.4)'; ctx.lineWidth = 1.5;
    ctx.beginPath();
    for (let px = 0; px <= graphW; px++) {
      const V = 0.3 + (px / graphW) * 0.7;
      const P = 1.0 / V;
      const py = graphY + graphH - ((P - 0.5) / 2.5) * graphH;
      if (px === 0) ctx.moveTo(graphX + px, py); else ctx.lineTo(graphX + px, py);
    }
    ctx.stroke();

    // 状态点
    const state1X = graphX + graphW * 0.25;
    const state1Y = graphY + graphH - ((1.0 / Math.pow(0.475, GAMMA_TRUE) - 0.5) / 2.5) * graphH;
    const state2X = graphX + graphW * 0.6;
    const state2Y = graphY + graphH - ((1.0 / Math.pow(0.72, GAMMA_TRUE) - 0.5) / 2.5) * graphH;
    const state3X = graphX + graphW * 0.55;
    const state3Y = graphY + graphH - ((1.0 / 0.685 - 0.5) / 2.5) * graphH;

    ctx.fillStyle = '#e74c3c'; ctx.beginPath(); ctx.arc(state1X, state1Y, 3, 0, Math.PI * 2); ctx.fill();
    ctx.fillStyle = '#f1c40f'; ctx.beginPath(); ctx.arc(state2X, state2Y, 3, 0, Math.PI * 2); ctx.fill();
    ctx.fillStyle = '#2ecc71'; ctx.beginPath(); ctx.arc(state3X, state3Y, 3, 0, Math.PI * 2); ctx.fill();

    ctx.fillStyle = 'rgba(255,255,255,0.5)'; ctx.font = '8px sans-serif'; ctx.textAlign = 'left';
    ctx.fillText('①', state1X + 5, state1Y - 3);
    ctx.fillText('②', state2X + 5, state2Y - 3);
    ctx.fillText('③', state3X + 5, state3Y - 3);

    ctx.fillStyle = 'rgba(255,255,255,0.5)'; ctx.font = '9px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('P-V 绝热过程', graphX + graphW / 2, graphY - 5);
    ctx.textAlign = 'left';
    ctx.fillText('V', graphX + graphW - 10, graphY + graphH + 12);
    ctx.fillText('P', graphX - 5, graphY - 3);

    // 底部
    ctx.fillStyle = 'rgba(255,255,255,0.5)'; ctx.font = '9px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText(`γ = P₁/(P₁-P₂) = ${GAMMA_TRUE}`, w / 2, h - 5);
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
      p1_kpa: parseFloat(r.p1_kpa) || 0,
      p2_kpa: parseFloat(r.p2_kpa) || 0
    }));
    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit', method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 17,
          experiment: 'gas_heat_ratio',
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
      console.error('[gas-heat-ratio] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally { this.setData({ submitting: false }); }
  }
});
