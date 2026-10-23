// pages/experiments/cubic-spline/cubic-spline.js - 三次样条插值（数值分析 L25）
// 契约（docs/api-contract.md 附·数值分析批次 L15，v0.20）：
//   config: { problem_key, problem_desc, func_desc, x_eval, method_hint }
//   readings: [{ value, m_col? }]（value=样条 S(x_eval)，必填；m_col=学生解出的
//             弯矩列 M_0..M_{n-1}（含端点 0），选填仅记入审计不参与打分）
//   user_config: { problem_key, x_eval }（换题/换求值点，后端重解三弯矩重算样条真值）
// 教学设计：页面算好 μ/λ/d 并摆出三弯矩方程（自然边界 M_0=M_{n-1}=0），追赶法解 M
// 与段公式求 S(x_eval) 留给学生手算填；样条不外推，x_eval 须在节点区间内。
// 龙格题与 L16 拉格朗日同节点同求值点：灰虚线 P 剧烈振荡 vs 橙实线 S 光滑贴合，三法对照。
// 图像支持：点击全屏、双指缩放、单指拖动、双击复位（utils/graphview.js）。
import { request } from '../../../utils/request';
import { splineProblems, splineMoments, splineEvalAt, lagrangeP, fmtNum, parseNum } from '../../../utils/numlab';
import {
  panView, zoomView, clampView, pxToData,
  niceStep, fmtTick, createGestureTracker
} from '../../../utils/graphview';

const PROBLEM_KEYS = Object.keys(splineProblems);

