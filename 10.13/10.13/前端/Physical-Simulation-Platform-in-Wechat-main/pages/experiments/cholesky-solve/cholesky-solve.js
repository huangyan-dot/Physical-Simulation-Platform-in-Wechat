// pages/experiments/cholesky-solve/cholesky-solve.js - 平方根法解对称正定方程组（数值分析 L29）
// 契约（docs/api-contract.md 附·数值分析批次 L19，v0.24）：
//   config: { system_key, system_desc, method_hint }
//   readings: [{ solution: [x1, ..., xn], L? 可选审计, y? 可选审计 }]（solution 长度须=未知数个数；
//             L=n 阶方阵（对照取下三角含主对角 n(n+1)/2 个元）、y=前代中间向量长 n，
//             各全或无，仅记审计不打分）
//   user_config: { system_key }（换题时后端按新阵重做 LLᵀ 分解重算真值）
// 教学主线（与 L27 LU 分解/L28 追赶法同族，直接法收官）：对称正定时 A = L·Lᵀ，
//   按行 Cholesky-Crout 分解（主对角 l_ii=√(...) 恒正、永不选主元），
//   前代 Ly=b、回代 Lᵀx=y——计算量约为 LU 的一半，协方差阵/有限元刚度阵的标准解法。
// 对称正定题库与后端 scoring.go 的 spdSystems 同步（utils/numlab.js）。
import { request } from '../../../utils/request';
import { spdSystems, fmtNum } from '../../../utils/numlab';
import { createStepPlayer } from '../../../utils/anim';

// 矩阵单元展示：消去浮点噪声，6 位有效数字；零/未长出显示 '·'
function fmtCell(v) {
  if (v === null || v === undefined || isNaN(v) || !isFinite(v)) return '-';
  if (Math.abs(v) < 1e-10) return '0';
  return String(Number(v.toPrecision(6)));
}

// 下标数字（x₁ ~ x₄ / l₂₁ / y₃ 等）
function sub(d) {
  return '₀₁₂₃₄₅₆₇₈₉'.charAt(d);
}

