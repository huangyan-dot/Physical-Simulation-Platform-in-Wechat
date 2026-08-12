package service

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func mustJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// newtonReadings 生成牛顿环理论读数 r(k)=sqrt(k·λ·R)，λ=589.3nm、R=855mm。
func newtonReadings() []newtonReading {
	lam := 589.3 * 1e-6 // nm -> mm
	R := 855.0
	rs := make([]newtonReading, 0, 10)
	for k := 1; k <= 10; k++ {
		rs = append(rs, newtonReading{K: k, R: math.Sqrt(float64(k) * lam * R)})
	}
	return rs
}

func TestScoreByExperiment_EmptyReadingsIsError(t *testing.T) {
	// 契约 §12：readings 非法 -> 400。空数组必须返回 err（上层映射为 400）。
	target := `{"method":"least_squares_R","lens_radius_mm":855,"wavelength_nm":589.3,"pass_score":60}`
	for _, bad := range []string{`[]`, `{}`, `"x"`, `null`} {
		if _, _, _, err := scoreByExperiment("newton_ring", []byte(bad), []byte(target), nil); err == nil {
			t.Fatalf("readings=%s: want error, got nil", bad)
		}
	}
}

func TestScoreByExperiment_UnknownExperimentIsError(t *testing.T) {
	if _, _, _, err := scoreByExperiment("nope", []byte(`[{"x":1}]`), []byte(`{}`), nil); err == nil {
		t.Fatal("unknown experiment code: want error, got nil")
	}
}

func TestScoreByExperiment_NewtonRingTheoreticalScores100(t *testing.T) {
	target := `{"method":"least_squares_R","lens_radius_mm":855,"wavelength_nm":589.3,"pass_score":60}`
	rs := newtonReadings()
	raw := mustJSON(t, rs)
	score, passed, _, err := scoreByExperiment("newton_ring", raw, []byte(target), nil)
	if err != nil || score != 100 || !passed {
		t.Fatalf("newton theoretical: want 100/passed/nil, got %d/%v/%v", score, passed, err)
	}
}

func TestScoreByExperiment_OscilloscopeDefaultsScore100(t *testing.T) {
	target := `{"method":"param_match","channels":{"CH1":{"A":2.0,"f":50},"CH2":{"A":1.5,"f":50}},"pass_score":60}`
	readings := `[{"channel":"CH1","f":50,"A":2.0},{"channel":"CH2","f":50,"A":1.5}]`
	score, passed, _, err := scoreByExperiment("oscilloscope", []byte(readings), []byte(target), nil)
	if err != nil || score != 100 || !passed {
		t.Fatalf("scope defaults: want 100/passed/nil, got %d/%v/%v", score, passed, err)
	}
}

