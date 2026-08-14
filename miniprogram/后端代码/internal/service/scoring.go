package service

import (
	"encoding/json"
	"fmt"
	"math"
)

// ---- 评分目标结构（对应 experiments.target JSON） ----

type newtonTarget struct {
	Method        string  `json:"method"`          // least_squares_R
	LensRadiusMM  float64 `json:"lens_radius_mm"`  // 透镜曲率半径真值
	WavelengthNM  float64 `json:"wavelength_nm"`   // 光波长（nm）
	PassScore     int     `json:"pass_score"`      // 过关分数线
}

type scopeTarget struct {
	Method   string                   `json:"method"` // param_match
	Channels map[string]scopeChanTrue `json:"channels"`
	PassScore int                    `json:"pass_score"`
}

type scopeChanTrue struct {
	A float64 `json:"A"`
	F float64 `json:"f"`
}

type pendulumTarget struct {
	Method   string  `json:"method"` // gravity_fit
	Gravity  float64 `json:"gravity"`
	LengthM  float64 `json:"length_m"`
	PassScore int   `json:"pass_score"`
}

// ---- 提交读数结构 ----

type newtonReading struct {
	K int     `json:"k"`
	R float64 `json:"r"`
	// CalcR 学生自己用平方差法算出的曲率半径（mm）。
	// 与单摆的 CalcG 同理：评的是学生的计算结果，而不是后端替他拟合出来的值。
	// 只要有一条读数带了 CalcR>0 就按它评分；全都没带（老客户端）才回落到最小二乘拟合。
	CalcR float64 `json:"calc_r"`
	// Rounds 学生一共测了几轮，SelectedCount 是他用来算 R 的轮数。
	// 只记录不评分：多测几轮不加分，但教师能看出他的取舍过程。
	Rounds        int `json:"rounds"`
	SelectedCount int `json:"selected_count"`
}

type scopeReading struct {
	Channel string  `json:"channel"`
	F       float64 `json:"f"`
	A       float64 `json:"A"`
}

type pendulumReading struct {
	Period float64 `json:"period"`
	CalcG  float64 `json:"calc_g"`
	// 学生实验所在地区（县级）。有合理的纬度/海拔时，评分基准改用当地 g，
	// 学生算出的就是「当地重力加速度」而不是一个全国统一的常数。
	Latitude  float64 `json:"latitude"`
	Altitude  float64 `json:"altitude"`
	HasRegion bool    `json:"has_region"`
	// Rounds 学生一共做了几轮，SelectedCount 是他勾选来求平均的轮数。
	// 只记录不评分：多做几轮不加分，但教师能看出他的取舍过程。
	Rounds        int `json:"rounds"`
	SelectedCount int `json:"selected_count"`
}

// scoreFromRelErr 把平均相对误差映射到 0~100 分。
// 误差 0 -> 100；误差越大分越低，封顶 0。乘数 5：8% 误差约 60 分（刚过线）。
func scoreFromRelErr(rel float64) int {
	score := 100.0 * (1 - rel*5)
	if score < 0 {
		score = 0
	}
	return int(math.Round(score))
}

// pendulumScoreFromRelErr 单摆专用的误差—分数曲线。
//
// 本实验起表与停表都由装置自动触发，没有人工反应时间误差，
// 规范操作（θ≤5°、累积法、多轮取平均）算出的误差本就在 0.1%~0.3% 量级。
// 若沿用「1%~3% 给满分」的宽容标准，忘加 d/2（约 1.1%）、摆角开到 20°（约 1.4%）
// 这类真实的概念性错误也会拿满分，分数就完全失去区分度。因此按自动计时的实际
// 精度收紧：
//
//	≤0.3% -> 100     规范操作应得的满分区间
//	 0.5% -> 97
//	 1%   -> 90       漏乘 d/2 之类的小错
//	 2%   -> 75
//	 3%   -> 60       及格线
//	 5%   -> 30
//	≥8%   -> 0
//
// 段内线性插值，整体单调不增。通用的 scoreFromRelErr 仍供牛顿环/示波器使用。
func pendulumScoreFromRelErr(rel float64) int {
	if rel < 0 {
		rel = -rel
	}
	// 分段端点：{误差, 该点分数}
	pts := [][2]float64{
		{0.000, 100},
		{0.003, 100},
		{0.005, 97},
		{0.010, 90},
		{0.020, 75},
		{0.030, 60},
		{0.050, 30},
		{0.080, 0},
	}
	if rel >= 0.08 {
		return 0
	}
	for i := 0; i < len(pts)-1; i++ {
		x0, y0 := pts[i][0], pts[i][1]
		x1, y1 := pts[i+1][0], pts[i+1][1]
		if rel <= x1 {
			if x1 == x0 {
				return int(math.Round(y1))
			}
			y := y0 + (y1-y0)*(rel-x0)/(x1-x0)
			return int(math.Round(y))
		}
	}
	return 0
}

