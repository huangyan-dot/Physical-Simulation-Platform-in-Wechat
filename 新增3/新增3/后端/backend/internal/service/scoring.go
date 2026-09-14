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
