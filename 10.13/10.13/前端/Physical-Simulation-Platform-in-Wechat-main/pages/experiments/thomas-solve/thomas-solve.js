// pages/experiments/thomas-solve/thomas-solve.js - 追赶法解三对角方程组（数值分析 L28）
// 契约（docs/api-contract.md 附·数值分析批次 L18，v0.23）：
//   config: { system_key, system_desc, method_hint }
//   readings: [{ solution: [x1, ..., xn], m? 可选审计, beta? 可选审计 }]（solution 长度须=未知数个数；
//             m=乘子 m2..mn 长度 n-1、beta=新主对角 beta1..betan 长度 n，各全或无，仅记审计不打分）
//   user_config: { system_key }（换题时后端按新三对角重做追赶重算真值）
// 教学主线（与 L27 LU 分解同族）：三对角 LU 特例——追（Crout 消元）逐行算乘数 m_i、
//   新主对角 beta_i 并更新右端 d_i，赶（回代）自最后一个未知量倒推解；
//   仅存三条对角线 O(n) 计算量（满阵 LU 为 O(n^3)）——热传导差分/样条三弯矩的标准解法。
// 三对角题库与后端 scoring.go 的 triSystems 同步（utils/numlab.js）。
import { request } from '../../../utils/request';
import { triSystems, fmtNum } from '../../../utils/numlab';
import { createStepPlayer } from '../../../utils/anim';

// 矩阵单元展示：消去浮点噪声，6 位有效数字；带外显示 '·'
function fmtCell(v) {
  if (v === null || v === undefined || isNaN(v) || !isFinite(v)) return '-';
  if (Math.abs(v) < 1e-10) return '0';
  return String(Number(v.toPrecision(6)));
}

// 下标数字（x₁ ~ x₈ / m₂ / β₁ 等）
function sub(d) {
  return '₀₁₂₃₄₅₆₇₈₉'.charAt(d);
}

const DEFAULT_CONFIG = {
  system_key: 'tri1_basic',
  system_desc: '追（Crout 消元）逐行算乘数 m 与新主对角 beta 并更新右端，赶（回代）倒推解。默认题解恰 (1,2,3,4)，m=(0.5,0.8,0.3125)、beta=(2,2.5,3.2,4.6875) 全手算',
  a: [1, 2, 1],
  b: [2, 3, 4, 5],
  c: [1, 1, 1],
  d: [4, 10, 20, 23]
};

