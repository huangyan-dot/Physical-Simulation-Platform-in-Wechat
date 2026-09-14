// pages/experiments/falling-ball/falling-ball.js
import { request } from '../../../utils/request';

const ETA_TRUE = 0.985;   // 蓖麻油粘滞系数 Pa·s
const LIQ_RHO = 960;      // 液体密度 kg/m³
const G = 9.8;

Page({
  data: {
    levelId: null,
    ballDensity: 7800,
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
    q.select('#fbCanvas').fields({ node: true, size: true }).exec((res) => {
      if (!res[0]) { console.error('[falling-ball] canvas 获取失败'); return; }
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
      if (data.config && data.config.ball_density) {
        this.setData({ ballDensity: data.config.ball_density });
      }
    } catch (err) { console.warn('[falling-ball] 后端不可用'); }
    this.generateReadings();
  },

  generateReadings() {
    const ballRho = this.data.ballDensity;
    const diameterMM = 2.0;
    const distanceM = 0.2;
    const r = diameterMM / 2.0 * 1e-3;
    const v = (2.0 / 9.0) * r * r * (ballRho - LIQ_RHO) * G / ETA_TRUE;
    const timeS = distanceM / v;

    const readings = [];
    for (let i = 1; i <= 5; i++) {
      const noise = (Math.random() - 0.5) * 0.3;
      readings.push({
        set_num: i,
        ball_diameter_mm: diameterMM.toFixed(2),
        distance_m: distanceM.toFixed(3),
        time_s: (timeS + noise).toFixed(2)
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
    const { canvasWidth: w, canvasHeight: h, pixelRatio: pr, ballDensity } = this.data;
    canvas.width = w * pr; canvas.height = h * pr;
    ctx.setTransform(pr, 0, 0, pr, 0, 0);

    ctx.fillStyle = '#0d1117'; ctx.fillRect(0, 0, w, h);

    // 量筒（盛放液体）
    const tubeX = w * 0.25, tubeY = h * 0.1, tubeW = w * 0.2, tubeH = h * 0.55;
    ctx.strokeStyle = 'rgba(255,255,255,0.4)'; ctx.lineWidth = 2;
    ctx.strokeRect(tubeX, tubeY, tubeW, tubeH);

    // 液体
    ctx.fillStyle = 'rgba(218, 165, 32, 0.2)';
    ctx.fillRect(tubeX + 2, tubeY + tubeH * 0.15, tubeW - 4, tubeH * 0.82);

    // 小球下落动画
    const t = Date.now() / 1000;
    const cycle = t % 3; // 3 秒一个周期
    const ballProgress = cycle / 3;
    const ballY = tubeY + tubeH * 0.15 + ballProgress * tubeH * 0.75;
    const ballX = tubeX + tubeW / 2;
    const ballR = 6;

    // 小球轨迹
    ctx.strokeStyle = 'rgba(231, 76, 60, 0.2)'; ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(ballX, tubeY + tubeH * 0.15);
    ctx.lineTo(ballX, ballY);
    ctx.stroke();

    // 小球
    ctx.fillStyle = '#e74c3c';
    ctx.beginPath(); ctx.arc(ballX, ballY, ballR, 0, Math.PI * 2); ctx.fill();

    // 标尺（下落距离标注）
    const markX = tubeX + tubeW + 10;
    ctx.strokeStyle = 'rgba(241, 196, 15, 0.5)'; ctx.lineWidth = 1;
    ctx.setLineDash([3, 3]);
    ctx.beginPath();
    ctx.moveTo(tubeX + tubeW, tubeY + tubeH * 0.15);
    ctx.lineTo(markX + 20, tubeY + tubeH * 0.15);
    ctx.moveTo(tubeX + tubeW, tubeY + tubeH * 0.9);
    ctx.lineTo(markX + 20, tubeY + tubeH * 0.9);
    ctx.stroke();
    ctx.setLineDash([]);

    ctx.strokeStyle = 'rgba(241, 196, 15, 0.7)'; ctx.lineWidth = 1.5;
    ctx.beginPath();
    ctx.moveTo(markX + 15, tubeY + tubeH * 0.15);
    ctx.lineTo(markX + 15, tubeY + tubeH * 0.9);
    ctx.stroke();
    // 箭头
    ctx.beginPath();
    ctx.moveTo(markX + 12, tubeY + tubeH * 0.15 + 6);
    ctx.lineTo(markX + 15, tubeY + tubeH * 0.15);
    ctx.lineTo(markX + 18, tubeY + tubeH * 0.15 + 6);
    ctx.stroke();
    ctx.beginPath();
    ctx.moveTo(markX + 12, tubeY + tubeH * 0.9 - 6);
    ctx.lineTo(markX + 15, tubeY + tubeH * 0.9);
    ctx.lineTo(markX + 18, tubeY + tubeH * 0.9 - 6);
    ctx.stroke();

    ctx.fillStyle = 'rgba(241, 196, 15, 0.7)'; ctx.font = '9px sans-serif'; ctx.textAlign = 'left';
    ctx.fillText('0.2 m', markX + 22, tubeY + tubeH * 0.5);

    // 速度向量
    if (ballProgress > 0.15 && ballProgress < 0.9) {
      ctx.strokeStyle = '#2ecc71'; ctx.lineWidth = 2;
      ctx.beginPath();
      ctx.moveTo(ballX, ballY);
      ctx.lineTo(ballX, ballY + 20);
      ctx.stroke();
      // 箭头
      ctx.beginPath();
      ctx.moveTo(ballX - 3, ballY + 17);
      ctx.lineTo(ballX, ballY + 22);
      ctx.lineTo(ballX + 3, ballY + 17);
      ctx.stroke();
      ctx.fillStyle = 'rgba(46, 204, 113, 0.7)'; ctx.font = '9px sans-serif';
      ctx.fillText('v', ballX + 5, ballY + 18);
    }

    // 右侧：速度-时间曲线
    const graphX = w * 0.55, graphY = h * 0.15, graphW = w * 0.38, graphH = h * 0.5;
    ctx.strokeStyle = 'rgba(255,255,255,0.2)'; ctx.lineWidth = 1;
    ctx.strokeRect(graphX, graphY, graphW, graphH);

    // 加速 -> 匀速
    ctx.strokeStyle = '#3498db'; ctx.lineWidth = 2;
    ctx.beginPath();
    for (let px = 0; px <= graphW; px++) {
      const progress = px / graphW;
      let v;
      if (progress < 0.2) {
        v = progress / 0.2; // 加速段
      } else {
        v = 1.0; // 匀速段
      }
      const py = graphY + graphH - v * graphH * 0.85;
      if (px === 0) ctx.moveTo(graphX + px, py); else ctx.lineTo(graphX + px, py);
    }
    ctx.stroke();

    // 当前位置点
    const curPx = ballProgress * graphW;
    const curV = ballProgress < 0.2 ? ballProgress / 0.2 : 1.0;
    const curPy = graphY + graphH - curV * graphH * 0.85;
    ctx.fillStyle = '#e74c3c';
    ctx.beginPath(); ctx.arc(graphX + curPx, curPy, 3, 0, Math.PI * 2); ctx.fill();

    ctx.fillStyle = 'rgba(255,255,255,0.5)'; ctx.font = '9px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText('v-t 曲线', graphX + graphW / 2, graphY - 5);
    ctx.textAlign = 'left';
    ctx.fillText('t', graphX + graphW - 10, graphY + graphH + 12);
    ctx.fillText('v', graphX - 5, graphY - 3);

    // 底部参数
    ctx.fillStyle = 'rgba(255,255,255,0.5)'; ctx.font = '9px sans-serif'; ctx.textAlign = 'center';
    ctx.fillText(`ρ_ball=${ballDensity}, ρ₀=${LIQ_RHO}, η=0.985 Pa·s`, w / 2, h - 5);
  },

  onDensityChange(e) {
    this.setData({ ballDensity: e.detail.value });
    this.generateReadings();
  },

  onReadingInput(e) {
    const idx = parseInt(e.currentTarget.dataset.index, 10);
    const field = e.currentTarget.dataset.field;
    this.setData({ [`readings[${idx}].${field}`]: e.detail.value });
  },

  async submitReadings() {
    const { levelId, readings, ballDensity } = this.data;
    const payload = readings.map(r => ({
      set_num: r.set_num,
      ball_diameter_mm: parseFloat(r.ball_diameter_mm) || 0,
      distance_m: parseFloat(r.distance_m) || 0,
      time_s: parseFloat(r.time_s) || 0
    }));
    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit', method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 16,
          experiment: 'falling_ball',
          readings: payload,
          user_config: { ball_density: ballDensity, liquid_density: LIQ_RHO }
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
      console.error('[falling-ball] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally { this.setData({ submitting: false }); }
  }
});
