// components/sol-bars/sol-bars.js
// 高斯消去（直接法）解向量可视化：
//   柱图模式 —— 每求出一个分量 xᵢ，对应柱条长出；
//   三维模式 —— n=3 时由页面传入 space={A,b}，画三张平面，全部回代完成后标出交点（解）。
// 单指点击小图全屏；三维模式下拖动旋转、双指缩放；全屏双击退出。
import { niceStep, fmtTick, createGestureTracker } from '../../utils/graphview';
import { drawScene3d } from '../../utils/plane3d';
import { createFxBucket, startRaf, tween, easeLinear, easeOutCubic, mixColor } from '../../utils/anim';

function sub(d) {
  return '₀₁₂₃₄₅₆₇₈₉'.charAt(d);
}

Component({
  // 虚拟节点：外部 class（页面 .iter-graph）合并到根节点，直接参与页面 flex 布局
  options: { virtualHost: true },

  properties: {
    values: { type: Array, value: [] },
    // n=3 时由页面传入 { A, b }（原始方程组），开启「三维空间」标签
    space: { type: Object, value: null }
  },

  data: {
    p3mode: 'bars', // bars | space
    fullscreen: false,
    cw: 0,
    ch: 280,
    fw: 0,
    fh: 0
  },

  lifetimes: {
    ready() {
      const sys = wx.getSystemInfoSync();
      this._pr = sys.pixelRatio || 1;
      this._gest = createGestureTracker();
      this._s3 = { yaw: -0.7, pitch: 0.38, zoom: 1 };
      this._fx = createFxBucket();
      this._loop = null;          // 三维模式的帧循环（脉冲光环/拖拽惯性互换）
      this._loopKey = null;
      this._pulseT = 0;
      this._spaceShownAt = 0;     // 进入三维时刻（平面 500ms 淡入）
      this._solvedFlags = [];
      this._barProg = []; // 每根柱的生长进度 0..1
      this._greenT = 0;   // 全部求齐后 蓝→绿 颜色补间进度
      this._wasAllDone = false;
      this._smallReady = false;
      // 全屏画布高度 = 屏高 − 顶栏(88rpx) − 底栏提示行（约 64rpx）
      const rpx = sys.windowWidth / 750;
      this.setData({
        cw: sys.windowWidth,
        fw: sys.windowWidth,
        fh: Math.max(160, sys.windowHeight - 88 * rpx - 64 * rpx),
        ch: 280
      }, () => {
        const q = this.createSelectorQuery();
        q.select('#plot').fields({ node: true, size: true }).exec((res) => {
          if (!res || !res[0] || !res[0].node) return;
          this._cv = res[0].node;
          this._ctx = this._cv.getContext('2d');
          this._smallW = res[0].width;
          this._smallH = res[0].height;
          this._smallReady = true;
          this.draw();
        });
      });
    },
    detached() {
      if (this._fx) this._fx.dispose();
      this._loop = null;
      this._loopKey = null;
      this._cv = null;
      this._ctx = null;
      this._fcv = null;
      this._fctx = null;
    }
  },

  observers: {
    'values,space'() {
      // 换题后不再是三元时退回柱图
      if (!this.properties.space && this.data.p3mode === 'space') {
        if (this._fx) {
          this._fx.cancelKey('s3-inertia');
          this._fx.cancelKey('pulse');
        }
        this._loop = null;
        this._loopKey = null;
        this.setData({ p3mode: 'bars' });
      }
      this.syncBarAnim();
      if (this._smallReady) this.draw();
    }
  },

  methods: {
    modeBars() {
      if (this.data.p3mode === 'bars') return;
      if (this._fx) {
        this._fx.cancelKey('s3-inertia');
        this._fx.cancelKey('pulse');
      }
      this._loop = null;
      this._loopKey = null;
      this.setData({ p3mode: 'bars' });
      this.draw();
    },

    modeSpace() {
      if (this.data.p3mode === 'space' || !this.properties.space) return;
      this._spaceShownAt = Date.now(); // 平面 500ms 淡入
      this.setData({ p3mode: 'space' });
      this.ensureLoop();
      this.draw();
    },

    // 三维帧循环：脉冲光环与拖拽惯性共用，离开三维模式即停（省电）
    ensureLoop() {
      if (this._loop || !this._cv) return;
      this._loop = startRaf(this._cv, (dt) => {
        if (this.data.p3mode !== 'space') {
          this._loop = null;
          this._loopKey = null;
          return false;
        }
        this._pulseT += (dt > 0 && dt < 100) ? dt : 16;
        this.draw();
        return true;
      });
      this._loopKey = 'pulse';
      this._fx.add(this._loop, 'pulse');
    },

    // 三维抬手惯性：与 conv-plot 同款（样本估速、指数衰减、俯仰夹边）
    kickS3Inertia() {
      if (this._fx) {
        this._fx.cancelKey('s3-inertia');
        this._fx.cancelKey('pulse');
      }
      this._loop = null;
      this._loopKey = null;
      const samples = this._dragSamples || [];
      if (samples.length < 2 || !this._cv) { this.ensureLoop(); return; }
      const a = samples[0], z = samples[samples.length - 1];
      const dtms = Math.max(1, z.t - a.t);
      let vyaw = (z.dx - a.dx) / dtms * 0.011;
      let vpitch = (z.dy - a.dy) / dtms * 0.011;
      const VMIN = 0.00012;
      if (Math.abs(vyaw) < VMIN && Math.abs(vpitch) < VMIN) { this.ensureLoop(); return; }
      const decayPerMs = Math.pow(0.94, 1 / 16);
      let last = Date.now();
      const h = startRaf(this._cv, (dt) => {
        if (this.data.p3mode !== 'space' || !this._s3) {
          this._loop = null;
          this._loopKey = null;
          return false;
        }
        const now = Date.now();
        const d = (dt > 0 && dt < 100) ? dt : Math.min(50, now - last);
        last = now;
        const f = Math.pow(decayPerMs, d);
        vyaw *= f;
        vpitch *= f;
        this._s3.yaw += vyaw * d;
        const np = this._s3.pitch + vpitch * d;
        if (np >= 1.45 || np <= -1.45) {
          this._s3.pitch = np >= 1.45 ? 1.45 : -1.45;
          vpitch = 0;
        } else {
          this._s3.pitch = np;
        }
        this._pulseT += d;
        this.draw();
        if (Math.abs(vyaw) < VMIN * 0.25 && Math.abs(vpitch) < VMIN * 0.25) {
          this._loop = null;
          this._loopKey = null;
          this.ensureLoop();
          return false;
        }
        return true;
      });
      this._loop = h;
      this._loopKey = 's3-inertia';
      this._fx.add(h, 's3-inertia');
    },

    enterFs() {
      this.setData({ fullscreen: true }, () => {
        if (this._fcv) {
          if (this._loop) this._loop.setCanvas(this._fcv); // 帧循环挂到可见的全屏画布
          this.draw();
          return;
        }
        const q = this.createSelectorQuery();
        q.select('#fullPlot').fields({ node: true, size: true }).exec((res) => {
          if (!res || !res[0] || !res[0].node) return;
          this._fcv = res[0].node;
          this._fctx = this._fcv.getContext('2d');
          if (this._loop) this._loop.setCanvas(this._fcv);
          this.draw();
        });
      });
    },

    exitFs() {
      // wx:if 会销毁全屏 canvas 节点，缓存须作废，下次进入重新查询
      this._fcv = null;
      this._fctx = null;
      if (this._loop && this._cv) this._loop.setCanvas(this._cv);
      this.setData({ fullscreen: false }, () => this.draw());
    },

    zoomBy(e) {
      const f = Number(e.currentTarget.dataset.f);
      if (!(f > 0)) return;
      this._s3.zoom = Math.max(0.35, Math.min(8, this._s3.zoom * f));
      this.draw();
    },

    reset3d() {
      if (this._fx) this._fx.cancelKey('s3-inertia');
      if (this._loopKey === 's3-inertia') {
        this._loop = null;
        this._loopKey = null;
        this.ensureLoop();
      }
      this._s3 = { yaw: -0.7, pitch: 0.38, zoom: 1 };
      this.draw();
    },

    // ================= 触摸（柱图模式只认点击；三维模式拖动旋转/捏合缩放） =================

    onTS(e) {
      if (this.data.p3mode === 'space') {
        const mode = this._gest.anchor(e.touches);
        if (mode) {
          if (this._fx) this._fx.cancelKey('s3-inertia');
          // 惯性循环被取消：补一个脉冲循环，保证轻点/停留时解点光环不停
          if (this._loopKey === 's3-inertia') {
            this._loop = null;
            this._loopKey = null;
            this.ensureLoop();
          }
          this._s30 = { ...this._s3 };
          this._dragSamples = [];
        }
        return;
      }
      const t = e.touches[0];
      this._down = { x: t.clientX, y: t.clientY, t: Date.now() };
    },

    onTM(e) {
      if (this.data.p3mode !== 'space') return;
      const g = this._gest.move(e.touches);
      if (!g || !this._s30) return;
      const s = this._s30;
      if (g.type === 'pan') {
        this._s3.yaw = s.yaw + g.dxPx * 0.011;
        this._s3.pitch = Math.max(-1.45, Math.min(1.45, s.pitch + g.dyPx * 0.011));
        // 最近拖动样本（相对锚点的累计位移），抬手时估速度做惯性
        (this._dragSamples = this._dragSamples || []).push({ t: Date.now(), dx: g.dxPx, dy: g.dyPx });
        if (this._dragSamples.length > 6) this._dragSamples.shift();
      } else {
        this._s3.yaw = s.yaw;
        this._s3.pitch = Math.max(-1.45, Math.min(1.45, s.pitch));
        this._s3.zoom = Math.max(0.35, Math.min(8, s.zoom * g.ratio));
      }
      this.draw();
    },

    onTE(e) {
      // 三维模式：剩余手指重新锚定；全部离开交给 tracker 判定双击退出
      if (this.data.p3mode === 'space') {
        if (e.touches && e.touches.length > 0) {
          const mode = this._gest.anchor(e.touches);
          if (mode) {
            this._s30 = { ...this._s3 };
            this._dragSamples = []; // 重新锚点后旧样本坐标系失效
          }
          return;
        }
        const t = this._gest.end();
        this.kickS3Inertia(); // 抬手惯性（轻点速度为 0，内部自动转交脉冲循环）
        this._dragSamples = [];
        if (t === 'doubletap' && e.currentTarget.id === 'fullPlot') this.exitFs();
        if (t === 'tap' && e.currentTarget.id !== 'fullPlot') this.enterFs();
        return;
      }

      const d = this._down;
      this._down = null;
      if (!d) return;
      const t = e.changedTouches[0];
      const moved = Math.hypot(t.clientX - d.x, t.clientY - d.y);
      if (moved > 12 || Date.now() - d.t > 350) return;
      const isFull = e.currentTarget.id === 'fullPlot';
      const now = Date.now();
      if (isFull && now - this._lastTap < 320) {
        this.exitFs();
        this._lastTap = 0;
        return;
      }
      this._lastTap = now;
      if (!isFull) this.enterFs();
    },

    // values 变化 → 驱动柱条生长与全部求齐后的变绿补间
    syncBarAnim() {
      if (!this._fx) return;
      const vals = this.properties.values || [];
      const prev = this._solvedFlags || [];
      const flags = vals.map(v => v !== null && v !== undefined && isFinite(v));
      // 维数变化（换题重开）：已有的已求出分量直接到位，其余等待生长
      if (vals.length !== prev.length || !this._barProg || this._barProg.length !== vals.length) {
        this._barProg = flags.map(() => 1);
      }
      const newly = [];
      flags.forEach((f, i) => { if (f && !prev[i]) newly.push(i); });
      this._solvedFlags = flags;

      if (newly.length) {
        newly.forEach(i => { this._barProg[i] = 0; });
        const GROW = 380;       // 单根柱生长时长
        const STAGGER = 70;     // 多根同时求出时的错峰
        const total = GROW + STAGGER * (newly.length - 1);
        this._fx.add(tween({
          duration: total,
          ease: easeLinear,
          canvas: this.data.fullscreen ? this._fcv : this._cv,
          onUpdate: (_, raw) => {
            newly.forEach((idx, order) => {
              const u = Math.max(0, Math.min(1, (raw * total - STAGGER * order) / GROW));
              this._barProg[idx] = easeOutCubic(u);
            });
            this.draw();
          }
        }), 'bars-grow');
      }

      const solvedCount = flags.filter(Boolean).length;
      const allDone = vals.length > 0 && solvedCount === vals.length;
      if (allDone && !this._wasAllDone) {
        this._wasAllDone = true;
        this._greenT = 0;
        this._fx.add(tween({
          duration: 420,
          ease: easeOutCubic,
          canvas: this.data.fullscreen ? this._fcv : this._cv,
          onUpdate: (t) => { this._greenT = t; this.draw(); },
          onDone: () => { this._greenT = 1; this.draw(); }
        }), 'bars-green');
      } else if (!allDone && this._wasAllDone) {
        // 重开（回代清零）：立刻退回蓝色
        this._wasAllDone = false;
        this._greenT = 0;
        this._fx.cancelKey('bars-green');
      }
    },

    draw() {
      // 全屏期间小画布被隐藏，跳过其重绘（S9：触摸拖动只画一块画布）
      if (!this.data.fullscreen && this._smallReady && this._cv && this._ctx) {
        this._drawOn(this._cv, this._ctx, this._smallW, this._smallH);
      }
      if (this.data.fullscreen && this._fcv && this._fctx) {
        this._drawOn(this._fcv, this._fctx, this.data.fw, this.data.fh);
      }
    },

    _drawOn(canvas, ctx, w, h) {
      const pr = this._pr || 1;
      // backing store 仅在 css 尺寸变化时重设（每帧重赋 width 会清空状态且有开销）
      const bw = Math.round(w * pr), bh = Math.round(h * pr);
      if (canvas.width !== bw || canvas.height !== bh) {
        canvas.width = bw;
        canvas.height = bh;
      }
      ctx.setTransform(pr, 0, 0, pr, 0, 0);
      ctx.fillStyle = '#10182a';
      ctx.fillRect(0, 0, w, h);

      if (this.data.p3mode === 'space' && this.properties.space) {
        const vals = this.properties.values || [];
        const solved = vals.length === 3 && vals.every(v => v !== null && v !== undefined && isFinite(v));
        const s3 = this._s3;
        const planeAlpha = this._spaceShownAt
          ? Math.min(1, (Date.now() - this._spaceShownAt) / 500) : 1;
        drawScene3d({
          ctx, w, h,
          mL: 10, mT: 12, pw: w - 20, ph: h - 24,
          A: this.properties.space.A,
          b: this.properties.space.b,
          yaw: s3.yaw, pitch: s3.pitch, zoom: s3.zoom,
          path: [],
          point: solved ? vals.slice() : null,
          time: this._pulseT,    // 解点双脉冲光环
          planeAlpha             // 进入三维时平面淡入
        });
        if (!solved) {
          ctx.textAlign = 'center';
          ctx.font = '12px sans-serif';
          ctx.fillStyle = 'rgba(255,255,255,0.55)';
          ctx.fillText('完成回代后在此标出三平面交点（解）', w / 2, h - 10);
        }
        return;
      }

      this._drawBars(canvas, ctx, w, h);
    },

    _drawBars(canvas, ctx, w, h) {
      const vals = this.properties.values || [];
      const n = vals.length;

      const mL = 52, mR = 16, mT = 20, mB = 36;
      const pw = w - mL - mR;
      const ph = h - mT - mB;
      if (pw <= 0 || ph <= 0) return;

      // 值轴范围（包含 0）
      let lo = 0, hi = 0;
      vals.forEach((v) => {
        if (v === null || v === undefined || !isFinite(v)) return;
        if (v < lo) lo = v;
        if (v > hi) hi = v;
      });
      if (hi - lo < 1e-9) { hi += 1; lo -= 1; }
      const pad = (hi - lo) * 0.18;
      lo -= pad; hi += pad;

      const Y = (v) => mT + (hi - v) / (hi - lo) * ph;

      // 网格与刻度
      const step = niceStep((hi - lo) / 4);
      ctx.font = '10px sans-serif';
      ctx.lineWidth = 1;
      ctx.textAlign = 'right';
      for (let i = Math.ceil(lo / step); i * step <= hi; i++) {
        const py = Y(i * step);
        if (py < mT - 2 || py > mT + ph + 2) continue;
        ctx.strokeStyle = 'rgba(255,255,255,0.08)';
        ctx.beginPath(); ctx.moveTo(mL, py); ctx.lineTo(mL + pw, py); ctx.stroke();
        ctx.fillStyle = 'rgba(255,255,255,0.45)';
        ctx.fillText(fmtTick(i * step), mL - 6, py + 3);
      }

      // 零基线
      ctx.strokeStyle = 'rgba(255,255,255,0.45)';
      ctx.beginPath(); ctx.moveTo(mL, Y(0)); ctx.lineTo(mL + pw, Y(0)); ctx.stroke();

      const solvedCount = vals.filter(v => v !== null && v !== undefined && isFinite(v)).length;
      const allDone = n > 0 && solvedCount === n;

      if (n === 0) {
        ctx.textAlign = 'center';
        ctx.font = '13px sans-serif';
        ctx.fillStyle = 'rgba(255,255,255,0.5)';
        ctx.fillText('完成消元后开始回代，此处逐根显示解分量', mL + pw / 2, mT + ph / 2);
        return;
      }

      // 柱条
      const slot = pw / n;
      const bw = Math.min(64, slot * 0.56);
      vals.forEach((v, i) => {
        const cx = mL + slot * (i + 0.5);
        const x = cx - bw / 2;
        const label = 'x' + sub(i + 1);

        if (v === null || v === undefined || !isFinite(v)) {
          // 未求出：虚框占位
          ctx.setLineDash([5, 4]);
          ctx.strokeStyle = 'rgba(255,255,255,0.28)';
          ctx.lineWidth = 1.4;
          ctx.strokeRect(x, mT + 6, bw, ph - 12);
          ctx.setLineDash([]);
          ctx.textAlign = 'center';
          ctx.font = '16px sans-serif';
          ctx.fillStyle = 'rgba(255,255,255,0.3)';
          ctx.fillText('?', cx, mT + ph / 2 + 5);
        } else {
          // 生长进度（syncBarAnim 驱动）：柱顶从零基线长出，数值标签淡入上移
          const prog = this._barProg && this._barProg[i] != null ? this._barProg[i] : 1;
          const y0 = Y(0);
          const yvFull = Y(v);
          const yv = y0 + (yvFull - y0) * prog;
          const top = Math.min(y0, yv);
          const bh = Math.abs(yv - y0);
          const gt = allDone ? (this._greenT || 0) : 0;
          const grad = ctx.createLinearGradient(0, top, 0, top + Math.max(bh, 2));
          grad.addColorStop(0, mixColor('#5dade2', '#44e08a', gt));
          grad.addColorStop(1, mixColor('#2471a3', '#1f9d57', gt));
          ctx.fillStyle = grad;
          if (bh > 0.5) ctx.fillRect(x, top, bw, Math.max(bh, 2));
          ctx.textAlign = 'center';
          ctx.font = '12px sans-serif';
          ctx.globalAlpha = 0.3 + 0.7 * prog;
          ctx.fillStyle = '#fff';
          const ty = (v >= 0 ? top - 6 : top + bh + 14) - (1 - prog) * 6;
          ctx.fillText(fmtTick(v), cx, ty);
          ctx.globalAlpha = 1;
        }

        ctx.font = '13px sans-serif';
        ctx.fillStyle = 'rgba(255,255,255,0.75)';
        ctx.fillText(label, cx, mT + ph + 22);
      });

      // 状态角标
      ctx.textAlign = 'left';
      ctx.font = '11px sans-serif';
      ctx.fillStyle = allDone ? '#2ecc71' : 'rgba(255,255,255,0.5)';
      ctx.fillText(allDone ? '✓ 已求出全部 ' + n + ' 个分量' : '回代进度 ' + solvedCount + ' / ' + n, mL + 4, 14);
    }
  }
});
