package service

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// ========== 评分框架（数值分析批次起改用注册表模式）==========
//
// 新增实验四件套：实现 scoringFunc -> scorers 注册一行 -> migration 写
// experiments/levels 数据 -> 契约附录补 readings/user_config 格式。
// 评分真值一律由后端按 target/user_config 参数现算，不在库里手填数字。

// scoringFunc 单个实验的评分器（readings 已通过非空数组校验）。
type scoringFunc func(readings, target, userConfig []byte) (score int, passed bool, detail string, err error)

// scorers 实验 code -> 评分器。未注册的 code 在 scoreByExperiment 拦截。
var scorers = map[string]scoringFunc{
	"bisection":   scoreBisection,
	"newton_iter": scoreNewtonIter,
	"gauss_elim":  scoreGaussElim,
	"jacobi_iter": scoreJacobiIter,
	"sor_iter":    scoreSorIter,

	"lagrange_interp":     scoreLagrangeInterp,
	"simpson_integration": scoreSimpsonIntegration,
	"aitken_interp":       scoreAitkenInterp,
	"romberg_integration": scoreRombergIntegration,

	"euler_explicit": scoreEulerExplicit,
	"euler_improved": scoreEulerImproved,
	"euler_implicit": scoreEulerImplicit,

	"runge_kutta_ode": scoreRungeKuttaODE,

	"adams_pc_ode": scoreAdamsPCODE,

	"least_squares_fit": scoreLeastSquaresFit,

	"cubic_spline": scoreCubicSpline,

	"lu_decomp": scoreLUDecomp,

	"thomas_solve": scoreThomasSolve,

	"cholesky_solve": scoreCholeskySolve,
}

// scoreByExperiment 按实验 code 解析读数与目标并评分。
// 契约 §12：readings 非法 -> 400。空数组或非 JSON 数组视为非法，评分前拦截。
func scoreByExperiment(code string, readingsRaw, targetRaw, userConfigRaw []byte) (score int, passed bool, detail string, err error) {
	var probe []json.RawMessage
	if uerr := json.Unmarshal(readingsRaw, &probe); uerr != nil {
		err = fmt.Errorf("readings 不是合法数组: %w", uerr)
		return
	}
	if len(probe) == 0 {
		err = fmt.Errorf("readings 不能为空")
		return
	}

	fn, ok := scorers[code]
	if !ok {
		err = fmt.Errorf("unknown experiment: %s", code)
		return
	}
	return fn(readingsRaw, targetRaw, userConfigRaw)
}

// ========== 通用打分 ==========

// relErrBand 相对误差分段拐点：relErr <= maxErr 时得分不低于 score。
type relErrBand struct {
	maxErr float64
	score  int
}

// relErrBands 数值分析批次共用打分曲线（百分制），拐点间按 log10 线性插值：
// 收敛到位（<=1e-6）满分；达到题目精度量级（1e-4~1e-2）65~90；
// 明显偏差（~0.1）只剩 30；偏差超过 50% 为 0。
var relErrBands = []relErrBand{
	{1e-6, 100},
	{1e-4, 90},
	{1e-3, 80},
	{1e-2, 65},
	{1e-1, 30},
	{5e-1, 0},
}

// scoreFromRelErr 相对误差 -> 百分制得分，用默认数值分析曲线。
func scoreFromRelErr(relErr float64) int {
	return scoreFromRelErrBands(relErr, relErrBands)
}

// scoreFromRelErrBands 按给定分段曲线打分（NaN/负值按 0 处理；bands 为空用默认曲线）。
// bands 须按 maxErr 升序；首档及以下恒 100 分，末档以上恒 0 分，拐点间 log10 线性插值。
func scoreFromRelErrBands(relErr float64, bands []relErrBand) int {
	if len(bands) == 0 {
		bands = relErrBands
	}
	if !(relErr >= 0) {
		return 0
	}
	if relErr <= bands[0].maxErr {
		return 100
	}
	if relErr > bands[len(bands)-1].maxErr {
		return 0
	}
	lo := bands[0]
	for i := 1; i < len(bands); i++ {
		hi := bands[i]
		if relErr <= hi.maxErr {
			t := (math.Log10(relErr) - math.Log10(lo.maxErr)) / (math.Log10(hi.maxErr) - math.Log10(lo.maxErr))
			return int(math.Round(float64(lo.score) + t*float64(hi.score-lo.score)))
		}
		lo = hi
	}
	return 0
}

// parseRelErrBands 解析 target.bands：[[relErr, score], ...]（relErr 严格升序、score 0~100）。
// 缺省（无该字段）返回 nil，评分回退默认数值分析曲线；格式非法报错，尽早暴露数据配置错误。
// 物理实验读数精度（如牛顿环容差 0.02mm ≈ 0.5%~2% 相对误差）与数值分析收敛精度
// 量级不同，故曲线随实验数据下发，数据组可不改代码调参。
func parseRelErrBands(raw []byte) ([]relErrBand, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var arr [][2]float64
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, fmt.Errorf("bands 格式非法: %w", err)
	}
	if len(arr) == 0 {
		return nil, nil
	}
	bands := make([]relErrBand, 0, len(arr))
	for i, p := range arr {
		if !(p[0] > 0) || p[1] < 0 || p[1] > 100 {
			return nil, fmt.Errorf("bands[%d] 非法: [%.6g, %.6g]", i, p[0], p[1])
		}
		if i > 0 && p[0] <= arr[i-1][0] {
			return nil, fmt.Errorf("bands[%d] 的 relErr 须严格递增", i)
		}
		bands = append(bands, relErrBand{maxErr: p[0], score: int(math.Round(p[1]))})
	}
	return bands, nil
}

// relError 相对误差 |got-want|/|want|；want≈0 时退化为绝对误差，防 0/0。
func relError(got, want float64) float64 {
	d := math.Abs(want)
	if d < 1e-12 {
		d = 1e-12
	}
	return math.Abs(got-want) / d
}

// passedByScore 契约约定：target 缺 pass_score（<=0）时恒过关，否则 score>=pass_score。
func passedByScore(score, passScore int) bool {
	if passScore <= 0 {
		return true
	}
	return score >= passScore
}

// isFinite 有限数（非 NaN/Inf）。
func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// validInterval 区间须为两个有限数且 a < b。
func validInterval(iv []float64) bool {
	return len(iv) == 2 && isFinite(iv[0]) && isFinite(iv[1]) && iv[0] < iv[1]
}

// ========== 数值分析函数库（后端持真值）==========
//
// func_key -> {f, f'}，与 migration config.func 的展示表达式一一对应。
// 前端仅用 config 画图/出题；评分一律以这里的定义现算为准。

// numFuncDef 被测函数及其解析导数（牛顿迭代用）。
type numFuncDef struct {
	f  func(x float64) float64
	df func(x float64) float64
}

var numFuncs = map[string]numFuncDef{
	"cubic_1": { // x^3-x-1，唯一实根 ≈ 1.324718（L1/L2 共用，便于对比两法收敛速度）
		f:  func(x float64) float64 { return x*x*x - x - 1 },
		df: func(x float64) float64 { return 3*x*x - 1 },
	},
	"cos_x_x": { // cos x-x，唯一根 ≈ 0.739085
		f:  func(x float64) float64 { return math.Cos(x) - x },
		df: func(x float64) float64 { return -math.Sin(x) - 1 },
	},
	"sqrt_2": { // x^2-2，双根 ±√2（牛顿初值正负决定收敛到哪个根，教学用）
		f:  func(x float64) float64 { return x*x - 2 },
		df: func(x float64) float64 { return 2 * x },
	},
}

// bisectRoot 二分法参考解：对分至双精度极限，作为评分真值。
// 区间不夹根（含端点无定义）时报错，视为出题数据配置错误。
func bisectRoot(f func(x float64) float64, a, b float64) (float64, error) {
	fa, fb := f(a), f(b)
	if math.IsNaN(fa) || math.IsNaN(fb) {
		return 0, fmt.Errorf("函数在区间端点无定义")
	}
	if fa == 0 {
		return a, nil
	}
	if fb == 0 {
		return b, nil
	}
	if fa*fb > 0 {
		return 0, fmt.Errorf("区间 [%.6g, %.6g] 不夹根", a, b)
	}
	for i := 0; i < 200; i++ {
		m := (a + b) / 2
		fm := f(m)
		if fm == 0 || (b-a)/2 < 1e-15 {
			return m, nil
		}
		if fa*fm < 0 {
			b = m
		} else {
			a, fa = m, fm
		}
	}
	return (a + b) / 2, nil
}

// ========== L1 二分法求根 ==========

// scoreBisection 二分法求根评分。
//
// target:     {"method":"root_rel_err","func_key":"cubic_1","interval":[1.0,1.5],"pass_score":60}
// readings:   [{"root":1.324718,"iterations":19}]（iterations 可选，仅记入审计 detail）
// user_config 可覆盖 func_key / interval（前端换题或换区间时后端按新参数重算真值根）。
func scoreBisection(readings, target, userConfig []byte) (score int, passed bool, detail string, err error) {
	var tgt struct {
		FuncKey   string    `json:"func_key"`
		Interval  []float64 `json:"interval"`
		PassScore int       `json:"pass_score"`
	}
	if err = json.Unmarshal(target, &tgt); err != nil {
		err = fmt.Errorf("target 解析失败: %w", err)
		return
	}
	// v0.5 模式：user_config 覆盖同名字段
	var uc struct {
		FuncKey  string    `json:"func_key"`
		Interval []float64 `json:"interval"`
	}
	_ = json.Unmarshal(userConfig, &uc) // 可选字段，缺省/为空时忽略
	if uc.FuncKey != "" {
		tgt.FuncKey = uc.FuncKey
	}
	if len(uc.Interval) == 2 {
		tgt.Interval = uc.Interval
	}

	fd, ok := numFuncs[tgt.FuncKey]
	if !ok {
		err = fmt.Errorf("未知函数: %s", tgt.FuncKey)
		return
	}
	if !validInterval(tgt.Interval) {
		err = fmt.Errorf("区间非法: %v", tgt.Interval)
		return
	}
	trueRoot, berr := bisectRoot(fd.f, tgt.Interval[0], tgt.Interval[1])
	if berr != nil {
		err = berr
		return
	}

	var rs []struct {
		Root       *float64 `json:"root"`
		Iterations *int     `json:"iterations"`
	}
	if err = json.Unmarshal(readings, &rs); err != nil {
		err = fmt.Errorf("readings 解析失败: %w", err)
		return
	}
	r0 := rs[0]
	if r0.Root == nil {
		err = fmt.Errorf("readings[0] 缺少 root 字段")
		return
	}
	root := *r0.Root
	if !isFinite(root) {
		err = fmt.Errorf("root 不能是 NaN/Inf")
		return
	}

	relErr := relError(root, trueRoot)
	score = scoreFromRelErr(relErr)
	passed = passedByScore(score, tgt.PassScore)
	detail = fmt.Sprintf("root=%.10g rel_err=%.3g true_root=%.10g", root, relErr, trueRoot)
	if r0.Iterations != nil {
		detail += fmt.Sprintf(" iterations=%d", *r0.Iterations)
	}
	return
}

// ========== L2 牛顿迭代法求根 ==========

// newtonSolve 牛顿迭代参考解：x_{n+1} = x_n - f(x_n)/f'(x_n)，
// 停止准则 |x_{n+1}-x_n| < tol，返回根与迭代次数（产生满足准则的近似根的步数）。
// f'=0、函数无定义、越出有限域或 maxIter 内未收敛均报错（调用方决定回退策略）。
func newtonSolve(f, df func(x float64) float64, x0, tol float64, maxIter int) (float64, int, error) {
	x := x0
	for i := 1; i <= maxIter; i++ {
		d := df(x)
		if !isFinite(d) || d == 0 {
			return 0, 0, fmt.Errorf("f'(%.6g)=0 或无定义，迭代中断", x)
		}
		fn := f(x)
		if !isFinite(fn) {
			return 0, 0, fmt.Errorf("f(%.6g) 无定义，迭代中断", x)
		}
		xn := x - fn/d
		if !isFinite(xn) {
			return 0, 0, fmt.Errorf("迭代发散")
		}
		if math.Abs(xn-x) < tol {
			return xn, i, nil
		}
		x = xn
	}
	return 0, 0, fmt.Errorf("%d 次内未收敛", maxIter)
}

// scoreNewtonIter 牛顿迭代法求根评分。
//
// target:     {"method":"newton_rel_err","func_key":"cubic_1","interval":[1.0,1.5],
//
//	"x0":1.5,"tolerance":1e-6,"max_iterations":50,"pass_score":60}
//
// readings:   [{"root":1.324718,"iterations":5,"x0":1.5}]
//
//	root 必填；iterations/x0 可选（iterations 缺省不罚分；x0 缺省依次回退
//	user_config.x0 / target.x0）
//
// user_config 可覆盖 func_key / x0。
//
// 评分 = 根精度分 - 迭代效率罚分：
//   - 真值根优先取"后端从学生 x0 模拟牛顿迭代"的结果（多根函数如 sqrt_2
//     从负初值收敛到负根，按学生实际路径评分才公平）；模拟不收敛时回退
//     bisectRoot(target.interval) 且不罚迭代分
//   - 罚分 = 4 x max(0, iterations - expected - 2)（expected 为模拟迭代数，
//     宽限 2 步覆盖停止准则差异），防止用二分法冒充牛顿提交
func scoreNewtonIter(readings, target, userConfig []byte) (score int, passed bool, detail string, err error) {
	var tgt struct {
		FuncKey       string    `json:"func_key"`
		Interval      []float64 `json:"interval"`
		X0            *float64  `json:"x0"`
		Tolerance     float64   `json:"tolerance"`
		MaxIterations int       `json:"max_iterations"`
		PassScore     int       `json:"pass_score"`
	}
	if err = json.Unmarshal(target, &tgt); err != nil {
		err = fmt.Errorf("target 解析失败: %w", err)
		return
	}
	// v0.5 模式：user_config 覆盖同名字段
	var uc struct {
		FuncKey string   `json:"func_key"`
		X0      *float64 `json:"x0"`
	}
	_ = json.Unmarshal(userConfig, &uc) // 可选字段，缺省/为空时忽略
	if uc.FuncKey != "" {
		tgt.FuncKey = uc.FuncKey
	}
	if uc.X0 != nil {
		tgt.X0 = uc.X0
	}

	fd, ok := numFuncs[tgt.FuncKey]
	if !ok {
		err = fmt.Errorf("未知函数: %s", tgt.FuncKey)
		return
	}
	if tgt.Tolerance <= 0 {
		tgt.Tolerance = 1e-6
	}
	if tgt.MaxIterations <= 0 {
		tgt.MaxIterations = 50
	}

	var rs []struct {
		Root       *float64 `json:"root"`
		Iterations *int     `json:"iterations"`
		X0         *float64 `json:"x0"`
	}
	if err = json.Unmarshal(readings, &rs); err != nil {
		err = fmt.Errorf("readings 解析失败: %w", err)
		return
	}
	r0 := rs[0]
	if r0.Root == nil {
		err = fmt.Errorf("readings[0] 缺少 root 字段")
		return
	}
	root := *r0.Root
	if !isFinite(root) {
		err = fmt.Errorf("root 不能是 NaN/Inf")
		return
	}
	if r0.X0 != nil && !isFinite(*r0.X0) {
		err = fmt.Errorf("x0 不能是 NaN/Inf")
		return
	}
	if r0.Iterations != nil && (*r0.Iterations < 1 || *r0.Iterations > tgt.MaxIterations) {
		err = fmt.Errorf("iterations 须在 1~%d 之间", tgt.MaxIterations)
		return
	}
	// 学生实际初值优先级：readings.x0 > user_config.x0 > target.x0
	x0 := r0.X0
	if x0 == nil {
		x0 = tgt.X0
	}

	// 真值根：优先按学生初值模拟牛顿迭代
	trueRoot, expectedIters := 0.0, 0
	haveTrue := false
	if x0 != nil {
		if r, n, nerr := newtonSolve(fd.f, fd.df, *x0, tgt.Tolerance, tgt.MaxIterations); nerr == nil {
			trueRoot, expectedIters, haveTrue = r, n, true
		}
	}
	if !haveTrue {
		// 回退：target 区间二分真值（换题后区间不夹根会明确报错）
		if !validInterval(tgt.Interval) {
			err = fmt.Errorf("牛顿模拟未收敛且无合法兜底区间")
			return
		}
		if trueRoot, err = bisectRoot(fd.f, tgt.Interval[0], tgt.Interval[1]); err != nil {
			return
		}
	}

	relErr := relError(root, trueRoot)
	rootScore := scoreFromRelErr(relErr)
	penalty := 0
	if expectedIters > 0 && r0.Iterations != nil {
		penalty = 4 * (*r0.Iterations - expectedIters - 2)
		if penalty < 0 {
			penalty = 0
		}
	}
	score = rootScore - penalty
	if score < 0 {
		score = 0
	}
	passed = passedByScore(score, tgt.PassScore)

	detail = fmt.Sprintf("root=%.10g rel_err=%.3g true_root=%.10g", root, relErr, trueRoot)
	if x0 != nil {
		detail += fmt.Sprintf(" x0=%.6g", *x0)
	}
	if r0.Iterations != nil {
		detail += fmt.Sprintf(" iterations=%d", *r0.Iterations)
	}
	if expectedIters > 0 {
		detail += fmt.Sprintf(" expected_iterations=%d", expectedIters)
	}
	if penalty > 0 {
		detail += fmt.Sprintf(" penalty=%d", penalty)
	}
	return
}

// ========== L3 高斯消去法解线性方程组 ==========

// linSysDef 线性方程组 Ax = b（后端持真值）。
// system_key -> {A, b}，与 migration config 下发的 A/b 展示数据一一对应；
// 前端仅用 config 出题/渲染，评分一律以这里的定义现算为准。
type linSysDef struct {
	A [][]float64
	b []float64
}

var linSystems = map[string]linSysDef{
	"sys3_basic": { // 经典教材例题，解恰为整数 (2, 3, -1)，良态
		A: [][]float64{{2, 1, -1}, {-3, -1, 2}, {-2, 1, 2}},
		b: []float64{8, -11, -3},
	},
	"sys4_tridiag": { // 三对角矩阵（弹簧链/热传导差分模型），解 (1, -1, 2, 0.5)，良态且对角占优
		A: [][]float64{{4, 1, 0, 0}, {1, 4, 1, 0}, {0, 1, 4, 1}, {0, 0, 1, 4}},
		b: []float64{3, -1, 7.5, 4},
	},
	"sys3_diagdom": { // 严格对角占优（雅可比/高斯-赛德尔必收敛），解恰为整数 (1, 2, 3)
		A: [][]float64{{10, 2, 1}, {1, 10, 2}, {2, 1, 10}},
		b: []float64{17, 27, 34},
	},
	"hilbert4": { // 4x4 Hilbert 病态矩阵（教学：无选主元时精度急剧损失），解全 1
		A: [][]float64{
			{1, 1.0 / 2, 1.0 / 3, 1.0 / 4},
			{1.0 / 2, 1.0 / 3, 1.0 / 4, 1.0 / 5},
			{1.0 / 3, 1.0 / 4, 1.0 / 5, 1.0 / 6},
			{1.0 / 4, 1.0 / 5, 1.0 / 6, 1.0 / 7},
		},
		b: []float64{25.0 / 12, 77.0 / 60, 57.0 / 60, 319.0 / 420}, // 行和，故解为全 1
	},
	"sys6_poisson": { // 一维泊松方程差分（-u''=f，等距网格 6 内点），三对角对称正定，解全 1。
		// SOR 主场：雅可比 100 步内不收敛、高斯-赛德尔 61 步、理论最优 ω≈1.39 仅 19 步
		A: [][]float64{
			{2, -1, 0, 0, 0, 0},
			{-1, 2, -1, 0, 0, 0},
			{0, -1, 2, -1, 0, 0},
			{0, 0, -1, 2, -1, 0},
			{0, 0, 0, -1, 2, -1},
			{0, 0, 0, 0, -1, 2},
		},
		b: []float64{1, 0, 0, 0, 0, 1}, // 边界值驱动，故解为全 1
	},
}

