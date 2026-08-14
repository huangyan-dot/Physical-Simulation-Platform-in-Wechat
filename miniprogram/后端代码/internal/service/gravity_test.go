package service

import (
	"math"
	"testing"

	"physics-lab/backend/internal/model"
)

// TestLocalGravityKnownPoints 校验正常重力公式在公认参考点上的取值。
// 这三个值同时也是前端 utils/region.js 的校验点，两边必须一致。
func TestLocalGravityKnownPoints(t *testing.T) {
	cases := []struct {
		name string
		lat  float64
		alt  float64
		want float64
	}{
		{"赤道海平面", 0, 0, 9.78033},
		{"45度海平面", 45, 0, 9.80620},
		{"极点海平面", 90, 0, 9.83219},
	}
	for _, c := range cases {
		got := LocalGravity(c.lat, c.alt)
		if math.Abs(got-c.want) > 5e-5 {
			t.Errorf("%s: LocalGravity(%.0f,%.0f)=%.5f, want %.5f", c.name, c.lat, c.alt, got, c.want)
		}
	}
}

// TestLocalGravityMonotonic g 应随纬度升高而增大、随海拔升高而减小。
func TestLocalGravityMonotonic(t *testing.T) {
	prev := LocalGravity(0, 0)
	for lat := 5.0; lat <= 90; lat += 5 {
		g := LocalGravity(lat, 0)
		if g <= prev {
			t.Fatalf("g 应随纬度单调增：lat=%.0f g=%.6f 不大于前一个 %.6f", lat, g, prev)
		}
		prev = g
	}
	// 拉萨（29.65°N, 3650m）应明显小于同纬度海平面
	sea := LocalGravity(29.65, 0)
	lhasa := LocalGravity(29.65, 3650)
	if !(lhasa < sea) {
		t.Fatalf("海拔升高 g 应减小：3650m=%.5f 海平面=%.5f", lhasa, sea)
	}
	// 3650m 的自由空气改正约 0.0113 m/s²
	if d := sea - lhasa; math.Abs(d-0.01126) > 1e-4 {
		t.Errorf("自由空气改正量 %.5f，期望约 0.01126", d)
	}
}

// TestScorePendulumUsesLocalGravity 上报定位时应按当地 g 评分：
// 在拉萨算出当地 g 的学生得满分，而算出「标准」9.8 的学生要扣分。
func TestScorePendulumUsesLocalGravity(t *testing.T) {
	target := pendulumTarget{Method: "gravity_fit", Gravity: 9.8, LengthM: 1.0, PassScore: 60}
	lhasaG := LocalGravity(29.65, 3650) // ≈9.7796

	// 学生按当地 g 算对 -> 满分
	score, passed, detail := ScorePendulum([]pendulumReading{{
		Period: 2.0, CalcG: math.Round(lhasaG*1000) / 1000,
		Latitude: 29.65, Altitude: 3650, HasRegion: true,
	}}, target)
	if score != 100 || !passed {
		t.Errorf("按当地 g 算对应为 100 分，得 %d (passed=%v) %s", score, passed, detail)
	}

	// 未上报定位 -> 回退配置真值 9.8，此时报 9.8 才满分
	score2, _, _ := ScorePendulum([]pendulumReading{{Period: 2.0, CalcG: 9.8}}, target)
	if score2 != 100 {
		t.Errorf("未上报定位时应按配置真值 9.8 评分，得 %d", score2)
	}

	// 坐标不合理（海拔 99999m）-> 视为无效，回退配置真值
	score3, _, _ := ScorePendulum([]pendulumReading{{
		Period: 2.0, CalcG: 9.8, Latitude: 29.65, Altitude: 99999, HasRegion: true,
	}}, target)
	if score3 != 100 {
		t.Errorf("越界海拔应回退配置真值，得 %d", score3)
	}
}

