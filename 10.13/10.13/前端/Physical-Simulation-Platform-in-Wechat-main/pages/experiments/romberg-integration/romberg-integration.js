// pages/experiments/romberg-integration/romberg-integration.js - 龙贝格积分法（数值分析 L19）
// 契约（docs/api-contract.md 附·数值分析批次 L9）：
//   config: { problem_key, problem_desc, func_desc, a, b, k, method_hint }
//   readings: [{ value, t_col? }]（value=学生递推出的终值 T_k^(k)，必填；
//             t_col=梯形列 T_0..T_k，长度 k+1，选填仅记入审计不参与打分）
//   user_config: { problem_key, a, b, k }（换题/换区间/换外推深度，后端按提交参数重算真值）
// 教学设计：页面演示到「节点表 + T 列公式结构 + 外推公式与权重」，梯形求和、
// 逐列乘减除留给学生手算填终值；真值是「方法的输出」T_k^(k) 而非精确积分 I——
// 抄 I=2 约 81 分（能过线但非满分），抄 L17 答案 2π/3 或抄 T_2 均 ~41 分不过线。
// 外推第 1 列恰为复合辛普森 S_{2^m}（sin_pi 默认 k=2 时 S_1=2π/3 正是 L17 的 S_2）。
// 图像支持：点击全屏、双指缩放、单指拖动、双击复位（utils/graphview.js）。
import { request } from '../../../utils/request';
import { integProblems, fmtNum, parseNum, parseIntStrict } from '../../../utils/numlab';
import {
  panView, zoomView, clampView, pxToData,
  niceStep, fmtTick, createGestureTracker
} from '../../../utils/graphview';

const PROBLEM_KEYS = Object.keys(integProblems);