// gaussSolve 列主元高斯消去法参考解（消去 + 回代），作为评分真值。
// A 须为 n x n 方阵且与 b 同维（2~8 阶由调用方把关）；消去中主元≈0
// （按原矩阵最大元的相对阈值判奇异）或数值溢出时报错。
func gaussSolve(A [][]float64, b []float64) ([]float64, error) {
	n := len(A)
	if n == 0 || len(b) != n {
		return nil, fmt.Errorf("A 与 b 维度不匹配（A %d 阶, b 长 %d）", n, len(b))
	}
	M := make([][]float64, n)
	maxAbs := 0.0
	for i := 0; i < n; i++ {
		if len(A[i]) != n {
			return nil, fmt.Errorf("A 第 %d 行长 %d，不是方阵", i+1, len(A[i]))
		}
		M[i] = append([]float64(nil), A[i]...)
		for _, v := range A[i] {
			if !isFinite(v) {
				return nil, fmt.Errorf("A[%d] 含 NaN/Inf", i+1)
			}
			if av := math.Abs(v); av > maxAbs {
				maxAbs = av
			}
		}
	}
	x := make([]float64, n)
	for i, v := range b {
		if !isFinite(v) {
			return nil, fmt.Errorf("b 含 NaN/Inf")
		}
		x[i] = v
	}
	singTol := 1e-12 * maxAbs // 奇异阈值随矩阵元素量级缩放
	for k := 0; k < n; k++ {
		p := k // 选列主元
		for i := k + 1; i < n; i++ {
			if math.Abs(M[i][k]) > math.Abs(M[p][k]) {
				p = i
			}
		}
		piv := math.Abs(M[p][k])
		if !isFinite(piv) || piv <= singTol {
			return nil, fmt.Errorf("矩阵奇异（第 %d 步主元≈0），无法求解", k+1)
		}
		if p != k {
			M[k], M[p] = M[p], M[k]
			x[k], x[p] = x[p], x[k]
		}
		for i := k + 1; i < n; i++ {
			m := M[i][k] / M[k][k]
			if !isFinite(m) {
				return nil, fmt.Errorf("消去过程数值溢出")
			}
			for j := k; j < n; j++ {
				M[i][j] -= m * M[k][j]
			}
			x[i] -= m * x[k]
		}
	}
	for i := n - 1; i >= 0; i-- {
		s := x[i]
		for j := i + 1; j < n; j++ {
			s -= M[i][j] * x[j]
		}
		x[i] = s / M[i][i]
		if !isFinite(x[i]) {
			return nil, fmt.Errorf("回代结果非有限值")
		}
	}
	return x, nil
}

// vecRelError 向量无穷范数相对误差 ||x-x*||inf / max(||x*||inf, 1e-12)，
// 真值范数≈0 时退化为绝对误差（与标量 relError 同防 0/0）。
func vecRelError(got, want []float64) float64 {
	diff, norm := 0.0, 0.0
	for i := range want {
		if d := math.Abs(got[i] - want[i]); d > diff {
			diff = d
		}
		if w := math.Abs(want[i]); w > norm {
			norm = w
		}
	}
	if norm < 1e-12 {
		norm = 1e-12
	}
	return diff / norm
}

// joinGauss 数值切片拼成 "%.6g,%.6g"（detail 记录用）。
func joinGauss(xs []float64) string {
	parts := make([]string, len(xs))
	for i, v := range xs {
		parts[i] = fmt.Sprintf("%.6g", v)
	}
	return strings.Join(parts, ",")
}

// scoreGaussElim 高斯消去法解线性方程组评分。
//
// target:     {"method":"linear_rel_err","system_key":"sys3_basic","pass_score":60}
//
// readings:   [{"solution":[2,3,-1]}]（长度须等于未知数个数）
//
// user_config 两种换题方式（后端一律重算真值，防背答案）：
//
//	{"system_key":"sys4_tridiag"}  换库内方程组；
//	{"A":[[...]],"b":[...]}        自定义方程组（A/b 优先于 system_key；
//	                               2~8 阶方阵、元素有限，奇异 -> 400）
//
// 评分：后端用列主元高斯消去法现算真值解 x*（真值不落库），按无穷范数
// 相对误差 ||x-x*||inf/||x*||inf 打分；缺省走数值分析默认曲线，可用
// target.bands 覆盖（与物理实验同一参数化机制）。
func scoreGaussElim(readings, target, userConfig []byte) (score int, passed bool, detail string, err error) {
	var tgt struct {
		SystemKey string          `json:"system_key"`
		PassScore int             `json:"pass_score"`
		Bands     json.RawMessage `json:"bands"`
	}
	if err = json.Unmarshal(target, &tgt); err != nil {
		err = fmt.Errorf("target 解析失败: %w", err)
		return
	}
	var bands []relErrBand
	if bands, err = parseRelErrBands(tgt.Bands); err != nil {
		return
	}
	// v0.5 模式：user_config 覆盖（自定义 A/b 优先于 system_key）
	var uc struct {
		SystemKey string      `json:"system_key"`
		A         [][]float64 `json:"A"`
		B         []float64   `json:"b"`
	}
	_ = json.Unmarshal(userConfig, &uc) // 可选字段，缺省/为空时忽略

	A, b := [][]float64(nil), []float64(nil)
	sysKey := tgt.SystemKey
	switch {
	case len(uc.A) > 0:
		sysKey = "custom"
		if len(uc.A) < 2 || len(uc.A) > 8 {
			err = fmt.Errorf("自定义 A 须为 2~8 阶方阵，got %d 阶", len(uc.A))
			return
		}
		A, b = uc.A, uc.B
	case uc.SystemKey != "":
		sysKey = uc.SystemKey
	}
	if A == nil {
		def, ok := linSystems[sysKey]
		if !ok {
			err = fmt.Errorf("未知方程组: %s", sysKey)
			return
		}
		A, b = def.A, def.b
	}

	trueX, serr := gaussSolve(A, b)
	if serr != nil {
		err = serr
		return
	}

	var rs []struct {
		Solution []float64 `json:"solution"`
	}
	if err = json.Unmarshal(readings, &rs); err != nil {
		err = fmt.Errorf("readings 解析失败: %w", err)
		return
	}
	r0 := rs[0]
	if r0.Solution == nil {
		err = fmt.Errorf("readings[0] 缺少 solution 字段")
		return
	}
	if len(r0.Solution) != len(trueX) {
		err = fmt.Errorf("solution 长度须为 %d，got %d", len(trueX), len(r0.Solution))
		return
	}
	for i, v := range r0.Solution {
		if !isFinite(v) {
			err = fmt.Errorf("solution[%d] 不能是 NaN/Inf", i)
			return
		}
	}

	relErr := vecRelError(r0.Solution, trueX)
	score = scoreFromRelErrBands(relErr, bands)
	passed = passedByScore(score, tgt.PassScore)
	detail = fmt.Sprintf("system=%s n=%d rel_err=%.3g solution=[%s] true=[%s]",
		sysKey, len(trueX), relErr, joinGauss(r0.Solution), joinGauss(trueX))
	return
}

// ========== L4 雅可比迭代法解线性方程组 ==========

// jacobiSolve 雅可比迭代参考解：x^{(k+1)}_i = (b_i - Σ_{j≠i} a_ij·x^{(k)}_j) / a_ii，
// 停止准则 max_i |x^{(k+1)}_i - x^{(k)}_i| < tol，返回迭代解与产生满足准则的
// 近似解的迭代步数。非方阵/维度不匹配/a_ii=0 报错；maxIter 内未收敛报错
// （调用方决定回退策略：本实验真值走 gaussSolve，模拟失败只影响期望迭代数）。
func jacobiSolve(A [][]float64, b, x0 []float64, tol float64, maxIter int) ([]float64, int, error) {
	n := len(A)
	if n == 0 || len(b) != n {
		return nil, 0, fmt.Errorf("A 与 b 维度不匹配（A %d 阶, b 长 %d）", n, len(b))
	}
	x := make([]float64, n)
	if x0 != nil {
		if len(x0) != n {
			return nil, 0, fmt.Errorf("x0 长度须为 %d，got %d", n, len(x0))
		}
		copy(x, x0)
	}
	xn := make([]float64, n)
	for k := 1; k <= maxIter; k++ {
		for i := 0; i < n; i++ {
			if len(A[i]) != n {
				return nil, 0, fmt.Errorf("A 第 %d 行长 %d，不是方阵", i+1, len(A[i]))
			}
			if A[i][i] == 0 || !isFinite(A[i][i]) {
				return nil, 0, fmt.Errorf("A[%d][%d]=0，雅可比迭代无定义", i+1, i+1)
			}
			s := b[i]
			for j := 0; j < n; j++ {
				if j != i {
					s -= A[i][j] * x[j]
				}
			}
			xn[i] = s / A[i][i]
			if !isFinite(xn[i]) {
				return nil, 0, fmt.Errorf("迭代出现非有限值")
			}
		}
		diff := 0.0
		for i := 0; i < n; i++ {
			if d := math.Abs(xn[i] - x[i]); d > diff {
				diff = d
			}
		}
		copy(x, xn)
		if diff < tol {
			return x, k, nil
		}
	}
	return nil, 0, fmt.Errorf("%d 次内未收敛", maxIter)
}

// scoreJacobiIter 雅可比迭代法解线性方程组评分。
//
// target:     {"method":"jacobi_rel_err","system_key":"sys3_diagdom","x0":[0,0,0],
//
//	"tolerance":1e-6,"max_iterations":100,"pass_score":60}
//
// readings:   [{"solution":[1,2,3],"iterations":14,"x0":[0,0,0]}]
//
//	solution 必填；iterations/x0 可选（iterations 缺省不罚分；x0 缺省依次回退
//	user_config.x0 / target.x0 / 零向量）
//
// user_config 可覆盖 system_key / x0，或自定义 A/b（2~8 阶方阵，对角元须非零）。
//
// 评分 = 解精度分 - 迭代效率罚分：
//   - 真值解 = gaussSolve 列主元高斯精确解（真值不落库）；雅可比收敛时
//     迭代解与之一致，用精确解可同时覆盖不收敛系统的评分
//   - 期望迭代数 = 后端从学生 x0 模拟雅可比（max_i|Δx|<tol 停止）的步数；
//     模拟不收敛（如非对角占优系统）则无期望值、不罚分，仅按精度计分
//   - 罚分 = 4 x max(0, expected/2 - 2 - iterations)：方向与牛顿实验相反，
//     声称迭代数低于期望一半再留 2 步宽限才罚（高斯-赛德尔约快一倍属合法
//     实现，不可罚），防用直接消去法求出精确解后冒充少量雅可比迭代提交；
//     如实报高迭代数（收敛慢）不罚
func scoreJacobiIter(readings, target, userConfig []byte) (score int, passed bool, detail string, err error) {
	var tgt struct {
		SystemKey     string          `json:"system_key"`
		X0            []float64       `json:"x0"`
		Tolerance     float64         `json:"tolerance"`
		MaxIterations int             `json:"max_iterations"`
		PassScore     int             `json:"pass_score"`
		Bands         json.RawMessage `json:"bands"`
	}
	if err = json.Unmarshal(target, &tgt); err != nil {
		err = fmt.Errorf("target 解析失败: %w", err)
		return
	}
	var bands []relErrBand
	if bands, err = parseRelErrBands(tgt.Bands); err != nil {
		return
	}
	// v0.5 模式：user_config 覆盖（自定义 A/b 优先于 system_key）
	var uc struct {
		SystemKey string      `json:"system_key"`
		A         [][]float64 `json:"A"`
		B         []float64   `json:"b"`
		X0        []float64   `json:"x0"`
	}
	_ = json.Unmarshal(userConfig, &uc) // 可选字段，缺省/为空时忽略

	A, b := [][]float64(nil), []float64(nil)
	sysKey := tgt.SystemKey
	switch {
	case len(uc.A) > 0:
		sysKey = "custom"
		if len(uc.A) < 2 || len(uc.A) > 8 {
			err = fmt.Errorf("自定义 A 须为 2~8 阶方阵，got %d 阶", len(uc.A))
			return
		}
		A, b = uc.A, uc.B
	case uc.SystemKey != "":
		sysKey = uc.SystemKey
	}
	if A == nil {
		def, ok := linSystems[sysKey]
		if !ok {
			err = fmt.Errorf("未知方程组: %s", sysKey)
			return
		}
		A, b = def.A, def.b
	}

	// 真值解：直接法精确解（gaussSolve 同时完成方阵/有限/非奇异校验）
	var trueX []float64
	if trueX, err = gaussSolve(A, b); err != nil {
		return
	}
	// 雅可比要求对角元非零（gaussSolve 已保证方阵与元素有限）
	for i := range A {
		if A[i][i] == 0 {
			err = fmt.Errorf("雅可比迭代要求对角元非零（A[%d][%d]=0）", i+1, i+1)
			return
		}
	}

	var rs []struct {
		Solution   []float64 `json:"solution"`
		Iterations *int      `json:"iterations"`
		X0         []float64 `json:"x0"`
	}
	if err = json.Unmarshal(readings, &rs); err != nil {
		err = fmt.Errorf("readings 解析失败: %w", err)
		return
	}
	r0 := rs[0]
	if r0.Solution == nil {
		err = fmt.Errorf("readings[0] 缺少 solution 字段")
		return
	}
	if len(r0.Solution) != len(trueX) {
		err = fmt.Errorf("solution 长度须为 %d，got %d", len(trueX), len(r0.Solution))
		return
	}
	for i, v := range r0.Solution {
		if !isFinite(v) {
			err = fmt.Errorf("solution[%d] 不能是 NaN/Inf", i)
			return
		}
	}
	if tgt.Tolerance <= 0 {
		tgt.Tolerance = 1e-6
	}
	if tgt.MaxIterations <= 0 {
		tgt.MaxIterations = 100
	}
	if r0.Iterations != nil && (*r0.Iterations < 1 || *r0.Iterations > tgt.MaxIterations) {
		err = fmt.Errorf("iterations 须在 1~%d 之间", tgt.MaxIterations)
		return
	}
	// 学生实际初值优先级：readings.x0 > user_config.x0 > target.x0 > 零向量。
	// readings/user_config 的 x0 由学生/前端给出，长度须与方程组匹配（错 -> 400）；
	// target.x0 属于 target 的默认题，换题（user_config）后维度可能不符，
	// 此时回退零向量而非报错（学生无过错，零初值也是标准教学起点）。
	x0 := r0.X0
	if x0 == nil {
		x0 = uc.X0
	}
	if x0 == nil && len(tgt.X0) == len(trueX) {
		x0 = tgt.X0
	}
	if x0 != nil {
		if len(x0) != len(trueX) {
			err = fmt.Errorf("x0 长度须为 %d，got %d", len(trueX), len(x0))
			return
		}
		for _, v := range x0 {
			if !isFinite(v) {
				err = fmt.Errorf("x0 不能是 NaN/Inf")
				return
			}
		}
	}

	// 期望迭代数：从学生 x0 模拟雅可比；不收敛（如非对角占优）则无期望、不罚分
	expected := 0
	if _, n, jerr := jacobiSolve(A, b, x0, tgt.Tolerance, tgt.MaxIterations); jerr == nil {
		expected = n
	}

	relErr := vecRelError(r0.Solution, trueX)
	score = scoreFromRelErrBands(relErr, bands)
	penalty := 0
	if expected > 0 && r0.Iterations != nil {
		if p := expected/2 - 2 - *r0.Iterations; p > 0 {
			penalty = 4 * p
		}
	}
	score -= penalty
	if score < 0 {
		score = 0
	}
	passed = passedByScore(score, tgt.PassScore)

	detail = fmt.Sprintf("system=%s n=%d rel_err=%.3g solution=[%s] true=[%s]",
		sysKey, len(trueX), relErr, joinGauss(r0.Solution), joinGauss(trueX))
	if x0 != nil {
		detail += fmt.Sprintf(" x0=[%s]", joinGauss(x0))
	}
	if r0.Iterations != nil {
		detail += fmt.Sprintf(" iterations=%d", *r0.Iterations)
	}
	if expected > 0 {
		detail += fmt.Sprintf(" expected_iterations=%d", expected)
	}
	if penalty > 0 {
		detail += fmt.Sprintf(" penalty=%d", penalty)
	}
	return
}

// ========== L5 SOR 逐次超松弛迭代法解线性方程组 ==========

// sorSolve SOR 迭代参考解：x^{(k+1)}_i = (1-ω)·x_i^{(k)} + (ω/a_ii)·(b_i - Σ_{j≠i} a_ij·x_j)，
// 其中 j<i 用刚更新的新值 x_j^{(k+1)}、j>i 用旧值 x_j^{(k)}（高斯-赛德尔式就地更新），
// ω=1 时退化为高斯-赛德尔。停止准则与雅可比一致：max_i |x^{(k+1)}_i - x^{(k)}_i| < tol，
// 返回迭代解与产生满足准则的近似解的迭代步数。非方阵/维度不匹配/a_ii=0/ω∉(0,2) 报错；
// maxIter 内未收敛报错（本实验真值走 gaussSolve，模拟失败只影响期望迭代数）。
func sorSolve(A [][]float64, b, x0 []float64, omega float64, tol float64, maxIter int) ([]float64, int, error) {
	n := len(A)
	if n == 0 || len(b) != n {
		return nil, 0, fmt.Errorf("A 与 b 维度不匹配（A %d 阶, b 长 %d）", n, len(b))
	}
	if !isFinite(omega) || omega <= 0 || omega >= 2 {
		return nil, 0, fmt.Errorf("松弛因子 ω 须在 (0, 2) 开区间内（SOR 收敛的必要条件），got %g", omega)
	}
	x := make([]float64, n)
	if x0 != nil {
		if len(x0) != n {
			return nil, 0, fmt.Errorf("x0 长度须为 %d，got %d", n, len(x0))
		}
		copy(x, x0)
	}
	xn := make([]float64, n)
	copy(xn, x) // 就地更新起点：上一轮（或初值）的完整近似
	for k := 1; k <= maxIter; k++ {
		for i := 0; i < n; i++ {
			if len(A[i]) != n {
				return nil, 0, fmt.Errorf("A 第 %d 行长 %d，不是方阵", i+1, len(A[i]))
			}
			if A[i][i] == 0 || !isFinite(A[i][i]) {
				return nil, 0, fmt.Errorf("A[%d][%d]=0，SOR 迭代无定义", i+1, i+1)
			}
			s := b[i]
			for j := 0; j < n; j++ {
				if j != i {
					s -= A[i][j] * xn[j] // j<i 为新值、j>i 为旧值
				}
			}
			v := (1-omega)*xn[i] + omega*s/A[i][i]
			if !isFinite(v) {
				return nil, 0, fmt.Errorf("迭代出现非有限值")
			}
			xn[i] = v
		}
		diff := 0.0
		for i := 0; i < n; i++ {
			if d := math.Abs(xn[i] - x[i]); d > diff {
				diff = d
			}
		}
		copy(x, xn)
		if diff < tol {
			return x, k, nil
		}
	}
	return nil, 0, fmt.Errorf("%d 次内未收敛", maxIter)
}