// TestComboScore 综合得分加权：默认 60:40，教师可调。
func TestComboScore(t *testing.T) {
	cases := []struct {
		data, quiz, weight, want int
	}{
		{100, 100, 60, 100},
		{100, 0, 60, 60},  // 只做好测量
		{0, 100, 60, 40},  // 只做好自测
		{90, 70, 60, 82},  // 90*0.6 + 70*0.4 = 82
		{90, 70, 40, 78},  // 教师改成 40:60 -> 90*0.4 + 70*0.6 = 78
		{85, 60, 100, 85}, // 全看测量
		{75, 80, 50, 78},  // 77.5 -> 四舍五入 78
	}
	for _, c := range cases {
		if got := comboScore(c.data, c.quiz, c.weight); got != c.want {
			t.Errorf("comboScore(%d,%d,%d)=%d, want %d", c.data, c.quiz, c.weight, got, c.want)
		}
	}
	// 权重 0 = 未设置（老数据/迁移前的行），必须回落到默认 60:40，
	// 不能当成「测量占 0%」，否则学生的测量分会被整段丢掉。
	if got := comboScore(90, 70, 0); got != 82 {
		t.Errorf("权重 0 应按默认 60:40 计（=82），得 %d", got)
	}
	if got := comboScore(90, 70, -20); got != 82 {
		t.Errorf("负权重应按默认 60:40 计（=82），得 %d", got)
	}
	// 越界权重应被夹紧，不产生离谱分数
	if got := comboScore(100, 0, 150); got != 100 {
		t.Errorf("权重>100 应夹紧到 100，得 %d", got)
	}
}

// TestEffectiveDataWeightDefault 默认比例必须是测量数据 60% : 自测 40%
func TestEffectiveDataWeightDefault(t *testing.T) {
	if model.DefaultDataWeight != 60 {
		t.Fatalf("默认数据权重应为 60，得 %d", model.DefaultDataWeight)
	}
	if got := effectiveDataWeight(0); got != 60 {
		t.Errorf("未设置(0) 应回落到 60，得 %d", got)
	}
	if got := effectiveDataWeight(40); got != 40 {
		t.Errorf("教师显式设 40 应保持 40，得 %d", got)
	}
	if got := effectiveDataWeight(120); got != 100 {
		t.Errorf("超过 100 应夹紧到 100，得 %d", got)
	}
}

// TestPendulumScoreCurve 单摆误差—分数曲线。
// 自动计时没有人工反应误差，规范操作误差在 0.3% 以内，故满分区间收紧到 0.3%。
func TestPendulumScoreCurve(t *testing.T) {
	cases := []struct {
		rel  float64
		want int
	}{
		{0.000, 100},
		{0.001, 100}, // 规范操作的典型水平
		{0.003, 100}, // 满分区间上界
		{0.005, 97},
		{0.010, 90}, // 漏乘 d/2 之类的小错
		{0.020, 75},
		{0.030, 60}, // 及格线
		{0.050, 30},
		{0.080, 0},
		{0.500, 0},
	}
	for _, c := range cases {
		if got := pendulumScoreFromRelErr(c.rel); got != c.want {
			t.Errorf("pendulumScoreFromRelErr(%.4f)=%d, want %d", c.rel, got, c.want)
		}
	}
	// 负误差按绝对值处理
	if pendulumScoreFromRelErr(-0.003) != 100 {
		t.Error("负误差应取绝对值")
	}
	// 单调不增
	prev := 101
	for rel := 0.0; rel <= 0.10; rel += 0.0005 {
		s := pendulumScoreFromRelErr(rel)
		if s > prev {
			t.Fatalf("曲线在 rel=%.4f 处回升: %d > %d", rel, s, prev)
		}
		prev = s
	}
	// 区分度：概念性错误必须明显低于规范操作
	if pendulumScoreFromRelErr(0.0012) <= pendulumScoreFromRelErr(0.011) {
		t.Error("规范操作(0.12%)应明显高于漏乘 d/2(1.1%)")
	}
}

// TestScorePendulumRejectsMissingG 未填平均值不得给分
func TestScorePendulumRejectsMissingG(t *testing.T) {
	target := pendulumTarget{Method: "gravity_fit", Gravity: 9.8, LengthM: 1.0, PassScore: 60}
	score, passed, _ := ScorePendulum([]pendulumReading{{Period: 2.0, CalcG: 0}}, target)
	if score != 0 || passed {
		t.Errorf("未填 g 应 0 分不及格，得 %d passed=%v", score, passed)
	}
}
