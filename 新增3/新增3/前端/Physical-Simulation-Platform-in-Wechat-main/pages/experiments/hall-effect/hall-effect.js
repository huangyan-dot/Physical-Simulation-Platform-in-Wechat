// pages/experiments/hall-effect/hall-effect.js
import { request } from '../../../utils/request';

const q = 1.602e-19; // 电子电荷 C
const n = 1.0e21;    // 载流子浓度 m^-3

Page({
  data: {
    levelId: null,
    config: {
      b_field_t: 0.3,
      thickness_mm: 0.5
    },
    currentMA: 5, // mA
    voltageStr: '0.000',
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
    query.select('#hallCanvas')
      .fields({ node: true, size: true })
      .exec((res) => {
        if (!res[0]) {
          console.error('[hall-effect] canvas 节点获取失败');
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
      if (data.config) {
        const c = data.config;
        this.setData({
          config: {
            b_field_t: c.b_field_t || 0.3,
            thickness_mm: (c.thickness_m || 0.0005) * 1000
          }
        });
      }
    } catch (err) {
      console.warn('[hall-effect] 后端不可用，使用默认配置');
    }
    this.generateReadings();
  },

  // 生成霍尔电压理论读数
  generateReadings() {
    const { b_field_t, thickness_mm } = this.data.config;
    const d = thickness_mm * 1e-3; // mm -> m
    const B = b_field_t;

    const currentsMA = [1, 2, 3, 5, 8, 10];
    const readings = currentsMA.map(ma => {
      const I = ma * 1e-3; // mA -> A
      // V_H = I·B / (n·q·d)
      const VH = (I * B) / (n * q * d); // V
      const VH_mV = (VH * 1e3).toFixed(4); // V -> mV
      return { current_a: I, current_ma: ma, voltage_mv: VH_mV };
    });

    // 更新当前电流对应的理论电压
    this.updateVoltage();

    this.setData({ readings });
  },

  updateVoltage() {
    const { b_field_t, thickness_mm } = this.data.config;
    const d = thickness_mm * 1e-3;
    const I = this.data.currentMA * 1e-3;
    const VH = (I * b_field_t) / (n * q * d);
    this.setData({ voltageStr: (VH * 1e3).toFixed(4) });
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

  // 绘制霍尔效应示意图
  drawScene() {
    const canvas = this._canvas;
    const ctx = this._ctx;
    if (!canvas || !ctx) return;

    const { canvasWidth, canvasHeight, pixelRatio, config, currentMA } = this.data;
    canvas.width = canvasWidth * pixelRatio;
    canvas.height = canvasHeight * pixelRatio;
    ctx.setTransform(pixelRatio, 0, 0, pixelRatio, 0, 0);

    const w = canvasWidth;
    const h = canvasHeight;

    // 背景
    ctx.fillStyle = '#0d1117';
    ctx.fillRect(0, 0, w, h);

    const cx = w / 2;
    const cy = h / 2;

    // 磁场区域（方框 + 叉号表示磁场方向向内）
    const fieldW = w * 0.6;
    const fieldH = h * 0.55;
    const fieldX = cx - fieldW / 2;
    const fieldY = cy - fieldH / 2;

    ctx.strokeStyle = 'rgba(100, 149, 237, 0.3)';
    ctx.lineWidth = 2;
    ctx.strokeRect(fieldX, fieldY, fieldW, fieldH);

    // 磁场叉号
    ctx.strokeStyle = 'rgba(100, 149, 237, 0.2)';
    ctx.lineWidth = 1;
    const spacing = 30;
    for (let x = fieldX + spacing; x < fieldX + fieldW; x += spacing) {
      for (let y = fieldY + spacing; y < fieldY + fieldH; y += spacing) {
        ctx.beginPath();
        ctx.moveTo(x - 4, y - 4);
        ctx.lineTo(x + 4, y + 4);
        ctx.moveTo(x + 4, y - 4);
        ctx.lineTo(x - 4, y + 4);
        ctx.stroke();
      }
    }

    // 半导体样品（矩形薄片）
    const sampleW = fieldW * 0.5;
    const sampleH = 20;
    const sampleX = cx - sampleW / 2;
    const sampleY = cy - sampleH / 2;

    ctx.fillStyle = '#2ecc71';
    ctx.fillRect(sampleX, sampleY, sampleW, sampleH);
    ctx.strokeStyle = '#27ae60';
    ctx.lineWidth = 2;
    ctx.strokeRect(sampleX, sampleY, sampleW, sampleH);

    // 电流方向箭头（从左到右）
    const arrowY = cy;
    ctx.strokeStyle = '#e74c3c';
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(sampleX - 40, arrowY);
    ctx.lineTo(sampleX, arrowY);
    ctx.stroke();
    // 箭头
    ctx.beginPath();
    ctx.moveTo(sampleX - 8, arrowY - 5);
    ctx.lineTo(sampleX, arrowY);
    ctx.lineTo(sampleX - 8, arrowY + 5);
    ctx.stroke();

    // 霍尔电压引线（上下两侧）
    ctx.strokeStyle = '#f1c40f';
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(cx, sampleY);
    ctx.lineTo(cx, sampleY - 25);
    ctx.stroke();
    ctx.beginPath();
    ctx.moveTo(cx, sampleY + sampleH);
    ctx.lineTo(cx, sampleY + sampleH + 25);
    ctx.stroke();

    // 电压表
    ctx.fillStyle = '#3498db';
    ctx.beginPath();
    ctx.arc(cx, sampleY - 35, 12, 0, Math.PI * 2);
    ctx.fill();
    ctx.strokeStyle = '#2980b9';
    ctx.lineWidth = 1;
    ctx.stroke();
    ctx.fillStyle = '#fff';
    ctx.font = '8px sans-serif';
    ctx.textAlign = 'center';
    ctx.fillText('V', cx, sampleY - 32);

    // 霍尔电场方向（上正下负，电子偏转）
    ctx.fillStyle = '#e74c3c';
    ctx.font = '12px sans-serif';
    ctx.textAlign = 'center';
    ctx.fillText('+', cx, sampleY - 20);
    ctx.fillText('−', cx, sampleY + sampleH + 18);

    // 电子运动偏转（弧线）
    ctx.strokeStyle = 'rgba(231, 76, 60, 0.5)';
    ctx.lineWidth = 1;
    ctx.setLineDash([3, 3]);
    const t = Date.now() / 500;
    const offset = (t % 2) * 10;
    for (let i = 0; i < 3; i++) {
      const ex = sampleX + 20 + i * (sampleW / 4) + offset;
      if (ex > sampleX + sampleW - 10) continue;
      ctx.beginPath();
      ctx.moveTo(ex, sampleY + sampleH - 2);
      ctx.quadraticCurveTo(ex + 5, sampleY + sampleH / 2, ex, sampleY + 2);
      ctx.stroke();
    }
    ctx.setLineDash([]);

    // 标注
    ctx.fillStyle = 'rgba(255, 255, 255, 0.6)';
    ctx.font = '10px sans-serif';
    ctx.textAlign = 'left';
    ctx.fillText(`B = ${config.b_field_t} T (向内)`, fieldX, fieldY - 8);
    ctx.fillText(`I = ${currentMA} mA`, sampleX - 40, arrowY - 10);
    ctx.fillText('半导体样品', sampleX, sampleY + sampleH + 35);
    ctx.fillText(`d = ${config.thickness_mm} mm`, sampleX + sampleW - 60, sampleY + sampleH + 35);
  },

  onBFieldChange(e) {
    this.setData({ 'config.b_field_t': e.detail.value });
    this.generateReadings();
  },

  onThicknessChange(e) {
    this.setData({ 'config.thickness_mm': e.detail.value });
    this.generateReadings();
  },

  onCurrentChange(e) {
    this.setData({ currentMA: e.detail.value });
    this.updateVoltage();
  },

  onReadingInput(e) {
    const index = parseInt(e.currentTarget.dataset.index, 10);
    this.setData({
      [`readings[${index}].voltage_mv`]: e.detail.value
    });
  },

  async submitReadings() {
    const { levelId, readings, config } = this.data;
    const payload = readings.map(r => ({
      current_a: r.current_a,
      voltage_v: parseFloat(r.voltage_mv || 0) * 1e-3 // mV -> V
    }));

    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit',
        method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 5,
          experiment: 'hall_effect',
          readings: payload,
          user_config: {
            b_field_t: config.b_field_t,
            thickness_m: config.thickness_mm * 1e-3
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
      console.error('[hall-effect] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally {
      this.setData({ submitting: false });
    }
  }
});