func TestScoreByExperiment_OscilloscopeChannelCaseMatters(t *testing.T) {
	// 通道键大小写必须与 target 一致（CH1），否则不匹配 -> 0 分。
	// 这锁死了数据组"channels 键用大写"的约定。
	target := `{"method":"param_match","channels":{"CH1":{"A":2.0,"f":50}},"pass_score":60}`
	readings := `[{"channel":"ch1","f":50,"A":2.0}]` // 小写 ch1 不匹配
	score, passed, _, err := scoreByExperiment("oscilloscope", []byte(readings), []byte(target), nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if score != 0 || passed {
		t.Fatalf("wrong-case channel should score 0/not-pass, got %d/%v", score, passed)
	}
}

func TestScoreByExperiment_PendulumDefaultsScore100(t *testing.T) {
	target := `{"method":"gravity_fit","gravity":9.8,"length_m":1.0,"pass_score":60}`
	readings := `[{"period":2.01,"calc_g":9.80}]`
	score, passed, _, err := scoreByExperiment("pendulum", []byte(readings), []byte(target), nil)
	if err != nil || score != 100 || !passed {
		t.Fatalf("pendulum defaults: want 100/passed/nil, got %d/%v/%v", score, passed, err)
	}
}

func TestScoreByExperiment_NewtonRingUserConfigOverridesTarget(t *testing.T) {
	// 滑块问题修复：DB target R=855mm，但前端滑块设为 R=1000mm。
	// 提交的读数基于 R=1000mm 生成，user_config 携带 lens_radius_mm=1000。
	// 后端应按 R=1000mm 评分 -> 满分；不按 DB 的 855mm -> 0 分。
	dbTarget := `{"method":"least_squares_R","lens_radius_mm":855,"wavelength_nm":589.3,"pass_score":60}`
	userConfig := `{"lens_radius_mm":1000,"wavelength_nm":589.3}`

	lam := 589.3 * 1e-6
	R := 1000.0
	rs := make([]newtonReading, 0, 10)
	for k := 1; k <= 10; k++ {
		rs = append(rs, newtonReading{K: k, R: math.Sqrt(float64(k) * lam * R)})
	}
	raw := mustJSON(t, rs)

	// 带 user_config -> 满分
	score, passed, _, err := scoreByExperiment("newton_ring", raw, []byte(dbTarget), []byte(userConfig))
	if err != nil || score != 100 || !passed {
		t.Fatalf("with user_config R=1000: want 100/passed/nil, got %d/%v/%v", score, passed, err)
	}
	// 不带 user_config -> 按 DB R=855 评分，应低分
	score2, _, _, err := scoreByExperiment("newton_ring", raw, []byte(dbTarget), nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if score2 >= 60 {
		t.Fatalf("without user_config: R=1000 readings vs R=855 target should score <60, got %d", score2)
	}
}

func TestScoreByExperiment_PendulumUserConfigOverridesTarget(t *testing.T) {
	// 滑块问题修复：DB target g=9.8，前端滑块设为 g=9.0。
	// 提交 calc_g=9.0，user_config 携带 gravity=9.0 -> 满分。
	dbTarget := `{"method":"gravity_fit","gravity":9.8,"length_m":1.0,"pass_score":60}`
	userConfig := `{"gravity":9.0,"length_m":1.0}`
	readings := `[{"period":2.0944,"calc_g":9.0}]`

	score, passed, _, err := scoreByExperiment("pendulum", []byte(readings), []byte(dbTarget), []byte(userConfig))
	if err != nil || score != 100 || !passed {
		t.Fatalf("with user_config g=9.0: want 100/passed/nil, got %d/%v/%v", score, passed, err)
	}

	// 不带 user_config -> 按 DB g=9.8 评分，应低分
	score2, _, _, err := scoreByExperiment("pendulum", []byte(readings), []byte(dbTarget), nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if score2 >= 60 {
		t.Fatalf("without user_config: g=9.0 vs target g=9.8 should score <60, got %d", score2)
	}
}

func TestScoreNewtonRing_RejectsNaNAndInf(t *testing.T) {
	// 数据校验：NaN / Inf 读数应被跳过，不导致 panic
	target := `{"method":"least_squares_R","lens_radius_mm":855,"wavelength_nm":589.3,"pass_score":60}`
	// JSON 不支持 NaN/Infinity，所以这里直接测 Go 层
	rs := []newtonReading{
		{K: 1, R: 0.71},
		{K: 2, R: math.NaN()},
		{K: 3, R: math.Inf(1)},
		{K: 4, R: 1.42},
	}
	var t1 newtonTarget
	json.Unmarshal([]byte(target), &t1)
	score, _, _ := ScoreNewtonRing(rs, t1)
	if score < 0 {
		t.Fatalf("score should be >= 0, got %d", score)
	}
}

func TestScorePendulum_RejectsInvalidCalcG(t *testing.T) {
	// 数据校验：calc_g <= 0 / NaN / Inf 应返回 0 分
	t1 := pendulumTarget{Gravity: 9.8, PassScore: 60}
	for _, bad := range []float64{0, -1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		score, passed, _ := ScorePendulum([]pendulumReading{{Period: 2.0, CalcG: bad}}, t1)
		if score != 0 || passed {
			t.Fatalf("calc_g=%v: want 0/false, got %d/%v", bad, score, passed)
		}
	}
}

func TestScoreOscilloscope_SkipsNegativeValues(t *testing.T) {
	// 数据校验：负频率/振幅应被跳过，不影响有效读数评分
	target := `{"method":"param_match","channels":{"CH1":{"A":2.0,"f":50}},"pass_score":60}`
	readings := `[{"channel":"CH1","f":-1,"A":-1},{"channel":"CH1","f":50,"A":2.0}]`
	score, passed, _, err := scoreByExperiment("oscilloscope", []byte(readings), []byte(target), nil)
	if err != nil || score != 100 || !passed {
		t.Fatalf("scope with some bad values: want 100/passed/nil, got %d/%v/%v", score, passed, err)
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

func TestScoreFromRelErr(t *testing.T) {
	cases := []struct{ rel, want float64 }{
		{0, 100},   // 零误差满分
		{0.04, 80}, // 4% 误差 -> 80
		{0.08, 60}, // 8% 误差刚过 pass_score=60 线
		{0.2, 0},   // 大误差封顶 0
	}
	for _, c := range cases {
		got := scoreFromRelErr(c.rel)
		// scoreFromRelErr 用 math.Round，允许 ±1
		if math.Abs(float64(got)-c.want) > 1 {
			t.Errorf("rel=%v: want ~%v, got %d", c.rel, c.want, got)
		}
	}
}

// ===== 新增实验评分测试 =====

// youngModulusReadings 生成杨氏模量理论读数。
// E=2.0e11 Pa, F=4.905N, L=1.0m, d=0.5mm, b=70mm, D=1.5m
// 理论伸长 Δn = (8·F·n·L·D) / (π·d²·b·E) (m) -> mm
func youngModulusReadings() []youngModulusReading {
	E := 2.0e11
	F := 4.905
	L := 1.0
	d := 0.0005
	b := 0.070
	D := 1.5
	rs := make([]youngModulusReading, 0, 6)
	// 6 个砝码，逐个加载
	baseReading := 5.0 // mm，初始读数
	for i := 0; i <= 5; i++ {
		// 累计伸长 = (8·F·i·L·D) / (π·d²·b·E) (m) -> mm
		deltaM := (8.0 * F * float64(i) * L * D) / (math.Pi * d * d * b * E)
		deltaMM := deltaM * 1e3
		rs = append(rs, youngModulusReading{Load: i, Reading: baseReading + deltaMM})
	}
	return rs
}

func TestScoreByExperiment_YoungModulusTheoreticalScores100(t *testing.T) {
	target := `{"method":"young_fit","young_modulus_pa":2.0e11,"force_n":4.905,"length_m":1.0,"diameter_m":0.0005,"lever_arm_m":0.070,"mirror_dist_m":1.5,"pass_score":60}`
	rs := youngModulusReadings()
	raw := mustJSON(t, rs)
	score, passed, _, err := scoreByExperiment("young_modulus", raw, []byte(target), nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if score < 90 || !passed {
		t.Fatalf("young modulus theoretical: want >=90/passed, got %d/%v (detail: %s)", score, passed, "")
	}
}

func TestScoreByExperiment_YoungModulusUserConfigOverrides(t *testing.T) {
	// DB target 用 F=4.905N，但前端改 F=9.81N 并按 F=9.81 生成读数
	// user_config 携带 force_n=9.81 -> 满分
	dbTarget := `{"method":"young_fit","young_modulus_pa":2.0e11,"force_n":4.905,"length_m":1.0,"diameter_m":0.0005,"lever_arm_m":0.070,"mirror_dist_m":1.5,"pass_score":60}`
	userConfig := `{"force_n":9.81}`

	E := 2.0e11
	F := 9.81
	L := 1.0
	d := 0.0005
	b := 0.070
	D := 1.5
	rs := make([]youngModulusReading, 0, 6)
	baseReading := 5.0
	for i := 0; i <= 5; i++ {
		deltaM := (8.0 * F * float64(i) * L * D) / (math.Pi * d * d * b * E)
		rs = append(rs, youngModulusReading{Load: i, Reading: baseReading + deltaM*1e3})
	}
	raw := mustJSON(t, rs)

	score, passed, _, err := scoreByExperiment("young_modulus", raw, []byte(dbTarget), []byte(userConfig))
	if err != nil || score < 90 || !passed {
		t.Fatalf("with user_config F=9.81: want >=90/passed, got %d/%v (err=%v)", score, passed, err)
	}
}

func TestScoreYoungModulus_RejectsInvalidReadings(t *testing.T) {
	t1 := youngModulusTarget{
		YoungModulusPa: 2.0e11, ForceN: 4.905, LengthM: 1.0,
		DiameterM: 0.0005, LeverArmM: 0.070, MirrorDistM: 1.5, PassScore: 60,
	}
	// 含 NaN/Inf/负值的读数应被跳过
	rs := []youngModulusReading{
		{Load: 0, Reading: 5.0},
		{Load: 1, Reading: math.NaN()},
		{Load: 2, Reading: math.Inf(1)},
		{Load: 3, Reading: -1},
		{Load: 4, Reading: 5.5},
	}
	score, _, _ := ScoreYoungModulus(rs, t1)
	if score < 0 {
		t.Fatalf("score should be >= 0, got %d", score)
	}
}

func TestScoreByExperiment_HallEffectTheoreticalScores100(t *testing.T) {
	// V_H = (I·B) / (n·q·d)
	// n=1e21, q=1.602e-19, d=0.5mm=5e-4, B=0.3
	// V_H = I * 0.3 / (1e21 * 1.602e-19 * 5e-4) = I * 0.3 / 80100 = I * 3.7453e-6
	target := `{"method":"hall_fit","b_field_t":0.3,"thickness_m":0.0005,"carrier_conc":1.0e21,"pass_score":60}`

	const q = 1.602e-19
	n := 1.0e21
	B := 0.3
	d := 0.0005

	rs := make([]hallEffectReading, 0, 5)
	for _, I := range []float64{0.001, 0.002, 0.003, 0.004, 0.005} {
		VH := (I * B) / (n * q * d)
		rs = append(rs, hallEffectReading{CurrentA: I, VoltageV: VH})
	}
	raw := mustJSON(t, rs)

	score, passed, _, err := scoreByExperiment("hall_effect", raw, []byte(target), nil)
	if err != nil || score != 100 || !passed {
		t.Fatalf("hall effect theoretical: want 100/passed/nil, got %d/%v/%v", score, passed, err)
	}
}

func TestScoreHallEffect_RejectsInvalidReadings(t *testing.T) {
	t1 := hallEffectTarget{BFieldT: 0.3, ThicknessM: 0.0005, CarrierConc: 1e21, PassScore: 60}
	for _, bad := range []hallEffectReading{
		{CurrentA: 0, VoltageV: 1},
		{CurrentA: -1, VoltageV: 1},
		{CurrentA: 1, VoltageV: 0},
		{CurrentA: math.NaN(), VoltageV: 1},
		{CurrentA: 1, VoltageV: math.Inf(1)},
	} {
		score, passed, _ := ScoreHallEffect([]hallEffectReading{bad}, t1)
		if score != 0 || passed {
			t.Fatalf("bad reading %+v: want 0/false, got %d/%v", bad, score, passed)
		}
	}
}

func TestScoreByExperiment_HallEffectUserConfigOverrides(t *testing.T) {
	// DB target B=0.3T，前端改 B=0.5T 并按 B=0.5 生成读数
	dbTarget := `{"method":"hall_fit","b_field_t":0.3,"thickness_m":0.0005,"carrier_conc":1.0e21,"pass_score":60}`
	userConfig := `{"b_field_t":0.5}`

	const q = 1.602e-19
	n := 1.0e21
	B := 0.5 // 改后的 B
	d := 0.0005

	rs := make([]hallEffectReading, 0, 3)
	for _, I := range []float64{0.001, 0.002, 0.003} {
		VH := (I * B) / (n * q * d)
		rs = append(rs, hallEffectReading{CurrentA: I, VoltageV: VH})
	}
	raw := mustJSON(t, rs)

	score, passed, _, err := scoreByExperiment("hall_effect", raw, []byte(dbTarget), []byte(userConfig))
	if err != nil || score != 100 || !passed {
		t.Fatalf("with user_config B=0.5: want 100/passed, got %d/%v (err=%v)", score, passed, err)
	}
}

func TestScoreByExperiment_MichelsonTheoreticalScores100(t *testing.T) {
	// λ = 2d / N -> d = N·λ / 2
	// λ=632.8nm, 取 N=100 -> d = 100*632.8e-9/2 = 3.164e-5 m
	target := `{"method":"wavelength_fit","wavelength_nm":632.8,"pass_score":60}`
	lambdaM := 632.8e-9

	rs := make([]michelsonReading, 0, 3)
	for _, N := range []int{100, 200, 50} {
		d := float64(N) * lambdaM / 2
		rs = append(rs, michelsonReading{N: N, MirrorM: d})
	}
	raw := mustJSON(t, rs)

	score, passed, _, err := scoreByExperiment("michelson", raw, []byte(target), nil)
	if err != nil || score != 100 || !passed {
		t.Fatalf("michelson theoretical: want 100/passed/nil, got %d/%v/%v", score, passed, err)
	}
}

func TestScoreByExperiment_MichelsonUserConfigOverrides(t *testing.T) {
	// DB target λ=632.8nm，前端改 λ=589.3nm 并按 589.3 生成读数
	dbTarget := `{"method":"wavelength_fit","wavelength_nm":632.8,"pass_score":60}`
	userConfig := `{"wavelength_nm":589.3}`

	lambdaM := 589.3e-9
	rs := make([]michelsonReading, 0, 3)
	for _, N := range []int{100, 200, 50} {
		d := float64(N) * lambdaM / 2
		rs = append(rs, michelsonReading{N: N, MirrorM: d})
	}
	raw := mustJSON(t, rs)

	score, passed, _, err := scoreByExperiment("michelson", raw, []byte(dbTarget), []byte(userConfig))
	if err != nil || score != 100 || !passed {
		t.Fatalf("with user_config λ=589.3: want 100/passed, got %d/%v (err=%v)", score, passed, err)
	}
}

func TestScoreMichelson_RejectsInvalidReadings(t *testing.T) {
	t1 := michelsonTarget{WavelengthNM: 632.8, PassScore: 60}
	for _, bad := range []michelsonReading{
		{N: 0, MirrorM: 1e-5},
		{N: -1, MirrorM: 1e-5},
		{N: 100, MirrorM: 0},
		{N: 100, MirrorM: -1},
		{N: 100, MirrorM: math.NaN()},
		{N: 100, MirrorM: math.Inf(1)},
	} {
		score, passed, _ := ScoreMichelson([]michelsonReading{bad}, t1)
		if score != 0 || passed {
			t.Fatalf("bad reading %+v: want 0/false, got %d/%v", bad, score, passed)
		}
	}
}
