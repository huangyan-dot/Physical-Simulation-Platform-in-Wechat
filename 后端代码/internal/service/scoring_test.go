package service

import (
	"encoding/json"
	"math"
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
		if _, _, _, err := scoreByExperiment("newton_ring", []byte(bad), []byte(target)); err == nil {
			t.Fatalf("readings=%s: want error, got nil", bad)
		}
	}
}

func TestScoreByExperiment_UnknownExperimentIsError(t *testing.T) {
	if _, _, _, err := scoreByExperiment("nope", []byte(`[{"x":1}]`), []byte(`{}`)); err == nil {
		t.Fatal("unknown experiment code: want error, got nil")
	}
}

func TestScoreByExperiment_NewtonRingTheoreticalScores100(t *testing.T) {
	target := `{"method":"least_squares_R","lens_radius_mm":855,"wavelength_nm":589.3,"pass_score":60}`
	rs := newtonReadings()
	raw := mustJSON(t, rs)
	score, passed, _, err := scoreByExperiment("newton_ring", raw, []byte(target))
	if err != nil || score != 100 || !passed {
		t.Fatalf("newton theoretical: want 100/passed/nil, got %d/%v/%v", score, passed, err)
	}
}

func TestScoreByExperiment_OscilloscopeDefaultsScore100(t *testing.T) {
	target := `{"method":"param_match","channels":{"CH1":{"A":2.0,"f":50},"CH2":{"A":1.5,"f":50}},"pass_score":60}`
	readings := `[{"channel":"CH1","f":50,"A":2.0},{"channel":"CH2","f":50,"A":1.5}]`
	score, passed, _, err := scoreByExperiment("oscilloscope", []byte(readings), []byte(target))
	if err != nil || score != 100 || !passed {
		t.Fatalf("scope defaults: want 100/passed/nil, got %d/%v/%v", score, passed, err)
	}
}

func TestScoreByExperiment_OscilloscopeChannelCaseMatters(t *testing.T) {
	// 通道键大小写必须与 target 一致（CH1），否则不匹配 -> 0 分。
	// 这锁死了数据组"channels 键用大写"的约定。
	target := `{"method":"param_match","channels":{"CH1":{"A":2.0,"f":50}},"pass_score":60}`
	readings := `[{"channel":"ch1","f":50,"A":2.0}]` // 小写 ch1 不匹配
	score, passed, _, err := scoreByExperiment("oscilloscope", []byte(readings), []byte(target))
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
	score, passed, _, err := scoreByExperiment("pendulum", []byte(readings), []byte(target))
	if err != nil || score != 100 || !passed {
		t.Fatalf("pendulum defaults: want 100/passed/nil, got %d/%v/%v", score, passed, err)
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