Page({
  data: {
    levelId: null,
    experimentId: null,
    config: {
      problem_key: 'dist_4',
      x_eval: 3,
      x_eval_hint: 'S(3)：先解三弯矩方程求 M1、M2，再按段公式求 S(3)'
    },
    problemLabels: [],
    problemIndex: 0,
    // 展示用（WXML 不能调函数，均在 JS 侧预格式化）
    problemLabel: '',
    problemDesc: '',
    fLabel: '',
    // 插值表（含区间宽 h_i）
    nodeRows: [],
    // 三弯矩方程行（i=1..n-2）
    momentRows: [],
    // 弯矩列 M_0..M_{n-1}：端点固定 0（自然边界），内点学生填写
    mRows: [],
    // 段上求值分步（x_eval 所在段与代入后的公式）
    segmentLines: [],
    // 求值点（input 文本 + 已应用的数值）
    xEvalText: '',
    xEvalApplied: 0,
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
  // 当前节点下的真实弯矩列（画样条曲线用；学生仍须手算自己的 M）
  _M: null,
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
      problemLabels: PROBLEM_KEYS.map(k => splineProblems[k].label)
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
          console.error('[spline] canvas 节点获取失败');
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

  // 加载实验配置（后端不可用时回退默认 dist_4）
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
      if (cfg.problem_key && splineProblems[cfg.problem_key]) {
        this.setData({
          experimentId: data.id,
          config: { ...this.data.config, ...cfg }
        });
      }
    } catch (err) {
      console.warn('[spline] 后端不可用，使用默认配置');
    }
    this.applyProblem(this.data.config.problem_key, this.data.config.x_eval);
  },

  // 换题/初始化统一入口：刷新插值表、三弯矩卡、弯矩列与画布
  applyProblem(problemKey, xEval) {
    const prob = splineProblems[problemKey] || splineProblems.dist_4;
    this._prob = prob;
    this._M = splineMoments(prob.xs, prob.ys);
    const idx = PROBLEM_KEYS.indexOf(problemKey);
    this.setData({
      problemIndex: idx >= 0 ? idx : 0,
      problemLabel: prob.label,
      problemDesc: prob.desc,
      fLabel: prob.fLabel || '（纯观测数据，无原函数）'
    });
    this.applyXEval(xEval === undefined ? prob.xEval : xEval);
  },

  // 应用求值点：刷新插值表/三弯矩卡/段公式并重绘
  applyXEval(xEval) {
    this._cfgGen = (this._cfgGen || 0) + 1; // 学生主动重算：使在途配置请求作废
    const prob = this._prob;
    const xe = isFinite(xEval) ? xEval : prob.xEval;
    this.setData({
      xEvalApplied: xe,
      xEvalText: String(Number(xe.toPrecision(8))),
      nodeRows: this.buildNodeRows(prob),
      momentRows: this.buildMomentRows(prob),
      mRows: this.buildMRows(prob),
      segmentLines: this.buildSegmentLines(prob, xe),
      // 换题/换求值点后旧答案作废
      valueText: ''
    });
    this._view = null; // 换求值点/换题后重新自适应视野
    this.drawPlot();
  },

  // 插值表行：x_i、y_i 与区间宽 h_i（h_i = x_{i+1} − x_i，末行留空）
  buildNodeRows(prob) {
    return prob.xs.map((x, i) => ({
      i,
      x: fmtNum(x, 6),
      y: fmtNum(prob.ys[i], 6),
      h: i < prob.xs.length - 1 ? fmtNum(prob.xs[i + 1] - x, 6) : '—'
    }));
  },

  // 三弯矩方程行（i=1..n-2）：符号式 + μ/λ/d 数值代入式
  buildMomentRows(prob) {
    const { xs, ys } = prob;
    const n = xs.length;
    const rows = [];
    for (let i = 1; i <= n - 2; i++) {
      const h0 = xs[i] - xs[i - 1], h1 = xs[i + 1] - xs[i];
      const mu = h0 / (h0 + h1), lam = h1 / (h0 + h1);
      const d = 6 * ((ys[i + 1] - ys[i]) / h1 - (ys[i] - ys[i - 1]) / h0) / (h0 + h1);
      rows.push({
        i,
        formula: `μ${i}·M${i - 1} + 2M${i} + λ${i}·M${i + 1} = d${i}`,
        substituted: `${fmtNum(mu, 4)}·M${i - 1} + 2·M${i} + ${fmtNum(lam, 4)}·M${i + 1} = ${fmtNum(d, 6)}`
      });
    }
    return rows;
  },

  // 弯矩列 M_0..M_{n-1}：端点固定 0（自然边界），内点留给学生解
  buildMRows(prob) {
    const n = prob.xs.length;
    return Array.from({ length: n }, (_, i) => ({
      i,
      fixed: i === 0 || i === n - 1,
      text: i === 0 || i === n - 1 ? '0（自然边界）' : ''
    }));
  },

  // 段上求值分步：定位 x_eval 所在段并给出代入数值的段公式（M 留给学生）
  buildSegmentLines(prob, xe) {
    const { xs, ys } = prob;
    const n = xs.length;
    let k = 0;
    for (let i = 0; i < n - 1; i++) {
      if (xe >= xs[i]) k = i;
    }
    if (k > n - 2) k = n - 2;
    const h = xs[k + 1] - xs[k];
    const w = xe - xs[k];
    return [
      `x_eval=${fmtNum(xe, 6)} 落在段 [x${k}, x${k + 1}]=[${fmtNum(xs[k], 6)}, ${fmtNum(xs[k + 1], 6)}]，h=${fmtNum(h, 6)}，w=x_eval−x${k}=${fmtNum(w, 6)}`,
      `S(x_eval) = y${k} + B·w + C·w² + D·w³，y${k}=${fmtNum(ys[k], 6)}`,
      `B = (y${k + 1}−y${k})/h − h·(2M${k}+M${k + 1})/6 = ${fmtNum((ys[k + 1] - ys[k]) / h, 6)} − ${fmtNum(h, 6)}·(2M${k}+M${k + 1})/6`,
      `C = M${k}/2，D = (M${k + 1}−M${k})/${fmtNum(6 * h, 6)}`
    ];
  },

  // ===== 交互 =====

  // 换题（换样条题库）：三弯矩卡/弯矩列/画布全部重开
  onProblemChange(e) {
    const idx = Number(e.detail.value);
    const key = PROBLEM_KEYS[idx];
    if (!key || !splineProblems[key]) return;
    this.setData({ config: { ...this.data.config, problem_key: key } });
    this.applyProblem(key, undefined);
  },

  onXEvalInput(e) {
    this.setData({ xEvalText: e.detail.value });
  },

  // 应用自定义求值点：数值合法且在节点区间内才重算（样条不外推）
  applyXEvalInput() {
    const prob = this._prob;
    const xe = parseNum(this.data.xEvalText);
    if (xe === null) {
      wx.showToast({ title: '求值点须为数字', icon: 'none' });
      return;
    }
    const lo = Math.min(...prob.xs), hi = Math.max(...prob.xs);
    if (xe < lo || xe > hi) {
      wx.showToast({ title: `样条不外推：x_eval 须在 [${fmtNum(lo, 4)}, ${fmtNum(hi, 4)}] 内`, icon: 'none' });
      return;
    }
    this.applyXEval(xe);
    wx.showToast({ title: '已按新求值点重排三弯矩方程', icon: 'success' });
  },

  // 学生填写各内点弯矩 M_i（选填，仅随 readings 上报作审计）
  onMInput(e) {
    const idx = Number(e.currentTarget.dataset.index);
    this.setData({ [`mRows[${idx}].text`]: e.detail.value });
  },

  onValueInput(e) {
    this.setData({ valueText: e.detail.value });
  },

  // 提交读数评分：value 必填；m_col 内点全填且为数字才随 readings 上报（端点自动补 0）
  async submitReadings() {
    if (this.data.submitting) return;
    const { levelId, valueText, mRows, xEvalApplied, config } = this.data;
    const value = parseFloat(valueText);
    if (valueText === '' || !isFinite(value)) {
      wx.showToast({ title: '请填写你算出的 S(x_eval)', icon: 'none' });
      return;
    }
    // m_col：内点全空则不上报；填了任一内点则须全部为数字（端点恒为 0）
    let mCol = null;
    const inner = mRows.filter(r => !r.fixed);
    const filled = inner.filter(r => String(r.text).trim() !== '');
    if (filled.length > 0) {
      if (filled.length !== inner.length) {
        wx.showToast({ title: '弯矩 M 要么全填、要么全空（选填，仅作审计）', icon: 'none' });
        return;
      }
      mCol = mRows.map(r => r.fixed ? 0 : parseFloat(r.text));
      if (mCol.some(v => !isFinite(v))) {
        wx.showToast({ title: '弯矩 M 须为数字', icon: 'none' });
        return;
      }
    }
    const reading = { value };
    if (mCol) reading.m_col = mCol;

    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit',
        method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 25,
          experiment: 'cubic_spline',
          readings: [reading],
          // 携带实际出题参数：换题/换求值点时后端按此重解三弯矩重算样条真值（契约 §12）
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
      console.error('[spline] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally {
      this.setData({ submitting: false });
    }
  },

  // ===== 图像视图：全屏 / 缩放 / 平移 =====

  // 初始视野：节点区间左右各外扩 15%（样条只在节点区间上有定义），
  // y 取 f（有原函数时）、S 与拉格朗日 P（对照）采样的并集再外扩 10%
  computeView() {
    const prob = this._prob || splineProblems.dist_4;
    const spanMin = Math.min(...prob.xs), spanMax = Math.max(...prob.xs);
    const span = spanMax - spanMin;
    const xMin = spanMin - span * 0.15, xMax = spanMax + span * 0.15;
    const N = 200;
    let yMin = Infinity, yMax = -Infinity;
    const M = this._M;
    const sample = (y) => {
      if (isFinite(y)) {
        yMin = Math.min(yMin, y);
        yMax = Math.max(yMax, y);
      }
    };
    for (let k = 0; k <= N; k++) {
      const x = xMin + (xMax - xMin) * k / N;
      if (prob.f) sample(prob.f(x));
      // S 与 P 仅在节点区间内采样（P 区间外发散、S 区间外无定义）
      if (x >= spanMin && x <= spanMax) {
        sample(splineEvalAt(prob.xs, prob.ys, M, x));
        if (prob.f) sample(lagrangeP(prob.xs, prob.ys, x));
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
            console.error('[spline] 全屏 canvas 节点获取失败');
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

  // 在指定 canvas 上绘制 f / S / P 曲线、节点、x_eval 线与刻度网格
  _drawOn(canvas, ctx, w, h, which) {
    if (!this._view) this._view = this.computeView();
    const view = this._view;
    const prob = this._prob || splineProblems.dist_4;
    const { xs, ys, f } = prob;
    const M = this._M;
    const xe = this.data.xEvalApplied;
    const spanMin = Math.min(...xs), spanMax = Math.max(...xs);

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

    // 通用折线绘制：fn(x) 在采样区间外断笔（样条不外推、P 区间外发散）
    const drawCurve = (fn, color, width, dash, restrictSpan) => {
      ctx.strokeStyle = color;
      ctx.lineWidth = width;
      ctx.setLineDash(dash);
      ctx.beginPath();
      const N = 240;
      let started = false;
      for (let i = 0; i <= N; i++) {
        const x = view.xMin + (view.xMax - view.xMin) * i / N;
        if (restrictSpan && (x < spanMin || x > spanMax)) {
          started = false;
          continue;
        }
        const y = fn(x);
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
    };

    // f 曲线（蓝实线，有原函数时对照）
    if (f) drawCurve(f, '#3498db', 2.5, [], false);
    // 拉格朗日 P（灰虚线，与 L16 同法对照：龙格题上剧烈振荡）
    if (f) drawCurve(x => lagrangeP(xs, ys, x), '#95a5a6', 1.8, [5, 5], true);
    // 样条 S（橙实线，仅节点区间内有定义）
    drawCurve(x => splineEvalAt(xs, ys, M, x), '#e67e22', 2.5, [], true);

    // 插值节点散点（橙心白环，位于 S 与 f/P 的交点上）
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

    // 求值点 x_eval：绿竖线 + S(x_eval) 点（有原函数时加 f(x_eval) 点）
    if (isFinite(xe) && xe >= spanMin && xe <= spanMax) {
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

        const yS = splineEvalAt(xs, ys, M, xe);
        const rawS = Y(yS);
        const pS = clampY(rawS);
        ctx.beginPath();
        ctx.arc(pX, pS, 5, 0, Math.PI * 2);
        ctx.fillStyle = '#2ecc71';
        ctx.fill();
        if (f) {
          const yF = f(xe);
          const rawF = Y(yF);
          // 插值误差 f(x_eval)−S(x_eval)：红虚线连出两点之差
          if (isFinite(yF) &&
              rawS > mT - 30 && rawS < mT + ph + 30 &&
              rawF > mT - 30 && rawF < mT + ph + 30 &&
              Math.abs(rawS - rawF) >= 2) {
            ctx.strokeStyle = '#e74c3c';
            ctx.lineWidth = 1.4;
            ctx.setLineDash([2, 2]);
            ctx.beginPath();
            ctx.moveTo(pX, rawF);
            ctx.lineTo(pX, rawS);
            ctx.stroke();
            ctx.setLineDash([]);
            if (Math.abs(rawS - rawF) > 14) {
              ctx.fillStyle = '#e74c3c';
              ctx.font = '10px sans-serif';
              ctx.textAlign = 'left';
              ctx.fillText('误差', pX + 4, (rawF + rawS) / 2 + 3);
            }
          }
          const pF = clampY(rawF);
          ctx.beginPath();
          ctx.arc(pX, pF, 4, 0, Math.PI * 2);
          ctx.fillStyle = '#3498db';
          ctx.fill();
        }
      }
    }

    // 图例（画在画布内，原生 canvas 上的 view 可能被遮挡）
    ctx.font = '11px sans-serif';
    ctx.textAlign = 'left';
    let lx = mL + 8;
    const ly = mT + 14;
    ctx.fillStyle = 'rgba(255, 255, 255, 0.8)';
    if (f) {
      ctx.strokeStyle = '#3498db';
      ctx.lineWidth = 2.5;
      ctx.setLineDash([]);
      ctx.beginPath();
      ctx.moveTo(lx, ly);
      ctx.lineTo(lx + 18, ly);
      ctx.stroke();
      ctx.fillText('f(x)', lx + 22, ly + 4);
      lx += 56;
    }
    ctx.strokeStyle = '#e67e22';
    ctx.lineWidth = 2.5;
    ctx.setLineDash([]);
    ctx.beginPath();
    ctx.moveTo(lx, ly);
    ctx.lineTo(lx + 18, ly);
    ctx.stroke();
    ctx.fillStyle = 'rgba(255, 255, 255, 0.8)';
    ctx.fillText('S(x) 三次样条', lx + 22, ly + 4);
    lx += 118;
    if (f) {
      ctx.strokeStyle = '#95a5a6';
      ctx.lineWidth = 1.8;
      ctx.setLineDash([5, 5]);
      ctx.beginPath();
      ctx.moveTo(lx, ly);
      ctx.lineTo(lx + 18, ly);
      ctx.stroke();
      ctx.setLineDash([]);
      ctx.fillStyle = 'rgba(255, 255, 255, 0.8)';
      ctx.fillText('P(x) 拉格朗日对照', lx + 22, ly + 4);
    }

    ctx.textAlign = 'left';
    ctx.font = '12px sans-serif';
    ctx.fillStyle = 'rgba(255, 255, 255, 0.75)';
    ctx.fillText(this.data.problemLabel, mL + 4, mT + 32);
    if (which === 'full') {
      ctx.textAlign = 'right';
      ctx.fillStyle = 'rgba(255, 255, 255, 0.5)';
      ctx.fillText('双击图像退出全屏', mL + pw, mT + 14);
    }
  }
});