const DEFAULT_CONFIG = {
  system_key: 'spd2_min3',
  system_desc: '对称正定方程组的专用解法（LU 分解的特例）：A = L·Lᵀ，按行依次算出下三角因子 L（主对角全正），再前代 Ly=b、回代 Lᵀx=y。默认题 L 恰为下三角全 1，y=(6,5,3)，解恰 (1,2,3) 全手算',
  A: [[1, 1, 1], [1, 2, 2], [1, 2, 3]],
  b: [6, 11, 14]
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
    n: 3,
    equations: [],
    aRows: [],            // 对称正定阵 A 展示（零元 '·' 置灰）
    lRows: [],            // 下三角因子 L 网格：分解逐行长出（上三角恒 '·'）
    // 阶段状态机：factor（逐行分解）→ forward（前代 Ly=b）→ back（回代 Lᵀx=y）→ done
    stage: 'factor',
    factorLines: [],
    forwardLines: [],
    backLines: [],
    solutionInputs: [],
    lInputs: [],          // L 审计输入（选填，下三角含主对角按行序）
    yInputs: [],          // 前代中间向量审计输入（选填，y1..yn）
    solutionText: '',
    finished: false,
    // 解向量条形图数据（components/sol-bars）：回代每步长出一根
    chartValues: []
  },

  // 工作数组（不进 data）：下三角因子、前代向量、解向量
  _L: null,
  _y: null,
  _x: null,

  onLoad(options) {
    const levelId = options.levelId;
    // 教师/管理员预览：可自由操作实验，隐藏提交（/progress/submit 仅学生可用）
    const app = getApp();
    const role = app.globalData.userInfo ? app.globalData.userInfo.role : null;
    const keys = Object.keys(spdSystems);
    this.setData({
      levelId,
      isPreview: role === 'teacher' || role === 'admin',
      systemKeys: keys,
      systemLabels: keys.map(k => spdSystems[k].label)
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
        const key = data.config.system_key || 'spd2_min3';
        // A/b 一律以库内定义为准（与后端评分真值同源）
        const def = spdSystems[key];
        if (def) {
          this.setData({
            experimentId: data.id,
            config: {
              system_key: key,
              system_desc: data.config.system_desc || def.desc,
              A: def.A,
              b: def.b
            },
            systemIndex: this.data.systemKeys.indexOf(key)
          });
        }
      }
    } catch (err) {
      console.warn('[cholesky-solve] 后端不可用，使用默认配置');
    }
    this.restart();
  },

  // 换题（picker）：随 user_config 提交 system_key，后端按新阵重做分解重算真值
  onSystemChange(e) {
    const idx = Number(e.detail.value);
    const key = this.data.systemKeys[idx];
    if (!key || key === this.data.config.system_key) return;
    const def = spdSystems[key];
    this.setData({
      config: {
        system_key: key,
        system_desc: def.desc,
        A: def.A,
        b: def.b
      },
      systemIndex: idx
    });
    this.restart();
  },

  // 重开：L 全空、y/x 置空；题目展示冻结在初始 A/b 上
  restart() {
    this._cfgGen = (this._cfgGen || 0) + 1; // 学生主动重算：使在途配置请求作废
    this.stopAutoRun();
    const { config } = this.data;
    const A = config.A;
    const n = A.length;
    this._L = Array.from({ length: n }, () => new Array(n));
    this._y = null;
    this._x = null;
    this.setData({
      n,
      equations: A.map((row, i) => this.formatEquation(row, config.b[i])),
      aRows: A.map((row, ri) => ({
        k: 'r' + ri,
        cells: row.map((v, ci) => ({
          k: 'c' + ci,
          t: Math.abs(v) < 1e-12 ? '·' : fmtCell(v),
          cls: Math.abs(v) < 1e-12 ? 'cell-zero' : ''
        }))
      })),
      stage: 'factor',
      factorLines: [],
      forwardLines: [],
      backLines: [],
      solutionInputs: [],
      lInputs: [],
      yInputs: [],
      solutionText: '',
      finished: false,
      chartValues: new Array(n).fill(null)
    });
    this._lPrev = null; // 重开：首帧不弹入
    this.refreshLGrid(0);
  },

  // L 因子网格刷新：已分解行给数值（含主对角），未分解与上三角显示 '·'；activeIdx 高亮当前行。
  // 上一帧是 '·'、本帧出值的格子加 cell-new 弹入（含主对角开根号落定的一拍）
  refreshLGrid(activeIdx) {
    const n = this.data.n;
    const done = this.data.factorLines.length;
    const prev = this._lPrev;
    const snap = [];
    const rows = [];
    for (let i = 0; i < n; i++) {
      const factored = i < done;
      const snapRow = [];
      rows.push({
        k: 'l' + i,
        cells: Array.from({ length: n }, (_, j) => {
          let t, cls;
          if (j > i) { t = '·'; cls = 'cell-zero'; }
          else if (factored && this._L[i][j] !== undefined) {
            t = fmtCell(this._L[i][j]);
            cls = j === i ? 'cell-diag' : '';
          } else { t = '·'; cls = 'cell-zero'; }
          if (prev && prev[i][j] === '·' && t !== '·') cls += ' cell-new';
          snapRow[j] = t;
          return { k: 'c' + j, t, cls };
        }),
        cls: i === activeIdx ? 'matrix-row row-active' : 'matrix-row'
      });
      snap.push(snapRow);
    }
    this._lPrev = snap;
    this.setData({ lRows: rows });
  },

  // 原方程组的可读形式：x₁ + x₂ + x₃ = 6
  formatEquation(aRow, bVal) {
    const n = aRow.length;
    const terms = [];
    for (let j = 0; j < n; j++) {
      const co = aRow[j];
      if (Math.abs(co) < 1e-12) continue;
      const abs = Math.abs(co);
      const coef = Math.abs(abs - 1) < 1e-12 ? '' : fmtCell(abs);
      const sign = co < 0 ? '−' : (terms.length ? '+' : '');
      terms.push(`${sign}${coef}x${sub(j + 1)}`);
    }
    const lhs = terms.length ? terms.join(' ') : '0';
    return `${lhs} = ${fmtCell(bVal)}`;
  },

  // 分解下一行（第 i 行，0 基）：l_i1 = a_i1/l_11；
  // l_ij = (a_ij − Σ_{k<j} l_ik·l_jk)/l_jj（j<i）；l_ii = √(a_ii − Σ_{k<i} l_ik²)。
  // 返回 true 推进一拍，false 被拦截（供自动播放器停机）；播放中手动点步不响应。
  onFactorStep() {
    if (this._player && !this._autoStepping) return false;
    if (this.data.stage !== 'factor') return false;
    const n = this.data.n;
    const i = this.data.factorLines.length;
    if (i >= n) return false;
    const { A } = this.data.config;
    const parts = [];
    if (i === 0) {
      if (!(A[0][0] > 0)) {
        wx.showToast({ title: 'a₁₁≤0，矩阵非正定，分解中断', icon: 'none' });
        return false;
      }
      const v = Math.sqrt(A[0][0]);
      this._L[0][0] = v;
      parts.push(`l₁₁ = √a₁₁ = √${fmtCell(A[0][0])} = ${fmtCell(v)}`);
    } else {
      for (let j = 0; j < i; j++) {
        let s = A[i][j];
        const prodTerms = [];
        const numTerms = [];
        for (let k = 0; k < j; k++) {
          s -= this._L[i][k] * this._L[j][k];
          prodTerms.push(`l${sub(i + 1)}${sub(k + 1)}·l${sub(j + 1)}${sub(k + 1)}`);
          numTerms.push(`${fmtCell(this._L[i][k])}×${fmtCell(this._L[j][k])}`);
        }
        const v = s / this._L[j][j];
        this._L[i][j] = v;
        if (j === 0) {
          parts.push(`l${sub(i + 1)}₁ = a${sub(i + 1)}₁/l₁₁ = ${fmtCell(A[i][0])}/${fmtCell(this._L[0][0])} = ${fmtCell(v)}`);
        } else {
          parts.push(`l${sub(i + 1)}${sub(j + 1)} = (a${sub(i + 1)}${sub(j + 1)} − ${prodTerms.join(' − ')})/l${sub(j + 1)}${sub(j + 1)} = (${fmtCell(A[i][j])} − ${numTerms.join(' − ')})/${fmtCell(this._L[j][j])} = ${fmtCell(v)}`);
        }
      }
      let s2 = A[i][i];
      const sqTerms = [];
      const sqNums = [];
      for (let k = 0; k < i; k++) {
        s2 -= this._L[i][k] * this._L[i][k];
        sqTerms.push(`l${sub(i + 1)}${sub(k + 1)}²`);
        sqNums.push(fmtCell(this._L[i][k] * this._L[i][k]));
      }
      if (s2 <= 1e-12) {
        wx.showToast({ title: `第 ${i + 1} 行根号内≤0，矩阵非正定，分解中断`, icon: 'none' });
        return false;
      }
      const d = Math.sqrt(s2);
      this._L[i][i] = d;
      parts.push(`l${sub(i + 1)}${sub(i + 1)} = √(a${sub(i + 1)}${sub(i + 1)} − ${sqTerms.join(' − ')}) = √(${fmtCell(A[i][i])} − ${sqNums.join(' − ')}) = ${fmtCell(d)}`);
    }
    const factorLines = this.data.factorLines.concat([{ k: 'f' + i, text: parts.join('；') }]);
    this.setData({ factorLines });
    if (factorLines.length === n) {
      this.setData({ stage: 'forward' });
      this.refreshLGrid(-1);
    } else {
      this.refreshLGrid(factorLines.length);
    }
    return true;
  },

  // 前代下一步（第 i 行，0 基）：y_i = (b_i − Σ_{k<i} l_ik·y_k)/l_ii。
  // 返回 true 推进一拍，false 被拦截；播放中手动点步不响应。
  onForwardStep() {
    if (this._player && !this._autoStepping) return false;
    if (this.data.stage !== 'forward') return false;
    const n = this.data.n;
    const i = this.data.forwardLines.length;
    if (i >= n) return false;
    const { b } = this.data.config;
    if (!this._y) this._y = new Array(n);
    let s = b[i];
    const prodTerms = [];
    const numTerms = [];
    for (let k = 0; k < i; k++) {
      s -= this._L[i][k] * this._y[k];
      prodTerms.push(`l${sub(i + 1)}${sub(k + 1)}·y${sub(k + 1)}`);
      numTerms.push(`${fmtCell(this._L[i][k])}×${fmtCell(this._y[k])}`);
    }
    this._y[i] = s / this._L[i][i];
    let line;
    if (i === 0) {
      line = `y₁ = b₁/l₁₁ = ${fmtCell(b[0])}/${fmtCell(this._L[0][0])} = ${fmtCell(this._y[0])}`;
    } else {
      line = `y${sub(i + 1)} = (b${sub(i + 1)} − ${prodTerms.join(' − ')})/l${sub(i + 1)}${sub(i + 1)} = (${fmtCell(b[i])} − ${numTerms.join(' − ')})/${fmtCell(this._L[i][i])} = ${fmtCell(this._y[i])}`;
    }
    const forwardLines = this.data.forwardLines.concat([{ k: 'y' + i, text: line }]);
    if (forwardLines.length === n) {
      this.setData({ forwardLines, stage: 'back' });
    } else {
      this.setData({ forwardLines });
    }
    return true;
  },

  // 回代下一步（第 i 行，0 基 n-1..0）：末行 x = y/l_ii；其余 x = (y − Σ_{k>i} l_ki·x_k)/l_ii。
  // 返回 true 推进一拍，false 被拦截；播放中手动点步不响应。
  onBackStep() {
    if (this._player && !this._autoStepping) return false;
    if (this.data.stage !== 'back') return false;
    const n = this.data.n;
    const i = n - 1 - this.data.backLines.length;
    if (i < 0) return false;
    if (!this._x) this._x = new Array(n);
    let s = this._y[i];
    const prodTerms = [];
    const numTerms = [];
    for (let k = i + 1; k < n; k++) {
      s -= this._L[k][i] * this._x[k];
      prodTerms.push(`l${sub(k + 1)}${sub(i + 1)}·x${sub(k + 1)}`);
      numTerms.push(`${fmtCell(this._L[k][i])}×${fmtCell(this._x[k])}`);
    }
    this._x[i] = s / this._L[i][i];
    let line;
    if (i === n - 1) {
      line = `x${sub(n)} = y${sub(n)}/l${sub(n)}${sub(n)} = ${fmtCell(this._y[i])}/${fmtCell(this._L[i][i])} = ${fmtCell(this._x[i])}`;
    } else {
      line = `x${sub(i + 1)} = (y${sub(i + 1)} − ${prodTerms.join(' − ')})/l${sub(i + 1)}${sub(i + 1)} = (${fmtCell(this._y[i])} − ${numTerms.join(' − ')})/${fmtCell(this._L[i][i])} = ${fmtCell(this._x[i])}`;
    }
    const backLines = this.data.backLines.concat([{ k: 'x' + i, text: line }]);
    if (backLines.length === n) {
      // 全部求出 → 进入结果阶段，给出解向量输入与 L/y 审计输入
      const inputs = [];
      for (let d = 0; d < n; d++) {
        inputs.push({ label: `x${sub(d + 1)}`, value: '' });
      }
      const ls = [];
      for (let r = 0; r < n; r++) {
        for (let c = 0; c <= r; c++) {
          ls.push({ label: `l${sub(r + 1)}${sub(c + 1)}`, ri: r, ci: c, value: '' });
        }
      }
      const ys = [];
      for (let d = 0; d < n; d++) {
        ys.push({ label: `y${sub(d + 1)}`, value: '' });
      }
      this.setData({
        backLines,
        stage: 'done',
        finished: true,
        solutionInputs: inputs,
        lInputs: ls,
        yInputs: ys,
        solutionText: `x = (${this._x.map(v => fmtCell(v)).join(', ')})`
      });
      this.setData({ chartValues: this.snapshotChartValues() });
    } else {
      this.setData({ backLines });
      this.setData({ chartValues: this.snapshotChartValues() });
    }
    return true;
  },

  // 自动演示 / 停止演示：分解 → 前代 → 回代，一拍拍填满 L 网格与条形图
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
        if (this.data.stage === 'factor') r = this.onFactorStep();
        else if (this.data.stage === 'forward') r = this.onForwardStep();
        else if (this.data.stage === 'back') r = this.onBackStep();
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

  // 回代的进度快照：已求出的分量给数值，其余 null（条形图逐根长出）
  snapshotChartValues() {
    const n = this.data.n;
    const x = this._x || [];
    const arr = new Array(n);
    for (let j = 0; j < n; j++) {
      arr[j] = isFinite(x[j]) ? x[j] : null;
    }
    return arr;
  },

  // 学生填写解向量 / L 因子 / 前代向量审计
  onSolutionInput(e) {
    const idx = Number(e.currentTarget.dataset.idx);
    this.setData({ [`solutionInputs[${idx}].value`]: e.detail.value });
  },

  onLInput(e) {
    const idx = Number(e.currentTarget.dataset.idx);
    this.setData({ [`lInputs[${idx}].value`]: e.detail.value });
  },

  onYInput(e) {
    const idx = Number(e.currentTarget.dataset.idx);
    this.setData({ [`yInputs[${idx}].value`]: e.detail.value });
  },

  async submitReadings() {
    if (this.data.submitting) return;
    const { levelId, finished, solutionInputs, lInputs, yInputs, config } = this.data;
    if (!finished || !solutionInputs.length) {
      wx.showToast({ title: '请先完成分解、前代与回代', icon: 'none' });
      return;
    }
    if (solutionInputs.some(it => it.value === '' || !isFinite(Number(it.value)))) {
      wx.showToast({ title: '请完整填写全部解分量（数值）', icon: 'none' });
      return;
    }
    const solution = solutionInputs.map(it => Number(it.value));
    const reading = { solution };
    // L 因子 / 前代向量审计选填：各自全填或全空，填了才上报（仅审计不打分）
    if (lInputs.some(it => String(it.value).trim() !== '')) {
      if (lInputs.some(it => String(it.value).trim() === '' || !isFinite(Number(it.value)))) {
        wx.showToast({ title: 'L 因子要么全填要么全空', icon: 'none' });
        return;
      }
      // 下三角输入还原为 n 阶方阵（上三角补 0），与契约 readings.L 对齐
      const n = solution.length;
      const L = Array.from({ length: n }, () => new Array(n).fill(0));
      lInputs.forEach(it => { L[it.ri][it.ci] = Number(it.value); });
      reading.L = L;
    }
    if (yInputs.some(it => String(it.value).trim() !== '')) {
      if (yInputs.some(it => String(it.value).trim() === '' || !isFinite(Number(it.value)))) {
        wx.showToast({ title: '前代向量 y 要么全填要么全空', icon: 'none' });
        return;
      }
      reading.y = yInputs.map(it => Number(it.value));
    }
    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit',
        method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 29,
          experiment: 'cholesky_solve',
          readings: [reading],
          // 携带实际出题参数：换题时后端按此阵重做 LLᵀ 分解重算真值（契约 §12）
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
      console.error('[cholesky-solve] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally {
      this.setData({ submitting: false });
    }
  }
});
