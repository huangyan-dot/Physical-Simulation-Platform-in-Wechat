// pages/experiments/newton-ring/newton-ring.js
import { request } from '../../../utils/request';
import { getTheory } from '../../../utils/theory';
import { pickQuiz, checkAnswer, QUIZ_SIZE } from '../../../utils/quiz';

// 简化后的操作模型：不做调焦、不调光路，进页面就是一幅正对环心的牛顿环。
// 十字叉丝固定在视场正中，学生点图片两侧的 ‹ › 让整幅环平移，
// 等效于真实实验里转动测微鼓轮带动镜筒横向移动。

// 环心在显微镜主尺上的位置（mm）。取 25 是为了让读数落在 0~50mm 量程中段，
// 主尺读数看起来像真仪器（21~29mm）。这个值学生不可见。
const CENTER_MM = 25.0;
// 视场宽度（mm）。真显微镜视场很窄，装不下高级次的环，必须转鼓轮扫过去，
// 这正是本实验要练的操作，所以不能为了「好看」把整幅环塞进一屏。
// 取 3.2mm 是算过的折中：初始正对环心时能看到 k=1~5 五个环，认得出是牛顿环；
// 同时放大率够高，第 25 环与第 26 环在屏上还能差出 6.5px，看得出是两个环。
// 再宽（4.0mm）高级次环就糊成一团灰，学生根本看不清在对准哪个环。
const FIELD_SPAN_MM = 3.2;
// 点一次 ‹ › 平移的距离（mm）。刻意取成「均匀步长」：
// 环间距随级数增大而变密（k=1→2 要 59 步，k=20→21 只要 16 步），
// 学生点着就能自己感觉到「环是不均匀的」。
// 取 0.005 而非 0.01：步长就是对准的量化误差，0.01 会让算出的 R 有 1% 的
// 系统偏差且无法靠多测几环消掉；0.005 把中位误差压到 0.1% 以内。
const STEP_MM = 0.005;
// 判定「对准外切线」的容差（mm）。必须 ≥ STEP_MM/2，
// 否则有些环会正好落在步长网格的缝里，学生怎么点都对不准。
const ALIGN_TOL_MM = 0.003;
// 各环半径的固有偏差（mm）：镜面加工精度与牛顿环装置压紧程度带来的偏离。
// 同一个环，画出来的圆和判定用的外切线共用这一个偏差，
// 保证「看到的」和「测到的」是同一个东西。
const RADIUS_SIGMA = 0.002;
// 参与绘制与对准判定的最高环级。取 25 是教材常用测量范围；
// 更高级次在屏幕分辨率下已经分不开，判定「对准第 40 环」是骗人的。
const MAX_K = 25;
// 鼓轮行程边界（mm）：刚好覆盖第 25 环两侧外切线，
// 让整个可走范围都对应得到能测的环，不会滑到一片没有环的地方。
const POS_MIN = 21.3;
const POS_MAX = 28.7;
// 长按连续移动的间隔（ms）
const HOLD_INTERVAL = 60;
// 长按加速：每 HOLD_RAMP 个 tick 把单次步数加 1，上限 HOLD_MAX_MULT 步。
// 不加速的话，0.005mm 一步从环心走到第 25 环要按 19 秒；
// 加速后 3.8 秒走完，而松手后单点仍是 0.005mm，粗调细调都有。
// 这也正是真鼓轮的手感：先快摇过去，再慢慢对准。
const HOLD_RAMP = 3;
const HOLD_MAX_MULT = 24;

