package service

import (
	"fmt"
	"strings"
	"testing"
)

func TestScoreByExperiment_EmptyReadingsIsError(t *testing.T) {
	// 契约 §12：readings 非法 -> 400。空数组必须返回 err（上层映射为 400）。
	for _, bad := range []string{`[]`, `{}`, `"x"`, `null`} {
		if _, _, _, err := scoreByExperiment("whatever", []byte(bad), []byte(`{}`), nil); err == nil {
			t.Fatalf("readings=%s: want error, got nil", bad)
		}
	}
}

func TestScoreByExperiment_UnknownExperimentIsError(t *testing.T) {
	if _, _, _, err := scoreByExperiment("nope", []byte(`[{"x":1}]`), []byte(`{}`), nil); err == nil {
		t.Fatal("unknown experiment code: want error, got nil")
	}
}

func TestValidateClassName(t *testing.T) {
	cases := []struct {
		name string
		want bool // true = 合法
	}{
		{"物理实验1班", true},
		{"  x  ", true}, // trim 后非空
		{"", false},
		{"   ", false},
		{strings.Repeat("a", 64), true},  // 刚好 64
		{strings.Repeat("a", 65), false}, // 超 64
	}
	for _, c := range cases {
		err := validateClassName(c.name)
		got := err == nil
		if got != c.want {
			t.Errorf("validateClassName(%q): want ok=%v, got ok=%v (err=%v)", c.name, c.want, got, err)
		}
	}
}

// ========== L1 二分法求根 ==========

// cubicTrueRoot x^3-x-1 在 [1.0,1.5] 的真值根（bisectRoot 双精度解）。
const cubicTrueRoot = 1.3247179572447460

const bisectionTarget = `{"method":"root_rel_err","func_key":"cubic_1","interval":[1.0,1.5],"pass_score":60}`

func submitBisection(t *testing.T, readings, userConfig string) (int, bool, string) {
	t.Helper()
	uc := []byte(nil)
	if userConfig != "" {
		uc = []byte(userConfig)
	}
	score, passed, detail, err := scoreByExperiment("bisection", []byte(readings), []byte(bisectionTarget), uc)
	if err != nil {
		t.Fatalf("readings=%s user_config=%s: %v", readings, userConfig, err)
	}
	return score, passed, detail
}

func TestScoreBisection_CorrectRootFullScore(t *testing.T) {
	readings := fmt.Sprintf(`[{"root":%.16g,"iterations":19}]`, cubicTrueRoot)
	score, passed, detail := submitBisection(t, readings, "")
	if score != 100 || !passed {
		t.Fatalf("want 100/passed, got %d/%v (detail=%s)", score, passed, detail)
	}
	if !strings.Contains(detail, "iterations=19") {
		t.Errorf("detail 应记录 iterations: %s", detail)
	}
}

func TestScoreBisection_RelErrCurve(t *testing.T) {
	// 打分曲线拐点：相对误差恰为分段边界时的得分
	cases := []struct {
		relErr float64
		want   int
	}{
		{1e-6, 100},
		{1e-4, 90},
		{1e-3, 80},
		{1e-2, 65},
		{1e-1, 30},
		{0.03, 48}, // 拐点间 log 线性插值：65 - 35*log10(3) ≈ 48
	}
	for _, c := range cases {
		readings := fmt.Sprintf(`[{"root":%.17g}]`, cubicTrueRoot*(1+c.relErr))
		score, _, detail := submitBisection(t, readings, "")
		if score != c.want {
			t.Errorf("relErr=%.3g: want %d, got %d (detail=%s)", c.relErr, c.want, score, detail)
		}
	}
}

func TestScoreBisection_PassFail(t *testing.T) {
	// pass_score=60：relErr≈0.0139 是及格线（65-35*log10(relErr/1e-2)/1=60）
	for _, c := range []struct {
		relErr     float64
		wantPassed bool
	}{
		{0.012, true},
		{0.03, false},
		{0.25, false},
	} {
		readings := fmt.Sprintf(`[{"root":%.17g}]`, cubicTrueRoot*(1+c.relErr))
		_, passed, _ := submitBisection(t, readings, "")
		if passed != c.wantPassed {
			t.Errorf("relErr=%.3g: want passed=%v, got %v", c.relErr, c.wantPassed, passed)
		}
	}
}

func TestScoreBisection_UserConfigOverride(t *testing.T) {
	// v0.5 模式：前端换题 cos_x_x / [0,1]，真值根变为 0.739085...
	uc := `{"func_key":"cos_x_x","interval":[0.0,1.0]}`

	// 提交旧题（cubic_1）的根应不及格
	score, passed, _ := submitBisection(t, fmt.Sprintf(`[{"root":%.16g}]`, cubicTrueRoot), uc)
	if passed || score > 0 {
		t.Errorf("换题后旧根应 0 分不及格, got %d/%v", score, passed)
	}

	// 提交 cos_x_x 的正确根应满分
	score, passed, _ = submitBisection(t, `[{"root":0.7390851332151607}]`, uc)
	if score != 100 || !passed {
		t.Errorf("换题后正确根应 100/passed, got %d/%v", score, passed)
	}
}

