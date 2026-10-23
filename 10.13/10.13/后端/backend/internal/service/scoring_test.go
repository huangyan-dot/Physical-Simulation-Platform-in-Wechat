package service

import (
	"fmt"
	"math"
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

// ========== L6 拉格朗日插值 ==========

const lagrangeTarget = `{"method":"interp_rel_err","problem_key":"sin_quad","pass_score":60}`

func submitLagrange(t *testing.T, readings, userConfig string) (int, bool, string) {
	t.Helper()
	uc := []byte(nil)
	if userConfig != "" {
		uc = []byte(userConfig)
	}
	score, passed, detail, err := scoreByExperiment("lagrange_interp", []byte(readings), []byte(lagrangeTarget), uc)
	if err != nil {
		t.Fatalf("readings=%s user_config=%s: %v", readings, userConfig, err)
	}
	return score, passed, detail
}

// interpTrueP 运行时算库内题真值 P(x_eval)（测试与评分共用同一 lagrangeEval，
// 另以 sin_quad 的手工闭式解交叉验证，防"自己证自己"）。
func interpTrueP(t *testing.T, key string, xEval float64) float64 {
	t.Helper()
	def := interpProblems[key]
	if xEval != xEval { // NaN -> 用库内默认求值点
		xEval = def.XEval
	}
	p, err := lagrangeEval(def.Xs, def.Ys, xEval)
	if err != nil {
		t.Fatalf("lagrangeEval(%s, %g): %v", key, xEval, err)
	}
	return p
}

func TestScoreLagrangeInterp_ExactSolution(t *testing.T) {
	// sin_quad P(π/6) = (4√2−1)/9 ≈ 0.517428（闭式解交叉验证）
	wantP := (4*math.Sqrt2 - 1) / 9
	got := interpTrueP(t, "sin_quad", math.Pi/6)
	if math.Abs(got-wantP) > 1e-12 {
		t.Fatalf("sin_quad 真值: want %.12g, got %.12g", wantP, got)
	}
	readings := fmt.Sprintf(`[{"value":%.12g,"basis":[%.12g,%.12g,%.12g]}]`,
		got, 2.0/9, 8.0/9, -1.0/9)
	score, passed, detail := submitLagrange(t, readings, "")
	if score != 100 || !passed {
		t.Fatalf("want 100/passed, got %d/%v (detail=%s)", score, passed, detail)
	}
	if !strings.Contains(detail, "problem=sin_quad") || !strings.Contains(detail, "n=3") {
		t.Errorf("detail 应记录 problem/n: %s", detail)
	}
	if !strings.Contains(detail, "basis=[0.222222,0.888889,-0.111111]") {
		t.Errorf("detail 应记录 basis: %s", detail)
	}
}

func TestScoreLagrangeInterp_RelErrCurve(t *testing.T) {
	p := interpTrueP(t, "sin_quad", math.NaN())
	for _, c := range []struct {
		relErr float64
		want   int
	}{
		{1e-7, 100},
		{1e-4, 90},
		{1e-3, 80},
		{1e-2, 65},
		{0.1, 30},
		{0.5, 0},
	} {
		readings := fmt.Sprintf(`[{"value":%.12g}]`, p*(1+c.relErr))
		if score, _, _ := submitLagrange(t, readings, ""); score != c.want {
			t.Errorf("rel_err=%g: want %d, got %d", c.relErr, c.want, score)
		}
	}
}

func TestScoreLagrangeInterp_RungeAntiCheat(t *testing.T) {
	// 龙格现象题：P(0.8) ≈ −0.379310 而 f(0.8) = 1/17 ≈ 0.058824。
	// 不做插值直接抄原函数值 -> 相对误差 ~1.15 -> 0 分（插值实验天然防抄函数值）
	p := interpTrueP(t, "runge_5", math.NaN())
	if p >= 0 {
		t.Fatalf("runge_5 P(0.8) 应为负（龙格振荡），got %g", p)
	}
	if score, _, _ := submitLagrange(t, fmt.Sprintf(`[{"value":%.12g}]`, p), `{"problem_key":"runge_5"}`); score != 100 {
		t.Fatalf("插值真值: want 100, got %d", score)
	}
	if score, passed, _ := submitLagrange(t, fmt.Sprintf(`[{"value":%.12g}]`, 1.0/17), `{"problem_key":"runge_5"}`); score != 0 || passed {
		t.Fatalf("抄 f 值: want 0/not passed, got %d/%v", score, passed)
	}
}

func TestScoreLagrangeInterp_UserConfigProblemKey(t *testing.T) {
	// 换题 sqrt_table：教材经典，P(115) ≈ 10.722756（vs √115 ≈ 10.723805）
	p := interpTrueP(t, "sqrt_table", math.NaN())
	if math.Abs(p-10.722756) > 1e-6 {
		t.Fatalf("sqrt_table P(115): want ~10.722756, got %g", p)
	}
	score, _, detail := submitLagrange(t, fmt.Sprintf(`[{"value":%.12g}]`, p), `{"problem_key":"sqrt_table"}`)
	if score != 100 {
		t.Fatalf("want 100, got %d (detail=%s)", score, detail)
	}
	if !strings.Contains(detail, "problem=sqrt_table") {
		t.Errorf("detail 应记录换题后的 problem: %s", detail)
	}
}

func TestScoreLagrangeInterp_UserConfigCustomNodes(t *testing.T) {
	// 自定义 3 节点 (0,0),(1,1),(2,4) 插 x²，P(0.5) = 0.25
	score, _, detail := submitLagrange(t, `[{"value":0.25}]`, `{"nodes_x":[0,1,2],"nodes_y":[0,1,4],"x_eval":0.5}`)
	if score != 100 {
		t.Fatalf("want 100, got %d (detail=%s)", score, detail)
	}
	if !strings.Contains(detail, "problem=custom") {
		t.Errorf("detail 应记录 problem=custom: %s", detail)
	}
	// 自定义节点数上下限：1 个 / 9 个 -> 错
	if _, _, _, err := scoreByExperiment("lagrange_interp", []byte(`[{"value":1}]`), []byte(lagrangeTarget), []byte(`{"nodes_x":[1],"nodes_y":[2],"x_eval":1}`)); err == nil {
		t.Error("单节点应报错")
	}
}

func TestScoreLagrangeInterp_UserConfigXEval(t *testing.T) {
	// 覆盖求值点：sin_quad 节点上求值 P(π/4) = √2/2（插值多项式过节点）
	if score, _, _ := submitLagrange(t, fmt.Sprintf(`[{"value":%.12g}]`, math.Sqrt2/2), `{"x_eval":0.7853981633974483}`); score != 100 {
		t.Fatalf("节点处求值: want 100, got %d", score)
	}
	// 非节点处 x_eval=π/3，真值运行时算
	p := interpTrueP(t, "sin_quad", math.Pi/3)
	if score, _, _ := submitLagrange(t, fmt.Sprintf(`[{"value":%.12g}]`, p), `{"x_eval":1.0471975511965976}`); score != 100 {
		t.Fatalf("x_eval 覆盖: want 100, got %d", score)
	}
}

func TestScoreLagrangeInterp_BasisAudit(t *testing.T) {
	// basis 可选：值随意不参与打分，只须长度=节点数且有限，记入审计 detail
	score, _, detail := submitLagrange(t, `[{"value":0.5174281,"basis":[1,2,3]}]`, "")
	if score != 100 {
		t.Fatalf("basis 不影响打分: want 100, got %d", score)
	}
	if !strings.Contains(detail, "basis=[1,2,3]") {
		t.Errorf("detail 应原样记录 basis: %s", detail)
	}
}

func TestScoreLagrangeInterp_BadInput(t *testing.T) {
	cases := []struct {
		name, readings, userConfig string
	}{
		{"缺 value", `[{"basis":[1,2,3]}]`, ""},
		{"value 为 NaN", `[{"value":NaN}]`, ""},
		{"非法 JSON", `[{"value":abc}]`, ""},
		{"basis 长度错", `[{"value":0.5,"basis":[1,2]}]`, ""},
		{"basis 含 NaN", `[{"value":0.5,"basis":[1,NaN,3]}]`, ""},
		{"未知插值题", `[{"value":0.5}]`, `{"problem_key":"nope"}`},
		{"自定义缺 x_eval", `[{"value":0.5}]`, `{"nodes_x":[0,1,2],"nodes_y":[0,1,4]}`},
		{"自定义 x 重复", `[{"value":0.5}]`, `{"nodes_x":[1,1,2],"nodes_y":[0,1,4],"x_eval":1.5}`},
		{"自定义 x/y 不匹配", `[{"value":0.5}]`, `{"nodes_x":[0,1,2],"nodes_y":[0,1],"x_eval":1.5}`},
		{"自定义节点数 9", `[{"value":0.5}]`, `{"nodes_x":[1,2,3,4,5,6,7,8,9],"nodes_y":[1,2,3,4,5,6,7,8,9],"x_eval":5}`},
	}
	for _, c := range cases {
		uc := []byte(nil)
		if c.userConfig != "" {
			uc = []byte(c.userConfig)
		}
		if _, _, _, err := scoreByExperiment("lagrange_interp", []byte(c.readings), []byte(lagrangeTarget), uc); err == nil {
			t.Errorf("%s: want error, got nil", c.name)
		}
	}
}

func TestScoreLagrangeInterp_TargetXEvalOnlyForTargetProblem(t *testing.T) {
	// target 带 sin_quad 的求值点（真库 target 即如此）；user_config 换 runge_5
	// 不带 x_eval 时，应用 runge_5 库内默认 0.8，而非沿用 target 的 0.5236
	// （target 的 x_eval 只属于 target 自己的题）。v0.12.1 修复。
	tgt := `{"method":"interp_rel_err","problem_key":"sin_quad","x_eval":0.5235987755982988,"pass_score":60}`
	p := interpTrueP(t, "runge_5", math.NaN()) // 库内默认 0.8
	score, _, _, err := scoreByExperiment("lagrange_interp",
		[]byte(fmt.Sprintf(`[{"value":%.12g}]`, p)), []byte(tgt), []byte(`{"problem_key":"runge_5"}`))
	if err != nil {
		t.Fatal(err)
	}
	if score != 100 {
		t.Fatalf("换题不带 x_eval 应用库内默认求值点: want 100, got %d", score)
	}
	// 抄 f(0.8)=1/17 应 0 分（相对误差 ~1.15，e2e 曾因此 bug 误判 14 分）
	score, _, _, err = scoreByExperiment("lagrange_interp",
		[]byte(`[{"value":0.0588235294}]`), []byte(tgt), []byte(`{"problem_key":"runge_5"}`))
	if err != nil {
		t.Fatal(err)
	}
	if score != 0 {
		t.Fatalf("换题后抄 f 值: want 0, got %d", score)
	}
	// 换题且显式给 x_eval 时按给定值（0.5 是 runge_5 节点，P=f(0.5)=4/29）
	score, _, _, err = scoreByExperiment("lagrange_interp",
		[]byte(fmt.Sprintf(`[{"value":%.12g}]`, 4.0/29)), []byte(tgt), []byte(`{"problem_key":"runge_5","x_eval":0.5}`))
	if err != nil {
		t.Fatal(err)
	}
	if score != 100 {
		t.Fatalf("换题+显式 x_eval: want 100, got %d", score)
	}
}

// ========== L17 复合辛普森积分 ==========

const simpsonTarget = `{"method":"integ_rel_err","problem_key":"sin_pi","pass_score":60}`

func submitSimpson(t *testing.T, readings, userConfig string) (int, bool, string) {
	t.Helper()
	uc := []byte(nil)
	if userConfig != "" {
		uc = []byte(userConfig)
	}
	score, passed, detail, err := scoreByExperiment("simpson_integration", []byte(readings), []byte(simpsonTarget), uc)
	if err != nil {
		t.Fatalf("readings=%s user_config=%s: %v", readings, userConfig, err)
	}
	return score, passed, detail
}

// simpsonTrueS 运行时算库内题真值 S_n（测试与评分共用同一 compositeSimpson，
// 各题另以手工闭式解交叉验证，防"自己证自己"）。
func simpsonTrueS(t *testing.T, key string, n int) float64 {
	t.Helper()
	def := integProblems[key]
	if n <= 0 {
		n = def.N
	}
	return compositeSimpson(def.F, def.A, def.B, n)
}

func TestScoreSimpsonIntegration_ExactSolution(t *testing.T) {
	// sin_pi n=2 单次辛普森闭式：S = (π/6)(0+4·1+0) = 2π/3 ≈ 2.094395（交叉验证）
	wantS := 2 * math.Pi / 3
	got := simpsonTrueS(t, "sin_pi", 2)
	if math.Abs(got-wantS) > 1e-12 {
		t.Fatalf("sin_pi S_2: want %.12g, got %.12g", wantS, got)
	}
	readings := fmt.Sprintf(`[{"value":%.12g,"odd_sum":1,"even_sum":0}]`, got)
	score, passed, detail := submitSimpson(t, readings, "")
	if score != 100 || !passed {
		t.Fatalf("want 100/passed, got %d/%v (detail=%s)", score, passed, detail)
	}
	if !strings.Contains(detail, "problem=sin_pi") || !strings.Contains(detail, "n=2") {
		t.Errorf("detail 应记录 problem/n: %s", detail)
	}
	if !strings.Contains(detail, "odd_sum=1") || !strings.Contains(detail, "even_sum=0") {
		t.Errorf("detail 应记录 odd/even_sum: %s", detail)
	}
	if !strings.Contains(detail, "ref_I=2") {
		t.Errorf("detail 应记录参考积分 ref_I: %s", detail)
	}
}

func TestScoreSimpsonIntegration_RelErrCurve(t *testing.T) {
	s := simpsonTrueS(t, "sin_pi", 2)
	for _, c := range []struct {
		relErr float64
		want   int
	}{
		{1e-7, 100},
		{1e-4, 90},
		{1e-3, 80},
		{1e-2, 65},
		{0.1, 30},
		{0.5, 0},
	} {
		readings := fmt.Sprintf(`[{"value":%.12g}]`, s*(1+c.relErr))
		if score, _, _ := submitSimpson(t, readings, ""); score != c.want {
			t.Errorf("rel_err=%g: want %d, got %d", c.relErr, c.want, score)
		}
	}
}

func TestScoreSimpsonIntegration_ExactIntegralTrap(t *testing.T) {
	// 不算辛普森直接抄精确积分 I=2：|2−2π/3|/(2π/3) ≈ 4.5% -> ~42 分不过线
	// （真值是"方法的输出"S_n 而非精确积分，同 L16 真值=P 而非 f 的设计）
	score, passed, _ := submitSimpson(t, `[{"value":2}]`, "")
	if passed || score >= 60 {
		t.Fatalf("抄精确积分值: want <60/not passed, got %d/%v", score, passed)
	}
}

func TestScoreSimpsonIntegration_UserConfigProblemKey(t *testing.T) {
	// 换题 erf_gauss n=4：I=(√π/2)·erf(1) 无初等闭式必须数值积分；
	// S_4 手工闭式 (1/12)(1 + 4(e^{−1/16}+e^{−9/16}) + 2e^{−1/4} + e^{−1}) 交叉验证
	wantS := (1.0 + 4*(math.Exp(-1.0/16)+math.Exp(-9.0/16)) + 2*math.Exp(-0.25) + math.Exp(-1)) / 12
	got := simpsonTrueS(t, "erf_gauss", 4)
	if math.Abs(got-wantS) > 1e-12 {
		t.Fatalf("erf_gauss S_4: want %.12g, got %.12g", wantS, got)
	}
	score, _, detail := submitSimpson(t, fmt.Sprintf(`[{"value":%.12g}]`, got), `{"problem_key":"erf_gauss"}`)
	if score != 100 {
		t.Fatalf("want 100, got %d (detail=%s)", score, detail)
	}
	if !strings.Contains(detail, "problem=erf_gauss") {
		t.Errorf("detail 应记录换题后的 problem: %s", detail)
	}
	// 抄精确积分 I ≈ 0.746824：n=4 截断误差小，只得低分但不至于 0（防白嫖满分）
	iErf := math.Sqrt(math.Pi) / 2 * 0.8427007929497149 // erf(1)
	if score, _, _ := submitSimpson(t, fmt.Sprintf(`[{"value":%.12g}]`, iErf), `{"problem_key":"erf_gauss"}`); score >= 100 {
		t.Fatalf("抄精确积分应 <100, got %d", score)
	}

	// 换题 sqrt_x n=8：端点导数奇异收敛慢，S_8 ≈ 0.663079 vs I=2/3（差 3.6e-3）
	got = simpsonTrueS(t, "sqrt_x", 8)
	if math.Abs(got-0.6630793) > 1e-6 {
		t.Fatalf("sqrt_x S_8: want ~0.6630793, got %g", got)
	}
	if score, _, _ := submitSimpson(t, fmt.Sprintf(`[{"value":%.12g}]`, got), `{"problem_key":"sqrt_x"}`); score != 100 {
		t.Fatalf("sqrt_x 正确 S_8: want 100")
	}
	if score, _, _ := submitSimpson(t, `[{"value":0.6666666666666666}]`, `{"problem_key":"sqrt_x"}`); score >= 100 {
		t.Fatalf("sqrt_x 抄精确积分应 <100, got %d", score)
	}
}

func TestScoreSimpsonIntegration_UserConfigN(t *testing.T) {
	// 加密分段 n=8：真值同步变为 S_8（更接近 I=2），正确算仍 100 分
	s8 := simpsonTrueS(t, "sin_pi", 8)
	s2 := simpsonTrueS(t, "sin_pi", 2)
	if math.Abs(s8-2) >= math.Abs(s2-2) {
		t.Fatalf("n=8 应比 n=2 更接近精确积分: |S8-2|=%g |S2-2|=%g", math.Abs(s8-2), math.Abs(s2-2))
	}
	score, _, detail := submitSimpson(t, fmt.Sprintf(`[{"value":%.12g}]`, s8), `{"n":8}`)
	if score != 100 {
		t.Fatalf("want 100, got %d (detail=%s)", score, detail)
	}
	if !strings.Contains(detail, "n=8") {
		t.Errorf("detail 应记录 n=8: %s", detail)
	}
}

func TestScoreSimpsonIntegration_UserConfigInterval(t *testing.T) {
	// 换区间 [0,π/2] n=2：S = (π/12)(4·sin(π/4) + 1) = (π/12)(2√2+1)（闭式交叉验证）
	wantS := (2*math.Sqrt2 + 1) * math.Pi / 12
	got := compositeSimpson(math.Sin, 0, math.Pi/2, 2)
	if math.Abs(got-wantS) > 1e-12 {
		t.Fatalf("[0,π/2] S_2: want %.12g, got %.12g", wantS, got)
	}
	score, _, detail := submitSimpson(t, fmt.Sprintf(`[{"value":%.12g}]`, got), `{"a":0,"b":1.5707963267948966}`)
	if score != 100 {
		t.Fatalf("want 100, got %d (detail=%s)", score, detail)
	}
	if !strings.Contains(detail, "b=1.5708") {
		t.Errorf("detail 应记录区间: %s", detail)
	}
}

func TestScoreSimpsonIntegration_SumsAudit(t *testing.T) {
	// odd_sum/even_sum 可选：值随意不参与打分，只须有限，记入审计 detail
	score, _, detail := submitSimpson(t, `[{"value":2.0943951023931953,"odd_sum":3.14,"even_sum":-2}]`, "")
	if score != 100 {
		t.Fatalf("odd/even_sum 不影响打分: want 100, got %d", score)
	}
	if !strings.Contains(detail, "odd_sum=3.14") || !strings.Contains(detail, "even_sum=-2") {
		t.Errorf("detail 应原样记录 odd/even_sum: %s", detail)
	}
}

func TestScoreSimpsonIntegration_BadInput(t *testing.T) {
	cases := []struct {
		name, readings, userConfig string
	}{
		{"缺 value", `[{"odd_sum":1}]`, ""},
		{"value 为 NaN", `[{"value":NaN}]`, ""},
		{"非法 JSON", `[{"value":abc}]`, ""},
		{"odd_sum 为 NaN", `[{"value":2,"odd_sum":NaN}]`, ""},
		{"未知积分题", `[{"value":2}]`, `{"problem_key":"nope"}`},
		{"n 为奇数", `[{"value":2}]`, `{"n":3}`},
		{"n=1", `[{"value":2}]`, `{"n":1}`},
		{"n 超上限", `[{"value":2}]`, `{"n":1026}`},
		{"区间 a>=b", `[{"value":2}]`, `{"a":1,"b":1}`},
		{"区间只给 a", `[{"value":2}]`, `{"a":0.5}`},
	}
	for _, c := range cases {
		uc := []byte(nil)
		if c.userConfig != "" {
			uc = []byte(c.userConfig)
		}
		if _, _, _, err := scoreByExperiment("simpson_integration", []byte(c.readings), []byte(simpsonTarget), uc); err == nil {
			t.Errorf("%s: want error, got nil", c.name)
		}
	}
}

func TestScoreSimpsonIntegration_TargetParamsOnlyForTargetProblem(t *testing.T) {
	// target 带 sin_pi 的区间/n（真库 target 即如此）；user_config 换 sqrt_x
	// 不带区间/n 时，应用 sqrt_x 库内默认 [0,1] n=8，而非沿用 target 的
	//（target 的题专属参数只属于 target 自己的题，v0.12.1 求值点归属规则同款）
	tgt := `{"method":"integ_rel_err","problem_key":"sin_pi","a":0,"b":3.141592653589793,"n":2,"pass_score":60}`
	s8 := simpsonTrueS(t, "sqrt_x", 8)
	score, _, _, err := scoreByExperiment("simpson_integration",
		[]byte(fmt.Sprintf(`[{"value":%.12g}]`, s8)), []byte(tgt), []byte(`{"problem_key":"sqrt_x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if score != 100 {
		t.Fatalf("换题不带区间/n 应用库内默认: want 100, got %d", score)
	}
	// 若错误沿用 target 的 [0,π] n=2，真值会变成 (π/6)(4√(π/2)+√π) ≈ 3.55，S_8 得 0 分
	// 换题且显式给 n 时按给定值（n=4 的 S_4 运行时算）
	s4 := simpsonTrueS(t, "sqrt_x", 4)
	score, _, _, err = scoreByExperiment("simpson_integration",
		[]byte(fmt.Sprintf(`[{"value":%.12g}]`, s4)), []byte(tgt), []byte(`{"problem_key":"sqrt_x","n":4}`))
	if err != nil {
		t.Fatal(err)
	}
	if score != 100 {
		t.Fatalf("换题+显式 n: want 100, got %d", score)
	}
}

// ========== L18 逐次线性插值法（Aitken/Neville） ==========

const aitkenTarget = `{"method":"aitken_rel_err","problem_key":"sin_quad","pass_score":60}`

func submitAitken(t *testing.T, readings, userConfig string) (int, bool, string) {
	t.Helper()
	uc := []byte(nil)
	if userConfig != "" {
		uc = []byte(userConfig)
	}
	score, passed, detail, err := scoreByExperiment("aitken_interp", []byte(readings), []byte(aitkenTarget), uc)
	if err != nil {
		t.Fatalf("readings=%s user_config=%s: %v", readings, userConfig, err)
	}
	return score, passed, detail
}

// nevilleTrueP 运行时算库内题递推真值（测试与评分共用同一 nevilleEval，
// 另以 lagrangeEval 交叉验证——两路计算同一插值多项式必相等）。
func nevilleTrueP(t *testing.T, key string, xEval float64) float64 {
	t.Helper()
	def := interpProblems[key]
	if xEval != xEval { // NaN -> 用库内默认求值点
		xEval = def.XEval
	}
	p, err := nevilleEval(def.Xs, def.Ys, xEval)
	if err != nil {
		t.Fatalf("nevilleEval(%s, %g): %v", key, xEval, err)
	}
	return p
}

func TestScoreAitkenInterp_NevilleEqualsLagrange(t *testing.T) {
	// 核心交叉验证：Neville 递推与拉格朗日公式评同一插值多项式，
	// 题库全部三题（含自定义 4 节点）两路结果须一致到 1e-12
	for _, key := range []string{"sin_quad", "sqrt_table", "runge_5"} {
		def := interpProblems[key]
		nv, err := nevilleEval(def.Xs, def.Ys, def.XEval)
		if err != nil {
			t.Fatalf("nevilleEval(%s): %v", key, err)
		}
		lg, err := lagrangeEval(def.Xs, def.Ys, def.XEval)
		if err != nil {
			t.Fatalf("lagrangeEval(%s): %v", key, err)
		}
		if math.Abs(nv-lg) > 1e-12 {
			t.Errorf("%s: neville %.15g != lagrange %.15g", key, nv, lg)
		}
	}
	// 自定义 4 节点（含乱序符号的 y）
	xs := []float64{-2, 0, 1, 3}
	ys := []float64{5, -1, 2, 0.5}
	for _, xe := range []float64{-1, 0.7, 2.2} {
		nv, _ := nevilleEval(xs, ys, xe)
		lg, _ := lagrangeEval(xs, ys, xe)
		if math.Abs(nv-lg) > 1e-12 {
			t.Errorf("custom x_eval=%g: neville %.15g != lagrange %.15g", xe, nv, lg)
		}
	}
}

func TestScoreAitkenInterp_ExactSolution(t *testing.T) {
	// sin_quad P(π/6) = (4√2−1)/9（与 L16 同一真值，闭式解交叉验证）；
	// 第 1 列闭式：P_{1,1}=√2/3（(0,0)-(π/4,√2/2) 线性插值在 π/6），
	// P_{2,1}=(2√2−1)/3（(π/4,√2/2)-(π/2,1) 线性插值在 π/6）
	wantP := (4*math.Sqrt2 - 1) / 9
	got := nevilleTrueP(t, "sin_quad", math.Pi/6)
	if math.Abs(got-wantP) > 1e-12 {
		t.Fatalf("sin_quad 递推真值: want %.12g, got %.12g", wantP, got)
	}
	p11 := math.Sqrt2 / 3
	p21 := (2*math.Sqrt2 - 1) / 3
	readings := fmt.Sprintf(`[{"value":%.12g,"col1":[%.12g,%.12g]}]`, got, p11, p21)
	score, passed, detail := submitAitken(t, readings, "")
	if score != 100 || !passed {
		t.Fatalf("want 100/passed, got %d/%v (detail=%s)", score, passed, detail)
	}
	if !strings.Contains(detail, "problem=sin_quad") || !strings.Contains(detail, "n=3") {
		t.Errorf("detail 应记录 problem/n: %s", detail)
	}
	if !strings.Contains(detail, "col1=[0.471405,0.609476]") {
		t.Errorf("detail 应记录 col1: %s", detail)
	}
}

func TestScoreAitkenInterp_RelErrCurve(t *testing.T) {
	p := nevilleTrueP(t, "sin_quad", math.NaN())
	for _, c := range []struct {
		relErr float64
		want   int
	}{
		{1e-7, 100},
		{1e-4, 90},
		{1e-3, 80},
		{1e-2, 65},
		{0.1, 30},
		{0.5, 0},
	} {
		readings := fmt.Sprintf(`[{"value":%.12g}]`, p*(1+c.relErr))
		if score, _, _ := submitAitken(t, readings, ""); score != c.want {
			t.Errorf("rel_err=%g: want %d, got %d", c.relErr, c.want, score)
		}
	}
}

func TestScoreAitkenInterp_RungeAntiCheat(t *testing.T) {
	// 龙格现象题：递推终值 P(0.8) ≈ −0.379310 而 f(0.8) = 1/17 ≈ 0.058824。
	// 不做递推直接抄原函数值 -> 相对误差 ~1.15 -> 0 分（同 L16 天然防抄 f 值）
	p := nevilleTrueP(t, "runge_5", math.NaN())
	if p >= 0 {
		t.Fatalf("runge_5 P(0.8) 应为负（龙格振荡），got %g", p)
	}
	if score, _, _ := submitAitken(t, fmt.Sprintf(`[{"value":%.12g}]`, p), `{"problem_key":"runge_5"}`); score != 100 {
		t.Fatalf("递推真值: want 100, got %d", score)
	}
	if score, passed, _ := submitAitken(t, fmt.Sprintf(`[{"value":%.12g}]`, 1.0/17), `{"problem_key":"runge_5"}`); score != 0 || passed {
		t.Fatalf("抄 f 值: want 0/not passed, got %d/%v", score, passed)
	}
}

func TestScoreAitkenInterp_UserConfigProblemKey(t *testing.T) {
	// 换题 sqrt_table：P(115) ≈ 10.722756（与 L16 真值一致）
	p := nevilleTrueP(t, "sqrt_table", math.NaN())
	if math.Abs(p-10.722756) > 1e-6 {
		t.Fatalf("sqrt_table P(115): want ~10.722756, got %g", p)
	}
	score, _, detail := submitAitken(t, fmt.Sprintf(`[{"value":%.12g}]`, p), `{"problem_key":"sqrt_table"}`)
	if score != 100 {
		t.Fatalf("want 100, got %d (detail=%s)", score, detail)
	}
	if !strings.Contains(detail, "problem=sqrt_table") {
		t.Errorf("detail 应记录换题后的 problem: %s", detail)
	}
}

func TestScoreAitkenInterp_UserConfigCustomNodes(t *testing.T) {
	// 自定义 3 节点 (0,0),(1,1),(2,4) 递推 x²，P(0.5) = 0.25
	score, _, detail := submitAitken(t, `[{"value":0.25}]`, `{"nodes_x":[0,1,2],"nodes_y":[0,1,4],"x_eval":0.5}`)
	if score != 100 {
		t.Fatalf("want 100, got %d (detail=%s)", score, detail)
	}
	if !strings.Contains(detail, "problem=custom") {
		t.Errorf("detail 应记录 problem=custom: %s", detail)
	}
}

func TestScoreAitkenInterp_UserConfigXEval(t *testing.T) {
	// 覆盖求值点：节点处 P(π/4) = √2/2（插值多项式过节点）
	if score, _, _ := submitAitken(t, fmt.Sprintf(`[{"value":%.12g}]`, math.Sqrt2/2), `{"x_eval":0.7853981633974483}`); score != 100 {
		t.Fatalf("节点处求值: want 100, got %d", score)
	}
	// 非节点处 x_eval=π/3，真值运行时算
	p := nevilleTrueP(t, "sin_quad", math.Pi/3)
	if score, _, _ := submitAitken(t, fmt.Sprintf(`[{"value":%.12g}]`, p), `{"x_eval":1.0471975511965976}`); score != 100 {
		t.Fatalf("x_eval 覆盖: want 100, got %d", score)
	}
}

func TestScoreAitkenInterp_Col1Audit(t *testing.T) {
	// col1 可选：值随意不参与打分，只须长度=n−1 且有限，记入审计 detail
	score, _, detail := submitAitken(t, `[{"value":0.5174282,"col1":[9,-4]}]`, "")
	if score != 100 {
		t.Fatalf("col1 不影响打分: want 100, got %d", score)
	}
	if !strings.Contains(detail, "col1=[9,-4]") {
		t.Errorf("detail 应原样记录 col1: %s", detail)
	}
}

func TestScoreAitkenInterp_BadInput(t *testing.T) {
	cases := []struct {
		name, readings, userConfig string
	}{
		{"缺 value", `[{"col1":[1,2]}]`, ""},
		{"value 为 NaN", `[{"value":NaN}]`, ""},
		{"非法 JSON", `[{"value":abc}]`, ""},
		{"col1 长度错", `[{"value":0.5,"col1":[1,2,3]}]`, ""},
		{"col1 含 NaN", `[{"value":0.5,"col1":[1,NaN]}]`, ""},
		{"未知插值题", `[{"value":0.5}]`, `{"problem_key":"nope"}`},
		{"自定义缺 x_eval", `[{"value":0.5}]`, `{"nodes_x":[0,1,2],"nodes_y":[0,1,4]}`},
		{"自定义 x 重复", `[{"value":0.5}]`, `{"nodes_x":[1,1,2],"nodes_y":[0,1,4],"x_eval":1.5}`},
		{"自定义 x/y 不匹配", `[{"value":0.5}]`, `{"nodes_x":[0,1,2],"nodes_y":[0,1],"x_eval":1.5}`},
		{"自定义节点数 9", `[{"value":0.5}]`, `{"nodes_x":[1,2,3,4,5,6,7,8,9],"nodes_y":[1,2,3,4,5,6,7,8,9],"x_eval":5}`},
	}
	for _, c := range cases {
		uc := []byte(nil)
		if c.userConfig != "" {
			uc = []byte(c.userConfig)
		}
		if _, _, _, err := scoreByExperiment("aitken_interp", []byte(c.readings), []byte(aitkenTarget), uc); err == nil {
			t.Errorf("%s: want error, got nil", c.name)
		}
	}
}

func TestScoreAitkenInterp_TargetXEvalOnlyForTargetProblem(t *testing.T) {
	// target 带 sin_quad 的求值点（真库 target 即如此）；user_config 换 runge_5
	// 不带 x_eval 时，应用 runge_5 库内默认 0.8，而非沿用 target 的
	//（target 的题专属参数只属于 target 自己的题，v0.12.1 规则同款）
	tgt := `{"method":"aitken_rel_err","problem_key":"sin_quad","x_eval":0.5235987755982988,"pass_score":60}`
	p := nevilleTrueP(t, "runge_5", math.NaN()) // 库内默认 0.8
	score, _, _, err := scoreByExperiment("aitken_interp",
		[]byte(fmt.Sprintf(`[{"value":%.12g}]`, p)), []byte(tgt), []byte(`{"problem_key":"runge_5"}`))
	if err != nil {
		t.Fatal(err)
	}
	if score != 100 {
		t.Fatalf("换题不带 x_eval 应用库内默认求值点: want 100, got %d", score)
	}
	// 换题且显式给 x_eval 时按给定值（0.5 是 runge_5 节点，P=f(0.5)=4/29）
	score, _, _, err = scoreByExperiment("aitken_interp",
		[]byte(fmt.Sprintf(`[{"value":%.12g}]`, 4.0/29)), []byte(tgt), []byte(`{"problem_key":"runge_5","x_eval":0.5}`))
	if err != nil {
		t.Fatal(err)
	}
	if score != 100 {
		t.Fatalf("换题+显式 x_eval: want 100, got %d", score)
	}
}

// ---------- L19 龙贝格积分法（Romberg） ----------

const rombergTarget = `{"method":"romberg_rel_err","problem_key":"sin_pi","pass_score":60}`

func submitRomberg(t *testing.T, readings, userConfig string) (int, bool, string) {
	t.Helper()
	uc := []byte(nil)
	if userConfig != "" {
		uc = []byte(userConfig)
	}
	score, passed, detail, err := scoreByExperiment("romberg_integration", []byte(readings), []byte(rombergTarget), uc)
	if err != nil {
		t.Fatalf("readings=%s user_config=%s: %v", readings, userConfig, err)
	}
	return score, passed, detail
}

// rombergTrueR 运行时算库内题真值 T_k^(k)（测试与评分共用同一 rombergEval，
// 各题另以手工闭式解交叉验证，防"自己证自己"）。
func rombergTrueR(t *testing.T, key string, k int) float64 {
	t.Helper()
	def := integProblems[key]
	return rombergEval(def.F, def.A, def.B, k)
}

func TestScoreRombergIntegration_TableClosedForms(t *testing.T) {
	// sin_pi k=2 全表闭式（交叉验证 rombergTColumn/rombergEval 的每一步）：
	// T_0 = 0（端点值全零），T_1 = π/2，T_2 = π(1+√2)/4；
	// S_1 = 2π/3 正是 L17 的 S_2，S_2 = (4T_2−T_1)/3，R = (16S_2−S_1)/15
	tCol := rombergTColumn(math.Sin, 0, math.Pi, 2)
	wantT := []float64{0, math.Pi / 2, math.Pi * (1 + math.Sqrt2) / 4}
	for i, w := range wantT {
		if math.Abs(tCol[i]-w) > 1e-12 {
			t.Fatalf("T_%d: want %.12g, got %.12g", i, w, tCol[i])
		}
	}
	s1 := (4*tCol[1] - tCol[0]) / 3
	s2 := (4*tCol[2] - tCol[1]) / 3
	if math.Abs(s1-2*math.Pi/3) > 1e-12 {
		t.Fatalf("S_1 应为 2π/3（= L17 的 S_2）: got %.12g", s1)
	}
	wantR := (16*s2 - s1) / 15
	if got := rombergEval(math.Sin, 0, math.Pi, 2); math.Abs(got-wantR) > 1e-12 {
		t.Fatalf("R: want %.12g, got %.12g", wantR, got)
	}
}

func TestScoreRombergIntegration_FirstColumnEqualsSimpson(t *testing.T) {
	// 龙贝格外推第 1 列 = 复合辛普森 S_{2^m}（L17 同一公式，结构性交叉验证）
	for key, def := range integProblems {
		tCol := rombergTColumn(def.F, def.A, def.B, 3)
		s := append([]float64(nil), tCol...)
		for i := 3; i >= 1; i-- { // 手工做一轮外推（4¹−1=3）
			s[i] = (4*s[i] - s[i-1]) / 3
		}
		for m := 1; m <= 3; m++ {
			want := compositeSimpson(def.F, def.A, def.B, 1<<uint(m))
			if math.Abs(s[m]-want) > 1e-12 {
				t.Fatalf("%s: 第 1 列 S_%d 应等于复合辛普森: want %.12g, got %.12g",
					key, 1<<uint(m), want, s[m])
			}
		}
	}
}

func TestScoreRombergIntegration_ExactSolution(t *testing.T) {
	r := rombergTrueR(t, "sin_pi", 2)
	if math.Abs(r-1.9985707317) > 1e-9 { // 手算锚点：R = (16S_2−S_1)/15
		t.Fatalf("sin_pi k=2 终值: want ~1.9985707317, got %.12g", r)
	}
	readings := fmt.Sprintf(`[{"value":%.12g,"t_col":[0,%.12g,%.12g]}]`,
		r, math.Pi/2, math.Pi*(1+math.Sqrt2)/4)
	score, passed, detail := submitRomberg(t, readings, "")
	if score != 100 || !passed {
		t.Fatalf("want 100/passed, got %d/%v (detail=%s)", score, passed, detail)
	}
	if !strings.Contains(detail, "problem=sin_pi") || !strings.Contains(detail, "k=2") {
		t.Errorf("detail 应记录 problem/k: %s", detail)
	}
	if !strings.Contains(detail, "ref_I=2") {
		t.Errorf("detail 应记录参考积分 ref_I: %s", detail)
	}
	if !strings.Contains(detail, "t_col=[0,") {
		t.Errorf("detail 应记录 t_col: %s", detail)
	}
}

func TestScoreRombergIntegration_RelErrCurve(t *testing.T) {
	r := rombergTrueR(t, "sin_pi", 2)
	for _, c := range []struct {
		relErr float64
		want   int
	}{
		{1e-7, 100},
		{1e-4, 90},
		{1e-3, 80},
		{1e-2, 65},
		{0.1, 30},
		{0.5, 0},
	} {
		readings := fmt.Sprintf(`[{"value":%.12g}]`, r*(1+c.relErr))
		if score, _, _ := submitRomberg(t, readings, ""); score != c.want {
			t.Errorf("rel_err=%g: want %d, got %d", c.relErr, c.want, score)
		}
	}
}

func TestScoreRombergIntegration_Traps(t *testing.T) {
	// 抄精确积分 I=2：与真值只差 0.07% 截断误差 -> ~81 分——能过线但拿不到满分
	// （龙贝格收敛快，与 L17 n=2 辛普森"抄 I 直接挂"不同；过程是否真做由 t_col 审计暴露）
	if score, passed, _ := submitRomberg(t, `[{"value":2}]`, ""); !passed || score < 60 || score >= 90 {
		t.Fatalf("抄精确积分: want 60~89/passed, got %d/%v", score, passed)
	}
	// 抄 L17 的答案 2π/3（= 本表 S_1，漏做第二轮外推）：~41 分不过线
	if score, passed, _ := submitRomberg(t, fmt.Sprintf(`[{"value":%.12g}]`, 2*math.Pi/3), ""); passed || score >= 60 {
		t.Fatalf("抄 S_1/L17 答案: want <60/not passed, got %d/%v", score, passed)
	}
	// 抄最后一行梯形 T_2（漏做全部外推）：~40 分不过线
	if score, passed, _ := submitRomberg(t, fmt.Sprintf(`[{"value":%.12g}]`, math.Pi*(1+math.Sqrt2)/4), ""); passed || score >= 60 {
		t.Fatalf("抄 T_2: want <60/not passed, got %d/%v", score, passed)
	}
}

func TestScoreRombergIntegration_UserConfigProblemKey(t *testing.T) {
	// 换题 erf_gauss k=2：全表闭式（指数值手算可得）交叉验证后提交
	t0 := 0.5 * (1 + math.Exp(-1))
	t1 := 0.25 * (1 + 2*math.Exp(-0.25) + math.Exp(-1))
	t2 := 0.125 * (1 + 2*(math.Exp(-1.0/16)+math.Exp(-0.25)+math.Exp(-9.0/16)) + math.Exp(-1))
	erfR := (16*(4*t2-t1)/3 - (4*t1-t0)/3) / 15
	if got := rombergTrueR(t, "erf_gauss", 2); math.Abs(got-erfR) > 1e-12 {
		t.Fatalf("erf_gauss k=2 终值闭式不符: want %.12g, got %.12g", erfR, got)
	}
	if math.Abs(erfR-0.7468337100) > 1e-9 {
		t.Fatalf("erf_gauss k=2 终值锚点: got %.12g", erfR)
	}
	score, _, detail := submitRomberg(t, fmt.Sprintf(`[{"value":%.12g}]`, erfR), `{"problem_key":"erf_gauss"}`)
	if score != 100 {
		t.Fatalf("erf_gauss 正确终值: want 100, got %d (detail=%s)", score, detail)
	}
	if !strings.Contains(detail, "problem=erf_gauss") {
		t.Errorf("detail 应记录换题后的 problem: %s", detail)
	}

	// 换题 sqrt_x k=2：端点导数奇异，外推失效观察题（R ≈ 0.6577566 vs I = 2/3）
	st0 := 0.5
	st1 := 0.25 * (2*math.Sqrt(0.5) + 1)
	st2 := 0.125 * (2*(0.5 + math.Sqrt(0.5) + math.Sqrt(0.75)) + 1)
	sqrtR := (16*(4*st2-st1)/3 - (4*st1-st0)/3) / 15
	if math.Abs(sqrtR-0.6577566032) > 1e-9 {
		t.Fatalf("sqrt_x k=2 终值锚点: got %.12g", sqrtR)
	}
	if score, _, _ := submitRomberg(t, fmt.Sprintf(`[{"value":%.12g}]`, sqrtR), `{"problem_key":"sqrt_x"}`); score != 100 {
		t.Fatalf("sqrt_x 正确终值: want 100")
	}
	if score, _, _ := submitRomberg(t, `[{"value":0.6666666666666666}]`, `{"problem_key":"sqrt_x"}`); score >= 100 {
		t.Fatalf("sqrt_x 抄精确积分应 <100, got %d", score)
	}
}

func TestScoreRombergIntegration_UserConfigK(t *testing.T) {
	// 加深外推 k=3：S_3 恰为 L17 的 S_8（=2.0002691699），终值 R 更近 2
	var sum float64
	for i := 1; i <= 7; i++ {
		sum += math.Sin(float64(i) * math.Pi / 8)
	}
	t2 := math.Pi * (1 + math.Sqrt2) / 4
	t3 := math.Pi / 8 * sum // (h/2)·[0+2Σ+0] = π/8·Σ（端点值为零）
	s2 := (4*t2 - math.Pi/2) / 3
	s3 := (4*t3 - t2) / 3
	if math.Abs(s3-2.000269169948296) > 1e-12 {
		t.Fatalf("S_3 应等于 L17 的 S_8: got %.12g", s3)
	}
	// k=3 终值在第 3 列：C_3=(16S_3−S_2)/15，C_2=(16S_2−S_1)/15，R=(64C_3−C_2)/63
	c2 := (16*s2 - 2*math.Pi/3) / 15
	c3 := (16*s3 - s2) / 15
	wantR := (64*c3 - c2) / 63
	if got := rombergEval(math.Sin, 0, math.Pi, 3); math.Abs(got-wantR) > 1e-12 {
		t.Fatalf("k=3 终值闭式不符: want %.12g, got %.12g", wantR, got)
	}
	score, _, detail := submitRomberg(t, fmt.Sprintf(`[{"value":%.12g}]`, wantR), `{"k":3}`)
	if score != 100 {
		t.Fatalf("sin_pi k=3 正确终值: want 100, got %d (detail=%s)", score, detail)
	}
	if !strings.Contains(detail, "k=3") {
		t.Errorf("detail 应记录 k=3: %s", detail)
	}
}

func TestScoreRombergIntegration_UserConfigInterval(t *testing.T) {
	// 换区间 [0, π/2]：I = 1 已知，k=2 终值闭式（手算可得）
	t0 := math.Pi / 4
	t1 := math.Pi / 8 * (1 + math.Sqrt2)
	t2 := math.Pi / 16 * (1 + 2*(math.Sin(math.Pi/8)+math.Sin(math.Pi/4)+math.Sin(3*math.Pi/8)))
	wantR := (16*(4*t2-t1)/3 - (4*t1-t0)/3) / 15
	if got := rombergEval(math.Sin, 0, math.Pi/2, 2); math.Abs(got-wantR) > 1e-12 {
		t.Fatalf("[0,π/2] 终值闭式不符: want %.12g, got %.12g", wantR, got)
	}
	score, _, detail := submitRomberg(t,
		fmt.Sprintf(`[{"value":%.12g}]`, wantR), `{"a":0,"b":1.5707963267948966}`)
	if score != 100 {
		t.Fatalf("换区间正确终值: want 100, got %d (detail=%s)", score, detail)
	}
	if !strings.Contains(detail, "b=1.5708") {
		t.Errorf("detail 应记录换后的区间: %s", detail)
	}
}

func TestScoreRombergIntegration_TColAudit(t *testing.T) {
	// t_col 可选：值随意不参与打分，只须长度=k+1 且有限，记入审计 detail
	r := rombergTrueR(t, "sin_pi", 2)
	score, _, detail := submitRomberg(t, fmt.Sprintf(`[{"value":%.12g,"t_col":[9,-4,2]}]`, r), "")
	if score != 100 {
		t.Fatalf("t_col 不影响打分: want 100, got %d", score)
	}
	if !strings.Contains(detail, "t_col=[9,-4,2]") {
		t.Errorf("detail 应原样记录 t_col: %s", detail)
	}
}

func TestScoreRombergIntegration_BadInput(t *testing.T) {
	cases := []struct {
		name, readings, userConfig string
	}{
		{"缺 value", `[{"t_col":[0,1,2]}]`, ""},
		{"value 为 NaN", `[{"value":NaN}]`, ""},
		{"非法 JSON", `[{"value":abc}]`, ""},
		{"k 为 0", `[{"value":2}]`, `{"k":0}`},
		{"k 超上限 11", `[{"value":2}]`, `{"k":11}`},
		{"k 为非整数", `[{"value":2}]`, `{"k":2.5}`},
		{"区间只给 a", `[{"value":2}]`, `{"a":0}`},
		{"t_col 长度错", `[{"value":2,"t_col":[0,1]}]`, ""},
		{"t_col 含 NaN", `[{"value":2,"t_col":[0,NaN,1]}]`, ""},
		{"未知积分题", `[{"value":2}]`, `{"problem_key":"nope"}`},
	}
	for _, c := range cases {
		uc := []byte(nil)
		if c.userConfig != "" {
			uc = []byte(c.userConfig)
		}
		if _, _, _, err := scoreByExperiment("romberg_integration", []byte(c.readings), []byte(rombergTarget), uc); err == nil {
			t.Errorf("%s: want error, got nil", c.name)
		}
	}
}

func TestScoreRombergIntegration_TargetParamsOnlyForTargetProblem(t *testing.T) {
	// target 带 sin_pi 的 k=3（真库 target 即如此）；user_config 换 erf_gauss 不带 k
	// 时应用库内默认 k=2，而非沿用 target 的 3（target 的题专属参数只属于
	// target 自己的题，v0.12.1 规则同款）
	tgt := `{"method":"romberg_rel_err","problem_key":"sin_pi","k":3,"pass_score":60}`
	t0 := 0.5 * (1 + math.Exp(-1))
	t1 := 0.25 * (1 + 2*math.Exp(-0.25) + math.Exp(-1))
	t2 := 0.125 * (1 + 2*(math.Exp(-1.0/16)+math.Exp(-0.25)+math.Exp(-9.0/16)) + math.Exp(-1))
	erfR2 := (16*(4*t2-t1)/3 - (4*t1-t0)/3) / 15 // erf_gauss k=2 终值
	fErf := func(x float64) float64 { return math.Exp(-x * x) }
	erfR3 := rombergEval(fErf, 0, 1, 3)
	if math.Abs(erfR2-erfR3) < 1e-6 {
		t.Fatalf("k=2 与 k=3 终值应不同（测试前提失效）")
	}
	score, _, _, err := scoreByExperiment("romberg_integration",
		[]byte(fmt.Sprintf(`[{"value":%.12g}]`, erfR2)), []byte(tgt), []byte(`{"problem_key":"erf_gauss"}`))
	if err != nil {
		t.Fatal(err)
	}
	if score != 100 {
		t.Fatalf("换题不带 k 应用库内默认 k=2: want 100, got %d", score)
	}
	// 换题后若错误沿用 target 的 k=3（bug 场景），按 k=3 提交会对不上 k=2 的真值
	score, _, _, err = scoreByExperiment("romberg_integration",
		[]byte(fmt.Sprintf(`[{"value":%.12g}]`, erfR3)), []byte(tgt), []byte(`{"problem_key":"erf_gauss"}`))
	if err != nil {
		t.Fatal(err)
	}
	if score >= 100 {
		t.Fatalf("按 k=3 提交应暴露不符（truth 是 k=2）: got %d", score)
	}
	// 未换题：target 的 k=3 生效（sin_pi k=3 终值闭式见 UserConfigK 测试）
	var sum float64
	for i := 1; i <= 7; i++ {
		sum += math.Sin(float64(i) * math.Pi / 8)
	}
	sinR3 := rombergEval(math.Sin, 0, math.Pi, 3)
	// k=3 终值闭式：S_1=2π/3, S_2, S_3 → C_2, C_3 → R=(64C_3−C_2)/63
	s1 := 2 * math.Pi / 3
	s2 := (4*(math.Pi*(1+math.Sqrt2)/4) - math.Pi/2) / 3
	s3 := (4*(math.Pi/8*sum) - math.Pi*(1+math.Sqrt2)/4) / 3
	wantR := (64*(16*s3-s2)/15 - (16*s2-s1)/15) / 63
	if math.Abs(sinR3-wantR) > 1e-12 {
		t.Fatalf("sin_pi k=3 终值闭式不符: want %.12g, got %.12g", wantR, sinR3)
	}
	score, _, _, err = scoreByExperiment("romberg_integration",
		[]byte(fmt.Sprintf(`[{"value":%.12g}]`, sinR3)), []byte(tgt), nil)
	if err != nil {
		t.Fatal(err)
	}
	if score != 100 {
		t.Fatalf("未换题时 target 的 k=3 应生效: want 100, got %d", score)
	}
}

// ========== L20/L21 欧拉法（常微分方程初值问题数值解） ==========

const eulerExplicitTarget = `{"method":"euler_rel_err","problem_key":"exp_growth","n":4,"pass_score":60}`
const eulerImprovedTarget = `{"method":"euler_improved_rel_err","problem_key":"exp_growth","n":2,"pass_score":60}`

func submitEuler(t *testing.T, readings, userConfig string) (int, bool, string) {
	t.Helper()
	uc := []byte(nil)
	if userConfig != "" {
		uc = []byte(userConfig)
	}
	score, passed, detail, err := scoreByExperiment("euler_explicit", []byte(readings), []byte(eulerExplicitTarget), uc)
	if err != nil {
		t.Fatalf("readings=%s user_config=%s: %v", readings, userConfig, err)
	}
	return score, passed, detail
}

func submitEulerImproved(t *testing.T, readings, userConfig string) (int, bool, string) {
	t.Helper()
	uc := []byte(nil)
	if userConfig != "" {
		uc = []byte(userConfig)
	}
	score, passed, detail, err := scoreByExperiment("euler_improved", []byte(readings), []byte(eulerImprovedTarget), uc)
	if err != nil {
		t.Fatalf("readings=%s user_config=%s: %v", readings, userConfig, err)
	}
	return score, passed, detail
}

// eulerTrueY 运行时算库内题真值 y_N（测试与评分共用同一递推，各题另以
// 手工闭式解交叉验证，防"自己证自己"）。
func eulerTrueY(key string, n int, improved bool) float64 {
	def := odeProblems[key]
	var ys []float64
	if improved {
		ys = eulerImproved(def.F, def.A, def.B, def.Y0, n)
	} else {
		ys = eulerExplicit(def.F, def.A, def.B, def.Y0, n)
	}
	return ys[n]
}

func TestScoreEulerExplicit_ClosedForms(t *testing.T) {
	// exp_growth：显式欧拉 (1+h)^n，n=4 h=0.25 恰 1.25^4 = 2.44140625
	if got := eulerTrueY("exp_growth", 4, false); got != 2.44140625 {
		t.Fatalf("exp_growth n=4: want 2.44140625, got %.12g", got)
	}
	// decay_sq：手算全为 2 的幂分数（f(x,0)=0 使 y1=1）：
	// y = [1, 1, 0.875, 0.65625, 0.41015625]
	if got := eulerTrueY("decay_sq", 4, false); got != 0.41015625 {
		t.Fatalf("decay_sq n=4: want 0.41015625, got %.12g", got)
	}
	// stiff_decay：hλ=−2.5，每步乘 (1−2.5)=−1.5 振荡发散，(−1.5)^4 = 5.0625
	if got := eulerTrueY("stiff_decay", 4, false); got != 5.0625 {
		t.Fatalf("stiff_decay n=4: want 5.0625, got %.12g", got)
	}
	// 结构性恒等式：y'=y 的显式欧拉对任意 a/b/n 都是 (1+h)^n
	for _, c := range []struct{ a, b float64; n int }{
		{0, 1, 1}, {0, 2, 7}, {-1, 1, 3}, {0.5, 1.25, 6},
	} {
		h := (c.b - c.a) / float64(c.n)
		want := math.Pow(1+h, float64(c.n))
		if got := eulerExplicit(odeProblems["exp_growth"].F, c.a, c.b, 1, c.n)[c.n]; math.Abs(got-want) > 1e-12 {
			t.Fatalf("exp_growth 显式欧拉 (1+h)^n 恒等式破坏: a=%g b=%g n=%d: want %.12g got %.12g",
				c.a, c.b, c.n, want, got)
		}
	}
}

func TestScoreEulerImproved_ClosedForms(t *testing.T) {
	// exp_growth：改进欧拉每步乘 1+h+h²/2：
	// n=2 h=0.5 恰 1.625² = 2.640625；n=4 h=0.25 恰 (41/32)^4
	if got := eulerTrueY("exp_growth", 2, true); got != 2.640625 {
		t.Fatalf("exp_growth n=2: want 2.640625, got %.12g", got)
	}
	if got := eulerTrueY("exp_growth", 4, true); math.Abs(got-2.6948556900024414) > 1e-12 {
		t.Fatalf("exp_growth n=4: want (41/32)^4=2.6948556900, got %.12g", got)
	}
	// stiff_decay：每步乘 1+hλ+(hλ)²/2 = 1−2.5+3.125 = 1.625，(13/8)^4 = 28561/4096
	if got := eulerTrueY("stiff_decay", 4, true); got != 28561.0/4096.0 {
		t.Fatalf("stiff_decay n=4: want 28561/4096, got %.12g", got)
	}
	// decay_sq：手算锚点（预测-校正逐步代入，两位小数手工可复算）
	if got := eulerTrueY("decay_sq", 4, true); math.Abs(got-0.3741575) > 1e-6 {
		t.Fatalf("decay_sq n=4: want ~0.3741575, got %.12g", got)
	}
	// 结构性恒等式：y'=y 的改进欧拉对任意 a/b/n 都是 (1+h+h²/2)^n
	for _, c := range []struct{ a, b float64; n int }{
		{0, 1, 1}, {0, 2, 5}, {-1, 1, 4}, {0.5, 1.25, 8},
	} {
		h := (c.b - c.a) / float64(c.n)
		want := math.Pow(1+h+h*h/2, float64(c.n))
		if got := eulerImproved(odeProblems["exp_growth"].F, c.a, c.b, 1, c.n)[c.n]; math.Abs(got-want) > 1e-12 {
			t.Fatalf("exp_growth 改进欧拉 (1+h+h²/2)^n 恒等式破坏: a=%g b=%g n=%d: want %.12g got %.12g",
				c.a, c.b, c.n, want, got)
		}
	}
}

func TestScoreEulerExplicit_ExactSolution(t *testing.T) {
	// exp_growth n=4 全列手算闭式：y_col=[1,1.25,1.5625,1.953125,2.44140625]
	readings := `[{"value":2.44140625,"y_col":[1,1.25,1.5625,1.953125,2.44140625]}]`
	score, passed, detail := submitEuler(t,readings, "")
	if score != 100 || !passed {
		t.Fatalf("want 100/passed, got %d/%v (detail=%s)", score, passed, detail)
	}
	if !strings.Contains(detail, "problem=exp_growth") || !strings.Contains(detail, "n=4") {
		t.Errorf("detail 应记录 problem/n: %s", detail)
	}
	if !strings.Contains(detail, "ref_Y=2.71828") {
		t.Errorf("detail 应记录精确解 ref_Y: %s", detail)
	}
	if !strings.Contains(detail, "y_col=[1,") {
		t.Errorf("detail 应记录 y_col: %s", detail)
	}
}

func TestScoreEulerExplicit_RelErrCurve(t *testing.T) {
	y := eulerTrueY("exp_growth", 4, false)
	for _, c := range []struct {
		relErr float64
		want   int
	}{
		{1e-7, 100},
		{1e-4, 90},
		{1e-3, 80},
		{1e-2, 65},
		{0.1, 30},
		{0.5, 0},
	} {
		readings := fmt.Sprintf(`[{"value":%.12g}]`, y*(1+c.relErr))
		if score, _, _ := submitEuler(t,readings, ""); score != c.want {
			t.Errorf("rel_err=%g: want %d, got %d", c.relErr, c.want, score)
		}
	}
}

func TestScoreEulerExplicit_Traps(t *testing.T) {
	// 抄精确解 e（漏做全部递推）：O(h) 截断误差 ~11% -> ~28 分不过线
	if score, passed, _ := submitEuler(t,`[{"value":2.7182818285}]`, ""); passed || score >= 60 {
		t.Fatalf("抄精确解: want <60/not passed, got %d/%v", score, passed)
	}
}

func TestScoreEulerExplicit_StiffDivergence(t *testing.T) {
	// 刚性观察题：hλ=−2.5 在绝对稳定区间外，解振荡发散——"真值=方法输出"
	// 仍是确定的 5.0625，老实递推照样 100
	readings := `[{"value":5.0625,"y_col":[1,-1.5,2.25,-3.375,5.0625]}]`
	if score, passed, detail := submitEuler(t,readings, `{"problem_key":"stiff_decay"}`); score != 100 || !passed {
		t.Fatalf("stiff 正确递推（发散值仍确定）: want 100/passed, got %d/%v (detail=%s)", score, passed, detail)
	}
	// 抄精确解 e^{-10}：与发散值差 ~100% -> 0 分
	if score, passed, _ := submitEuler(t,fmt.Sprintf(`[{"value":%.12g}]`, math.Exp(-10)), `{"problem_key":"stiff_decay"}`); passed || score > 0 {
		t.Fatalf("stiff 抄精确解: want 0/not passed, got %d/%v", score, passed)
	}
}

func TestScoreEulerImproved_ExactSolution(t *testing.T) {
	// exp_growth n=2 全列闭式：y_col=[1,1.75,2.640625]（1.625 因子）
	readings := `[{"value":2.640625,"y_col":[1,1.75,2.640625]}]`
	score, passed, detail := submitEulerImproved(t, readings, "")
	if score != 100 || !passed {
		t.Fatalf("want 100/passed, got %d/%v (detail=%s)", score, passed, detail)
	}
	if !strings.Contains(detail, "改进欧拉") {
		t.Errorf("detail 应标注方法（改进欧拉）: %s", detail)
	}
}

func TestScoreEulerImproved_RelErrCurve(t *testing.T) {
	y := eulerTrueY("exp_growth", 2, true)
	for _, c := range []struct {
		relErr float64
		want   int
	}{
		{1e-7, 100},
		{1e-4, 90},
		{1e-3, 80},
		{1e-2, 65},
		{0.1, 30},
		{0.5, 0},
	} {
		readings := fmt.Sprintf(`[{"value":%.12g}]`, y*(1+c.relErr))
		if score, _, _ := submitEulerImproved(t, readings, ""); score != c.want {
			t.Errorf("rel_err=%g: want %d, got %d", c.relErr, c.want, score)
		}
	}
}

func TestScoreEulerImproved_Traps(t *testing.T) {
	// 抄精确解 e：与 n=2 改进欧拉值差 ~2.9% -> ~49 分不过线
	if score, passed, _ := submitEulerImproved(t, `[{"value":2.7182818285}]`, ""); passed || score >= 60 {
		t.Fatalf("抄精确解: want <60/not passed, got %d/%v", score, passed)
	}
	// 抄显式欧拉 L20 的答案 2.44140625（只做预测漏做校正）：差 ~7.5% -> ~34 分不过线
	if score, passed, _ := submitEulerImproved(t, `[{"value":2.44140625}]`, ""); passed || score >= 60 {
		t.Fatalf("抄显式欧拉答案: want <60/not passed, got %d/%v", score, passed)
	}
}

func TestScoreEulerImproved_StiffVsExplicit(t *testing.T) {
	// 改进欧拉在 stiff_decay 上也发散（因子 1.625>1），但值与显式欧拉不同：
	// 抄显式答案 5.0625 差 ~27% -> ~2 分不过线
	if score, passed, _ := submitEulerImproved(t, fmt.Sprintf(`[{"value":%.12g}]`, 28561.0/4096.0), `{"problem_key":"stiff_decay"}`); score != 100 || !passed {
		t.Fatalf("stiff 改进欧拉正确值: want 100, got %d/%v", score, passed)
	}
	if score, passed, _ := submitEulerImproved(t, `[{"value":5.0625}]`, `{"problem_key":"stiff_decay"}`); passed || score >= 60 {
		t.Fatalf("stiff 抄显式欧拉答案: want <60/not passed, got %d/%v", score, passed)
	}
}

func TestScoreEulerExplicit_UserConfigProblemKey(t *testing.T) {
	// 换题 decay_sq（库默认 n=4）：闭式 0.41015625 -> 100
	score, _, detail := submitEuler(t,`[{"value":0.41015625}]`, `{"problem_key":"decay_sq"}`)
	if score != 100 {
		t.Fatalf("decay_sq 正确值: want 100, got %d (detail=%s)", score, detail)
	}
	if !strings.Contains(detail, "problem=decay_sq") {
		t.Errorf("detail 应记录换题后的 problem: %s", detail)
	}
}

func TestScoreEulerImproved_UserConfigProblemKey(t *testing.T) {
	// 换题 decay_sq（换题后 n 回落库默认 4，非 target 的 2）
	if score, _, detail := submitEulerImproved(t, fmt.Sprintf(`[{"value":%.12g}]`, eulerTrueY("decay_sq", 4, true)), `{"problem_key":"decay_sq"}`); score != 100 {
		t.Fatalf("decay_sq 换题 n=4: want 100, got %d (detail=%s)", score, detail)
	}
	// 换题 decay_sq 抄精确解 e^{-1}：差 ~1.7% -> ~57 分不过线
	if score, passed, _ := submitEulerImproved(t, fmt.Sprintf(`[{"value":%.12g}]`, math.Exp(-1)), `{"problem_key":"decay_sq"}`); passed || score >= 60 {
		t.Fatalf("decay_sq 抄精确解: want <60/not passed, got %d/%v", score, passed)
	}
}

func TestScoreEulerExplicit_UserConfigN(t *testing.T) {
	// n=8：h=0.125，(1.125)^8 = 2.5657845139...（12 位手算锚点）
	if got := eulerTrueY("exp_growth", 8, false); math.Abs(got-2.56578451395) > 1e-10 {
		t.Fatalf("exp_growth n=8 锚点: got %.12g", got)
	}
	if score, _, _ := submitEuler(t,fmt.Sprintf(`[{"value":%.12g}]`, eulerTrueY("exp_growth", 8, false)), `{"n":8}`); score != 100 {
		t.Fatalf("n=8 正确值: want 100, got %d", score)
	}
}

func TestScoreEulerImproved_UserConfigN(t *testing.T) {
	// n=4：每步因子 41/32，(41/32)^4 -> 100
	if score, _, _ := submitEulerImproved(t, fmt.Sprintf(`[{"value":%.12g}]`, eulerTrueY("exp_growth", 4, true)), `{"n":4}`); score != 100 {
		t.Fatalf("n=4 正确值: want 100, got %d", score)
	}
}

func TestScoreEulerExplicit_UserConfigInterval(t *testing.T) {
	// 换区间 [0,2] 并换 n=8：h=0.25，(1.25)^8 = 5.9604644775...
	if got := eulerExplicit(odeProblems["exp_growth"].F, 0, 2, 1, 8)[8]; math.Abs(got-5.96046447754) > 1e-10 {
		t.Fatalf("[0,2] n=8 锚点: got %.12g", got)
	}
	score, _, detail := submitEuler(t,
		fmt.Sprintf(`[{"value":%.12g}]`, eulerExplicit(odeProblems["exp_growth"].F, 0, 2, 1, 8)[8]),
		`{"a":0,"b":2,"n":8}`)
	if score != 100 {
		t.Fatalf("换区间换 n 正确值: want 100, got %d", score)
	}
	if !strings.Contains(detail, "a=0") || !strings.Contains(detail, "b=2") {
		t.Errorf("detail 应记录换后区间: %s", detail)
	}
}

func TestScoreEulerImproved_UserConfigInterval(t *testing.T) {
	// 换区间 [0,2]（未换 n，target 的 n=2 生效）：h=1，因子 1+1+0.5=2.5，
	// 改进欧拉终值闭式 2.5² = 6.25
	if score, _, _ := submitEulerImproved(t, `[{"value":6.25}]`, `{"a":0,"b":2}`); score != 100 {
		t.Fatalf("换区间正确值: want 100, got %d", score)
	}
}

func TestScoreEuler_YColAudit(t *testing.T) {
	// y_col 长度错（n=4 须 5 个）直接 400 暴露
	if _, _, _, err := scoreByExperiment("euler_explicit",
		[]byte(`[{"value":2.44140625,"y_col":[1,1.25,1.5625]}]`), []byte(eulerExplicitTarget), nil); err == nil {
		t.Fatal("y_col 长度错应报错")
	}
	if _, _, _, err := scoreByExperiment("euler_improved",
		[]byte(`[{"value":2.640625,"y_col":[1,1.75,2.640625,3]}]`), []byte(eulerImprovedTarget), nil); err == nil {
		t.Fatal("改进欧拉 y_col 长度错应报错")
	}
	if _, _, _, err := scoreByExperiment("euler_implicit",
		[]byte(`[{"value":3.16049382716049,"y_col":[1,1.33333333333333,1.77777777777778]}]`), []byte(eulerImplicitTarget), nil); err == nil {
		t.Fatal("隐式欧拉 y_col 长度错应报错")
	}
}

func TestScoreEuler_BadInput(t *testing.T) {
	for _, code := range []string{"euler_explicit", "euler_improved", "euler_implicit"} {
		if _, _, _, err := scoreByExperiment(code, []byte(`[{}]`), []byte(eulerExplicitTarget), nil); err == nil {
			t.Errorf("%s: 缺 value 应报错", code)
		}
		if _, _, _, err := scoreByExperiment(code, []byte(`[{"value":"x"}]`), []byte(eulerExplicitTarget), nil); err == nil {
			t.Errorf("%s: value 非数值应报错", code)
		}
		if _, _, _, err := scoreByExperiment(code, []byte(`[{"value":2}]`), []byte(eulerExplicitTarget), []byte(`{"n":0}`)); err == nil {
			t.Errorf("%s: n=0 应报错", code)
		}
		if _, _, _, err := scoreByExperiment(code, []byte(`[{"value":2}]`), []byte(eulerExplicitTarget), []byte(`{"n":1025}`)); err == nil {
			t.Errorf("%s: n=1025 应报错", code)
		}
		if _, _, _, err := scoreByExperiment(code, []byte(`[{"value":2}]`), []byte(eulerExplicitTarget), []byte(`{"a":0}`)); err == nil {
			t.Errorf("%s: 只给 a 应报错", code)
		}
		if _, _, _, err := scoreByExperiment(code, []byte(`[{"value":2}]`), []byte(eulerExplicitTarget), []byte(`{"a":1,"b":0}`)); err == nil {
			t.Errorf("%s: a>=b 应报错", code)
		}
		if _, _, _, err := scoreByExperiment(code, []byte(`[{"value":2}]`), []byte(eulerExplicitTarget), []byte(`{"problem_key":"nope"}`)); err == nil {
			t.Errorf("%s: 未知题应报错", code)
		}
		if _, _, _, err := scoreByExperiment(code, []byte(`[{"value":NaN}]`), []byte(eulerExplicitTarget), nil); err == nil {
			t.Errorf("%s: NaN value 应报错", code)
		}
	}
}

func TestScoreEuler_TargetParamsOnlyForTargetProblem(t *testing.T) {
	// target 带 exp_growth 的 n=8；user_config 换 decay_sq 不带 n 时应用库内
	// 默认 n=4，而非沿用 target 的 8（target 的题专属参数只属于 target 自己的题）
	tgt := `{"method":"euler_rel_err","problem_key":"exp_growth","n":8,"pass_score":60}`
	score, _, _, err := scoreByExperiment("euler_explicit",
		[]byte(`[{"value":0.41015625}]`), []byte(tgt), []byte(`{"problem_key":"decay_sq"}`))
	if err != nil {
		t.Fatal(err)
	}
	if score != 100 {
		t.Fatalf("换题不带 n 应用库内默认 n=4: want 100, got %d", score)
	}
	// 换题后若错误沿用 target 的 n=8（bug 场景），按 n=8 提交会对不上 n=4 的真值
	d8 := eulerExplicit(odeProblems["decay_sq"].F, 0, 1, 1, 8)[8]
	score, _, _, err = scoreByExperiment("euler_explicit",
		[]byte(fmt.Sprintf(`[{"value":%.12g}]`, d8)), []byte(tgt), []byte(`{"problem_key":"decay_sq"}`))
	if err != nil {
		t.Fatal(err)
	}
	if score >= 100 {
		t.Fatalf("按 n=8 提交应暴露不符（truth 是 n=4）: got %d", score)
	}
	// 未换题：target 的 n=8 生效
	score, _, _, err = scoreByExperiment("euler_explicit",
		[]byte(fmt.Sprintf(`[{"value":%.12g}]`, eulerTrueY("exp_growth", 8, false))), []byte(tgt), nil)
	if err != nil {
		t.Fatal(err)
	}
	if score != 100 {
		t.Fatalf("未换题时 target 的 n=8 应生效: want 100, got %d", score)
	}
}

// ===== L26 隐式（后退）欧拉法（euler_implicit）=====

const eulerImplicitTarget = `{"method":"euler_implicit_rel_err","problem_key":"exp_growth","n":4,"pass_score":60}`

func submitEulerImplicit(t *testing.T, readings, userConfig string) (int, bool, string) {
	t.Helper()
	uc := []byte(nil)
	if userConfig != "" {
		uc = []byte(userConfig)
	}
	score, passed, detail, err := scoreByExperiment("euler_implicit", []byte(readings), []byte(eulerImplicitTarget), uc)
	if err != nil {
		t.Fatalf("readings=%s user_config=%s: %v", readings, userConfig, err)
	}
	return score, passed, detail
}

// implicitTrueY 运行时算隐式欧拉真值 y_N（另以手工闭式交叉验证，防"自己证自己"）。
func implicitTrueY(key string, n int) float64 {
	def := odeProblems[key]
	return eulerImplicit(def.F, def.A, def.B, def.Y0, n)[n]
}

func TestScoreEulerImplicit_ClosedForms(t *testing.T) {
	// exp_growth：隐式欧拉每步除以 (1−h)，n=4 h=0.25 恰 (4/3)^4 = 256/81
	if got := implicitTrueY("exp_growth", 4); math.Abs(got-256.0/81.0) > 1e-12 {
		t.Fatalf("exp_growth n=4: want 256/81=%.12g, got %.12g", 256.0/81.0, got)
	}
	// decay_sq：手算锚点（每步除以 1+2h·x_{n+1}，两位小数可复算）
	if got := implicitTrueY("decay_sq", 4); math.Abs(got-0.344781144781) > 1e-12 {
		t.Fatalf("decay_sq n=4: want ~0.3447814781, got %.12g", got)
	}
	// stiff_decay：hλ=−2.5，R=1/(1−hλ)=1/3.5=2/7，A 稳定单调收敛 (2/7)^4=16/2401
	if got := implicitTrueY("stiff_decay", 4); math.Abs(got-16.0/2401.0) > 1e-15 {
		t.Fatalf("stiff_decay n=4: want 16/2401=%.12g, got %.12g", 16.0/2401.0, got)
	}
	// 结构性恒等式：y'=y 的隐式欧拉对任意 a/b/n 都是 (1−h)^{−n}
	for _, c := range []struct{ a, b float64; n int }{
		{0, 1, 2}, {0, 2, 5}, {-1, 1, 4}, {0.5, 1.25, 8},
	} {
		h := (c.b - c.a) / float64(c.n)
		want := math.Pow(1-h, float64(-c.n))
		if got := eulerImplicit(odeProblems["exp_growth"].F, c.a, c.b, 1, c.n)[c.n]; math.Abs(got-want) > 1e-12 {
			t.Fatalf("exp_growth 隐式欧拉 (1−h)^{−n} 恒等式破坏: a=%g b=%g n=%d: want %.12g got %.12g",
				c.a, c.b, c.n, want, got)
		}
	}
	// A 稳定恒等式：stiff_decay 任意区间/步数下终值 = (1+10h)^{−n}（恒 |R|<1 收敛）
	for _, c := range []struct{ a, b float64; n int }{
		{0, 0.4, 4}, {0, 1, 2}, {0, 2, 8},
	} {
		h := (c.b - c.a) / float64(c.n)
		want := math.Pow(1+10*h, float64(-c.n))
		if got := eulerImplicit(odeProblems["stiff_decay"].F, c.a, c.b, 1, c.n)[c.n]; math.Abs(got-want) > 1e-12 {
			t.Fatalf("stiff_decay A 稳定 (1+10h)^{−n} 恒等式破坏: a=%g b=%g n=%d: want %.12g got %.12g",
				c.a, c.b, c.n, want, got)
		}
	}
}

func TestScoreEulerImplicit_ExactSolution(t *testing.T) {
	// exp_growth n=4 全列手算闭式：y_col=[1,4/3,16/9,64/27,256/81]
	readings := `[{"value":3.16049382716049,"y_col":[1,1.33333333333333,1.77777777777778,2.37037037037037,3.16049382716049]}]`
	score, passed, detail := submitEulerImplicit(t, readings, "")
	if score != 100 || !passed {
		t.Fatalf("want 100/passed, got %d/%v (detail=%s)", score, passed, detail)
	}
	if !strings.Contains(detail, "problem=exp_growth") || !strings.Contains(detail, "n=4") {
		t.Errorf("detail 应记录 problem/n: %s", detail)
	}
	if !strings.Contains(detail, "ref_Y=2.71828") {
		t.Errorf("detail 应记录精确解 ref_Y: %s", detail)
	}
	if !strings.Contains(detail, "y_col=[1,") {
		t.Errorf("detail 应记录 y_col: %s", detail)
	}
	if !strings.Contains(detail, "隐式欧拉") {
		t.Errorf("detail 应标注方法（隐式欧拉）: %s", detail)
	}
}

func TestScoreEulerImplicit_RelErrCurve(t *testing.T) {
	y := implicitTrueY("exp_growth", 4)
	for _, c := range []struct {
		relErr float64
		want   int
	}{
		{1e-7, 100},
		{1e-4, 90},
		{1e-3, 80},
		{1e-2, 65},
		{0.1, 30},
		{0.5, 0},
	} {
		readings := fmt.Sprintf(`[{"value":%.12g}]`, y*(1+c.relErr))
		if score, _, _ := submitEulerImplicit(t, readings, ""); score != c.want {
			t.Errorf("rel_err=%g: want %d, got %d", c.relErr, c.want, score)
		}
	}
}

func TestScoreEulerImplicit_Traps(t *testing.T) {
	// 抄精确解 e（漏做全部递推）：隐式比显式离 e 更远（差 ~14%）→ ~24 分不过线
	if score, passed, _ := submitEulerImplicit(t, `[{"value":2.7182818285}]`, ""); passed || score >= 60 {
		t.Fatalf("抄精确解: want <60/not passed, got %d/%v", score, passed)
	}
	// 抄 L20 显式欧拉答案 2.44140625（方向用反）：差 ~23% → ~15 分不过线
	if score, passed, _ := submitEulerImplicit(t, `[{"value":2.44140625}]`, ""); passed || score >= 60 {
		t.Fatalf("抄显式欧拉答案: want <60/not passed, got %d/%v", score, passed)
	}
	// 抄 L21 改进欧拉答案 2.640625：差 ~16% → ~21 分不过线
	if score, passed, _ := submitEulerImplicit(t, `[{"value":2.640625}]`, ""); passed || score >= 60 {
		t.Fatalf("抄改进欧拉答案: want <60/not passed, got %d/%v", score, passed)
	}
	// 抄 L22 RK4 答案 65/24：差 ~14% → ~23 分不过线
	if score, passed, _ := submitEulerImplicit(t, `[{"value":2.70833333333333}]`, ""); passed || score >= 60 {
		t.Fatalf("抄 RK4 答案: want <60/not passed, got %d/%v", score, passed)
	}
	// 抄 L23 阿当姆斯答案 2.7182260：差 ~14% → ~24 分不过线
	if score, passed, _ := submitEulerImplicit(t, `[{"value":2.71822598327}]`, ""); passed || score >= 60 {
		t.Fatalf("抄阿当姆斯答案: want <60/not passed, got %d/%v", score, passed)
	}
}

func TestScoreEulerImplicit_StiffAStability(t *testing.T) {
	// 刚性对照主场：hλ=−2.5 时 L20 显式振荡发散 (−1.5)^4=5.0625，
	// 隐式 A 稳定单调收敛 (2/7)^4=16/2401≈0.006664——老实递推照样 100
	readings := `[{"value":0.00666389004664,"y_col":[1,0.285714285714,0.0816326530612,0.0233236151604,0.00666389004664]}]`
	if score, passed, detail := submitEulerImplicit(t, readings, `{"problem_key":"stiff_decay"}`); score != 100 || !passed {
		t.Fatalf("stiff 正确递推（A 稳定收敛）: want 100/passed, got %d/%v (detail=%s)", score, passed, detail)
	}
	// 抄精确解 e^{−10}：与收敛值差 ~99% → 0 分
	if score, passed, _ := submitEulerImplicit(t, fmt.Sprintf(`[{"value":%.12g}]`, math.Exp(-10)), `{"problem_key":"stiff_decay"}`); passed || score > 0 {
		t.Fatalf("stiff 抄精确解: want 0/not passed, got %d/%v", score, passed)
	}
	// 抄 L20 显式发散值 5.0625：差 ~99% → 0 分
	if score, passed, _ := submitEulerImplicit(t, `[{"value":5.0625}]`, `{"problem_key":"stiff_decay"}`); passed || score > 0 {
		t.Fatalf("stiff 抄显式发散值: want 0/not passed, got %d/%v", score, passed)
	}
}

func TestScoreEulerImplicit_UserConfigProblemKey(t *testing.T) {
	// 换题 decay_sq（库默认 n=4）：隐式闭式 ~0.3447814781 -> 100
	score, _, detail := submitEulerImplicit(t, fmt.Sprintf(`[{"value":%.12g}]`, implicitTrueY("decay_sq", 4)), `{"problem_key":"decay_sq"}`)
	if score != 100 {
		t.Fatalf("decay_sq 正确值: want 100, got %d (detail=%s)", score, detail)
	}
	if !strings.Contains(detail, "problem=decay_sq") {
		t.Errorf("detail 应记录换题后的 problem: %s", detail)
	}
	// 抄精确解 e^{−1}：差 ~6.7% → ~36 分不过线
	if score, passed, _ := submitEulerImplicit(t, fmt.Sprintf(`[{"value":%.12g}]`, math.Exp(-1)), `{"problem_key":"decay_sq"}`); passed || score >= 60 {
		t.Fatalf("decay_sq 抄精确解: want <60/not passed, got %d/%v", score, passed)
	}
}

func TestScoreEulerImplicit_UserConfigN(t *testing.T) {
	// n=8：h=0.125，(8/7)^8 = 16777216/5764801 ≈ 2.91028536805（12 位手算锚点）
	if got := implicitTrueY("exp_growth", 8); math.Abs(got-2.91028536805) > 1e-10 {
		t.Fatalf("exp_growth n=8 锚点: got %.12g", got)
	}
	if score, _, _ := submitEulerImplicit(t, fmt.Sprintf(`[{"value":%.12g}]`, implicitTrueY("exp_growth", 8)), `{"n":8}`); score != 100 {
		t.Fatalf("n=8 正确值: want 100, got %d", score)
	}
}

func TestScoreEulerImplicit_UserConfigInterval(t *testing.T) {
	// 换区间 [0,2] 并换 n=8：h=0.25，(4/3)^8 = 65536/6561 ≈ 9.98872123152
	if got := eulerImplicit(odeProblems["exp_growth"].F, 0, 2, 1, 8)[8]; math.Abs(got-9.98872123152) > 1e-10 {
		t.Fatalf("[0,2] n=8 锚点: got %.12g", got)
	}
	score, _, detail := submitEulerImplicit(t,
		fmt.Sprintf(`[{"value":%.12g}]`, eulerImplicit(odeProblems["exp_growth"].F, 0, 2, 1, 8)[8]),
		`{"a":0,"b":2,"n":8}`)
	if score != 100 {
		t.Fatalf("换区间换 n 正确值: want 100, got %d", score)
	}
	if !strings.Contains(detail, "a=0") || !strings.Contains(detail, "b=2") {
		t.Errorf("detail 应记录换后区间: %s", detail)
	}
}

func TestScoreEulerImplicit_SingularStep(t *testing.T) {
	// h=1 恰使 1−h·f(x_{n+1},1)=0：隐式方程 y=y_n+h·y 无有限解（后退欧拉
	// 真值发散），应 400 而非给出错误真值
	if _, _, _, err := scoreByExperiment("euler_implicit",
		[]byte(`[{"value":2}]`), []byte(eulerImplicitTarget), []byte(`{"n":1}`)); err == nil {
		t.Fatal("h=1 奇异步长应报错（隐式方程无有限解）")
	}
}

func TestScoreEulerImplicit_TargetParamsOnlyForTargetProblem(t *testing.T) {
	// target 带 exp_growth 的 n=8；user_config 换 decay_sq 不带 n 时应用库内
	// 默认 n=4，而非沿用 target 的 8（v0.12.1 归属规则，L20 同款陷阱）
	tgt := `{"method":"euler_implicit_rel_err","problem_key":"exp_growth","n":8,"pass_score":60}`
	score, _, _, err := scoreByExperiment("euler_implicit",
		[]byte(fmt.Sprintf(`[{"value":%.12g}]`, implicitTrueY("decay_sq", 4))), []byte(tgt), []byte(`{"problem_key":"decay_sq"}`))
	if err != nil {
		t.Fatal(err)
	}
	if score != 100 {
		t.Fatalf("换题不带 n 应用库内默认 n=4: want 100, got %d", score)
	}
	// 换题后若错误沿用 target 的 n=8，按 n=8 提交对不上 n=4 真值
	d8 := eulerImplicit(odeProblems["decay_sq"].F, 0, 1, 1, 8)[8]
	score, _, _, err = scoreByExperiment("euler_implicit",
		[]byte(fmt.Sprintf(`[{"value":%.12g}]`, d8)), []byte(tgt), []byte(`{"problem_key":"decay_sq"}`))
	if err != nil {
		t.Fatal(err)
	}
	if score >= 100 {
		t.Fatalf("按 n=8 提交应暴露不符（truth 是 n=4）: got %d", score)
	}
	// 未换题：target 的 n=8 生效
	score, _, _, err = scoreByExperiment("euler_implicit",
		[]byte(fmt.Sprintf(`[{"value":%.12g}]`, implicitTrueY("exp_growth", 8))), []byte(tgt), nil)
	if err != nil {
		t.Fatal(err)
	}
	if score != 100 {
		t.Fatalf("未换题时 target 的 n=8 应生效: want 100, got %d", score)
	}
}

// ===== L17 LU 分解解线性方程组（lu_decomp，L27 id 27）=====

const luTarget = `{"method":"lu_rel_err","system_key":"sys3_basic","pass_score":60}`

func submitLU(t *testing.T, readings, userConfig string) (int, bool, string) {
	t.Helper()
	uc := []byte(nil)
	if userConfig != "" {
		uc = []byte(userConfig)
	}
	score, passed, detail, err := scoreByExperiment("lu_decomp", []byte("["+readings+"]"), []byte(luTarget), uc)
	if err != nil {
		t.Fatalf("readings=%s user_config=%s: %v", readings, userConfig, err)
	}
	return score, passed, detail
}

func TestScoreLUDecomp_ClosedForms(t *testing.T) {
	// sys3_basic 手算闭式：L=[[1,0,0],[-1.5,1,0],[-1,4,1]]，l_col=[-1.5,-1,4]，
	// U=[[2,1,-1],[0,0.5,0.5],[0,0,-1]]，前代 y=(8,1,1)，回代 x=(2,3,-1)
	def := linSystems["sys3_basic"]
	L, U, lCol, err := luDecompose(def.A)
	if err != nil {
		t.Fatal(err)
	}
	if len(lCol) != 3 || math.Abs(lCol[0]+1.5) > 1e-15 || math.Abs(lCol[1]+1) > 1e-15 || math.Abs(lCol[2]-4) > 1e-15 {
		t.Fatalf("sys3_basic 乘子列: want [-1.5,-1,4], got %v", lCol)
	}
	_ = L
	if math.Abs(L[1][0]+1.5) > 1e-15 || math.Abs(L[2][0]+1) > 1e-15 || math.Abs(L[2][1]-4) > 1e-15 {
		t.Fatalf("sys3_basic L 乘子: want -1.5/-1/4, got %v %v %v", L[1][0], L[2][0], L[2][1])
	}
	if math.Abs(U[1][1]-0.5) > 1e-15 || math.Abs(U[2][2]+1) > 1e-15 {
		t.Fatalf("sys3_basic U 对角: want 0.5,-1, got %v %v", U[1][1], U[2][2])
	}
	x, _, err := luSolve(def.A, def.b)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []float64{2, 3, -1} {
		if math.Abs(x[i]-want) > 1e-12 {
			t.Fatalf("sys3_basic LU 解: want (2,3,-1), got %v", x)
		}
	}
	// 库内五题 + 若干自定义阵：LU 路径与列主元高斯同解（精确算术恒等，
	// 病态 hilbert4 放宽容差到 1e-8 相对）
	customs := []struct {
		A [][]float64
		b []float64
	}{
		{[][]float64{{2, 0, 0}, {0, 3, 0}, {0, 0, 4}}, []float64{2, 6, 12}},
		{[][]float64{{4, 2}, {2, 3}}, []float64{8, 7}},
		{[][]float64{{5, 1, 0, 2}, {1, 6, 1, 0}, {0, 1, 7, 1}, {2, 0, 1, 8}}, []float64{10, 11, 12, 13}},
	}
	tol := map[string]float64{"sys3_basic": 1e-12, "sys4_tridiag": 1e-12, "sys3_diagdom": 1e-12, "sys6_poisson": 1e-12, "hilbert4": 1e-8}
	for key, def := range linSystems {
		xLU, _, err := luSolve(def.A, def.b)
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		xG, err := gaussSolve(def.A, def.b)
		if err != nil {
			t.Fatalf("%s gauss: %v", key, err)
		}
		if rel := vecRelError(xLU, xG); rel > tol[key] {
			t.Fatalf("%s LU 与高斯解应一致: rel=%g", key, rel)
		}
	}
	for i, c := range customs {
		xLU, _, err := luSolve(c.A, c.b)
		if err != nil {
			t.Fatalf("custom#%d: %v", i, err)
		}
		xG, err := gaussSolve(c.A, c.b)
		if err != nil {
			t.Fatalf("custom#%d gauss: %v", i, err)
		}
		if rel := vecRelError(xLU, xG); rel > 1e-12 {
			t.Fatalf("custom#%d LU 与高斯解应一致: rel=%g", i, rel)
		}
	}
}

func TestScoreLUDecomp_ExactSolution(t *testing.T) {
	// 正确解 + 正确乘子列 → 100；detail 含方法标注与 l_col/ref_l
	score, passed, detail := submitLU(t, `{"solution":[2,3,-1]}`, ``)
	if score != 100 || !passed {
		t.Fatalf("正确解: want 100/passed, got %d/%v", score, passed)
	}
	for _, kw := range []string{"Doolittle", "lu", "true=[2,3,-1]"} {
		if !strings.Contains(detail, kw) {
			t.Fatalf("detail 缺 %q: %s", kw, detail)
		}
	}
	score, _, detail = submitLU(t, `{"solution":[2,3,-1],"l_col":[-1.5,-1,4]}`, ``)
	if score != 100 {
		t.Fatalf("正确解+审计: want 100, got %d", score)
	}
	for _, kw := range []string{"l_col=[-1.5,-1,4]", "ref_l=[-1.5,-1,4]"} {
		if !strings.Contains(detail, kw) {
			t.Fatalf("detail 缺 %q: %s", kw, detail)
		}
	}
}

func TestScoreLUDecomp_RelErrCurve(t *testing.T) {
	// sys3_basic 真值 (2,3,-1)：按无穷范数相对误差走默认曲线
	// （期望值用同一曲线 helper 计算，钉住"向量范数+默认曲线"的接线）
	sols := [][]float64{
		{2, 3, -1.000001},
		{2.001, 3, -1},
		{2.01, 3, -1},
		{2.1, 3, -1},
		{2.5, 3, -1},
		{8, -11, -3},
	}
	for _, sol := range sols {
		want := scoreFromRelErrBands(vecRelError(sol, []float64{2, 3, -1}), nil)
		score, _, _ := submitLU(t, fmt.Sprintf(`{"solution":[%s]}`, joinGauss(sol)), ``)
		if score != want {
			t.Fatalf("solution=%v: want %d, got %d", sol, want, score)
		}
	}
	// 抄 b 向量应恒 0
	score, _, _ := submitLU(t, `{"solution":[8,-11,-3]}`, ``)
	if score != 0 {
		t.Fatalf("抄 b 向量: want 0, got %d", score)
	}
}

func TestScoreLUDecomp_Traps(t *testing.T) {
	// 抄 b 向量 (8,-11,-3)、抄零向量、分量换位——均低分不过线
	for _, sol := range [][]float64{{8, -11, -3}, {0, 0, 0}, {3, 2, -1}} {
		score, passed, _ := submitLU(t, fmt.Sprintf(`{"solution":[%s]}`, joinGauss(sol)), ``)
		if passed || score >= 60 {
			t.Fatalf("陷阱 solution=%v 不应过线: %d/%v", sol, score, passed)
		}
	}
	// 乘子审计错但解对：仍 100（l_col 仅审计不打分），detail 记录可对照
	score, _, detail := submitLU(t, `{"solution":[2,3,-1],"l_col":[-1.4,-1,4]}`, ``)
	if score != 100 {
		t.Fatalf("l_col 仅审计: want 100, got %d", score)
	}
	if !strings.Contains(detail, "l_col=[-1.4") {
		t.Fatalf("detail 应记录学生 l_col: %s", detail)
	}
}

func TestScoreLUDecomp_GaussAnswerIsCorrect(t *testing.T) {
	// 契约语义：解与 L13 列主元高斯数学同源，抄 L13 答案=算对（同 L18/L16 先例）
	xG, err := gaussSolve(linSystems["hilbert4"].A, linSystems["hilbert4"].b)
	if err != nil {
		t.Fatal(err)
	}
	score, passed, _ := submitLU(t,
		fmt.Sprintf(`{"solution":[%s]}`, joinGauss(xG)), `{"system_key":"hilbert4"}`)
	if score != 100 || !passed {
		t.Fatalf("高斯同解应 100: got %d/%v", score, passed)
	}
}

func TestScoreLUDecomp_UserConfigSystemKey(t *testing.T) {
	// 换题 sys4_tridiag：解 (1,-1,2,0.5)，乘子列长度 4*3/2=6
	score, passed, _ := submitLU(t, `{"solution":[1,-1,2,0.5]}`, `{"system_key":"sys4_tridiag"}`)
	if score != 100 || !passed {
		t.Fatalf("换题 sys4_tridiag: want 100, got %d/%v", score, passed)
	}
	// 换题后按原题答案提交（补一位凑成长度 4）→ 低分（防背答案）
	score, _, _ = submitLU(t, `{"solution":[2,3,-1,0]}`, `{"system_key":"sys4_tridiag"}`)
	if score >= 60 {
		t.Fatalf("换题后旧答案应不过线: got %d", score)
	}
}

func TestScoreLUDecomp_UserConfigCustom(t *testing.T) {
	// 自定义 2 阶 A=[[4,2],[2,3]], b=[8,7]：LU 解 (1.7,0.6)... 手算：
	// l21=0.5；U=[[4,2],[0,2]]；y=(8,3)；x2=1.5, x1=1.25 → (1.25,1.5)
	score, passed, _ := submitLU(t,
		`{"solution":[1.25,1.5],"l_col":[0.5]}`,
		`{"A":[[4,2],[2,3]],"b":[8,7]}`)
	if score != 100 || !passed {
		t.Fatalf("自定义 2 阶: want 100, got %d/%v", score, passed)
	}
}

func TestScoreLUDecomp_PivotBreakdownIsError(t *testing.T) {
	// 非奇异但首主元为 0：不选主元的 LU 分解中断 → 400（与 L13 选主元可解的差异点）
	_, _, _, err := scoreByExperiment("lu_decomp",
		[]byte(`[{"solution":[2,1]}]`), []byte(luTarget),
		[]byte(`{"A":[[0,1],[1,0]],"b":[2,1]}`))
	if err == nil {
		t.Fatal("主元为 0 应报错（LU 中断）")
	}
	// 奇异阵：同样 400
	_, _, _, err = scoreByExperiment("lu_decomp",
		[]byte(`[{"solution":[1,1]}]`), []byte(luTarget),
		[]byte(`{"A":[[1,1],[1,1]],"b":[2,2]}`))
	if err == nil {
		t.Fatal("奇异阵应报错")
	}
}

func TestScoreLUDecomp_BadInput(t *testing.T) {
	cases := []struct {
		readings   string
		userConfig string
	}{
		{`{}`, ``},                                    // 缺 solution
		{`{"solution":[2,3]}`, ``},                    // 长度错
		{`{"solution":[2,3,NaN]}`, ``},                // NaN
		{`{"solution":[2,3,-1],"l_col":[-1.5]}`, ``},  // l_col 长度错（须 3）
		{`{"solution":[1,2,3]}`, `{"system_key":"no_such"}`}, // 未知方程组
		{`{"solution":[1,2,3]}`, `{"A":[[1,2]],"b":[1,2]}`},  // 非方阵
		{`{"solution":[1,2,3]}`, `{"A":[[1,2,3]],"b":[1]}`},  // 1 阶（下限 2）
		{`{"solution":[1,2]}`, `{"A":[[1,2],[3,4]]}`},        // 自定义缺 b
	}
	for i, c := range cases {
		uc := []byte(nil)
		if c.userConfig != "" {
			uc = []byte(c.userConfig)
		}
		_, _, _, err := scoreByExperiment("lu_decomp", []byte("["+c.readings+"]"), []byte(luTarget), uc)
		if err == nil {
			t.Fatalf("case %d 应报错: readings=%s uc=%s", i, c.readings, c.userConfig)
		}
	}
	// 9 阶超上限
	big := `{"A":[[` + strings.Repeat("1,", 8) + `1]`
	for i := 1; i < 9; i++ {
		big += `,[` + strings.Repeat("1,", 8) + `1]`
	}
	big += `],"b":[1,1,1,1,1,1,1,1,1]}`
	_, _, _, err := scoreByExperiment("lu_decomp", []byte(`[{"solution":[1,1,1,1,1,1,1,1,1]}]`),
		[]byte(luTarget), []byte(big))
	if err == nil {
		t.Fatal("9 阶应报错")
	}
}

func TestScoreLUDecomp_TargetSystemKeyOnlyForTarget(t *testing.T) {
	// 未换题时按 target 的 sys3_basic 评分：其它题的答案（sys4 解的前三位）
	// 应不过线（target 生效、真值按目标题现算）
	score, _, _ := submitLU(t, `{"solution":[1,-1,2]}`, ``)
	if score >= 60 {
		t.Fatalf("未换题按 sys3_basic 评分，外来答案应不过线: got %d", score)
	}
}

// ===== L18 追赶法解三对角方程组（thomas_solve，L28 id 28）=====

const thomasTarget = `{"method":"thomas_rel_err","system_key":"tri1_basic","pass_score":60}`

func submitThomas(t *testing.T, readings, userConfig string) (int, bool, string) {
	t.Helper()
	uc := []byte(nil)
	if userConfig != "" {
		uc = []byte(userConfig)
	}
	score, passed, detail, err := scoreByExperiment("thomas_solve", []byte("["+readings+"]"), []byte(thomasTarget), uc)
	if err != nil {
		t.Fatalf("readings=%s user_config=%s: %v", readings, userConfig, err)
	}
	return score, passed, detail
}

// triToFull 三对角 a/b/c 还原为满矩阵（与 gaussSolve/luSolve 交叉验证用）
func triToFull(a, b, c []float64) [][]float64 {
	n := len(b)
	A := make([][]float64, n)
	for i := range A {
		A[i] = make([]float64, n)
		A[i][i] = b[i]
		if i > 0 {
			A[i][i-1] = a[i-1]
		}
		if i < n-1 {
			A[i][i+1] = c[i]
		}
	}
	return A
}

func TestScoreThomas_ClosedForms(t *testing.T) {
	// tri1_basic 手算闭式：追出 m=(0.5,0.8,0.3125)，beta=(2,2.5,3.2,4.6875)，
	// d'=(4,8,13.6,18.75)；赶回 x=(1,2,3,4)
	def := triSystems["tri1_basic"]
	x, m, beta, err := thomasChase(def.a, def.b, def.c, def.d)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []float64{1, 2, 3, 4} {
		if math.Abs(x[i]-want) > 1e-12 {
			t.Fatalf("tri1_basic 追赶解: want (1,2,3,4), got %v", x)
		}
	}
	for i, want := range []float64{0.5, 0.8, 0.3125} {
		if math.Abs(m[i]-want) > 1e-12 {
			t.Fatalf("tri1_basic 乘子: want (0.5,0.8,0.3125), got %v", m)
		}
	}
	for i, want := range []float64{2, 2.5, 3.2, 4.6875} {
		if math.Abs(beta[i]-want) > 1e-12 {
			t.Fatalf("tri1_basic 新主对角: want (2,2.5,3.2,4.6875), got %v", beta)
		}
	}
	// tri2_poisson 闭式：beta=(2,3/2,4/3,5/4,6/5)，解全 1
	def2 := triSystems["tri2_poisson"]
	x2, m2, beta2, err := thomasChase(def2.a, def2.b, def2.c, def2.d)
	if err != nil {
		t.Fatal(err)
	}
	for i := range x2 {
		if math.Abs(x2[i]-1) > 1e-12 {
			t.Fatalf("tri2_poisson 解: want 全 1, got %v", x2)
		}
		if math.Abs(beta2[i]-(float64(i)+2)/float64(i+1)) > 1e-12 {
			t.Fatalf("tri2_poisson beta: want (2,3/2,4/3,5/4,6/5), got %v", beta2)
		}
	}
	for i := range m2 { // 泊松阵下对角为 -1，乘子必为负（长 n-1）
		if m2[i] > 0 {
			t.Fatalf("tri2_poisson 乘子应为负, got %v", m2)
		}
	}
	// 题库四题 + 自定义三对角：追赶法与列主元高斯 / LU 分解同解（精确算术恒等）
	customs := []struct {
		a, b, c, d []float64
	}{
		{[]float64{1, 1}, []float64{2, 2, 2}, []float64{1, 1}, []float64{4, 8, 8}}, // 解恰 (1,2,3)
		{[]float64{1, 1, 1, 1, 1, 1, 1}, []float64{3, 3, 3, 3, 3, 3, 3, 3},
			[]float64{1, 1, 1, 1, 1, 1, 1}, []float64{2, 1, 1, 1, 1, 1, 1, 2}}, // 8 阶
	}
	for key, def := range triSystems {
		xT, _, _, err := thomasChase(def.a, def.b, def.c, def.d)
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		A := triToFull(def.a, def.b, def.c)
		xG, err := gaussSolve(A, def.d)
		if err != nil {
			t.Fatalf("%s gauss: %v", key, err)
		}
		if rel := vecRelError(xT, xG); rel > 1e-12 {
			t.Fatalf("%s 追赶与高斯解应一致: rel=%g", key, rel)
		}
		xU, _, err := luSolve(A, def.d)
		if err != nil {
			t.Fatalf("%s lu: %v", key, err)
		}
		if rel := vecRelError(xT, xU); rel > 1e-12 {
			t.Fatalf("%s 追赶与 LU 解应一致: rel=%g", key, rel)
		}
	}
	for i, cc := range customs {
		xT, _, _, err := thomasChase(cc.a, cc.b, cc.c, cc.d)
		if err != nil {
			t.Fatalf("custom#%d: %v", i, err)
		}
		A := triToFull(cc.a, cc.b, cc.c)
		xG, err := gaussSolve(A, cc.d)
		if err != nil {
			t.Fatalf("custom#%d gauss: %v", i, err)
		}
		if rel := vecRelError(xT, xG); rel > 1e-12 {
			t.Fatalf("custom#%d 追赶与高斯解应一致: rel=%g", i, rel)
		}
	}
	// 手验 custom#0 解恰 (1,2,3)
	xC, mC, betaC, err := thomasChase(customs[0].a, customs[0].b, customs[0].c, customs[0].d)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []float64{1, 2, 3} {
		if math.Abs(xC[i]-want) > 1e-12 {
			t.Fatalf("custom#0 解: want (1,2,3), got %v", xC)
		}
	}
	if math.Abs(mC[0]-0.5) > 1e-15 || math.Abs(betaC[2]-4.0/3) > 1e-15 {
		t.Fatalf("custom#0 m/beta: want (0.5,2/3)/(2,1.5,4/3), got %v/%v", mC, betaC)
	}
}

func TestScoreThomas_ExactSolution(t *testing.T) {
	// 正确解 → 100；detail 含方法标注与 true
	score, passed, detail := submitThomas(t, `{"solution":[1,2,3,4]}`, ``)
	if score != 100 || !passed {
		t.Fatalf("正确解: want 100/passed, got %d/%v", score, passed)
	}
	for _, kw := range []string{"追赶法", "true=[1,2,3,4]"} {
		if !strings.Contains(detail, kw) {
			t.Fatalf("detail 缺 %q: %s", kw, detail)
		}
	}
	// 正确解 + m/beta 双审计 → 100，detail 记录 ref_m/ref_beta 供教师对照
	score, _, detail = submitThomas(t,
		`{"solution":[1,2,3,4],"m":[0.5,0.8,0.3125],"beta":[2,2.5,3.2,4.6875]}`, ``)
	if score != 100 {
		t.Fatalf("正确解+双审计: want 100, got %d", score)
	}
	for _, kw := range []string{"m=[0.5,0.8,0.3125]", "ref_m=[0.5,0.8,0.3125]",
		"beta=[2,2.5,3.2,4.6875]", "ref_beta=[2,2.5,3.2,4.6875]"} {
		if !strings.Contains(detail, kw) {
			t.Fatalf("detail 缺 %q: %s", kw, detail)
		}
	}
}

func TestScoreThomas_RelErrCurve(t *testing.T) {
	// tri1_basic 真值 (1,2,3,4)：按无穷范数相对误差走默认曲线
	// （期望值用同一曲线 helper 计算，钉住"向量范数+默认曲线"的接线）
	sols := [][]float64{
		{1, 2, 3, 4.000001},
		{1.001, 2, 3, 4},
		{1.01, 2, 3, 4},
		{1.1, 2, 3, 4},
		{1.5, 2, 3, 4},
		{4, 10, 20, 23},
	}
	for _, sol := range sols {
		want := scoreFromRelErrBands(vecRelError(sol, []float64{1, 2, 3, 4}), nil)
		score, _, _ := submitThomas(t, fmt.Sprintf(`{"solution":[%s]}`, joinGauss(sol)), ``)
		if score != want {
			t.Fatalf("solution=%v: want %d, got %d", sol, want, score)
		}
	}
	// 抄 d 向量应恒 0
	score, _, _ := submitThomas(t, `{"solution":[4,10,20,23]}`, ``)
	if score != 0 {
		t.Fatalf("抄 d 向量: want 0, got %d", score)
	}
}

func TestScoreThomas_Traps(t *testing.T) {
	// 抄 d 向量 (4,10,20,23)、抄零向量、分量换位——均低分不过线
	for _, sol := range [][]float64{{4, 10, 20, 23}, {0, 0, 0, 0}, {2, 1, 4, 3}} {
		score, passed, _ := submitThomas(t, fmt.Sprintf(`{"solution":[%s]}`, joinGauss(sol)), ``)
		if passed || score >= 60 {
			t.Fatalf("陷阱 solution=%v 不应过线: %d/%v", sol, score, passed)
		}
	}
	// m/beta 审计错但解对：仍 100（仅审计不打分），detail 记录可对照
	score, _, detail := submitThomas(t,
		`{"solution":[1,2,3,4],"m":[0.4,0.8,0.3125],"beta":[2,2.5,3.2,4.6875]}`, ``)
	if score != 100 {
		t.Fatalf("m 仅审计: want 100, got %d", score)
	}
	if !strings.Contains(detail, "m=[0.4") {
		t.Fatalf("detail 应记录学生 m: %s", detail)
	}
}

func TestScoreThomas_GaussAnswerIsCorrect(t *testing.T) {
	// 契约语义：解与 L13/L27 数学同源，抄高斯答案=算对（同 L18/L16 先例）
	def := triSystems["tri4_spline"]
	xG, err := gaussSolve(triToFull(def.a, def.b, def.c), def.d)
	if err != nil {
		t.Fatal(err)
	}
	score, passed, _ := submitThomas(t,
		fmt.Sprintf(`{"solution":[%s]}`, joinGauss(xG)), `{"system_key":"tri4_spline"}`)
	if score != 100 || !passed {
		t.Fatalf("高斯同解应 100: got %d/%v", score, passed)
	}
}

func TestScoreThomas_UserConfigSystemKey(t *testing.T) {
	// 换题 tri2_poisson：解全 1；换题后按原题答案提交（补一位凑长度 5）→ 低分
	score, passed, _ := submitThomas(t, `{"solution":[1,1,1,1,1]}`, `{"system_key":"tri2_poisson"}`)
	if score != 100 || !passed {
		t.Fatalf("换题 tri2_poisson: want 100, got %d/%v", score, passed)
	}
	score, _, _ = submitThomas(t, `{"solution":[1,2,3,4,0]}`, `{"system_key":"tri2_poisson"}`)
	if score >= 60 {
		t.Fatalf("换题后旧答案应不过线: got %d", score)
	}
}

func TestScoreThomas_UserConfigCustom(t *testing.T) {
	// 自定义 3 阶 a=[1,1], b=[2,2,2], c=[1,1], d=[4,8,8]：
	// 追出 m=(0.5,2/3)，beta=(2,1.5,4/3)，解恰 (1,2,3)
	score, passed, _ := submitThomas(t,
		`{"solution":[1,2,3],"m":[0.5,0.6666666667],"beta":[2,1.5,1.3333333333]}`,
		`{"a":[1,1],"b":[2,2,2],"c":[1,1],"d":[4,8,8]}`)
	if score != 100 || !passed {
		t.Fatalf("自定义 3 阶: want 100, got %d/%v", score, passed)
	}
}

func TestScoreThomas_PivotBreakdownIsError(t *testing.T) {
	// 非奇异但首主元为 0（b1=0，det=-1 需选主元）：追赶法第 1 步中断 → 400
	_, _, _, err := scoreByExperiment("thomas_solve",
		[]byte(`[{"solution":[2,1,1]}]`), []byte(thomasTarget),
		[]byte(`{"a":[1,1],"b":[0,1,1],"c":[1,1],"d":[2,2,2]}`))
	if err == nil {
		t.Fatal("首主元为 0 应报错（追赶中断）")
	}
	// 非奇异但中途 beta≈0（beta2 = 1 - 1*1 = 0）：同样 400
	_, _, _, err = scoreByExperiment("thomas_solve",
		[]byte(`[{"solution":[1,1,1]}]`), []byte(thomasTarget),
		[]byte(`{"a":[1,1],"b":[1,1,1],"c":[1,1],"d":[1,1,1]}`))
	if err == nil {
		t.Fatal("中途 beta≈0 应报错")
	}
}

func TestScoreThomas_BadInput(t *testing.T) {
	cases := []struct {
		readings   string
		userConfig string
	}{
		{`{}`, ``},                                            // 缺 solution
		{`{"solution":[1,2,3]}`, ``},                          // 长度错（须 4）
		{`{"solution":[1,2,3,NaN]}`, ``},                      // NaN
		{`{"solution":[1,2,3,4],"m":[0.5]}`, ``},              // m 长度错（须 3）
		{`{"solution":[1,2,3,4],"beta":[2,2.5]}`, ``},         // beta 长度错（须 4）
		{`{"solution":[1,2,3,4]}`, `{"system_key":"no_such"}`}, // 未知方程组
		{`{"solution":[1,2,3]}`, `{"b":[2,2,2]}`},              // 自定义缺 a/c/d
		{`{"solution":[1,2]}`, `{"a":[1],"b":[2,2],"c":[1],"d":[1,2]}`}, // 2 阶（下限 3）
		{`{"solution":[1,2,3]}`, `{"a":[1,1,1],"b":[2,2,2],"c":[1,1],"d":[4,8,8]}`}, // a 长度错
	}
	for i, c := range cases {
		uc := []byte(nil)
		if c.userConfig != "" {
			uc = []byte(c.userConfig)
		}
		_, _, _, err := scoreByExperiment("thomas_solve", []byte("["+c.readings+"]"), []byte(thomasTarget), uc)
		if err == nil {
			t.Fatalf("case %d 应报错: readings=%s uc=%s", i, c.readings, c.userConfig)
		}
	}
	// 9 阶超上限
	big := `{"a":[1,1,1,1,1,1,1,1],"b":[3,3,3,3,3,3,3,3,3],"c":[1,1,1,1,1,1,1,1],"d":[` +
		strings.Repeat("1,", 8) + `1]}`
	_, _, _, err := scoreByExperiment("thomas_solve", []byte(`[{"solution":[1,1,1,1,1,1,1,1,1]}]`),
		[]byte(thomasTarget), []byte(big))
	if err == nil {
		t.Fatal("9 阶应报错")
	}
}

func TestScoreThomas_TargetSystemKeyOnlyForTarget(t *testing.T) {
	// 未换题时按 target 的 tri1_basic 评分：外来答案（tri2 泊松解全 1）
	// 应不过线（target 生效、真值按目标题现算）
	score, _, _ := submitThomas(t, `{"solution":[1,1,1,1]}`, ``)
	if score >= 60 {
		t.Fatalf("未换题按 tri1_basic 评分，外来答案应不过线: got %d", score)
	}
}

const choleskyTarget = `{"method":"cholesky_rel_err","system_key":"spd2_min3","pass_score":60}`

func submitCholesky(t *testing.T, readings, userConfig string) (int, bool, string) {
	t.Helper()
	uc := []byte(nil)
	if userConfig != "" {
		uc = []byte(userConfig)
	}
	score, passed, detail, err := scoreByExperiment("cholesky_solve", []byte("["+readings+"]"), []byte(choleskyTarget), uc)
	if err != nil {
		t.Fatalf("readings=%s user_config=%s: %v", readings, userConfig, err)
	}
	return score, passed, detail
}

func TestScoreCholesky_ClosedForms(t *testing.T) {
	// spd2_min3（默认题）手算闭式：min(i,j) 阵的 L=下三角全 1，
	// 前代 y=(6,5,3)，回代解恰 (1,2,3)
	def := spdSystems["spd2_min3"]
	x, y, L, err := choleskySolve(def.A, def.b)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []float64{1, 2, 3} {
		if math.Abs(x[i]-want) > 1e-12 {
			t.Fatalf("spd2_min3 解: want (1,2,3), got %v", x)
		}
		if math.Abs(y[i]-[]float64{6, 5, 3}[i]) > 1e-12 {
			t.Fatalf("spd2_min3 y: want (6,5,3), got %v", y)
		}
	}
	for i := range L {
		for j := 0; j <= i; j++ {
			if math.Abs(L[i][j]-1) > 1e-12 {
				t.Fatalf("spd2_min3 L: want 下三角全 1, got %v", L)
			}
		}
		for j := i + 1; j < len(L); j++ {
			if L[i][j] != 0 {
				t.Fatalf("spd2_min3 L: 上三角应为 0, got %v", L)
			}
		}
	}
	// spd1_2x2 闭式：L=[[2,0],[1,2]]，y=(3,2)，解恰 (1,1)
	x1, y1, L1, err := choleskySolve(spdSystems["spd1_2x2"].A, spdSystems["spd1_2x2"].b)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(x1[0]-1) > 1e-12 || math.Abs(x1[1]-1) > 1e-12 ||
		math.Abs(y1[0]-3) > 1e-12 || math.Abs(y1[1]-2) > 1e-12 ||
		math.Abs(L1[0][0]-2) > 1e-12 || math.Abs(L1[1][0]-1) > 1e-12 || math.Abs(L1[1][1]-2) > 1e-12 {
		t.Fatalf("spd1_2x2 闭式: want L=[[2,0],[1,2]] y=(3,2) x=(1,1), got L=%v y=%v x=%v", L1, y1, x1)
	}
	// spd3_tridiag3 闭式：L=[[2,0,0],[1,2,0],[0,1,3]]，y=(3,3,3)，解恰 (1,1,1)
	x3, y3, L3, err := choleskySolve(spdSystems["spd3_tridiag3"].A, spdSystems["spd3_tridiag3"].b)
	if err != nil {
		t.Fatal(err)
	}
	for i := range x3 {
		if math.Abs(x3[i]-1) > 1e-12 || math.Abs(y3[i]-3) > 1e-12 {
			t.Fatalf("spd3_tridiag3: want x=(1,1,1) y=(3,3,3), got x=%v y=%v", x3, y3)
		}
	}
	if math.Abs(L3[0][0]-2) > 1e-12 || math.Abs(L3[1][0]-1) > 1e-12 || math.Abs(L3[1][1]-2) > 1e-12 ||
		math.Abs(L3[2][1]-1) > 1e-12 || math.Abs(L3[2][2]-3) > 1e-12 {
		t.Fatalf("spd3_tridiag3 L: want [[2,0,0],[1,2,0],[0,1,3]], got %v", L3)
	}
	// spd4_min4 闭式：L=下三角全 1，y=(4,3,2,1)，解全 1
	x4, y4, _, err := choleskySolve(spdSystems["spd4_min4"].A, spdSystems["spd4_min4"].b)
	if err != nil {
		t.Fatal(err)
	}
	for i := range x4 {
		if math.Abs(x4[i]-1) > 1e-12 || math.Abs(y4[i]-float64(4-i)) > 1e-12 {
			t.Fatalf("spd4_min4: want x 全 1 y=(4,3,2,1), got x=%v y=%v", x4, y4)
		}
	}
	// 题库四题 + 自定义对称正定阵：LLᵀ 重建恒等式、L 主对角全正、
	// 与列主元高斯 / LU 分解同解（精确算术恒等）
	customs := []struct {
		A [][]float64
		b []float64
	}{
		{[][]float64{{1, 2}, {2, 5}}, []float64{3, 7}},        // L=[[1,0],[2,1]]，解恰 (1,1)
		{[][]float64{{4, 2, 0, 0}, {2, 5, 2, 0}, {0, 2, 5, 2}, {0, 0, 2, 5}}, []float64{6, 9, 9, 7}}, // 4 阶三对角 SPD
	}
	all := map[string]spdDef{}
	for k, v := range spdSystems {
		all[k] = v
	}
	for i, cc := range customs {
		all[fmt.Sprintf("custom#%d", i)] = spdDef{A: cc.A, b: cc.b}
	}
	for key, def := range all {
		xC, _, LC, err := choleskySolve(def.A, def.b)
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		n := len(def.A)
		for i := 0; i < n; i++ {
			if LC[i][i] <= 0 {
				t.Fatalf("%s L 主对角应全正: %v", key, LC[i][i])
			}
			for j := 0; j < n; j++ {
				s := 0.0
				for k := 0; k < n; k++ {
					s += LC[i][k] * LC[j][k]
				}
				if math.Abs(s-def.A[i][j]) > 1e-9 {
					t.Fatalf("%s LLᵀ 重建: A[%d][%d] want %g, got %g", key, i, j, def.A[i][j], s)
				}
			}
		}
		xG, err := gaussSolve(def.A, def.b)
		if err != nil {
			t.Fatalf("%s gauss: %v", key, err)
		}
		if rel := vecRelError(xC, xG); rel > 1e-12 {
			t.Fatalf("%s 平方根法与高斯解应一致: rel=%g", key, rel)
		}
		xU, _, err := luSolve(def.A, def.b)
		if err != nil {
			t.Fatalf("%s lu: %v", key, err)
		}
		if rel := vecRelError(xC, xU); rel > 1e-12 {
			t.Fatalf("%s 平方根法与 LU 解应一致: rel=%g", key, rel)
		}
	}
	// 手验 custom#0：L=[[1,0],[2,1]]，y=(3,1)，解恰 (1,1)
	xC0, yC0, LC0, err := choleskySolve(customs[0].A, customs[0].b)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(xC0[0]-1) > 1e-15 || math.Abs(xC0[1]-1) > 1e-15 ||
		math.Abs(yC0[0]-3) > 1e-15 || math.Abs(yC0[1]-1) > 1e-15 ||
		math.Abs(LC0[1][0]-2) > 1e-15 {
		t.Fatalf("custom#0: want L=[[1,0],[2,1]] y=(3,1) x=(1,1), got L=%v y=%v x=%v", LC0, yC0, xC0)
	}
}

func TestScoreCholesky_ExactSolution(t *testing.T) {
	// 正确解 → 100；detail 含方法标注与 true
	score, passed, detail := submitCholesky(t, `{"solution":[1,2,3]}`, ``)
	if score != 100 || !passed {
		t.Fatalf("正确解: want 100/passed, got %d/%v", score, passed)
	}
	for _, kw := range []string{"平方根法", "true=[1,2,3]"} {
		if !strings.Contains(detail, kw) {
			t.Fatalf("detail 缺 %q: %s", kw, detail)
		}
	}
	// 正确解 + L/y 双审计 → 100，detail 记录 ref_L/ref_y 供教师对照
	score, _, detail = submitCholesky(t,
		`{"solution":[1,2,3],"L":[[1,0,0],[1,1,0],[1,1,1]],"y":[6,5,3]}`, ``)
	if score != 100 {
		t.Fatalf("正确解+双审计: want 100, got %d", score)
	}
	for _, kw := range []string{"L=[1,1,1,1,1,1]", "ref_L=[1,1,1,1,1,1]",
		"y=[6,5,3]", "ref_y=[6,5,3]"} {
		if !strings.Contains(detail, kw) {
			t.Fatalf("detail 缺 %q: %s", kw, detail)
		}
	}
}

func TestScoreCholesky_RelErrCurve(t *testing.T) {
	// spd2_min3 真值 (1,2,3)：按无穷范数相对误差走默认曲线
	// （期望值用同一曲线 helper 计算，钉住"向量范数+默认曲线"的接线）
	sols := [][]float64{
		{1, 2, 3.000001},
		{1.001, 2, 3},
		{1.01, 2, 3},
		{1.1, 2, 3},
		{1.5, 2, 3},
		{6, 11, 14},
	}
	for _, sol := range sols {
		want := scoreFromRelErrBands(vecRelError(sol, []float64{1, 2, 3}), nil)
		score, _, _ := submitCholesky(t, fmt.Sprintf(`{"solution":[%s]}`, joinGauss(sol)), ``)
		if score != want {
			t.Fatalf("solution=%v: want %d, got %d", sol, want, score)
		}
	}
	// 抄 b 向量应恒 0
	score, _, _ := submitCholesky(t, `{"solution":[6,11,14]}`, ``)
	if score != 0 {
		t.Fatalf("抄 b 向量: want 0, got %d", score)
	}
}

func TestScoreCholesky_Traps(t *testing.T) {
	// 抄 b 向量 (6,11,14)、抄零向量、分量换位——均低分不过线
	for _, sol := range [][]float64{{6, 11, 14}, {0, 0, 0}, {2, 1, 3}} {
		score, passed, _ := submitCholesky(t, fmt.Sprintf(`{"solution":[%s]}`, joinGauss(sol)), ``)
		if passed || score >= 60 {
			t.Fatalf("陷阱 solution=%v 不应过线: %d/%v", sol, score, passed)
		}
	}
	// L/y 审计错但解对：仍 100（仅审计不打分），detail 记录可对照
	score, _, detail := submitCholesky(t,
		`{"solution":[1,2,3],"L":[[1,0,0],[2,1,0],[1,1,1]],"y":[6,5,4]}`, ``)
	if score != 100 {
		t.Fatalf("L/y 仅审计: want 100, got %d", score)
	}
	if !strings.Contains(detail, "L=[1,2,1,1,1,1]") {
		t.Fatalf("detail 应记录学生 L: %s", detail)
	}
}

func TestScoreCholesky_GaussAnswerIsCorrect(t *testing.T) {
	// 契约语义：解与 L13/L27/L28 数学同源，抄高斯答案=算对（同 L18/L16 先例）
	def := spdSystems["spd3_tridiag3"]
	xG, err := gaussSolve(def.A, def.b)
	if err != nil {
		t.Fatal(err)
	}
	score, passed, _ := submitCholesky(t,
		fmt.Sprintf(`{"solution":[%s]}`, joinGauss(xG)), `{"system_key":"spd3_tridiag3"}`)
	if score != 100 || !passed {
		t.Fatalf("高斯同解应 100: got %d/%v", score, passed)
	}
}

func TestScoreCholesky_UserConfigSystemKey(t *testing.T) {
	// 换题 spd4_min4：解全 1；换题后按默认题答案 (1,2,3,4) 提交 → 低分
	score, passed, _ := submitCholesky(t, `{"solution":[1,1,1,1]}`, `{"system_key":"spd4_min4"}`)
	if score != 100 || !passed {
		t.Fatalf("换题 spd4_min4: want 100, got %d/%v", score, passed)
	}
	score, _, _ = submitCholesky(t, `{"solution":[1,2,3,4]}`, `{"system_key":"spd4_min4"}`)
	if score >= 60 {
		t.Fatalf("换题后旧答案应不过线: got %d", score)
	}
}

func TestScoreCholesky_UserConfigCustom(t *testing.T) {
	// 自定义 2 阶 A=[[1,2],[2,5]], b=(3,7)：
	// L=[[1,0],[2,1]]，y=(3,1)，解恰 (1,1)
	score, passed, _ := submitCholesky(t,
		`{"solution":[1,1],"L":[[1,0],[2,1]],"y":[3,1]}`,
		`{"A":[[1,2],[2,5]],"b":[3,7]}`)
	if score != 100 || !passed {
		t.Fatalf("自定义 2 阶: want 100, got %d/%v", score, passed)
	}
}

func TestScoreCholesky_NonSPDIsError(t *testing.T) {
	// 不定阵（特征值 3,-1）：第 2 步根号内=1-4=-3 ≤0 → 400
	_, _, _, err := scoreByExperiment("cholesky_solve",
		[]byte(`[{"solution":[1,1]}]`), []byte(choleskyTarget),
		[]byte(`{"A":[[1,2],[2,1]],"b":[3,3]}`))
	if err == nil {
		t.Fatal("不定阵应报错（根号内≤0）")
	}
	// 半正定奇异阵 [[1,1],[1,1]]：根号内=0 → 400
	_, _, _, err = scoreByExperiment("cholesky_solve",
		[]byte(`[{"solution":[1,1]}]`), []byte(choleskyTarget),
		[]byte(`{"A":[[1,1],[1,1]],"b":[2,2]}`))
	if err == nil {
		t.Fatal("半正定奇异阵应报错")
	}
	// 非对称阵：直接 400（平方根法只适用于对称阵）
	_, _, _, err = scoreByExperiment("cholesky_solve",
		[]byte(`[{"solution":[1,1]}]`), []byte(choleskyTarget),
		[]byte(`{"A":[[4,2],[3,5]],"b":[6,8]}`))
	if err == nil {
		t.Fatal("非对称阵应报错")
	}
}

func TestScoreCholesky_BadInput(t *testing.T) {
	cases := []struct {
		readings   string
		userConfig string
	}{
		{`{}`, ``},                                                     // 缺 solution
		{`{"solution":[1,2]}`, ``},                                     // 长度错（须 3）
		{`{"solution":[1,2,NaN]}`, ``},                                 // NaN
		{`{"solution":[1,2,3],"L":[[1,0,0],[1,1,0]]}`, ``},             // L 行数错（须 3）
		{`{"solution":[1,2,3],"L":[[1,0,0],[1,1]]}`, ``},               // L 行长错
		{`{"solution":[1,2,3],"y":[6,5]}`, ``},                         // y 长度错（须 3）
		{`{"solution":[1,2,3]}`, `{"system_key":"no_such"}`},           // 未知方程组
		{`{"solution":[1,2]}`, `{"A":[[4,2],[2,5]]}`},                  // 自定义缺 b
		{`{"solution":[1]}`, `{"A":[[1]],"b":[1]}`},                    // 1 阶（下限 2）
		{`{"solution":[1,2,3]}`, `{"A":[[1,2],[2,5]],"b":[3,7]}`},      // solution 长度≠自定义阶数
	}
	for i, c := range cases {
		uc := []byte(nil)
		if c.userConfig != "" {
			uc = []byte(c.userConfig)
		}
		_, _, _, err := scoreByExperiment("cholesky_solve", []byte("["+c.readings+"]"), []byte(choleskyTarget), uc)
		if err == nil {
			t.Fatalf("case %d 应报错: readings=%s uc=%s", i, c.readings, c.userConfig)
		}
	}
	// 6 阶超上限
	big := `{"A":[[2,1,0,0,0,0],[1,2,1,0,0,0],[0,1,2,1,0,0],[0,0,1,2,1,0],[0,0,0,1,2,1],[0,0,0,0,1,2]],"b":[1,1,1,1,1,1]}`
	_, _, _, err := scoreByExperiment("cholesky_solve", []byte(`[{"solution":[1,1,1,1,1,1]}]`),
		[]byte(choleskyTarget), []byte(big))
	if err == nil {
		t.Fatal("6 阶应报错")
	}
}

func TestScoreCholesky_TargetSystemKeyOnlyForTarget(t *testing.T) {
	// 未换题时按 target 的 spd2_min3 评分：外来答案（spd3 的解全 1）
	// 应不过线（target 生效、真值按目标题现算）
	score, _, _ := submitCholesky(t, `{"solution":[1,1,1]}`, ``)
	if score >= 60 {
		t.Fatalf("未换题按 spd2_min3 评分，外来答案应不过线: got %d", score)
	}
}

const rk4Target = `{"method":"rk4_rel_err","problem_key":"exp_growth","n":1,"pass_score":60}`

func submitRK4(t *testing.T, readings, userConfig string) (int, bool, string) {
	t.Helper()
	uc := []byte(nil)
	if userConfig != "" {
		uc = []byte(userConfig)
	}
	score, passed, detail, err := scoreByExperiment("runge_kutta_ode", []byte(readings), []byte(rk4Target), uc)
	if err != nil {
		t.Fatalf("readings=%s user_config=%s: %v", readings, userConfig, err)
	}
	return score, passed, detail
}

func rk4TrueY(key string, n int) float64 {
	def := odeProblems[key]
	return rk4Column(def.F, def.A, def.B, def.Y0, n)[n]
}

func TestScoreRK4_ClosedForms(t *testing.T) {
	// exp_growth 单步 n=1：K1..K4 = 1、1.5、1.75、2.75，终值恰 65/24 ≈ 2.7083333
	if got := rk4TrueY("exp_growth", 1); math.Abs(got-65.0/24.0) > 1e-15 {
		t.Fatalf("exp_growth n=1: want 65/24, got %.12g", got)
	}
	// decay_sq 单步 n=1：K1..K4 = 0、−1、−0.5、−1，终值恰 1/3
	if got := rk4TrueY("decay_sq", 1); math.Abs(got-1.0/3.0) > 1e-15 {
		t.Fatalf("decay_sq n=1: want 1/3, got %.12g", got)
	}
	// stiff_decay 单步 n=1：hλ=−10 远超绝对稳定区间，|R(−10)|=291>1 发散
	// （真值仍确定，且与 L20 显式的 5.0625、L21 改进的量级均不同）
	if got := rk4TrueY("stiff_decay", 1); math.Abs(got-291) > 1e-9 {
		t.Fatalf("stiff_decay n=1: want 291, got %.12g", got)
	}
	// 结构性恒等式：y'=y 的 RK4 对任意 a/b/n 都是 R(h)^n，
	// R(h)=1+h+h²/2+h³/6+h⁴/24（e 的 4 项泰勒截断）
	for _, c := range []struct {
		a, b float64
		n    int
	}{
		{0, 1, 1}, {0, 2, 5}, {-1, 1, 4}, {0.5, 1.25, 7},
	} {
		h := (c.b - c.a) / float64(c.n)
		want := math.Pow(1+h+h*h/2+math.Pow(h, 3)/6+math.Pow(h, 4)/24, float64(c.n))
		if got := rk4Column(odeProblems["exp_growth"].F, c.a, c.b, 1, c.n)[c.n]; math.Abs(got-want) > 1e-12 {
			t.Fatalf("exp_growth RK4 R(h)^n 恒等式破坏: a=%g b=%g n=%d: want %.12g got %.12g",
				c.a, c.b, c.n, want, got)
		}
	}
}

func TestScoreRK4_ExactSolution(t *testing.T) {
	// exp_growth 单步全列：y_col=[1, 65/24]
	readings := `[{"value":2.7083333333333335,"y_col":[1,2.7083333333333335]}]`
	score, passed, detail := submitRK4(t, readings, "")
	if score != 100 || !passed {
		t.Fatalf("want 100/passed, got %d/%v (detail=%s)", score, passed, detail)
	}
	if !strings.Contains(detail, "problem=exp_growth") || !strings.Contains(detail, "n=1") {
		t.Errorf("detail 应记录 problem/n: %s", detail)
	}
	if !strings.Contains(detail, "ref_Y=2.71828") {
		t.Errorf("detail 应记录精确解 ref_Y: %s", detail)
	}
	if !strings.Contains(detail, "y_col=[1,") {
		t.Errorf("detail 应记录 y_col: %s", detail)
	}
	if !strings.Contains(detail, "RK4") {
		t.Errorf("detail 应标注方法（RK4）: %s", detail)
	}
}

func TestScoreRK4_RelErrCurve(t *testing.T) {
	y := rk4TrueY("exp_growth", 1)
	for _, c := range []struct {
		relErr float64
		want   int
	}{
		{1e-7, 100},
		{1e-4, 90},
		{1e-3, 80},
		{1e-2, 65},
		{0.1, 30},
		{0.5, 0},
	} {
		readings := fmt.Sprintf(`[{"value":%.12g}]`, y*(1+c.relErr))
		if score, _, _ := submitRK4(t, readings, ""); score != c.want {
			t.Errorf("rel_err=%g: want %d, got %d", c.relErr, c.want, score)
		}
	}
}

func TestScoreRK4_Traps(t *testing.T) {
	// 抄精确解 e：与单步真值 65/24 差 ~0.37% -> 约 72 分——RK4 收敛快，
	// 能过线但非满分（L19 龙贝格同款语义：过程真伪由 y_col 审计暴露）
	score, passed, _ := submitRK4(t, `[{"value":2.7182818285}]`, "")
	if !passed || score < 60 || score >= 90 {
		t.Fatalf("抄精确解: want 60~89/passed（约 72）, got %d/%v", score, passed)
	}
	// 抄 L21 改进欧拉答案 2.640625（低一阶方法）：差 ~2.5% -> ~51 分不过线
	if score, passed, _ := submitRK4(t, `[{"value":2.640625}]`, ""); passed || score >= 60 {
		t.Fatalf("抄改进欧拉答案: want <60/not passed, got %d/%v", score, passed)
	}
	// 抄 L20 显式欧拉答案 2.44140625（低两阶方法）：差 ~9.9% -> ~30 分不过线
	if score, passed, _ := submitRK4(t, `[{"value":2.44140625}]`, ""); passed || score >= 60 {
		t.Fatalf("抄显式欧拉答案: want <60/not passed, got %d/%v", score, passed)
	}
}

func TestScoreRK4_StiffDivergence(t *testing.T) {
	// 刚性观察题：hλ=−10 远超 RK4 绝对稳定区间（|R(−10)|=291>1），
	// 单步即发散到 291——"真值=方法输出"仍确定，老实递推照样 100
	//（换题须显式带 n=1：不带则回落库默认 n=4）
	readings := `[{"value":291,"y_col":[1,291]}]`
	if score, passed, detail := submitRK4(t, readings, `{"problem_key":"stiff_decay","n":1}`); score != 100 || !passed {
		t.Fatalf("stiff 正确递推（发散值仍确定）: want 100/passed, got %d/%v (detail=%s)", score, passed, detail)
	}
	// 抄精确解 e^{-10}：与发散值差 ~100% -> 0 分
	if score, passed, _ := submitRK4(t, fmt.Sprintf(`[{"value":%.12g}]`, math.Exp(-10)), `{"problem_key":"stiff_decay","n":1}`); passed || score > 0 {
		t.Fatalf("stiff 抄精确解: want 0/not passed, got %d/%v", score, passed)
	}
	// 抄 L20/L21 的发散答案也拦（RK4 发散得更快，数值差异大）
	if score, passed, _ := submitRK4(t, `[{"value":5.0625}]`, `{"problem_key":"stiff_decay","n":1}`); passed || score > 0 {
		t.Fatalf("stiff 抄显式欧拉答案: want 0/not passed, got %d/%v", score, passed)
	}
}

func TestScoreRK4_UserConfigProblemKey(t *testing.T) {
	// 换题 decay_sq（换题后 n 回落库默认 4，非 target 的 1）
	if score, _, detail := submitRK4(t, fmt.Sprintf(`[{"value":%.12g}]`, rk4TrueY("decay_sq", 4)), `{"problem_key":"decay_sq"}`); score != 100 {
		t.Fatalf("decay_sq 换题 n=4: want 100, got %d (detail=%s)", score, detail)
	}
	// 换题且显式 n=1：真值 1/3，抄精确解 e^{-1} 差 ~10% -> ~29 分不过线
	if score, passed, _ := submitRK4(t, fmt.Sprintf(`[{"value":%.12g}]`, math.Exp(-1)), `{"problem_key":"decay_sq","n":1}`); passed || score >= 60 {
		t.Fatalf("decay_sq n=1 抄精确解: want <60/not passed, got %d/%v", score, passed)
	}
}

func TestScoreRK4_UserConfigN(t *testing.T) {
	// n=2：每步因子 R(0.5)=1.6484375，终值恰 1.6484375² = 2.71734619140625
	if got := rk4TrueY("exp_growth", 2); math.Abs(got-2.71734619140625) > 1e-12 {
		t.Fatalf("exp_growth n=2 锚点: got %.12g", got)
	}
	if score, _, _ := submitRK4(t, fmt.Sprintf(`[{"value":%.12g}]`, rk4TrueY("exp_growth", 2)), `{"n":2}`); score != 100 {
		t.Fatalf("n=2 正确值: want 100, got %d", score)
	}
}

func TestScoreRK4_UserConfigInterval(t *testing.T) {
	// 换区间 [0,2] 且换 n=2：h=1，单步因子 R(1)=65/24，终值 (65/24)²
	score, _, detail := submitRK4(t,
		fmt.Sprintf(`[{"value":%.12g}]`, math.Pow(65.0/24.0, 2)),
		`{"a":0,"b":2,"n":2}`)
	if score != 100 {
		t.Fatalf("换区间换 n 正确值: want 100, got %d", score)
	}
	if !strings.Contains(detail, "a=0") || !strings.Contains(detail, "b=2") {
		t.Errorf("detail 应记录换后区间: %s", detail)
	}
}

func TestScoreRK4_YColAudit(t *testing.T) {
	// y_col 长度错（n=1 须 2 个）直接 400 暴露
	if _, _, _, err := scoreByExperiment("runge_kutta_ode",
		[]byte(`[{"value":2.7083333333333335,"y_col":[1,2.7,3]}]`), []byte(rk4Target), nil); err == nil {
		t.Fatal("y_col 长度错应报错")
	}
}

func TestScoreRK4_BadInput(t *testing.T) {
	if _, _, _, err := scoreByExperiment("runge_kutta_ode", []byte(`[{}]`), []byte(rk4Target), nil); err == nil {
		t.Error("缺 value 应报错")
	}
	if _, _, _, err := scoreByExperiment("runge_kutta_ode", []byte(`[{"value":"x"}]`), []byte(rk4Target), nil); err == nil {
		t.Error("value 非数值应报错")
	}
	if _, _, _, err := scoreByExperiment("runge_kutta_ode", []byte(`[{"value":2}]`), []byte(rk4Target), []byte(`{"n":0}`)); err == nil {
		t.Error("n=0 应报错")
	}
	if _, _, _, err := scoreByExperiment("runge_kutta_ode", []byte(`[{"value":2}]`), []byte(rk4Target), []byte(`{"n":1025}`)); err == nil {
		t.Error("n=1025 应报错")
	}
	if _, _, _, err := scoreByExperiment("runge_kutta_ode", []byte(`[{"value":2}]`), []byte(rk4Target), []byte(`{"a":0}`)); err == nil {
		t.Error("只给 a 应报错")
	}
	if _, _, _, err := scoreByExperiment("runge_kutta_ode", []byte(`[{"value":2}]`), []byte(rk4Target), []byte(`{"a":1,"b":0}`)); err == nil {
		t.Error("a>=b 应报错")
	}
	if _, _, _, err := scoreByExperiment("runge_kutta_ode", []byte(`[{"value":2}]`), []byte(rk4Target), []byte(`{"problem_key":"nope"}`)); err == nil {
		t.Error("未知题应报错")
	}
	if _, _, _, err := scoreByExperiment("runge_kutta_ode", []byte(`[{"value":NaN}]`), []byte(rk4Target), nil); err == nil {
		t.Error("NaN value 应报错")
	}
}

func TestScoreRK4_TargetParamsOnlyForTargetProblem(t *testing.T) {
	// target 带 exp_growth 的 n=2；user_config 换 decay_sq 不带 n 时应用库内
	// 默认 n=4，而非沿用 target 的 2（target 的题专属参数只属于 target 自己的题）
	tgt := `{"method":"rk4_rel_err","problem_key":"exp_growth","n":2,"pass_score":60}`
	score, _, _, err := scoreByExperiment("runge_kutta_ode",
		[]byte(fmt.Sprintf(`[{"value":%.12g}]`, rk4TrueY("decay_sq", 4))), []byte(tgt), []byte(`{"problem_key":"decay_sq"}`))
	if err != nil {
		t.Fatal(err)
	}
	if score != 100 {
		t.Fatalf("换题不带 n 应用库内默认 n=4: want 100, got %d", score)
	}
	// 换题后若错误沿用 target 的 n=2（bug 场景），按 n=2 提交会对不上 n=4 的真值
	d2 := rk4TrueY("decay_sq", 2)
	score, _, _, err = scoreByExperiment("runge_kutta_ode",
		[]byte(fmt.Sprintf(`[{"value":%.12g}]`, d2)), []byte(tgt), []byte(`{"problem_key":"decay_sq"}`))
	if err != nil {
		t.Fatal(err)
	}
	if score >= 100 {
		t.Fatalf("按 n=2 提交应暴露不符（truth 是 n=4）: got %d", score)
	}
	// 未换题：target 的 n=2 生效
	score, _, _, err = scoreByExperiment("runge_kutta_ode",
		[]byte(fmt.Sprintf(`[{"value":%.12g}]`, rk4TrueY("exp_growth", 2))), []byte(tgt), nil)
	if err != nil {
		t.Fatal(err)
	}
	if score != 100 {
		t.Fatalf("未换题时 target 的 n=2 应生效: want 100, got %d", score)
	}
}

const adamsTarget = `{"method":"adams_rel_err","problem_key":"exp_growth","n":4,"pass_score":60}`

func submitAdams(t *testing.T, readings, userConfig string) (int, bool, string) {
	t.Helper()
	uc := []byte(nil)
	if userConfig != "" {
		uc = []byte(userConfig)
	}
	score, passed, detail, err := scoreByExperiment("adams_pc_ode", []byte(readings), []byte(adamsTarget), uc)
	if err != nil {
		t.Fatalf("readings=%s user_config=%s: %v", readings, userConfig, err)
	}
	return score, passed, detail
}

func adamsTrueY(key string, n int) float64 {
	def := odeProblems[key]
	return adamsPCColumn(def.F, def.A, def.B, def.Y0, n)[n]
}

func TestScoreAdams_ClosedForms(t *testing.T) {
	// 结构性交叉验证（不调 adamsPCColumn 的独立公式重算）：
	// y'=y、h=0.25 时表头 y_1..y_3 = R^i（R=1+h+h²/2+h³/6+h⁴/24，RK4 单步因子），
	// 第 4 步 AB4 预报 + AM4 校正一次：
	//   y4p = y3 + h/24·(55y3 − 59y2 + 37y1 − 9y0)（f=y 时斜率即函数值）
	//   y4  = y3 + h/24·(9y4p + 19y3 − 5y2 + y1)
	const h = 0.25
	R := 1 + h + h*h/2 + math.Pow(h, 3)/6 + math.Pow(h, 4)/24
	y0, y1, y2, y3 := 1.0, R, R*R, R*R*R
	y4p := y3 + h/24*(55*y3-59*y2+37*y1-9*y0)
	y4 := y3 + h/24*(9*y4p+19*y3-5*y2+y1)
	if got := adamsTrueY("exp_growth", 4); math.Abs(got-y4) > 1e-14 {
		t.Fatalf("exp_growth n=4 闭式: want %.12g, got %.12g", y4, got)
	}
	// 表头段应恰为 RK4 列（同 h=0.25 的 rk4Column n=4 前 3 步）
	rk := rk4Column(odeProblems["exp_growth"].F, 0, 1, 1, 4)
	ad := adamsPCColumn(odeProblems["exp_growth"].F, 0, 1, 1, 4)
	for i := 1; i <= 3; i++ {
		if math.Abs(ad[i]-rk[i]) > 1e-15 {
			t.Fatalf("表头步 %d 应与 RK4 同式: rk=%.12g adams=%.12g", i, rk[i], ad[i])
		}
	}
	// decay_sq / stiff_decay n=4 锚点（node 对拍值）
	if got := adamsTrueY("decay_sq", 4); math.Abs(got-0.36547601036542743) > 1e-14 {
		t.Fatalf("decay_sq n=4 锚点: got %.12g", got)
	}
	if got := adamsTrueY("stiff_decay", 4); math.Abs(got-0.13474098779261112) > 1e-14 {
		t.Fatalf("stiff_decay n=4 锚点: got %.12g", got)
	}
}

func TestScoreAdams_ExactSolution(t *testing.T) {
	// exp_growth n=4 全列：表头 3 步 RK4 + 1 步 AB4-AM4
	readings := `[{"value":2.718225983274504,"y_col":[1,1.2840169270833333,1.648699469036526,2.1169580259162033,2.718225983274504]}]`
	score, passed, detail := submitAdams(t, readings, "")
	if score != 100 || !passed {
		t.Fatalf("want 100/passed, got %d/%v (detail=%s)", score, passed, detail)
	}
	if !strings.Contains(detail, "problem=exp_growth") || !strings.Contains(detail, "n=4") {
		t.Errorf("detail 应记录 problem/n: %s", detail)
	}
	if !strings.Contains(detail, "ref_Y=2.71828") {
		t.Errorf("detail 应记录精确解 ref_Y: %s", detail)
	}
	if !strings.Contains(detail, "y_col=[1,") {
		t.Errorf("detail 应记录 y_col: %s", detail)
	}
	if !strings.Contains(detail, "阿当姆斯") || !strings.Contains(detail, "RK4 表头") {
		t.Errorf("detail 应标注方法（阿当姆斯 AB4-AM4，RK4 表头）: %s", detail)
	}
}

func TestScoreAdams_RelErrCurve(t *testing.T) {
	y := adamsTrueY("exp_growth", 4)
	for _, c := range []struct {
		relErr float64
		want   int
	}{
		{1e-7, 100},
		{1e-4, 90},
		{1e-3, 80},
		{1e-2, 65},
		{0.1, 30},
		{0.5, 0},
	} {
		readings := fmt.Sprintf(`[{"value":%.12g}]`, y*(1+c.relErr))
		if score, _, _ := submitAdams(t, readings, ""); score != c.want {
			t.Errorf("rel_err=%g: want %d, got %d", c.relErr, c.want, score)
		}
	}
}

func TestScoreAdams_Traps(t *testing.T) {
	// 抄精确解 e：与真值 2.7182259833 差 ~2e-5 -> 约 93 分——四阶法收敛快，
	// 能过线但非满分（L19/L22 同款语义：过程真伪由 y_col 审计暴露）
	score, passed, _ := submitAdams(t, `[{"value":2.7182818285}]`, "")
	if !passed || score < 90 || score >= 100 {
		t.Fatalf("抄精确解: want 90~99/passed（约 93）, got %d/%v", score, passed)
	}
	// 抄 L22 默认答案 65/24（同为四阶法，n=1 单步值）：差 ~0.36% -> 约 72 分，
	// 过线但非满分（四阶方法互相抄数学上难分，靠 y_col 审计区分）
	if score, passed, _ := submitAdams(t, `[{"value":2.7083333333333335}]`, ""); !passed || score < 60 || score >= 90 {
		t.Fatalf("抄 L22 默认答案: want 60~89/passed（约 72）, got %d/%v", score, passed)
	}
	// 抄 L21 改进欧拉答案 2.640625：差 ~2.9% -> ~49 分不过线
	if score, passed, _ := submitAdams(t, `[{"value":2.640625}]`, ""); passed || score >= 60 {
		t.Fatalf("抄改进欧拉答案: want <60/not passed, got %d/%v", score, passed)
	}
	// 抄 L20 显式欧拉答案 2.44140625：差 ~10% -> ~30 分不过线
	if score, passed, _ := submitAdams(t, `[{"value":2.44140625}]`, ""); passed || score >= 60 {
		t.Fatalf("抄显式欧拉答案: want <60/not passed, got %d/%v", score, passed)
	}
}

func TestScoreAdams_StiffStability(t *testing.T) {
	// 刚性观察题：hλ=−2.5 时 RK4 表头尚稳定（|R(−2.5)|=0.648<1），但 AB4-AM4
	// 多步法绝对稳定区间远小于 RK4，第 4 步起已开始偏离衰减轨道——
	// "真值=方法输出"仍确定（n=4 真值 0.1347409878），老实递推照样 100
	//（换题不带 n 时回落库默认 n=4，恰与本题吻合）
	readings := `[{"value":0.13474098779261112,"y_col":[1,0.6484375,0.42047119140625,0.27264928817749023,0.13474098779261112]}]`
	if score, passed, detail := submitAdams(t, readings, `{"problem_key":"stiff_decay"}`); score != 100 || !passed {
		t.Fatalf("stiff 正确递推: want 100/passed, got %d/%v (detail=%s)", score, passed, detail)
	}
	// 抄精确解 e^{-10}：与真值差 ~100% -> 0 分
	if score, passed, _ := submitAdams(t, fmt.Sprintf(`[{"value":%.12g}]`, math.Exp(-10)), `{"problem_key":"stiff_decay"}`); passed || score > 0 {
		t.Fatalf("stiff 抄精确解: want 0/not passed, got %d/%v", score, passed)
	}
	// 抄 RK4 n=4 的答案 0.1768（只做表头漏做多步法）：差 ~31% -> ~9 分不过线
	if score, passed, _ := submitAdams(t, `[{"value":0.1767960228}]`, `{"problem_key":"stiff_decay"}`); passed || score >= 60 {
		t.Fatalf("stiff 抄 RK4 表头答案: want <60/not passed, got %d/%v", score, passed)
	}
}

func TestScoreAdams_UserConfigProblemKey(t *testing.T) {
	// 换题 decay_sq（换题后 n 回落库默认 4）
	if score, _, detail := submitAdams(t, fmt.Sprintf(`[{"value":%.12g}]`, adamsTrueY("decay_sq", 4)), `{"problem_key":"decay_sq"}`); score != 100 {
		t.Fatalf("decay_sq 换题 n=4: want 100, got %d (detail=%s)", score, detail)
	}
	// 换题抄精确解 e^{-1}：差 ~0.66% -> 约 68 分，过线但非满分（审计暴露）
	if score, passed, _ := submitAdams(t, fmt.Sprintf(`[{"value":%.12g}]`, math.Exp(-1)), `{"problem_key":"decay_sq"}`); !passed || score < 60 || score >= 100 {
		t.Fatalf("decay_sq 抄精确解: want 60~99/passed（约 68）, got %d/%v", score, passed)
	}
}

func TestScoreAdams_UserConfigN(t *testing.T) {
	// n=8：3 步表头 + 5 步 AB4-AM4，锚点（node 对拍值）
	if got := adamsTrueY("exp_growth", 8); math.Abs(got-2.7182840853162915) > 1e-14 {
		t.Fatalf("exp_growth n=8 锚点: got %.12g", got)
	}
	if score, _, _ := submitAdams(t, fmt.Sprintf(`[{"value":%.12g}]`, adamsTrueY("exp_growth", 8)), `{"n":8}`); score != 100 {
		t.Fatalf("n=8 正确值: want 100, got %d", score)
	}
}

func TestScoreAdams_UserConfigInterval(t *testing.T) {
	// 换区间 [0,2] n=4：h=0.5，锚点 7.382826779658595（node 对拍值）
	score, _, detail := submitAdams(t,
		fmt.Sprintf(`[{"value":%.12g}]`, 7.382826779658595),
		`{"a":0,"b":2,"n":4}`)
	if score != 100 {
		t.Fatalf("换区间正确值: want 100, got %d", score)
	}
	if !strings.Contains(detail, "a=0") || !strings.Contains(detail, "b=2") {
		t.Errorf("detail 应记录换后区间: %s", detail)
	}
}

func TestScoreAdams_YColAudit(t *testing.T) {
	// y_col 长度错（n=4 须 5 个）直接 400 暴露
	if _, _, _, err := scoreByExperiment("adams_pc_ode",
		[]byte(`[{"value":2.7182259833,"y_col":[1,2.7]}]`), []byte(adamsTarget), nil); err == nil {
		t.Fatal("y_col 长度错应报错")
	}
}

func TestScoreAdams_BadInput(t *testing.T) {
	if _, _, _, err := scoreByExperiment("adams_pc_ode", []byte(`[{}]`), []byte(adamsTarget), nil); err == nil {
		t.Error("缺 value 应报错")
	}
	if _, _, _, err := scoreByExperiment("adams_pc_ode", []byte(`[{"value":"x"}]`), []byte(adamsTarget), nil); err == nil {
		t.Error("value 非数值应报错")
	}
	// n=3 < 4：多步法需 3 步表头 + 至少 1 步阿当姆斯，n 不足直接拦
	if _, _, _, err := scoreByExperiment("adams_pc_ode", []byte(`[{"value":2}]`), []byte(adamsTarget), []byte(`{"n":3}`)); err == nil {
		t.Error("n=3（不足表头+1 步）应报错")
	}
	if _, _, _, err := scoreByExperiment("adams_pc_ode", []byte(`[{"value":2}]`), []byte(adamsTarget), []byte(`{"n":1025}`)); err == nil {
		t.Error("n=1025 应报错")
	}
	if _, _, _, err := scoreByExperiment("adams_pc_ode", []byte(`[{"value":2}]`), []byte(adamsTarget), []byte(`{"a":0}`)); err == nil {
		t.Error("只给 a 应报错")
	}
	if _, _, _, err := scoreByExperiment("adams_pc_ode", []byte(`[{"value":2}]`), []byte(adamsTarget), []byte(`{"a":1,"b":0}`)); err == nil {
		t.Error("a>=b 应报错")
	}
	if _, _, _, err := scoreByExperiment("adams_pc_ode", []byte(`[{"value":2}]`), []byte(adamsTarget), []byte(`{"problem_key":"nope"}`)); err == nil {
		t.Error("未知题应报错")
	}
	if _, _, _, err := scoreByExperiment("adams_pc_ode", []byte(`[{"value":NaN}]`), []byte(adamsTarget), nil); err == nil {
		t.Error("NaN value 应报错")
	}
}

func TestScoreAdams_TargetParamsOnlyForTargetProblem(t *testing.T) {
	// target 带 exp_growth 的 n=8；user_config 换 decay_sq 不带 n 时应用库内
	// 默认 n=4，而非沿用 target 的 8（target 的题专属参数只属于 target 自己的题）
	tgt := `{"method":"adams_rel_err","problem_key":"exp_growth","n":8,"pass_score":60}`
	score, _, _, err := scoreByExperiment("adams_pc_ode",
		[]byte(fmt.Sprintf(`[{"value":%.12g}]`, adamsTrueY("decay_sq", 4))), []byte(tgt), []byte(`{"problem_key":"decay_sq"}`))
	if err != nil {
		t.Fatal(err)
	}
	if score != 100 {
		t.Fatalf("换题不带 n 应用库内默认 n=4: want 100, got %d", score)
	}
	// 换题后若错误沿用 target 的 n=8（bug 场景），按 n=8 提交会对不上 n=4 的真值
	d8 := adamsTrueY("decay_sq", 8)
	score, _, _, err = scoreByExperiment("adams_pc_ode",
		[]byte(fmt.Sprintf(`[{"value":%.12g}]`, d8)), []byte(tgt), []byte(`{"problem_key":"decay_sq"}`))
	if err != nil {
		t.Fatal(err)
	}
	if score >= 100 {
		t.Fatalf("按 n=8 提交应暴露不符（truth 是 n=4）: got %d", score)
	}
	// 未换题：target 的 n=8 生效
	score, _, _, err = scoreByExperiment("adams_pc_ode",
		[]byte(fmt.Sprintf(`[{"value":%.12g}]`, adamsTrueY("exp_growth", 8))), []byte(tgt), nil)
	if err != nil {
		t.Fatal(err)
	}
	if score != 100 {
		t.Fatalf("未换题时 target 的 n=8 应生效: want 100, got %d", score)
	}
}

// ---------- L24 最小二乘拟合 ----------

const lsqTarget = `{"method":"lsq_rel_err","problem_key":"lin_line","m":1,"x_eval":3.5,"pass_score":60}`

func submitLSQ(t *testing.T, readings, userConfig string) (int, bool, string) {
	t.Helper()
	score, passed, detail, err := scoreByExperiment("least_squares_fit",
		[]byte(readings), []byte(lsqTarget), []byte(userConfig))
	if err != nil {
		t.Fatal(err)
	}
	return score, passed, detail
}

// lsqTrueP 库内题真值 P(x_eval)（ln 题回原域），与评分器同一计算路径，
// 单测里用作锚点基准。
func lsqTrueP(key string) float64 {
	def := fitProblems[key]
	cs, err := lsqFit(def.Xs, def.Ys, def.M, def.Ln)
	if err != nil {
		panic(err)
	}
	v := lsqEval(cs, def.XEval)
	if def.Ln {
		v = math.Exp(v)
	}
	return v
}

func TestScoreLSQ_ClosedForms(t *testing.T) {
	// lin_line：法方程 4a+10b=14、10a+30b=39.8 手算 a=1.1、b=0.96，P(3.5)=4.46
	cs, err := lsqFit([]float64{1, 2, 3, 4}, []float64{2, 3.1, 4, 4.9}, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(cs[0]-1.1) > 1e-9 || math.Abs(cs[1]-0.96) > 1e-9 {
		t.Fatalf("直线拟合系数应恰为 1.1/0.96: got %v", cs)
	}
	if p := lsqEval(cs, 3.5); math.Abs(p-4.46) > 1e-9 {
		t.Fatalf("P(3.5) 应恰为 4.46: got %.12g", p)
	}
	// quad_parabola：系数恰 27/350、1/28、137/140，P(2)=711/175
	def := fitProblems["quad_parabola"]
	cs, err = lsqFit(def.Xs, def.Ys, 2, false)
	if err != nil {
		t.Fatal(err)
	}
	want := []float64{27.0 / 350, 1.0 / 28, 137.0 / 140}
	for i := range want {
		if math.Abs(cs[i]-want[i]) > 1e-9 {
			t.Fatalf("二次拟合系数[%d] 应为 %.12g: got %.12g", i, want[i], cs[i])
		}
	}
	if p := lsqEval(cs, 2); math.Abs(p-711.0/175) > 1e-9 {
		t.Fatalf("P(2) 应恰为 711/175=4.0628571...: got %.12g", p)
	}
	// exp_lnw：ln 线性化后 2×2 法方程闭式（与幂基路径独立）交叉验证
	// B=(nΣxY−ΣxΣY)/(nΣx²−(Σx)²)、A=(ΣY−BΣx)/n，真值=e^{A+2.5B}
	ex := fitProblems["exp_lnw"]
	n := len(ex.Xs)
	sumX, sumX2, sumY, sumXY := 0.0, 0.0, 0.0, 0.0
	for i := 0; i < n; i++ {
		Y := math.Log(ex.Ys[i])
		sumX += ex.Xs[i]
		sumX2 += ex.Xs[i] * ex.Xs[i]
		sumY += Y
		sumXY += ex.Xs[i] * Y
	}
	B := (float64(n)*sumXY - sumX*sumY) / (float64(n)*sumX2 - sumX*sumX)
	A := (sumY - B*sumX) / float64(n)
	wantExp := math.Exp(A + 2.5*B)
	gotExp := lsqTrueP("exp_lnw")
	if math.Abs(wantExp-gotExp) > 1e-9*math.Max(1, math.Abs(wantExp)) {
		t.Fatalf("指数题真值: 闭式 %.12g vs 幂基路径 %.12g", wantExp, gotExp)
	}
	if math.Abs(gotExp-6.910393061532121) > 1e-9 {
		t.Fatalf("指数题真值锚点 6.910393061532121: got %.12g", gotExp)
	}
}

func TestScoreLSQ_OrthogonalProjection(t *testing.T) {
	// 法方程解 = 残差对幂基正交：Σ r_k·x_k^p ≈ 0（p=0..m），
	// 各题与换 m（lin 的 m=0/1/2）都须成立，验证法方程组装与求解一致。
	cases := []struct {
		key string
		m   int
	}{
		{"lin_line", 0}, {"lin_line", 1}, {"lin_line", 2},
		{"quad_parabola", 2}, {"exp_lnw", 1},
	}
	for _, c := range cases {
		def := fitProblems[c.key]
		cs, err := lsqFit(def.Xs, def.Ys, c.m, def.Ln)
		if err != nil {
			t.Fatalf("%s m=%d: %v", c.key, c.m, err)
		}
		for p := 0; p <= c.m; p++ {
			s := 0.0
			for k := range def.Xs {
				y := def.Ys[k]
				if def.Ln {
					y = math.Log(y)
				}
				s += (y - lsqEval(cs, def.Xs[k])) * math.Pow(def.Xs[k], float64(p))
			}
			if math.Abs(s) > 1e-9 {
				t.Fatalf("%s m=%d: 残差应与 x^%d 正交（Σr·x^p=%.3g 非零）", c.key, c.m, p, s)
			}
		}
	}
}

func TestScoreLSQ_ExactSolution(t *testing.T) {
	score, passed, detail := submitLSQ(t,
		`[{"value":4.46,"coef":[1.1,0.96]}]`, `{}`)
	if score != 100 || !passed {
		t.Fatalf("正确拟合值+系数: want 100/passed, got %d/%v", score, passed)
	}
	for _, frag := range []string{"problem=lin_line", "pts=4", "m=1", "x_eval=3.5",
		"true_P=4.46", "最小二乘法方程拟合", "coef=[1.1,0.96]"} {
		if !strings.Contains(detail, frag) {
			t.Errorf("detail 应含 %q: %s", frag, detail)
		}
	}
}

func TestScoreLSQ_RelErrCurve(t *testing.T) {
	// 与其它数值实验共用的默认曲线锚点（truth=4.46）
	for _, tc := range []struct {
		f     float64
		score int
	}{
		{1.001, 80}, {1.01, 65}, {1.1, 30}, {1.5, 0},
	} {
		score, _, _ := submitLSQ(t, fmt.Sprintf(`[{"value":%.12g}]`, 4.46*tc.f), `{}`)
		if score != tc.score {
			t.Errorf("truth*%v: want %d, got %d", tc.f, tc.score, score)
		}
	}
}

func TestScoreLSQ_TrapsCopyObservation(t *testing.T) {
	// 抄观测值/均值：x_eval=3.5 处拟合线"靠近但不过点"，残差把分数压在线下
	for _, tc := range []struct {
		v     float64
		label string
	}{
		{4, "抄观测 y(3)"}, {4.9, "抄观测 y(4)"}, {3.5, "抄均值 ȳ"}, {3.1, "抄观测 y(2)"},
	} {
		score, passed, _ := submitLSQ(t, fmt.Sprintf(`[{"value":%g}]`, tc.v), `{}`)
		if passed || score >= 60 {
			t.Errorf("%s=%g 应不过线: got %d", tc.label, tc.v, score)
		}
	}
	// 抄三次插值 P3(3.5)=4.4375：插值≠拟合，能过线但非满分（同 RK4 抄 e 语义）
	score, passed, _ := submitLSQ(t, `[{"value":4.4375}]`, `{}`)
	if !passed || score < 60 || score >= 100 {
		t.Errorf("抄插值 4.4375: want 过线不满分（约 69）, got %d/%v", score, passed)
	}
}

func TestScoreLSQ_TrapQuad(t *testing.T) {
	// quad_parabola 默认 m=2、x_eval=2（数据点上）：LSQ 不过点
	correct := fmt.Sprintf(`[{"value":%.12g}]`, lsqTrueP("quad_parabola"))
	score, passed, _ := submitLSQ(t, correct, `{"problem_key":"quad_parabola"}`)
	if score != 100 || !passed {
		t.Fatalf("二次拟合正确值: want 100, got %d", score)
	}
	for _, tc := range []struct {
		v     float64
		label string
	}{
		{3.9, "抄观测 y(2)"}, {4, "抄真函数 x²"}, {6.02, "次数用错（直线拟合值）"},
	} {
		score, passed, _ := submitLSQ(t, fmt.Sprintf(`[{"value":%g}]`, tc.v), `{"problem_key":"quad_parabola"}`)
		if passed || score >= 60 {
			t.Errorf("%s=%g 应不过线: got %d", tc.label, tc.v, score)
		}
	}
}

func TestScoreLSQ_TrapExpLinearize(t *testing.T) {
	// exp_lnw：ln 线性化是必经步骤，跳过它直接对 y 做直线拟合也对不上
	uc := `{"problem_key":"exp_lnw"}`
	score, passed, _ := submitLSQ(t, fmt.Sprintf(`[{"value":%.12g}]`, lsqTrueP("exp_lnw")), uc)
	if score != 100 || !passed {
		t.Fatalf("指数题正确值: want 100, got %d", score)
	}
	// 不线性化直接直线拟合 y 得 8.765（node 对拍值），与真值差 ~27%
	def := fitProblems["exp_lnw"]
	linCs, err := lsqFit(def.Xs, def.Ys, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	noLn := lsqEval(linCs, 2.5)
	if math.Abs(noLn-8.765) > 1e-9 {
		t.Fatalf("不线性化直线拟合值锚点 8.765: got %.12g", noLn)
	}
	for _, tc := range []struct {
		v     float64
		label string
	}{
		{5.2, "抄观测 y(2)"}, {9, "抄观测 y(3)"}, {16.2, "抄观测 y(4)"}, {noLn, "不线性化直接拟 y"},
	} {
		score, passed, _ := submitLSQ(t, fmt.Sprintf(`[{"value":%.12g}]`, tc.v), uc)
		if passed || score >= 60 {
			t.Errorf("%s=%.6g 应不过线: got %d", tc.label, tc.v, score)
		}
	}
	// detail 应带指数模型标注
	_, _, detail := submitLSQ(t, fmt.Sprintf(`[{"value":%.12g}]`, lsqTrueP("exp_lnw")), uc)
	if !strings.Contains(detail, "指数模型 ln 线性化") {
		t.Errorf("指数题 detail 应含线性化标注: %s", detail)
	}
}

func TestScoreLSQ_UserConfigProblemKey(t *testing.T) {
	// 换题不带 m/x_eval：回落新题库默认（v0.12.1 归属规则）
	score, _, _ := submitLSQ(t, fmt.Sprintf(`[{"value":%.12g}]`, lsqTrueP("quad_parabola")), `{"problem_key":"quad_parabola"}`)
	if score != 100 {
		t.Fatalf("换题 quad 按库默认算对应 100: got %d", score)
	}
	// 若错误沿用 target 的 x_eval=3.5（bug 场景）：按 3.5 算的值对不上 2.0 的真值
	def := fitProblems["quad_parabola"]
	cs, err := lsqFit(def.Xs, def.Ys, 2, false)
	if err != nil {
		t.Fatal(err)
	}
	score, _, _ = submitLSQ(t, fmt.Sprintf(`[{"value":%.12g}]`, lsqEval(cs, 3.5)), `{"problem_key":"quad_parabola"}`)
	if score >= 100 {
		t.Fatalf("按 x_eval=3.5 提交应暴露不符（真值在 2.0）: got %d", score)
	}
}

func TestScoreLSQ_UserConfigM(t *testing.T) {
	// 换拟合次数：m=0 常数拟合（=均值 3.5）、m=2 二次拟合（4.4725）
	for _, tc := range []struct {
		uc    string
		v     float64
		label string
	}{
		{`{"m":0}`, 3.5, "m=0 常数拟合"},
		{`{"m":2}`, 4.4725, "m=2 二次拟合"},
	} {
		score, _, _ := submitLSQ(t, fmt.Sprintf(`[{"value":%.12g}]`, tc.v), tc.uc)
		if score != 100 {
			t.Errorf("%s 应 100: got %d", tc.label, score)
		}
	}
	// quad 换 m=1：直线拟合语义下算对也是满分（探索次数合法）
	score, _, _ := submitLSQ(t, `[{"value":6.02}]`, `{"problem_key":"quad_parabola","m":1}`)
	if score != 100 {
		t.Errorf("quad m=1 直线拟合值 6.02 应 100: got %d", score)
	}
	// m 越界：>点数-2（退化成插值）、>5、负数都拦
	for _, uc := range []string{`{"m":3}`, `{"m":6}`, `{"m":-1}`, `{"problem_key":"quad_parabola","m":4}`} {
		if _, _, _, err := scoreByExperiment("least_squares_fit",
			[]byte(`[{"value":1}]`), []byte(lsqTarget), []byte(uc)); err == nil {
			t.Errorf("user_config %s 应报错", uc)
		}
	}
}

func TestScoreLSQ_UserConfigXEval(t *testing.T) {
	// 换求值点到 0：P(0)=截距 1.1
	score, _, detail := submitLSQ(t, `[{"value":1.1}]`, `{"x_eval":0}`)
	if score != 100 {
		t.Fatalf("x_eval=0 处 P=1.1 应 100: got %d", score)
	}
	if !strings.Contains(detail, "x_eval=0") {
		t.Errorf("detail 应记录换后求值点: %s", detail)
	}
}

func TestScoreLSQ_CustomPoints(t *testing.T) {
	// 自定义数据点（默认 m=1，须显式给 x_eval）：指数增长型数据
	xs := []float64{0, 1, 2, 3}
	ys := []float64{1, 2.1, 4.2, 8.7}
	cs, err := lsqFit(xs, ys, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	truth := lsqEval(cs, 1.5)
	score, _, detail := submitLSQ(t,
		fmt.Sprintf(`[{"value":%.12g}]`, truth),
		`{"pts_x":[0,1,2,3],"pts_y":[1,2.1,4.2,8.7],"x_eval":1.5}`)
	if score != 100 {
		t.Fatalf("自定义数据点正确值: want 100, got %d", score)
	}
	if !strings.Contains(detail, "problem=custom") || !strings.Contains(detail, "自定义数据点") {
		t.Errorf("detail 应记录 custom: %s", detail)
	}
	// 自定义缺 x_eval / 只给 pts_x / 2 个点 / x 重复 均 400
	for _, uc := range []string{
		`{"pts_x":[0,1,2],"pts_y":[1,2,4]}`,
		`{"pts_x":[0,1,2]}`,
		`{"pts_x":[0,1],"pts_y":[1,2],"x_eval":0.5}`,
		`{"pts_x":[0,0,2],"pts_y":[1,2,4],"x_eval":1}`,
	} {
		if _, _, _, err := scoreByExperiment("least_squares_fit",
			[]byte(`[{"value":1}]`), []byte(lsqTarget), []byte(uc)); err == nil {
			t.Errorf("user_config %s 应报错", uc)
		}
	}
}

func TestScoreLSQ_CoefAudit(t *testing.T) {
	// coef 长度须 m+1（m=1 时 2 个）、数值有限
	if _, _, _, err := scoreByExperiment("least_squares_fit",
		[]byte(`[{"value":4.46,"coef":[1.1,0.96,3]}]`), []byte(lsqTarget), nil); err == nil {
		t.Error("coef 长度错应报错")
	}
	if _, _, _, err := scoreByExperiment("least_squares_fit",
		[]byte(`[{"value":4.46,"coef":[1.1,NaN]}]`), []byte(lsqTarget), nil); err == nil {
		t.Error("coef 含 NaN 应报错")
	}
}

func TestScoreLSQ_BadInput(t *testing.T) {
	if _, _, _, err := scoreByExperiment("least_squares_fit", []byte(`[{}]`), []byte(lsqTarget), nil); err == nil {
		t.Error("缺 value 应报错")
	}
	if _, _, _, err := scoreByExperiment("least_squares_fit", []byte(`[{"value":"x"}]`), []byte(lsqTarget), nil); err == nil {
		t.Error("value 非数值应报错")
	}
	if _, _, _, err := scoreByExperiment("least_squares_fit", []byte(`[{"value":NaN}]`), []byte(lsqTarget), nil); err == nil {
		t.Error("NaN value 应报错")
	}
	if _, _, _, err := scoreByExperiment("least_squares_fit",
		[]byte(`[{"value":1}]`), []byte(lsqTarget), []byte(`{"problem_key":"nope"}`)); err == nil {
		t.Error("未知题应报错")
	}
}

func TestScoreLSQ_TargetParamsOnlyForTargetProblem(t *testing.T) {
	// target 带 lin_line 的 m=0、x_eval=3.5；换题 quad 不带 m/x_eval 时
	// 应用 quad 库默认 m=2、x_eval=2，而非沿用 target 的
	tgt := `{"method":"lsq_rel_err","problem_key":"lin_line","m":0,"x_eval":3.5,"pass_score":60}`
	// 未换题：target 的 m=0（常数拟合=均值 3.5）与 x_eval=3.5 生效
	score, _, _, err := scoreByExperiment("least_squares_fit",
		[]byte(`[{"value":3.5}]`), []byte(tgt), nil)
	if err != nil {
		t.Fatal(err)
	}
	if score != 100 {
		t.Fatalf("未换题时 target 的 m=0/x_eval=3.5 应生效: want 100, got %d", score)
	}
	// 换题不带 m/x_eval：按 quad 默认（m=2, x_eval=2）算对满分
	score, _, _, err = scoreByExperiment("least_squares_fit",
		[]byte(fmt.Sprintf(`[{"value":%.12g}]`, lsqTrueP("quad_parabola"))), []byte(tgt), []byte(`{"problem_key":"quad_parabola"}`))
	if err != nil {
		t.Fatal(err)
	}
	if score != 100 {
		t.Fatalf("换题不带 m/x_eval 应用库默认: want 100, got %d", score)
	}
	// 换题后若错误沿用 target 的 m=0：按均值 30.1/5=6.02 提交对不上 m=2 真值
	score, _, _, err = scoreByExperiment("least_squares_fit",
		[]byte(`[{"value":6.02}]`), []byte(tgt), []byte(`{"problem_key":"quad_parabola"}`))
	if err != nil {
		t.Fatal(err)
	}
	if score >= 100 {
		t.Fatalf("按 m=0 提交应暴露不符（真值是 m=2）: got %d", score)
	}
	// 换题后若错误沿用 target 的 x_eval=3.5：按 3.5 算对不上 2.0 的真值
	def := fitProblems["quad_parabola"]
	cs, ferr := lsqFit(def.Xs, def.Ys, 2, false)
	if ferr != nil {
		t.Fatal(ferr)
	}
	score, _, _, err = scoreByExperiment("least_squares_fit",
		[]byte(fmt.Sprintf(`[{"value":%.12g}]`, lsqEval(cs, 3.5))), []byte(tgt), []byte(`{"problem_key":"quad_parabola"}`))
	if err != nil {
		t.Fatal(err)
	}
	if score >= 100 {
		t.Fatalf("按 x_eval=3.5 提交应暴露不符（真值在 2.0）: got %d", score)
	}
}

// ========== L15 三次样条插值（cubic_spline，v0.20）==========

const splineTarget = `{"method":"spline_rel_err","problem_key":"dist_4","x_eval":3,"pass_score":60}`

func submitSpline(t *testing.T, readings, userConfig string) (int, bool, string) {
	t.Helper()
	score, passed, detail, err := scoreByExperiment("cubic_spline",
		[]byte(readings), []byte(splineTarget), []byte(userConfig))
	if err != nil {
		t.Fatal(err)
	}
	return score, passed, detail
}

// splineTrueSAt 库内题真值 S(x_eval)：与评分器同一计算路径，单测锚点基准。
func splineTrueSAt(key string, xEval float64) float64 {
	def := splineProblems[key]
	m, err := splineMoments(def.Xs, def.Ys)
	if err != nil {
		panic(err)
	}
	return splineEvalAt(def.Xs, def.Ys, m, xEval)
}

func TestScoreSpline_ClosedForms(t *testing.T) {
	// dist_4 手算三弯矩（不等距 h=1,1,2，M_0=M_3=0）：
	//   d_1 = 6·[(6−2.5)/1 − (2.5−0)/1]/2 = 3
	//   d_2 = 6·[(20−6)/2 − (6−2.5)/1]/3 = 7
	//   方程：2M_1+0.5M_2=3；(1/3)M_1+2M_2=7，手解 M_1=15/23、M_2=78/23
	//   S(3)=559/46（段 [2,4]：B=109/23、C=39/23、D=−13/46）
	xs := splineProblems["dist_4"].Xs
	ys := splineProblems["dist_4"].Ys
	m, err := splineMoments(xs, ys)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(m[1]-15.0/23) > 1e-9 || math.Abs(m[2]-78.0/23) > 1e-9 {
		t.Fatalf("dist_4 弯矩应为 15/23、78/23: got %v", m)
	}
	if s := splineEvalAt(xs, ys, m, 3); math.Abs(s-559.0/46) > 1e-9 {
		t.Fatalf("dist_4 S(3) 应恰为 559/46: got %.12g", s)
	}
	// 手解方程的独立路径（高斯消去解 2×2）交叉验证追赶法
	g, gerr := gaussSolve([][]float64{{2, 0.5}, {1.0 / 3, 2}}, []float64{3, 7})
	if gerr != nil {
		t.Fatal(gerr)
	}
	if math.Abs(g[0]-15.0/23) > 1e-9 || math.Abs(g[1]-78.0/23) > 1e-9 {
		t.Fatalf("高斯独立解应与追赶法一致: got %v", g)
	}
	// sin_5：等距对称题 M_1=M_3，真值 S(π/6)≈0.49973964808906（node 锚点）
	sm, err := splineMoments(splineProblems["sin_5"].Xs, splineProblems["sin_5"].Ys)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(sm[1]-sm[3]) > 1e-12 {
		t.Fatalf("sin_5 对称性 M_1 应= M_3: got %v", sm)
	}
	if math.Abs(sm[1]-(-0.744149886578872)) > 1e-9 || math.Abs(sm[2]-(-1.05238686203824)) > 1e-9 {
		t.Fatalf("sin_5 弯矩锚点: got %v", sm)
	}
	if v := splineTrueSAt("sin_5", splineProblems["sin_5"].XEval); math.Abs(v-0.49973964808906) > 1e-9 {
		t.Fatalf("sin_5 S(π/6) 锚点 0.49973964808906: got %.12g", v)
	}
	// runge_5：与 L16 同节点同求值点，真值 S(0.8)≈−0.036339522546419（node 锚点）
	rm, err := splineMoments(splineProblems["runge_5"].Xs, splineProblems["runge_5"].Ys)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(rm[1]-rm[3]) > 1e-12 {
		t.Fatalf("runge_5 对称性 M_1 应= M_3: got %v", rm)
	}
	if math.Abs(rm[1]-8.18491852974612) > 1e-9 || math.Abs(rm[2]-(-14.43728685108)) > 1e-9 {
		t.Fatalf("runge_5 弯矩锚点: got %v", rm)
	}
	if v := splineTrueSAt("runge_5", 0.8); math.Abs(v-(-0.0363395225464190)) > 1e-9 {
		t.Fatalf("runge_5 S(0.8) 锚点 −0.036339522546419: got %.12g", v)
	}
	// 自定义 x² 四节点：M=[0,2.4,2.4,0]，S(2.5)=6.35（自然边界对 x² 并不精确——
	// 端段弯矩被迫为 0，S(2.5)=6.35 ≠ 6.25，抄原函数 6.25 约 58 分恰差线 2 分）
	cm, err := splineMoments([]float64{0, 1, 2, 3}, []float64{0, 1, 4, 9})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(cm[1]-2.4) > 1e-9 || math.Abs(cm[2]-2.4) > 1e-9 {
		t.Fatalf("x² 节点弯矩应为 [0,2.4,2.4,0]: got %v", cm)
	}
	if v := splineEvalAt([]float64{0, 1, 2, 3}, []float64{0, 1, 4, 9}, cm, 2.5); math.Abs(v-6.35) > 1e-9 {
		t.Fatalf("x² 节点 S(2.5) 应恰为 6.35: got %.12g", v)
	}
}

func TestScoreSpline_EquationIdentity(t *testing.T) {
	// 三弯矩方程恒等式：解出的 M 代回方程残差≈0；节点复现 S(x_k)=y_k；
	// 端点弯矩自然为零；追赶法与稠密高斯消去双路径一致。
	type c struct {
		xs, ys []float64
		name   string
	}
	cases := []c{
		{splineProblems["dist_4"].Xs, splineProblems["dist_4"].Ys, "dist_4"},
		{splineProblems["sin_5"].Xs, splineProblems["sin_5"].Ys, "sin_5"},
		{splineProblems["runge_5"].Xs, splineProblems["runge_5"].Ys, "runge_5"},
		{[]float64{0, 1, 2, 3}, []float64{0, 1, 4, 9}, "custom_x2"},
		{[]float64{0, 1, 3, 4, 7}, []float64{1, 2, 2.5, 6, 4}, "custom_5"}, // 5 节点不等距
	}
	for _, c := range cases {
		n := len(c.xs)
		m, err := splineMoments(c.xs, c.ys)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if m[0] != 0 || m[n-1] != 0 {
			t.Fatalf("%s: 自然边界 M_0/M_{n-1} 应为 0: got %v", c.name, m)
		}
		// 方程残差 + 稠密高斯双路径
		k := n - 2
		A := make([][]float64, k)
		d := make([]float64, k)
		for i := range A {
			A[i] = make([]float64, k)
			A[i][i] = 2
		}
		for i := 1; i <= n-2; i++ {
			h0, h1 := c.xs[i]-c.xs[i-1], c.xs[i+1]-c.xs[i]
			mu, lam := h0/(h0+h1), h1/(h0+h1)
			di := 6 * ((c.ys[i+1]-c.ys[i])/h1 - (c.ys[i]-c.ys[i-1])/h0) / (h0 + h1)
			d[i-1] = di
			if i > 1 {
				A[i-1][i-2] = mu
			}
			if i < n-2 {
				A[i-1][i] = lam
			}
			r := mu*m[i-1] + 2*m[i] + lam*m[i+1] - di
			if math.Abs(r) > 1e-9 {
				t.Fatalf("%s: 三弯矩方程 %d 残差 %.3g 非零", c.name, i, r)
			}
		}
		if k > 0 {
			g, gerr := gaussSolve(A, d)
			if gerr != nil {
				t.Fatalf("%s 高斯消去: %v", c.name, gerr)
			}
			for j := range g {
				if math.Abs(g[j]-m[j+1]) > 1e-10 {
					t.Fatalf("%s: 追赶法 vs 高斯不一致（第 %d 个弯矩 %.12g vs %.12g）", c.name, j+1, m[j+1], g[j])
				}
			}
		}
		// 节点复现：样条必过所有节点
		for i := 0; i < n; i++ {
			if v := splineEvalAt(c.xs, c.ys, m, c.xs[i]); math.Abs(v-c.ys[i]) > 1e-9 {
				t.Fatalf("%s: 节点 x_%d 处 S=%.12g 应= y=%.12g", c.name, i, v, c.ys[i])
			}
		}
	}
}

func TestScoreSpline_ExactSolution(t *testing.T) {
	// 默认题 dist_4：闭式 559/46 + 弯矩审计
	score, passed, detail := submitSpline(t,
		fmt.Sprintf(`[{"value":%g,"m_col":[0,%g,%g,0]}]`, 559.0/46, 15.0/23, 78.0/23), `{}`)
	if score != 100 || !passed {
		t.Fatalf("正确样条值+弯矩: want 100/passed, got %d/%v", score, passed)
	}
	for _, frag := range []string{"problem=dist_4", "nodes=4", "x_eval=3", "true_S=12.15217391", "三次样条插值·自然边界三弯矩", "m_col=[0,0.652174,3.3913,0]"} {
		if !strings.Contains(detail, frag) {
			t.Fatalf("detail 缺片段 %q: %s", frag, detail)
		}
	}
	// 换题 sin_5 正确（detail 应记审计 ref_f=0.5）
	score, passed, detail = submitSpline(t,
		fmt.Sprintf(`[{"value":%.15g}]`, splineTrueSAt("sin_5", splineProblems["sin_5"].XEval)),
		`{"problem_key":"sin_5"}`)
	if score != 100 || !passed {
		t.Fatalf("sin_5 正确值: want 100/passed, got %d/%v", score, passed)
	}
	if !strings.Contains(detail, "ref_f=0.5") {
		t.Fatalf("sin_5 detail 应含 ref_f=0.5: %s", detail)
	}
	// 换题 runge_5 正确（ref_f=1/17）
	score, passed, detail = submitSpline(t,
		fmt.Sprintf(`[{"value":%.15g}]`, splineTrueSAt("runge_5", 0.8)),
		`{"problem_key":"runge_5"}`)
	if score != 100 || !passed {
		t.Fatalf("runge_5 正确值: want 100/passed, got %d/%v", score, passed)
	}
	if !strings.Contains(detail, "ref_f=0.0588235294") {
		t.Fatalf("runge_5 detail 应含 ref_f=1/17: %s", detail)
	}
}

func TestScoreSpline_RelErrCurve(t *testing.T) {
	base := 559.0 / 46
	cases := []struct {
		eps  float64
		want int
	}{
		{1e-6, 100}, {1e-4, 90}, {1e-3, 80}, {1e-2, 65},
	}
	for _, c := range cases {
		score, _, _ := submitSpline(t, fmt.Sprintf(`[{"value":%.15g}]`, base*(1+c.eps)), `{}`)
		if score != c.want {
			t.Fatalf("相对误差 %.0e: want %d, got %d", c.eps, c.want, score)
		}
	}
}

func TestScoreSpline_TrapsDist4(t *testing.T) {
	// 默认题主场：只有真解三弯矩才过线
	// 抄拉格朗日 P(3)=11.5（在 L16 页对同数据做插值）→ 39 不过线
	score, passed, _ := submitSpline(t, `[{"value":11.5}]`, `{}`)
	if score != 39 || passed {
		t.Fatalf("抄拉格朗日 11.5: want 39/不过线, got %d/%v", score, passed)
	}
	// 抄线性内插 13（不做样条只做折线）→ 35 不过线
	score, passed, _ = submitSpline(t, `[{"value":13}]`, `{}`)
	if score != 35 || passed {
		t.Fatalf("抄线性内插 13: want 35/不过线, got %d/%v", score, passed)
	}
	// 抄观测值全 0
	for _, v := range []float64{0, 2.5, 6, 20} {
		score, passed, _ = submitSpline(t, fmt.Sprintf(`[{"value":%.12g}]`, v), `{}`)
		if passed {
			t.Fatalf("抄观测 %.12g 不应过线: got %d", v, score)
		}
	}
}

func TestScoreSpline_TrapSin5(t *testing.T) {
	uc := `{"problem_key":"sin_5"}`
	// 样条逼近光滑函数极好：抄 sin(π/6)=0.5 → 83 过线不满分（m_col 审计暴露）
	score, passed, _ := submitSpline(t, `[{"value":0.5}]`, uc)
	if score != 83 || !passed {
		t.Fatalf("抄 sin(π/6): want 83/过线不满分, got %d/%v", score, passed)
	}
	// 抄拉格朗日 P(π/6)=0.499070658352655 → 78 过线不满分
	score, passed, _ = submitSpline(t, `[{"value":0.499070658352655}]`, uc)
	if score != 78 || !passed {
		t.Fatalf("抄拉格朗日: want 78/过线不满分, got %d/%v", score, passed)
	}
	// 抄观测值不过线
	for _, v := range []float64{0, 0.7071067811865476, 1} {
		score, passed, _ = submitSpline(t, fmt.Sprintf(`[{"value":%.12g}]`, v), uc)
		if passed {
			t.Fatalf("抄观测 %.12g 不应过线: got %d", v, score)
		}
	}
}

func TestScoreSpline_TrapRunge5(t *testing.T) {
	uc := `{"problem_key":"runge_5"}`
	// 抄 L16 拉格朗日 P(0.8)=−0.379310 剧烈震荡值 → 0 分
	score, passed, _ := submitSpline(t, `[{"value":-0.379310344827586}]`, uc)
	if score != 0 || passed {
		t.Fatalf("抄拉格朗日 P(0.8): want 0, got %d/%v", score, passed)
	}
	// 抄原函数 f(0.8)=1/17 → 0 分
	score, passed, _ = submitSpline(t, fmt.Sprintf(`[{"value":%.12g}]`, 1.0/17), uc)
	if score != 0 || passed {
		t.Fatalf("抄 f(0.8)=1/17: want 0, got %d/%v", score, passed)
	}
	// 抄观测值全 0 分（真值为负，全部观测为正——端段下弯）
	for _, v := range []float64{1.0 / 26, 4.0 / 29, 1} {
		score, passed, _ = submitSpline(t, fmt.Sprintf(`[{"value":%.12g}]`, v), uc)
		if score != 0 || passed {
			t.Fatalf("抄观测 %.12g: want 0, got %d/%v", v, score, passed)
		}
	}
}

func TestScoreSpline_SegmentEndpoints(t *testing.T) {
	// 端点与中间节点求值：样条必过节点，S(x_k)=y_k 精确成立
	cases := []struct {
		xEval, want float64
	}{
		{0, 0},   // 左端点
		{1, 2.5}, // 内部节点
		{2, 6},   // 内部节点
		{4, 20},  // 右端点（段定位须归最后一段）
	}
	for _, c := range cases {
		score, _, _ := submitSpline(t,
			fmt.Sprintf(`[{"value":%.12g}]`, c.want), fmt.Sprintf(`{"x_eval":%.12g}`, c.xEval))
		if score != 100 {
			t.Fatalf("x_eval=%.6g 处节点值 %.6g: want 100, got %d", c.xEval, c.want, score)
		}
	}
}

func TestScoreSpline_UserConfigXEval(t *testing.T) {
	// 换求值点（段 [1,2] 内）：手算闭式 S(1.5)=1471/368
	score, _, _ := submitSpline(t, fmt.Sprintf(`[{"value":%g}]`, 1471.0/368), `{"x_eval":1.5}`)
	if score != 100 {
		t.Fatalf("换 x_eval=1.5: want 100, got %d", score)
	}
	// 样条不外推：x_eval 超区间应报错
	for _, x := range []float64{-0.1, 4.5} {
		if _, _, _, err := scoreByExperiment("cubic_spline",
			[]byte(`[{"value":1}]`), []byte(splineTarget), []byte(fmt.Sprintf(`{"x_eval":%.12g}`, x))); err == nil {
			t.Errorf("x_eval=%.6g 超区间应报错（样条不外推）", x)
		}
	}
}

func TestScoreSpline_CustomNodes(t *testing.T) {
	uc := `{"nodes_x":[0,1,2,3],"nodes_y":[0,1,4,9],"x_eval":2.5}`
	// 自定义节点真值只能现算：正确 6.35 → 100
	score, passed, detail := submitSpline(t, `[{"value":6.35}]`, uc)
	if score != 100 || !passed {
		t.Fatalf("自定义节点正确值: want 100/passed, got %d/%v", score, passed)
	}
	if !strings.Contains(detail, "problem=custom") {
		t.Fatalf("detail 应记 problem=custom: %s", detail)
	}
	// 抄原函数 x²(2.5)=6.25：自然边界端段偏差 → 58 恰差线 2 分
	score, passed, _ = submitSpline(t, `[{"value":6.25}]`, uc)
	if score != 58 || passed {
		t.Fatalf("抄 x²(2.5): want 58/不过线, got %d/%v", score, passed)
	}
	// 坏输入：2 节点 / 9 节点 / 乱序 / 缺 x_eval / 超区间 / 数量不匹配
	bads := []string{
		`{"nodes_x":[0,1],"nodes_y":[0,1],"x_eval":0.5}`,
		`{"nodes_x":[0,1,2,3,4,5,6,7,8],"nodes_y":[0,1,4,9,16,25,36,49,64],"x_eval":2.5}`,
		`{"nodes_x":[1,0,2,3],"nodes_y":[1,0,4,9],"x_eval":2.5}`,
		`{"nodes_x":[0,1,2,3],"nodes_y":[0,1,4,9]}`,
		`{"nodes_x":[0,1,2,3],"nodes_y":[0,1,4,9],"x_eval":5}`,
		`{"nodes_x":[0,1,2,3],"nodes_y":[0,1,4],"x_eval":2.5}`,
	}
	for _, b := range bads {
		if _, _, _, err := scoreByExperiment("cubic_spline",
			[]byte(`[{"value":1}]`), []byte(splineTarget), []byte(b)); err == nil {
			t.Errorf("自定义节点坏输入应报错: %s", b)
		}
	}
}

func TestScoreSpline_MColAudit(t *testing.T) {
	// m_col 长度须为节点数（含端点零弯矩）
	if _, _, _, err := scoreByExperiment("cubic_spline",
		[]byte(fmt.Sprintf(`[{"value":%g,"m_col":[0,%g,%g]}]`, 559.0/46, 15.0/23, 78.0/23)),
		[]byte(splineTarget), nil); err == nil {
		t.Error("m_col 长度错应报错")
	}
	if _, _, _, err := scoreByExperiment("cubic_spline",
		[]byte(fmt.Sprintf(`[{"value":%g,"m_col":[0,NaN,%g,0]}]`, 559.0/46, 78.0/23)),
		[]byte(splineTarget), nil); err == nil {
		t.Error("m_col 含 NaN 应报错")
	}
}

func TestScoreSpline_BadInput(t *testing.T) {
	if _, _, _, err := scoreByExperiment("cubic_spline", []byte(`[{}]`), []byte(splineTarget), nil); err == nil {
		t.Error("缺 value 应报错")
	}
	if _, _, _, err := scoreByExperiment("cubic_spline", []byte(`[{"value":"x"}]`), []byte(splineTarget), nil); err == nil {
		t.Error("value 非数值应报错")
	}
	if _, _, _, err := scoreByExperiment("cubic_spline", []byte(`[{"value":NaN}]`), []byte(splineTarget), nil); err == nil {
		t.Error("NaN value 应报错")
	}
	if _, _, _, err := scoreByExperiment("cubic_spline",
		[]byte(`[{"value":1}]`), []byte(splineTarget), []byte(`{"problem_key":"nope"}`)); err == nil {
		t.Error("未知题应报错")
	}
}

func TestScoreSpline_TargetParamsOnlyForTargetProblem(t *testing.T) {
	// target 带 dist_4 的 x_eval=3；换题 sin_5 不带 x_eval 时应用 sin_5 库默认 π/6，
	// 而非沿用 target 的 3（v0.12.1 归属规则）
	tgt := `{"method":"spline_rel_err","problem_key":"dist_4","x_eval":3,"pass_score":60}`
	// 未换题：target 的 x_eval=3 生效
	score, _, _, err := scoreByExperiment("cubic_spline",
		[]byte(fmt.Sprintf(`[{"value":%g}]`, 559.0/46)), []byte(tgt), nil)
	if err != nil {
		t.Fatal(err)
	}
	if score != 100 {
		t.Fatalf("未换题时 target 的 x_eval=3 应生效: want 100, got %d", score)
	}
	// 换题不带 x_eval：按 sin_5 库默认 π/6 算对满分
	score, _, _, err = scoreByExperiment("cubic_spline",
		[]byte(fmt.Sprintf(`[{"value":%.15g}]`, splineTrueSAt("sin_5", splineProblems["sin_5"].XEval))),
		[]byte(tgt), []byte(`{"problem_key":"sin_5"}`))
	if err != nil {
		t.Fatal(err)
	}
	if score != 100 {
		t.Fatalf("换题不带 x_eval 应用库默认 π/6: want 100, got %d", score)
	}
	// 换题后若错误沿用 target 的 x_eval=3：按 sin_5 样条在 3 处的值提交对不上 π/6 真值
	score, _, _, err = scoreByExperiment("cubic_spline",
		[]byte(fmt.Sprintf(`[{"value":%.15g}]`, splineTrueSAt("sin_5", 3))),
		[]byte(tgt), []byte(`{"problem_key":"sin_5"}`))
	if err != nil {
		t.Fatal(err)
	}
	if score >= 100 {
		t.Fatalf("按 x_eval=3 提交应暴露不符（真值在 π/6）: got %d", score)
	}
	// 自定义节点不带 x_eval：应报错（若错误沿用 target 的 3 则不报错——归属回归）
	if _, _, _, err := scoreByExperiment("cubic_spline",
		[]byte(`[{"value":6.35}]`), []byte(tgt),
		[]byte(`{"nodes_x":[0,1,2,3],"nodes_y":[0,1,4,9]}`)); err == nil {
		t.Error("自定义节点缺 x_eval 应报错（target 的 x_eval 不跨题沿用）")
	}
}
