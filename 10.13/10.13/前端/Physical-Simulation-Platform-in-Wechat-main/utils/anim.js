// utils/anim.js - Canvas 动效底座（缓动 + 帧驱动 + 分拍播放器）
// 小程序没有全局 requestAnimationFrame；Canvas 2D 节点上有 canvas.requestAnimationFrame，
// 拿不到节点时降级 setTimeout。所有时间一律用 Date.now() 差值计算（真机切后台 rAF
// 会暂停，恢复后不能按帧数累计进度）。页面 onUnload / 组件 detached 必须 cancel 句柄。

export const easeLinear = (t) => t;
export const easeOutCubic = (t) => 1 - Math.pow(1 - t, 3);
export const easeInOutCubic = (t) =>
  t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2;
export const easeOutBack = (t) => {
  const c1 = 1.70158;
  const c3 = c1 + 1;
  return 1 + c3 * Math.pow(t - 1, 3) + c1 * Math.pow(t - 1, 2);
};

export const lerp = (a, b, t) => a + (b - a) * t;

// #rrggbb 颜色插值（动效只在项目既有色板之间过渡，不处理 rgba() 入参）
export function mixColor(c1, c2, t) {
  const p = (c) => [parseInt(c.slice(1, 3), 16), parseInt(c.slice(3, 5), 16), parseInt(c.slice(5, 7), 16)];
  const a = p(c1), b = p(c2);
  const r = Math.round(lerp(a[0], b[0], t));
  const g = Math.round(lerp(a[1], b[1], t));
  const gn = Math.round(lerp(a[2], b[2], t));
  return `rgb(${r},${g},${gn})`;
}

function schedule(cb, canvas) {
  if (canvas && typeof canvas.requestAnimationFrame === 'function') {
    const id = canvas.requestAnimationFrame(() => cb(Date.now()));
    return {
      cancel: () => {
        if (typeof canvas.cancelAnimationFrame === 'function') canvas.cancelAnimationFrame(id);
      }
    };
  }
  const id = setTimeout(() => cb(Date.now()), 1000 / 60);
  return { cancel: () => clearTimeout(id) };
}

// 单次补间：onUpdate(easedT, rawT) 每帧回调，onDone 在 t=1 时回调一次。
// 返回句柄 { cancel() }；canvas 缺省时走 setTimeout（组件/页面可随时切换画布节点）。
export function tween(opts) {
  const duration = opts.duration > 0 ? opts.duration : 350;
  const ease = opts.ease || easeOutCubic;
  let start = 0;
  let cancelled = false;
  let job = null;
  const handle = {
    cancel() {
      cancelled = true;
      if (job) job.cancel();
    }
  };
  const tick = (now) => {
    if (cancelled) return;
    if (!start) start = now;
    const t = Math.max(0, Math.min(1, (now - start) / duration));
    opts.onUpdate && opts.onUpdate(ease(t), t);
    if (t < 1) {
      job = schedule(tick, opts.canvas);
    } else {
      opts.onDone && opts.onDone();
    }
  };
  job = schedule(tick, opts.canvas);
  return handle;
}

// 连续帧循环（末端点脉冲、3D 惯性等）：frame(dtMs, elapsedMs) 返回 false 即停止。
// 返回 { cancel() }；canvas 可在 setCanvas() 中热替换（小画布↔全屏切换）。
export function startRaf(canvas, frame) {
  let cancelled = false;
  let job = null;
  let last = 0;
  let begin = 0;
  const handle = {
    cancel() {
      cancelled = true;
      if (job) job.cancel();
    },
    setCanvas(cv) {
      canvas = cv;
    }
  };
  const tick = (now) => {
    if (cancelled) return;
    if (!begin) { begin = now; last = now; }
    const dt = now - last;
    last = now;
    const cont = frame(dt, now - begin);
    if (cont !== false && !cancelled) job = schedule(tick, canvas);
  };
  job = schedule(tick, canvas);
  return handle;
}

// 分拍播放器：把「自动演示」的同步 while 循环拆成一拍拍可见的步骤。
// opts: {
//   isDone()  返回 true 立即结束；
//   step()    执行一拍（返回 false 表示无法继续，提前结束）；
//   interval  每拍毫秒（默认 420）；
//   onEnd()   自然结束或取消时回调，参数 reason='done'|'blocked'|'cancel'；
//   maxSteps  硬上限防失控（默认 200）
// }
// 返回 { cancel, pause, resume, get paused, setInterval }
export function createStepPlayer(opts) {
  const interval0 = opts.interval > 0 ? opts.interval : 420;
  let timer = null;
  let cancelled = false;
  let paused = false;
  let count = 0;
  const maxSteps = opts.maxSteps || 200;

  function finish(reason) {
    if (timer) clearTimeout(timer);
    timer = null;
    cancelled = true;
    opts.onEnd && opts.onEnd(reason);
  }

  function tick() {
    if (cancelled || paused) return;
    if (opts.isDone && opts.isDone()) { finish('done'); return; }
    if (count >= maxSteps) { finish('done'); return; }
    const cont = opts.step();
    count++;
    if (cont === false) { finish('blocked'); return; }
    if (opts.isDone && opts.isDone()) { finish('done'); return; }
    timer = setTimeout(tick, opts.interval > 0 ? opts.interval : interval0);
  }

  timer = setTimeout(tick, interval0 * 0.6); // 首拍略快，点击后立刻有反馈

  return {
    cancel() { if (!cancelled) finish('cancel'); },
    pause() {
      paused = true;
      if (timer) { clearTimeout(timer); timer = null; }
    },
    resume() {
      if (!paused || cancelled) return;
      paused = false;
      if (!timer) timer = setTimeout(tick, interval0 * 0.4);
    },
    get paused() { return paused; },
    setInterval(ms) {
      opts.interval = ms > 0 ? ms : interval0;
    }
  };
}

// 页面/组件统一保管动效句柄：add 登记的句柄在 dispose() 时全部 cancel。
// 用法：onLoad 里 this._fx = createFxBucket()；所有 tween/raf/player 经 this._fx.add(...)；
// onUnload/detached 里 this._fx.dispose()。同 key 重复 add 会先取消旧句柄。
export function createFxBucket() {
  const list = [];
  const byKey = new Map();
  return {
    add(handle, key) {
      if (!handle || typeof handle.cancel !== 'function') return handle;
      list.push(handle);
      if (key) {
        if (byKey.has(key)) byKey.get(key).cancel();
        byKey.set(key, handle);
      }
      return handle;
    },
    cancelKey(key) {
      const h = byKey.get(key);
      if (h) { h.cancel(); byKey.delete(key); }
    },
    dispose() {
      list.forEach(h => { try { h.cancel(); } catch (e) { /* 节点已销毁 */ } });
      list.length = 0;
      byKey.clear();
    }
  };
}