// scoreSorIter SOR 逐次超松弛迭代法解线性方程组评分。
//
// target:     {"method":"sor_rel_err","system_key":"sys6_poisson","x0":[0,0,0,0,0,0],
//
//	"omega":1.0,"tolerance":1e-6,"max_iterations":100,"pass_score":60}
//
// readings:   [{"solution":[...],"iterations":19,"omega":1.4,"x0":[...]}]
//
//	solution 必填；iterations/omega/x0 可选（iterations 缺省不罚分；omega 缺省依次
//	回退 user_config.omega / target.omega / 1.0 即高斯-赛德尔；x0 缺省依次回退
//	user_config.x0 / target.x0 / 零向量）
//
// user_config 可覆盖 system_key / x0 / omega，或自定义 A/b（2~8 阶方阵，对角元须非零）。
//
// 评分 = 解精度分 - 迭代效率罚分（与雅可比同式 4 x max(0, expected/2 - 2 - iterations)）：
//   - 真值解 = gaussSolve 列主元高斯精确解（真值不落库），收敛时与 SOR 迭代解一致
//   - 期望迭代数 = 后端从学生 x0 与 ω 模拟 SOR（max_i|Δx|<tol 停止）；ω 与 x0 一律
//     以学生实际提交为准（换 ω 后按新口径算期望，防按旧口径虚报）；模拟不收敛
//     （如 ω 过大或系统不合适）则无期望、不罚分，仅按精度计分
func scoreSorIter(readings, target, userConfig []byte) (score int, passed bool, detail string, err error) {
	var tgt struct {
		SystemKey     string          `json:"system_key"`
		X0            []float64       `json:"x0"`
		Omega         *float64        `json:"omega"`
		Tolerance     float64         `json:"tolerance"`
		MaxIterations int             `json:"max_iterations"`
		PassScore     int             `json:"pass_score"`
		Bands         json.RawMessage `json:"bands"`
	}
	if err = json.Unmarshal(target, &tgt); err != nil {
		err = fmt.Errorf("target 解析失败: %w", err)
		return
	}
	var bands []relErrBand
	if bands, err = parseRelErrBands(tgt.Bands); err != nil {
		return
	}
	// v0.5 模式：user_config 覆盖（自定义 A/b 优先于 system_key）
	var uc struct {
		SystemKey string      `json:"system_key"`
		A         [][]float64 `json:"A"`
		B         []float64   `json:"b"`
		X0        []float64   `json:"x0"`
		Omega     *float64    `json:"omega"`
	}
	_ = json.Unmarshal(userConfig, &uc) // 可选字段，缺省/为空时忽略

	A, b := [][]float64(nil), []float64(nil)
	sysKey := tgt.SystemKey
	switch {
	case len(uc.A) > 0:
		sysKey = "custom"
		if len(uc.A) < 2 || len(uc.A) > 8 {
			err = fmt.Errorf("自定义 A 须为 2~8 阶方阵，got %d 阶", len(uc.A))
			return
		}
		A, b = uc.A, uc.B
	case uc.SystemKey != "":
		sysKey = uc.SystemKey
	}
	if A == nil {
		def, ok := linSystems[sysKey]
		if !ok {
			err = fmt.Errorf("未知方程组: %s", sysKey)
			return
		}
		A, b = def.A, def.b
	}

	// 真值解：直接法精确解（gaussSolve 同时完成方阵/有限/非奇异校验）
	var trueX []float64
	if trueX, err = gaussSolve(A, b); err != nil {
		return
	}
	// SOR 要求对角元非零（gaussSolve 已保证方阵与元素有限）
	for i := range A {
		if A[i][i] == 0 {
			err = fmt.Errorf("SOR 迭代要求对角元非零（A[%d][%d]=0）", i+1, i+1)
			return
		}
	}

	var rs []struct {
		Solution   []float64 `json:"solution"`
		Iterations *int      `json:"iterations"`
		X0         []float64 `json:"x0"`
		Omega      *float64  `json:"omega"`
	}
	if err = json.Unmarshal(readings, &rs); err != nil {
		err = fmt.Errorf("readings 解析失败: %w", err)
		return
	}
	r0 := rs[0]
	if r0.Solution == nil {
		err = fmt.Errorf("readings[0] 缺少 solution 字段")
		return
	}
	if len(r0.Solution) != len(trueX) {
		err = fmt.Errorf("solution 长度须为 %d，got %d", len(trueX), len(r0.Solution))
		return
	}
	for i, v := range r0.Solution {
		if !isFinite(v) {
			err = fmt.Errorf("solution[%d] 不能是 NaN/Inf", i)
			return
		}
	}
	if tgt.Tolerance <= 0 {
		tgt.Tolerance = 1e-6
	}
	if tgt.MaxIterations <= 0 {
		tgt.MaxIterations = 100
	}
	if r0.Iterations != nil && (*r0.Iterations < 1 || *r0.Iterations > tgt.MaxIterations) {
		err = fmt.Errorf("iterations 须在 1~%d 之间", tgt.MaxIterations)
		return
	}
	// 松弛因子优先级：readings.omega > user_config.omega > target.omega > 1.0（高斯-赛德尔）
	omega := 1.0
	if r0.Omega != nil {
		omega = *r0.Omega
	} else if uc.Omega != nil {
		omega = *uc.Omega
	} else if tgt.Omega != nil {
		omega = *tgt.Omega
	}
	if !isFinite(omega) || omega <= 0 || omega >= 2 {
		err = fmt.Errorf("松弛因子 ω 须在 (0, 2) 开区间内（SOR 收敛的必要条件），got %g", omega)
		return
	}
	// 学生实际初值优先级：readings.x0 > user_config.x0 > target.x0 > 零向量。
	// readings/user_config 的 x0 由学生/前端给出，长度须与方程组匹配（错 -> 400）；
	// target.x0 属于 target 的默认题，换题（user_config）后维度可能不符，
	// 此时回退零向量而非报错（学生无过错，零初值也是标准教学起点）。
	x0 := r0.X0
	if x0 == nil {
		x0 = uc.X0
	}
	if x0 == nil && len(tgt.X0) == len(trueX) {
		x0 = tgt.X0
	}
	if x0 != nil {
		if len(x0) != len(trueX) {
			err = fmt.Errorf("x0 长度须为 %d，got %d", len(trueX), len(x0))
			return
		}
		for _, v := range x0 {
			if !isFinite(v) {
				err = fmt.Errorf("x0 不能是 NaN/Inf")
				return
			}
		}
	}

	// 期望迭代数：从学生 x0 与 ω 模拟 SOR；不收敛（如 ω 过大/系统不合适）则无期望、不罚分
	expected := 0
	if _, n, jerr := sorSolve(A, b, x0, omega, tgt.Tolerance, tgt.MaxIterations); jerr == nil {
		expected = n
	}

	relErr := vecRelError(r0.Solution, trueX)
	score = scoreFromRelErrBands(relErr, bands)
	penalty := 0
	if expected > 0 && r0.Iterations != nil {
		if p := expected/2 - 2 - *r0.Iterations; p > 0 {
			penalty = 4 * p
		}
	}
	score -= penalty
	if score < 0 {
		score = 0
	}
	passed = passedByScore(score, tgt.PassScore)

	detail = fmt.Sprintf("system=%s n=%d rel_err=%.3g solution=[%s] true=[%s] omega=%g",
		sysKey, len(trueX), relErr, joinGauss(r0.Solution), joinGauss(trueX), omega)
	if x0 != nil {
		detail += fmt.Sprintf(" x0=[%s]", joinGauss(x0))
	}
	if r0.Iterations != nil {
		detail += fmt.Sprintf(" iterations=%d", *r0.Iterations)
	}
	if expected > 0 {
		detail += fmt.Sprintf(" expected_iterations=%d", expected)
	}
	if penalty > 0 {
		detail += fmt.Sprintf(" penalty=%d", penalty)
	}
	return
}

// ========== L6 拉格朗日插值 ==========

// interpProblemDef 插值题节点表（后端持真值）。
// problem_key -> {xs, ys, x_eval}，与 migration config 下发的节点数据一一对应。
// 评分真值 = 拉格朗日插值多项式在 x_eval 处的值 P(x_eval)（由节点唯一确定）；
// 原函数仅供前端画图对照（插值误差 / 龙格现象是教学观察点，不参与评分），
// 不进后端数据结构。
type interpProblemDef struct {
	Xs    []float64
	Ys    []float64
	XEval float64
}

var interpProblems = map[string]interpProblemDef{
	"sin_quad": { // sin x 三点二次插值：基函数值恰为 2/9、8/9、−1/9（手工可验证），
		// P(π/6) = (4√2−1)/9 ≈ 0.517428，sin(π/6) = 0.5（插值误差 0.017）
		Xs:    []float64{0, math.Pi / 4, math.Pi / 2},
		Ys:    []float64{0, math.Sqrt2 / 2, 1},
		XEval: math.Pi / 6,
	},
	"sqrt_table": { // 教材经典：开方表二次插值求 √115，P(115) ≈ 10.722756
		// vs √115 ≈ 10.723805（插值误差仅 0.001，插值求根号的历史应用）
		Xs:    []float64{100, 121, 144},
		Ys:    []float64{10, 11, 12},
		XEval: 115,
	},
	"runge_5": { // 龙格现象：f = 1/(1+25x²) 取 5 个等距节点，P(0.8) ≈ −0.379310
		// 而 f(0.8) = 1/17 ≈ 0.058824——等距高次插值在区间中部已剧烈振荡，
		// 抄原函数值与算插值的相对误差差 1.15 倍，天然防"不插值抄 f 值"
		Xs:    []float64{-1, -0.5, 0, 0.5, 1},
		Ys:    []float64{1.0 / 26, 4.0 / 29, 1, 4.0 / 29, 1.0 / 26},
		XEval: 0.8,
	},
}

// lagrangeEval 拉格朗日插值求值：P(x) = Σ_i y_i·Π_{j≠i}(x−x_j)/(x_i−x_j)。
// 节点 2~8 个、x 互异、全部数值有限（含 x_eval），否则报错（视为数据配置错误）。
func lagrangeEval(xs, ys []float64, xEval float64) (float64, error) {
	n := len(xs)
	if n < 2 || n > 8 {
		return 0, fmt.Errorf("插值节点数须为 2~8，got %d", n)
	}
	if len(ys) != n {
		return 0, fmt.Errorf("x/y 节点数不匹配（x %d 个, y %d 个）", n, len(ys))
	}
	for i := 0; i < n; i++ {
		if !isFinite(xs[i]) || !isFinite(ys[i]) {
			return 0, fmt.Errorf("节点含 NaN/Inf")
		}
		for j := 0; j < i; j++ {
			if xs[i] == xs[j] {
				return 0, fmt.Errorf("节点 x 须互异（x[%d]=x[%d]=%g）", i, j, xs[i])
			}
		}
	}
	if !isFinite(xEval) {
		return 0, fmt.Errorf("x_eval 不能是 NaN/Inf")
	}
	v := 0.0
	for i := 0; i < n; i++ {
		li := 1.0
		for j := 0; j < n; j++ {
			if j != i {
				li *= (xEval - xs[j]) / (xs[i] - xs[j])
			}
		}
		v += ys[i] * li
	}
	if !isFinite(v) {
		return 0, fmt.Errorf("插值结果非有限值")
	}
	return v, nil
}

// scoreLagrangeInterp 拉格朗日插值评分。
//
// target:     {"method":"interp_rel_err","problem_key":"sin_quad","x_eval":0.5236,"pass_score":60}
// readings:   [{"value":0.5174,"basis":[0.2222,0.8889,-0.1111]}]
//
//	value 必填（= 插值多项式在 x_eval 处的值）；basis 可选（学生算的各基函数
//	值 L_i(x_eval)，长度须等于节点数，仅记入审计 detail，不参与打分）
//
// user_config 可覆盖 problem_key（换题）/ x_eval（换求值点），或自定义节点
//
//	{"nodes_x":[...],"nodes_y":[...],"x_eval":...}（2~8 个互异节点，后端重算真值）
//
// 评分 = 插值值精度分：真值 P(x_eval) 由后端按 target/user_config 节点现算
// （拉格朗日插值多项式由节点唯一确定，无迭代路径歧义），按 |value−P|/|P|
// 相对误差走默认曲线（target.bands 可覆盖）。龙格题上直接抄原函数值 f(x_eval)
// 与插值真值差 ~1.15 倍相对误差 -> 0 分，"不做插值抄函数值"天然不可行。
func scoreLagrangeInterp(readings, target, userConfig []byte) (score int, passed bool, detail string, err error) {
	var tgt struct {
		ProblemKey string          `json:"problem_key"`
		XEval      *float64        `json:"x_eval"`
		PassScore  int             `json:"pass_score"`
		Bands      json.RawMessage `json:"bands"`
	}
	if err = json.Unmarshal(target, &tgt); err != nil {
		err = fmt.Errorf("target 解析失败: %w", err)
		return
	}
	var bands []relErrBand
	if bands, err = parseRelErrBands(tgt.Bands); err != nil {
		return
	}
	// v0.5 模式：user_config 覆盖（自定义节点优先于 problem_key）
	var uc struct {
		ProblemKey string    `json:"problem_key"`
		NodesX     []float64 `json:"nodes_x"`
		NodesY     []float64 `json:"nodes_y"`
		XEval      *float64  `json:"x_eval"`
	}
	_ = json.Unmarshal(userConfig, &uc) // 可选字段，缺省/为空时忽略

	xs, ys := []float64(nil), []float64(nil)
	probKey := tgt.ProblemKey
	defXEval := math.NaN() // 库内题的默认求值点（custom 题无默认，须显式给出）
	switch {
	case len(uc.NodesX) > 0 || len(uc.NodesY) > 0:
		probKey = "custom"
		xs, ys = uc.NodesX, uc.NodesY
	case uc.ProblemKey != "":
		probKey = uc.ProblemKey
	}
	if xs == nil {
		def, ok := interpProblems[probKey]
		if !ok {
			err = fmt.Errorf("未知插值题: %s", probKey)
			return
		}
		xs, ys = def.Xs, def.Ys
		defXEval = def.XEval
	}
	// 求值点优先级：user_config.x_eval >（仅未换题时）target.x_eval > 库内默认。
	// target.x_eval 属于 target 自己的题：user_config 换了 problem_key（或自定义节点）后
	// 不能沿用，须回落到新题的库内默认求值点（custom 题无默认，上方要求显式给出）。
	xEval := defXEval
	if probKey == tgt.ProblemKey && tgt.XEval != nil {
		xEval = *tgt.XEval
	}
	if uc.XEval != nil {
		xEval = *uc.XEval
	}
	if math.IsNaN(xEval) {
		err = fmt.Errorf("自定义节点须同时给出 x_eval")
		return
	}

	trueP, perr := lagrangeEval(xs, ys, xEval)
	if perr != nil {
		err = perr
		return
	}

	var rs []struct {
		Value *float64  `json:"value"`
		Basis []float64 `json:"basis"`
	}
	if err = json.Unmarshal(readings, &rs); err != nil {
		err = fmt.Errorf("readings 解析失败: %w", err)
		return
	}
	r0 := rs[0]
	if r0.Value == nil {
		err = fmt.Errorf("readings[0] 缺少 value 字段")
		return
	}
	value := *r0.Value
	if !isFinite(value) {
		err = fmt.Errorf("value 不能是 NaN/Inf")
		return
	}
	// basis 可选：长度须等于节点数、数值有限（仅审计记录用，错维度直接 400 暴露）
	if r0.Basis != nil {
		if len(r0.Basis) != len(xs) {
			err = fmt.Errorf("basis 长度须为 %d（节点数），got %d", len(xs), len(r0.Basis))
			return
		}
		for i, b := range r0.Basis {
			if !isFinite(b) {
				err = fmt.Errorf("basis[%d] 不能是 NaN/Inf", i)
				return
			}
		}
	}

	relErr := relError(value, trueP)
	score = scoreFromRelErrBands(relErr, bands)
	passed = passedByScore(score, tgt.PassScore)
	detail = fmt.Sprintf("problem=%s n=%d x_eval=%.6g value=%.10g rel_err=%.3g true_P=%.10g",
		probKey, len(xs), xEval, value, relErr, trueP)
	if r0.Basis != nil {
		detail += fmt.Sprintf(" basis=[%s]", joinGauss(r0.Basis))
	}
	return
}

// ---------- L17 复合辛普森积分 ----------

// integProblemDef 数值积分题库定义（前后端逐字同步，f 仅供画图与真值计算）。
type integProblemDef struct {
	A, B float64           // 积分区间
	N    int               // 默认偶数子区间数（题目指定学生用多少段算）
	F    func(x float64) float64
}

var integProblems = map[string]integProblemDef{
	"sin_pi": { // 经典单次辛普森：I = ∫₀^π sin x dx = 2 已知，S_2 = (π/6)(0+4·1+0)
		// = 2π/3 ≈ 2.094395。抄精确积分值 2 的相对误差 ~4.5% -> ~42 分不过线，
		// "不算辛普森抄积分答案"天然被拦（类比 L16 抄 f 值）。
		A: 0, B: math.Pi, N: 2,
		F: math.Sin,
	},
	"erf_gauss": { // f = e^(−x²)，I = (√π/2)·erf(1) ≈ 0.746824 无初等闭式，
		// 必须老老实实数值积分；n=4 复合辛普森手工可算（5 个节点值查表/计算器）
		A: 0, B: 1, N: 4,
		F: func(x float64) float64 { return math.Exp(-x * x) },
	},
	"sqrt_x": { // 端点导数奇异（f'(0) 发散）：辛普森四阶收敛假设被破坏，
		// 收敛慢（n=8 时 S_8 ≈ 0.663079 vs I = 2/3）——方法适用性观察题
		A: 0, B: 1, N: 8,
		F: math.Sqrt,
	},
}

// compositeSimpson 复合辛普森公式：n 为偶数子区间数，
// S_n = (h/3)·[f(a) + 4·Σf(x_奇) + 2·Σf(x_偶) + f(b)]，h = (b−a)/n。
// 每个子区间三元组上恰为过三节点的抛物线（拉格朗日二次）积分。
func compositeSimpson(f func(x float64) float64, a, b float64, n int) float64 {
	h := (b - a) / float64(n)
	odd, even := 0.0, 0.0
	for i := 1; i < n; i++ {
		y := f(a + float64(i)*h)
		if i%2 == 1 {
			odd += y
		} else {
			even += y
		}
	}
	return h / 3 * (f(a) + 4*odd + 2*even + f(b))
}

// simpsonRef 高精度参考积分（n=100000 复合辛普森，光滑函数达机器精度）。
// 仅供审计 detail 记录参考真积分 I，展示 S_n 的截断误差；不参与打分。
func simpsonRef(f func(x float64) float64, a, b float64) float64 {
	return compositeSimpson(f, a, b, 100000)
}

// scoreSimpsonIntegration 复合辛普森积分评分。
//
// target:     {"method":"integ_rel_err","problem_key":"sin_pi","a":0,"b":π,"n":2,"pass_score":60}
// readings:   [{"value":2.0943951,"odd_sum":1,"even_sum":0}]
//
//	value 必填（= 学生算出的复合辛普森值 S_n）；odd_sum/even_sum 可选
//	（学生算的奇/偶下标节点和，仅记入审计 detail，不参与打分）
//
// user_config 可覆盖 problem_key（换题）/ a+b（换区间，须成对给出）/ n（换分段数）
//
// 评分 = 辛普森值精度分：真值 S_n 由后端按同一 f、区间、n 现算（同 L16 真值
// = 插值多项式值的设计：真值是"方法的输出"而非"精确积分 I"），按 |value−S_n|/|S_n|
// 相对误差走默认曲线（target.bands 可覆盖）。直接抄精确积分 I 的分数被截断误差
// 压住（sin_pi 默认 n=2 抄 I=2 只得 ~42 分不过线）；n 须为偶数 2~1024。
// target 的题专属参数（a/b/n）不跨题沿用（v0.12.1 求值点归属规则同款）。
func scoreSimpsonIntegration(readings, target, userConfig []byte) (score int, passed bool, detail string, err error) {
	var tgt struct {
		ProblemKey string          `json:"problem_key"`
		A          *float64        `json:"a"`
		B          *float64        `json:"b"`
		N          *int            `json:"n"`
		PassScore  int             `json:"pass_score"`
		Bands      json.RawMessage `json:"bands"`
	}
	if err = json.Unmarshal(target, &tgt); err != nil {
		err = fmt.Errorf("target 解析失败: %w", err)
		return
	}
	var bands []relErrBand
	if bands, err = parseRelErrBands(tgt.Bands); err != nil {
		return
	}
	// v0.5 模式：user_config 覆盖（problem_key / a+b / n）
	var uc struct {
		ProblemKey string   `json:"problem_key"`
		A          *float64 `json:"a"`
		B          *float64 `json:"b"`
		N          *int     `json:"n"`
	}
	_ = json.Unmarshal(userConfig, &uc) // 可选字段，缺省/为空时忽略

	probKey := tgt.ProblemKey
	if uc.ProblemKey != "" {
		probKey = uc.ProblemKey
	}
	def, ok := integProblems[probKey]
	if !ok {
		err = fmt.Errorf("未知积分题: %s", probKey)
		return
	}

	// 区间归属：user_config.a+b（须成对）>（仅未换题时）target.a+b > 库内默认。
	// target 的区间属于 target 自己的题（v0.12.1 求值点归属规则同款），
	// 换题后回落新题库内默认区间。
	a, b := def.A, def.B
	sameProblem := probKey == tgt.ProblemKey
	if sameProblem && tgt.A != nil && tgt.B != nil {
		a, b = *tgt.A, *tgt.B
	}
	if uc.A != nil || uc.B != nil {
		if uc.A == nil || uc.B == nil {
			err = fmt.Errorf("user_config 区间须成对给出 a、b")
			return
		}
		a, b = *uc.A, *uc.B
	}
	if !isFinite(a) || !isFinite(b) || a >= b {
		err = fmt.Errorf("积分区间非法（须 a < b 且有限）: [%.6g, %.6g]", a, b)
		return
	}

	// 分段数归属：user_config.n >（仅未换题时）target.n > 库内默认。
	n := def.N
	if sameProblem && tgt.N != nil {
		n = *tgt.N
	}
	if uc.N != nil {
		n = *uc.N
	}
	if n < 2 || n > 1024 || n%2 != 0 {
		err = fmt.Errorf("n 须为 2~1024 的偶数，got %d", n)
		return
	}

	trueS := compositeSimpson(def.F, a, b, n)
	if !isFinite(trueS) {
		err = fmt.Errorf("辛普森真值非有限值（区间 [%g,%g] 内 f 无界？）", a, b)
		return
	}

	var rs []struct {
		Value   *float64 `json:"value"`
		OddSum  *float64 `json:"odd_sum"`
		EvenSum *float64 `json:"even_sum"`
	}
	if err = json.Unmarshal(readings, &rs); err != nil {
		err = fmt.Errorf("readings 解析失败: %w", err)
		return
	}
	r0 := rs[0]
	if r0.Value == nil {
		err = fmt.Errorf("readings[0] 缺少 value 字段")
		return
	}
	value := *r0.Value
	if !isFinite(value) {
		err = fmt.Errorf("value 不能是 NaN/Inf")
		return
	}
	// odd_sum/even_sum 可选：数值有限（仅审计记录用，错值直接 400 暴露）
	for name, v := range map[string]*float64{"odd_sum": r0.OddSum, "even_sum": r0.EvenSum} {
		if v != nil && !isFinite(*v) {
			err = fmt.Errorf("%s 不能是 NaN/Inf", name)
			return
		}
	}

	relErr := relError(value, trueS)
	score = scoreFromRelErrBands(relErr, bands)
	passed = passedByScore(score, tgt.PassScore)
	detail = fmt.Sprintf("problem=%s a=%.6g b=%.6g n=%d h=%.6g value=%.10g rel_err=%.3g true_S=%.10g ref_I=%.10g",
		probKey, a, b, n, (b-a)/float64(n), value, relErr, trueS, simpsonRef(def.F, a, b))
	if r0.OddSum != nil {
		detail += fmt.Sprintf(" odd_sum=%.10g", *r0.OddSum)
	}
	if r0.EvenSum != nil {
		detail += fmt.Sprintf(" even_sum=%.10g", *r0.EvenSum)
	}
	return
}

