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

// ===== 第二批新增实验评分测试 =====

func TestScoreByExperiment_PhotoelectricTheoreticalScores100(t *testing.T) {
	// V_s = (h/e)·ν - W/e
	// h=6.626e-34, e=1.602e-19, W=3.0e-19
	// 波长: 405, 436, 546, 577 nm -> ν = c/λ
	const h = 6.626e-34
	const e = 1.602e-19
	const W = 3.0e-19
	const c = 3.0e8

	wavelengths := []float64{405e-9, 436e-9, 546e-9, 577e-9}
	rs := make([]photoelectricReading, 0, len(wavelengths))
	for _, lambda := range wavelengths {
		nu := c / lambda
		Vs := (h/e)*nu - W/e
		rs = append(rs, photoelectricReading{FrequencyHz: nu, StopV: Vs})
	}
	raw := mustJSON(t, rs)
	target := `{"method":"planck_fit","planck_const":6.626e-34,"work_function":3.0e-19,"pass_score":60}`

	score, passed, _, err := scoreByExperiment("photoelectric", raw, []byte(target), nil)
	if err != nil || score != 100 || !passed {
		t.Fatalf("photoelectric theoretical: want 100/passed/nil, got %d/%v/%v", score, passed, err)
	}
}

func TestScorePhotoelectric_RejectsInsufficientReadings(t *testing.T) {
	t1 := photoelectricTarget{PlanckConst: 6.626e-34, WorkFunction: 3.0e-19, PassScore: 60}
	// 单条读数无法线性拟合
	score, passed, _ := ScorePhotoelectric([]photoelectricReading{
		{FrequencyHz: 5e14, StopV: 1.0},
	}, t1)
	if score != 0 || passed {
		t.Fatalf("single reading should score 0, got %d/%v", score, passed)
	}
}

