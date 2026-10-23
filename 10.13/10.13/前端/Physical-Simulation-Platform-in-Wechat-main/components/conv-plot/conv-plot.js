// components/conv-plot/conv-plot.js
// 迭代法（雅可比 / SOR）收敛过程可视化组件，供两个实验页以 <conv-plot> 引用：
//   properties：
//     runs：已归档的历史运行 [{ label, color, deltas:[δ₁..δk], xs:[[x(0)],[x(1)]..], state }]
//     live：本次运行（结构同上，每步由页面追加后整体回传）
//     tol ：收敛容差（图中绿色虚线；δ 模式纵轴为 log10(δ)）
//   两种视图：
//     delta —— 每条运行一条 δ(k) 折线（历史运行半透明叠加，SOR 调 ω 对比用）
//     comp  —— 本次运行各分量 xᵢ(k) 折线（历史运行太多线会乱，只画本次）
// 手势（utils/graphview.js）：单指拖动、双指缩放、小图单击全屏、全屏双击退出。
import { panView, zoomView, clampView, niceStep, fmtTick, createGestureTracker } from '../../utils/graphview';
import { drawScene3d } from '../../utils/plane3d';
import { createFxBucket, startRaf, tween, easeLinear } from '../../utils/anim';

// 归档运行与分量轨迹的配色
const ARCHIVE_COLORS = ['#f39c12', '#2ecc71', '#9b59b6', '#e74c3c', '#1abc9c', '#e67e22'];
const COMP_COLORS = ['#3498db', '#e74c3c', '#2ecc71', '#f39c12', '#9b59b6', '#1abc9c', '#e67e22', '#34495e'];
const LIVE_COLOR = '#3498db';

function sub(d) {
  return '₀₁₂₃₄₅₆₇₈₉'.charAt(d);
}