// ---------- L18 逐次线性插值法（Aitken/Neville） ----------

// nevilleEval 逐次线性插值（Neville 递推）求插值多项式在 xEval 处的值。
// 与 L16 的 lagrangeEval 评的是同一个数学对象（插值多项式由节点唯一确定，
// 两路计算结果必相等，单测交叉验证），本关考察的是**递推过程**：
//
//	P_{i,0} = y_i
//	P_{i,k} = ((x−x_{i−k})·P_{i,k−1} − (x−x_i)·P_{i−1,k−1}) / (x_i − x_{i−k})   （k=1..i）
//
// 每一步都是两个低一阶结果之间的线性插值，返回最终 P_{n−1,n−1}。
// 节点校验复用 lagrangeEval（2~8 个互异节点、全部有限）。
func nevilleEval(xs, ys []float64, xEval float64) (float64, error) {
	if _, err := lagrangeEval(xs, ys, xEval); err != nil {
		return 0, err // 同一套节点合法性校验
	}
	n := len(xs)
	// col[k] 暂存第 k 列：先填第 0 列 y_i，再逐列推进（就地覆盖即可，
	// 每列只用本列与上一列当前行）。
	col := make([]float64, n)
	copy(col, ys)
	for k := 1; k < n; k++ {
		for i := n - 1; i >= k; i-- {
			col[i] = ((xEval-xs[i-k])*col[i] - (xEval-xs[i])*col[i-1]) / (xs[i] - xs[i-k])
		}
	}
	if !isFinite(col[n-1]) {
		return 0, fmt.Errorf("递推结果非有限值")
	}
	return col[n-1], nil
}

// scoreAitkenInterp 逐次线性插值法评分（Aitken/Neville 递推）。
//
// target:     {"method":"aitken_rel_err","problem_key":"sin_quad","x_eval":0.5236,"pass_score":60}
// readings:   [{"value":0.5174,"col1":[0.6667,0.3333]}]
//
//	value 必填（= 递推终值 P_{n−1,n−1}，即插值多项式在 x_eval 处的值）；
//	col1 可选（学生算的第 1 列：相邻节点线性插值结果 P_{i,1}，长度须为 n−1，
//	仅记入审计 detail，不参与打分）
//
// user_config 同 L6：problem_key（换题）/ x_eval（换求值点）/ 自定义 nodes_x+nodes_y+x_eval
//
// 评分 = 插值值精度分：真值由后端按 target/user_config 节点用 Neville 递推现算
// （题库与 L6 共用 interpProblems，真值与 L6 数学上相等），按 |value−P|/|P| 相对
// 误差走默认曲线。龙格题抄原函数值同样 0 分。求值点归属规则同 v0.12.1（target.x_eval
// 不跨题沿用）。
func scoreAitkenInterp(readings, target, userConfig []byte) (score int, passed bool, detail string, err error) {
	var tgt struct {
		ProblemKey string          `json:"problem_key"`
		XEval      *float64        `json:"x_eval"`
		PassScore  int             `json:"pass_score"`
		Bands      json.RawMessage `json:"bands"`
	}
	if err = json.Unmarshal(target, &tgt); err != nil {
		err = fmt.Errorf("target 解析失败: %w", err)
		return
	}
	var bands []relErrBand
	if bands, err = parseRelErrBands(tgt.Bands); err != nil {
		return
	}
	var uc struct {
		ProblemKey string    `json:"problem_key"`
		NodesX     []float64 `json:"nodes_x"`
		NodesY     []float64 `json:"nodes_y"`
		XEval      *float64  `json:"x_eval"`
	}
	_ = json.Unmarshal(userConfig, &uc) // 可选字段，缺省/为空时忽略

	xs, ys := []float64(nil), []float64(nil)
	probKey := tgt.ProblemKey
	defXEval := math.NaN() // 库内题的默认求值点（custom 题无默认，须显式给出）
	switch {
	case len(uc.NodesX) > 0 || len(uc.NodesY) > 0:
		probKey = "custom"
		xs, ys = uc.NodesX, uc.NodesY
	case uc.ProblemKey != "":
		probKey = uc.ProblemKey
	}
	if xs == nil {
		def, ok := interpProblems[probKey]
		if !ok {
			err = fmt.Errorf("未知插值题: %s", probKey)
			return
		}
		xs, ys = def.Xs, def.Ys
		defXEval = def.XEval
	}
	// 求值点归属（v0.12.1 规则同款）：user_config >（仅未换题时）target > 库内默认
	xEval := defXEval
	if probKey == tgt.ProblemKey && tgt.XEval != nil {
		xEval = *tgt.XEval
	}
	if uc.XEval != nil {
		xEval = *uc.XEval
	}
	if math.IsNaN(xEval) {
		err = fmt.Errorf("自定义节点须同时给出 x_eval")
		return
	}

	trueP, perr := nevilleEval(xs, ys, xEval)
	if perr != nil {
		err = perr
		return
	}

	var rs []struct {
		Value *float64  `json:"value"`
		Col1  []float64 `json:"col1"`
	}
	if err = json.Unmarshal(readings, &rs); err != nil {
		err = fmt.Errorf("readings 解析失败: %w", err)
		return
	}
	r0 := rs[0]
	if r0.Value == nil {
		err = fmt.Errorf("readings[0] 缺少 value 字段")
		return
	}
	value := *r0.Value
	if !isFinite(value) {
		err = fmt.Errorf("value 不能是 NaN/Inf")
		return
	}
	// col1 可选：长度须为 n−1、数值有限（仅审计记录用，错维度直接 400 暴露）
	if r0.Col1 != nil {
		if len(r0.Col1) != len(xs)-1 {
			err = fmt.Errorf("col1 长度须为 %d（节点数−1），got %d", len(xs)-1, len(r0.Col1))
			return
		}
		for i, v := range r0.Col1 {
			if !isFinite(v) {
				err = fmt.Errorf("col1[%d] 不能是 NaN/Inf", i)
				return
			}
		}
	}

	relErr := relError(value, trueP)
	score = scoreFromRelErrBands(relErr, bands)
	passed = passedByScore(score, tgt.PassScore)
	detail = fmt.Sprintf("problem=%s n=%d x_eval=%.6g value=%.10g rel_err=%.3g true_P=%.10g",
		probKey, len(xs), xEval, value, relErr, trueP)
	if r0.Col1 != nil {
		detail += fmt.Sprintf(" col1=[%s]", joinGauss(r0.Col1))
	}
	return
}

// ---------- L19 龙贝格积分法（Romberg） ----------

// rombergTColumn 逐次减半的复合梯形列 T_0..T_k：
//
//	T_0 = (b−a)/2·[f(a)+f(b)]（整区间梯形）
//	T_m = T_{m−1}/2 + h_m·Σ f(a+(2i−1)·h_m)，h_m = (b−a)/2^m
//
// 每层只新增上一层分点中间的奇数位节点（f 值不重复计算），2^k 段梯形。
func rombergTColumn(f func(x float64) float64, a, b float64, k int) []float64 {
	t := make([]float64, k+1)
	t[0] = (b - a) * (f(a) + f(b)) / 2
	for m := 1; m <= k; m++ {
		steps := 1 << uint(m) // 2^m 段
		h := (b - a) / float64(steps)
		var s float64
		for i := 1; i < steps; i += 2 {
			s += f(a + float64(i)*h)
		}
		t[m] = t[m-1]/2 + h*s
	}
	return t
}

// rombergEval 龙贝格积分：T 列逐次减半后做 k 轮理查逊外推
//
//	T_i^(j) = (4^j·T_i^(j−1) − T_{i−1}^(j−1)) / (4^j − 1)
//
// 原地自下而上收缩（i 自 k 降到 j），返回终值 T_k^(k)。
// 外推第 1 列恰为复合辛普森 S_{2^i}（与 L17 同一公式，单测交叉验证），
// 第 2 列起为更高阶的值；光滑函数上终值以 2(2k+2) 阶逼近精确积分。
func rombergEval(f func(x float64) float64, a, b float64, k int) float64 {
	t := rombergTColumn(f, a, b, k)
	for j := 1; j <= k; j++ {
		factor := math.Pow(4, float64(j))
		for i := k; i >= j; i-- {
			t[i] = (factor*t[i] - t[i-1]) / (factor - 1)
		}
	}
	return t[k]
}

// scoreRombergIntegration 龙贝格积分评分。
//
// target:     {"method":"romberg_rel_err","problem_key":"sin_pi","k":2,"pass_score":60}
// readings:   [{"value":1.9985707,"t_col":[0,1.5707963,1.8961189]}]
//
//	value 必填（= 学生递推出的终值 T_k^(k)）；t_col 可选
//	（学生算的梯形列 T_0..T_k，长度 k+1，仅记入审计 detail，不参与打分）
//
// user_config 可覆盖 problem_key（换题）/ a+b（换区间，须成对给出）/ k（换外推深度）
//
// 评分 = 龙贝格终值精度分：真值 T_k^(k) 由后端按同一 f、区间、k 现算（真值=
// "方法的输出"而非精确积分 I，同 L17/L18 设计），按 |value−T_k^(k)|/|T_k^(k)|
// 相对误差走默认曲线（target.bands 可覆盖）。题库复用 L17 的 integProblems：
// sin_pi 默认 k=2 时终值 ≈ 1.998571，外推第 1 列 S_1=2π/3 正是 L17 的 S_2——
// 三列龙贝格表 = "梯形递推 + 辛普森 + 更高阶"一条链。抄精确积分 I=2 得 ~81 分
// （能过线但损失精度分：龙贝格收敛快，抄答案与算对只差 0.07% 截断误差；
// 抄 L17 答案 2π/3 或抄最后一行梯形 T_2 均 ~41 分不过线）；过程是否真做
// 由 t_col 审计暴露。k 须为 1~10。target 的题专属参数（a/b/k）不跨题沿用
// （v0.12.1 求值点归属规则同款）。
func scoreRombergIntegration(readings, target, userConfig []byte) (score int, passed bool, detail string, err error) {
	var tgt struct {
		ProblemKey string          `json:"problem_key"`
		A          *float64        `json:"a"`
		B          *float64        `json:"b"`
		K          *int            `json:"k"`
		PassScore  int             `json:"pass_score"`
		Bands      json.RawMessage `json:"bands"`
	}
	if err = json.Unmarshal(target, &tgt); err != nil {
		err = fmt.Errorf("target 解析失败: %w", err)
		return
	}
	var bands []relErrBand
	if bands, err = parseRelErrBands(tgt.Bands); err != nil {
		return
	}
	// v0.5 模式：user_config 覆盖（problem_key / a+b / k）
	var uc struct {
		ProblemKey string   `json:"problem_key"`
		A          *float64 `json:"a"`
		B          *float64 `json:"b"`
		K          *int     `json:"k"`
	}
	_ = json.Unmarshal(userConfig, &uc) // 可选字段，缺省/为空时忽略

	probKey := tgt.ProblemKey
	if uc.ProblemKey != "" {
		probKey = uc.ProblemKey
	}
	def, ok := integProblems[probKey]
	if !ok {
		err = fmt.Errorf("未知积分题: %s", probKey)
		return
	}

	// 区间归属：user_config.a+b（须成对）>（仅未换题时）target.a+b > 库内默认。
	// target 的区间属于 target 自己的题（v0.12.1 求值点归属规则同款），
	// 换题后回落新题库内默认区间。
	a, b := def.A, def.B
	sameProblem := probKey == tgt.ProblemKey
	if sameProblem && tgt.A != nil && tgt.B != nil {
		a, b = *tgt.A, *tgt.B
	}
	if uc.A != nil || uc.B != nil {
		if uc.A == nil || uc.B == nil {
			err = fmt.Errorf("user_config 区间须成对给出 a、b")
			return
		}
		a, b = *uc.A, *uc.B
	}
	if !isFinite(a) || !isFinite(b) || a >= b {
		err = fmt.Errorf("积分区间非法（须 a < b 且有限）: [%.6g, %.6g]", a, b)
		return
	}

	// 外推深度归属：user_config.k >（仅未换题时）target.k > 库内默认 2。
	k := 2
	if sameProblem && tgt.K != nil {
		k = *tgt.K
	}
	if uc.K != nil {
		k = *uc.K
	}
	if k < 1 || k > 10 {
		err = fmt.Errorf("k 须为 1~10 的外推深度，got %d", k)
		return
	}

	trueR := rombergEval(def.F, a, b, k)
	if !isFinite(trueR) {
		err = fmt.Errorf("龙贝格真值非有限值（区间 [%g,%g] 内 f 无界？）", a, b)
		return
	}

	var rs []struct {
		Value *float64  `json:"value"`
		TCol  []float64 `json:"t_col"`
	}
	if err = json.Unmarshal(readings, &rs); err != nil {
		err = fmt.Errorf("readings 解析失败: %w", err)
		return
	}
	r0 := rs[0]
	if r0.Value == nil {
		err = fmt.Errorf("readings[0] 缺少 value 字段")
		return
	}
	value := *r0.Value
	if !isFinite(value) {
		err = fmt.Errorf("value 不能是 NaN/Inf")
		return
	}
	// t_col 可选：长度须为 k+1、数值有限（仅审计记录用，错维度直接 400 暴露）
	if r0.TCol != nil {
		if len(r0.TCol) != k+1 {
			err = fmt.Errorf("t_col 长度须为 %d（k+1），got %d", k+1, len(r0.TCol))
			return
		}
		for i, v := range r0.TCol {
			if !isFinite(v) {
				err = fmt.Errorf("t_col[%d] 不能是 NaN/Inf", i)
				return
			}
		}
	}

	relErr := relError(value, trueR)
	score = scoreFromRelErrBands(relErr, bands)
	passed = passedByScore(score, tgt.PassScore)
	detail = fmt.Sprintf("problem=%s a=%.6g b=%.6g k=%d value=%.10g rel_err=%.3g true_R=%.10g ref_I=%.10g",
		probKey, a, b, k, value, relErr, trueR, simpsonRef(def.F, a, b))
	if r0.TCol != nil {
		detail += fmt.Sprintf(" t_col=[%s]", joinGauss(r0.TCol))
	}
	return
}

// ========== L20/L21 欧拉法（常微分方程初值问题数值解） ==========

// odeProblemDef 常微分方程初值问题题库：y' = f(x, y)，y(a) = y0，求 y(b)。
// F 为右端函数；Exact 为精确解闭式（记审计 ref_Y 展示截断误差，不参与打分）。
type odeProblemDef struct {
	F     func(x, y float64) float64
	A, B  float64
	Y0    float64
	N     int                   // 库内默认步数（h=(b−a)/n）
	Desc  string                // 精确解的文字说明（供审计/前端展示）
	Exact func(x float64) float64
}

// odeProblems 常微分方程初值问题题库（前后端逐字同步，共 3 题，均 [0,1] h=0.25）：
//
//	exp_growth：y'=y，y(0)=1，精确解 e^x。显式欧拉 n=4 时 (1+h)^n=1.25^4
//	  =2.44140625，抄精确解 e 得 ~28 分不过线（O(h) 截断误差 11% 拦截）；
//	  改进欧拉每步因子 1+h+h²/2（n=2 默认 1.625²=2.640625，抄 e ~49 分不过线）。
//	decay_sq：y'=−2xy，y(0)=1，精确解 e^{−x²}（与 L17 erf_gauss 同源被积函数）。
//	  f 显含 x 与 y；显式 n=4 手算全为 2 的幂分数（y4=0.41015625），
//	  抄精确解 ~29 分不过线；改进 n=4 y4≈0.374158，抄精确解 ~57 分不过线。
//	stiff_decay：y'=−10y，y(0)=1，精确解 e^{−10x}。刚性问题观察题：h=0.25 时
//	  hλ=−2.5，显式欧拉 |1+hλ|=1.5>1、改进欧拉 |1+hλ+(hλ)²/2|=1.625>1 均在
//	  绝对稳定区间外，解振荡发散（真值=方法输出仍确定：显式 (−1.5)^4=5.0625、
//	  改进 1.625^4=6.972900390625）——抄精确解 e^{-10}≈4.5e-5 直接 0 分，
//	  抄另一方法的答案也因数值差异大被拦。
var odeProblems = map[string]odeProblemDef{
	"exp_growth": {
		F: func(x, y float64) float64 { return y },
		A: 0, B: 1, Y0: 1, N: 4,
		Desc:  "y = e^x",
		Exact: math.Exp,
	},
	"decay_sq": {
		F: func(x, y float64) float64 { return -2 * x * y },
		A: 0, B: 1, Y0: 1, N: 4,
		Desc:  "y = e^{-x^2}",
		Exact: func(x float64) float64 { return math.Exp(-x * x) },
	},
	"stiff_decay": {
		F: func(x, y float64) float64 { return -10 * y },
		A: 0, B: 1, Y0: 1, N: 4,
		Desc:  "y = e^{-10x}",
		Exact: func(x float64) float64 { return math.Exp(-10 * x) },
	},
}

// eulerExplicit 显式（前进）欧拉：y_{n+1} = y_n + h·f(x_n, y_n)。
// 返回整列 y_0..y_n（h=(b−a)/n），末元素即提交值。
func eulerExplicit(f func(x, y float64) float64, a, b, y0 float64, n int) []float64 {
	ys := make([]float64, n+1)
	ys[0] = y0
	h := (b - a) / float64(n)
	for i := 0; i < n; i++ {
		x := a + float64(i)*h
		ys[i+1] = ys[i] + h*f(x, ys[i])
	}
	return ys
}

// eulerImproved 改进欧拉（预测-校正 / Heun）：
//
//	预测 y_p = y_n + h·f(x_n, y_n)          （显式欧拉探一步）
//	校正 y_{n+1} = y_n + h/2·[f(x_n,y_n) + f(x_{n+1},y_p)]
//
// 即单步梯形公式取显式欧拉做预测，局部截断误差 O(h²)。
func eulerImproved(f func(x, y float64) float64, a, b, y0 float64, n int) []float64 {
	ys := make([]float64, n+1)
	ys[0] = y0
	h := (b - a) / float64(n)
	for i := 0; i < n; i++ {
		x := a + float64(i)*h
		yp := ys[i] + h*f(x, ys[i])
		ys[i+1] = ys[i] + h/2*(f(x, ys[i])+f(x+h, yp))
	}
	return ys
}