func TestScoreByExperiment_FrankHertzTheoreticalScores100(t *testing.T) {
	// 模拟 I-V 曲线：激发电位 4.9V，峰位在 4.9, 9.8, 14.7, 19.6V
	// 每个周期：电流上升->峰->急剧下降(激发消耗)->谷->再上升
	target := `{"method":"excitation_fit","excitation_pot_v":4.9,"pass_score":60}`

	rs := make([]frankHertzReading, 0, 300)
	for i := 0; i <= 300; i++ {
		V := float64(i) * 0.1 // 0 ~ 30V, 步长 0.1V
		// 找当前 V 处于哪个周期
		cycle := int(V/4.9) // 0,1,2,3...
		phase := V - float64(cycle)*4.9 // 周期内相位 0~4.9

		// 电流模型：前半段上升，后半段骤降
		var I float64
		if phase < 4.0 {
			// 上升段
			I = 0.002 * (float64(cycle)*0.5 + phase/4.0)
		} else {
			// 骤降段（激发后电流下降）
			drop := (phase - 4.0) / 0.9 // 0~1
			I = 0.002 * (float64(cycle)*0.5 + 1.0) * (1 - drop*0.8)
		}
		if I < 0 {
			I = 0
		}
		rs = append(rs, frankHertzReading{VoltageV: V, CurrentA: I})
	}
	raw := mustJSON(t, rs)

	score, passed, _, err := scoreByExperiment("frank_hertz", raw, []byte(target), nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if score < 80 || !passed {
		t.Fatalf("frank-hertz theoretical: want >=80/passed, got %d/%v", score, passed)
	}
}

func TestScoreFrankHertz_RejectsInsufficientReadings(t *testing.T) {
	t1 := frankHertzTarget{ExcitationPotV: 4.9, PassScore: 60}
	// 数据太少无法找峰
	score, passed, _ := ScoreFrankHertz([]frankHertzReading{
		{VoltageV: 1, CurrentA: 0.001},
		{VoltageV: 2, CurrentA: 0.002},
	}, t1)
	if score != 0 || passed {
		t.Fatalf("insufficient readings should score 0, got %d/%v", score, passed)
	}
}

func TestScoreByExperiment_DiffractionGratingTheoreticalScores100(t *testing.T) {
	// d·sin(θ) = k·λ -> θ = arcsin(k·λ/d)
	// d=3.333e-6 m, λ=589.3nm
	const d = 3.333e-6
	const lambda = 589.3e-9

	target := `{"method":"grating_fit","grating_const_m":3.333e-6,"pass_score":60}`

	rs := make([]gratingReading, 0, 4)
	for _, k := range []int{1, 2, -1, -2} {
		sinTheta := math.Abs(float64(k)) * lambda / d
		if sinTheta >= 1 {
			continue
		}
		theta := math.Asin(sinTheta)
		rs = append(rs, gratingReading{OrderM: k, AngleRad: theta})
	}
	raw := mustJSON(t, rs)

	score, passed, _, err := scoreByExperiment("diffraction_grating", raw, []byte(target), nil)
	if err != nil || score != 100 || !passed {
		t.Fatalf("grating theoretical: want 100/passed/nil, got %d/%v/%v", score, passed, err)
	}
}

func TestScoreByExperiment_DiffractionGratingUserConfigOverrides(t *testing.T) {
	// DB target d=3.333e-6，前端改 d=2e-6 并按 d=2e-6 生成读数
	dbTarget := `{"method":"grating_fit","grating_const_m":3.333e-6,"pass_score":60}`
	userConfig := `{"grating_const_m":2.0e-6}`

	const d = 2.0e-6
	const lambda = 589.3e-9

	rs := make([]gratingReading, 0, 2)
	for _, k := range []int{1, 2} {
		sinTheta := float64(k) * lambda / d
		if sinTheta >= 1 {
			continue
		}
		rs = append(rs, gratingReading{OrderM: k, AngleRad: math.Asin(sinTheta)})
	}
	raw := mustJSON(t, rs)

	score, passed, _, err := scoreByExperiment("diffraction_grating", raw, []byte(dbTarget), []byte(userConfig))
	if err != nil || score != 100 || !passed {
		t.Fatalf("with user_config d=2e-6: want 100/passed, got %d/%v (err=%v)", score, passed, err)
	}
}

func TestScoreDiffractionGrating_RejectsInvalidReadings(t *testing.T) {
	t1 := gratingTarget{GratingConstM: 3.333e-6, PassScore: 60}
	for _, bad := range []gratingReading{
		{OrderM: 0, AngleRad: 0.1},
		{OrderM: 1, AngleRad: 0},
		{OrderM: 1, AngleRad: -1},
		{OrderM: 1, AngleRad: math.NaN()},
		{OrderM: 1, AngleRad: math.Pi}, // ≥ 90°
	} {
		score, passed, _ := ScoreDiffractionGrating([]gratingReading{bad}, t1)
		if score != 0 || passed {
			t.Fatalf("bad reading %+v: want 0/false, got %d/%v", bad, score, passed)
		}
	}
}

// ===== 第三批新增实验评分测试 =====

func TestScoreByExperiment_OilDropTheoreticalScores100(t *testing.T) {
	// 生成几组理论一致的油滴数据
	// q = n·e，n 取 1,2,3
	// 反推：给定 n，选 V 和 t 使公式自洽
	const e = 1.602e-19
	const eta = 1.83e-5
	const rho = 850.0
	const g = 9.8
	const plateDist = 0.006
	const distM = 0.002 // 下落 2mm

	target := `{"method":"oil_drop_fit","elementary":1.602e-19,"pass_score":60}`

	rs := make([]oilDropReading, 0, 4)
	for _, n := range []float64{1, 2, 3, 4} {
		q := n * e
		// 选一个下落时间 t，反推所需平衡电压 V
		// q = (6πηr·v·d) / V，其中 r = sqrt(l/(2ρgt²))，v = l/t
		// 先固定 t，算 r 和 v，再算 V
		t_fall := 10.0 + n*3.0 // 不同下落时间
		vTerm := distM / t_fall
		radius := math.Sqrt(distM / (2 * rho * g * t_fall * t_fall))
		V := (6 * math.Pi * eta * radius * vTerm * plateDist) / q
		rs = append(rs, oilDropReading{
			VoltageV:  V,
			DistanceM: distM,
			TimeS:     t_fall,
		})
	}
	raw := mustJSON(t, rs)

	score, passed, _, err := scoreByExperiment("oil_drop", raw, []byte(target), nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if score < 90 || !passed {
		t.Fatalf("oil_drop theoretical: want >=90/passed, got %d/%v", score, passed)
	}
}

func TestScoreOilDrop_RejectsInvalidReadings(t *testing.T) {
	t1 := oilDropTarget{Elementary: 1.602e-19, PassScore: 60}
	for _, bad := range []oilDropReading{
		{VoltageV: 0, DistanceM: 0.002, TimeS: 10},
		{VoltageV: 300, DistanceM: 0, TimeS: 10},
		{VoltageV: 300, DistanceM: 0.002, TimeS: 0},
		{VoltageV: math.NaN(), DistanceM: 0.002, TimeS: 10},
		{VoltageV: 300, DistanceM: 0.002, TimeS: math.Inf(1)},
	} {
		score, passed, _ := ScoreOilDrop([]oilDropReading{bad}, t1)
		if score != 0 || passed {
			t.Fatalf("bad reading %+v: want 0/false, got %d/%v", bad, score, passed)
		}
	}
}

func TestScoreByExperiment_PolarizationTheoreticalScores100(t *testing.T) {
	// I = I₀cos²(θ)，I₀ = 1.0
	target := `{"method":"malus_fit","pass_score":60}`

	rs := make([]polarizationReading, 0, 10)
	for deg := 0; deg <= 90; deg += 10 {
		theta := float64(deg) * math.Pi / 180
		I := math.Cos(theta) * math.Cos(theta)
		rs = append(rs, polarizationReading{AngleDeg: float64(deg), Intensity: I})
	}
	raw := mustJSON(t, rs)

	score, passed, _, err := scoreByExperiment("polarization", raw, []byte(target), nil)
	if err != nil || score != 100 || !passed {
		t.Fatalf("polarization theoretical: want 100/passed/nil, got %d/%v/%v", score, passed, err)
	}
}

func TestScorePolarization_RejectsInsufficientReadings(t *testing.T) {
	t1 := polarizationTarget{PassScore: 60}
	// 少于 3 条
	score, passed, _ := ScorePolarization([]polarizationReading{
		{AngleDeg: 0, Intensity: 1.0},
		{AngleDeg: 30, Intensity: 0.75},
	}, t1)
	if score != 0 || passed {
		t.Fatalf("insufficient readings should score 0, got %d/%v", score, passed)
	}
}

func TestScoreByExperiment_SoundSpeedTheoreticalScores100(t *testing.T) {
	// v = λ·f，f=37000Hz，v=343 m/s -> λ = 343/37000 ≈ 0.00927 m
	// λ/2 ≈ 0.00464 m，共振点间距 = λ/2
	const f = 37000.0
	const v = 343.0
	halfLambda := v / f / 2

	target := `{"method":"sound_speed_fit","speed_ms":343.0,"freq_hz":37000,"pass_score":60}`

	rs := make([]soundSpeedReading, 0, 8)
	for i := 1; i <= 8; i++ {
		pos := float64(i) * halfLambda
		rs = append(rs, soundSpeedReading{OrderN: i, PositionM: pos})
	}
	raw := mustJSON(t, rs)

	score, passed, _, err := scoreByExperiment("sound_speed", raw, []byte(target), nil)
	if err != nil || score != 100 || !passed {
		t.Fatalf("sound_speed theoretical: want 100/passed/nil, got %d/%v/%v", score, passed, err)
	}
}

func TestScoreByExperiment_SoundSpeedUserConfigOverrides(t *testing.T) {
	// DB target f=37000Hz，前端改 f=40000Hz 并按 f=40000 生成读数
	const f = 40000.0
	const v = 343.0
	halfLambda := v / f / 2

	dbTarget := `{"method":"sound_speed_fit","speed_ms":343.0,"freq_hz":37000,"pass_score":60}`
	userConfig := `{"freq_hz":40000}`

	rs := make([]soundSpeedReading, 0, 6)
	for i := 1; i <= 6; i++ {
		rs = append(rs, soundSpeedReading{OrderN: i, PositionM: float64(i) * halfLambda})
	}
	raw := mustJSON(t, rs)

	score, passed, _, err := scoreByExperiment("sound_speed", raw, []byte(dbTarget), []byte(userConfig))
	if err != nil || score != 100 || !passed {
		t.Fatalf("with user_config f=40000: want 100/passed, got %d/%v (err=%v)", score, passed, err)
	}
}

func TestScoreSoundSpeed_RejectsInvalidReadings(t *testing.T) {
	t1 := soundSpeedTarget{SpeedMS: 343.0, FreqHz: 37000, PassScore: 60}
	// 数据太少
	score, passed, _ := ScoreSoundSpeed([]soundSpeedReading{
		{OrderN: 1, PositionM: 0.005},
		{OrderN: 2, PositionM: 0.010},
	}, t1)
	if score != 0 || passed {
		t.Fatalf("insufficient readings should score 0, got %d/%v", score, passed)
	}
}
