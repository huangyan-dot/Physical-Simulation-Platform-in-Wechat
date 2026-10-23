// pages/experiments/lu-decomp/lu-decomp.js - LU 分解（Doolittle）解线性方程组（数值分析 L27）
// 契约（docs/api-contract.md 附·数值分析批次 L17，v0.22）：
//   config: { system_key, system_desc, method_hint }
//   readings: [{ solution: [x1, ..., xn], l_col? 可选审计 }]（solution 长度须=未知数个数；
//             l_col=L 乘子按行序 l21,l31,l32,...，长度 n(n-1)/2，全或无仅记审计不打分）
//   user_config: { system_key }（换题时后端按新方程组重做 Doolittle 分解重算真值解）
// 教学主线（与 L13 高斯消去同族对照）：A=LU 三角分解（不选主元——乘数 m_ik 存入 L、
//   消去结果即 U，与 L13 的差异点：无行交换，主元≈0 直接中断）→ 前代 Ly=b →
//   回代 Ux=y → 学生填解向量（选填乘子列审计）提交。一次分解可解多组 b。
// 方程组库与后端 scoring.go 的 linSystems 同步（utils/numlab.js）。
import { request } from '../../../utils/request';
import { linSystems, fmtNum } from '../../../utils/numlab';
import { createStepPlayer } from '../../../utils/anim';

// 矩阵单元展示：消去浮点噪声（如 1e-15 显示为 0），6 位有效数字
function fmtCell(v) {
  if (v === null || v === undefined || isNaN(v) || !isFinite(v)) return '-';
  if (Math.abs(v) < 1e-10) return '0';
  return String(Number(v.toPrecision(6)));
}

// 下标数字（x₁ ~ x₈ / l₂₁ 等）
function sub(d) {
  return '₀₁₂₃₄₅₆₇₈₉'.charAt(d);
}