// eulerImplicit 隐式（后退）欧拉：y_{n+1} = y_n + h·f(x_{n+1}, y_{n+1})。
// 未知量 y_{n+1} 同时出现在等式两侧，须解隐式方程——题库 odeProblems 均为
// 线性齐次 y' = a(x)·y（即 f(x,y) = y·f(x,1)），代入后闭式解出
//
//	y_{n+1} = y_n / (1 − h·a(x_{n+1}))，a(x) = f(x, 1)
//
// 无须迭代（非线性 f 才需 Newton/不动点）。稳定性函数 R(hλ) = 1/(1−hλ)：
// 绝对稳定域为 {z : |1−z| ≥ 1}——包含整个左半平面（A 稳定），与显式欧拉
// 的圆盘 |1+z| ≤ 1 正好互补，刚性题 hλ=−2.5 下 |R|=2/7<1 单调收敛不发散。
// 返回整列 y_0..y_n，末元素即提交值。
func eulerImplicit(f func(x, y float64) float64, a, b, y0 float64, n int) []float64 {
	ys := make([]float64, n+1)
	ys[0] = y0
	h := (b - a) / float64(n)
	for i := 0; i < n; i++ {
		x1 := a + float64(i+1)*h
		ys[i+1] = ys[i] / (1 - h*f(x1, 1))
	}
	return ys
}

// scoreEulerExplicit 显式欧拉法评分（L20）。
//
// target:     {"method":"euler_rel_err","problem_key":"exp_growth","n":4,"pass_score":60}
// readings:   [{"value":2.44140625,"y_col":[1,1.25,1.5625,1.953125,2.44140625]}]
//
//	value 必填（= 学生递推出的终值 y_N）；y_col 可选（学生算的逐点列
//	y_0..y_N，长度须 n+1，仅记入审计 detail，不参与打分）
//
// user_config 可覆盖 problem_key（换题）/ a+b（换区间，须成对给出）/ n（换步数）
//
// 评分 = 欧拉终值精度分：真值 y_N 由后端按同一 f、区间、步长现算（真值="方法
// 的输出"而非精确解，同 L17/L19 设计），按 |value−y_N|/|y_N| 相对误差走默认
// 曲线（target.bands 可覆盖）。题库 odeProblems 三题抄精确解均被 O(h) 截断
// 误差拦住；n 须为 1~1024。target 的题专属参数（a/b/n）不跨题沿用
// （v0.12.1 求值点归属规则同款）。
func scoreEulerExplicit(readings, target, userConfig []byte) (score int, passed bool, detail string, err error) {
	return scoreEulerODE(readings, target, userConfig, eulerExplicitV)
}

// scoreEulerImproved 改进欧拉法评分（L21）。语义与 L20 完全一致，仅真值
// 由 eulerImproved（预测-校正）现算；target 默认 exp_growth n=2（两轮
// 预测-校正）：抄精确解 e ~49 分、抄显式欧拉 L20 答案 ~34 分均不过线。
func scoreEulerImproved(readings, target, userConfig []byte) (score int, passed bool, detail string, err error) {
	return scoreEulerODE(readings, target, userConfig, eulerImprovedV)
}

// eulerVariant 欧拉族变体：显式（前进）/ 改进（预测-校正）/ 隐式（后退）。
type eulerVariant int

const (
	eulerExplicitV eulerVariant = iota
	eulerImprovedV
	eulerImplicitV
)

// scoreEulerImplicit 隐式（后退）欧拉法评分（L26）。语义与 L20/L21 完全一致，
// 仅真值由 eulerImplicit 现算；target 默认 exp_growth n=4：y'=y 时每步除以
// (1−h)，真值闭式 (4/3)^4 = 256/81 ≈ 3.160494——隐式欧拉单向低估（数值从
// 上方压向精确解 e，比显式的 2.44 离 e 更远），抄精确解 e ~24 分、抄 L20 显式
// 答案 ~15 分、抄 L21 改进欧拉 ~21 分、抄 L22 RK4/L23 阿当姆斯 ~23/24 分均
// 不过线；stiff_decay 上隐式 A 稳定收敛（真值 16/2401 ≈ 0.006664，与 L20 显式
// 发散值 5.0625 正面对照），抄精确解 e^{−10} 或抄显式发散值均 0 分。
func scoreEulerImplicit(readings, target, userConfig []byte) (score int, passed bool, detail string, err error) {
	return scoreEulerODE(readings, target, userConfig, eulerImplicitV)
}

// scoreEulerODE L20/L21/L26 共用评分实现（v 区分显式/改进/隐式欧拉真值）。
func scoreEulerODE(readings, target, userConfig []byte, v eulerVariant) (score int, passed bool, detail string, err error) {
	var tgt struct {
		ProblemKey string          `json:"problem_key"`
		A          *float64        `json:"a"`
		B          *float64        `json:"b"`
		N          *int            `json:"n"`
		PassScore  int             `json:"pass_score"`
		Bands      json.RawMessage `json:"bands"`
	}
	if err = json.Unmarshal(target, &tgt); err != nil {
		err = fmt.Errorf("target 解析失败: %w", err)
		return
	}
	var bands []relErrBand
	if bands, err = parseRelErrBands(tgt.Bands); err != nil {
		return
	}
	// v0.5 模式：user_config 覆盖（problem_key / a+b / n）
	var uc struct {
		ProblemKey string   `json:"problem_key"`
		A          *float64 `json:"a"`
		B          *float64 `json:"b"`
		N          *int     `json:"n"`
	}
	_ = json.Unmarshal(userConfig, &uc) // 可选字段，缺省/为空时忽略

	probKey := tgt.ProblemKey
	if uc.ProblemKey != "" {
		probKey = uc.ProblemKey
	}
	def, ok := odeProblems[probKey]
	if !ok {
		err = fmt.Errorf("未知常微分方程题: %s", probKey)
		return
	}

	// 区间归属：user_config.a+b（须成对）>（仅未换题时）target.a+b > 库内默认。
	// 换题后回落新题库内默认区间（v0.12.1 求值点归属规则同款）。
	a, b := def.A, def.B
	sameProblem := probKey == tgt.ProblemKey
	if sameProblem && tgt.A != nil && tgt.B != nil {
		a, b = *tgt.A, *tgt.B
	}
	if uc.A != nil || uc.B != nil {
		if uc.A == nil || uc.B == nil {
			err = fmt.Errorf("user_config 区间须成对给出 a、b")
			return
		}
		a, b = *uc.A, *uc.B
	}
	if !isFinite(a) || !isFinite(b) || a >= b {
		err = fmt.Errorf("积分区间非法（须 a < b 且有限）: [%.6g, %.6g]", a, b)
		return
	}

	// 步数归属：user_config.n >（仅未换题时）target.n > 库内默认 4。
	n := def.N
	if sameProblem && tgt.N != nil {
		n = *tgt.N
	}
	if uc.N != nil {
		n = *uc.N
	}
	if n < 1 || n > 1024 {
		err = fmt.Errorf("n 须为 1~1024 的步数，got %d", n)
		return
	}

	// 真值 = 方法输出：后端按同一 f、初值、区间、步长现算欧拉列。
	var ys []float64
	switch v {
	case eulerImprovedV:
		ys = eulerImproved(def.F, a, b, def.Y0, n)
	case eulerImplicitV:
		ys = eulerImplicit(def.F, a, b, def.Y0, n)
	default:
		ys = eulerExplicit(def.F, a, b, def.Y0, n)
	}
	trueY := ys[n]
	if !isFinite(trueY) {
		err = fmt.Errorf("欧拉真值非有限值（区间 [%.6g, %.6g] 内发散？）", a, b)
		return
	}

	var rs []struct {
		Value *float64  `json:"value"`
		YCol  []float64 `json:"y_col"`
	}
	if err = json.Unmarshal(readings, &rs); err != nil {
		err = fmt.Errorf("readings 解析失败: %w", err)
		return
	}
	r0 := rs[0]
	if r0.Value == nil {
		err = fmt.Errorf("readings[0] 缺少 value 字段")
		return
	}
	value := *r0.Value
	if !isFinite(value) {
		err = fmt.Errorf("value 不能是 NaN/Inf")
		return
	}
	// y_col 可选：长度须为 n+1、数值有限（仅审计记录用，错维度直接 400 暴露）
	if r0.YCol != nil {
		if len(r0.YCol) != n+1 {
			err = fmt.Errorf("y_col 长度须为 %d（n+1），got %d", n+1, len(r0.YCol))
			return
		}
		for i, v := range r0.YCol {
			if !isFinite(v) {
				err = fmt.Errorf("y_col[%d] 不能是 NaN/Inf", i)
				return
			}
		}
	}

	relErr := relError(value, trueY)
	score = scoreFromRelErrBands(relErr, bands)
	passed = passedByScore(score, tgt.PassScore)
	method := "显式"
	switch v {
	case eulerImprovedV:
		method = "改进"
	case eulerImplicitV:
		method = "隐式"
	}
	detail = fmt.Sprintf("problem=%s a=%.6g b=%.6g n=%d value=%.10g rel_err=%.3g true_Y=%.10g ref_Y=%.10g（%s欧拉，精确解 %s）",
		probKey, a, b, n, value, relErr, trueY, def.Exact(b), method, def.Desc)
	if r0.YCol != nil {
		detail += fmt.Sprintf(" y_col=[%s]", joinGauss(r0.YCol))
	}
	return
}

// rk4Column 经典四阶 Runge-Kutta（RK4）列 y_0..y_N（h=(b−a)/n）：
//
//	K1 = f(x_n, y_n)
//	K2 = f(x_n + h/2, y_n + h/2·K1)
//	K3 = f(x_n + h/2, y_n + h/2·K2)
//	K4 = f(x_n + h,   y_n + h·K3)
//	y_{n+1} = y_n + h/6·(K1 + 2K2 + 2K3 + K4)
//
// 局部截断误差 O(h⁵)、全局 O(h⁴)——单步法精度的行业标准。
// 返回整列 y_0..y_N，末元素即提交值。
func rk4Column(f func(x, y float64) float64, a, b, y0 float64, n int) []float64 {
	ys := make([]float64, n+1)
	ys[0] = y0
	h := (b - a) / float64(n)
	for i := 0; i < n; i++ {
		x := a + float64(i)*h
		k1 := f(x, ys[i])
		k2 := f(x+h/2, ys[i]+h/2*k1)
		k3 := f(x+h/2, ys[i]+h/2*k2)
		k4 := f(x+h, ys[i]+h*k3)
		ys[i+1] = ys[i] + h/6*(k1+2*k2+2*k3+k4)
	}
	return ys
}

// scoreRungeKuttaODE 四阶龙格-库塔法评分（L22）。
//
// target:     {"method":"rk4_rel_err","problem_key":"exp_growth","n":1,"pass_score":60}
// readings:   [{"value":2.7083333333333335,"y_col":[1,2.7083333333333335]}]
//
//	value 必填（= 学生递推出的终值 y_N）；y_col 可选（学生算的逐点列
//	y_0..y_N，长度须 n+1，仅记入审计 detail，不参与打分）
//
// user_config 可覆盖 problem_key（换题）/ a+b（换区间，须成对给出）/ n（换步数）
//
// 评分语义与 L20/L21 完全一致：真值 y_N 由后端按同一 f、初值、区间、步长用
// rk4Column 现算（"真值=方法输出"，同 L7/L9/L20/L21 设计），按
// |value−y_N|/|y_N| 相对误差走默认曲线（target.bands 可覆盖）；题库共用
// odeProblems，题专属参数（a/b/n）归属规则同 v0.12.1。
//
// target 默认 exp_growth n=1（单步四阶：K1..K4 = 1、1.5、1.75、2.75 全手算，
// 终值恰 65/24≈2.708333）。防作弊语义（与 L20/L21 精度阶对照）：抄精确解 e
// 约 72 分（RK4 收敛快，能过线但非满分——过程真伪由 y_col 审计暴露，L19 同款
// 语义）；抄 L21 改进欧拉答案 2.640625 约 51 分、抄 L20 显式答案 2.44140625
// 约 30 分均不过线；stiff_decay 单步即发散（|R(−10)|=291>1），抄精确解
// e^{−10} 与抄 L20/L21 的发散值均直接 0 分。
func scoreRungeKuttaODE(readings, target, userConfig []byte) (score int, passed bool, detail string, err error) {
	var tgt struct {
		ProblemKey string          `json:"problem_key"`
		A          *float64        `json:"a"`
		B          *float64        `json:"b"`
		N          *int            `json:"n"`
		PassScore  int             `json:"pass_score"`
		Bands      json.RawMessage `json:"bands"`
	}
	if err = json.Unmarshal(target, &tgt); err != nil {
		err = fmt.Errorf("target 解析失败: %w", err)
		return
	}
	var bands []relErrBand
	if bands, err = parseRelErrBands(tgt.Bands); err != nil {
		return
	}
	// v0.5 模式：user_config 覆盖（problem_key / a+b / n）
	var uc struct {
		ProblemKey string   `json:"problem_key"`
		A          *float64 `json:"a"`
		B          *float64 `json:"b"`
		N          *int     `json:"n"`
	}
	_ = json.Unmarshal(userConfig, &uc) // 可选字段，缺省/为空时忽略

	probKey := tgt.ProblemKey
	if uc.ProblemKey != "" {
		probKey = uc.ProblemKey
	}
	def, ok := odeProblems[probKey]
	if !ok {
		err = fmt.Errorf("未知常微分方程题: %s", probKey)
		return
	}

	// 区间归属：user_config.a+b（须成对）>（仅未换题时）target.a+b > 库内默认。
	// 换题后回落新题库内默认区间（v0.12.1 求值点归属规则同款）。
	sameProblem := probKey == tgt.ProblemKey
	a, b := def.A, def.B
	if sameProblem && tgt.A != nil && tgt.B != nil {
		a, b = *tgt.A, *tgt.B
	}
	if uc.A != nil || uc.B != nil {
		if uc.A == nil || uc.B == nil {
			err = fmt.Errorf("user_config 区间须成对给出 a、b")
			return
		}
		a, b = *uc.A, *uc.B
	}
	if !isFinite(a) || !isFinite(b) || a >= b {
		err = fmt.Errorf("积分区间非法（须 a < b 且有限）: [%.6g, %.6g]", a, b)
		return
	}

	// 步数归属：user_config.n >（仅未换题时）target.n > 库内默认 4。
	n := def.N
	if sameProblem && tgt.N != nil {
		n = *tgt.N
	}
	if uc.N != nil {
		n = *uc.N
	}
	if n < 1 || n > 1024 {
		err = fmt.Errorf("n 须为 1~1024 的步数，got %d", n)
		return
	}

	// 真值 = 方法输出：后端按同一 f、初值、区间、步长现算 RK4 列。
	ys := rk4Column(def.F, a, b, def.Y0, n)
	trueY := ys[n]
	if !isFinite(trueY) {
		err = fmt.Errorf("RK4 真值非有限值（区间 [%.6g, %.6g] 内发散？）", a, b)
		return
	}

	var rs []struct {
		Value *float64  `json:"value"`
		YCol  []float64 `json:"y_col"`
	}
	if err = json.Unmarshal(readings, &rs); err != nil {
		err = fmt.Errorf("readings 解析失败: %w", err)
		return
	}
	r0 := rs[0]
	if r0.Value == nil {
		err = fmt.Errorf("readings[0] 缺少 value 字段")
		return
	}
	value := *r0.Value
	if !isFinite(value) {
		err = fmt.Errorf("value 不能是 NaN/Inf")
		return
	}
	// y_col 可选：长度须为 n+1、数值有限（仅审计记录用，错维度直接 400 暴露）
	if r0.YCol != nil {
		if len(r0.YCol) != n+1 {
			err = fmt.Errorf("y_col 长度须为 %d（n+1），got %d", n+1, len(r0.YCol))
			return
		}
		for i, v := range r0.YCol {
			if !isFinite(v) {
				err = fmt.Errorf("y_col[%d] 不能是 NaN/Inf", i)
				return
			}
		}
	}

	relErr := relError(value, trueY)
	score = scoreFromRelErrBands(relErr, bands)
	passed = passedByScore(score, tgt.PassScore)
	detail = fmt.Sprintf("problem=%s a=%.6g b=%.6g n=%d value=%.10g rel_err=%.3g true_Y=%.10g ref_Y=%.10g（RK4 四阶龙格-库塔，精确解 %s）",
		probKey, a, b, n, value, relErr, trueY, def.Exact(b), def.Desc)
	if r0.YCol != nil {
		detail += fmt.Sprintf(" y_col=[%s]", joinGauss(r0.YCol))
	}
	return
}

// adamsPCColumn 阿当姆斯预报-校正（L23）列 y_0..y_N：线性多步法 AB4+AM4（PECE），
// 表头 y_1..y_3 用 RK4 启动（与 L22 同式，保证表头同阶精度）：
//
//	预报（AB4 外推）y_{n+1}^p = y_n + h/24·(55f_n − 59f_{n-1} + 37f_{n-2} − 9f_{n-3})
//	校正（AM4 内插）y_{n+1}   = y_n + h/24·(9f(x_{n+1}, y^p) + 19f_n − 5f_{n-1} + f_{n-2})
//
// 每步只校正一次（PECE 模式，教材标准做法），全局截断误差 O(h⁴)。
// n 须 ≥ 4（3 步表头 + 至少 1 步阿当姆斯）。
func adamsPCColumn(f func(x, y float64) float64, a, b, y0 float64, n int) []float64 {
	ys := make([]float64, n+1)
	ys[0] = y0
	h := (b - a) / float64(n)
	for i := 0; i < n; i++ {
		x := a + float64(i)*h
		if i < 3 {
			// 表头：RK4 启动（与 rk4Column 同式）
			k1 := f(x, ys[i])
			k2 := f(x+h/2, ys[i]+h/2*k1)
			k3 := f(x+h/2, ys[i]+h/2*k2)
			k4 := f(x+h, ys[i]+h*k3)
			ys[i+1] = ys[i] + h/6*(k1+2*k2+2*k3+k4)
			continue
		}
		// AB4 预报（显式外推，用前 4 点斜率）
		yp := ys[i] + h/24*(55*f(x, ys[i]) -
			59*f(x-h, ys[i-1]) +
			37*f(x-2*h, ys[i-2]) -
			9*f(x-3*h, ys[i-3]))
		// AM4 校正（隐式内插，用预报值代 f 在新点的斜率，只校一次）
		ys[i+1] = ys[i] + h/24*(9*f(x+h, yp) +
			19*f(x, ys[i]) -
			5*f(x-h, ys[i-1]) +
			f(x-2*h, ys[i-2]))
	}
	return ys
}