Page({
  data: {
    levelId: null,
    experimentId: null,
    config: {
      problem_key: 'sin_pi',
      a: 0,
      b: 3.141592653589793,
      k: 2,
      method_hint: 'T_m = T_{m−1}/2 + h_m·Σf(新节点)；逐列外推 T_i^(j) = (4^j·T_i^(j−1) − T_{i−1}^(j−1))/(4^j − 1)'
    },
    problemLabels: [],
    problemIndex: 0,
    // 展示用（WXML 不能调函数，均在 JS 侧预格式化）
    problemLabel: '',
    problemDesc: '',
    fLabel: '',
    // 节点表（i / x_i / f(x_i)，第 k 级梯形的 2^k+1 个分点）
    nodeRows: [],
    nodeRowsTail: [],
    nodeTableTruncated: false,
    // 生效参数（input 文本 + 已应用的数值）
    aText: '',
    bText: '',
    kText: '',
    aApplied: 0,
    bApplied: 0,
    kApplied: 2,
    nKLabel: '4',
    // 梯形列卡（T_0..T_k：公式 + 数值代入，求和留给学生；带选填审计输入）
    tRows: [],
    // 外推三角表卡（逐列理查逊外推，公式带权重，乘减除留给学生）
    extRows: [],
    // 终值答案输入
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
      problemLabels: PROBLEM_KEYS.map(k => integProblems[k].label)
    });
    this._gest = createGestureTracker();
    this.initCanvas();
    if (levelId) {
      this.loadExperimentConfig(levelId);
    } else {
      this.applyProblem(this.data.config.problem_key);
    }
  },

  onReady() {
    const query = wx.createSelectorQuery().in(this);
    query.select('#plotCanvas')
      .fields({ node: true, size: true })
      .exec((res) => {
        if (!res[0]) {
          console.error('[romberg] canvas 节点获取失败');
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

  // 加载实验配置（后端不可用时回退默认 sin_pi）
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
      if (cfg.problem_key && integProblems[cfg.problem_key]) {
        this.setData({
          experimentId: data.id,
          config: { ...this.data.config, ...cfg }
        });
      }
    } catch (err) {
      console.warn('[romberg] 后端不可用，使用默认配置');
    }
    this.applyProblem(this.data.config.problem_key);
  },

  // 换题/初始化统一入口：应用题库默认区间与外推深度，刷新表格与画布
  applyProblem(problemKey) {
    const prob = integProblems[problemKey] || integProblems.sin_pi;
    this._prob = prob;
    const idx = PROBLEM_KEYS.indexOf(problemKey);
    this.setData({
      problemIndex: idx >= 0 ? idx : 0,
      problemLabel: prob.label,
      problemDesc: prob.desc,
      fLabel: prob.fLabel
    });
    // 题库默认区间（k 由 config 带入，后端下发了同一组数值；换题回落 2）
    this.applyParams(prob.a, prob.b, this.data.config.k || 2);
  },

  // 应用区间与外推深度：刷新节点表/公式卡并重绘；返回 true=已应用，false=参数非法
  applyParams(a, b, k) {
    this._cfgGen = (this._cfgGen || 0) + 1; // 学生主动重算：使在途配置请求作废
    const prob = this._prob;
    if (!isFinite(a) || !isFinite(b) || !(a < b)) {
      wx.showToast({ title: '区间须满足 a < b', icon: 'none' });
      return false;
    }
    k = Math.round(k);
    if (!(k >= 1 && k <= 6)) {
      wx.showToast({ title: 'k 须为 1~6 的整数', icon: 'none' });
      return false;
    }
    this.setData({
      aApplied: a,
      bApplied: b,
      kApplied: k,
      aText: String(Number(a.toPrecision(8))),
      bText: String(Number(b.toPrecision(8))),
      kText: String(k),
      nKLabel: String(Math.pow(2, k)),
      ...this.buildNodeRows(prob, a, b, k),
      tRows: this.buildTRows(prob, a, b, k),
      extRows: this.buildExtRows(k),
      // 换题/换参数后旧答案作废
      valueText: ''
    });
    this._view = null; // 换参数后重新自适应视野
    this.drawPlot();
    return true;
  },

  // 节点表行：i、x_i、f(x_i)（第 k 级梯形的分点，也是外推用到的全部函数值）。
  // 2^k+1 > 32（k ≥ 5）时表太长，只显示头 10 行 + 尾 2 行
  buildNodeRows(prob, a, b, k) {
    const n = Math.pow(2, k);
    const rows = [];
    for (let i = 0; i <= n; i++) {
      const x = a + (b - a) * i / n;
      rows.push({
        i,
        x: fmtNum(x, 6),
        y: fmtNum(prob.f(x), 6)
      });
    }
    if (n <= 32) {
      return { nodeRows: rows, nodeRowsTail: [], nodeTableTruncated: false };
    }
    return { nodeRows: rows.slice(0, 10), nodeRowsTail: rows.slice(-2), nodeTableTruncated: true };
  },

  // 梯形列卡：T_0..T_k 递推公式与数值代入（新节点 x 列表），求和与乘加留学生手算；
  // 每行带选填输入（= t_col 审计，全填或全空）
  buildTRows(prob, a, b, k) {
    const rows = [];
    const fA = fmtNum(prob.f(a), 6);
    const fB = fmtNum(prob.f(b), 6);
    rows.push({
      key: 'T0',
      label: 'T₀',
      formula: '(b−a)/2 · [f(a)+f(b)]',
      substituted: `${fmtNum((b - a) / 2, 6)} · (${fA} + ${fB})`,
      hasInput: true,
      input: ''
    });
    for (let m = 1; m <= k; m++) {
      const steps = Math.pow(2, m);
      const h = (b - a) / steps;
      const newXs = [];
      for (let i = 1; i < steps; i += 2) {
        newXs.push(fmtNum(a + i * h, 6));
        if (newXs.length >= 6) break; // 列表太长截断显示
      }
      const listText = newXs.length >= 6
        ? newXs.slice(0, 5).join(', ') + ', …'
        : newXs.join(', ');
      rows.push({
        key: 'T' + m,
        label: `T${this._sub(m)}`,
        formula: `T${this._sub(m - 1)}/2 + h${this._sub(m)} · Σf(新节点)`,
        substituted: `h${this._sub(m)} = ${fmtNum(h, 6)}，新节点 ${Math.pow(2, m - 1)} 个：x = ${listText}`,
        hasInput: true,
        input: ''
      });
    }
    return rows;
  },

  // 外推三角表卡：T_i^(j) = (4^j·上 − 下)/(4^j−1)，权重数值代入、上下值是学生
  // 自己算出的低一列结果（页面不替算）；终值格（i=k, j=k）高亮
  buildExtRows(k) {
    const rows = [];
    const labelOf = (i, j) => {
      if (j === 0) return `T${this._sub(i)}`;
      if (j === 1) return `S${this._sub(i)}`;
      if (j === 2) return `C${this._sub(i)}`;
      return `T${this._sub(i)}^(${j})`;
    };
    for (let j = 1; j <= k; j++) {
      const factor = Math.pow(4, j);
      for (let i = j; i <= k; i++) {
        const upper = labelOf(i, j - 1);
        const lower = labelOf(i - 1, j - 1);
        rows.push({
          key: `E${j}_${i}`,
          label: labelOf(i, j),
          formula: `(4^${j}·${upper} − ${lower}) / (4^${j}−1)`,
          substituted: `(${factor}·${upper} − ${lower}) / ${factor - 1}`,
          isFinal: i === k && j === k
        });
      }
    }
    return rows;
  },

  // 下标数字转 Unicode（仅展示用）
  _sub(n) {
    const map = ['₀', '₁', '₂', '₃', '₄', '₅', '₆', '₇', '₈', '₉'];
    return String(n).split('').map(d => map[Number(d)]).join('');
  },

  // ===== 交互 =====

  // 换题（换积分题库）：区间/表/画布全部重开
  onProblemChange(e) {
    const idx = Number(e.detail.value);
    const key = PROBLEM_KEYS[idx];
    if (!key || !integProblems[key]) return;
    this.setData({ config: { ...this.data.config, problem_key: key, k: 2 } });
    this.applyProblem(key);
  },

  onAInput(e) { this.setData({ aText: e.detail.value }); },
  onBInput(e) { this.setData({ bText: e.detail.value }); },
  onKInput(e) { this.setData({ kText: e.detail.value }); },

  // 应用自定义区间/k：数值合法即重算节点表与公式卡（提交时随 user_config 上报）
  applyParamsInput() {
    const a = parseNum(this.data.aText);
    const b = parseNum(this.data.bText);
    const k = parseIntStrict(this.data.kText);
    if (a === null || b === null) {
      wx.showToast({ title: '区间端点须为数字', icon: 'none' });
      return;
    }
    if (k === null) {
      wx.showToast({ title: 'k 须为整数', icon: 'none' });
      return;
    }
    if (this.applyParams(a, b, k)) {
      wx.showToast({ title: '已按新参数重开表格', icon: 'success' });
    }
  },

  // 梯形列审计输入（选填，全填或全空才上报）
  onTColInput(e) {
    const idx = Number(e.currentTarget.dataset.index);
    this.setData({ [`tRows[${idx}].input`]: e.detail.value });
  },

  onValueInput(e) { this.setData({ valueText: e.detail.value }); },

  // 提交读数评分：value 必填；t_col 选填随 readings 上报作审计
  async submitReadings() {
    if (this.data.submitting) return;
    const { levelId, valueText, tRows, aApplied, bApplied, kApplied, config } = this.data;
    const value = parseFloat(valueText);
    if (valueText === '' || !isFinite(value)) {
      wx.showToast({ title: '请填写你递推出的终值', icon: 'none' });
      return;
    }
    const reading = { value };
    // t_col 选填：全填或全空（长度须 k+1），填了才上报（仅审计，不参与打分）
    const tCol = tRows.map(r => parseFloat(r.input));
    const anyFilled = tRows.some(r => String(r.input).trim() !== '');
    if (anyFilled) {
      if (tCol.some((v, i) => String(tRows[i].input).trim() !== '' && !isFinite(v))) {
        wx.showToast({ title: 'T 列须为数字', icon: 'none' });
        return;
      }
      if (tCol.some((v, i) => String(tRows[i].input).trim() === '')) {
        wx.showToast({ title: 't_col 要么全填要么全空', icon: 'none' });
        return;
      }
      reading.t_col = tCol;
    }

    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit',
        method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 19,
          experiment: 'romberg_integration',
          readings: [reading],
          // 携带实际出题参数：换题/换区间/换 k 时后端按此重算龙贝格真值（契约 §12）
          user_config: {
            problem_key: config.problem_key,
            a: aApplied,
            b: bApplied,
            k: kApplied
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
      console.error('[romberg] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally {
      this.setData({ submitting: false });
    }
  },

  // ===== 图像视图：全屏 / 缩放 / 平移 =====

  // x 处第 k 级梯形折线的值（相邻分点线性插值，龙贝格最后一行 T_k 的几何形状）
  trapezoidY(x) {
    const a = this.data.aApplied, b = this.data.bApplied;
    const n = Math.pow(2, this.data.kApplied);
    const h = (b - a) / n;
    let i = Math.floor((x - a) / h);
    if (i < 0) i = 0;
    if (i > n - 1) i = n - 1;
    const x0 = a + i * h;
    const x1 = x0 + h;
    const y0 = this._prob.f(x0);
    const y1 = this._prob.f(x1);
    return y0 + (y1 - y0) * (x - x0) / h;
  },

  // 初始视野：题库横轴范围，y 取 f 与第 k 级梯形折线采样的并集再外扩 10%
  computeView() {
    const prob = this._prob || integProblems.sin_pi;
    const a = this.data.aApplied, b = this.data.bApplied;
    const span = b - a;
    const xMin = a - span * 0.12, xMax = b + span * 0.12;
    const N = 200;
    let yMin = Infinity, yMax = -Infinity;
    for (let k = 0; k <= N; k++) {
      const x = xMin + (xMax - xMin) * k / N;
      const yf = prob.f(x);
      if (isFinite(yf)) {
        yMin = Math.min(yMin, yf);
        yMax = Math.max(yMax, yf);
      }
      if (x >= a && x <= b) {
        const yt = this.trapezoidY(x);
        if (isFinite(yt)) {
          yMin = Math.min(yMin, yt);
          yMax = Math.max(yMax, yt);
        }
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
            console.error('[romberg] 全屏 canvas 节点获取失败');
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

  // 在指定 canvas 上绘制：f 曲线、0..k 各级梯形折线（逐次减半）、T_k 面积阴影、
  // 分点、区间端点与刻度网格
  _drawOn(canvas, ctx, w, h, which) {
    if (!this._view) this._view = this.computeView();
    const view = this._view;
    const prob = this._prob || integProblems.sin_pi;
    const f = prob.f;
    const a = this.data.aApplied, b = this.data.bApplied, k = this.data.kApplied;
    const nK = Math.pow(2, k); // 第 k 级梯形段数

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

    // T_k 面积阴影：第 k 级梯形折线与 x 轴围成的面积即 T_k 的几何含义
    ctx.fillStyle = 'rgba(230, 126, 34, 0.12)';
    ctx.beginPath();
    ctx.moveTo(X(a), clampY(Y(f(a))));
    for (let i = 1; i <= nK; i++) {
      const x = a + (b - a) * i / nK;
      ctx.lineTo(X(x), clampY(Y(f(x))));
    }
    ctx.lineTo(X(b), clampY(Y(0)));
    ctx.lineTo(X(a), clampY(Y(0)));
    ctx.closePath();
    ctx.fill();

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

    // 0..k−1 级梯形折线（灰，透明度随层数加深：逐次减半的过程可视化）
    for (let m = 0; m < k; m++) {
      const steps = Math.pow(2, m);
      const alpha = 0.18 + 0.32 * (m / k);
      ctx.strokeStyle = `rgba(148, 163, 184, ${alpha.toFixed(2)})`;
      ctx.lineWidth = 1.5;
      ctx.setLineDash([]);
      ctx.beginPath();
      for (let i = 0; i <= steps; i++) {
        const x = a + (b - a) * i / steps;
        const py = clampY(Y(f(x)));
        if (i === 0) ctx.moveTo(X(x), py);
        else ctx.lineTo(X(x), py);
      }
      ctx.stroke();
    }

    // 第 k 级梯形折线（橙虚线：龙贝格表最后一行 T_k）
    ctx.strokeStyle = '#e67e22';
    ctx.lineWidth = 2;
    ctx.setLineDash([6, 4]);
    ctx.beginPath();
    for (let i = 0; i <= nK; i++) {
      const x = a + (b - a) * i / nK;
      const py = clampY(Y(f(x)));
      if (i === 0) ctx.moveTo(X(x), py);
      else ctx.lineTo(X(x), py);
    }
    ctx.stroke();
    ctx.setLineDash([]);

    // 分点散点（橙心白环，位于梯形折线与 f 的交点上；太密时抽稀）
    const dotEvery = nK <= 32 ? 1 : Math.ceil(nK / 32);
    for (let i = 0; i <= nK; i += dotEvery) {
      const x = a + (b - a) * i / nK;
      const py = clampY(Y(f(x)));
      ctx.beginPath();
      ctx.arc(X(x), py, 4.5, 0, Math.PI * 2);
      ctx.fillStyle = '#e67e22';
      ctx.fill();
      ctx.lineWidth = 1.5;
      ctx.strokeStyle = '#fff';
      ctx.stroke();
    }

    // 区间端点 a/b：绿竖线（积分上下限）
    [a, b].forEach((xv, idx) => {
      const pX = X(xv);
      if (pX < mL - 4 || pX > mL + pw + 4) return;
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
      ctx.fillText(idx === 0 ? 'a' : 'b', clampX(pX), mT + 12);
    });

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
    ctx.fillText(`梯形 T${this._sub(k)} ×${nK}`, lx + 22, ly + 4);
    lx += 118;
    ctx.strokeStyle = 'rgba(148, 163, 184, 0.5)';
    ctx.lineWidth = 1.5;
    ctx.setLineDash([]);
    ctx.beginPath();
    ctx.moveTo(lx, ly);
    ctx.lineTo(lx + 18, ly);
    ctx.stroke();
    ctx.fillStyle = 'rgba(255, 255, 255, 0.6)';
    ctx.fillText('更早层 T_m', lx + 22, ly + 4);

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
