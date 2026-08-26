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
		{"  x  ", true},  // trim 后非空
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

// ========== 物理实验批次（牛顿环 / 示波器，PDF 交付物）==========

const physicsBandsJSON = `[[0.005,100],[0.02,90],[0.05,80],[0.1,65],[0.3,30],[0.6,0]]`

const newtonRingTarget = `{"method":"least_squares_R","lens_radius_mm":855,"wavelength_nm":589.3,"pass_score":60,"bands":` + physicsBandsJSON + `}`

// newtonRingReadings 生成 k=1..10 的读数（r 模拟前端 toFixed(3) 三位小数 + 偏置）
func newtonRingReadings(R, lambdaNM, bias float64) string {
	lambdaMM := lambdaNM * 1e-6
	var sb strings.Builder
	sb.WriteString("[")
	for k := 1; k <= 10; k++ {
		r := math.Round(math.Sqrt(float64(k)*lambdaMM*R)*1000)/1000 + bias
		if k > 1 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, `{"k":%d,"r":%.4f}`, k, r)
	}
	sb.WriteString("]")
	return sb.String()
}

func submitNewtonRing(t *testing.T, readings, userConfig string) (int, bool, string) {
	t.Helper()
	uc := []byte(nil)
	if userConfig != "" {
		uc = []byte(userConfig)
	}
	score, passed, detail, err := scoreByExperiment("newton_ring", []byte(readings), []byte(newtonRingTarget), uc)
	if err != nil {
		t.Fatalf("readings=%s user_config=%s: %v", readings, userConfig, err)
	}
	return score, passed, detail
}

func TestScoreNewtonRing_PerfectReadings(t *testing.T) {
	// 理论暗环半径 r_k=√(kλR) 保留 3 位小数（前端 toFixed(3)），拟合误差远小于满分档
	score, passed, detail := submitNewtonRing(t, newtonRingReadings(855, 589.3, 0), "")
	if score != 100 || !passed {
		t.Fatalf("want 100/passed, got %d/%v (detail=%s)", score, passed, detail)
	}
}

func TestScoreNewtonRing_WithinTolerance(t *testing.T) {
	// 全部读数偏 +0.01mm（容差 0.02mm 内）：R 拟合约偏 1%，bands 插值得 95
	score, passed, _ := submitNewtonRing(t, newtonRingReadings(855, 589.3, 0.01), "")
	if score != 95 || !passed {
		t.Fatalf("want 95/passed, got %d/%v", score, passed)
	}
}

func TestScoreNewtonRing_GrosslyWrong(t *testing.T) {
	// 半径全部偏 50%：R_fit 偏 125%，0 分
	score, passed, _ := submitNewtonRing(t, newtonRingReadings(855*2.25, 589.3, 0), "")
	if score != 0 || passed {
		t.Fatalf("want 0/not-passed, got %d/%v", score, passed)
	}
}

func TestScoreNewtonRing_UserConfigOverride(t *testing.T) {
	// 滑块换了 R=1000：读数按 1000 算，须携带 user_config 才能高分（v0.5 滑块修复）
	uc := `{"lens_radius_mm":1000,"wavelength_nm":589.3}`
	score, passed, _ := submitNewtonRing(t, newtonRingReadings(1000, 589.3, 0), uc)
	if score != 100 || !passed {
		t.Fatalf("user_config R=1000: want 100/passed, got %d/%v", score, passed)
	}
	// 不携带 user_config（回退 DB 真值 855）：R 拟合偏 ~17%，应不及格
	score, passed, _ = submitNewtonRing(t, newtonRingReadings(1000, 589.3, 0), "")
	if passed || score >= 60 {
		t.Fatalf("不携带 user_config 的换参读数应不及格, got %d/%v", score, passed)
	}
}

func TestScoreNewtonRing_BadInput(t *testing.T) {
	cases := []struct {
		name, readings string
	}{
		{"少于 3 组", `[{"k":1,"r":0.71},{"k":2,"r":1.0}]`},
		{"r 为 0", `[{"k":1,"r":0.71},{"k":2,"r":0},{"k":3,"r":1.23}]`},
		{"k 为 0", `[{"k":0,"r":0.71},{"k":2,"r":1.0},{"k":3,"r":1.23}]`},
		{"r 非法 JSON", `[{"k":1,"r":NaN},{"k":2,"r":1.0},{"k":3,"r":1.23}]`},
	}
	for _, c := range cases {
		if _, _, _, err := scoreByExperiment("newton_ring", []byte(c.readings), []byte(newtonRingTarget), nil); err == nil {
			t.Errorf("%s: want error, got nil", c.name)
		}
	}
	// target.bands 非法（relErr 未递增）
	badTarget := `{"method":"least_squares_R","lens_radius_mm":855,"wavelength_nm":589.3,"bands":[[0.02,90],[0.005,100]]}`
	if _, _, _, err := scoreByExperiment("newton_ring", []byte(newtonRingReadings(855, 589.3, 0)), []byte(badTarget), nil); err == nil {
		t.Error("bands 未递增: want error, got nil")
	}
}

const oscilloscopeTarget = `{"method":"param_match","channels":{"CH1":{"A":2.0,"f":50},"CH2":{"A":1.5,"f":50}},"pass_score":60,"bands":` + physicsBandsJSON + `}`

func submitOscilloscope(t *testing.T, readings string) (int, bool, string) {
	t.Helper()
	score, passed, detail, err := scoreByExperiment("oscilloscope", []byte(readings), []byte(oscilloscopeTarget), nil)
	if err != nil {
		t.Fatalf("readings=%s: %v", readings, err)
	}
	return score, passed, detail
}

func TestScoreOscilloscope_ExactMatch(t *testing.T) {
	score, passed, _ := submitOscilloscope(t, `[{"channel":"CH1","f":50,"A":2.0},{"channel":"CH2","f":50,"A":1.5}]`)
	if score != 100 || !passed {
		t.Fatalf("want 100/passed, got %d/%v", score, passed)
	}
}

func TestScoreOscilloscope_PartialError(t *testing.T) {
	// CH1 频率读偏 10%（55 vs 50）-> 该参数 65 分，其余满分：(65+100*3)/4 = 91
	score, passed, _ := submitOscilloscope(t, `[{"channel":"CH1","f":55,"A":2.0},{"channel":"CH2","f":50,"A":1.5}]`)
	if score != 91 || !passed {
		t.Fatalf("want 91/passed, got %d/%v", score, passed)
	}
}

func TestScoreOscilloscope_BadInput(t *testing.T) {
	cases := []struct {
		name, readings string
	}{
		{"缺 CH2", `[{"channel":"CH1","f":50,"A":2.0}]`},
		{"未知通道", `[{"channel":"CH1","f":50,"A":2.0},{"channel":"CH3","f":50,"A":1.5}]`},
		{"通道重复", `[{"channel":"CH1","f":50,"A":2.0},{"channel":"CH1","f":50,"A":1.5}]`},
		{"f 为 NaN", `[{"channel":"CH1","f":NaN,"A":2.0},{"channel":"CH2","f":50,"A":1.5}]`},
	}
	for _, c := range cases {
		if _, _, _, err := scoreByExperiment("oscilloscope", []byte(c.readings), []byte(oscilloscopeTarget), nil); err == nil {
			t.Errorf("%s: want error, got nil", c.name)
		}
	}
}