// scoreAdamsPCODE 阿当姆斯预报-校正法评分（L23）。
//
// target:     {"method":"adams_rel_err","problem_key":"exp_growth","n":4,"pass_score":60}
// readings:   [{"value":2.718225983274504,"y_col":[1,1.2840169271,1.648699469,2.1169580259,2.7182259833]}]
//
//	value 必填（= 学生递推出的终值 y_N）；y_col 可选（学生算的逐点列
//	y_0..y_N，长度须 n+1，仅记入审计 detail，不参与打分）
//
// user_config 可覆盖 problem_key（换题）/ a+b（换区间，须成对给出）/ n（换步数，4~1024）
//
// 评分语义与 L20/L21/L22 完全一致：真值 y_N 由后端按同一 f、初值、区间、步长用
// adamsPCColumn 现算（"真值=方法输出"），按 |value−y_N|/|y_N| 相对误差走默认曲线
// （target.bands 可覆盖）；题库共用 odeProblems，题专属参数（a/b/n）归属规则
// 同 v0.12.1。n 须 4~1024（3 步 RK4 表头 + 至少 1 步阿当姆斯）。
//
// target 默认 exp_growth n=4（3 步 RK4 表头 + 1 步 AB4-AM4，经典手算规模：
// y_1..y_3 = R(1/4)^i，R(h)=1+h+h²/2+h³/6+h⁴/24，终值 2.7182259833）。
// 防作弊语义（与 L20-L22 精度阶对照）：四阶方法收敛快，抄精确解 e 约 93 分、
// 抄 L22 默认答案 65/24≈2.708333 约 72 分，均能过线但非满分——同为四阶法
// 数学上难分（过程真伪由 y_col 审计暴露，L19/L22 同款语义）；抄 L21 改进欧拉
// 答案 2.640625 约 49 分、抄 L20 显式答案 2.44140625 约 30 分均不过线；
// stiff_decay（hλ=−2.5）多步法绝对稳定区间远小于 RK4，抄精确解 e^{−10}
// 直接 0 分、抄 RK4 n=4 答案 0.1768 约 9 分均被拦。
func scoreAdamsPCODE(readings, target, userConfig []byte) (score int, passed bool, detail string, err error) {
	var tgt struct {
		ProblemKey string          `json:"problem_key"`
		A          *float64        `json:"a"`
		B          *float64        `json:"b"`
		N          *int            `json:"n"`
		PassScore  int             `json:"pass_score"`
		Bands      json.RawMessage `json:"bands"`
	}
	if err = json.Unmarshal(target, &tgt); err != nil {
		err = fmt.Errorf("target 解析失败: %w", err)
		return
	}
	var bands []relErrBand
	if bands, err = parseRelErrBands(tgt.Bands); err != nil {
		return
	}
	// v0.5 模式：user_config 覆盖（problem_key / a+b / n）
	var uc struct {
		ProblemKey string   `json:"problem_key"`
		A          *float64 `json:"a"`
		B          *float64 `json:"b"`
		N          *int     `json:"n"`
	}
	_ = json.Unmarshal(userConfig, &uc) // 可选字段，缺省/为空时忽略

	probKey := tgt.ProblemKey
	if uc.ProblemKey != "" {
		probKey = uc.ProblemKey
	}
	def, ok := odeProblems[probKey]
	if !ok {
		err = fmt.Errorf("未知常微分方程题: %s", probKey)
		return
	}

	// 区间归属：user_config.a+b（须成对）>（仅未换题时）target.a+b > 库内默认。
	// 换题后回落新题库内默认区间（v0.12.1 求值点归属规则同款）。
	sameProblem := probKey == tgt.ProblemKey
	a, b := def.A, def.B
	if sameProblem && tgt.A != nil && tgt.B != nil {
		a, b = *tgt.A, *tgt.B
	}
	if uc.A != nil || uc.B != nil {
		if uc.A == nil || uc.B == nil {
			err = fmt.Errorf("user_config 区间须成对给出 a、b")
			return
		}
		a, b = *uc.A, *uc.B
	}
	if !isFinite(a) || !isFinite(b) || a >= b {
		err = fmt.Errorf("积分区间非法（须 a < b 且有限）: [%.6g, %.6g]", a, b)
		return
	}

	// 步数归属：user_config.n >（仅未换题时）target.n > 库内默认 4。
	// n 须 4~1024（3 步 RK4 表头 + 至少 1 步阿当姆斯）。
	n := def.N
	if sameProblem && tgt.N != nil {
		n = *tgt.N
	}
	if uc.N != nil {
		n = *uc.N
	}
	if n < 4 || n > 1024 {
		err = fmt.Errorf("n 须为 4~1024 的步数（3 步表头+至少 1 步阿当姆斯），got %d", n)
		return
	}

	// 真值 = 方法输出：后端按同一 f、初值、区间、步长现算 AB4-AM4 列。
	ys := adamsPCColumn(def.F, a, b, def.Y0, n)
	trueY := ys[n]
	if !isFinite(trueY) {
		err = fmt.Errorf("阿当姆斯真值非有限值（区间 [%.6g, %.6g] 内发散？）", a, b)
		return
	}

	var rs []struct {
		Value *float64  `json:"value"`
		YCol  []float64 `json:"y_col"`
	}
	if err = json.Unmarshal(readings, &rs); err != nil {
		err = fmt.Errorf("readings 解析失败: %w", err)
		return
	}
	r0 := rs[0]
	if r0.Value == nil {
		err = fmt.Errorf("readings[0] 缺少 value 字段")
		return
	}
	value := *r0.Value
	if !isFinite(value) {
		err = fmt.Errorf("value 不能是 NaN/Inf")
		return
	}
	// y_col 可选：长度须为 n+1、数值有限（仅审计记录用，错维度直接 400 暴露）
	if r0.YCol != nil {
		if len(r0.YCol) != n+1 {
			err = fmt.Errorf("y_col 长度须为 %d（n+1），got %d", n+1, len(r0.YCol))
			return
		}
		for i, v := range r0.YCol {
			if !isFinite(v) {
				err = fmt.Errorf("y_col[%d] 不能是 NaN/Inf", i)
				return
			}
		}
	}

	relErr := relError(value, trueY)
	score = scoreFromRelErrBands(relErr, bands)
	passed = passedByScore(score, tgt.PassScore)
	detail = fmt.Sprintf("problem=%s a=%.6g b=%.6g n=%d value=%.10g rel_err=%.3g true_Y=%.10g ref_Y=%.10g（阿当姆斯预报-校正 AB4-AM4 PECE，RK4 表头，精确解 %s）",
		probKey, a, b, n, value, relErr, trueY, def.Exact(b), def.Desc)
	if r0.YCol != nil {
		detail += fmt.Sprintf(" y_col=[%s]", joinGauss(r0.YCol))
	}
	return
}

// ========== L24 最小二乘拟合 ==========

// fitProblemDef 最小二乘拟合题库（后端持真值）。problem_key -> {xs, ys, m, x_eval, ln}。
// 真值 = 法方程解出的拟合多项式在 x_eval 的值 P(x_eval)（"真值=方法输出"同款）：
// 幂基法方程 Σ_k x_k^{i+j}·c_i = Σ_k x_k^j·y_k（j=0..m），列主元高斯求解。
// ln=true 为指数模型 y=a·e^{bx}：对 (x, ln y) 做 m 次多项式拟合（ln 线性化），
// 真值 = e^{P(x_eval)}。
// 拟合次数 m 须 <= 点数-2：m=点数-1 时法方程解=插值多项式，"拟合"退化成穿过
// 所有点，防"抄 L16/L18 插值答案"的语义随之失效（同阿当姆斯不用精确解做表头）。
type fitProblemDef struct {
	Xs    []float64
	Ys    []float64
	M     int
	XEval float64
	Ln    bool // true: 数据先取 ln 再拟合（指数模型线性化），真值=e^{P(x_eval)}
	Desc  string
}

var fitProblems = map[string]fitProblemDef{
	"lin_line": { // 直线拟合经典：法方程 4a+10b=14、10a+30b=39.8 手算 a=1.1、b=0.96
		// P(3.5)=4.46 整洁。抄观测 y(3)=4→29、y(4)=4.9→30、抄均值 ȳ=3.5→16
		// 均不过线；抄三次插值 P3(3.5)=4.4375→69 过线不满分（插值≠拟合）。
		Xs:    []float64{1, 2, 3, 4},
		Ys:    []float64{2, 3.1, 4, 4.9},
		M:     1,
		XEval: 3.5,
		Desc:  "直线拟合 y=a+bx，法方程手算 a=1.1、b=0.96",
	},
	"quad_parabola": { // 二次多项式拟合 5 点：系数恰 27/350、1/28、137/140，P(2)=711/175。
		// x_eval=2 落在数据点上：LSQ 只"靠近"不过点，抄观测 3.9→44、抄真函数 x²=4→58
		// 均不过线（次线差 2 分），次数用错（直线拟合得 6.02）→1 分。
		Xs:    []float64{0, 1, 2, 3, 4},
		Ys:    []float64{0.1, 1.1, 3.9, 9.2, 15.8},
		M:     2,
		XEval: 2,
		Desc:  "二次多项式拟合（数据≈x²），系数 27/350、1/28、137/140",
	},
	"exp_lnw": { // 指数模型 y=a·e^{bx} 经典（细菌增长）：ln 线性化后拟合 A+Bx，
		// 真值=e^{A+B·2.5}=6.910393。抄观测 5.2→13、9.0→9，不做线性化直接
		// 直线拟合 y 得 8.765→12 均不过线（两域距离拉大防"跳过线性化"）。
		Xs:    []float64{0, 1, 2, 3, 4},
		Ys:    []float64{1.8, 2.9, 5.2, 9.0, 16.2},
		M:     1,
		XEval: 2.5,
		Ln:    true,
		Desc:  "指数模型 y=a·e^{bx}，ln y 线性化拟合（a=e^A≈1.7355、b≈0.5527）",
	},
}

// lsqFit 幂基最小二乘拟合：组装法方程 [Σx^{i+j}]c=[Σx^j y]（j 行 i 列，0..m 阶），
// 列主元高斯求解，返回系数 [c_0..c_m]。
// 点数 3~12、x 互异、数值有限；m 须 0<=m<=min(5, 点数-2)（上界防拟合退化成插值，
// 5 封顶防法方程病态）；ln=true 时先对 y 取 ln，返回 ln 域系数（调用方自行 e^ 回原域）。
func lsqFit(xs, ys []float64, m int, ln bool) ([]float64, error) {
	n := len(xs)
	if n < 3 || n > 12 {
		return nil, fmt.Errorf("数据点数须为 3~12，got %d", n)
	}
	if len(ys) != n {
		return nil, fmt.Errorf("x/y 点数不匹配（x %d 个, y %d 个）", n, len(ys))
	}
	if m < 0 || m > 5 || m > n-2 {
		return nil, fmt.Errorf("拟合次数 m 须为 0~min(5, 点数-2)（点数 %d 时上限 %d），got %d", n, min(5, n-2), m)
	}
	for i := 0; i < n; i++ {
		if !isFinite(xs[i]) || !isFinite(ys[i]) {
			return nil, fmt.Errorf("数据点含 NaN/Inf")
		}
		if ln && ys[i] <= 0 {
			return nil, fmt.Errorf("ln 线性化要求数据 y>0，y[%d]=%g", i, ys[i])
		}
		for j := 0; j < i; j++ {
			if xs[i] == xs[j] {
				return nil, fmt.Errorf("数据点 x 须互异（x[%d]=x[%d]=%g）", i, j, xs[i])
			}
		}
	}
	A := make([][]float64, m+1)
	b := make([]float64, m+1)
	// 幂和与右端各累加一次：Σx^p（p=0..2m）与 Σ x^p·y（p=0..m），免重复累乘
	powSum := make([]float64, 2*m+1)
	xySum := make([]float64, m+1)
	for k := 0; k < n; k++ {
		xp, y := 1.0, ys[k]
		if ln {
			y = math.Log(y)
		}
		for p := 0; p <= 2*m; p++ {
			powSum[p] += xp
			if p <= m {
				xySum[p] += xp * y
			}
			xp *= xs[k]
		}
	}
	for j := 0; j <= m; j++ {
		A[j] = make([]float64, m+1)
		for i := 0; i <= m; i++ {
			A[j][i] = powSum[i+j]
		}
		b[j] = xySum[j]
	}
	return gaussSolve(A, b)
}

// lsqEval 幂基多项式求值 P(x)=Σ c_i x^i（霍纳嵌套）。
func lsqEval(cs []float64, x float64) float64 {
	v := 0.0
	for i := len(cs) - 1; i >= 0; i-- {
		v = v*x + cs[i]
	}
	return v
}

// scoreLeastSquaresFit 最小二乘拟合评分。
//
// target:     {"method":"lsq_rel_err","problem_key":"lin_line","m":1,"x_eval":3.5,"pass_score":60}
// readings:   [{"value":4.46,"coef":[1.1,0.96]}]
//
//	value 必填（= 拟合多项式在 x_eval 处的值；ln 题为回原域的 ŷ(x_eval)）；
//	coef 可选（学生解法方程得的系数 [c_0..c_m]，长度须 m+1，仅记审计不打分）
//
// user_config 可覆盖 problem_key（换题）/ m（换拟合次数）/ x_eval（换求值点），
// 或自定义数据点 {"pts_x":[...],"pts_y":[...],"x_eval":...}（3~12 个互异 x，
// 后端重算真值）。m/x_eval 归属规则同 v0.12.1：target 的只属于 target 自己的题，
// 换题/自定义后回落新题库默认（custom 题默认 m=1、无默认求值点须显式给 x_eval）。
//
// 评分 = 拟合值精度分：真值 P(x_eval) 由后端按同一数据点、同一 m 现算，
// 按 |value-P|/|P| 相对误差走默认曲线（target.bands 可覆盖）。
// 防作弊语义：拟合≠插值也≠观测值——抄数据点 y(x_eval)（30 分左右）、抄均值、
// 抄 L16 式插值（69 过线不满分，与 RK4 抄 e 同款"同为合法路径但不该得满分"）、
// 指数题不线性化直接拟 y（12 分）均被残差/两域距离压住，认真解法方程才满分。
func scoreLeastSquaresFit(readings, target, userConfig []byte) (score int, passed bool, detail string, err error) {
	var tgt struct {
		ProblemKey string          `json:"problem_key"`
		M          *int            `json:"m"`
		XEval      *float64        `json:"x_eval"`
		PassScore  int             `json:"pass_score"`
		Bands      json.RawMessage `json:"bands"`
	}
	if err = json.Unmarshal(target, &tgt); err != nil {
		err = fmt.Errorf("target 解析失败: %w", err)
		return
	}
	var bands []relErrBand
	if bands, err = parseRelErrBands(tgt.Bands); err != nil {
		return
	}
	// v0.5 模式：user_config 覆盖（自定义数据点优先于 problem_key）
	var uc struct {
		ProblemKey string    `json:"problem_key"`
		PtsX       []float64 `json:"pts_x"`
		PtsY       []float64 `json:"pts_y"`
		M          *int      `json:"m"`
		XEval      *float64 `json:"x_eval"`
	}
	_ = json.Unmarshal(userConfig, &uc) // 可选字段，缺省/为空时忽略

	xs, ys := []float64(nil), []float64(nil)
	ln := false
	probKey := tgt.ProblemKey
	defM, defXEval := -1, math.NaN() // 库内题的默认 m / 求值点（custom 题无默认求值点）
	desc := ""
	switch {
	case len(uc.PtsX) > 0 || len(uc.PtsY) > 0:
		probKey = "custom"
		xs, ys = uc.PtsX, uc.PtsY
		defM, desc = 1, "自定义数据点"
	case uc.ProblemKey != "":
		probKey = uc.ProblemKey
	}
	if xs == nil {
		def, ok := fitProblems[probKey]
		if !ok {
			err = fmt.Errorf("未知拟合题: %s", probKey)
			return
		}
		xs, ys, ln, desc = def.Xs, def.Ys, def.Ln, def.Desc
		defM, defXEval = def.M, def.XEval
	}
	// m 归属：user_config.m >（仅未换题时）target.m > 库内默认（custom 默认 1）。
	// target.m 属于 target 自己的题：换题/自定义后回落新题默认（v0.12.1 同款）。
	sameProblem := probKey == tgt.ProblemKey
	m := defM
	if sameProblem && tgt.M != nil {
		m = *tgt.M
	}
	if uc.M != nil {
		m = *uc.M
	}
	// 求值点归属：user_config.x_eval >（仅未换题时）target.x_eval > 库内默认。
	xEval := defXEval
	if sameProblem && tgt.XEval != nil {
		xEval = *tgt.XEval
	}
	if uc.XEval != nil {
		xEval = *uc.XEval
	}
	if math.IsNaN(xEval) {
		err = fmt.Errorf("自定义数据点须同时给出 x_eval")
		return
	}
	if !isFinite(xEval) {
		err = fmt.Errorf("x_eval 不能是 NaN/Inf")
		return
	}

	cs, ferr := lsqFit(xs, ys, m, ln)
	if ferr != nil {
		err = ferr
		return
	}
	trueP := lsqEval(cs, xEval)
	if ln {
		trueP = math.Exp(trueP)
	}
	if !isFinite(trueP) {
		err = fmt.Errorf("拟合真值非有限值（m=%d，数据点量级过大？）", m)
		return
	}

	var rs []struct {
		Value *float64  `json:"value"`
		Coef  []float64 `json:"coef"`
	}
	if err = json.Unmarshal(readings, &rs); err != nil {
		err = fmt.Errorf("readings 解析失败: %w", err)
		return
	}
	r0 := rs[0]
	if r0.Value == nil {
		err = fmt.Errorf("readings[0] 缺少 value 字段")
		return
	}
	value := *r0.Value
	if !isFinite(value) {
		err = fmt.Errorf("value 不能是 NaN/Inf")
		return
	}
	// coef 可选：长度须为 m+1、数值有限（仅审计记录用，错维度直接 400 暴露）
	if r0.Coef != nil {
		if len(r0.Coef) != m+1 {
			err = fmt.Errorf("coef 长度须为 %d（m+1），got %d", m+1, len(r0.Coef))
			return
		}
		for i, c := range r0.Coef {
			if !isFinite(c) {
				err = fmt.Errorf("coef[%d] 不能是 NaN/Inf", i)
				return
			}
		}
	}

	relErr := relError(value, trueP)
	score = scoreFromRelErrBands(relErr, bands)
	passed = passedByScore(score, tgt.PassScore)
	methodNote := "最小二乘法方程拟合"
	if ln {
		methodNote += "（指数模型 ln 线性化）"
	}
	detail = fmt.Sprintf("problem=%s pts=%d m=%d x_eval=%.6g value=%.10g rel_err=%.3g true_P=%.10g（%s，%s）",
		probKey, len(xs), m, xEval, value, relErr, trueP, methodNote, desc)
	if r0.Coef != nil {
		detail += fmt.Sprintf(" coef=[%s]", joinGauss(r0.Coef))
	}
	return
}

// ========== L15 三次样条插值（cubic_spline，v0.20，difficulty 4）==========

// splineProblemDef 样条题库条目：节点 Xs/Ys（x 严格递增）、默认求值点 XEval。
// F 非空时原函数值记入审计 ref_f（不参与打分，同 L17 ref_I / L20-L23 ref_Y 惯例）。
type splineProblemDef struct {
	Xs    []float64
	Ys    []float64
	XEval float64
	F     func(x float64) float64
	Desc  string
}

// splineProblems 样条题库（与前端 utils/numlab.js splineProblems 逐字同步）。
// 三题覆盖：不等距 h 手算主场 / 光滑函数对称性（抄 f 过线不满分，m_col 审计暴露）/
// 龙格函数（与 L16 同节点同求值点三法对照，抄谁都 0 分）。
var splineProblems = map[string]splineProblemDef{
	"dist_4": { // 位移观测数据，不等距 h=1,1,2：三弯矩只有 2 个未知量，手算友好；
		// 闭式 M1=15/23、M2=78/23，真值 S(3)=559/46≈12.1521739130435。
		// 抄拉格朗日 P(3)=11.5 约 39 分、线性内插 13 约 35 分均不过线，抄观测全 0
		Xs:    []float64{0, 1, 2, 4},
		Ys:    []float64{0, 2.5, 6, 20},
		XEval: 3,
		Desc:  "位移观测数据（不等距区间 h=1,1,2），三弯矩手算 M1=15/23、M2=78/23，S(3)=559/46",
	},
	"sin_5": { // sin 五等距节点，弯矩对称 M1=M3：真值 S(π/6)≈0.49973964808906。
		// 样条对光滑函数逼近极好 -> 抄 sin(π/6)=0.5 约 83 分、抄拉格朗日 P 约 78 分，
		// 过线不满分（过程真伪由 m_col 审计暴露，L19 抄 I / L22 抄 e 同款语义）
		Xs:    []float64{0, math.Pi / 4, math.Pi / 2, 3 * math.Pi / 4, math.Pi},
		Ys:    []float64{0, math.Sqrt2 / 2, 1, math.Sqrt2 / 2, 0},
		XEval: math.Pi / 6,
		F:     math.Sin,
		Desc:  "sin 五等距节点（弯矩对称 M1=M3），样条逼近光滑函数极好",
	},
	"runge_5": { // 与 L16 拉格朗日 runge_5 同节点同求值点（三法对照）：
		// P(0.8)≈-0.379310 剧烈振荡 vs 自然样条 S(0.8)≈-0.036339522546419；
		// 抄 P / 抄 f(0.8)=1/17 / 抄观测全 0 分——样条治龙格的主场题。
		// 教学点：自然边界端段仍下弯（S(0.8)<0），clamped 边界更准但需导数信息
		Xs:    []float64{-1, -0.5, 0, 0.5, 1},
		Ys:    []float64{1.0 / 26, 4.0 / 29, 1, 4.0 / 29, 1.0 / 26},
		XEval: 0.8,
		F:     func(x float64) float64 { return 1 / (1 + 25*x*x) },
		Desc:  "龙格函数五等距节点（与 L16 同题对照），样条治龙格现象",
	},
}

// thomasSolve 三对角方程组追赶法（教材三弯矩方程标准解法）：
// a[i] 连左次对角（a[0] 恒 0）、b 主对角、c[i] 连右超对角（c[n-1] 恒 0）、d 右端。
func thomasSolve(a, b, c, d []float64) ([]float64, error) {
	n := len(b)
	if n == 0 {
		return nil, fmt.Errorf("追赶法方程数为 0")
	}
	if len(a) != n || len(c) != n || len(d) != n {
		return nil, fmt.Errorf("追赶法三对角维度不匹配")
	}
	cp := make([]float64, n)
	dp := make([]float64, n)
	m := b[0]
	if !isFinite(m) || m == 0 {
		return nil, fmt.Errorf("追赶法消元主元异常（第 0 行）")
	}
	cp[0], dp[0] = c[0]/m, d[0]/m
	for i := 1; i < n; i++ {
		m = b[i] - a[i]*cp[i-1]
		if !isFinite(m) || m == 0 {
			return nil, fmt.Errorf("追赶法消元主元异常（第 %d 行）", i)
		}
		cp[i] = c[i] / m
		dp[i] = (d[i] - a[i]*dp[i-1]) / m
	}
	x := make([]float64, n)
	x[n-1] = dp[n-1]
	for i := n - 2; i >= 0; i-- {
		x[i] = dp[i] - cp[i]*x[i+1]
	}
	return x, nil
}

