// pages/experiments/pendulum/pendulum.js
import { request } from '../../../utils/request';
import { getTheory } from '../../../utils/theory';
import { pickQuiz, checkAnswer, QUIZ_SIZE } from '../../../utils/quiz';
import {
  provinceNames,
  countyNames,
  regionByIndex,
  nearestRegion,
  defaultRegion
} from '../../../utils/region';

// 悬点位置的系统偏差（m）：夹头夹持点与米尺零点对不齐、摆线微伸长，
// 使真实摆长与学生按 L = l + d/2 算出的值有约 0.8mm 的差。
// 这一项进页面固定一次——同一套装置的系统误差不会自己变。
const LENGTH_SIGMA = 0.0008;
// 每轮重新拉开摆球时夹头的微小位移（m，约 1.5mm）。
// 这是「轮与轮之间」的随机误差：正因为有它，各轮算出的 g 才会有零点几个百分点的
// 离散，多测几轮取平均才有意义。少了这一项，每轮结果完全相同，数据表就没有存在价值。
const ROUND_LENGTH_SIGMA = 0.0015;
// 触发分辨误差（s）：释放瞬间自动起表、计满周期自动停表，没有人工反应误差，
// 但释放检测与周期计数仍有约 20ms 的分辨极限。累积法（增大 n）就是靠
// 把这个固定误差摊到 n 个周期上来提高 T 的精度。
const TRIGGER_SIGMA = 0.02;

// 演示倍速档位。秒表显示的始终是「实验时间」，与倍速无关，
// 所以加速只是少等，不影响任何测量值。
const SPEEDS = [1, 2, 5, 10];

// 3D 视角
const ELEVATION = 0.22;      // 俯视倾角（弧度），固定
const FOCAL = 900;           // 透视焦距（像素）
const SWIPE_SENSITIVITY = 0.011; // 每像素滑动转多少弧度

// 摆球的视觉放大倍数。真实比例下 2cm 的球挂在 1m 线上只有约 3px，
// 学生根本看不出直径变化。放大后仍严格随输入的 d 单调变化，
// 但绘制尺寸不再与摆线等比——界面上已注明这一点，避免学生误读几何关系。
const BALL_VISUAL_GAIN = 6;

