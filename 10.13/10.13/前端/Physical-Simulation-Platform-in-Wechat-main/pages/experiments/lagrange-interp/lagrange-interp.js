// pages/experiments/lagrange-interp/lagrange-interp.js - 拉格朗日插值（数值分析 L16）
// 契约（docs/api-contract.md 附·数值分析批次 L6）：
//   config: { problem_key, problem_desc, func_desc, nodes_x, nodes_y, x_eval, x_eval_hint }
//   readings: [{ value, basis? }]（value=插值多项式在 x_eval 处的值，必填；basis=学生算的
//             各基函数值 L_i(x_eval)，选填仅记入审计不参与打分）
//   user_config: { problem_key, x_eval }（换题/换求值点，后端按提交参数重算插值真值）
// 教学设计：页面演示到「基函数分子/分母的乘积」，除法与加权求和留给学生手算填 P(x_eval)；
// 龙格题上抄原函数值 f(x_eval) 与插值真值差 ~1.15 倍相对误差，直接 0 分。
// 图像支持：点击全屏、双指缩放、单指拖动、双击复位（utils/graphview.js）。
import { request } from '../../../utils/request';
import { interpProblems, lagrangeP, fmtNum, parseNum } from '../../../utils/numlab';
import {
  panView, zoomView, clampView, pxToData,
  niceStep, fmtTick, createGestureTracker
} from '../../../utils/graphview';

const PROBLEM_KEYS = Object.keys(interpProblems);