// splineMoments 自然边界三次样条的弯矩列 M_0..M_{n-1}：
// M_0 = M_{n-1} = 0，内部 i=1..n-2 共 n-2 个三弯矩方程
//	mu_i·M_{i-1} + 2M_i + lam_i·M_{i+1} = d_i，
//	mu_i = h_i/(h_i+h_{i+1})，lam_i = h_{i+1}/(h_i+h_{i+1})，
//	d_i  = 6·[(y_{i+1}−y_i)/h_{i+1} − (y_i−y_{i−1})/h_i]/(h_i+h_{i+1})
// 主对角全 2 且 mu+lam=1，严格对角占优，追赶法必可解。
func splineMoments(xs, ys []float64) ([]float64, error) {
	n := len(xs)
	if n < 3 {
		return nil, fmt.Errorf("样条至少需要 3 个节点，got %d", n)
	}
	if len(ys) != n {
		return nil, fmt.Errorf("节点 x/y 数量不匹配（x %d 个, y %d 个）", n, len(ys))
	}
	mAll := make([]float64, n)
	k := n - 2 // 内部未知弯矩个数
	a := make([]float64, k)
	b := make([]float64, k)
	c := make([]float64, k)
	d := make([]float64, k)
	for j := range b {
		b[j] = 2
	}
	for i := 1; i <= n-2; i++ {
		h0, h1 := xs[i]-xs[i-1], xs[i+1]-xs[i]
		d[i-1] = 6 * ((ys[i+1]-ys[i])/h1 - (ys[i]-ys[i-1])/h0) / (h0 + h1)
		if i > 1 {
			a[i-1] = h0 / (h0 + h1) // mu_i 连左
		}
		if i < n-2 {
			c[i-1] = h1 / (h0 + h1) // lam_i 连右
		}
	}
	inner, terr := thomasSolve(a, b, c, d)
	if terr != nil {
		return nil, terr
	}
	for i := 1; i <= n-2; i++ {
		mAll[i] = inner[i-1]
	}
	return mAll, nil
}

// splineEvalAt 样条在求值点处的值：先定位所在段 k（xs[k] <= x_eval，
// 右端点归最后一段），再按系数式 S(t) = y_k + B·w + C·w² + D·w³（w = t−x_k）
// Horner 求值，与教材 M 式分段三次式数学等价：
//	B = (y_{k+1}−y_k)/h − h·(2M_k+M_{k+1})/6，C = M_k/2，D = (M_{k+1}−M_k)/(6h)
// 调用方须保证 x_eval ∈ [x_0, x_{n-1}]（样条不做外推）。
func splineEvalAt(xs, ys, m []float64, xEval float64) float64 {
	n := len(xs)
	k := 0
	for i := 0; i < n-1; i++ {
		if xEval >= xs[i] {
			k = i
		}
	}
	if k > n-2 {
		k = n - 2
	}
	h := xs[k+1] - xs[k]
	w := xEval - xs[k]
	b := (ys[k+1]-ys[k])/h - h*(2*m[k]+m[k+1])/6
	c := m[k] / 2
	d := (m[k+1] - m[k]) / (6 * h)
	return ys[k] + w*(b+w*(c+w*d))
}

// scoreCubicSpline 三次样条插值评分（真值 = 自然边界样条 S(x_eval) 后端现算）。
// readings：{value 必填, m_col? 可选审计}——m_col = 学生解出的弯矩 M_0..M_{n-1}，
// 长度须 = 节点数、全或无，仅记审计 detail 不打分（同 L18 col1 / L24 coef 惯例）。
// user_config：problem_key 换题 / x_eval 换求值点 / nodes_x+nodes_y 自定义节点
// （3~8 个、x 严格递增、须带 x_eval 且在区间内——样条不外推，与 L16 的差异点）。
// target 的 x_eval 仅属于 target 自己的题（v0.12.1 归属规则）。
func scoreCubicSpline(readings, target, userConfig []byte) (score int, passed bool, detail string, err error) {
	var tgt struct {
		ProblemKey string          `json:"problem_key"`
		XEval      *float64        `json:"x_eval"`
		PassScore  int             `json:"pass_score"`
		Bands      json.RawMessage `json:"bands"`
	}
	if err = json.Unmarshal(target, &tgt); err != nil {
		err = fmt.Errorf("target 解析失败: %w", err)
		return
	}
	var bands []relErrBand
	if bands, err = parseRelErrBands(tgt.Bands); err != nil {
		return
	}
	var uc struct {
		ProblemKey string    `json:"problem_key"`
		NodesX     []float64 `json:"nodes_x"`
		NodesY     []float64 `json:"nodes_y"`
		XEval      *float64  `json:"x_eval"`
	}
	_ = json.Unmarshal(userConfig, &uc) // 可选字段，缺省/为空时忽略

	xs, ys := []float64(nil), []float64(nil)
	probKey := tgt.ProblemKey
	var f func(float64) float64
	defXEval := math.NaN() // 库内题默认求值点（custom 无默认，必须给）
	desc := ""
	switch {
	case len(uc.NodesX) > 0 || len(uc.NodesY) > 0:
		probKey = "custom"
		xs, ys = uc.NodesX, uc.NodesY
		desc = "自定义节点"
	case uc.ProblemKey != "":
		probKey = uc.ProblemKey
	}
	if xs == nil {
		def, ok := splineProblems[probKey]
		if !ok {
			err = fmt.Errorf("未知样条题: %s", probKey)
			return
		}
		xs, ys, f, desc = def.Xs, def.Ys, def.F, def.Desc
		defXEval = def.XEval
	}
	// 求值点归属：user_config.x_eval >（仅未换题时）target.x_eval > 库内默认（v0.12.1）
	sameProblem := probKey == tgt.ProblemKey
	xEval := defXEval
	if sameProblem && tgt.XEval != nil {
		xEval = *tgt.XEval
	}
	if uc.XEval != nil {
		xEval = *uc.XEval
	}
	if math.IsNaN(xEval) {
		err = fmt.Errorf("自定义节点须同时给出 x_eval")
		return
	}
	if !isFinite(xEval) {
		err = fmt.Errorf("x_eval 不能是 NaN/Inf")
		return
	}

	// 节点校验：3~8 个、数值有限、x 严格递增、x_eval 在区间内（样条不外推）
	if len(xs) < 3 || len(xs) > 8 {
		err = fmt.Errorf("节点数须为 3~8，got %d", len(xs))
		return
	}
	if len(ys) != len(xs) {
		err = fmt.Errorf("节点 x/y 数量不匹配（x %d 个, y %d 个）", len(xs), len(ys))
		return
	}
	for i := range xs {
		if !isFinite(xs[i]) || !isFinite(ys[i]) {
			err = fmt.Errorf("节点数值不能是 NaN/Inf（第 %d 个）", i)
			return
		}
		if i > 0 && xs[i] <= xs[i-1] {
			err = fmt.Errorf("节点 x 须严格递增（第 %d 个起破坏升序），样条节点请按 x 从小到大排列", i)
			return
		}
	}
	if xEval < xs[0] || xEval > xs[len(xs)-1] {
		err = fmt.Errorf("x_eval=%.6g 超出样条区间 [%.6g, %.6g]（样条不做外推）", xEval, xs[0], xs[len(xs)-1])
		return
	}
	mAll, merr := splineMoments(xs, ys)
	if merr != nil {
		err = merr
		return
	}
	trueS := splineEvalAt(xs, ys, mAll, xEval)
	if !isFinite(trueS) {
		err = fmt.Errorf("样条真值非有限值（节点量级过大？）")
		return
	}

	var rs []struct {
		Value *float64  `json:"value"`
		MCol  []float64 `json:"m_col"`
	}
	if err = json.Unmarshal(readings, &rs); err != nil {
		err = fmt.Errorf("readings 解析失败: %w", err)
		return
	}
	r0 := rs[0]
	if r0.Value == nil {
		err = fmt.Errorf("readings[0] 缺少 value 字段")
		return
	}
	value := *r0.Value
	if !isFinite(value) {
		err = fmt.Errorf("value 不能是 NaN/Inf")
		return
	}
	// m_col 可选：长度须为节点数（M_0..M_{n-1}，自然边界 M_0=M_{n-1}=0 也要填）、数值有限
	if r0.MCol != nil {
		if len(r0.MCol) != len(xs) {
			err = fmt.Errorf("m_col 长度须为 %d（节点数，含端点零弯矩），got %d", len(xs), len(r0.MCol))
			return
		}
		for i, mv := range r0.MCol {
			if !isFinite(mv) {
				err = fmt.Errorf("m_col[%d] 不能是 NaN/Inf", i)
				return
			}
		}
	}

	relErr := relError(value, trueS)
	score = scoreFromRelErrBands(relErr, bands)
	passed = passedByScore(score, tgt.PassScore)
	detail = fmt.Sprintf("problem=%s nodes=%d x_eval=%.6g value=%.10g rel_err=%.3g true_S=%.10g（三次样条插值·自然边界三弯矩，%s）",
		probKey, len(xs), xEval, value, relErr, trueS, desc)
	if f != nil {
		detail += fmt.Sprintf(" ref_f=%.10g", f(xEval))
	}
	if r0.MCol != nil {
		detail += fmt.Sprintf(" m_col=[%s]", joinGauss(r0.MCol))
	}
	return
}

// ========== L17 LU 分解（Doolittle）解线性方程组（lu_decomp，v0.22）==========

// luDecompose Doolittle 三角分解 A=LU（不选主元）：L 单位下三角（乘数 m_ik），
// U 上三角（消去结果）。题库 linSystems 五题均无需选主元即可分解（严格对角
// 占优/对称正定/手验主元非零）；自定义矩阵遇主元≈0 时报错——本身就是教学点：
// 不选主元的 LU 可能中断，列主元高斯（L13）才稳定。
// 返回 L、U 与按行序展平的乘子列 l_col（l21,l31,l32,...，长度 n(n-1)/2）。
func luDecompose(A [][]float64) (L, U [][]float64, lCol []float64, err error) {
	n := len(A)
	if n == 0 {
		return nil, nil, nil, fmt.Errorf("A 为空")
	}
	maxAbs := 0.0
	U = make([][]float64, n)
	for i := 0; i < n; i++ {
		if len(A[i]) != n {
			return nil, nil, nil, fmt.Errorf("A 第 %d 行长 %d，不是方阵", i+1, len(A[i]))
		}
		U[i] = append([]float64(nil), A[i]...)
		for _, v := range A[i] {
			if !isFinite(v) {
				return nil, nil, nil, fmt.Errorf("A[%d] 含 NaN/Inf", i+1)
			}
			if av := math.Abs(v); av > maxAbs {
				maxAbs = av
			}
		}
	}
	L = make([][]float64, n)
	for i := range L {
		L[i] = make([]float64, n)
		L[i][i] = 1
	}
	lCol = make([]float64, 0, n*(n-1)/2)
	singTol := 1e-12 * maxAbs // 奇异阈值随矩阵元素量级缩放（与 gaussSolve 同款）
	for k := 0; k < n; k++ {
		piv := math.Abs(U[k][k])
		if !isFinite(piv) || piv <= singTol {
			return nil, nil, nil, fmt.Errorf("第 %d 步主元≈0，不选主元的 LU 分解中断（该阵需选主元或奇异）", k+1)
		}
		for i := k + 1; i < n; i++ {
			m := U[i][k] / U[k][k]
			if !isFinite(m) {
				return nil, nil, nil, fmt.Errorf("分解过程数值溢出")
			}
			L[i][k] = m
			lCol = append(lCol, m)
			for j := k; j < n; j++ {
				U[i][j] -= m * U[k][j]
			}
		}
	}
	return L, U, lCol, nil
}

// luSolve LU 分解路径求解 Ax=b：Doolittle 分解 + Ly=b 前代 + Ux=y 回代。
// 与 gaussSolve（列主元）在精确算术下同解；本题真值走 LU 路径（真值=方法输出）。
func luSolve(A [][]float64, b []float64) (x []float64, lCol []float64, err error) {
	n := len(A)
	if len(b) != n {
		return nil, nil, fmt.Errorf("A 与 b 维度不匹配（A %d 阶, b 长 %d）", n, len(b))
	}
	for i, v := range b {
		if !isFinite(v) {
			return nil, nil, fmt.Errorf("b[%d] 含 NaN/Inf", i)
		}
	}
	_, U, lCol, err := luDecompose(A)
	if err != nil {
		return nil, nil, err
	}
	L := make([][]float64, n)
	for i := 0; i < n; i++ {
		L[i] = make([]float64, n)
		L[i][i] = 1
	}
	idx := 0
	for j := 0; j < n; j++ {
		for i := j + 1; i < n; i++ {
			L[i][j] = lCol[idx]
			idx++
		}
	}
	// 前代 Ly=b（L 单位下三角，按行解 y_i = b_i − Σ_{j<i} l_ij·y_j）
	y := make([]float64, n)
	for i := 0; i < n; i++ {
		s := b[i]
		for j := 0; j < i; j++ {
			s -= L[i][j] * y[j]
		}
		y[i] = s
	}
	// 回代 Ux=y（上三角，按行倒序）
	x = make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		s := y[i]
		for j := i + 1; j < n; j++ {
			s -= U[i][j] * x[j]
		}
		x[i] = s / U[i][i]
		if !isFinite(x[i]) {
			return nil, nil, fmt.Errorf("回代结果非有限值")
		}
	}
	return x, lCol, nil
}

// scoreLUDecomp LU 分解（Doolittle）解线性方程组评分。
//
// target:     {"method":"lu_rel_err","system_key":"sys3_basic","pass_score":60}
//
// readings:   [{"solution":[2,3,-1], "l_col":[-1.5,-1,4]}]——solution 长度须等于
//             未知数个数（必填）；l_col 选填审计（L 的乘子按行序 l21,l31,l32,...
//             长度 n(n-1)/2，全或无，仅记 detail 不参与打分）。
//
// user_config 与 L13 高斯消去完全同款（换题后端一律重算真值，防背答案）：
//
//	{"system_key":"sys4_tridiag"}  换库内方程组；
//	{"A":[[...]],"b":[...]}        自定义方程组（A/b 优先于 system_key；
//	                               2~8 阶方阵、元素有限；主元≈0 或奇异 -> 400）
//
// 评分：真值 = LU 路径解（Doolittle 分解+前代+回代现算不落库），无穷范数相对
// 误差走默认曲线，target.bands 可覆盖。防作弊语义：解与 L13 列主元高斯数学上
// 同源（同 L18/L16 先例：抄=算对），过程真伪由 l_col 乘子审计暴露；自定义 A/b
// 遇主元≈0 直接 400（不选主元 LU 会中断，正是与 L13 的稳定性差异点）。
func scoreLUDecomp(readings, target, userConfig []byte) (score int, passed bool, detail string, err error) {
	var tgt struct {
		SystemKey string          `json:"system_key"`
		PassScore int             `json:"pass_score"`
		Bands     json.RawMessage `json:"bands"`
	}
	if err = json.Unmarshal(target, &tgt); err != nil {
		err = fmt.Errorf("target 解析失败: %w", err)
		return
	}
	var bands []relErrBand
	if bands, err = parseRelErrBands(tgt.Bands); err != nil {
		return
	}
	var uc struct {
		SystemKey string      `json:"system_key"`
		A         [][]float64 `json:"A"`
		B         []float64   `json:"b"`
	}
	_ = json.Unmarshal(userConfig, &uc)

	A, b := [][]float64(nil), []float64(nil)
	sysKey := tgt.SystemKey
	switch {
	case len(uc.A) > 0:
		sysKey = "custom"
		if len(uc.A) < 2 || len(uc.A) > 8 {
			err = fmt.Errorf("自定义 A 须为 2~8 阶方阵，got %d 阶", len(uc.A))
			return
		}
		A, b = uc.A, uc.B
	case uc.SystemKey != "":
		sysKey = uc.SystemKey
	}
	if A == nil {
		def, ok := linSystems[sysKey]
		if !ok {
			err = fmt.Errorf("未知方程组: %s", sysKey)
			return
		}
		A, b = def.A, def.b
	}

	trueX, trueLCol, serr := luSolve(A, b)
	if serr != nil {
		err = serr
		return
	}

	var rs []struct {
		Solution []float64 `json:"solution"`
		LCol     []float64 `json:"l_col"`
	}
	if err = json.Unmarshal(readings, &rs); err != nil {
		err = fmt.Errorf("readings 解析失败: %w", err)
		return
	}
	r0 := rs[0]
	if r0.Solution == nil {
		err = fmt.Errorf("readings[0] 缺少 solution 字段")
		return
	}
	if len(r0.Solution) != len(trueX) {
		err = fmt.Errorf("solution 长度须为 %d，got %d", len(trueX), len(r0.Solution))
		return
	}
	for i, v := range r0.Solution {
		if !isFinite(v) {
			err = fmt.Errorf("solution[%d] 不能是 NaN/Inf", i)
			return
		}
	}
	// l_col 可选：长度须为 n(n-1)/2（L 的乘子按行序）、数值有限
	n := len(trueX)
	if r0.LCol != nil {
		if len(r0.LCol) != n*(n-1)/2 {
			err = fmt.Errorf("l_col 长度须为 %d（乘子 l21,l31,l32,... 行序），got %d", n*(n-1)/2, len(r0.LCol))
			return
		}
		for i, v := range r0.LCol {
			if !isFinite(v) {
				err = fmt.Errorf("l_col[%d] 不能是 NaN/Inf", i)
				return
			}
		}
	}

	relErr := vecRelError(r0.Solution, trueX)
	score = scoreFromRelErrBands(relErr, bands)
	passed = passedByScore(score, tgt.PassScore)
	detail = fmt.Sprintf("system=%s n=%d rel_err=%.3g solution=[%s] true=[%s]（Doolittle LU 分解·Ly=b 前代·Ux=y 回代）",
		sysKey, n, relErr, joinGauss(r0.Solution), joinGauss(trueX))
	if r0.LCol != nil {
		detail += fmt.Sprintf(" l_col=[%s] ref_l=[%s]", joinGauss(r0.LCol), joinGauss(trueLCol))
	}
	return
}

// ========== L18 追赶法解三对角方程组（thomas_solve，v0.23）==========

// triDiagDef 三对角方程组（后端持真值）：a 下对角（长 n-1）、b 主对角（长 n）、
// c 上对角（长 n-1）、d 右端（长 n）。满矩阵由 triToFull 还原（测试交叉验证用），
// 前端展示矩阵自行按 a/b/c 生成，数值须与这里逐字同步。
type triDiagDef struct {
	a []float64
	b []float64
	c []float64
	d []float64
}

// triSystems 三对角方程组题库（追赶法主场）：四题均对角占优或对称正定，
// 追赶法（不选主元）必安全；适用条件的教学对照由自定义输入承担（β≈0 -> 400）。
var triSystems = map[string]triDiagDef{
	"tri1_basic": { // 手算例题 4 阶：m=(0.5,0.8,0.3125)，beta=(2,2.5,3.2,4.6875)，解恰 (1,2,3,4)
		a: []float64{1, 2, 1},
		b: []float64{2, 3, 4, 5},
		c: []float64{1, 1, 1},
		d: []float64{4, 10, 20, 23},
	},
	"tri2_poisson": { // 一维泊松差分 5 内点（-u''=f，对称正定）：beta=(2,3/2,4/3,5/4,6/5)，解全 1
		a: []float64{-1, -1, -1, -1},
		b: []float64{2, 2, 2, 2, 2},
		c: []float64{-1, -1, -1, -1},
		d: []float64{1, 0, 0, 0, 1},
	},
	"tri3_small": { // 入门 3 阶（严格对角占优）：m=(1/4,2/7)，解恰 (1,1,1)
		a: []float64{1, 1},
		b: []float64{4, 4, 4},
		c: []float64{2, 2},
		d: []float64{6, 7, 5},
	},
	"tri4_spline": { // 三次样条三弯矩系数阵 6 阶（L25 呼应：[2,1; 1,4,1; ...; 1,2]），解全 1
		a: []float64{1, 1, 1, 1, 1},
		b: []float64{2, 4, 4, 4, 4, 2},
		c: []float64{1, 1, 1, 1, 1},
		d: []float64{3, 6, 6, 6, 6, 3},
	},
}