// newtonScoreFromRelErr 牛顿环专用的误差—分数曲线。
//
// 与单摆同样的理由：本模拟省去了调焦与光路调节，叉丝对准外切线的量化步长是
// 0.005mm，规范操作（左右两侧各对准同一级环、取两个相差较大的环级用平方差法）
// 算出的 R 误差本就在 0.1%~0.5% 量级。若沿用通用的 scoreFromRelErr
// （8% 误差还给 60 分），下面这些真实的概念性错误也能轻松及格：
//
//	用半径代直径（差 4 倍）、λ 忘了 nm→mm、只用单环 r²=kλR 而不作差、
//	两个环级挑得太近导致差值被读数误差放大。
//
// 因此按本模拟的实际精度收紧：
//
//	≤0.5% -> 100    规范操作应得的满分区间
//	 1%   -> 95
//	 2%   -> 88
//	 3%   -> 80
//	 5%   -> 65
//	 8%   -> 50     低于 pass_score(60)，概念性错误不再及格
//	12%   -> 20
//	≥15%  -> 0
//
// 段内线性插值，整体单调不增。通用的 scoreFromRelErr 仍供示波器使用。
func newtonScoreFromRelErr(rel float64) int {
	if rel < 0 {
		rel = -rel
	}
	pts := [][2]float64{
		{0.000, 100},
		{0.005, 100},
		{0.010, 95},
		{0.020, 88},
		{0.030, 80},
		{0.050, 65},
		{0.080, 50},
		{0.120, 20},
		{0.150, 0},
	}
	if rel >= 0.15 {
		return 0
	}
	for i := 0; i < len(pts)-1; i++ {
		x0, y0 := pts[i][0], pts[i][1]
		x1, y1 := pts[i+1][0], pts[i+1][1]
		if rel <= x1 {
			if x1 == x0 {
				return int(math.Round(y1))
			}
			return int(math.Round(y0 + (y1-y0)*(rel-x0)/(x1-x0)))
		}
	}
	return 0
}

// ScoreNewtonRing 牛顿环：评学生用平方差法算出的曲率半径 R。
//
// 优先按学生填的 CalcR 评分（与单摆评 CalcG 一致——考的是他会不会算，
// 而不是后端替他拟合）。读数里没有 CalcR 时回落到旧行为：
// r² = k·λ·R 过原点最小二乘求斜率 m=λ·R，反解 R。
func ScoreNewtonRing(readings []newtonReading, t newtonTarget) (score int, passed bool, detail string) {
	if len(readings) == 0 || t.LensRadiusMM <= 0 || t.WavelengthNM <= 0 {
		return 0, false, "读数或目标参数无效"
	}

	// ---- 学生自己算出的 R ----
	for _, r := range readings {
		if r.CalcR > 0 {
			rel := math.Abs(r.CalcR-t.LensRadiusMM) / t.LensRadiusMM
			score = newtonScoreFromRelErr(rel)
			passed = score >= t.PassScore
			detail = fmt.Sprintf("R=%.1fmm(真值%.0fmm), 相对误差%.2f%%, 采用%d/%d轮",
				r.CalcR, t.LensRadiusMM, rel*100, r.SelectedCount, r.Rounds)
			return
		}
	}

	// ---- 回落：后端最小二乘拟合 ----
	lamMM := t.WavelengthNM * 1e-6 // nm -> mm
	var num, den float64
	for _, r := range readings {
		if r.K <= 0 || r.R <= 0 {
			continue
		}
		num += float64(r.K) * r.R * r.R // x·y, x=k, y=r²
		den += float64(r.K) * float64(r.K)
	}
	if den == 0 {
		return 0, false, "无有效读数"
	}
	m := num / den
	rest := m / lamMM
	rel := math.Abs(rest-t.LensRadiusMM) / t.LensRadiusMM
	score = newtonScoreFromRelErr(rel)
	passed = score >= t.PassScore
	detail = fmt.Sprintf("R̂=%.2fmm(真值%.0fmm, 后端拟合), 相对误差%.2f%%", rest, t.LensRadiusMM, rel*100)
	return
}