Page({
  data: {
    levelId: null,
    experimentId: null,
    config: {
      problem_key: 'sin_quad',
      x_eval: 0.5235987755982988,
      x_eval_hint: 'P(π/6)：先算三个基函数 L_i(π/6)，再加权求和'
    },
    problemLabels: [],
    problemIndex: 0,
    // 展示用（WXML 不能调函数，均在 JS 侧预格式化）
    problemLabel: '',
    problemDesc: '',
    fLabel: '',
    // 插值表
    nodeRows: [],
    // 求值点（input 文本 + 已应用的数值）
    xEvalText: '',
    xEvalApplied: 0,
    // 基函数分步计算卡（每行：公式 / 分子 / 分母 / 学生填写 L_i(x_eval)）
    basisRows: [],
    // 加权求和卡
    sumFormula: '',
    sumSubstituted: '',
    valueText: '',
    submitting: false,
    isPreview: false,
    // 图像视图
    fullscreen: false,
    fullW: 0,
    fullH: 0,
    fullCanvasH: 0,
    canvasWidth: 0,
    canvasHeight: 0,
    pixelRatio: 1
  },

  // 当前题库对象（非 data，避免大对象序列化）
  _prob: null,
  // 缓存的 canvas 节点和上下文（小画布 + 全屏画布）
  _canvas: null,
  _ctx: null,
  _fullCanvas: null,
  _fullCtx: null,
  // 图像视野（数据坐标）与手势状态
  _view: null,
  _geoSmall: null,
  _geoFull: null,
  _gest: null,
  _gestStartView: null,

  onLoad(options) {
    const levelId = options.levelId;
    // 教师/管理员预览：可自由操作实验，隐藏提交（/progress/submit 仅学生可用）
    const app = getApp();
    const role = app.globalData.userInfo ? app.globalData.userInfo.role : null;
    this.setData({
      levelId,
      isPreview: role === 'teacher' || role === 'admin',
      problemLabels: PROBLEM_KEYS.map(k => interpProblems[k].label)
    });
    this._gest = createGestureTracker();
    this.initCanvas();
    if (levelId) {
      this.loadExperimentConfig(levelId);
    } else {
      this.applyProblem(this.data.config.problem_key, this.data.config.x_eval);
    }
  },

  onReady() {
    const query = wx.createSelectorQuery().in(this);
    query.select('#plotCanvas')
      .fields({ node: true, size: true })
      .exec((res) => {
        if (!res[0]) {
          console.error('[lagrange] canvas 节点获取失败');
          return;
        }
        this._canvas = res[0].node;
        this._ctx = this._canvas.getContext('2d');
        this.drawPlot();
      });
  },

  onUnload() {
    this._unloaded = true;
    this._canvas = null;
    this._ctx = null;
    this._fullCanvas = null;
    this._fullCtx = null;
  },

  // 初始化 canvas 尺寸（小画布 + 全屏画布）
  initCanvas() {
    const sysInfo = wx.getSystemInfoSync();
    this.setData({
      canvasWidth: sysInfo.windowWidth,
      canvasHeight: Math.min(sysInfo.windowWidth * 0.9, 420),
      fullW: sysInfo.windowWidth,
      fullH: sysInfo.windowHeight,
      // 全屏画布高度扣掉上下工具栏（与 wxss 的 44px / 72px 严格对应）
      fullCanvasH: Math.max(120, sysInfo.windowHeight - 44 - 72),
      pixelRatio: sysInfo.pixelRatio || 1
    });
  },

  // 加载实验配置（后端不可用时回退默认 sin_quad）
  async loadExperimentConfig(levelId) {
    // 代次守卫：学生若在配置返回前换题/改参数（或已退出页面），晚到的配置不再覆盖
    this._cfgGen = (this._cfgGen || 0) + 1;
    const gen = this._cfgGen;
    try {
      const res = await request({
        url: `/experiments/${levelId}`,
        method: 'GET'
      });
      if (this._unloaded || gen !== this._cfgGen) return;
      const data = res.data || res || {};
      const cfg = data.config || {};
      if (cfg.problem_key && interpProblems[cfg.problem_key]) {
        this.setData({
          experimentId: data.id,
          config: { ...this.data.config, ...cfg }
        });
      }
    } catch (err) {
      console.warn('[lagrange] 后端不可用，使用默认配置');
    }
    this.applyProblem(this.data.config.problem_key, this.data.config.x_eval);
  },

  // 换题/初始化统一入口：刷新插值表、基函数卡与画布
  applyProblem(problemKey, xEval) {
    const prob = interpProblems[problemKey] || interpProblems.sin_quad;
    this._prob = prob;
    const idx = PROBLEM_KEYS.indexOf(problemKey);
    this.setData({
      problemIndex: idx >= 0 ? idx : 0,
      problemLabel: prob.label,
      problemDesc: prob.desc,
      fLabel: prob.fLabel
    });
    this.applyXEval(xEval === undefined ? prob.xEval : xEval);
  },

  // 应用求值点：刷新插值表/基函数分步卡/求和公式并重绘
  applyXEval(xEval) {
    this._cfgGen = (this._cfgGen || 0) + 1; // 学生主动重算：使在途配置请求作废
    const prob = this._prob;
    const xe = isFinite(xEval) ? xEval : prob.xEval;
    this.setData({
      xEvalApplied: xe,
      xEvalText: String(Number(xe.toPrecision(8))),
      nodeRows: this.buildNodeRows(prob),
      basisRows: this.buildBasisRows(prob, xe),
      sumFormula: this.buildSumFormula(prob),
      sumSubstituted: this.buildSumSubstituted(prob, xe),
      // 换题/换求值点后旧答案作废
      valueText: ''
    });
    this._view = null; // 换求值点/换题后重新自适应视野
    this.drawPlot();
  },

  // 插值表行：x_i、y_i（y_i 取自 f(x_i)，原函数仅用于造表与画图对照）
  buildNodeRows(prob) {
    return prob.xs.map((x, i) => ({
      i,
      x: fmtNum(x, 6),
      y: fmtNum(prob.ys[i], 6)
    }));
  },

  // 基函数分步卡：每行给公式与分子/分母的乘积（除法留给学生）
  buildBasisRows(prob, xe) {
    return prob.xs.map((xi, i) => {
      const others = prob.xs.map((xj, j) => j === i ? null : xj).filter(v => v !== null);
      // 公式（节点数值代入）：L_i(x) = Π(x−x_j) / Π(x_i−x_j)
      let formula = `L${i}(x) = `;
      formula += others.map(xj => `(x−${fmtNum(xj, 5)})`).join('');
      formula += ' / (';
      formula += others.map(xj => `(${fmtNum(xi, 5)}−${fmtNum(xj, 5)})`).join('');
      formula += ')';
      // 分子：Π(x_eval − x_j)
      let num = 1;
      const numParts = others.map(xj => {
        const f = xe - xj;
        num *= f;
        return `(${fmtNum(xe, 5)}−${fmtNum(xj, 5)})`;
      });
      // 分母：Π(x_i − x_j)
      let den = 1;
      const denParts = others.map(xj => {
        const f = xi - xj;
        den *= f;
        return `(${fmtNum(xi, 5)}−${fmtNum(xj, 5)})`;
      });
      return {
        i,
        formula,
        num: numParts.join('') + ' = ' + fmtNum(num, 6),
        den: denParts.join('') + ' = ' + fmtNum(den, 6),
        input: ''
      };
    });
  },

  // 求和公式（符号形式）
  buildSumFormula(prob) {
    return prob.xs.map((_, i) => `y${i}·L${i}(x)`).join(' + ');
  },

  // 求和公式（y_i 数值代入，学生须自行乘加）
  buildSumSubstituted(prob) {
    return prob.ys.map((yi, i) => `${fmtNum(yi, 6)}·L${i}(x_eval)`).join(' + ');
  },

  // ===== 交互 =====

  // 换题（换插值题库）：基函数卡/插值表/画布全部重开
  onProblemChange(e) {
    const idx = Number(e.detail.value);
    const key = PROBLEM_KEYS[idx];
    if (!key || !interpProblems[key]) return;
    this.setData({ config: { ...this.data.config, problem_key: key } });
    this.applyProblem(key, undefined);
  },

  onXEvalInput(e) {
    this.setData({ xEvalText: e.detail.value });
  },

  // 应用自定义求值点：数值合法即重算基函数卡（提交时随 user_config 上报）
  applyXEvalInput() {
    const xe = parseNum(this.data.xEvalText);
    if (xe === null) {
      wx.showToast({ title: '求值点须为数字', icon: 'none' });
      return;
    }
    this.applyXEval(xe);
    wx.showToast({ title: '已按新求值点重算基函数', icon: 'success' });
  },

  // 学生填写各基函数值 L_i(x_eval)（选填，仅随 readings 上报作审计）
  onBasisInput(e) {
    const idx = Number(e.currentTarget.dataset.index);
    this.setData({ [`basisRows[${idx}].input`]: e.detail.value });
  },

  onValueInput(e) {
    this.setData({ valueText: e.detail.value });
  },

  // 提交读数评分：value 必填；basis 全部填写且为数字才随 readings 上报
  async submitReadings() {
    if (this.data.submitting) return;
    const { levelId, valueText, basisRows, xEvalApplied, config } = this.data;
    const value = parseFloat(valueText);
    if (valueText === '' || !isFinite(value)) {
      wx.showToast({ title: '请填写你算出的 P(x_eval)', icon: 'none' });
      return;
    }
    // basis：全空则不上报；填了任一项则须全部为数字
    let basis = null;
    const filled = basisRows.filter(r => String(r.input).trim() !== '');
    if (filled.length > 0) {
      if (filled.length !== basisRows.length) {
        wx.showToast({ title: '基函数值要么全填、要么全空（选填，仅作审计）', icon: 'none' });
        return;
      }
      basis = basisRows.map(r => parseFloat(r.input));
      if (basis.some(v => !isFinite(v))) {
        wx.showToast({ title: '基函数值须为数字', icon: 'none' });
        return;
      }
    }
    const reading = { value };
    if (basis) reading.basis = basis;

    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit',
        method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 16,
          experiment: 'lagrange_interp',
          readings: [reading],
          // 携带实际出题参数：换题/换求值点时后端按此重算插值真值（契约 §12）
          user_config: {
            problem_key: config.problem_key,
            x_eval: xEvalApplied
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
      console.error('[lagrange] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally {
      this.setData({ submitting: false });
    }
  },

  // ===== 图像视图：全屏 / 缩放 / 平移 =====

  // 初始视野：题库横轴范围（含求值点外扩），y 取 f 与 P 采样的并集再外扩 10%
  computeView() {
    const prob = this._prob || interpProblems.sin_quad;
    let xMin = prob.xMin, xMax = prob.xMax;
    const xe = this.data.xEvalApplied;
    if (isFinite(xe)) {
      const span = xMax - xMin;
      if (xe < xMin) xMin = xe - span * 0.08;
      if (xe > xMax) xMax = xe + span * 0.08;
    }
    const N = 200;
    let yMin = Infinity, yMax = -Infinity;
    for (let k = 0; k <= N; k++) {
      const x = xMin + (xMax - xMin) * k / N;
      const yf = prob.f(x);
      const yp = lagrangeP(prob.xs, prob.ys, x);
      if (isFinite(yf)) {
        yMin = Math.min(yMin, yf);
        yMax = Math.max(yMax, yf);
      }
      if (isFinite(yp)) {
        yMin = Math.min(yMin, yp);
        yMax = Math.max(yMax, yp);
      }
    }
    if (!isFinite(yMin) || !isFinite(yMax)) { yMin = -1; yMax = 1; }
    if (yMin === yMax) { yMin -= 0.5; yMax += 0.5; }
    const pad = (yMax - yMin) * 0.1;
    return clampView({ xMin, xMax, yMin: yMin - pad, yMax: yMax + pad });
  },

  // 复位视野（双击或换题时调用）
  resetView() {
    this._view = this.computeView();
    this.drawPlot();
  },

  // 按钮缩放：以当前视野中心为不动点（模拟器鼠标无法双指捏合时的替代操作）
  zoomBy(e) {
    const factor = Number(e.currentTarget.dataset.factor);
    if (!(factor > 0)) return;
    if (!this._view) this._view = this.computeView();
    const center = {
      x: (this._view.xMin + this._view.xMax) / 2,
      y: (this._view.yMin + this._view.yMax) / 2
    };
    this._view = clampView(zoomView(this._view, center, factor));
    this.drawPlot();
  },

  // 进入全屏：懒加载全屏 canvas 节点后按当前视野重绘
  enterFullscreen() {
    this.setData({ fullscreen: true }, () => {
      if (this._fullCanvas) {
        this.drawPlot();
        return;
      }
      const query = wx.createSelectorQuery().in(this);
      query.select('#fullCanvas')
        .fields({ node: true, size: true })
        .exec((res) => {
          if (!res[0]) {
            console.error('[lagrange] 全屏 canvas 节点获取失败');
            return;
          }
          this._fullCanvas = res[0].node;
          this._fullCtx = this._fullCanvas.getContext('2d');
          this.drawPlot();
        });
    });
  },

  exitFullscreen() {
    // 全屏期间小画布跳过重绘，收回后补画一帧
    this.setData({ fullscreen: false }, () => this.drawPlot());
  },

  // 画布触摸：小画布单击进全屏；两块画布均可缩放/平移；双击复位
  onPlotTouchStart(e) {
    const mode = this._gest.anchor(e.touches);
    if (mode) this._gestStartView = { ...this._view };
  },

  onPlotTouchMove(e) {
    const g = this._gest.move(e.touches);
    if (!g || !this._gestStartView) return;
    const geo = e.currentTarget.id === 'fullCanvas' ? this._geoFull : this._geoSmall;
    if (!geo) return;
    if (g.type === 'pan') {
      const dxData = -(g.dxPx / geo.pw) * (this._gestStartView.xMax - this._gestStartView.xMin);
      const dyData = (g.dyPx / geo.ph) * (this._gestStartView.yMax - this._gestStartView.yMin);
      this._view = clampView(panView(this._gestStartView, dxData, dyData));
    } else {
      const center = pxToData(this._gestStartView, geo, g.cx, g.cy);
      this._view = clampView(zoomView(this._gestStartView, center, g.ratio));
    }
    this.drawPlot();
  },

  onPlotTouchEnd(e) {
    if (e.touches && e.touches.length > 0) {
      // 手指未全部离开：对剩余触点重新锚定（捏合转拖动等场景）
      const mode = this._gest.anchor(e.touches);
      if (mode) this._gestStartView = { ...this._view };
      return;
    }
    const t = this._gest.end();
    const isSmall = e.currentTarget.id === 'plotCanvas';
    if (t === 'tap' && isSmall) {
      this.enterFullscreen();
    } else if (t === 'doubletap') {
      // 页内双击复位视野；全屏下双击退出，作为按钮被遮挡时的保底出口
      if (isSmall) this.resetView();
      else this.exitFullscreen();
    }
  },

  // 绘制：小画布始终绘制；全屏时同步绘制全屏画布
  drawPlot() {
    // 全屏期间小画布被隐藏，跳过其重绘（拖动/脉冲每帧只画一块画布）
    if (!this.data.fullscreen && this._canvas && this._ctx) {
      this._drawOn(this._canvas, this._ctx, this.data.canvasWidth, this.data.canvasHeight, 'small');
    }
    if (this.data.fullscreen && this._fullCanvas && this._fullCtx) {
      this._drawOn(this._fullCanvas, this._fullCtx, this.data.fullW, this.data.fullCanvasH, 'full');
    }
  },

  // 在指定 canvas 上绘制 f / P 双曲线、节点、x_eval 线与刻度网格
  _drawOn(canvas, ctx, w, h, which) {
    if (!this._view) this._view = this.computeView();
    const view = this._view;
    const prob = this._prob || interpProblems.sin_quad;
    const { xs, ys, f } = prob;
    const xe = this.data.xEvalApplied;

    // backing store 仅在尺寸变化时重设（每帧重赋 width 会清空画布状态且有开销）
    const pr = this.data.pixelRatio || 1;
    const bw = Math.round(w * pr), bh = Math.round(h * pr);
    if (canvas.width !== bw || canvas.height !== bh) {
      canvas.width = bw;
      canvas.height = bh;
    }
    ctx.setTransform(pr, 0, 0, pr, 0, 0);

    // 布局边距
    const mL = 52, mR = 16, mT = 16, mB = 36;
    const pw = w - mL - mR;
    const ph = h - mT - mB;
    if (pw <= 0 || ph <= 0) return;
    const geo = { mL, mT, pw, ph };
    if (which === 'full') this._geoFull = geo;
    else this._geoSmall = geo;

    const X = (x) => mL + (x - view.xMin) / (view.xMax - view.xMin) * pw;
    const Y = (y) => mT + (view.yMax - y) / (view.yMax - view.yMin) * ph;
    const clampY = (py) => Math.max(mT - 1000, Math.min(mT + ph + 1000, py));
    const clampX = (px) => Math.max(mL + 24, Math.min(mL + pw - 24, px));

    // 背景
    ctx.fillStyle = '#10182a';
    ctx.fillRect(0, 0, w, h);

    // 刻度网格与刻度值（1-2-5 步长）
    ctx.font = '10px sans-serif';
    ctx.lineWidth = 1;
    const stepX = niceStep((view.xMax - view.xMin) / 5);
    ctx.textAlign = 'center';
    for (let i = Math.ceil(view.xMin / stepX); i * stepX <= view.xMax; i++) {
      const x = i * stepX;
      const px = X(x);
      if (px < mL - 2 || px > mL + pw + 2) continue;
      ctx.strokeStyle = 'rgba(255, 255, 255, 0.08)';
      ctx.beginPath();
      ctx.moveTo(px, mT);
      ctx.lineTo(px, mT + ph);
      ctx.stroke();
      ctx.fillStyle = 'rgba(255, 255, 255, 0.45)';
      ctx.fillText(fmtTick(x), px, mT + ph + 24);
    }
    const stepY = niceStep((view.yMax - view.yMin) / 4);
    ctx.textAlign = 'right';
    for (let i = Math.ceil(view.yMin / stepY); i * stepY <= view.yMax; i++) {
      const y = i * stepY;
      const py = Y(y);
      if (py < mT - 2 || py > mT + ph + 2) continue;
      ctx.strokeStyle = 'rgba(255, 255, 255, 0.08)';
      ctx.beginPath();
      ctx.moveTo(mL, py);
      ctx.lineTo(mL + pw, py);
      ctx.stroke();
      ctx.fillStyle = 'rgba(255, 255, 255, 0.45)';
      ctx.fillText(fmtTick(y), mL - 6, py + 3);
    }

    // 坐标轴（y=0 / x=0 在视野内时加粗）
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.4)';
    if (view.yMin <= 0 && view.yMax >= 0) {
      ctx.beginPath();
      ctx.moveTo(mL, Y(0));
      ctx.lineTo(mL + pw, Y(0));
      ctx.stroke();
    }
    if (view.xMin <= 0 && view.xMax >= 0) {
      ctx.beginPath();
      ctx.moveTo(X(0), mT);
      ctx.lineTo(X(0), mT + ph);
      ctx.stroke();
    }

    // f 曲线（蓝实线；离屏过远的 y 夹取，防 canvas 精度问题）
    ctx.strokeStyle = '#3498db';
    ctx.lineWidth = 2.5;
    ctx.setLineDash([]);
    ctx.beginPath();
    const N = 240;
    let started = false;
    for (let i = 0; i <= N; i++) {
      const x = view.xMin + (view.xMax - view.xMin) * i / N;
      const y = f(x);
      if (!isFinite(y)) {
        started = false;
        continue;
      }
      const py = clampY(Y(y));
      if (!started) {
        ctx.moveTo(X(x), py);
        started = true;
      } else {
        ctx.lineTo(X(x), py);
      }
    }
    ctx.stroke();

    // P 曲线（橙虚线）
    ctx.strokeStyle = '#e67e22';
    ctx.lineWidth = 2;
    ctx.setLineDash([6, 4]);
    ctx.beginPath();
    started = false;
    for (let i = 0; i <= N; i++) {
      const x = view.xMin + (view.xMax - view.xMin) * i / N;
      const y = lagrangeP(xs, ys, x);
      if (!isFinite(y)) {
        started = false;
        continue;
      }
      const py = clampY(Y(y));
      if (!started) {
        ctx.moveTo(X(x), py);
        started = true;
      } else {
        ctx.lineTo(X(x), py);
      }
    }
    ctx.stroke();
    ctx.setLineDash([]);

    // 插值节点散点（橙心白环，位于 P 与 f 的交点上）
    xs.forEach((x, i) => {
      const py = clampY(Y(ys[i]));
      ctx.beginPath();
      ctx.arc(X(x), py, 4.5, 0, Math.PI * 2);
      ctx.fillStyle = '#e67e22';
      ctx.fill();
      ctx.lineWidth = 1.5;
      ctx.strokeStyle = '#fff';
      ctx.stroke();
    });

    // 求值点 x_eval：绿竖线 + P(x_eval) 与 f(x_eval) 两点（两点之差即插值误差）
    if (isFinite(xe)) {
      const pX = X(xe);
      if (pX >= mL - 4 && pX <= mL + pw + 4) {
        ctx.strokeStyle = '#2ecc71';
        ctx.lineWidth = 1.5;
        ctx.setLineDash([4, 4]);
        ctx.beginPath();
        ctx.moveTo(pX, mT);
        ctx.lineTo(pX, mT + ph);
        ctx.stroke();
        ctx.setLineDash([]);
        ctx.fillStyle = '#2ecc71';
        ctx.font = '11px sans-serif';
        ctx.textAlign = 'center';
        ctx.fillText('x_eval', clampX(pX), mT + 12);

        const yP = lagrangeP(xs, ys, xe);
        const yF = f(xe);
        const rawP = Y(yP), rawF = Y(yF);
        // 插值误差 Rₙ=f(x_eval)−P(x_eval)：红虚线把两值之差显式连出
        // （龙格题端部两点会明显错开；离屏过远或重合时不画）
        if (isFinite(yP) && isFinite(yF) &&
            rawP > mT - 30 && rawP < mT + ph + 30 &&
            rawF > mT - 30 && rawF < mT + ph + 30 &&
            Math.abs(rawP - rawF) >= 2) {
          ctx.strokeStyle = '#e74c3c';
          ctx.lineWidth = 1.4;
          ctx.setLineDash([2, 2]);
          ctx.beginPath();
          ctx.moveTo(pX, rawF);
          ctx.lineTo(pX, rawP);
          ctx.stroke();
          ctx.setLineDash([]);
          if (Math.abs(rawP - rawF) > 14) {
            ctx.fillStyle = '#e74c3c';
            ctx.font = '10px sans-serif';
            ctx.textAlign = 'left';
            ctx.fillText('误差', pX + 4, (rawF + rawP) / 2 + 3);
          }
        }
        const pP = clampY(rawP);
        ctx.beginPath();
        ctx.arc(pX, pP, 5, 0, Math.PI * 2);
        ctx.fillStyle = '#2ecc71';
        ctx.fill();
        const pF = clampY(rawF);
        ctx.beginPath();
        ctx.arc(pX, pF, 4, 0, Math.PI * 2);
        ctx.fillStyle = '#3498db';
        ctx.fill();
      }
    }

    // 图例（画在画布内，原生 canvas 上的 view 可能被遮挡）
    ctx.font = '11px sans-serif';
    ctx.textAlign = 'left';
    let lx = mL + 8;
    const ly = mT + 14;
    ctx.strokeStyle = '#3498db';
    ctx.lineWidth = 2.5;
    ctx.setLineDash([]);
    ctx.beginPath();
    ctx.moveTo(lx, ly);
    ctx.lineTo(lx + 18, ly);
    ctx.stroke();
    ctx.fillStyle = 'rgba(255, 255, 255, 0.8)';
    ctx.fillText('f(x)', lx + 22, ly + 4);
    lx += 56;
    ctx.strokeStyle = '#e67e22';
    ctx.lineWidth = 2;
    ctx.setLineDash([6, 4]);
    ctx.beginPath();
    ctx.moveTo(lx, ly);
    ctx.lineTo(lx + 18, ly);
    ctx.stroke();
    ctx.setLineDash([]);
    ctx.fillStyle = 'rgba(255, 255, 255, 0.8)';
    ctx.fillText('P(x) 插值多项式', lx + 22, ly + 4);

    ctx.textAlign = 'left';
    ctx.font = '12px sans-serif';
    ctx.fillStyle = 'rgba(255, 255, 255, 0.75)';
    ctx.fillText(this.data.fLabel, mL + 4, mT + 32);
    if (which === 'full') {
      ctx.textAlign = 'right';
      ctx.fillStyle = 'rgba(255, 255, 255, 0.5)';
      ctx.fillText('双击图像退出全屏', mL + pw, mT + 14);
    }
  }
});