// thomasChase 追赶法（Thomas 法）解三对角方程组——LU 分解（L27）的三对角特例。
// 追（Crout 消元）：beta_1=b_1；i=2..n：m_i = a_{i-1}/beta_{i-1}，
//   beta_i = b_i − m_i·c_{i-1}，d_i <- d_i − m_i·d_{i-1}。
// 赶（回代）：x_n = d_n/beta_n；x_i = (d_i − c_i·x_{i+1})/beta_i。
// 返回解 x、乘子列 m（长 n-1）与新主对角 beta（长 n）——审计对照用。
// （与 L25 样条内部用的 thomasSolve 同族：那个是归一化形式只返回解，
// 这里按教材原始形式留出 m/beta 供学生审计对照。）
// beta≈0（按输入最大元的相对阈值判）时报错：追赶法不选主元会中断
// （对角占优/对称正定阵必安全，正是适用条件教学点）。
func thomasChase(a, b, c, d []float64) (x, m, beta []float64, err error) {
	n := len(b)
	if n == 0 {
		return nil, nil, nil, fmt.Errorf("b 为空")
	}
	if len(a) != n-1 || len(c) != n-1 || len(d) != n {
		return nil, nil, nil, fmt.Errorf("三对角维度不匹配（a %d / b %d / c %d / d %d，须 n-1/n/n-1/n）", len(a), n, len(c), len(d))
	}
	maxAbs := 0.0
	for _, arr := range [][]float64{a, b, c, d} {
		for _, v := range arr {
			if !isFinite(v) {
				return nil, nil, nil, fmt.Errorf("三对角元素含 NaN/Inf")
			}
			if av := math.Abs(v); av > maxAbs {
				maxAbs = av
			}
		}
	}
	singTol := 1e-12 * maxAbs // 奇异阈值随元素量级缩放（与 gaussSolve/luDecompose 同款）
	m = make([]float64, n-1)
	beta = make([]float64, n)
	dd := make([]float64, n)
	beta[0] = b[0]
	dd[0] = d[0]
	if math.Abs(beta[0]) <= singTol {
		return nil, nil, nil, fmt.Errorf("第 1 步 beta≈0，追赶法中断（该阵需选主元或奇异）")
	}
	for i := 1; i < n; i++ {
		mm := a[i-1] / beta[i-1]
		if !isFinite(mm) {
			return nil, nil, nil, fmt.Errorf("追的过程数值溢出")
		}
		m[i-1] = mm
		beta[i] = b[i] - mm*c[i-1]
		dd[i] = d[i] - mm*dd[i-1]
		if !isFinite(beta[i]) || !isFinite(dd[i]) || math.Abs(beta[i]) <= singTol {
			return nil, nil, nil, fmt.Errorf("第 %d 步 beta≈0，追赶法中断（该阵需选主元或奇异）", i+1)
		}
	}
	x = make([]float64, n)
	x[n-1] = dd[n-1] / beta[n-1]
	if !isFinite(x[n-1]) {
		return nil, nil, nil, fmt.Errorf("赶的过程数值溢出")
	}
	for i := n - 2; i >= 0; i-- {
		x[i] = (dd[i] - c[i]*x[i+1]) / beta[i]
		if !isFinite(x[i]) {
			return nil, nil, nil, fmt.Errorf("赶的过程数值溢出")
		}
	}
	return x, m, beta, nil
}

// scoreThomasSolve 追赶法解三对角方程组评分。
//
// target:     {"method":"thomas_rel_err","system_key":"tri1_basic","pass_score":60}
//
// readings:   [{"solution":[1,2,3,4], "m":[0.5,0.8,0.3125],
//               "beta":[2,2.5,3.2,4.6875]}]——solution 长度须等于未知数个数
//             （必填）；m（乘子 m_2..m_n，长 n-1）与 beta（追出的新主对角
//             beta_1..beta_n，长 n）均为选填审计（各全或无，仅记 detail
//             不参与打分）。
//
// user_config（换题后端一律重算真值，防背答案）：
//
//	{"system_key":"tri2_poisson"}                换库内三对角方程组；
//	{"a":[...],"b":[...],"c":[...],"d":[...]}    自定义三对角（四数组优先于
//	                                               system_key；3~8 阶；beta≈0 -> 400）
//
// 评分：真值 = 追赶法解（thomasChase 追+赶现算不落库），无穷范数相对误差走默认曲线，
// target.bands 可覆盖。防作弊语义：解与 L13/L27 数学同源（抄=算对，同
// L18/L16 Neville≡Lagrange 先例），过程真伪由 m/beta 审计暴露；自定义
// 三对角 beta≈0 直接 400（追赶法不选主元会中断——适用条件：对角占优或
// 对称正定，正是与 L13 列主元高斯的差异点）。
func scoreThomasSolve(readings, target, userConfig []byte) (score int, passed bool, detail string, err error) {
	var tgt struct {
		SystemKey string          `json:"system_key"`
		PassScore int             `json:"pass_score"`
		Bands     json.RawMessage `json:"bands"`
	}
	if err = json.Unmarshal(target, &tgt); err != nil {
		err = fmt.Errorf("target 解析失败: %w", err)
		return
	}
	var bands []relErrBand
	if bands, err = parseRelErrBands(tgt.Bands); err != nil {
		return
	}
	var uc struct {
		SystemKey string      `json:"system_key"`
		Sub       []float64   `json:"a"`
		Diag      []float64   `json:"b"`
		Super     []float64   `json:"c"`
		Rhs       []float64   `json:"d"`
	}
	_ = json.Unmarshal(userConfig, &uc)

	a, b, c, d := []float64(nil), []float64(nil), []float64(nil), []float64(nil)
	sysKey := tgt.SystemKey
	switch {
	case len(uc.Diag) > 0 || len(uc.Sub) > 0 || len(uc.Super) > 0 || len(uc.Rhs) > 0:
		sysKey = "custom"
		if len(uc.Diag) < 3 || len(uc.Diag) > 8 {
			err = fmt.Errorf("自定义三对角须为 3~8 阶，got %d 阶", len(uc.Diag))
			return
		}
		a, b, c, d = uc.Sub, uc.Diag, uc.Super, uc.Rhs
	case uc.SystemKey != "":
		sysKey = uc.SystemKey
	}
	if a == nil {
		def, ok := triSystems[sysKey]
		if !ok {
			err = fmt.Errorf("未知三对角方程组: %s", sysKey)
			return
		}
		a, b, c, d = def.a, def.b, def.c, def.d
	}

	trueX, trueM, trueBeta, serr := thomasChase(a, b, c, d)
	if serr != nil {
		err = serr
		return
	}

	var rs []struct {
		Solution []float64 `json:"solution"`
		M        []float64 `json:"m"`
		Beta     []float64 `json:"beta"`
	}
	if err = json.Unmarshal(readings, &rs); err != nil {
		err = fmt.Errorf("readings 解析失败: %w", err)
		return
	}
	r0 := rs[0]
	if r0.Solution == nil {
		err = fmt.Errorf("readings[0] 缺少 solution 字段")
		return
	}
	n := len(trueX)
	if len(r0.Solution) != n {
		err = fmt.Errorf("solution 长度须为 %d，got %d", n, len(r0.Solution))
		return
	}
	for i, v := range r0.Solution {
		if !isFinite(v) {
			err = fmt.Errorf("solution[%d] 不能是 NaN/Inf", i)
			return
		}
	}
	// m / beta 可选审计：各全或无，长度分别为 n-1 / n、数值有限
	if r0.M != nil {
		if len(r0.M) != n-1 {
			err = fmt.Errorf("m 长度须为 %d（乘子 m2..mn），got %d", n-1, len(r0.M))
			return
		}
		for i, v := range r0.M {
			if !isFinite(v) {
				err = fmt.Errorf("m[%d] 不能是 NaN/Inf", i)
				return
			}
		}
	}
	if r0.Beta != nil {
		if len(r0.Beta) != n {
			err = fmt.Errorf("beta 长度须为 %d（新主对角 beta1..betan），got %d", n, len(r0.Beta))
			return
		}
		for i, v := range r0.Beta {
			if !isFinite(v) {
				err = fmt.Errorf("beta[%d] 不能是 NaN/Inf", i)
				return
			}
		}
	}

	relErr := vecRelError(r0.Solution, trueX)
	score = scoreFromRelErrBands(relErr, bands)
	passed = passedByScore(score, tgt.PassScore)
	detail = fmt.Sprintf("system=%s n=%d rel_err=%.3g solution=[%s] true=[%s]（追赶法·追出 m 与 beta·赶回代得解·三对角 LU 特例）",
		sysKey, n, relErr, joinGauss(r0.Solution), joinGauss(trueX))
	if r0.M != nil {
		detail += fmt.Sprintf(" m=[%s] ref_m=[%s]", joinGauss(r0.M), joinGauss(trueM))
	}
	if r0.Beta != nil {
		detail += fmt.Sprintf(" beta=[%s] ref_beta=[%s]", joinGauss(r0.Beta), joinGauss(trueBeta))
	}
	return
}

// ========== L19 平方根法解对称正定方程组（cholesky_solve，v0.24）==========

// spdDef 对称正定方程组（后端持真值）：A n 阶对称正定方阵、b 右端。
// 数值须与前端 numlab.js spdSystems 逐字同步（label/desc 为前端文案）。
type spdDef struct {
	A [][]float64
	b []float64
}

// spdSystems 平方根法题库（对称正定四题，LLᵀ 分解必安全，不选主元）：
// L/y/x 全整数，课堂手算友好；min(i,j) 阵的 L 恰为下三角全 1。
var spdSystems = map[string]spdDef{
	"spd1_2x2": { // 入门 2 阶：L=[[2,0],[1,2]]，y=(3,2)，解恰 (1,1)
		A: [][]float64{{4, 2}, {2, 5}},
		b: []float64{6, 7},
	},
	"spd2_min3": { // min(i,j) 阵 3 阶（默认题）：L=下三角全 1，y=(6,5,3)，解恰 (1,2,3)
		A: [][]float64{{1, 1, 1}, {1, 2, 2}, {1, 2, 3}},
		b: []float64{6, 11, 14},
	},
	"spd3_tridiag3": { // 三对角 SPD（L28 呼应）：L=[[2,0,0],[1,2,0],[0,1,3]]，y=(3,3,3)，解恰 (1,1,1)
		A: [][]float64{{4, 2, 0}, {2, 5, 2}, {0, 2, 10}},
		b: []float64{6, 9, 12},
	},
	"spd4_min4": { // min(i,j) 阵 4 阶：L=下三角全 1，y=(4,3,2,1)，解全 1
		A: [][]float64{{1, 1, 1, 1}, {1, 2, 2, 2}, {1, 2, 3, 3}, {1, 2, 3, 4}},
		b: []float64{4, 7, 9, 10},
	},
}

// choleskySolve 平方根法（Cholesky）解对称正定方程组——LU 分解（L27）的
// 对称正定特例：A = L·Lᵀ（L 下三角、主对角全正）。
// 分解（按行 Cholesky–Crout）：l_ii = sqrt(a_ii − Σ_{k<i} l_ik²)，
//   l_ij = (a_ij − Σ_{k<j} l_ik·l_jk)/l_jj（j<i）。
// 前代 Ly=b：y_i = (b_i − Σ_{k<i} l_ik·y_k)/l_ii；
// 回代 Lᵀx=y：x_i = (y_i − Σ_{k>i} l_ki·x_k)/l_ii。
// 返回解 x、中间向量 y 与因子 L——审计对照用。
// 输入须对称（不对称直接报错）；根号内≤0（按元素量级相对阈值判）报错：
// 矩阵非正定，平方根法中断——对称正定正是适用条件（教学点，与 L13
// 列主元高斯的差异：那里任何非奇异阵都能解，这里条件换来了约一半计算量）。
func choleskySolve(A [][]float64, b []float64) (x, y []float64, L [][]float64, err error) {
	n := len(A)
	if n == 0 {
		return nil, nil, nil, fmt.Errorf("A 为空")
	}
	maxAbs := 0.0
	for i := 0; i < n; i++ {
		if len(A[i]) != n {
			return nil, nil, nil, fmt.Errorf("A 第 %d 行长 %d，不是方阵", i+1, len(A[i]))
		}
		for _, v := range A[i] {
			if !isFinite(v) {
				return nil, nil, nil, fmt.Errorf("A[%d] 含 NaN/Inf", i+1)
			}
			if av := math.Abs(v); av > maxAbs {
				maxAbs = av
			}
		}
	}
	symTol := 1e-9 * math.Max(maxAbs, 1)
	for i := 0; i < n; i++ {
		for j := 0; j < i; j++ {
			if d := math.Abs(A[i][j] - A[j][i]); d > symTol {
				return nil, nil, nil, fmt.Errorf("矩阵须对称：A[%d][%d]=%g ≠ A[%d][%d]=%g", i+1, j+1, A[i][j], j+1, i+1, A[j][i])
			}
		}
	}
	if len(b) != n {
		return nil, nil, nil, fmt.Errorf("A 与 b 维度不匹配（A %d 阶, b 长 %d）", n, len(b))
	}
	for i, v := range b {
		if !isFinite(v) {
			return nil, nil, nil, fmt.Errorf("b[%d] 含 NaN/Inf", i)
		}
	}
	pdTol := 1e-12 * maxAbs // 正定阈值随元素量级缩放（与 gaussSolve/luDecompose 奇异阈值同款）
	L = make([][]float64, n)
	for i := range L {
		L[i] = make([]float64, n)
	}
	for i := 0; i < n; i++ {
		for j := 0; j <= i; j++ {
			s := A[i][j]
			for k := 0; k < j; k++ {
				s -= L[i][k] * L[j][k]
			}
			if i == j {
				if s <= pdTol {
					return nil, nil, nil, fmt.Errorf("第 %d 步根号内≤0，矩阵非正定，平方根法中断（对称正定是适用条件）", i+1)
				}
				L[i][i] = math.Sqrt(s)
			} else {
				L[i][j] = s / L[j][j]
			}
		}
	}
	// 前代 Ly=b（L 非单位下三角，除以 l_ii）
	y = make([]float64, n)
	for i := 0; i < n; i++ {
		s := b[i]
		for k := 0; k < i; k++ {
			s -= L[i][k] * y[k]
		}
		y[i] = s / L[i][i]
		if !isFinite(y[i]) {
			return nil, nil, nil, fmt.Errorf("前代结果非有限值")
		}
	}
	// 回代 Lᵀx=y（(Lᵀ)[i][k]=L[k][i]，主对角同 L）
	x = make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		s := y[i]
		for k := i + 1; k < n; k++ {
			s -= L[k][i] * x[k]
		}
		x[i] = s / L[i][i]
		if !isFinite(x[i]) {
			return nil, nil, nil, fmt.Errorf("回代结果非有限值")
		}
	}
	return x, y, L, nil
}

// joinLowerTri 方阵下三角（含主对角，按行序 l11,l21,l22,...）展平拼接——审计 detail 用
func joinLowerTri(M [][]float64) string {
	vals := make([]float64, 0, len(M)*(len(M)+1)/2)
	for i := range M {
		for j := 0; j <= i && j < len(M[i]); j++ {
			vals = append(vals, M[i][j])
		}
	}
	return joinGauss(vals)
}

// scoreCholeskySolve 平方根法解对称正定方程组评分。
//
// target:     {"method":"cholesky_rel_err","system_key":"spd2_min3","pass_score":60}
//
// readings:   [{"solution":[1,2,3], "L":[[1,0,0],[1,1,0],[1,1,1]],
//               "y":[6,5,3]}]——solution 长度须等于未知数个数（必填）；
//             L（n 阶方阵，对照取下三角含主对角 n(n+1)/2 个元）与
//             y（前代中间向量，长 n）均为选填审计（各全或无，仅记
//             detail 不参与打分）。
//
// user_config（换题后端一律重算真值，防背答案）：
//
//	{"system_key":"spd1_2x2"}          换库内方程组；
//	{"A":[[...]],"b":[...]}            自定义方程组（A/b 优先于 system_key；
//	                                   2~5 阶对称方阵；非对称/非正定 -> 400）
//
// 评分：真值 = 平方根法解（choleskySolve 分解+前代+回代现算不落库），
// 无穷范数相对误差走默认曲线，target.bands 可覆盖。防作弊语义：解与
// L13/L27/L28 数学同源（抄=算对，同 L18/L16 Neville≡Lagrange 先例），
// 过程真伪由 L/y 审计暴露；自定义 A 非对称或根号内≤0 直接 400
// （平方根法只适用于对称正定阵——正是与 L13 列主元高斯的差异点）。
func scoreCholeskySolve(readings, target, userConfig []byte) (score int, passed bool, detail string, err error) {
	var tgt struct {
		SystemKey string          `json:"system_key"`
		PassScore int             `json:"pass_score"`
		Bands     json.RawMessage `json:"bands"`
	}
	if err = json.Unmarshal(target, &tgt); err != nil {
		err = fmt.Errorf("target 解析失败: %w", err)
		return
	}
	var bands []relErrBand
	if bands, err = parseRelErrBands(tgt.Bands); err != nil {
		return
	}
	var uc struct {
		SystemKey string      `json:"system_key"`
		A         [][]float64 `json:"A"`
		B         []float64   `json:"b"`
	}
	_ = json.Unmarshal(userConfig, &uc)

	A, b := [][]float64(nil), []float64(nil)
	sysKey := tgt.SystemKey
	switch {
	case len(uc.A) > 0:
		sysKey = "custom"
		if len(uc.A) < 2 || len(uc.A) > 5 {
			err = fmt.Errorf("自定义 A 须为 2~5 阶对称方阵，got %d 阶", len(uc.A))
			return
		}
		A, b = uc.A, uc.B
	case uc.SystemKey != "":
		sysKey = uc.SystemKey
	}
	if A == nil {
		def, ok := spdSystems[sysKey]
		if !ok {
			err = fmt.Errorf("未知方程组: %s", sysKey)
			return
		}
		A, b = def.A, def.b
	}

	trueX, trueY, trueL, serr := choleskySolve(A, b)
	if serr != nil {
		err = serr
		return
	}

	var rs []struct {
		Solution []float64   `json:"solution"`
		L        [][]float64 `json:"L"`
		Y        []float64   `json:"y"`
	}
	if err = json.Unmarshal(readings, &rs); err != nil {
		err = fmt.Errorf("readings 解析失败: %w", err)
		return
	}
	r0 := rs[0]
	if r0.Solution == nil {
		err = fmt.Errorf("readings[0] 缺少 solution 字段")
		return
	}
	n := len(trueX)
	if len(r0.Solution) != n {
		err = fmt.Errorf("solution 长度须为 %d，got %d", n, len(r0.Solution))
		return
	}
	for i, v := range r0.Solution {
		if !isFinite(v) {
			err = fmt.Errorf("solution[%d] 不能是 NaN/Inf", i)
			return
		}
	}
	// L 可选审计：n 阶方阵、数值有限（对照取下三角含主对角 n(n+1)/2 个元）
	if r0.L != nil {
		if len(r0.L) != n {
			err = fmt.Errorf("L 须为 %d 阶方阵，got %d 行", n, len(r0.L))
			return
		}
		for i, row := range r0.L {
			if len(row) != n {
				err = fmt.Errorf("L 第 %d 行长 %d，须为 %d", i+1, len(row), n)
				return
			}
			for _, v := range row {
				if !isFinite(v) {
					err = fmt.Errorf("L[%d] 含 NaN/Inf", i+1)
					return
				}
			}
		}
	}
	// y 可选审计：长 n、数值有限
	if r0.Y != nil {
		if len(r0.Y) != n {
			err = fmt.Errorf("y 长度须为 %d（前代中间向量），got %d", n, len(r0.Y))
			return
		}
		for i, v := range r0.Y {
			if !isFinite(v) {
				err = fmt.Errorf("y[%d] 不能是 NaN/Inf", i)
				return
			}
		}
	}

	relErr := vecRelError(r0.Solution, trueX)
	score = scoreFromRelErrBands(relErr, bands)
	passed = passedByScore(score, tgt.PassScore)
	detail = fmt.Sprintf("system=%s n=%d rel_err=%.3g solution=[%s] true=[%s]（平方根法·LLᵀ 分解·前代 Ly=b·回代 Lᵀx=y·对称正定专属）",
		sysKey, n, relErr, joinGauss(r0.Solution), joinGauss(trueX))
	if r0.L != nil {
		detail += fmt.Sprintf(" L=[%s] ref_L=[%s]", joinLowerTri(r0.L), joinLowerTri(trueL))
	}
	if r0.Y != nil {
		detail += fmt.Sprintf(" y=[%s] ref_y=[%s]", joinGauss(r0.Y), joinGauss(trueY))
	}
	return
}