const DEFAULT_CONFIG = {
  system_key: 'sys3_basic',
  system_desc: 'Doolittle 三角分解（不选主元）后分两步求解：前代 Ly=b 得 y，回代 Ux=y 得解。解恰为整数 (2,3,-1)，乘子列 [-1.5,-1,4] 全手算',
  A: [[2, 1, -1], [-3, -1, 2], [-2, 1, 2]],
  b: [8, -11, -3]
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
    matrixRows: [],       // 分解工作矩阵（U 的演化，n x n）
    lLines: [],           // 分解过程乘子记录（逐步追加）
    // 阶段状态机：lu（逐列分解）→ fwd（前代 Ly=b）→ back（回代 Ux=y）→ done
    stage: 'lu',
    col: 0,
    history: [],
    fwdLines: [],
    backLines: [],
    lTable: [],           // 分解完成后的 L 矩阵展示（单位下三角+乘数）
    uTable: [],           // 分解完成后的 U 矩阵展示
    solutionInputs: [],
    luInputs: [],         // 乘子审计输入（选填，l21,l31,l32,... 行序）
    solutionText: '',
    yText: '',
    finished: false,
    // 解向量条形图数据（components/sol-bars）：回代逐根长出
    chartValues: [],
    // n=3 时传给组件的三维平面数据（用原始 A/b）
    space3d: null
  },

  // 工作矩阵（A 的副本，消去中演化为 U）、L、y、解向量（回代逐格填充），不进 data
  _U: null,
  _L: null,
  _y: null,
  _x: null,

  onLoad(options) {
    const levelId = options.levelId;
    // 教师/管理员预览：可自由操作实验，隐藏提交（/progress/submit 仅学生可用）
    const app = getApp();
    const role = app.globalData.userInfo ? app.globalData.userInfo.role : null;
    const keys = Object.keys(linSystems);
    this.setData({
      levelId,
      isPreview: role === 'teacher' || role === 'admin',
      systemKeys: keys,
      systemLabels: keys.map(k => linSystems[k].label)
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
        const key = data.config.system_key || 'sys3_basic';
        // A/b 一律以库内定义为准（与后端评分真值同源）
        const def = linSystems[key];
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
      console.warn('[lu-decomp] 后端不可用，使用默认配置');
    }
    this.restart();
  },

  // 换题（picker）：随 user_config 提交 system_key，后端按新方程组重做分解重算真值
  onSystemChange(e) {
    const idx = Number(e.detail.value);
    const key = this.data.systemKeys[idx];
    if (!key || key === this.data.config.system_key) return;
    const def = linSystems[key];
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

  // 重开分解：重建工作矩阵与 L；题目展示（原方程组）冻结在初始 A/b 上
  restart() {
    this._cfgGen = (this._cfgGen || 0) + 1; // 学生主动重算：使在途配置请求作废
    this.stopAutoRun();
    const { config } = this.data;
    const n = config.A.length;
    this._U = config.A.map(row => row.map(Number));
    this._L = Array.from({ length: n }, (_, i) =>
      Array.from({ length: n }, (_, j) => (i === j ? 1 : 0)));
    this._y = null;
    this._x = null;
    this.setData({
      n,
      equations: config.A.map((row, i) => this.formatEquation(row, config.b[i])),
      stage: 'lu',
      col: 0,
      history: [],
      lLines: [],
      fwdLines: [],
      backLines: [],
      lTable: [],
      uTable: [],
      solutionInputs: [],
      luInputs: [],
      solutionText: '',
      yText: '',
      finished: false,
      chartValues: new Array(n).fill(null),
      // 三维平面用原始方程组（学生认原平面，分解是等价变换）
      space3d: n === 3 ? { A: config.A.map(row => row.map(Number)), b: config.b.map(Number) } : null
    });
    this.refreshDisplay();
  },

  // 回代进度快照：已求出的分量给数值，其余 null（条形图逐根长出）
  snapshotChartValues() {
    const n = this.data.n;
    const x = this._x || [];
    const arr = new Array(n);
    for (let j = 0; j < n; j++) {
      arr[j] = isFinite(x[j]) ? x[j] : null;
    }
    return arr;
  },

  // 刷新展示字段（分解工作矩阵 + L 乘子进度；原方程组在 restart 冻结）
  refreshDisplay() {
    const { n, stage, col } = this.data;
    const U = this._U;
    const L = this._L;
    // 本拍变动快照（onLuStep 在动手前写入，消费一次后清空）
    const prev = this._pulsePrev;
    const pivotRow = this._pulsePivot;
    this.setData({
      systemDesc: this.data.config.system_desc,
      matrixRows: U.map((row, ri) => {
        let rcls = stage === 'lu' && ri < col ? 'done-row' : '';
        if (stage === 'lu' && pivotRow === ri) rcls += ' row-pivot';
        return {
          k: 'r' + ri,
          cls: rcls,
          cells: row.map((v, ci) => {
            let ccls = stage === 'lu' && ci === col && ri >= col ? 'cell-pc' : '';
            if (prev && Math.abs(prev[ri][ci] - v) > 1e-12) {
              ccls += (ci === col && Math.abs(v) < 1e-10) ? ' cell-zero-new' : ' cell-new';
            }
            return { k: 'c' + ci, t: fmtCell(v), cls: ccls };
          })
        };
      }),
      lLines: Array.from({ length: n }, (_, i) => {
        const parts = [];
        for (let j = 0; j < i; j++) {
          if (L[i][j] !== 0) parts.push(`l${sub(i + 1)}${sub(j + 1)} = ${fmtCell(L[i][j])}`);
        }
        return { k: 'l' + i, text: parts.length ? parts.join('，') : '' };
      }).filter(it => it.text)
    });
    this._pulsePrev = null;
    this._pulsePivot = null;
  },

  // 原方程组的可读形式：2x₁ + x₂ − x₃ = 8
  formatEquation(aRow, bVal) {
    const n = aRow.length;
    const terms = [];
    for (let j = 0; j < n; j++) {
      const c = aRow[j];
      if (Math.abs(c) < 1e-12) continue;
      const abs = Math.abs(c);
      const coef = Math.abs(abs - 1) < 1e-12 ? '' : fmtCell(abs);
      const sign = c < 0 ? '−' : (terms.length ? '+' : '');
      terms.push(`${sign}${coef}x${sub(j + 1)}`);
    }
    const lhs = terms.length ? terms.join(' ') : '0';
    return `${lhs} = ${fmtCell(bVal)}`;
  },

  // 分解下一步：对第 col 列消去，乘数 m_ik 存入 L（不选主元、不交换行）。
  // 返回 true 推进一拍，false 被拦截（供自动播放器停机）；播放中手动点步不响应。
  onLuStep() {
    if (this._player && !this._autoStepping) return false;
    if (this.data.stage !== 'lu') return false;
    const k = this.data.col;
    const n = this.data.n;
    const U = this._U;
    if (Math.abs(U[k][k]) < 1e-12) {
      wx.showToast({ title: '主元≈0，不选主元的 LU 分解中断', icon: 'none' });
      return false;
    }
    // 动效快照：本拍之前的 U 格值与主元行，refreshDisplay 据此弹入/零化/辉光
    this._pulsePrev = U.map(r => r.slice());
    this._pulsePivot = k;
    const mults = [];
    for (let i = k + 1; i < n; i++) {
      const m = U[i][k] / U[k][k];
      this._L[i][k] = m;
      mults.push(`l${sub(i + 1)}${sub(k + 1)} = ${fmtCell(U[i][k])}/${fmtCell(U[k][k])} = ${fmtCell(m)}`);
      for (let j = k; j < n; j++) {
        U[i][j] -= m * U[k][j];
      }
    }
    this.setData({
      col: k + 1,
      history: this.data.history.concat([
        `第 ${k + 1} 列：主元 u${sub(k + 1)}${sub(k + 1)} = ${fmtCell(U[k][k])}（不选主元、不交换行），乘数 ${mults.length ? mults.join('；') : '无（最后一列）'}`
      ])
    });
    if (this.data.col >= n - 1) {
      this.finishDecompose();
    }
    this.refreshDisplay();
    return true;
  },

  // 分解完成：冻结 L/U 展示矩阵，进入前代
  finishDecompose() {
    this.setData({
      stage: 'fwd',
      lTable: this._L.map((row, ri) => ({
        k: 'lr' + ri,
        cells: row.map((v, ci) => ({ k: 'lc' + ci, t: fmtCell(v), cls: 'cell-new' }))
      })),
      uTable: this._U.map((row, ri) => ({
        k: 'ur' + ri,
        cells: row.map((v, ci) => ({ k: 'uc' + ci, t: fmtCell(v), cls: 'cell-new' }))
      }))
    });
  },

  // 自动演示 / 停止演示：分解 → 前代 → 回代，一拍拍填满工作表与条形图
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
        if (this.data.stage === 'lu') r = this.onLuStep();
        else if (this.data.stage === 'fwd') r = this.onFwdStep();
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

  // 前代下一步：y_i = b_i − Σ_{j<i} l_ij·y_j（L 单位下三角，按行从上往下）。
  // 返回 true 推进一拍，false 被拦截；播放中手动点步不响应。
  onFwdStep() {
    if (this._player && !this._autoStepping) return false;
    if (this.data.stage !== 'fwd') return false;
    this.fwdOneStep();
    return true;
  },

  fwdOneStep() {
    const n = this.data.n;
    const L = this._L;
    const i = this.data.fwdLines.length;
    if (i >= n) return;
    if (!this._y) this._y = new Array(n);
    let s = this.data.config.b[i];
    const parts = [];
    for (let j = 0; j < i; j++) {
      if (Math.abs(L[i][j]) >= 1e-12) {
        parts.push(`${fmtCell(L[i][j])}·${fmtCell(this._y[j])}`);
        s -= L[i][j] * this._y[j];
      }
    }
    this._y[i] = s;
    const rhs = parts.length ? `(${fmtCell(this.data.config.b[i])} − (${parts.join(' + ')}))` : `${fmtCell(this.data.config.b[i])}`;
    const fwdLines = this.data.fwdLines.concat([
      { k: 'f' + i, text: `y${sub(i + 1)} = ${rhs} = ${fmtCell(s)}` }
    ]);
    if (fwdLines.length === n) {
      this.setData({
        fwdLines,
        stage: 'back',
        yText: `y = (${this._y.map(v => fmtCell(v)).join(', ')})`
      });
    } else {
      this.setData({ fwdLines });
    }
  },

  // 回代下一步：x_i = (y_i − Σ_{j>i} u_ij·x_j) / u_ii（上三角，从最后一个开始）。
  // 返回 true 推进一拍，false 被拦截；播放中手动点步不响应。
  onBackStep() {
    if (this._player && !this._autoStepping) return false;
    if (this.data.stage !== 'back') return false;
    this.backOneStep();
    this.setData({ chartValues: this.snapshotChartValues() });
    return true;
  },

  backOneStep() {
    const n = this.data.n;
    const U = this._U;
    const i = n - 1 - this.data.backLines.length;
    if (i < 0) return;
    if (!this._x) this._x = new Array(n);
    let s = this._y[i];
    const parts = [];
    for (let j = i + 1; j < n; j++) {
      if (Math.abs(U[i][j]) >= 1e-12) {
        parts.push(`${fmtCell(U[i][j])}·${fmtCell(this._x[j])}`);
        s -= U[i][j] * this._x[j];
      }
    }
    const xi = s / U[i][i];
    this._x[i] = xi;
    const rhs = parts.length ? `(${fmtCell(this._y[i])} − (${parts.join(' + ')}))` : `${fmtCell(this._y[i])}`;
    const backLines = this.data.backLines.concat([
      { k: 'b' + i, text: `x${sub(i + 1)} = ${rhs} / ${fmtCell(U[i][i])} = ${fmtCell(xi)}` }
    ]);
    // 全部求出 → 进入结果阶段，给出解向量输入与乘子审计输入
    if (backLines.length === n) {
      const inputs = [];
      for (let d = 0; d < n; d++) {
        inputs.push({ label: `x${sub(d + 1)}`, value: '' });
      }
      // 乘子审计输入（选填，全或无）：l21,l31,l32,... 按行序
      const lus = [];
      for (let i2 = 1; i2 < n; i2++) {
        for (let j2 = 0; j2 < i2; j2++) {
          lus.push({ label: `l${sub(i2 + 1)}${sub(j2 + 1)}`, value: '' });
        }
      }
      this.setData({
        backLines,
        stage: 'done',
        finished: true,
        solutionInputs: inputs,
        luInputs: lus,
        solutionText: `x = (${this._x.map(v => fmtCell(v)).join(', ')})`,
        chartValues: this.snapshotChartValues()
      });
    } else {
      this.setData({ backLines });
    }
  },

  // 学生填写解向量 / 乘子审计列
  onSolutionInput(e) {
    const idx = Number(e.currentTarget.dataset.idx);
    this.setData({ [`solutionInputs[${idx}].value`]: e.detail.value });
  },

  onLuInput(e) {
    const idx = Number(e.currentTarget.dataset.idx);
    this.setData({ [`luInputs[${idx}].value`]: e.detail.value });
  },

  async submitReadings() {
    if (this.data.submitting) return;
    const { levelId, finished, solutionInputs, luInputs, config } = this.data;
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
    // 乘子审计列选填：全填或全空（长度 n(n-1)/2），填了才上报（仅审计不打分）
    const anyFilled = luInputs.some(it => String(it.value).trim() !== '');
    if (anyFilled) {
      if (luInputs.some(it => String(it.value).trim() === '' || !isFinite(Number(it.value)))) {
        wx.showToast({ title: '乘子列要么全填要么全空', icon: 'none' });
        return;
      }
      reading.l_col = luInputs.map(it => Number(it.value));
    }
    this.setData({ submitting: true });
    try {
      const res = await request({
        url: '/progress/submit',
        method: 'POST',
        data: {
          level_id: parseInt(levelId, 10) || 27,
          experiment: 'lu_decomp',
          readings: [reading],
          // 携带实际出题参数：换题时后端按此方程组重做 Doolittle 分解重算真值（契约 §12）
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
      console.error('[lu-decomp] 提交失败:', err);
      wx.showToast({ title: err.message || '提交失败', icon: 'none' });
    } finally {
      this.setData({ submitting: false });
    }
  }
});