Component({
  // 虚拟节点：不渲染组件宿主标签，外部 class（页面的 .iter-graph）合并到根节点，
  // 使根节点直接成为页面 flex 布局子项（同二分页画布区，避免工具条被滚动错层）
  options: { virtualHost: true },

  properties: {
    runs: { type: Array, value: [] },
    live: { type: Object, value: null },
    tol: { type: Number, value: 1e-6 },
    // n=3 时由页面传入 { A, b }，开启「三维空间」标签；n≠3 传 null
    space: { type: Object, value: null }
  },

  data: {
    chartMode: 'delta', // delta | comp | space
    fullscreen: false,
    // 小图显式像素宽高（同二分页 canvas-wrap 做法，避免同层渲染错位），全屏走系统尺寸
    cw: 0,
    ch: 300,
    fw: 0,
    fh: 0
  },

  lifetimes: {
    ready() {
      const sys = wx.getSystemInfoSync();
      this._gest = createGestureTracker();
      this._fx = createFxBucket();
      this._loop = null;          // 当前帧循环句柄（脉冲或惯性，二者互换不并存）
      this._loopKey = null;
      this._pulseT = 0;
      this._lastDataAt = 0;
      this._segGrowAt = 0;        // 最新线段出现的时刻（260ms 描边生长）
      this._lastLiveLen = 0;
      this._view = null;
      this._userMoved = false;
      // 三维视角（yaw/pitch 弧度；zoom 相对自适应缩放）
      this._s3 = { yaw: -0.7, pitch: 0.38, zoom: 1 };
      this._smallReady = false;
      // 全屏画布高度 = 屏高 − 顶栏(88rpx) − 底栏（按钮行约 100rpx）
      const rpx = sys.windowWidth / 750;
      this.setData({
        cw: sys.windowWidth,
        fw: sys.windowWidth,
        fh: Math.max(160, sys.windowHeight - 88 * rpx - 100 * rpx),
        ch: 300
      }, () => {
        const q = this.createSelectorQuery();
        q.select('#plot').fields({ node: true, size: true }).exec((res) => {
          if (!res || !res[0] || !res[0].node) return;
          this._cv = res[0].node;
          this._ctx = this._cv.getContext('2d');
          this._smallW = res[0].width;
          this._smallH = res[0].height;
          this._pr = sys.pixelRatio || 1;
          this._smallReady = true;
          this.draw();
        });
      });
    },
    detached() {
      if (this._fx) this._fx.dispose();
      this._loop = null;
      this._cv = null;
      this._ctx = null;
      this._fcv = null;
      this._fctx = null;
    }
  },

  observers: {
    'runs,live,tol,space'() {
      // 换题后若不再是三元（三维数据被撤），强制退回收敛曲线
      if (!this.properties.space && this.data.chartMode === 'space') {
        if (this._fx) this._fx.cancelKey('s3-inertia');
        this.setData({ chartMode: 'delta' });
        this._view = null;
      }
      // live 追加新点：启动/续命脉冲循环，新线段做一次描边生长
      const live = this.properties.live;
      const len = live && Array.isArray(live.deltas) ? live.deltas.length : 0;
      if (len > 0 && len !== this._lastLiveLen) {
        this._segGrowAt = Date.now();
        this._lastDataAt = this._segGrowAt;
        this.ensureLoop();
      }
      if (len === 0) this._lastLiveLen = 0;
      else this._lastLiveLen = len;
      if (this._smallReady) this.draw();
    }
  },

  methods: {
    // 视图切换：强制重新自适应
    modeDelta() {
      if (this.data.chartMode === 'delta') return;
      if (this._fx) this._fx.cancelKey('s3-inertia');
      this.setData({ chartMode: 'delta' });
      this._userMoved = false;
      this._view = null;
      this.draw();
    },

    modeComp() {
      if (this.data.chartMode === 'comp') return;
      if (this._fx) this._fx.cancelKey('s3-inertia');
      this.setData({ chartMode: 'comp' });
      this._userMoved = false;
      this._view = null;
      this.draw();
    },

    // 三维空间视图（仅三元方程组可用，WXML 中标签随 space 属性显隐）
    modeSpace() {
      if (this.data.chartMode === 'space' || !this.properties.space) return;
      if (!this._s3) this._s3 = { yaw: -0.7, pitch: 0.38, zoom: 1 };
      this._spaceShownAt = Date.now(); // 平面淡入计时
      this.setData({ chartMode: 'space' });
      this.ensureLoop(); // 淡入 500ms 内需要帧循环
      this.draw();
    },

    // ================= 全屏 =================

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
      this._geoFull = null;
      if (this._loop && this._cv) this._loop.setCanvas(this._cv);
      this.setData({ fullscreen: false }, () => this.draw());
    },

    // ================= 缩放/复位 =================

    zoomBy(e) {
      const f = Number(e.currentTarget.dataset.f);
      if (!(f > 0)) return;
      if (this.data.chartMode === 'space') {
        if (!this._s3) this._s3 = { yaw: -0.7, pitch: 0.38, zoom: 1 };
        this._s3.zoom = Math.max(0.35, Math.min(8, this._s3.zoom * f));
        this.draw();
        return;
      }
      this._userMoved = true;
      if (!this._view) this._view = this.buildView();
      const v = this._view;
      const center = { x: (v.xMin + v.xMax) / 2, y: (v.yMin + v.yMax) / 2 };
      this._view = clampView(zoomView(v, center, f));
      this.draw();
    },

    resetView() {
      if (this.data.chartMode === 'space') {
        if (this._fx) this._fx.cancelKey('s3-inertia');
        this._s3 = { yaw: -0.7, pitch: 0.38, zoom: 1 };
        this.draw();
        return;
      }
      this._userMoved = false;
      this._view = null;
      this.draw();
    },

    // ================= 触摸手势 =================

    onTS(e) {
      const mode = this._gest.anchor(e.touches);
      if (!mode) return;
      if (this.data.chartMode === 'space') {
        if (!this._s3) this._s3 = { yaw: -0.7, pitch: 0.38, zoom: 1 };
        if (this._fx) this._fx.cancelKey('s3-inertia');
        // 若惯性循环被取消，立刻补一个脉冲循环（轻点/停顿时光环不停）
        if (this._loopKey === 's3-inertia') {
          this._loop = null;
          this._loopKey = null;
          this.ensureLoop();
        }
        this._s30 = { ...this._s3 };
        this._dragSamples = [];
        return;
      }
      if (!this._view) this._view = this.buildView();
      this._view0 = { ...this._view };
    },

    onTM(e) {
      const g = this._gest.move(e.touches);
      if (!g) return;

      // 三维模式：单指拖动旋转视角，双指捏合缩放
      if (this.data.chartMode === 'space') {
        if (!this._s30) return;
        const s = this._s30;
        if (g.type === 'pan') {
          this._s3.yaw = s.yaw + g.dxPx * 0.011;
          this._s3.pitch = Math.max(-1.45, Math.min(1.45, s.pitch + g.dyPx * 0.011));
          // 最近拖动样本（累计位移），抬手时估速度做惯性
          (this._dragSamples = this._dragSamples || []).push({ t: Date.now(), dx: g.dxPx, dy: g.dyPx });
          if (this._dragSamples.length > 6) this._dragSamples.shift();
        } else {
          this._s3.yaw = s.yaw;
          this._s3.pitch = Math.max(-1.45, Math.min(1.45, s.pitch));
          this._s3.zoom = Math.max(0.35, Math.min(8, s.zoom * g.ratio));
        }
        this.draw();
        return;
      }

      if (!this._view0 || !this._view) return;
      const geo = e.currentTarget.id === 'fullPlot' ? this._geoFull : this._geoSmall;
      if (!geo) return;
      this._userMoved = true;
      if (g.type === 'pan') {
        const dx = -(g.dxPx / geo.pw) * (this._view0.xMax - this._view0.xMin);
        const dy = (g.dyPx / geo.ph) * (this._view0.yMax - this._view0.yMin);
        this._view = clampView(panView(this._view0, dx, dy));
      } else {
        const cx = this._view0.xMin + (g.cx - geo.mL) / geo.pw * (this._view0.xMax - this._view0.xMin);
        const cy = this._view0.yMax - (g.cy - geo.mT) / geo.ph * (this._view0.yMax - this._view0.yMin);
        this._view = clampView(zoomView(this._view0, { x: cx, y: cy }, g.ratio));
      }
      this.draw();
    },

    onTE(e) {
      if (e.touches && e.touches.length > 0) {
        const mode = this._gest.anchor(e.touches);
        if (!mode) return;
        if (this.data.chartMode === 'space') {
          this._s30 = { ...this._s3 };
          this._dragSamples = []; // 重新锚点后旧样本坐标系失效
        }
        else {
          if (!this._view) this._view = this.buildView();
          this._view0 = { ...this._view };
        }
        return;
      }
      const t = this._gest.end();
      const isFull = e.currentTarget.id === 'fullPlot';
      // 三维模式：抬手后带惯性滑行（轻点速度为 0，自然跳过）
      if (this.data.chartMode === 'space') this.kickS3Inertia();
      this._dragSamples = [];
      // 小图单击进全屏（双击不做动作：第一击已打开全屏，第二击落到全屏画布上）
      if (t === 'tap' && !isFull) {
        this.enterFs();
      } else if (t === 'doubletap' && isFull) {
        this.exitFs();
      }
    },

    // 三维抬手惯性：用最近拖动样本估角速度，指数衰减滑行到停。
    // 惯性期间由它接管唯一帧循环，停后交还给 ensureLoop（避免两套 rAF 重复重绘）。
    kickS3Inertia() {
      if (this._fx) {
        this._fx.cancelKey('s3-inertia');
        this._fx.cancelKey('pulse');
      }
      this._loop = null;
      this._loopKey = null;
      const samples = this._dragSamples || [];
      const lowVelocity = () => { this.ensureLoop(); };
      if (samples.length < 2 || !this._cv || !this._s3) { lowVelocity(); return; }
      const a = samples[0], z = samples[samples.length - 1];
      const dtms = Math.max(1, z.t - a.t);
      let vyaw = (z.dx - a.dx) / dtms * 0.011;   // rad/ms（与拖动同系数 0.011 rad/px）
      let vpitch = (z.dy - a.dy) / dtms * 0.011;
      const VMIN = 0.00012;
      if (Math.abs(vyaw) < VMIN && Math.abs(vpitch) < VMIN) { lowVelocity(); return; }
      // 等效每 16ms 衰减到 0.94
      const decayPerMs = Math.pow(0.94, 1 / 16);
      let last = Date.now();
      const h = startRaf(this._cv, (dt) => {
        if (this.data.chartMode !== 'space' || !this._s3) {
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
        if (np >= 1.45 || np <= -1.45) { // 俯仰触边：夹回并消去该方向速度
          this._s3.pitch = np >= 1.45 ? 1.45 : -1.45;
          vpitch = 0;
        } else {
          this._s3.pitch = np;
        }
        this._pulseT += d; // 滑行期间脉冲/光环继续走
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

    // ================= 数据归集 =================

    allRuns() {
      const arch = Array.isArray(this.properties.runs) ? this.properties.runs : [];
      return this.properties.live ? arch.concat([this.properties.live]) : arch;
    },

    // 计算自适应视图（delta 模式 y 空间是 log10 δ；comp 模式是普通线性值）
    buildView() {
      const mode = this.data.chartMode;
      const tol = this.properties.tol > 0 ? this.properties.tol : 1e-6;

      if (mode === 'comp') {
        const live = this.properties.live;
        const xs = live && Array.isArray(live.xs) ? live.xs : [];
        let lo = Infinity, hi = -Infinity, kmax = 0;
        xs.forEach((vec) => {
          vec.forEach((v) => {
            if (isFinite(v)) { if (v < lo) lo = v; if (v > hi) hi = v; }
          });
        });
        kmax = xs.length - 1;
        if (!isFinite(lo)) { lo = -1; hi = 1; }
        lo = Math.min(lo, 0); hi = Math.max(hi, 0);
        if (hi - lo < 1e-9) { hi += 1; lo -= 1; }
        const pad = (hi - lo) * 0.12;
        return { xMin: 0, xMax: Math.max(kmax, 5) + 1, yMin: lo - pad, yMax: hi + pad };
      }

      // delta 模式
      let yLo = Math.log10(tol) - 1.2;
      let yHi = Math.log10(tol) + 4;
      let kmax = 0;
      let any = false;
      this.allRuns().forEach((r) => {
        (r.deltas || []).forEach((d, i) => {
          if (isFinite(d) && d > 0) {
            const ly = Math.log10(d);
            if (!any) { yLo = ly; yHi = ly; any = true; }
            if (ly < yLo) yLo = ly;
            if (ly > yHi) yHi = ly;
          }
          if (i + 1 > kmax) kmax = i + 1;
        });
      });
      const tolY = Math.log10(tol);
      yLo = Math.min(yLo, tolY) - 0.4;
      yHi = Math.max(yHi, tolY + 2) + 0.4;
      return { xMin: 0, xMax: Math.max(kmax, 5) + 1, yMin: yLo, yMax: yHi };
    },

    // ================= 绘制 =================

    // 脉冲/生长帧循环：有新数据时启动，空闲超过 1.2s 且迭代已结束则自动停（省电）
    ensureLoop() {
      if (this._loop || !this._cv) return;
      this._loop = startRaf(this._cv, (dt) => {
        this._pulseT += (dt > 0 && dt < 100) ? dt : 16;
        const live = this.properties.live;
        const running = live && live.state === 'running';
        if (!running && Date.now() - this._lastDataAt > 1200) {
          this._loop = null;
          this._loopKey = null;
          this.draw();
          return false;
        }
        this.draw();
        return true;
      });
      this._loopKey = 'pulse';
      this._fx.add(this._loop, 'pulse');
    },

    draw() {
      // 全屏期间小画布被隐藏，跳过其重绘（S9：拖动/脉冲每帧只画一块画布）
      if (!this.data.fullscreen && this._smallReady && this._cv && this._ctx) {
        this._drawOn(this._cv, this._ctx, this._smallW, this._smallH, 'small');
      }
      if (this.data.fullscreen && this._fcv && this._fctx) {
        this._drawOn(this._fcv, this._fctx, this.data.fw, this.data.fh, 'full');
      }
    },

    _drawOn(canvas, ctx, w, h, which) {
      // backing store 仅在 css 尺寸变化时重设（每帧重赋 width 会清空状态且有开销）
      const pr = this._pr || 1;
      const bw = Math.round(w * pr), bh = Math.round(h * pr);
      if (canvas.width !== bw || canvas.height !== bh) {
        canvas.width = bw;
        canvas.height = bh;
      }
      const view0 = this._view;
      const auto = this.buildView();
      // 用户手动缩放后：只向外扩张包含新数据，不主动收回
      let view = view0;
      if (!view || !this._userMoved) {
        view = auto;
      } else {
        view = {
          xMin: Math.min(view.xMin, auto.xMin),
          xMax: Math.max(view.xMax, auto.xMax),
          yMin: Math.min(view.yMin, auto.yMin),
          yMax: Math.max(view.yMax, auto.yMax)
        };
      }
      this._view = view;

      ctx.setTransform(pr, 0, 0, pr, 0, 0);

      const mode = this.data.chartMode;
      const mL = 56, mR = 16, mT = 40, mB = 34;
      const pw = w - mL - mR;
      const ph = h - mT - mB;
      if (pw <= 0 || ph <= 0) return;
      const geo = { mL, mT, pw, ph };
      if (which === 'full') this._geoFull = geo; else this._geoSmall = geo;

      const X = (x) => mL + (x - view.xMin) / (view.xMax - view.xMin) * pw;
      const Y = (y) => mT + (view.yMax - y) / (view.yMax - view.yMin) * ph;

      // 背景
      ctx.fillStyle = '#10182a';
      ctx.fillRect(0, 0, w, h);

      // 三维空间视图：三平面 + 迭代轨迹（/解点），独立于二维 view 体系
      if (mode === 'space' && this.properties.space) {
        const s3 = this._s3 || { yaw: -0.7, pitch: 0.38, zoom: 1 };
        const smL = 10, smR = 10, smT = 12, smB = 12;
        const sgeo = { mL: smL, mT: smT, pw: w - smL - smR, ph: h - smT - smB };
        if (sgeo.pw > 0 && sgeo.ph > 0) {
          if (which === 'full') this._geoFull = sgeo; else this._geoSmall = sgeo;
          const live = this.properties.live;
          const planeAlpha = this._spaceShownAt
            ? Math.min(1, (Date.now() - this._spaceShownAt) / 500) : 1;
          const pathReveal = this._segGrowAt
            ? Math.min(1, (Date.now() - this._segGrowAt) / 320) : 1;
          drawScene3d({
            ctx,
            w, h,
            mL: sgeo.mL, mT: sgeo.mT, pw: sgeo.pw, ph: sgeo.ph,
            A: this.properties.space.A,
            b: this.properties.space.b,
            yaw: s3.yaw, pitch: s3.pitch, zoom: s3.zoom,
            path: live && Array.isArray(live.xs) ? live.xs : [],
            point: null,
            time: this._pulseT,
            planeAlpha,
            pathReveal
          });
        }
        return;
      }

      // 网格与坐标刻度
      ctx.font = '10px sans-serif';
      ctx.lineWidth = 1;
      const stepX = niceStep((view.xMax - view.xMin) / 6);
      ctx.textAlign = 'center';
      ctx.strokeStyle = 'rgba(255,255,255,0.08)';
      ctx.fillStyle = 'rgba(255,255,255,0.45)';
      for (let i = Math.ceil(view.xMin / stepX); i * stepX <= view.xMax; i++) {
        const px = X(i * stepX);
        if (px < mL - 2 || px > mL + pw + 2) continue;
        ctx.beginPath(); ctx.moveTo(px, mT); ctx.lineTo(px, mT + ph); ctx.stroke();
        ctx.fillText(fmtTick(i * stepX), px, mT + ph + 22);
      }

      ctx.textAlign = 'right';
      if (mode === 'delta') {
        // 纵轴 log10 刻度：每个整数次幂一条线
        for (let p = Math.floor(view.yMin); p <= Math.ceil(view.yMax); p++) {
          const py = Y(p);
          if (py < mT - 2 || py > mT + ph + 2) continue;
          ctx.strokeStyle = 'rgba(255,255,255,0.08)';
          ctx.beginPath(); ctx.moveTo(mL, py); ctx.lineTo(mL + pw, py); ctx.stroke();
          ctx.fillStyle = 'rgba(255,255,255,0.45)';
          ctx.fillText(fmtTick(Math.pow(10, p)), mL - 6, py + 3);
        }
      } else {
        const stepY = niceStep((view.yMax - view.yMin) / 4);
        for (let i = Math.ceil(view.yMin / stepY); i * stepY <= view.yMax; i++) {
          const py = Y(i * stepY);
          if (py < mT - 2 || py > mT + ph + 2) continue;
          ctx.strokeStyle = 'rgba(255,255,255,0.08)';
          ctx.beginPath(); ctx.moveTo(mL, py); ctx.lineTo(mL + pw, py); ctx.stroke();
          ctx.fillStyle = 'rgba(255,255,255,0.45)';
          ctx.fillText(fmtTick(i * stepY), mL - 6, py + 3);
        }
      }

      // y=0 / 容差线
      if (mode === 'comp' && view.yMin <= 0 && view.yMax >= 0) {
        ctx.strokeStyle = 'rgba(255,255,255,0.4)';
        ctx.beginPath(); ctx.moveTo(mL, Y(0)); ctx.lineTo(mL + pw, Y(0)); ctx.stroke();
      }
      if (mode === 'delta') {
        const tolY = Math.log10(this.properties.tol > 0 ? this.properties.tol : 1e-6);
        if (tolY >= view.yMin && tolY <= view.yMax) {
          ctx.strokeStyle = '#2ecc71';
          ctx.setLineDash([6, 4]);
          ctx.beginPath(); ctx.moveTo(mL, Y(tolY)); ctx.lineTo(mL + pw, Y(tolY)); ctx.stroke();
          ctx.setLineDash([]);
          ctx.textAlign = 'left';
          ctx.fillStyle = '#2ecc71';
          ctx.fillText('容差 ' + fmtTick(this.properties.tol), mL + 6, Y(tolY) - 5);
        }
      }

      // 轴标题
      ctx.textAlign = 'left';
      ctx.fillStyle = 'rgba(255,255,255,0.55)';
      ctx.fillText(mode === 'delta' ? 'δ=max|Δx|（对数）' : 'xᵢ', 8, 14);
      ctx.textAlign = 'right';
      ctx.fillText('迭代步数 k', mL + pw, mT + ph + 22);

      const runs = this.allRuns();
      const hasData = runs.some((r) => (r.deltas || []).length > 0)
        || (mode === 'comp' && this.properties.live && (this.properties.live.xs || []).length > 1);

      if (mode === 'delta') {
        this._drawDeltaCurves(ctx, X, Y, runs, geo);
      } else {
        this._drawCompCurves(ctx, X, Y, geo);
      }

      if (!hasData) {
        ctx.textAlign = 'center';
        ctx.font = '13px sans-serif';
        ctx.fillStyle = 'rgba(255,255,255,0.5)';
        ctx.fillText(
          mode === 'delta' ? '开始迭代后此处显示收敛曲线' : '开始迭代后此处显示各分量轨迹',
          mL + pw / 2, mT + ph / 2
        );
      }
    },

    // δ(k) 折线：历史运行半透明，本次运行高亮并在末端画脉冲点/收敛点
    _drawDeltaCurves(ctx, X, Y, runs, geo) {
      const arch = Array.isArray(this.properties.runs) ? this.properties.runs : [];
      runs.forEach((r, idx) => {
        const deltas = r.deltas || [];
        if (!deltas.length) return;
        const isLive = this.properties.live && r === this.properties.live;
        ctx.strokeStyle = isLive ? LIVE_COLOR : (r.color || ARCHIVE_COLORS[idx % ARCHIVE_COLORS.length]);
        ctx.globalAlpha = isLive ? 1 : 0.75;
        ctx.lineWidth = isLive ? 2.4 : 1.6;

        // 本次运行最新线段做 260ms 描边生长（末端点从上一点滑入）
        let growU = 1;
        if (isLive && this._segGrowAt && deltas.length >= 2) {
          growU = Math.max(0, Math.min(1, (Date.now() - this._segGrowAt) / 260));
        }
        const pts = [];
        deltas.forEach((d, i) => {
          if (!isFinite(d) || d <= 0) return;
          pts.push({ px: X(i + 1), py: Y(Math.log10(d)) });
        });
        if (isLive && growU < 1 && pts.length >= 2) {
          const p0 = pts[pts.length - 2], p1 = pts[pts.length - 1];
          pts[pts.length - 1] = { px: p0.px + (p1.px - p0.px) * growU, py: p0.py + (p1.py - p0.py) * growU };
        }

        ctx.beginPath();
        pts.forEach((p, i) => { if (i === 0) ctx.moveTo(p.px, p.py); else ctx.lineTo(p.px, p.py); });
        ctx.stroke();

        // 末端点：迭代中蓝色脉冲；收敛绿心白环；发散红；达步数上限琥珀
        const lastP = pts[pts.length - 1];
        if (lastP) {
          if (isLive) {
            const state = this.properties.live.state || 'running';
            if (state === 'converged') {
              ctx.fillStyle = '#2ecc71';
              ctx.beginPath(); ctx.arc(lastP.px, lastP.py, 4.5, 0, Math.PI * 2); ctx.fill();
              ctx.strokeStyle = '#fff';
              ctx.lineWidth = 1.4;
              ctx.beginPath(); ctx.arc(lastP.px, lastP.py, 6.5, 0, Math.PI * 2); ctx.stroke();
            } else {
              if (state === 'diverged') ctx.fillStyle = '#e74c3c';
              else if (state === 'maxiter') ctx.fillStyle = '#f39c12';
              else ctx.fillStyle = LIVE_COLOR;
              const pulseR = state === 'running' ? 4 + 1.5 * Math.sin(this._pulseT / 180) : 4;
              if (state === 'running') {
                // 向外扩散的衰减环
                const phase = (this._pulseT % 1100) / 1100;
                ctx.strokeStyle = LIVE_COLOR;
                ctx.globalAlpha = 0.45 * (1 - phase);
                ctx.lineWidth = 1.4;
                ctx.beginPath(); ctx.arc(lastP.px, lastP.py, 4 + 7 * phase, 0, Math.PI * 2); ctx.stroke();
                ctx.globalAlpha = 1;
              }
              ctx.beginPath(); ctx.arc(lastP.px, lastP.py, pulseR, 0, Math.PI * 2); ctx.fill();
            }
          } else {
            ctx.fillStyle = ctx.strokeStyle;
            ctx.beginPath(); ctx.arc(lastP.px, lastP.py, 3, 0, Math.PI * 2); ctx.fill();
          }
        }
        ctx.globalAlpha = 1;
      });

      // 图例（右上，最多 7 条；本次固定置顶蓝色）
      const legend = [];
      arch.slice(-6).forEach((r, i) => legend.push({ color: r.color || ARCHIVE_COLORS[i % ARCHIVE_COLORS.length], text: r.label || `历史 ${i + 1}` }));
      if (this.properties.live && (this.properties.live.deltas || []).length > 0) {
        legend.push({ color: LIVE_COLOR, text: this.properties.live.label || '本次' });
      }
      ctx.font = '11px sans-serif';
      ctx.textAlign = 'right';
      let ly = 14;
      legend.forEach((it) => {
        ctx.fillStyle = 'rgba(16,24,42,0.6)';
        ctx.fillRect(geo.mL + geo.pw - 150, ly - 10, 150, 15);
        ctx.strokeStyle = it.color;
        ctx.lineWidth = 2;
        ctx.beginPath();
        ctx.moveTo(geo.mL + geo.pw - 144, ly - 3);
        ctx.lineTo(geo.mL + geo.pw - 118, ly - 3);
        ctx.stroke();
        ctx.fillStyle = 'rgba(255,255,255,0.9)';
        ctx.fillText(it.text, geo.mL + geo.pw - 6, ly);
        ly += 16;
      });
    },

    // xᵢ(k) 轨迹：只画本次运行的 n 个分量（k=0 取初值）
    _drawCompCurves(ctx, X, Y, geo) {
      const live = this.properties.live;
      const xs = live && Array.isArray(live.xs) ? live.xs : [];
      if (xs.length < 2) return;
      const n = xs[0].length;
      for (let j = 0; j < n; j++) {
        ctx.strokeStyle = COMP_COLORS[j % COMP_COLORS.length];
        ctx.lineWidth = j === 0 ? 2.2 : 1.8;
        ctx.beginPath();
        let started = false;
        xs.forEach((vec, k) => {
          const v = vec[j];
          if (!isFinite(v)) { started = false; return; }
          const px = X(k);
          const py = Y(v);
          if (!started) { ctx.moveTo(px, py); started = true; }
          else ctx.lineTo(px, py);
        });
        ctx.stroke();
      }
      // 末端点 + 图例
      const last = xs[xs.length - 1];
      ctx.font = '11px sans-serif';
      ctx.textAlign = 'right';
      let ly = 14;
      for (let j = 0; j < n; j++) {
        if (!isFinite(last[j])) continue;
        const c = COMP_COLORS[j % COMP_COLORS.length];
        ctx.fillStyle = c;
        ctx.beginPath();
        ctx.arc(X(xs.length - 1), Y(last[j]), 3, 0, Math.PI * 2);
        ctx.fill();
        ctx.fillStyle = 'rgba(16,24,42,0.6)';
        ctx.fillRect(geo.mL + geo.pw - 96, ly - 10, 96, 15);
        ctx.strokeStyle = c;
        ctx.lineWidth = 2;
        ctx.beginPath();
        ctx.moveTo(geo.mL + geo.pw - 90, ly - 3);
        ctx.lineTo(geo.mL + geo.pw - 70, ly - 3);
        ctx.stroke();
        ctx.fillStyle = 'rgba(255,255,255,0.9)';
        ctx.fillText(`x${sub(j + 1)}`, geo.mL + geo.pw - 6, ly);
        ly += 16;
      }
    }
  }
});