func TestScoreBisection_MissingPassScoreAlwaysPasses(t *testing.T) {
	// 契约约定（002 注释）：target 缺 pass_score 时恒过关
	target := `{"method":"root_rel_err","func_key":"cubic_1","interval":[1.0,1.5]}`
	_, passed, detail, err := scoreByExperiment("bisection", []byte(`[{"root":2.0}]`), []byte(target), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !passed {
		t.Errorf("缺 pass_score 应恒过关 (detail=%s)", detail)
	}
}

func TestScoreBisection_BadInput(t *testing.T) {
	cases := []struct {
		name, readings, target string
	}{
		{"缺 root 字段", `[{"iterations":19}]`, bisectionTarget},
		{"root 非法 JSON", `[{"root":NaN}]`, bisectionTarget},
		{"readings 元素非对象", `[1.3]`, bisectionTarget},
		{"未知函数", `[{"root":1.3}]`, `{"method":"root_rel_err","func_key":"nope","interval":[1.0,1.5]}`},
		{"区间不夹根", `[{"root":1.3}]`, `{"method":"root_rel_err","func_key":"cubic_1","interval":[2.0,3.0]}`},
		{"区间逆序", `[{"root":1.3}]`, `{"method":"root_rel_err","func_key":"cubic_1","interval":[1.5,1.0]}`},
		{"target 非法 JSON", `[{"root":1.3}]`, `{`},
	}
	for _, c := range cases {
		if _, _, _, err := scoreByExperiment("bisection", []byte(c.readings), []byte(c.target), nil); err == nil {
			t.Errorf("%s: want error, got nil", c.name)
		}
	}
}

// ========== L2 牛顿迭代法求根 ==========

const newtonTarget = `{"method":"newton_rel_err","func_key":"cubic_1","interval":[1.0,1.5],"x0":1.5,"tolerance":1e-6,"max_iterations":50,"pass_score":60}`

func submitNewton(t *testing.T, readings, userConfig string) (int, bool, string) {
	t.Helper()
	uc := []byte(nil)
	if userConfig != "" {
		uc = []byte(userConfig)
	}
	score, passed, detail, err := scoreByExperiment("newton_iter", []byte(readings), []byte(newtonTarget), uc)
	if err != nil {
		t.Fatalf("readings=%s user_config=%s: %v", readings, userConfig, err)
	}
	return score, passed, detail
}

func TestScoreNewtonIter_CorrectFastConvergence(t *testing.T) {
	// x0=1.5、|x_{n+1}-x_n|<1e-6 准则下牛顿 4 步收敛（二分要 19 步）
	score, passed, detail := submitNewton(t, `[{"root":1.324717957244746,"iterations":4,"x0":1.5}]`, "")
	if score != 100 || !passed {
		t.Fatalf("want 100/passed, got %d/%v (detail=%s)", score, passed, detail)
	}
	if !strings.Contains(detail, "expected_iterations=4") {
		t.Errorf("detail 应含 expected_iterations=4: %s", detail)
	}
}

func TestScoreNewtonIter_BisectionPaddingPenalized(t *testing.T) {
	// 根正确但报 19 次迭代（二分法冒充）：罚 4x(19-4-2)=52 -> 48 不及格
	score, passed, detail := submitNewton(t, `[{"root":1.324717957244746,"iterations":19,"x0":1.5}]`, "")
	if score != 48 || passed {
		t.Fatalf("want 48/not-passed, got %d/%v (detail=%s)", score, passed, detail)
	}
	if !strings.Contains(detail, "penalty=52") {
		t.Errorf("detail 应含 penalty=52: %s", detail)
	}
}

func TestScoreNewtonIter_RootErrOnly(t *testing.T) {
	// 根偏 0.4% 但迭代数正常：沿用打分曲线 71 分
	score, passed, _ := submitNewton(t, `[{"root":1.33,"iterations":5,"x0":1.5}]`, "")
	if score != 71 || !passed {
		t.Fatalf("want 71/passed, got %d/%v", score, passed)
	}
}

func TestScoreNewtonIter_ChangeFuncSqrt2(t *testing.T) {
	// 换题 sqrt_2：正初值收敛到 +√2
	score, passed, detail := submitNewton(t, `[{"root":1.4142135623730951,"iterations":5}]`, `{"func_key":"sqrt_2","x0":1.0}`)
	if score != 100 || !passed {
		t.Fatalf("sqrt_2 正初值: want 100/passed, got %d/%v (detail=%s)", score, passed, detail)
	}
	// 负初值收敛到 -√2，按学生实际路径评分（不是死对 target 区间正根）
	score, passed, detail = submitNewton(t, `[{"root":-1.4142135623730951,"iterations":5}]`, `{"func_key":"sqrt_2","x0":-1.0}`)
	if score != 100 || !passed {
		t.Fatalf("sqrt_2 负初值: want 100/passed, got %d/%v (detail=%s)", score, passed, detail)
	}
}

func TestScoreNewtonIter_ZeroDerivativeFallsBackToBisect(t *testing.T) {
	// x0=0 时 f'(0)=2x=0，牛顿中断 -> 回退 target 区间 [1,1.5] 二分真值 √2，且不罚迭代分
	score, passed, detail := submitNewton(t, `[{"root":1.4142135623730951,"iterations":5,"x0":0}]`, `{"func_key":"sqrt_2"}`)
	if score != 100 || !passed {
		t.Fatalf("want 100/passed, got %d/%v (detail=%s)", score, passed, detail)
	}
	if strings.Contains(detail, "expected_iterations") {
		t.Errorf("回退路径不应有 expected_iterations: %s", detail)
	}
}

func TestScoreNewtonIter_FallbackNoBracketIsError(t *testing.T) {
	// 牛顿中断（x0=0, f'=0）且兜底区间不夹根 -> 400
	target := `{"method":"newton_rel_err","func_key":"sqrt_2","interval":[2.0,3.0],"x0":0,"tolerance":1e-6,"max_iterations":50,"pass_score":60}`
	if _, _, _, err := scoreByExperiment("newton_iter", []byte(`[{"root":1.41,"iterations":5}]`), []byte(target), nil); err == nil {
		t.Fatal("牛顿中断且区间不夹根: want error, got nil")
	}
}

func TestScoreNewtonIter_BadInput(t *testing.T) {
	cases := []struct {
		name, readings string
	}{
		{"缺 root", `[{"iterations":5}]`},
		{"root 非法 JSON", `[{"root":NaN}]`},
		{"iterations 为 0", `[{"root":1.324718,"iterations":0}]`},
		{"iterations 超 max", `[{"root":1.324718,"iterations":51}]`},
		{"x0 非法 JSON", `[{"root":1.324718,"x0":abc}]`},
	}
	for _, c := range cases {
		if _, _, _, err := scoreByExperiment("newton_iter", []byte(c.readings), []byte(newtonTarget), nil); err == nil {
			t.Errorf("%s: want error, got nil", c.name)
		}
	}
	// 未知函数
	if _, _, _, err := scoreByExperiment("newton_iter", []byte(`[{"root":1.3}]`), []byte(`{"method":"newton_rel_err","func_key":"nope","interval":[1.0,1.5],"x0":1.5}`), nil); err == nil {
		t.Error("未知函数: want error, got nil")
	}
}

// ========== L3 高斯消去法解线性方程组 ==========

const gaussTarget = `{"method":"linear_rel_err","system_key":"sys3_basic","pass_score":60}`

func submitGauss(t *testing.T, readings, userConfig string) (int, bool, string) {
	t.Helper()
	uc := []byte(nil)
	if userConfig != "" {
		uc = []byte(userConfig)
	}
	score, passed, detail, err := scoreByExperiment("gauss_elim", []byte(readings), []byte(gaussTarget), uc)
	if err != nil {
		t.Fatalf("readings=%s user_config=%s: %v", readings, userConfig, err)
	}
	return score, passed, detail
}

func TestScoreGaussElim_ExactSolution(t *testing.T) {
	// sys3_basic 真值解恰为整数 (2, 3, -1)
	score, passed, detail := submitGauss(t, `[{"solution":[2,3,-1]}]`, "")
	if score != 100 || !passed {
		t.Fatalf("want 100/passed, got %d/%v (detail=%s)", score, passed, detail)
	}
	if !strings.Contains(detail, "system=sys3_basic") || !strings.Contains(detail, "n=3") {
		t.Errorf("detail 应记录 system 与 n: %s", detail)
	}
}

func TestScoreGaussElim_RelErrCurve(t *testing.T) {
	// 真值 [2,3,-1]，||x*||inf=3；在 x1 上注入偏差，relErr = 偏差/3
	cases := []struct {
		name string
		sol  string
		want int
	}{
		{"偏差 1e-7 -> 100", `[{"solution":[2.0000001,3,-1]}]`, 100}, // relErr≈3.3e-8
		{"偏差 0.003 -> 80", `[{"solution":[2.003,3,-1]}]`, 80},      // relErr=1e-3 拐点
		{"偏差 0.3 -> 30", `[{"solution":[2.3,3,-1]}]`, 30},          // relErr=0.1 拐点
		{"全错 -> 0", `[{"solution":[5,6,7]}]`, 0},                   // relErr=8/3
	}
	for _, c := range cases {
		score, _, _ := submitGauss(t, c.sol, "")
		if score != c.want {
			t.Errorf("%s: want %d, got %d", c.name, c.want, score)
		}
	}
}

func TestScoreGaussElim_UserConfigSystemKey(t *testing.T) {
	// 换库内方程组 sys4_tridiag，真值解 [1,-1,2,0.5]
	score, passed, _ := submitGauss(t, `[{"solution":[1,-1,2,0.5]}]`, `{"system_key":"sys4_tridiag"}`)
	if score != 100 || !passed {
		t.Fatalf("换题后正确解: want 100/passed, got %d/%v", score, passed)
	}
	// 换题后仍交旧解（补 0 凑 4 维）-> relErr=4/2=2 -> 0 分（防背答案）
	score, _, _ = submitGauss(t, `[{"solution":[2,3,-1,0]}]`, `{"system_key":"sys4_tridiag"}`)
	if score != 0 {
		t.Fatalf("换题后旧解: want 0, got %d", score)
	}
}

func TestScoreGaussElim_UserConfigCustomMatrix(t *testing.T) {
	// 前端自定义方程组 x+y=3, x-y=1 -> 解 [2,1]（A/b 优先于 system_key）
	score, _, _ := submitGauss(t, `[{"solution":[2,1]}]`, `{"A":[[1,1],[1,-1]],"b":[3,1]}`)
	if score != 100 {
		t.Fatalf("自定义方程组正确解: want 100, got %d", score)
	}
}

func TestScoreGaussElim_HilbertStability(t *testing.T) {
	// 4x4 Hilbert 病态矩阵：列主元消去双精度真值解与精确解全 1 差 ~1e-12
	score, _, detail := submitGauss(t, `[{"solution":[1,1,1,1]}]`, `{"system_key":"hilbert4"}`)
	if score != 100 {
		t.Fatalf("hilbert4 精确解: want 100, got %d (detail=%s)", score, detail)
	}
}

func TestScoreGaussElim_BandsOverride(t *testing.T) {
	// target.bands 覆盖默认曲线：5% 内满分（物理量级容差演示）
	target := `{"method":"linear_rel_err","system_key":"sys3_basic","pass_score":60,"bands":[[0.05,100],[0.5,60],[2,0]]}`
	// solution [2.5,3,-1]：relErr=0.5/3≈0.167，log10 插值 100-40*log10(3.33)≈79
	score, _, _, _ := scoreByExperiment("gauss_elim", []byte(`[{"solution":[2.5,3,-1]}]`), []byte(target), nil)
	if score != 79 {
		t.Fatalf("bands 覆盖: want 79, got %d", score)
	}
}

func TestScoreGaussElim_SingularMatrixIsError(t *testing.T) {
	// 奇异矩阵（第 2 行 = 2 x 第 1 行）-> 400
	_, _, _, err := scoreByExperiment("gauss_elim", []byte(`[{"solution":[1,2]}]`),
		[]byte(gaussTarget), []byte(`{"A":[[1,2],[2,4]],"b":[3,6]}`))
	if err == nil {
		t.Fatal("奇异矩阵: want error, got nil")
	}
}

func TestScoreGaussElim_BadInput(t *testing.T) {
	cases := []struct {
		name, readings, userConfig string
	}{
		{"缺 solution", `[{"root":1}]`, ""},
		{"solution 长度不匹配", `[{"solution":[2,3]}]`, ""},
		{"solution 含 NaN", `[{"solution":[2,3,NaN]}]`, ""},
		{"非法 JSON", `[{"solution":abc}]`, ""},
		{"未知方程组", `[{"solution":[2,3,-1]}]`, `{"system_key":"nope"}`},
		{"自定义 1 阶", `[{"solution":[1]}]`, `{"A":[[1]],"b":[1]}`},
		{"自定义 9 阶", `[{"solution":[1,2,3,4,5,6,7,8,9]}]`, `{"A":[[1,2,3,4,5,6,7,8,9],[1,2,3,4,5,6,7,8,9],[1,2,3,4,5,6,7,8,9],[1,2,3,4,5,6,7,8,9],[1,2,3,4,5,6,7,8,9],[1,2,3,4,5,6,7,8,9],[1,2,3,4,5,6,7,8,9],[1,2,3,4,5,6,7,8,9],[1,2,3,4,5,6,7,8,9]],"b":[1,2,3,4,5,6,7,8,9]}`},
		{"自定义缺 b", `[{"solution":[2,1]}]`, `{"A":[[1,1],[1,-1]]}`},
		{"自定义非方阵", `[{"solution":[2,1]}]`, `{"A":[[1,1,2],[1,-1,3]],"b":[3,1]}`},
	}
	for _, c := range cases {
		uc := []byte(nil)
		if c.userConfig != "" {
			uc = []byte(c.userConfig)
		}
		if _, _, _, err := scoreByExperiment("gauss_elim", []byte(c.readings), []byte(gaussTarget), uc); err == nil {
			t.Errorf("%s: want error, got nil", c.name)
		}
	}
}

// ========== L4 雅可比迭代法解线性方程组 ==========

const jacobiTarget = `{"method":"jacobi_rel_err","system_key":"sys3_diagdom","x0":[0,0,0],"tolerance":1e-6,"max_iterations":100,"pass_score":60}`

func submitJacobi(t *testing.T, readings, userConfig string) (int, bool, string) {
	t.Helper()
	uc := []byte(nil)
	if userConfig != "" {
		uc = []byte(userConfig)
	}
	score, passed, detail, err := scoreByExperiment("jacobi_iter", []byte(readings), []byte(jacobiTarget), uc)
	if err != nil {
		t.Fatalf("readings=%s user_config=%s: %v", readings, userConfig, err)
	}
	return score, passed, detail
}

// jacobiExpected 后端模拟 sys3_diagdom 从零向量的雅可比迭代数（罚分基准，约 14 步）。
func jacobiExpected(t *testing.T) int {
	t.Helper()
	def := linSystems["sys3_diagdom"]
	x, k, err := jacobiSolve(def.A, def.b, nil, 1e-6, 100)
	if err != nil || len(x) != 3 {
		t.Fatalf("jacobiSolve: err=%v len=%d", err, len(x))
	}
	return k
}

func TestScoreJacobiIter_ExactSolution(t *testing.T) {
	// sys3_diagdom 真值解恰为整数 (1, 2, 3)
	score, passed, detail := submitJacobi(t, `[{"solution":[1,2,3]}]`, "")
	if score != 100 || !passed {
		t.Fatalf("want 100/passed, got %d/%v (detail=%s)", score, passed, detail)
	}
	if !strings.Contains(detail, "system=sys3_diagdom") || !strings.Contains(detail, "n=3") {
		t.Errorf("detail 应记录 system 与 n: %s", detail)
	}
}

func TestScoreJacobiIter_HonestIterationsNoPenalty(t *testing.T) {
	exp := jacobiExpected(t)
	// 如实报期望迭代数：不罚
	readings := fmt.Sprintf(`[{"solution":[1,2,3],"iterations":%d}]`, exp)
	if score, _, detail := submitJacobi(t, readings, ""); score != 100 {
		t.Fatalf("期望迭代数不罚: want 100, got %d (detail=%s)", score, detail)
	}
	// 高斯-赛德尔量级（约一半）：合法实现，也不罚
	readings = fmt.Sprintf(`[{"solution":[1,2,3],"iterations":%d}]`, exp/2)
	if score, _, detail := submitJacobi(t, readings, ""); score != 100 {
		t.Fatalf("半量迭代不罚: want 100, got %d (detail=%s)", score, detail)
	}
}

func TestScoreJacobiIter_DirectMethodPenalized(t *testing.T) {
	// 直接消去得精确解后冒充 1 次雅可比迭代 -> 罚 4 x (expected/2 - 3)
	exp := jacobiExpected(t)
	want := 100 - 4*(exp/2-2-1)
	score, _, detail := submitJacobi(t, `[{"solution":[1,2,3],"iterations":1}]`, "")
	if score != want {
		t.Fatalf("want %d, got %d (detail=%s)", want, score, detail)
	}
	if !strings.Contains(detail, "penalty=") {
		t.Errorf("detail 应记录 penalty: %s", detail)
	}
}

func TestScoreJacobiIter_RelErrCurve(t *testing.T) {
	// 真值 [1,2,3]，||x*||inf=3；在 x1 上注入偏差，relErr = 偏差/3
	cases := []struct {
		name string
		sol  string
		want int
	}{
		{"偏差 0.003 -> 80", `[{"solution":[1.003,2,3]}]`, 80}, // relErr=1e-3 拐点
		{"偏差 0.03 -> 65", `[{"solution":[1.03,2,3]}]`, 65},   // relErr=1e-2 拐点
		{"偏差 0.3 -> 30", `[{"solution":[1.3,2,3]}]`, 30},     // relErr=0.1 拐点
		{"全错 -> 0", `[{"solution":[4,5,6]}]`, 0},             // relErr=1
	}
	for _, c := range cases {
		score, _, _ := submitJacobi(t, c.sol, "")
		if score != c.want {
			t.Errorf("%s: want %d, got %d", c.name, c.want, score)
		}
	}
}

func TestScoreJacobiIter_UserConfigSystemKey(t *testing.T) {
	// 换 sys4_tridiag（对角占优，雅可比同样收敛），真值解 [1,-1,2,0.5]
	score, passed, _ := submitJacobi(t, `[{"solution":[1,-1,2,0.5]}]`, `{"system_key":"sys4_tridiag"}`)
	if score != 100 || !passed {
		t.Fatalf("换题后正确解: want 100/passed, got %d/%v", score, passed)
	}
	// 换题后仍交原方程组的解（补 0 凑 4 维）-> relErr=4/2=2 -> 0 分（防背答案）
	score, _, _ = submitJacobi(t, `[{"solution":[1,2,3,0]}]`, `{"system_key":"sys4_tridiag"}`)
	if score != 0 {
		t.Fatalf("换题后旧解: want 0, got %d", score)
	}
}

func TestScoreJacobiIter_UserConfigCustomMatrix(t *testing.T) {
	// 前端自定义对角占优方程组 5x+y=6, x+4y=5 -> 解 [1,1]
	score, _, _ := submitJacobi(t, `[{"solution":[1,1]}]`, `{"A":[[5,1],[1,4]],"b":[6,5]}`)
	if score != 100 {
		t.Fatalf("自定义方程组正确解: want 100, got %d", score)
	}
}

func TestScoreJacobiIter_NonConvergentNoExpectedNoPenalty(t *testing.T) {
	// 非对角占优自定义系统：后端模拟不收敛 -> 无期望迭代数、不罚分，仍按精度计分
	score, _, detail := submitJacobi(t, `[{"solution":[2,1],"iterations":1}]`, `{"A":[[1,1],[1,-1]],"b":[3,1]}`)
	if score != 100 {
		t.Fatalf("want 100, got %d (detail=%s)", score, detail)
	}
	if strings.Contains(detail, "expected_iterations") {
		t.Errorf("不应有 expected_iterations: %s", detail)
	}
}

func TestScoreJacobiIter_ReadingsX0(t *testing.T) {
	// 初值即真值：后端从同一 x0 模拟，期望迭代数=1，如实报 1 次不罚
	score, _, detail := submitJacobi(t, `[{"solution":[1,2,3],"iterations":1,"x0":[1,2,3]}]`, "")
	if score != 100 {
		t.Fatalf("want 100, got %d (detail=%s)", score, detail)
	}
	if !strings.Contains(detail, "x0=[1,2,3]") {
		t.Errorf("detail 应记录 x0: %s", detail)
	}
}

func TestScoreJacobiIter_BadInput(t *testing.T) {
	cases := []struct {
		name, readings, userConfig string
	}{
		{"缺 solution", `[{"root":1}]`, ""},
		{"solution 长度不匹配", `[{"solution":[1,2]}]`, ""},
		{"solution 含 NaN", `[{"solution":[1,2,NaN]}]`, ""},
		{"非法 JSON", `[{"solution":abc}]`, ""},
		{"未知方程组", `[{"solution":[1,2,3]}]`, `{"system_key":"nope"}`},
		{"自定义 1 阶", `[{"solution":[1]}]`, `{"A":[[1]],"b":[1]}`},
		{"自定义 9 阶", `[{"solution":[1,2,3,4,5,6,7,8,9]}]`, `{"A":[[1,2,3,4,5,6,7,8,9],[1,2,3,4,5,6,7,8,9],[1,2,3,4,5,6,7,8,9],[1,2,3,4,5,6,7,8,9],[1,2,3,4,5,6,7,8,9],[1,2,3,4,5,6,7,8,9],[1,2,3,4,5,6,7,8,9],[1,2,3,4,5,6,7,8,9],[1,2,3,4,5,6,7,8,9]],"b":[1,2,3,4,5,6,7,8,9]}`},
		{"自定义非方阵", `[{"solution":[2,1]}]`, `{"A":[[1,1,2],[1,-1,3]],"b":[3,1]}`},
		{"自定义奇异", `[{"solution":[1,2]}]`, `{"A":[[1,2],[2,4]],"b":[3,6]}`},
		{"自定义零对角元", `[{"solution":[1,2]}]`, `{"A":[[0,1],[1,2]],"b":[2,5]}`},
		{"自定义缺 b", `[{"solution":[2,1]}]`, `{"A":[[5,1],[1,4]]}`},
		{"iterations 为 0", `[{"solution":[1,2,3],"iterations":0}]`, ""},
		{"iterations 超上限", `[{"solution":[1,2,3],"iterations":101}]`, ""},
		{"readings x0 长度错", `[{"solution":[1,2,3],"x0":[0,0]}]`, ""},
	}
	for _, c := range cases {
		uc := []byte(nil)
		if c.userConfig != "" {
			uc = []byte(c.userConfig)
		}
		if _, _, _, err := scoreByExperiment("jacobi_iter", []byte(c.readings), []byte(jacobiTarget), uc); err == nil {
			t.Errorf("%s: want error, got nil", c.name)
		}
	}
}

// ========== L5 SOR 逐次超松弛迭代法解线性方程组 ==========

const sorTarget = `{"method":"sor_rel_err","system_key":"sys6_poisson","x0":[0,0,0,0,0,0],"omega":1.0,"tolerance":1e-6,"max_iterations":100,"pass_score":60}`

func submitSor(t *testing.T, readings, userConfig string) (int, bool, string) {
	t.Helper()
	uc := []byte(nil)
	if userConfig != "" {
		uc = []byte(userConfig)
	}
	score, passed, detail, err := scoreByExperiment("sor_iter", []byte(readings), []byte(sorTarget), uc)
	if err != nil {
		t.Fatalf("readings=%s user_config=%s: %v", readings, userConfig, err)
	}
	return score, passed, detail
}

// sorExpected 后端模拟 sys6_poisson 从零向量、给定 ω 的 SOR 迭代数（罚分基准）。
// ω=1（高斯-赛德尔）约 61 步，ω≈1.4 最优约 19 步；雅可比要 114 步（超 100 上限不收敛）。
func sorExpected(t *testing.T, omega float64) int {
	t.Helper()
	def := linSystems["sys6_poisson"]
	x, k, err := sorSolve(def.A, def.b, nil, omega, 1e-6, 100)
	if err != nil || len(x) != 6 {
		t.Fatalf("sorSolve(omega=%g): err=%v len=%d", omega, err, len(x))
	}
	return k
}

func TestScoreSorIter_ExactSolution(t *testing.T) {
	// sys6_poisson 真值解为全 1（一维泊松方程差分，边界驱动）
	score, passed, detail := submitSor(t, `[{"solution":[1,1,1,1,1,1]}]`, "")
	if score != 100 || !passed {
		t.Fatalf("want 100/passed, got %d/%v (detail=%s)", score, passed, detail)
	}
	if !strings.Contains(detail, "system=sys6_poisson") || !strings.Contains(detail, "n=6") || !strings.Contains(detail, "omega=1") {
		t.Errorf("detail 应记录 system/n/omega: %s", detail)
	}
}

func TestScoreSorIter_HonestIterationsNoPenalty(t *testing.T) {
	// 如实报期望迭代数：不罚
	readings := fmt.Sprintf(`[{"solution":[1,1,1,1,1,1],"iterations":%d}]`, sorExpected(t, 1.0))
	if score, _, detail := submitSor(t, readings, ""); score != 100 {
		t.Fatalf("期望迭代数不罚: want 100, got %d (detail=%s)", score, detail)
	}
}

func TestScoreSorIter_OptimalOmegaFasterNoPenalty(t *testing.T) {
	// SOR 核心教学点：ω≈1.4 最优松弛因子把 61 步压到 ~19 步；
	// 期望迭代数按学生提交的 ω 现算，换 ω 后如实报数不罚
	exp := sorExpected(t, 1.4)
	if exp > 25 {
		t.Fatalf("ω=1.4 应约 19 步（<=25），got %d", exp)
	}
	readings := fmt.Sprintf(`[{"solution":[1,1,1,1,1,1],"iterations":%d,"omega":1.4}]`, exp)
	score, _, detail := submitSor(t, readings, "")
	if score != 100 {
		t.Fatalf("最优 ω 如实报数: want 100, got %d (detail=%s)", score, detail)
	}
	if !strings.Contains(detail, "omega=1.4") {
		t.Errorf("detail 应记录学生 ω: %s", detail)
	}
	// ω 记在 user_config 同样生效（前端滑块换 ω 后经 user_config 提交）
	readings = fmt.Sprintf(`[{"solution":[1,1,1,1,1,1],"iterations":%d}]`, exp)
	if score, _, _ := submitSor(t, readings, `{"omega":1.4}`); score != 100 {
		t.Fatalf("user_config.omega: want 100, got %d", score)
	}
}

func TestScoreSorIter_GaussSeidelFakeFastConvergencePenalized(t *testing.T) {
	// ω=1（高斯-赛德尔）却只报 19 步（仅最优 ω 可达）-> 罚 4 x (61/2 - 2 - 19) = 36
	exp := sorExpected(t, 1.0)
	want := 100 - 4*(exp/2 - 2 - 19)
	if want < 0 {
		want = 0
	}
	score, _, detail := submitSor(t, `[{"solution":[1,1,1,1,1,1],"iterations":19}]`, "")
	if score != want {
		t.Fatalf("want %d, got %d (detail=%s)", want, score, detail)
	}
	if !strings.Contains(detail, "penalty=") {
		t.Errorf("detail 应记录 penalty: %s", detail)
	}
}

func TestScoreSorIter_DirectMethodPenalized(t *testing.T) {
	// 直接消去得精确解后冒充 1 次 SOR 迭代 -> 罚 4 x (expected/2 - 2 - 1)
	exp := sorExpected(t, 1.0)
	want := 100 - 4*(exp/2 - 2 - 1)
	if want < 0 {
		want = 0
	}
	score, _, detail := submitSor(t, `[{"solution":[1,1,1,1,1,1],"iterations":1}]`, "")
	if score != want {
		t.Fatalf("want %d, got %d (detail=%s)", want, score, detail)
	}
	if !strings.Contains(detail, "penalty=") {
		t.Errorf("detail 应记录 penalty: %s", detail)
	}
}

func TestScoreSorIter_RelErrCurve(t *testing.T) {
	// 真值全 1，||x*||inf=1；在 x1 上注入偏差，relErr = 偏差
	cases := []struct {
		name string
		sol  string
		want int
	}{
		{"偏差 1e-3 -> 80", `[{"solution":[1.001,1,1,1,1,1]}]`, 80},  // relErr=1e-3 拐点
		{"偏差 1e-2 -> 65", `[{"solution":[1.01,1,1,1,1,1]}]`, 65},   // relErr=1e-2 拐点
		{"偏差 0.1 -> 30", `[{"solution":[1.1,1,1,1,1,1]}]`, 30},     // relErr=0.1 拐点
		{"全错 -> 0", `[{"solution":[2,2,2,2,2,2]}]`, 0},             // relErr=1
	}
	for _, c := range cases {
		score, _, _ := submitSor(t, c.sol, "")
		if score != c.want {
			t.Errorf("%s: want %d, got %d", c.name, c.want, score)
		}
	}
}

func TestScoreSorIter_UserConfigSystemKey(t *testing.T) {
	// 换 sys3_diagdom（对角占优，SOR 同样收敛），真值解 [1,2,3]；
	// target.x0 为 6 维与 3 阶系统不符，应静默回退零向量而非报错
	score, passed, _ := submitSor(t, `[{"solution":[1,2,3]}]`, `{"system_key":"sys3_diagdom"}`)
	if score != 100 || !passed {
		t.Fatalf("换题后正确解: want 100/passed, got %d/%v", score, passed)
	}
	// 换题后仍交原方程组的全 1 解 -> relErr=2/3 -> 0 分（防背答案）
	score, _, _ = submitSor(t, `[{"solution":[1,1,1]}]`, `{"system_key":"sys3_diagdom"}`)
	if score != 0 {
		t.Fatalf("换题后旧解: want 0, got %d", score)
	}
}

func TestScoreSorIter_UserConfigCustomMatrix(t *testing.T) {
	// 前端自定义对角占优方程组 5x+y=6, x+4y=5 -> 解 [1,1]，带 ω=1.2
	score, _, detail := submitSor(t, `[{"solution":[1,1],"omega":1.2}]`, `{"A":[[5,1],[1,4]],"b":[6,5]}`)
	if score != 100 {
		t.Fatalf("自定义方程组正确解: want 100, got %d (detail=%s)", score, detail)
	}
}

func TestScoreSorIter_NonConvergentNoExpectedNoPenalty(t *testing.T) {
	// 非对角占优自定义系统：后端模拟不收敛 -> 无期望迭代数、不罚分，仍按精度计分
	score, _, detail := submitSor(t, `[{"solution":[2,1],"iterations":1}]`, `{"A":[[1,1],[1,-1]],"b":[3,1]}`)
	if score != 100 {
		t.Fatalf("want 100, got %d (detail=%s)", score, detail)
	}
	if strings.Contains(detail, "expected_iterations") {
		t.Errorf("不应有 expected_iterations: %s", detail)
	}
}

func TestScoreSorIter_OverRelaxTooBigNoConvergenceNoPenalty(t *testing.T) {
	// ω 过大（1.95）：收敛半径 0.95，100 步内不收敛 -> 无期望、不罚分，按精度计分
	score, _, detail := submitSor(t, `[{"solution":[1,1,1,1,1,1],"iterations":50,"omega":1.95}]`, "")
	if score != 100 {
		t.Fatalf("want 100, got %d (detail=%s)", score, detail)
	}
	if strings.Contains(detail, "expected_iterations") {
		t.Errorf("不应有 expected_iterations: %s", detail)
	}
}

func TestScoreSorIter_ReadingsX0(t *testing.T) {
	// 初值即真值：后端从同一 x0 模拟，期望迭代数=1，如实报 1 次不罚
	score, _, detail := submitSor(t, `[{"solution":[1,1,1,1,1,1],"iterations":1,"x0":[1,1,1,1,1,1]}]`, "")
	if score != 100 {
		t.Fatalf("want 100, got %d (detail=%s)", score, detail)
	}
	if !strings.Contains(detail, "x0=[1,1,1,1,1,1]") {
		t.Errorf("detail 应记录 x0: %s", detail)
	}
}

func TestScoreSorIter_OmegaPriorityReadingsOverUserConfig(t *testing.T) {
	// readings.omega 优先于 user_config.omega：期望按 readings 的 1.3 现算
	score, _, detail := submitSor(t, `[{"solution":[1,1,1,1,1,1],"omega":1.3}]`, `{"omega":1.4}`)
	if score != 100 {
		t.Fatalf("want 100, got %d (detail=%s)", score, detail)
	}
	if !strings.Contains(detail, "omega=1.3") {
		t.Errorf("readings.omega 应优先: %s", detail)
	}
}

func TestScoreSorIter_BadInput(t *testing.T) {
	cases := []struct {
		name, readings, userConfig string
	}{
		{"缺 solution", `[{"root":1}]`, ""},
		{"solution 长度不匹配", `[{"solution":[1,1,1]}]`, ""},
		{"solution 含 NaN", `[{"solution":[1,1,1,1,1,NaN]}]`, ""},
		{"非法 JSON", `[{"solution":abc}]`, ""},
		{"未知方程组", `[{"solution":[1,1,1,1,1,1]}]`, `{"system_key":"nope"}`},
		{"自定义 1 阶", `[{"solution":[1]}]`, `{"A":[[1]],"b":[1]}`},
		{"自定义 9 阶", `[{"solution":[1,2,3,4,5,6,7,8,9]}]`, `{"A":[[1,2,3,4,5,6,7,8,9],[1,2,3,4,5,6,7,8,9],[1,2,3,4,5,6,7,8,9],[1,2,3,4,5,6,7,8,9],[1,2,3,4,5,6,7,8,9],[1,2,3,4,5,6,7,8,9],[1,2,3,4,5,6,7,8,9],[1,2,3,4,5,6,7,8,9],[1,2,3,4,5,6,7,8,9]],"b":[1,2,3,4,5,6,7,8,9]}`},
		{"自定义非方阵", `[{"solution":[2,1]}]`, `{"A":[[1,1,2],[1,-1,3]],"b":[3,1]}`},
		{"自定义奇异", `[{"solution":[1,2]}]`, `{"A":[[1,2],[2,4]],"b":[3,6]}`},
		{"自定义零对角元", `[{"solution":[1,2]}]`, `{"A":[[0,1],[1,2]],"b":[2,5]}`},
		{"自定义缺 b", `[{"solution":[2,1]}]`, `{"A":[[5,1],[1,4]]}`},
		{"iterations 为 0", `[{"solution":[1,1,1,1,1,1],"iterations":0}]`, ""},
		{"iterations 超上限", `[{"solution":[1,1,1,1,1,1],"iterations":101}]`, ""},
		{"readings x0 长度错", `[{"solution":[1,1,1,1,1,1],"x0":[0,0]}]`, ""},
		{"omega 为 0", `[{"solution":[1,1,1,1,1,1],"omega":0}]`, ""},
		{"omega 为 2", `[{"solution":[1,1,1,1,1,1],"omega":2}]`, ""},
		{"omega 超界 2.5", `[{"solution":[1,1,1,1,1,1],"omega":2.5}]`, ""},
		{"omega 为负", `[{"solution":[1,1,1,1,1,1],"omega":-0.5}]`, ""},
		{"user_config omega 非法", `[{"solution":[1,1,1,1,1,1]}]`, `{"omega":3}`},
	}
	for _, c := range cases {
		uc := []byte(nil)
		if c.userConfig != "" {
			uc = []byte(c.userConfig)
		}
		if _, _, _, err := scoreByExperiment("sor_iter", []byte(c.readings), []byte(sorTarget), uc); err == nil {
			t.Errorf("%s: want error, got nil", c.name)
		}
	}
}