Page({
  data: {
    tab: 'theory',
    levelId: null,
    assignmentId: null,
    config: { length_m: 1.0, angle_deg: 5, gravity: 9.8 },
    theory: null,

    // ===== 地区与当地 g =====
    region: null,
    locating: true,
    locateHint: '',
    regionPickerVisible: false,
    provinceList: [],
    countyList: [],
    pickerValue: [0, 0],

    // ===== 装置几何（学生设置，直接驱动上方 3D 装置）=====
    lineLength: '100.0',   // 悬线长 l (cm)
    ballDiameter: '2.00',  // 摆球直径 d (cm)
    measuredL: null,       // 学生算出的摆长 L (cm)

    // ===== 摆动与计时 =====
    swinging: false,
    timing: false,
    elapsed: '0.00',       // 秒表读数（实验时间，s）
    periodNow: 0,          // 实时周期数（装置右上角）
    periodCount: 30,       // 目标计数周期
    speedIndex: 2,         // 默认 5×
    speedText: '5×',

    // ===== 学生自己填的数据表 =====
    // rounds[i] = { round, angle, period, g, selected, tMeasured, nMeasured }
    // 角度/周期/g 三列都由学生填写，程序只提供秒表读数供他换算。
    plannedRounds: 3,      // 学生打算做几轮（只是计划，随时可多做或少做）
    rounds: [],
    roundDone: false,      // 本轮已摆完、等学生填表
    lastRun: null,         // 刚跑完那一轮的秒表读数（n 与总时间）
    selectedCount: 0,
    selectedLabel: '',     // 「已选用第 1、3、4 轮」
    avgGInput: '',         // 学生手填的 g 平均值
    finalG: null,          // 确认后的最终 g
    errorInfo: null,       // 确认后才出现：当地 g 与误差率

    // ===== 结果与提交 =====
    submitting: false,
    dataSubmitted: false,  // 提交测量数据后才解锁「自测」
    dataScore: null,

    // ===== 3D 视角 =====
    azimuthDeg: 35,
    canvasWidth: 0,
    canvasHeight: 0,
    pixelRatio: 1,

    // ===== 自测与综合得分 =====
    quiz: [],
    quizAnswers: {},
    quizState: {},
    quizScore: null,
    quizSize: QUIZ_SIZE,
    quizSubmitting: false,
    dataWeight: 60,
    quizWeight: 40,
    comboScore: null,
    bestScores: null
  },

  _canvas: null,
  _ctx: null,
  _rafId: 0,
  // 摆长真值的系统偏差，进页面固定一次
  _lNoise: null,
  // 本轮重新夹持摆球带来的摆长偏差，每次释放重抽
  _roundNoise: 0,
  // 起表反应误差，每次计时随机一次
  _startJitter: 0,

  // 仿真时钟（单位 s，"实验时间"）。倍速只改变它的推进速度。
  _simTime: 0,
  _lastFrameMs: 0,
  // 起摆时刻（仿真时间），用于算摆角相位
  _releaseSim: 0,
  // 本次计时的起点（仿真时间）
  _timeStartSim: 0,
  // 上次刷新 setData 的时间，节流用
  _lastPushMs: 0,
  // 触摸滑动起点
  _touchX: 0,
  _touchAz: 0,

  onLoad(options) {
    const levelId = options.levelId;
    const assignmentId = options.assignmentId || null;
    this.setData({
      levelId,
      assignmentId,
      theory: getTheory('pendulum'),
      quiz: pickQuiz('pendulum'),
      provinceList: provinceNames()
    });
    this.initCanvas();
    this.recalcL();
    this.resolveRegion();
    if (levelId) {
      this.loadExperimentConfig(levelId);
    }
    if (assignmentId) {
      this.loadAssignmentWeight(assignmentId);
    }
  },

  async loadExperimentConfig(levelId) {
    try {
      const data = await request({ url: `/experiments/${levelId}`, method: 'GET' });
      if (data.config) {
        this.setData({ config: { ...this.data.config, ...data.config } });
      }
    } catch (err) {
      console.error('[pendulum] 加载实验配置失败，使用页面默认参数:', err);
      wx.showToast({ title: '实验参数加载失败，使用默认值', icon: 'none' });
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
      console.error('[pendulum] 加载作业权重失败，用默认 60:40:', err);
    }
  },

  onReady() {
    // 画布在 tab === 'experiment' 的 wx:if 里，首屏是「原理」时节点还不存在，
    // 所以这里只在已经处于实验页时初始化；否则交给 switchTab。
    if (this.data.tab === 'experiment') this.setupCanvas();
  },

  onUnload() {
    this._stopAnimation();
    this._canvas = null;
    this._ctx = null;
  },

  // 切到后台就停掉渲染循环，回来再续上，避免白耗电
  onHide() {
    this._stopAnimation();
  },

  onShow() {
    if (this.data.tab === 'experiment' && this._canvas && !this._rafId) {
      this._lastFrameMs = Date.now();
      this.startAnimation();
    }
  },

  _stopAnimation() {
    if (this._rafId && this._canvas) this._canvas.cancelAnimationFrame(this._rafId);
    this._rafId = 0;
  },

  // 取画布节点并按 dpr 设好尺寸。节点渲染完成才拿得到，
  // 所以用 nextTick + 一次重试兜住慢机型。
  setupCanvas(retry = 0) {
    wx.nextTick(() => {
      const query = wx.createSelectorQuery().in(this);
      query.select('#pendulumCanvas')
        .fields({ node: true, size: true })
        .exec((res) => {
          const node = res && res[0] && res[0].node;
          if (!node) {
            if (retry < 5) {
              setTimeout(() => this.setupCanvas(retry + 1), 60);
            } else {
              console.error('[pendulum] canvas 节点获取失败');
            }
            return;
          }
          const { canvasWidth: w, canvasHeight: h, pixelRatio } = this.data;
          this._canvas = node;
          this._ctx = node.getContext('2d');
          // 尺寸只设一次：每帧重设 width 会清空画布并触发重新分配
          node.width = w * pixelRatio;
          node.height = h * pixelRatio;
          this._ctx.setTransform(pixelRatio, 0, 0, pixelRatio, 0, 0);
          this._lastFrameMs = Date.now();
          this._stopAnimation();
          this.startAnimation();
        });
    });
  },

  initCanvas() {
    const sysInfo = wx.getSystemInfoSync();
    const width = sysInfo.windowWidth;
    const winH = sysInfo.windowHeight || 667;
    // 画布固定不滚，占屏高的比例必须留够下方面板的滚动空间：
    // 最多 42% 屏高，再夹在 200~420px。下限取 200 而非 240，
    // 否则 iPhone SE 这类矮屏会把控制面板压到不足 150px。
    const height = Math.min(width * 1.0, winH * 0.42, 420);
    this.setData({
      canvasWidth: width,
      canvasHeight: Math.round(Math.max(200, height)),
      pixelRatio: sysInfo.pixelRatio || 1
    });
  },

  switchTab(e) {
    const tab = e.currentTarget.dataset.tab;
    if (tab === 'quiz' && !this.data.dataSubmitted) {
      wx.showToast({ title: '先提交测量数据才能进入自测', icon: 'none' });
      return;
    }
    const leaving = this.data.tab === 'experiment' && tab !== 'experiment';
    this.setData({ tab }, () => {
      if (tab === 'experiment') {
        // 画布随 wx:if 重新创建，节点变了，必须重新取一次
        this.setupCanvas();
      } else if (leaving) {
        this._stopAnimation();
        this._canvas = null;
        this._ctx = null;
      }
    });
  },

  // ===================== 地区与当地 g =====================

  // 进实验先取定位；失败/拒绝则回落到手选地区
  resolveRegion() {
    this.setData({ locating: true });
    wx.getLocation({
      type: 'gcj02',
      success: (res) => {
        // 定位是异步的，可能几秒后才回来。若这期间学生已经手动选过地区，
        // 就不能再拿定位结果去覆盖他的选择——否则看起来「只能用定位到的地区」。
        if (this._manualRegion) {
          this.setData({ locating: false });
          return;
        }
        const r = nearestRegion(res.latitude, res.longitude);
        if (!r) {
          this.applyRegion(defaultRegion(), '未能匹配到地区，已用默认地区，可手动选择');
          return;
        }
        this.applyRegion(r, `已定位到最近的县级地区（相距约 ${r.distanceKm} km），可手动更改`);
      },
      fail: (err) => {
        console.error('[pendulum] 定位失败:', err);
        if (this._manualRegion) {
          this.setData({ locating: false });
          return;
        }
        this.applyRegion(
          defaultRegion(),
          '未获取到定位，已用默认地区。请手动选择你所在的县/区，否则算出的 g 不是当地值'
        );
      }
    });
  },

  applyRegion(region, hint) {
    if (!region) return;
    this.setData({
      region,
      locating: false,
      locateHint: hint || '',
      pickerValue: [region.provinceIndex, region.countyIndex],
      countyList: countyNames(region.provinceIndex),
      // 换地区后当地 g 变了，已确认的结论不再对应同一基准，作废重算。
      // 表格里的原始数据保留——那是学生自己测的，不该被清掉。
      finalG: null,
      errorInfo: null
    });
  },

  openRegionPicker() {
    const r = this.data.region || defaultRegion();
    this.setData({
      regionPickerVisible: true,
      pickerValue: [r.provinceIndex, r.countyIndex],
      countyList: countyNames(r.provinceIndex)
    });
  },

  closeRegionPicker() {
    this.setData({ regionPickerVisible: false });
  },

  // 弹层内容区的空处理器：只用来 catchtap 拦住冒泡，不做任何事
  noop() {},

  // 省份列变化时要重置县列，否则下标会越界
  onRegionColumnChange(e) {
    const { column, value } = e.detail;
    if (column === 0) {
      this.setData({
        pickerValue: [value, 0],
        countyList: countyNames(value)
      });
    } else {
      this.setData({ pickerValue: [this.data.pickerValue[0], value] });
    }
  },

  confirmRegion() {
    const [pi, ci] = this.data.pickerValue;
    const r = regionByIndex(pi, ci, 'manual');
    if (!r) {
      wx.showToast({ title: '地区无效', icon: 'none' });
      return;
    }
    // 打上手选标记：晚到的定位回调不得再覆盖它
    this._manualRegion = true;
    this.setData({ regionPickerVisible: false });
    this.applyRegion(r, '已按你选择的地区计算当地重力加速度');
    wx.showToast({ title: `已切换到${r.county}`, icon: 'none' });
  },

  relocate() {
    // 学生主动要求重新定位，就放弃手选标记
    this._manualRegion = false;
    this.resolveRegion();
  },

  // 当地 g：优先用地区值，取不到才用后端配置兜底
  localG() {
    const r = this.data.region;
    return r && r.gravity ? r.gravity : this.data.config.gravity;
  },

  // ===================== 真值 =====================

  // 学生算出的摆长（m）
  nominalLengthM() {
    const l = parseFloat(this.data.lineLength);
    const d = parseFloat(this.data.ballDiameter);
    if (!isFinite(l) || !isFinite(d) || l <= 0 || d <= 0) return null;
    return (l + d / 2) / 100;
  },

  // 真实摆长（含悬点系统偏差 + 本轮重新夹持的随机偏差）——学生不可见。
  // 系统偏差 _lNoise 整套装置固定；_roundNoise 每次释放重抽，
  // 使各轮 g 有真实的离散度，从而「多测几轮取平均」确实能提高精度。
  trueLengthM() {
    const nominal = this.nominalLengthM();
    if (nominal === null) return this.data.config.length_m;
    if (this._lNoise === null) {
      this._lNoise = this.gaussNoise() * LENGTH_SIGMA;
    }
    return nominal + this._lNoise + this._roundNoise;
  },

  // 标准正态随机数（Box–Muller）
  gaussNoise() {
    const u1 = Math.random() || 1e-9;
    const u2 = Math.random();
    return Math.sqrt(-2 * Math.log(u1)) * Math.cos(2 * Math.PI * u2);
  },

  // 真实周期：T = 2π√(L/g)·[1 + (1/4)sin²(θ₀/2)]，g 用当地值
  truePeriod() {
    const L = this.trueLengthM();
    const g = this.localG();
    const th = (this.data.config.angle_deg * Math.PI) / 180;
    const corr = 1 + 0.25 * Math.pow(Math.sin(th / 2), 2);
    return 2 * Math.PI * Math.sqrt(L / g) * corr;
  },

  // ===================== 装置几何 =====================

  onLineInput(e) {
    this.setData({ lineLength: e.detail.value }, () => this.recalcL());
  },

  onBallInput(e) {
    this.setData({ ballDiameter: e.detail.value }, () => this.recalcL());
  },

  recalcL() {
    const l = parseFloat(this.data.lineLength);
    const d = parseFloat(this.data.ballDiameter);
    if (isFinite(l) && isFinite(d) && l > 0 && d > 0) {
      this.setData({ measuredL: (l + d / 2).toFixed(2) });
    } else {
      this.setData({ measuredL: null });
    }
    // 改几何 = 重装一套装置，悬点偏差重抽。
    // 表格数据不清：那是学生自己测得并记录的，换装置后他自己决定哪些还能用。
    this._lNoise = null;
    if (this.data.swinging) this.stopEverything();
    this.setData({ finalG: null, errorInfo: null });
  },

  onAngleChange(e) {
    // 摆角每轮都可以不同（学生要观察大摆角对周期的影响），
    // 因此改摆角只中止当前摆动，不动已记录的表格。
    this.setData({ 'config.angle_deg': Number(e.detail.value) });
    if (this.data.swinging) this.stopEverything();
  },

  onPeriodCountChange(e) {
    this.setData({ periodCount: Number(e.detail.value) });
  },

  // 学生自主设定计划轮数。只是个计划值：做够了可以直接停，
  // 想继续测也不受它限制，因此不做任何强制。
  onPlannedRoundsChange(e) {
    this.setData({ plannedRounds: Number(e.detail.value) });
  },

  onSpeedTap() {
    const next = (this.data.speedIndex + 1) % SPEEDS.length;
    this.setData({ speedIndex: next, speedText: `${SPEEDS[next]}×` });
  },

  // ===================== 起摆（同时自动起表） =====================

  // 拉动并释放摆球：起摆的同一瞬间自动开始计时，
  // 计满学生设置的周期数后装置与秒表一起自动停下，全程无需学生按表。
  releaseBall() {
    if (this.data.swinging) {
      // 摆动中再次点击 = 放弃本次，不记录数据
      this.stopEverything();
      return;
    }
    if (this.nominalLengthM() === null) {
      wx.showToast({ title: '请先设置悬线长和摆球直径', icon: 'none' });
      return;
    }
    if (!this.data.region) {
      wx.showToast({ title: '请先确认实验地区', icon: 'none' });
      return;
    }
    // 这一轮重新拉开摆球 -> 夹持位置微动，摆长真值重抽
    this._roundNoise = this.gaussNoise() * ROUND_LENGTH_SIGMA;
    // 自动触发的分辨误差，每次释放随机一次
    this._startJitter = this.gaussNoise() * TRIGGER_SIGMA;
    this._releaseSim = this._simTime;
    this._timeStartSim = this._simTime;
    this.setData({
      swinging: true,
      timing: true,
      periodNow: 0,
      elapsed: '0.00'
    });
  },

  stopEverything() {
    this.setData({ swinging: false, timing: false, periodNow: 0, elapsed: '0.00' });
  },

  // 计满目标周期数：装置停摆 + 秒表停下。
  // 数据不再由程序填进表里——只给出秒表读数，周期和 g 由学生自己算、自己填。
  finishTiming(elapsedSim) {
    const n = this.data.periodCount;
    const t = Math.max(0.1, elapsedSim);
    this.setData({
      swinging: false,
      timing: false,
      elapsed: t.toFixed(2),
      periodNow: n,
      roundDone: true,
      lastRun: { n, t: t.toFixed(2) }
    });
    wx.showToast({ title: `计满 ${n} 个周期，请记录数据`, icon: 'none' });
  },

  // 把这一轮加进表格：新增一行空白待填，学生自己填角度/周期/g
  addRound() {
    if (!this.data.roundDone) {
      wx.showToast({ title: '请先完成一轮摆动', icon: 'none' });
      return;
    }
    const { rounds, lastRun, config } = this.data;
    const rounds2 = rounds.concat([{
      round: rounds.length + 1,
      angle: String(config.angle_deg),  // 预填学生自己设的摆角，仍可改
      period: '',
      g: '',
      selected: true,                   // 默认选用，学生可取消
      tMeasured: lastRun ? lastRun.t : '',
      nMeasured: lastRun ? lastRun.n : this.data.periodCount
    }]);
    this.setData({ rounds: rounds2, roundDone: false, lastRun: null });
    this.refreshSelection(rounds2);
  },

  // 表格内输入：列 = angle | period | g
  onRoundInput(e) {
    const i = Number(e.currentTarget.dataset.index);
    const field = e.currentTarget.dataset.field;
    const rounds = this.data.rounds.slice();
    if (!rounds[i]) return;
    rounds[i] = Object.assign({}, rounds[i], { [field]: e.detail.value });
    // 改了数据就作废已确认的结果，避免表和结论不一致
    this.setData({ rounds, finalG: null, errorInfo: null });
  },

  // 勾选/取消某一轮参与求平均
  toggleRound(e) {
    const i = Number(e.currentTarget.dataset.index);
    const rounds = this.data.rounds.slice();
    if (!rounds[i]) return;
    rounds[i] = Object.assign({}, rounds[i], { selected: !rounds[i].selected });
    this.setData({ rounds, finalG: null, errorInfo: null });
    this.refreshSelection(rounds);
  },

  deleteRound(e) {
    const i = Number(e.currentTarget.dataset.index);
    // 删行后重编轮次号，保持 1..n 连续
    const rounds = this.data.rounds
      .filter((_, idx) => idx !== i)
      .map((r, idx) => Object.assign({}, r, { round: idx + 1 }));
    this.setData({ rounds, finalG: null, errorInfo: null });
    this.refreshSelection(rounds);
  },

  // 汇总选中的轮次，供「已选用第 X、Y 轮」显示
  refreshSelection(rounds) {
    const picked = rounds.filter((r) => r.selected);
    const nums = picked.map((r) => r.round);
    this.setData({
      selectedCount: picked.length,
      selectedLabel: nums.length ? `已选用第 ${nums.join('、')} 轮` : '尚未选择任何轮次'
    });
  },

  onAvgGInput(e) {
    this.setData({ avgGInput: e.detail.value, finalG: null, errorInfo: null });
  },

  // 确认最终结果：此时才揭示当地 g 与误差率
  confirmFinalG() {
    const { avgGInput, rounds, region } = this.data;
    const g = parseFloat(avgGInput);
    if (!isFinite(g) || g <= 0) {
      wx.showToast({ title: '请填写重力加速度平均值', icon: 'none' });
      return;
    }
    if (g < 5 || g > 15) {
      wx.showToast({ title: 'g 明显超出合理范围，请检查计算', icon: 'none' });
      return;
    }
    const picked = rounds.filter((r) => r.selected);
    if (picked.length === 0) {
      wx.showToast({ title: '请先勾选用于求平均的实验轮次', icon: 'none' });
      return;
    }
    if (!region) {
      wx.showToast({ title: '请先确认实验地区', icon: 'none' });
      return;
    }
    const gLocal = this.localG();
    const relErr = (Math.abs(g - gLocal) / gLocal) * 100;
    this.setData({
      finalG: g.toFixed(4),
      errorInfo: {
        gLocal: gLocal.toFixed(4),
        relErr: relErr.toFixed(2),
        // 自动计时无人工反应误差，规范操作误差应在 0.3% 以内。
        // 分级只是即时反馈，最终分数由后端同一条曲线算出。
        level: relErr <= 0.3 ? 'good' : (relErr <= 1 ? 'ok' : (relErr <= 3 ? 'warn' : 'bad')),
        comment:
          relErr <= 0.3 ? '很好，误差在 0.3% 以内，属于规范操作的水平' :
          relErr <= 1 ? '接近，可检查摆长是否算到球心（L = l + d/2）、摆角是否偏大' :
          relErr <= 3 ? '偏大，请检查 L 的单位（要用米）、T = t/n 是否除对、摆角是否在 5° 以内' :
          '误差过大，请检查 g = 4π²L/T² 的代入过程与各量单位'
      }
    });
  },

  // ===================== 3D 视角交互 =====================

  onCanvasTouchStart(e) {
    const t = e.touches && e.touches[0];
    if (!t) return;
    this._touchX = t.x;
    this._touchAz = this.data.azimuthDeg;
  },

  onCanvasTouchMove(e) {
    const t = e.touches && e.touches[0];
    if (!t) return;
    const dx = t.x - this._touchX;
    // 左右滑动绕竖直轴转，可看到装置的每一个角度
    let az = this._touchAz + (dx * SWIPE_SENSITIVITY * 180) / Math.PI;
    az = ((az % 360) + 360) % 360;
    this.setData({ azimuthDeg: Math.round(az) });
  },

  resetView() {
    this.setData({ azimuthDeg: 35 });
  },

  // ===================== 3D 投影与绘制 =====================

  // 世界坐标 -> 屏幕坐标。
  // 世界系：原点在悬点，x 沿摆动方向，y 竖直向下，z 垂直于摆动平面。
  // 先绕 y 轴转方位角（左右滑动），再绕 x 轴俯仰，最后透视投影。
  project(x, y, z, view) {
    const { sinAz, cosAz, sinEl, cosEl, cx, cy, scale } = view;
    // 绕竖直轴（y）旋转
    const x1 = x * cosAz + z * sinAz;
    const z1 = -x * sinAz + z * cosAz;
    // 绕 x 轴俯仰
    const y2 = y * cosEl - z1 * sinEl;
    const z2 = y * sinEl + z1 * cosEl;
    // 透视：z 越大（越远）越小
    const persp = FOCAL / (FOCAL + z2 * scale);
    return {
      sx: cx + x1 * scale * persp,
      sy: cy + y2 * scale * persp,
      depth: z2,
      persp
    };
  },

  startAnimation() {
    const canvas = this._canvas;
    if (!canvas) return;
    const loop = () => {
      this.tick();
      this.drawScene();
      this._rafId = canvas.requestAnimationFrame(loop);
    };
    loop();
  },

  // 推进仿真时钟、数周期、到点自动停
  tick() {
    const nowMs = Date.now();
    let dtMs = nowMs - this._lastFrameMs;
    this._lastFrameMs = nowMs;
    // 后台回来或掉帧时钳制，避免一帧跳过好几个周期
    if (dtMs < 0) dtMs = 0;
    if (dtMs > 100) dtMs = 100;

    const speed = SPEEDS[this.data.speedIndex] || 1;
    this._simTime += (dtMs / 1000) * speed;

    if (!this.data.swinging || !this.data.timing) return;

    const T = this.truePeriod();
    // 释放即起表，计数从 t0+jitter 起算，
    // 因此计满 n 个周期时秒表读数 = n·T + jitter。
    const elapsed = this._simTime - this._timeStartSim;
    const counted = Math.floor((elapsed - this._startJitter) / T);
    const shown = Math.max(0, Math.min(counted, this.data.periodCount));

    if (counted >= this.data.periodCount) {
      this.finishTiming(this.data.periodCount * T + this._startJitter);
      return;
    }
    // setData 节流到 ~10Hz，避免刷爆逻辑层
    if (nowMs - this._lastPushMs > 100) {
      this._lastPushMs = nowMs;
      this.setData({ elapsed: elapsed.toFixed(2), periodNow: shown });
    }
  },

  // 当前摆角（弧度）
  currentAngle() {
    const amp = (this.data.config.angle_deg * Math.PI) / 180;
    if (!this.data.swinging) return amp; // 静止时停在拉开的位置
    const T = this.truePeriod();
    const omega = (2 * Math.PI) / T;
    return amp * Math.cos(omega * (this._simTime - this._releaseSim));
  },

  drawScene() {
    const canvas = this._canvas;
    const ctx = this._ctx;
    if (!canvas || !ctx) return;

    const { canvasWidth: w, canvasHeight: h, azimuthDeg } = this.data;
    if (!w || !h) return;

    // 尺寸与 transform 已在 setupCanvas 里设好，这里只重绘内容
    // 背景
    const bg = ctx.createLinearGradient(0, 0, 0, h);
    bg.addColorStop(0, '#eef2f7');
    bg.addColorStop(1, '#dfe6ee');
    ctx.fillStyle = bg;
    ctx.fillRect(0, 0, w, h);

    // ---- 世界尺度：把摆长映射到画布 ----
    const nominal = this.nominalLengthM();
    const ropeM = nominal === null ? 1.0 : (parseFloat(this.data.lineLength) || 100) / 100;
    const ballRM = ((parseFloat(this.data.ballDiameter) || 2) / 100) / 2;
    // 绘制用半径：真实比例下摆球只有几个像素，看不出直径变化，故放大显示
    const ballRVis = ballRM * BALL_VISUAL_GAIN;
    // 摆长越长，整体缩小以保证装置完整可见（世界单位取米）
    const maxWorldH = Math.max(ropeM + ballRVis * 2 + 0.25, 0.6);
    const scale = (h * 0.78) / maxWorldH;

    const az = (azimuthDeg * Math.PI) / 180;
    const view = {
      sinAz: Math.sin(az), cosAz: Math.cos(az),
      sinEl: Math.sin(ELEVATION), cosEl: Math.cos(ELEVATION),
      cx: w / 2,
      cy: h * 0.13,   // 悬点位置
      scale
    };

    const P = (x, y, z) => this.project(x, y, z, view);

    // 支架尺寸（米）
    const standH = ropeM + ballRVis * 2 + 0.18; // 立柱高：比摆到底还留一点
    const baseHalf = Math.max(0.16, ropeM * 0.22);
    const beamHalfZ = 0.11;

    const theta = this.currentAngle();
    // 摆球中心：绕悬点在 x-y 平面内摆动。球心到悬点 = 摆线长 + 球半径，
    // 与学生用的 L = l + d/2 一致（此处半径用放大后的值以保持画面自洽）
    const centerM = ropeM + ballRVis;
    const bx = Math.sin(theta) * centerM;
    const by = Math.cos(theta) * centerM;
    const ball = P(bx, by, 0);

    // ---- 底座（x-z 平面矩形，位于立柱底端）----
    const yBase = standH;
    const c1 = P(-baseHalf, yBase, -baseHalf * 0.8);
    const c2 = P(baseHalf, yBase, -baseHalf * 0.8);
    const c3 = P(baseHalf, yBase, baseHalf * 0.8);
    const c4 = P(-baseHalf, yBase, baseHalf * 0.8);
    ctx.beginPath();
    ctx.moveTo(c1.sx, c1.sy);
    ctx.lineTo(c2.sx, c2.sy);
    ctx.lineTo(c3.sx, c3.sy);
    ctx.lineTo(c4.sx, c4.sy);
    ctx.closePath();
    ctx.fillStyle = 'rgba(120,132,145,0.55)';
    ctx.fill();
    ctx.strokeStyle = '#6b7683';
    ctx.lineWidth = 1.5;
    ctx.stroke();

    // ---- 摆球在底座上的投影（增强立体感）----
    const shadow = P(bx, yBase, 0);
    ctx.beginPath();
    ctx.ellipse(
      shadow.sx, shadow.sy,
      Math.max(3, ballRVis * scale * shadow.persp * 1.1),
      Math.max(1.5, ballRVis * scale * shadow.persp * 0.4),
      0, 0, Math.PI * 2
    );
    ctx.fillStyle = 'rgba(0,0,0,0.16)';
    ctx.fill();

    // ---- 立柱 + 横梁：按深度排序，远的先画，近的后画盖住 ----
    const posts = [-beamHalfZ, beamHalfZ].map((pz) => {
      const top = P(-baseHalf * 0.55, 0, pz);
      const bot = P(-baseHalf * 0.55, standH, pz);
      return { top, bot, depth: top.depth };
    });
    posts.sort((a, b) => b.depth - a.depth);

    const beamA = P(-baseHalf * 0.55, 0, -beamHalfZ);
    const beamB = P(-baseHalf * 0.55, 0, beamHalfZ);
    const pivot = P(0, 0, 0);

    const drawPost = (p) => {
      ctx.strokeStyle = '#7f8c8d';
      ctx.lineWidth = Math.max(3, 5 * p.top.persp);
      ctx.beginPath();
      ctx.moveTo(p.top.sx, p.top.sy);
      ctx.lineTo(p.bot.sx, p.bot.sy);
      ctx.stroke();
    };

    // 远侧立柱
    drawPost(posts[0]);

    // 横梁（连接两立柱顶端）+ 悬臂伸到悬点
    ctx.strokeStyle = '#95a5a6';
    ctx.lineWidth = 4;
    ctx.beginPath();
    ctx.moveTo(beamA.sx, beamA.sy);
    ctx.lineTo(beamB.sx, beamB.sy);
    ctx.stroke();

    ctx.strokeStyle = '#7f8c8d';
    ctx.lineWidth = 4;
    ctx.beginPath();
    ctx.moveTo((beamA.sx + beamB.sx) / 2, (beamA.sy + beamB.sy) / 2);
    ctx.lineTo(pivot.sx, pivot.sy);
    ctx.stroke();

    // ---- 平衡位置铅垂虚线 ----
    const plumb = P(0, ropeM + 0.06, 0);
    ctx.setLineDash([5, 6]);
    ctx.strokeStyle = 'rgba(44,62,80,0.28)';
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(pivot.sx, pivot.sy);
    ctx.lineTo(plumb.sx, plumb.sy);
    ctx.stroke();
    ctx.setLineDash([]);

    // ---- 摆动轨迹弧（在摆动平面内，按 3D 投影逐点连线）----
    const amp = (this.data.config.angle_deg * Math.PI) / 180;
    ctx.setLineDash([4, 6]);
    ctx.strokeStyle = 'rgba(52,152,219,0.45)';
    ctx.lineWidth = 1.5;
    ctx.beginPath();
    const STEPS = 28;
    for (let i = 0; i <= STEPS; i++) {
      const a = -amp + (2 * amp * i) / STEPS;
      const p = P(Math.sin(a) * ropeM, Math.cos(a) * ropeM, 0);
      if (i === 0) ctx.moveTo(p.sx, p.sy);
      else ctx.lineTo(p.sx, p.sy);
    }
    ctx.stroke();
    ctx.setLineDash([]);

    // ---- 摆角标注弧 ----
    ctx.strokeStyle = 'rgba(230,126,34,0.75)';
    ctx.lineWidth = 2;
    ctx.beginPath();
    const rArc = ropeM * 0.28;
    for (let i = 0; i <= 16; i++) {
      const a = (theta * i) / 16;
      const p = P(Math.sin(a) * rArc, Math.cos(a) * rArc, 0);
      if (i === 0) ctx.moveTo(p.sx, p.sy);
      else ctx.lineTo(p.sx, p.sy);
    }
    ctx.stroke();

    // ---- 悬点夹头 ----
    ctx.fillStyle = '#2c3e50';
    ctx.beginPath();
    ctx.arc(pivot.sx, pivot.sy, Math.max(3, 5 * pivot.persp), 0, Math.PI * 2);
    ctx.fill();

    // ---- 摆线 ----
    ctx.strokeStyle = '#5d6d7e';
    ctx.lineWidth = Math.max(1, 1.8 * ball.persp);
    ctx.beginPath();
    ctx.moveTo(pivot.sx, pivot.sy);
    ctx.lineTo(ball.sx, ball.sy);
    ctx.stroke();

    // ---- 摆球（半径随透视缩放，直径由学生输入驱动，已放大便于观察）----
    const rPx = Math.max(4, ballRVis * scale * ball.persp);
    const grad = ctx.createRadialGradient(
      ball.sx - rPx * 0.35, ball.sy - rPx * 0.4, rPx * 0.1,
      ball.sx, ball.sy, rPx
    );
    grad.addColorStop(0, '#f5b7b1');
    grad.addColorStop(0.45, '#e74c3c');
    grad.addColorStop(1, '#922b21');
    ctx.fillStyle = grad;
    ctx.beginPath();
    ctx.arc(ball.sx, ball.sy, rPx, 0, Math.PI * 2);
    ctx.fill();
    ctx.strokeStyle = 'rgba(0,0,0,0.25)';
    ctx.lineWidth = 1;
    ctx.stroke();

    // 经过平衡位置时高亮（计时参考点）
    if (this.data.swinging && Math.abs(theta) < 0.025) {
      ctx.strokeStyle = 'rgba(39,174,96,0.9)';
      ctx.lineWidth = 3;
      ctx.beginPath();
      ctx.arc(ball.sx, ball.sy, rPx + 6, 0, Math.PI * 2);
      ctx.stroke();
    }

    // 近侧立柱最后画，形成前后遮挡
    drawPost(posts[1]);

    // ---- 左下角标注 ----
    ctx.fillStyle = 'rgba(44,62,80,0.75)';
    ctx.font = '12px sans-serif';
    ctx.textAlign = 'left';
    ctx.fillText(`θ₀ = ${this.data.config.angle_deg}°`, 12, h - 30);
    ctx.fillText(
      `L = ${this.data.measuredL ? this.data.measuredL + ' cm' : '--'}`,
      12, h - 14
    );
  },

  // ===================== 提交测量数据 =====================

  async submitReadings() {
    const { levelId, assignmentId, region, finalG, rounds, selectedCount } = this.data;
    // 必须先确认最终结果：提交的就是学生自己算出的那个 g
    if (!finalG) {
      wx.showToast({ title: '请先填写并确认 g 的平均值', icon: 'none' });
      return;
    }
    const picked = rounds.filter((r) => r.selected);
    // 选中轮的周期平均值，只作记录（后端评分只看 calc_g）
    const periods = picked
      .map((r) => parseFloat(r.period))
      .filter((v) => isFinite(v) && v > 0);
    const avgT = periods.length
      ? periods.reduce((s, v) => s + v, 0) / periods.length
      : 0;

    const reading = {
      period: Number(avgT.toFixed(4)),
      calc_g: Number(parseFloat(finalG).toFixed(4)),
      rounds: rounds.length,
      selected_count: selectedCount
    };
    // 带上地区，后端据此用当地 g 作评分基准
    if (region) {
      reading.latitude = region.latitude;
      reading.altitude = region.altitude;
      reading.has_region = true;
    }

    this.setData({ submitting: true });
    try {
      const url = assignmentId ? `/assignments/${assignmentId}/submit` : '/progress/submit';
      const payload = {
        level_id: parseInt(levelId, 10) || 3,
        experiment: 'pendulum',
        readings: [reading]
      };
      if (assignmentId && region) {
        payload.region_label = region.label;
      }
      const data = await request({ url, method: 'POST', data: payload });

      // 作业接口返回综合分，练习接口只返回数据分
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
          if (res.confirm) this.setData({ tab: 'quiz' });
        }
      });
    } catch (err) {
      console.error('[pendulum] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally {
      this.setData({ submitting: false });
    }
  },

  // ===================== 自测 =====================

  onQuizChoice(e) {
    this.setData({ [`quizAnswers.${e.currentTarget.dataset.idx}`]: Number(e.detail.value) });
  },

  onQuizJudge(e) {
    const { idx, val } = e.currentTarget.dataset;
    this.setData({ [`quizAnswers.${idx}`]: val === 'true' });
  },

  onQuizInput(e) {
    this.setData({ [`quizAnswers.${e.currentTarget.dataset.idx}`]: e.detail.value });
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
    this.setData({
      [`quizState.${idx}`]: ok ? 'right' : (prev === 'hint' || prev === 'wrong' ? 'wrong' : 'hint')
    });
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
        data: { experiment: 'pendulum', quiz_score: score }
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
      console.error('[pendulum] 自测提交失败:', err);
      wx.showToast({ title: err.message || '自测提交失败', icon: 'none' });
    } finally {
      this.setData({ quizSubmitting: false });
    }
  },

  resetQuiz() {
    this.setData({
      quiz: pickQuiz('pendulum'),
      quizAnswers: {},
      quizState: {},
      quizScore: null,
      comboScore: null
    });
  }
});