Page({
  data: {
    levelId: null,
    experimentId: null,
    isPreview: false,
    submitting: false,
    // 自动分拍演示进行中（按钮在「自动完成 / 停止演示」间切换）
    autoPlaying: false,
    // 出题参数（后端 GET /experiments/:levelId 下发，缺省走库内默认）
    config: { ...DEFAULT_CONFIG },
    systemKeys: [],
    systemLabels: [],
    systemIndex: 0,
    systemDesc: '',
    // 展示用（WXML 不能调函数，均在 JS 侧预格式化）
    n: 4,
    equations: [],
    matrixRows: [],       // 三对角满矩阵展示（带外 '·'）
    worksheetRows: [],    // 追赶工作表：序号 | m | beta | d' | x 逐行演化
    // 阶段状态机：chase（逐行追）→ sweep（倒序赶）→ done
    stage: 'chase',
    chaseLines: [],
    sweepLines: [],
    solutionInputs: [],
    mInputs: [],          // 乘子审计输入（选填，m2..mn）
    betaInputs: [],       // 新主对角审计输入（选填，beta1..betan）
    solutionText: '',
    finished: false,
    // 解向量条形图数据（components/sol-bars）：赶的每步长出一根
    chartValues: []
  },

  // 工作数组（不进 data）：乘子列、新主对角、更新右端、解向量
  _m: null,
  _beta: null,
  _dd: null,
  _x: null,

  onLoad(options) {
    const levelId = options.levelId;
    // 教师/管理员预览：可自由操作实验，隐藏提交（/progress/submit 仅学生可用）
    const app = getApp();
    const role = app.globalData.userInfo ? app.globalData.userInfo.role : null;
    const keys = Object.keys(triSystems);
    this.setData({
      levelId,
      isPreview: role === 'teacher' || role === 'admin',
      systemKeys: keys,
      systemLabels: keys.map(k => triSystems[k].label)
    });
    if (levelId) {
      this.loadExperimentConfig(levelId);
    } else {
      this.restart();
    }
  },

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
      if (data.config && data.config.system_key) {
        const key = data.config.system_key || 'tri1_basic';
        // a/b/c/d 一律以库内定义为准（与后端评分真值同源）
        const def = triSystems[key];
        if (def) {
          this.setData({
            experimentId: data.id,
            config: {
              system_key: key,
              system_desc: data.config.system_desc || def.desc,
              a: def.a,
              b: def.b,
              c: def.c,
              d: def.d
            },
            systemIndex: this.data.systemKeys.indexOf(key)
          });
        }
      }
    } catch (err) {
      console.warn('[thomas-solve] 后端不可用，使用默认配置');
    }
    this.restart();
  },

  // 换题（picker）：随 user_config 提交 system_key，后端按新三对角重做追赶重算真值
  onSystemChange(e) {
    const idx = Number(e.detail.value);
    const key = this.data.systemKeys[idx];
    if (!key || key === this.data.config.system_key) return;
    const def = triSystems[key];
    this.setData({
      config: {
        system_key: key,
        system_desc: def.desc,
        a: def.a,
        b: def.b,
        c: def.c,
        d: def.d
      },
      systemIndex: idx
    });
    this.restart();
  },

  // 重开：beta/d' 复位为 b/d（第 1 行即初值），m 置空；题目展示冻结在初始 a/b/c/d 上
  restart() {
    this._cfgGen = (this._cfgGen || 0) + 1; // 学生主动重算：使在途配置请求作废
    this.stopAutoRun();
    const { config } = this.data;
    const n = config.b.length;
    this._m = new Array(n - 1);       // 乘子（追的过程逐个填）
    this._beta = config.b.map(Number); // 新主对角（第 1 行=b1，其余追出后覆盖）
    this._dd = config.d.map(Number);   // 更新后的右端
    this._x = null;
    // 三对角满矩阵（仅展示带内元素）
    const full = Array.from({ length: n }, (_, i) =>
      Array.from({ length: n }, (_, j) => {
        if (i === j) return config.b[i];
        if (i === j + 1) return config.a[j];
        if (i === j - 1) return config.c[i];
        return null; // 带外
      }));
    this.setData({
      n,
      equations: full.map((row, i) => this.formatEquation(row, config.d[i])),
      matrixRows: full.map((row, ri) => ({
        k: 'r' + ri,
        cells: row.map((v, ci) => ({
          k: 'c' + ci,
          t: v === null ? '·' : fmtCell(v),
          cls: v === null ? 'cell-zero' : ''
        }))
      })),
      stage: 'chase',
      chaseLines: [],
      sweepLines: [],
      solutionInputs: [],
      mInputs: [],
      betaInputs: [],
      solutionText: '',
      finished: false,
      chartValues: new Array(n).fill(null)
    });
    this._wsPrev = null; // 重开：首帧不弹入
    this.refreshWorksheet(-1);
  },

  // 追赶工作表刷新：m/beta/d' 按已追行数逐格长出，x 按已赶行数长出；activeIdx 高亮当前行
  refreshWorksheet(activeIdx) {
    const n = this.data.n;
    const prev = this._wsPrev; // 上一帧各格文本：'·'→数值的格子加弹入动效
    const snap = [];
    const rows = [];
    const fresh = (i, key, txt) => (prev && prev[i][key] === '·' && txt !== '·') ? 'cell-new' : '';
    for (let i = 0; i < n; i++) {
      const chased = i === 0 || this._m[i - 1] !== undefined;
      const r = {
        m: i === 0 ? '—' : (chased ? fmtCell(this._m[i - 1]) : '·'),
        beta: chased ? fmtCell(this._beta[i]) : '·',
        dd: chased ? fmtCell(this._dd[i]) : '·',
        x: this._x && this._x[i] !== undefined ? fmtCell(this._x[i]) : '·'
      };
      snap.push(r);
      rows.push({
        k: 'w' + i,
        idx: i + 1,
        m: r.m, beta: r.beta, dd: r.dd, x: r.x,
        mCls: fresh(i, 'm', r.m),
        betaCls: fresh(i, 'beta', r.beta),
        ddCls: fresh(i, 'dd', r.dd),
        xCls: fresh(i, 'x', r.x),
        cls: i === activeIdx ? 'ws-active' : ''
      });
    }
    this._wsPrev = snap;
    this.setData({ worksheetRows: rows });
  },

  // 当前工作行（高亮用）：追阶段=下一待追行，赶阶段=当前回代行，否则 -1
  activeRow() {
    const { stage, n, chaseLines, sweepLines } = this.data;
    if (stage === 'chase') return chaseLines.length + 1;
    if (stage === 'sweep') return n - 1 - sweepLines.length;
    return -1;
  },

  // 原方程组的可读形式：2x₁ + x₂ = 4
  formatEquation(aRow, bVal) {
    const n = aRow.length;
    const terms = [];
    for (let j = 0; j < n; j++) {
      const co = aRow[j];
      if (co === null || Math.abs(co) < 1e-12) continue;
      const abs = Math.abs(co);
      const coef = Math.abs(abs - 1) < 1e-12 ? '' : fmtCell(abs);
      const sign = co < 0 ? '−' : (terms.length ? '+' : '');
      terms.push(`${sign}${coef}x${sub(j + 1)}`);
    }
    const lhs = terms.length ? terms.join(' ') : '0';
    return `${lhs} = ${fmtCell(bVal)}`;
  },

  // 追下一步（第 i 行，0 基 1..n-1）：m_i = a_{i-1}/beta_{i-1}，
  // beta_i = b_i − m_i·c_{i-1}，d_i <- d_i − m_i·d_{i-1}（不选主元）。
  // 返回 true 推进一拍，false 被拦截（供自动播放器停机）；播放中手动点步不响应。
  onChaseStep() {
    if (this._player && !this._autoStepping) return false;
    if (this.data.stage !== 'chase') return false;
    const n = this.data.n;
    const i = this.data.chaseLines.length + 1;
    if (i >= n) return false;
    const { a, b, c } = this.data.config;
    const prevBeta = this._beta[i - 1];
    const mm = a[i - 1] / prevBeta;
    if (!isFinite(mm)) {
      wx.showToast({ title: '数值溢出，追赶中断', icon: 'none' });
      return false;
    }
    // 先算后验再落工作数组：β≈0 中断时不污染 m/β/d'，单步重试状态一致
    const newBeta = b[i] - mm * c[i - 1];
    const ddOld = this._dd[i];
    const newDd = ddOld - mm * this._dd[i - 1];
    if (!isFinite(newBeta) || !isFinite(newDd) || Math.abs(newBeta) < 1e-12) {
      wx.showToast({ title: `第 ${i + 1} 行 β≈0，追赶法中断`, icon: 'none' });
      return false;
    }
    this._m[i - 1] = mm;
    this._beta[i] = newBeta;
    this._dd[i] = newDd;
    const line = `m${sub(i + 1)} = a${sub(i)}/β${sub(i)} = ${fmtCell(a[i - 1])}/${fmtCell(prevBeta)} = ${fmtCell(mm)}；β${sub(i + 1)} = ${fmtCell(b[i])} − ${fmtCell(mm)}×${fmtCell(c[i - 1])} = ${fmtCell(newBeta)}；d${sub(i + 1)} = ${fmtCell(ddOld)} − ${fmtCell(mm)}×${fmtCell(this._dd[i - 1])} = ${fmtCell(newDd)}`;
    const chaseLines = this.data.chaseLines.concat([{ k: 'c' + i, text: line }]);
    this.setData({ chaseLines });
    if (chaseLines.length === n - 1) {
      this.setData({ stage: 'sweep' });
    }
    this.refreshWorksheet(this.activeRow());
    return true;
  },

  // 赶下一步（第 i 行，0 基 n-1..0）：末行 x = d'/β；其余 x = (d' − c·x_next)/β。
  // 返回 true 推进一拍，false 被拦截（供自动播放器停机）；播放中手动点步不响应。
  onSweepStep() {
    if (this._player && !this._autoStepping) return false;
    if (this.data.stage !== 'sweep') return false;
    const n = this.data.n;
    const i = n - 1 - this.data.sweepLines.length;
    if (i < 0) return false;
    const { c } = this.data.config;
    if (!this._x) this._x = new Array(n);
    let line;
    if (i === n - 1) {
      this._x[i] = this._dd[i] / this._beta[i];
      line = `x${sub(i + 1)} = d'${sub(i + 1)}/β${sub(i + 1)} = ${fmtCell(this._dd[i])}/${fmtCell(this._beta[i])} = ${fmtCell(this._x[i])}`;
    } else {
      this._x[i] = (this._dd[i] - c[i] * this._x[i + 1]) / this._beta[i];
      line = `x${sub(i + 1)} = (d'${sub(i + 1)} − c${sub(i + 1)}·x${sub(i + 2)})/β${sub(i + 1)} = (${fmtCell(this._dd[i])} − ${fmtCell(c[i])}×${fmtCell(this._x[i + 1])})/${fmtCell(this._beta[i])} = ${fmtCell(this._x[i])}`;
    }
    const sweepLines = this.data.sweepLines.concat([{ k: 's' + i, text: line }]);
    if (sweepLines.length === n) {
      // 全部求出 → 进入结果阶段，给出解向量输入与 m/beta 审计输入
      const inputs = [];
      for (let d = 0; d < n; d++) {
        inputs.push({ label: `x${sub(d + 1)}`, value: '' });
      }
      const ms = [];
      for (let d = 1; d < n; d++) {
        ms.push({ label: `m${sub(d + 1)}`, value: '' });
      }
      const betas = [];
      for (let d = 0; d < n; d++) {
        betas.push({ label: `β${sub(d + 1)}`, value: '' });
      }
      this.setData({
        sweepLines,
        stage: 'done',
        finished: true,
        solutionInputs: inputs,
        mInputs: ms,
        betaInputs: betas,
        solutionText: `x = (${this._x.map(v => fmtCell(v)).join(', ')})`
      });
      this.refreshWorksheet(-1);
      this.setData({ chartValues: this.snapshotChartValues() });
    } else {
      this.setData({ sweepLines });
      this.refreshWorksheet(this.activeRow());
      this.setData({ chartValues: this.snapshotChartValues() });
    }
    return true;
  },

  // 自动演示 / 停止演示：追行 → 赶行，一拍拍填满工作表与条形图
  onAutoRun() {
    if (this._player) {
      this.stopAutoRun();
      return;
    }
    if (this.data.stage === 'done') return;
    this.setData({ autoPlaying: true });
    this._player = createStepPlayer({
      interval: 480,
      isDone: () => this.data.stage === 'done',
      step: () => {
        this._autoStepping = true;
        let r;
        if (this.data.stage === 'chase') r = this.onChaseStep();
        else if (this.data.stage === 'sweep') r = this.onSweepStep();
        else r = false;
        this._autoStepping = false;
        return r;
      },
      onEnd: () => {
        this._player = null;
        if (this.data.autoPlaying) this.setData({ autoPlaying: false });
      }
    });
  },

  // 停止分拍演示（重开/换题/卸载时也会调用）
  stopAutoRun() {
    const p = this._player;
    this._player = null;
    if (p) p.cancel();
    if (this.data.autoPlaying) this.setData({ autoPlaying: false });
  },

  onUnload() {
    this._unloaded = true;
    this.stopAutoRun();
  },

  // 赶的进度快照：已求出的分量给数值，其余 null（条形图逐根长出）
  snapshotChartValues() {
    const n = this.data.n;
    const x = this._x || [];
    const arr = new Array(n);
    for (let j = 0; j < n; j++) {
      arr[j] = isFinite(x[j]) ? x[j] : null;
    }
    return arr;
  },

  // 学生填写解向量 / 乘子 / 新主对角审计
  onSolutionInput(e) {
    const idx = Number(e.currentTarget.dataset.idx);
    this.setData({ [`solutionInputs[${idx}].value`]: e.detail.value });
  },

  onMInput(e) {
    const idx = Number(e.currentTarget.dataset.idx);
    this.setData({ [`mInputs[${idx}].value`]: e.detail.value });
  },

  onBetaInput(e) {
    const idx = Number(e.currentTarget.dataset.idx);
    this.setData({ [`betaInputs[${idx}].value`]: e.detail.value });
  },

  async submitReadings() {
    if (this.data.submitting) return;
    const { levelId, finished, solutionInputs, mInputs, betaInputs, config } = this.data;
    if (!finished || !solutionInputs.length) {
      wx.showToast({ title: '请先完成追与赶', icon: 'none' });
      return;
    }
    if (solutionInputs.some(it => it.value === '' || !isFinite(Number(it.value)))) {
      wx.showToast({ title: '请完整填写全部解分量（数值）', icon: 'none' });
      return;
    }
    const solution = solutionInputs.map(it => Number(it.value));
    const reading = { solution };
    // 乘子 / 新主对角审计选填：各自全填或全空，填了才上报（仅审计不打分）
    if (mInputs.some(it => String(it.value).trim() !== '')) {
      if (mInputs.some(it => String(it.value).trim() === '' || !isFinite(Number(it.value)))) {
        wx.showToast({ title: '乘子 m 要么全填要么全空', icon: 'none' });
        return;
      }
      reading.m = mInputs.map(it => Number(it.value));
    }
    if (betaInputs.some(it => String(it.value).trim() !== '')) {
      if (betaInputs.some(it => String(it.value).trim() === '' || !isFinite(Number(it.value)))) {
        wx.showToast({ title: '新主对角 β 要么全填要么全空', icon: 'none' });
        return;
      }
      reading.beta = betaInputs.map(it => Number(it.value));
    }
    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit',
        method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 28,
          experiment: 'thomas_solve',
          readings: [reading],
          // 携带实际出题参数：换题时后端按此三对角重做追赶重算真值（契约 §12）
          user_config: {
            system_key: config.system_key
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
      console.error('[thomas-solve] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally {
      this.setData({ submitting: false });
    }
  }
});