Page({
  data: {
    tab: 'theory', // theory | experiment | quiz
    levelId: null,
    assignmentId: null,
    experimentId: null,
    config: {
      wavelength_nm: 589.3,
      lens_radius_mm: 855,
      k_range: [1, 10],
      tolerance_mm: 0.02
    },
    theory: null,

    // 叉丝在主尺上的位置（mm）。学生看不到这个总数，
    // 只看到拆开的主尺读数与鼓轮格数，合计要他自己加。
    pos: CENTER_MM,
    // 学生当前在读哪一侧的环
    readSide: 'left',
    // 对准状态：{ k } 或 null
    aligned: null,
    // 拆分显示的读数
    mainScale: '25',
    drumDiv: '0.0',

    // 学生手填的数据表：{ round, kLeft, readLeft, kRight, readRight, diameter }
    rounds: [],
    // 学生自己用平方差法算出的 R（mm）。页面不代算，只在确认后揭示真值与误差。
    rInput: '',
    finalR: null,
    errorInfo: null,
    submitting: false,

    // 两段式提交（与单摆一致）：先交测量数据拿到数据分，再做自测拿自测分，
    // 两项按权重合成综合分。没交测量数据之前自测是锁住的。
    dataSubmitted: false,
    dataScore: null,
    comboScore: null,
    bestScores: null,
    quizSubmitting: false,
    // 综合得分比例。作业模式下由教师设定（后端默认 60:40），
    // 拿不到就用这两个默认值，不阻塞实验。
    dataWeight: 60,
    quizWeight: 40,

    canvasWidth: 0,
    canvasHeight: 0,
    pixelRatio: 1,

    // 自测
    quiz: [],
    quizAnswers: {},
    quizState: {},
    quizScore: null,
    quizSize: QUIZ_SIZE
  },

  _canvas: null,
  _ctx: null,
  // 每个环半径的固有偏差，进页面时按需生成并缓存，
  // 保证同一个环反复对准得到同一个读数
  _radiusNoise: {},
  _holdTimer: 0,

  onLoad(options) {
    const levelId = options.levelId;
    const assignmentId = options.assignmentId || null;
    this.setData({
      levelId,
      assignmentId,
      theory: getTheory('newton_ring'),
      quiz: pickQuiz('newton_ring')
    });
    this.initCanvas();
    if (levelId) {
      this.loadExperimentConfig(levelId);
    }
    if (assignmentId) {
      this.loadAssignmentWeight(assignmentId);
    }
  },

  // 取本次作业的综合得分比例（教师可调，默认 60:40）
  async loadAssignmentWeight(assignmentId) {
    try {
      const rows = await request({ url: '/assignments/mine', method: 'GET', showLoading: false });
      const list = Array.isArray(rows) ? rows : [];
      const mine = list.find((a) => String(a.id) === String(assignmentId));
      if (mine && mine.data_weight > 0) {
        this.setData({
          dataWeight: mine.data_weight,
          quizWeight: 100 - mine.data_weight
        });
      }
    } catch (err) {
      // 拿不到就用默认 60:40，不阻塞实验
      console.error('[newton-ring] 加载作业权重失败，用默认 60:40:', err);
    }
  },

  onReady() {
    // canvas 在 tab === 'experiment' 的 wx:if 里，首屏是「原理」时节点还不存在，
    // 所以这里只在已经处于实验页时初始化；否则交给 switchTab。
    if (this.data.tab === 'experiment') this.setupCanvas();
  },

  onHide() {
    this.releaseHold();
  },

  onUnload() {
    this.releaseHold();
  },

  initCanvas() {
    const sysInfo = wx.getSystemInfoSync();
    const width = sysInfo.windowWidth;
    const winH = sysInfo.windowHeight || 667;
    // 视场不滚动，占屏高的比例必须给下方读数条和数据表留够空间：
    // 最多 40% 屏高，再夹在 200~380px。
    const height = Math.min(width * 0.86, winH * 0.4, 380);
    this.setData({
      canvasWidth: width,
      canvasHeight: Math.round(Math.max(200, height)),
      pixelRatio: sysInfo.pixelRatio || 1
    });
  },

  setupCanvas(retry = 0) {
    wx.nextTick(() => {
      const query = wx.createSelectorQuery().in(this);
      query.select('#ringCanvas').fields({ node: true, size: true }).exec((res) => {
        const node = res && res[0] && res[0].node;
        if (!node) {
          // wx:if 刚切过来时节点可能还没挂上，重试几次
          if (retry < 5) setTimeout(() => this.setupCanvas(retry + 1), 60);
          else console.error('[newton-ring] canvas 节点获取失败');
          return;
        }
        const { canvasWidth: w, canvasHeight: h, pixelRatio } = this.data;
        this._canvas = node;
        this._ctx = node.getContext('2d');
        // 尺寸只设一次：每次重设 width 都会清空画布并重置变换矩阵
        node.width = w * pixelRatio;
        node.height = h * pixelRatio;
        this._ctx.setTransform(pixelRatio, 0, 0, pixelRatio, 0, 0);
        this.refreshReading();
      });
    });
  },

  switchTab(e) {
    const tab = e.currentTarget.dataset.tab;
    // 自测在测量数据提交之后才开放：综合分要先有数据分才能算
    if (tab === 'quiz' && !this.data.dataSubmitted) {
      wx.showToast({ title: '先提交测量数据才能进入自测', icon: 'none' });
      return;
    }
    const leaving = this.data.tab === 'experiment' && tab !== 'experiment';
    this.releaseHold();
    this.setData({ tab }, () => {
      if (tab === 'experiment') {
        // 画布随 wx:if 重新创建，节点变了，必须重新取一次
        this.setupCanvas();
      } else if (leaving) {
        this._canvas = null;
        this._ctx = null;
      }
    });
  },

  async loadExperimentConfig(levelId) {
    try {
      const data = await request({ url: `/experiments/${levelId}`, method: 'GET' });
      const config = { ...this.data.config, ...(data.config || {}) };
      // 换了波长或曲率半径，环的位置全变了，缓存的半径偏差要作废
      this._radiusNoise = {};
      this.setData({ experimentId: data.id, config });
    } catch (err) {
      console.error('[newton-ring] 加载实验配置失败，使用页面默认参数:', err);
      wx.showToast({ title: '实验参数加载失败，使用默认值', icon: 'none' });
    }
    this.refreshReading();
  },

  // ===================== 环的几何 =====================

  // 第 k 级暗环的理论半径（mm）
  theoryRadius(k) {
    const { wavelength_nm, lens_radius_mm } = this.data.config;
    return Math.sqrt(k * wavelength_nm * 1e-6 * lens_radius_mm);
  },

  // 第 k 级暗环的实际半径（mm，含该环固有偏差）
  ringRadius(k) {
    if (this._radiusNoise[k] === undefined) {
      // Box–Muller 生成正态分布偏差
      const u1 = Math.random() || 1e-9;
      const u2 = Math.random();
      const z = Math.sqrt(-2 * Math.log(u1)) * Math.cos(2 * Math.PI * u2);
      this._radiusNoise[k] = z * RADIUS_SIGMA;
    }
    return this.theoryRadius(k) + this._radiusNoise[k];
  },

  // 第 k 级暗环某侧外切线在主尺上的位置（mm）
  tangentMM(k, side) {
    const r = this.ringRadius(k);
    return side === 'left' ? CENTER_MM - r : CENTER_MM + r;
  },

  // 每毫米对应的像素数
  pxPerMM() {
    return (this.fieldRadius() * 2) / FIELD_SPAN_MM;
  },

  // 视场圆半径（px）
  fieldRadius() {
    const { canvasWidth, canvasHeight } = this.data;
    return Math.min(canvasWidth / 2, canvasHeight / 2) * 0.92;
  },

  // ===================== 读数 =====================

  // 找当前侧离叉丝最近的暗环外切线
  nearestTangent(side) {
    const pos = this.data.pos;
    let bestK = 0;
    let bestD = Infinity;
    for (let k = 1; k <= MAX_K; k++) {
      const d = Math.abs(pos - this.tangentMM(k, side));
      if (d < bestD) { bestD = d; bestK = k; }
    }
    return { k: bestK, delta: bestD };
  },

  // 刷新对准状态与拆分读数，并重绘
  refreshReading() {
    const { pos, readSide } = this.data;
    const near = this.nearestTangent(readSide);
    const aligned = near.delta <= ALIGN_TOL_MM ? { k: near.k } : null;

    // 读数显微镜：主尺 1mm 分度读到整毫米，测微鼓轮一周 1mm 分 100 格，
    // 每格 0.01mm，再估读到格的十分之一（0.001mm）。
    const main = Math.floor(pos);
    const div = (pos - main) * 100;

    this.setData({
      aligned,
      mainScale: String(main),
      drumDiv: div.toFixed(1)
    });
    this.drawRings();
  },

  // ===================== 平移操作 =====================

  // 点左边的 ‹：牛顿环整体左移，右侧就会露出更高级次的环
  stepLeft() {
    this.movePos(STEP_MM);
  },

  stepRight() {
    this.movePos(-STEP_MM);
  },

  movePos(delta) {
    const pos = Math.max(POS_MIN, Math.min(POS_MAX, this.data.pos + delta));
    // 夹到边界后就别再 setData 了，否则长按到底会一直空转
    if (Math.abs(pos - this.data.pos) < 1e-9) return;
    // 吸附到 STEP_MM 网格上：所有环的外切线都是按这个网格算的可达性，
    // 一旦位置偏离网格，某些环就再也对不准了。
    const snapped = Math.round(pos / STEP_MM) * STEP_MM;
    this.setData({ pos: snapped });
    this.refreshReading();
  },

  // 长按连续移动，速度逐渐加快（见 HOLD_RAMP 说明）
  holdLeft() {
    this.startHold(1);
  },

  holdRight() {
    this.startHold(-1);
  },

  startHold(dir) {
    this.releaseHold();
    let tick = 0;
    this._holdTimer = setInterval(() => {
      const mult = Math.min(HOLD_MAX_MULT, 1 + Math.floor(tick / HOLD_RAMP));
      tick++;
      this.movePos(dir * mult * STEP_MM);
    }, HOLD_INTERVAL);
  },

  releaseHold() {
    if (this._holdTimer) {
      clearInterval(this._holdTimer);
      this._holdTimer = 0;
    }
  },

  onSideChange(e) {
    const readSide = e.currentTarget.dataset.side;
    if (readSide === this.data.readSide) return;
    this.setData({ readSide });
    this.refreshReading();
  },

  // ===================== 绘制 =====================

  drawRings() {
    const canvas = this._canvas;
    const ctx = this._ctx;
    if (!canvas || !ctx) return;

    const { canvasWidth: W, canvasHeight: H, config, pos } = this.data;
    const cx = W / 2;
    const cy = H / 2;
    const R = this.fieldRadius();
    const scale = this.pxPerMM();

    ctx.fillStyle = '#050505';
    ctx.fillRect(0, 0, W, H);

    // 视场圆内才有像
    ctx.save();
    ctx.beginPath();
    ctx.arc(cx, cy, R, 0, Math.PI * 2);
    ctx.clip();

    const grad = ctx.createRadialGradient(cx, cy, 0, cx, cy, R);
    grad.addColorStop(0, '#2a1e0a');
    grad.addColorStop(1, '#0a0602');
    ctx.fillStyle = grad;
    ctx.fillRect(0, 0, W, H);

    const rgb = this.wavelengthToRGB(config.wavelength_nm);
    // 环心随 pos 平移：pos 增大 → 整幅环左移
    const ringCx = cx + (CENTER_MM - pos) * scale;
    const dist = Math.abs(cx - ringCx);

    for (let k = 1; k <= MAX_K; k++) {
      const rp = this.ringRadius(k) * scale;
      // 该环整圈都在视场之外就不用画了
      if (Math.abs(dist - rp) > R) continue;

      const alpha = Math.max(0.15, 0.8 - k * 0.014);
      // 线宽跟着本地环间距收窄：高级次环间距只有 6~7px，
      // 若还按固定 5~7px 画，暗环和亮环会糊成一片灰，学生看不出在对准哪个环。
      const gapPx = (this.ringRadius(k + 1) - this.ringRadius(k)) * scale;
      const lw = Math.min(6, Math.max(0.8, gapPx * 0.4));

      // 亮环画在相邻两暗环中间
      const rnp = this.ringRadius(k + 0.5) * scale;
      if (Math.abs(dist - rnp) <= R) {
        ctx.strokeStyle = `rgba(${rgb[0]},${rgb[1]},${rgb[2]},${alpha * 0.75})`;
        ctx.lineWidth = lw;
        ctx.beginPath();
        ctx.arc(ringCx, cy, rnp, 0, Math.PI * 2);
        ctx.stroke();
      }

      // 暗环
      ctx.strokeStyle = `rgba(0,0,0,${alpha})`;
      ctx.lineWidth = lw;
      ctx.beginPath();
      ctx.arc(ringCx, cy, rp, 0, Math.PI * 2);
      ctx.stroke();
    }

    // 中心暗斑
    if (dist < R) {
      ctx.fillStyle = 'rgba(0,0,0,0.9)';
      ctx.beginPath();
      ctx.arc(ringCx, cy, Math.max(3, this.theoryRadius(0.6) * scale), 0, Math.PI * 2);
      ctx.fill();
    }

    ctx.restore();

    // 视场边框
    ctx.strokeStyle = '#333';
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.arc(cx, cy, R, 0, Math.PI * 2);
    ctx.stroke();

    // 十字叉丝：固定在视场正中，竖丝就是用来卡外切线的
    const on = !!this.data.aligned;
    ctx.strokeStyle = on ? 'rgba(80,255,140,0.95)' : 'rgba(255,80,80,0.9)';
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(cx, 0);
    ctx.lineTo(cx, H);
    ctx.stroke();
    ctx.beginPath();
    ctx.moveTo(0, cy);
    ctx.lineTo(W, cy);
    ctx.stroke();
  },

  // 可见光波长 -> RGB（近似）
  wavelengthToRGB(nm) {
    let r = 0, g = 0, b = 0;
    if (nm < 440) { r = -(nm - 440) / 60; b = 1; }
    else if (nm < 490) { g = (nm - 440) / 50; b = 1; }
    else if (nm < 510) { g = 1; b = -(nm - 510) / 20; }
    else if (nm < 580) { r = (nm - 510) / 70; g = 1; }
    else if (nm < 645) { r = 1; g = -(nm - 645) / 65; }
    else { r = 1; }
    return [Math.round(r * 255), Math.round(g * 255), Math.round(b * 255)];
  },

  // ===================== 数据表 =====================

  // 新增一轮。只带出轮次号，六列数据全部由学生自己填。
  addRound() {
    const rounds = this.data.rounds.concat([{
      round: this.data.rounds.length + 1,
      kLeft: '', readLeft: '', kRight: '', readRight: '', diameter: ''
    }]);
    // 表一动，之前确认过的 R 与揭示的误差就作废，必须重新算一次
    this.setData({ rounds, finalR: null, errorInfo: null });
  },

  onRoundInput(e) {
    const idx = Number(e.currentTarget.dataset.idx);
    const field = e.currentTarget.dataset.field;
    this.setData({
      [`rounds[${idx}].${field}`]: e.detail.value,
      finalR: null,
      errorInfo: null
    });
  },

  deleteRound(e) {
    const idx = Number(e.currentTarget.dataset.idx);
    const rounds = this.data.rounds.filter((_, i) => i !== idx);
    // 删掉中间一行后重新编号，保持 1..n 连续
    rounds.forEach((r, i) => { r.round = i + 1; });
    this.setData({ rounds, finalR: null, errorInfo: null });
  },

  // 把学生填的表整理成可计算的行，顺带做合法性检查
  validRows() {
    const rows = [];
    for (const r of this.data.rounds) {
      const kL = parseInt(r.kLeft, 10);
      const kR = parseInt(r.kRight, 10);
      const D = parseFloat(r.diameter);
      if (!kL || !kR || !(D > 0)) continue;
      if (kL !== kR) continue; // 左右不是同一个环，这一行算不出直径
      rows.push({ k: kL, D });
    }
    return rows;
  },

  onRInput(e) {
    this.setData({ rInput: e.detail.value, finalR: null, errorInfo: null });
  },

  // 确认最终结果：此时才揭示透镜曲率半径真值与误差率。
  // 页面不替学生算 R——平方差法的代入过程本身就是这个实验要考的东西。
  confirmFinalR() {
    const R = parseFloat(this.data.rInput);
    if (!isFinite(R) || R <= 0) {
      wx.showToast({ title: '请填写你算出的曲率半径 R', icon: 'none' });
      return;
    }
    // 量级护栏：把「λ 忘了换算」「R 填成 m 或 cm」这类单位错误挡在提交之前。
    // 上下限跨了四个数量级，不会漏题给答案。
    if (R < 10 || R > 100000) {
      wx.showToast({ title: 'R 请以 mm 为单位填写，注意数量级', icon: 'none' });
      return;
    }
    const rows = this.validRows();
    if (rows.length < 2) {
      wx.showToast({ title: '至少要两行左右环级相同且填了直径的数据', icon: 'none' });
      return;
    }
    const ks = new Set(rows.map((r) => r.k));
    if (ks.size < 2) {
      wx.showToast({ title: '平方差法需要两个不同的环级数', icon: 'none' });
      return;
    }

    const trueR = this.data.config.lens_radius_mm;
    const relErr = (Math.abs(R - trueR) / trueR) * 100;
    // 分级门槛与后端 newtonScoreFromRelErr 的曲线对齐：
    // ≤0.5% 满分，2% 约 88，5% 约 65，8% 已低于及格线。
    this.setData({
      finalR: R.toFixed(1),
      errorInfo: {
        trueR: trueR.toFixed(0),
        relErr: relErr.toFixed(2),
        level: relErr <= 0.5 ? 'good' : (relErr <= 2 ? 'ok' : (relErr <= 5 ? 'warn' : 'bad')),
        comment:
          relErr <= 0.5 ? '很好，误差在 0.5% 以内，属于规范操作的水平' :
          relErr <= 2 ? '接近，可检查读数是否估读到 0.001mm，以及两个环级是否相差得足够大' :
          relErr <= 5 ? '偏大，请检查用的是直径 D 还是半径，以及 λ 是否换成了 mm（589.3nm = 5.893×10⁻⁴mm）' :
          '误差过大，请重新核对 R = (D²ₘ − D²ₙ)/[4(m − n)λ] 的代入过程，各量单位统一到 mm'
      }
    });
  },

  async submitReadings() {
    const { levelId, assignmentId, finalR, rounds } = this.data;
    // 必须先确认最终结果：提交的就是学生自己算出的那个 R
    if (!finalR) {
      wx.showToast({ title: '请先填写并确认曲率半径 R', icon: 'none' });
      return;
    }
    const rows = this.validRows();
    if (rows.length < 2) {
      wx.showToast({ title: '至少要两行完整数据', icon: 'none' });
      return;
    }

    // 后端按 calc_r 评分；各环的 {k, r} 一并带上，只作记录与回落用。
    // calc_r 挂在第一条上，rounds/selected_count 让教师看得出取舍过程。
    const payload = rows.map((d, i) => (i === 0
      ? {
        k: d.k,
        r: Number((d.D / 2).toFixed(4)),
        calc_r: Number(parseFloat(finalR).toFixed(3)),
        rounds: rounds.length,
        selected_count: rows.length
      }
      : { k: d.k, r: Number((d.D / 2).toFixed(4)) }
    ));

    this.setData({ submitting: true });
    try {
      const url = assignmentId ? `/assignments/${assignmentId}/submit` : '/progress/submit';
      const data = await request({
        url,
        method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 1,
          experiment: 'newton_ring',
          readings: payload
        }
      });

      // 作业接口返回综合分（此时自测还没做，自测分沿用历史值），
      // 练习接口只返回数据分
      const dataScore = assignmentId ? data.data_score : data.score;
      this.setData({
        dataSubmitted: true,
        dataScore,
        bestScores: assignmentId ? data : null,
        dataWeight: assignmentId && data.data_weight ? data.data_weight : this.data.dataWeight,
        quizWeight: assignmentId && data.data_weight ? 100 - data.data_weight : this.data.quizWeight
      });

      wx.showModal({
        title: '测量数据已提交',
        content:
          `测量数据得分：${dataScore}\n` +
          `接下来完成自测题目，按 测量${this.data.dataWeight}% : 自测${this.data.quizWeight}% 计入综合得分。`,
        confirmText: '去自测',
        cancelText: '稍后',
        success: (res) => {
          // 跳到自测页，画布随 wx:if 卸载，句柄要一起清掉
          if (res.confirm) {
            this.releaseHold();
            this.setData({ tab: 'quiz' }, () => {
              this._canvas = null;
              this._ctx = null;
            });
          }
        }
      });
    } catch (err) {
      console.error('[newton-ring] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally {
      this.setData({ submitting: false });
    }
  },

  // ===================== 自测 =====================

  onQuizChoice(e) {
    const { idx } = e.currentTarget.dataset;
    this.setData({ [`quizAnswers.${idx}`]: Number(e.detail.value) });
  },

  onQuizJudge(e) {
    const { idx, val } = e.currentTarget.dataset;
    this.setData({ [`quizAnswers.${idx}`]: val === 'true' });
  },

  onQuizInput(e) {
    const { idx } = e.currentTarget.dataset;
    this.setData({ [`quizAnswers.${idx}`]: e.detail.value });
  },

  checkOne(e) {
    const idx = Number(e.currentTarget.dataset.idx);
    const q = this.data.quiz[idx];
    const ans = this.data.quizAnswers[idx];
    if (ans === undefined || ans === '') {
      wx.showToast({ title: '请先作答', icon: 'none' });
      return;
    }
    const ok = checkAnswer(q, ans);
    const prev = this.data.quizState[idx];
    let state;
    if (ok) state = 'right';
    else state = prev === 'hint' || prev === 'wrong' ? 'wrong' : 'hint';
    this.setData({ [`quizState.${idx}`]: state });
  },

  async submitQuiz() {
    const { quiz, quizAnswers, assignmentId, dataScore, dataWeight, quizWeight } = this.data;
    let correct = 0;
    const state = {};
    quiz.forEach((q, i) => {
      const ok = checkAnswer(q, quizAnswers[i]);
      if (ok) correct++;
      state[i] = ok ? 'right' : 'wrong';
    });
    const score = Math.round((correct / quiz.length) * 100);
    // 本地先按当前权重合成，作业模式下会被后端返回值覆盖
    const combo = Math.round((dataScore * dataWeight + score * quizWeight) / 100);
    this.setData({ quizState: state, quizScore: score, comboScore: combo });

    if (!assignmentId) {
      wx.showModal({
        title: '自测完成',
        content:
          `答对 ${correct}/${quiz.length} 题，自测得分 ${score}\n` +
          `测量数据 ${dataScore} × ${dataWeight}% + 自测 ${score} × ${quizWeight}%\n` +
          `综合得分：${combo}`,
        showCancel: false
      });
      return;
    }

    // 作业模式：把自测分回传，由后端按教师设定的权重算综合分并入库
    this.setData({ quizSubmitting: true });
    try {
      const data = await request({
        url: `/assignments/${assignmentId}/submit`,
        method: 'POST',
        data: { experiment: 'newton_ring', quiz_score: score }
      });
      this.setData({
        comboScore: data.score,
        bestScores: data,
        dataWeight: data.data_weight,
        quizWeight: 100 - data.data_weight
      });
      wx.showModal({
        title: '自测完成',
        content:
          `答对 ${correct}/${quiz.length} 题\n` +
          `测量数据 ${data.data_score} × ${data.data_weight}%\n` +
          `自测题目 ${data.quiz_score} × ${100 - data.data_weight}%\n` +
          `本次综合得分：${data.score}\n` +
          `历史最佳综合分：${data.best_score}`,
        showCancel: false
      });
    } catch (err) {
      console.error('[newton-ring] 自测提交失败:', err);
      wx.showToast({ title: err.message || '自测提交失败', icon: 'none' });
    } finally {
      this.setData({ quizSubmitting: false });
    }
  },

  resetQuiz() {
    this.setData({
      quiz: pickQuiz('newton_ring'),
      quizAnswers: {},
      quizState: {},
      quizScore: null,
      comboScore: null
    });
  }
});