// ScoreOscilloscope 示波器：测得的各通道 f/A 与真值比对，平均相对误差。
func ScoreOscilloscope(readings []scopeReading, t scopeTarget) (score int, passed bool, detail string) {
	if len(readings) == 0 || len(t.Channels) == 0 {
		return 0, false, "读数或目标参数无效"
	}
	var sumRel float64
	var n int
	for _, r := range readings {
		tg, ok := t.Channels[r.Channel]
		if !ok {
			continue
		}
		if tg.F > 0 {
			sumRel += math.Abs(r.F-tg.F) / tg.F
			n++
		}
		if tg.A > 0 {
			sumRel += math.Abs(r.A-tg.A) / tg.A
			n++
		}
	}
	if n == 0 {
		return 0, false, "无有效读数"
	}
	rel := sumRel / float64(n)
	score = scoreFromRelErr(rel)
	passed = score >= t.PassScore
	detail = fmt.Sprintf("平均相对误差%.2f%%", rel*100)
	return
}

// ScorePendulum 单摆：以学生自己算出并确认的 g 平均值与当地真值比对。
//
// 评分基准优先取学生所在地区的当地 g（由上报的纬度/海拔按 WGS84 正常重力公式算出）；
// 未上报或坐标不合理时回退到实验配置里的 target.gravity。
//
// readings[0].calc_g 是学生手填的最终平均值（不是程序替他算的），
// rounds/selected 只作记录，不参与评分——评分只看结果准不准。
func ScorePendulum(readings []pendulumReading, t pendulumTarget) (score int, passed bool, detail string) {
	if len(readings) == 0 || t.Gravity <= 0 {
		return 0, false, "读数或目标参数无效"
	}
	r := readings[0]
	if r.CalcG <= 0 {
		return 0, false, "未填写重力加速度平均值"
	}

	gTrue := t.Gravity
	basis := "配置真值"
	if r.HasRegion && plausibleLocation(r.Latitude, r.Altitude) {
		gTrue = LocalGravity(r.Latitude, r.Altitude)
		basis = fmt.Sprintf("当地(纬度%.2f°,海拔%.0fm)", r.Latitude, r.Altitude)
	}

	rel := math.Abs(r.CalcG-gTrue) / gTrue
	score = pendulumScoreFromRelErr(rel)
	passed = score >= t.PassScore
	detail = fmt.Sprintf("g=%.3f(%s%.3f), 相对误差%.2f%%, 采用%d/%d轮",
		r.CalcG, basis, gTrue, rel*100, r.SelectedCount, r.Rounds)
	return
}

// scoreByExperiment 按实验 code 解析读数与目标并评分
func scoreByExperiment(code string, readingsRaw, targetRaw []byte) (score int, passed bool, detail string, err error) {
	// 契约 §12：readings 非法 -> 400。空数组或非 JSON 数组视为非法，评分前拦截。
	var probe []json.RawMessage
	if uerr := json.Unmarshal(readingsRaw, &probe); uerr != nil {
		err = fmt.Errorf("readings 不是合法数组: %w", uerr)
		return
	}
	if len(probe) == 0 {
		err = fmt.Errorf("readings 不能为空")
		return
	}

	switch code {
	case "newton_ring":
		var rs []newtonReading
		var t newtonTarget
		if err = unmarshalBoth(readingsRaw, targetRaw, &rs, &t); err != nil {
			return
		}
		score, passed, detail = ScoreNewtonRing(rs, t)
	case "oscilloscope":
		var rs []scopeReading
		var t scopeTarget
		if err = unmarshalBoth(readingsRaw, targetRaw, &rs, &t); err != nil {
			return
		}
		score, passed, detail = ScoreOscilloscope(rs, t)
	case "pendulum":
		var rs []pendulumReading
		var t pendulumTarget
		if err = unmarshalBoth(readingsRaw, targetRaw, &rs, &t); err != nil {
			return
		}
		score, passed, detail = ScorePendulum(rs, t)
	default:
		err = fmt.Errorf("unknown experiment: %s", code)
	}
	return
}

// unmarshalBoth 同时解析读数与目标
func unmarshalBoth(readingsRaw, targetRaw []byte, rs interface{}, t interface{}) error {
	if err := json.Unmarshal(readingsRaw, rs); err != nil {
		return fmt.Errorf("解析读数失败: %w", err)
	}
	if err := json.Unmarshal(targetRaw, t); err != nil {
		return fmt.Errorf("解析目标失败: %w", err)
	}
	return nil
}
