// utils/graphview.js - 函数图像视图与手势工具（二分法/牛顿迭代实验页共用）
// 页面持 _view（数据坐标视野）状态，手势回调里换算后重绘；
// 全屏查看与小画布共用同一视野，缩放/平移对两者同时生效。

// 计算初始视图：x 范围 [xMin,xMax] 外扩 padRatio，y 从函数采样自适应并包含 0
export function computeInitialView(f, xMin, xMax, padRatio) {
  const pad = padRatio === undefined ? 0.15 : padRatio;
  const span = Math.max(xMax - xMin, 1e-6);
  const vxMin = xMin - span * pad;
  const vxMax = xMax + span * pad;
  let yMin = 0;
  let yMax = 0;
  const N = 200;
  for (let i = 0; i <= N; i++) {
    const x = vxMin + (vxMax - vxMin) * i / N;
    const y = f(x);
    if (isFinite(y)) {
      if (y < yMin) yMin = y;
      if (y > yMax) yMax = y;
    }
  }
  const yPad = Math.max(yMax - yMin, 1e-6) * 0.12;
  return { xMin: vxMin, xMax: vxMax, yMin: yMin - yPad, yMax: yMax + yPad };
}

// 平移视图（dx/dy 为数据坐标增量）
export function panView(view, dx, dy) {
  return {
    xMin: view.xMin + dx,
    xMax: view.xMax + dx,
    yMin: view.yMin + dy,
    yMax: view.yMax + dy
  };
}

// 缩放视图：scale>1 放大（视野变窄），以 center（数据坐标）为不动点
export function zoomView(view, center, scale) {
  if (!(scale > 0) || !isFinite(scale)) return view;
  const inv = 1 / scale;
  return {
    xMin: center.x - (center.x - view.xMin) * inv,
    xMax: center.x + (view.xMax - center.x) * inv,
    yMin: center.y - (center.y - view.yMin) * inv,
    yMax: center.y + (view.yMax - center.y) * inv
  };
}

// 视野保护：跨度限制在 [1e-9, 1e6]，防止缩放到退化或失控
export function clampView(view) {
  const MIN_SPAN = 1e-9;
  const MAX_SPAN = 1e6;
  function fix(lo, hi) {
    if (!isFinite(lo) || !isFinite(hi)) return [0, 1]; // 非法视野兜底
    let span = hi - lo;
    if (!(span > 0)) span = MIN_SPAN;
    if (span < MIN_SPAN) {
      const mid = (lo + hi) / 2;
      return [mid - MIN_SPAN / 2, mid + MIN_SPAN / 2];
    }
    if (span > MAX_SPAN) {
      const mid = (lo + hi) / 2;
      return [mid - MAX_SPAN / 2, mid + MAX_SPAN / 2];
    }
    return [lo, hi];
  }
  const x = fix(view.xMin, view.xMax);
  const y = fix(view.yMin, view.yMax);
  return { xMin: x[0], xMax: x[1], yMin: y[0], yMax: y[1] };
}

// 像素 -> 数据坐标（geo 含绘图区几何：mL/mT 边距 + pw/ph 绘图区尺寸）
export function pxToData(view, geo, px, py) {
  return {
    x: view.xMin + (px - geo.mL) / geo.pw * (view.xMax - view.xMin),
    y: view.yMax - (py - geo.mT) / geo.ph * (view.yMax - view.yMin)
  };
}

// 坐标轴刻度步长（1-2-5 系列）
export function niceStep(raw) {
  if (!(raw > 0) || !isFinite(raw)) return 1;
  const exp = Math.floor(Math.log10(raw));
  const f = raw / Math.pow(10, exp);
  let nf;
  if (f < 1.5) nf = 1;
  else if (f < 3.5) nf = 2;
  else if (f < 7.5) nf = 5;
  else nf = 10;
  return nf * Math.pow(10, exp);
}

// 刻度标签：常规量 4 位有效数字，过大/过小用指数记法
export function fmtTick(v) {
  if (v === 0) return '0';
  const a = Math.abs(v);
  if (a >= 1e-4 && a < 1e5) return String(Number(v.toPrecision(4)));
  return v.toExponential(0);
}

// 触摸手势追踪：单指拖动 / 双指捏合 / 单击 / 双击。
// touches 为 canvas 触摸事件的 e.touches（canvas 相对坐标 x/y，缺省回退 pageX/pageY）。
export function createGestureTracker() {
  let mode = null; // 'pan' | 'pinch'
  let anchor = null;
  let lastTap = 0;
  let multiEver = false; // 本轮手势中出现过双指（出现过就不判定为 tap）

  function touchPos(t) {
    return {
      x: t.x !== undefined ? t.x : t.pageX,
      y: t.y !== undefined ? t.y : t.pageY
    };
  }

  return {
    // 触摸点数变化（touchstart / touchend 后对剩余触点）时调用，返回当前模式
    anchor(touches) {
      if (touches.length === 1) {
        mode = 'pan';
        const p = touchPos(touches[0]);
        anchor = { x: p.x, y: p.y, time: Date.now(), multi: false, moved: false };
      } else if (touches.length >= 2) {
        mode = 'pinch';
        multiEver = true;
        const p0 = touchPos(touches[0]);
        const p1 = touchPos(touches[1]);
        const dx = p1.x - p0.x;
        const dy = p1.y - p0.y;
        anchor = {
          dist: Math.sqrt(dx * dx + dy * dy) || 1,
          cx: (p0.x + p1.x) / 2,
          cy: (p0.y + p1.y) / 2,
          time: Date.now(),
          multi: true,
          moved: false
        };
      } else {
        mode = null;
        anchor = null;
      }
      return mode;
    },

    // touchmove 时调用：返回 {type:'pan',dxPx,dyPx} 或 {type:'pinch',ratio,cx,cy}，无手势返回 null
    move(touches) {
      if (!anchor || !mode) return null;
      if (mode === 'pan' && touches.length === 1) {
        const p = touchPos(touches[0]);
        const dxPx = p.x - anchor.x;
        const dyPx = p.y - anchor.y;
        if (Math.abs(dxPx) > 6 || Math.abs(dyPx) > 6) anchor.moved = true;
        return { type: 'pan', dxPx, dyPx };
      }
      if (mode === 'pinch' && touches.length >= 2) {
        const p0 = touchPos(touches[0]);
        const p1 = touchPos(touches[1]);
        const dx = p1.x - p0.x;
        const dy = p1.y - p0.y;
        const dist = Math.sqrt(dx * dx + dy * dy) || 1;
        anchor.moved = true;
        return { type: 'pinch', ratio: dist / anchor.dist, cx: anchor.cx, cy: anchor.cy };
      }
      return null;
    },

    // 全部手指离开后调用：判定 'tap' / 'doubletap' / null
    end() {
      const a = anchor;
      const wasMulti = multiEver;
      mode = null;
      anchor = null;
      multiEver = false;
      if (!a || a.multi || a.moved || wasMulti) return null;
      if (Date.now() - a.time > 350) return null;
      const now = Date.now();
      const isDouble = now - lastTap < 300;
      lastTap = now;
      return isDouble ? 'doubletap' : 'tap';
    }
  };
}
