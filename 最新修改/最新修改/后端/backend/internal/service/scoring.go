package service

import (
	"encoding/json"
	"fmt"
	"math"
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
	"bisection":    scoreBisection,
	"newton_iter":  scoreNewtonIter,
	"newton_ring":  scoreNewtonRing,
	"oscilloscope": scoreOscilloscope,
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

// ========== 物理实验批次（PDF 交付物 #4：至少 2 个完整可玩实验）==========
//
// 2026-08-25 按《开发任务讲解》恢复牛顿环 + 示波器。readings 格式与前端页面
// （newton-ring.js / oscilloscope.js）的实际提交逐一对齐；打分曲线走 target.bands
// （读数容差 0.02mm ≈ 0.5%~2% 相对误差量级，与数值分析的收敛精度量级不同）。

// physicsBands 物理实验默认打分曲线（migration target.bands 与此一致）：
// 认真测量（<=0.5%）满分，容差内（~2%）90，明显偏差（~10%）65 及格边缘。
var physicsBands = []relErrBand{
	{0.005, 100},
	{0.02, 90},
	{0.05, 80},
	{0.1, 65},
	{0.3, 30},
	{0.6, 0},
}

// scoreNewtonRing 牛顿环实验（PDF 实验 #1）评分。
//
// target:     {"method":"least_squares_R","lens_radius_mm":855,"wavelength_nm":589.3,
//
//	"pass_score":60,"bands":[[0.005,100],[0.02,90],...]}
//
// readings:   [{"k":1,"r":0.710},...] 至少 3 组（前端按 config.k_range 逐环提交，r 单位 mm）
// user_config 可覆盖 lens_radius_mm / wavelength_nm（v0.5 滑块修复）
//
// 评分：暗环半径 r_k = sqrt(k*lambda*R) => r_k^2 = k*lambda*R，
// 对 (k, r^2) 做过原点最小二乘拟合，斜率 = lambda*R，
// R_fit = 斜率/lambda（lambda 换算 mm），与真值 R 的相对误差按 bands 打分。
func scoreNewtonRing(readings, target, userConfig []byte) (score int, passed bool, detail string, err error) {
	var tgt struct {
		LensRadiusMM float64         `json:"lens_radius_mm"`
		WavelengthNM float64         `json:"wavelength_nm"`
		PassScore    int             `json:"pass_score"`
		Bands        json.RawMessage `json:"bands"`
	}
	if err = json.Unmarshal(target, &tgt); err != nil {
		err = fmt.Errorf("target 解析失败: %w", err)
		return
	}
	// v0.5 模式：user_config 覆盖同名字段
	var uc struct {
		LensRadiusMM *float64 `json:"lens_radius_mm"`
		WavelengthNM *float64 `json:"wavelength_nm"`
	}
	_ = json.Unmarshal(userConfig, &uc) // 可选字段，缺省/为空时忽略
	if uc.LensRadiusMM != nil {
		tgt.LensRadiusMM = *uc.LensRadiusMM
	}
	if uc.WavelengthNM != nil {
		tgt.WavelengthNM = *uc.WavelengthNM
	}
	if !(tgt.LensRadiusMM > 0) || !isFinite(tgt.LensRadiusMM) {
		err = fmt.Errorf("lens_radius_mm 非法: %v", tgt.LensRadiusMM)
		return
	}
	if !(tgt.WavelengthNM > 0) || !isFinite(tgt.WavelengthNM) {
		err = fmt.Errorf("wavelength_nm 非法: %v", tgt.WavelengthNM)
		return
	}
	var bands []relErrBand
	if bands, err = parseRelErrBands(tgt.Bands); err != nil {
		return
	}

	var rs []struct {
		K float64 `json:"k"`
		R float64 `json:"r"`
	}
	if err = json.Unmarshal(readings, &rs); err != nil {
		err = fmt.Errorf("readings 解析失败: %w", err)
		return
	}
	if len(rs) < 3 {
		err = fmt.Errorf("至少 3 组 (k, r) 读数，got %d", len(rs))
		return
	}
	sumKR2, sumK2 := 0.0, 0.0
	for _, r := range rs {
		if !isFinite(r.K) || !isFinite(r.R) || r.K <= 0 || r.R <= 0 {
			err = fmt.Errorf("读数非法: k=%.6g r=%.6g（k/r 须为正的有限数）", r.K, r.R)
			return
		}
		sumKR2 += r.K * r.R * r.R
		sumK2 += r.K * r.K
	}

	lambdaMM := tgt.WavelengthNM * 1e-6
	rFit := (sumKR2 / sumK2) / lambdaMM
	relErr := relError(rFit, tgt.LensRadiusMM)
	score = scoreFromRelErrBands(relErr, bands)
	passed = passedByScore(score, tgt.PassScore)
	detail = fmt.Sprintf("R_fit=%.4gmm rel_err=%.3g true_R=%.4gmm lambda=%.4gnm n=%d",
		rFit, relErr, tgt.LensRadiusMM, tgt.WavelengthNM, len(rs))
	return
}

// scoreOscilloscope 示波器实验（PDF 实验 #2）评分。
//
// target:     {"method":"param_match","channels":{"CH1":{"A":2.0,"f":50},"CH2":{"A":1.5,"f":50}},
//
//	"pass_score":60,"bands":[...]}（phi 不评分，与前端提交一致）
//
// readings:   [{"channel":"CH1","f":50,"A":2.0},{"channel":"CH2","f":50,"A":1.5}]
//
//	须恰好覆盖 target 的全部通道各一次（前端固定提交 CH1/CH2）
//
// user_config: 无（示波器无滑块问题，v0.5 已约定）
//
// 评分：各通道 A/f 分别与真值求相对误差按 bands 打分，取算术平均。
func scoreOscilloscope(readings, target, userConfig []byte) (score int, passed bool, detail string, err error) {
	var tgt struct {
		Channels map[string]struct {
			A float64 `json:"A"`
			F float64 `json:"f"`
		} `json:"channels"`
		PassScore int             `json:"pass_score"`
		Bands     json.RawMessage `json:"bands"`
	}
	if err = json.Unmarshal(target, &tgt); err != nil {
		err = fmt.Errorf("target 解析失败: %w", err)
		return
	}
	if len(tgt.Channels) == 0 {
		err = fmt.Errorf("target.channels 为空")
		return
	}
	for ch, t := range tgt.Channels {
		if !isFinite(t.A) || !isFinite(t.F) {
			err = fmt.Errorf("target.channels.%s 的 A/f 非法", ch)
			return
		}
	}
	var bands []relErrBand
	if bands, err = parseRelErrBands(tgt.Bands); err != nil {
		return
	}

	var rs []struct {
		Channel string  `json:"channel"`
		A       float64 `json:"A"`
		F       float64 `json:"f"`
	}
	if err = json.Unmarshal(readings, &rs); err != nil {
		err = fmt.Errorf("readings 解析失败: %w", err)
		return
	}
	seen := make(map[string]bool, len(tgt.Channels))
	scoreSum, n := 0, 0
	detail = ""
	for _, r := range rs {
		t, ok := tgt.Channels[r.Channel]
		if !ok {
			err = fmt.Errorf("未知通道: %s", r.Channel)
			return
		}
		if seen[r.Channel] {
			err = fmt.Errorf("通道重复提交: %s", r.Channel)
			return
		}
		if !isFinite(r.A) || !isFinite(r.F) {
			err = fmt.Errorf("%s 的 A/f 不能是 NaN/Inf", r.Channel)
			return
		}
		seen[r.Channel] = true
		sA := scoreFromRelErrBands(relError(r.A, t.A), bands)
		sF := scoreFromRelErrBands(relError(r.F, t.F), bands)
		scoreSum += sA + sF
		n += 2
		detail += fmt.Sprintf("%s A_err=%.3g f_err=%.3g;", r.Channel, relError(r.A, t.A), relError(r.F, t.F))
	}
	if len(seen) != len(tgt.Channels) {
		err = fmt.Errorf("readings 须覆盖全部通道（%d/%d）", len(seen), len(tgt.Channels))
		return
	}
	score = int(math.Round(float64(scoreSum) / float64(n)))
	passed = passedByScore(score, tgt.PassScore)
	return
}
