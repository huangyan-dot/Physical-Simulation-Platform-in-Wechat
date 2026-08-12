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

// ---- 新增实验评分目标结构 ----

type youngModulusTarget struct {
	Method         string  `json:"method"`           // young_fit
	YoungModulusPa float64 `json:"young_modulus_pa"` // 杨氏模量真值 (Pa)
	ForceN         float64 `json:"force_n"`          // 单个砝码重力 (N)
	LengthM        float64 `json:"length_m"`         // 金属丝原长 (m)
	DiameterM      float64 `json:"diameter_m"`       // 金属丝直径 (m)
	LeverArmM      float64 `json:"lever_arm_m"`      // 光杠杆臂长 (m)
	MirrorDistM    float64 `json:"mirror_dist_m"`    // 镜尺距离 (m)
	PassScore      int     `json:"pass_score"`
}

type hallEffectTarget struct {
	Method      string  `json:"method"`         // hall_fit
	BFieldT     float64 `json:"b_field_t"`      // 磁感应强度 (T)
	ThicknessM  float64 `json:"thickness_m"`    // 样品厚度 (m)
	CarrierConc float64 `json:"carrier_conc"`   // 载流子浓度真值 (m^-3)
	PassScore   int     `json:"pass_score"`
}

type michelsonTarget struct {
	Method       string  `json:"method"`         // wavelength_fit
	WavelengthNM float64 `json:"wavelength_nm"`  // 光波真值 (nm)
	PassScore    int     `json:"pass_score"`
}

// ---- 提交读数结构 ----

type newtonReading struct {
	K int     `json:"k"`
	R float64 `json:"r"`
}

type scopeReading struct {
	Channel string  `json:"channel"`
	F       float64 `json:"f"`
	A       float64 `json:"A"`
}

type pendulumReading struct {
	Period float64 `json:"period"`
	CalcG  float64 `json:"calc_g"`
}

// ---- 新增实验读数结构 ----

type youngModulusReading struct {
	Load    int     `json:"load"`    // 砝码序号（0,1,2...）
	Reading float64 `json:"reading"` // 望远镜十字丝读数 (mm)
}

type hallEffectReading struct {
	CurrentA float64 `json:"current_a"` // 工作电流 (A)
	VoltageV float64 `json:"voltage_v"` // 霍尔电压 (V)
}

type michelsonReading struct {
	N       int     `json:"n"`        // 冒出/陷入条纹数
	MirrorM float64 `json:"mirror_m"` // 反射镜移动距离 (m)
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

// ScoreNewtonRing 牛顿环：r² = k·λ·R，过原点最小二乘求斜率 m=λ·R，反解 R 与真值比。
func ScoreNewtonRing(readings []newtonReading, t newtonTarget) (score int, passed bool, detail string) {
	if len(readings) == 0 || t.LensRadiusMM <= 0 || t.WavelengthNM <= 0 {
		return 0, false, "读数或目标参数无效"
	}
	lamMM := t.WavelengthNM * 1e-6 // nm -> mm
	var num, den float64
	validCount := 0
	for _, r := range readings {
		// 数据校验：k 须为正整数，r 须为正有限值
		if r.K <= 0 || r.R <= 0 || math.IsNaN(r.R) || math.IsInf(r.R, 0) {
			continue
		}
		num += float64(r.K) * r.R * r.R // x·y, x=k, y=r²
		den += float64(r.K) * float64(r.K)
		validCount++
	}
	if den == 0 || validCount == 0 {
		return 0, false, "无有效读数"
	}
	m := num / den
	rest := m / lamMM
	rel := math.Abs(rest-t.LensRadiusMM) / t.LensRadiusMM
	score = scoreFromRelErr(rel)
	passed = score >= t.PassScore
	detail = fmt.Sprintf("R̂=%.2fmm(真值%.0fmm), 相对误差%.2f%%", rest, t.LensRadiusMM, rel*100)
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
		// 数据校验：频率/振幅须为正有限值
		if math.IsNaN(r.F) || math.IsInf(r.F, 0) || math.IsNaN(r.A) || math.IsInf(r.A, 0) {
			continue
		}
		tg, ok := t.Channels[r.Channel]
		if !ok {
			continue
		}
		if tg.F > 0 && r.F > 0 {
			sumRel += math.Abs(r.F-tg.F) / tg.F
			n++
		}
		if tg.A > 0 && r.A > 0 {
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

// ScorePendulum 单摆：由测得周期反算 g，与真值比对。
func ScorePendulum(readings []pendulumReading, t pendulumTarget) (score int, passed bool, detail string) {
	if len(readings) == 0 || t.Gravity <= 0 {
		return 0, false, "读数或目标参数无效"
	}
	r := readings[0]
	// 数据校验：calc_g 须为正有限值
	if r.CalcG <= 0 || math.IsNaN(r.CalcG) || math.IsInf(r.CalcG, 0) {
		return 0, false, "calc_g 无效"
	}
	rel := math.Abs(r.CalcG-t.Gravity) / t.Gravity
	score = scoreFromRelErr(rel)
	passed = score >= t.PassScore
	detail = fmt.Sprintf("g=%.2f(真值%.2f), 相对误差%.2f%%", r.CalcG, t.Gravity, rel*100)
	return
}

// ScoreYoungModulus 杨氏模量：逐差法求光杠杆放大后的伸长量 Δn，
// 反算杨氏模量 E = (8·F·n·L·D) / (π·d²·b·Δn)，与真值 E_std 比对。
// 读数为望远镜十字丝读数（mm），load 为砝码序号。
func ScoreYoungModulus(readings []youngModulusReading, t youngModulusTarget) (score int, passed bool, detail string) {
	if len(readings) < 2 || t.YoungModulusPa <= 0 || t.ForceN <= 0 ||
		t.LengthM <= 0 || t.DiameterM <= 0 || t.LeverArmM <= 0 || t.MirrorDistM <= 0 {
		return 0, false, "读数或目标参数无效"
	}
	// 过滤无效读数
	var valid []youngModulusReading
	for _, r := range readings {
		if r.Reading <= 0 || math.IsNaN(r.Reading) || math.IsInf(r.Reading, 0) {
			continue
		}
		valid = append(valid, r)
	}
	if len(valid) < 2 {
		return 0, false, "无有效读数"
	}

	// 逐差法：前后两组对应项之差，取绝对值
	n := len(valid) / 2
	if n < 1 {
		n = 1
	}
	var sumDelta float64
	count := 0
	for i := 0; i < n && i+n < len(valid); i++ {
		d := valid[i+n].Reading - valid[i].Reading
		if d > 0 {
			sumDelta += d
			count++
		}
	}
	if count == 0 {
		return 0, false, "逐差法无有效差值"
	}
	avgDeltaMM := sumDelta / float64(count) // mm，光杠杆放大后每组平均伸长

	// E = (8·F·n·L·D) / (π·d²·b·Δn_m)
	// F: 单砝码重力(N), n: 逐差跨数, L: 丝长(m), D: 镜尺距离(m),
	// d: 丝直径(m), b: 光杠杆臂长(m), Δn_m: 放大伸长(m)
	deltaM := avgDeltaMM * 1e-3 // mm -> m
	if deltaM <= 0 {
		return 0, false, "伸长量无效"
	}
	E := (8.0 * t.ForceN * float64(n) * t.LengthM * t.MirrorDistM) /
		(math.Pi * t.DiameterM * t.DiameterM * t.LeverArmM * deltaM) // Pa

	rel := math.Abs(E-t.YoungModulusPa) / t.YoungModulusPa
	score = scoreFromRelErr(rel)
	passed = score >= t.PassScore
	detail = fmt.Sprintf("E=%.2e Pa(真值%.2e Pa), 相对误差%.2f%%", E, t.YoungModulusPa, rel*100)
	return
}

// ScoreHallEffect 霍尔效应：由霍尔电压反算载流子浓度 n = (I·B) / (q·d·V_H)，
// 与真值比对。读数为多组 (I, V_H) 数据。
func ScoreHallEffect(readings []hallEffectReading, t hallEffectTarget) (score int, passed bool, detail string) {
	if len(readings) == 0 || t.BFieldT <= 0 || t.ThicknessM <= 0 || t.CarrierConc <= 0 {
		return 0, false, "读数或目标参数无效"
	}
	const q = 1.602e-19 // 电子电荷 (C)

	var sumConc float64
	count := 0
	for _, r := range readings {
		// 数据校验：电流和电压须为正有限值
		if r.CurrentA <= 0 || r.VoltageV <= 0 ||
			math.IsNaN(r.CurrentA) || math.IsInf(r.CurrentA, 0) ||
			math.IsNaN(r.VoltageV) || math.IsInf(r.VoltageV, 0) {
			continue
		}
		// n = (I·B) / (q·d·V_H)
		n := (r.CurrentA * t.BFieldT) / (q * t.ThicknessM * r.VoltageV)
		if n > 0 && !math.IsNaN(n) && !math.IsInf(n, 0) {
			sumConc += n
			count++
		}
	}
	if count == 0 {
		return 0, false, "无有效读数"
	}
	avgConc := sumConc / float64(count)
	rel := math.Abs(avgConc-t.CarrierConc) / t.CarrierConc
	score = scoreFromRelErr(rel)
	passed = score >= t.PassScore
	detail = fmt.Sprintf("n=%.3e m⁻³(真值%.3e m⁻³), 相对误差%.2f%%", avgConc, t.CarrierConc, rel*100)
	return
}

// ScoreMichelson 迈克尔逊干涉仪：λ = 2·d / N，由条纹数 N 和反射镜移动距离 d
// 反算波长，与真值比对。读数为多组 (N, d) 数据，取平均波长。
func ScoreMichelson(readings []michelsonReading, t michelsonTarget) (score int, passed bool, detail string) {
	if len(readings) == 0 || t.WavelengthNM <= 0 {
		return 0, false, "读数或目标参数无效"
	}
	var sumLambda float64
	count := 0
	for _, r := range readings {
		// 数据校验：N 须为正整数，d 须为正有限值
		if r.N <= 0 || r.MirrorM <= 0 ||
			math.IsNaN(r.MirrorM) || math.IsInf(r.MirrorM, 0) {
			continue
		}
		// λ = 2d / N (m) -> nm
		lambdaNM := (2.0 * r.MirrorM / float64(r.N)) * 1e9
		if lambdaNM > 0 && !math.IsNaN(lambdaNM) && !math.IsInf(lambdaNM, 0) {
			sumLambda += lambdaNM
			count++
		}
	}
	if count == 0 {
		return 0, false, "无有效读数"
	}
	avgLambda := sumLambda / float64(count) // nm
	rel := math.Abs(avgLambda-t.WavelengthNM) / t.WavelengthNM
	score = scoreFromRelErr(rel)
	passed = score >= t.PassScore
	detail = fmt.Sprintf("λ=%.2fnm(真值%.2fnm), 相对误差%.2f%%", avgLambda, t.WavelengthNM, rel*100)
	return
}

// scoreByExperiment 按实验 code 解析读数与目标并评分。
// userConfigRaw 为前端提交时携带的实际实验参数（可选），非空时覆盖 DB target 中对应的真值，
// 解决"滑块改变仿真真值但评分 target 固定"的问题（契约 §12 v0.5）。
func scoreByExperiment(code string, readingsRaw, targetRaw, userConfigRaw []byte) (score int, passed bool, detail string, err error) {
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
		applyNewtonConfig(&t, userConfigRaw)
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
		applyPendulumConfig(&t, userConfigRaw)
		score, passed, detail = ScorePendulum(rs, t)
	case "young_modulus":
		var rs []youngModulusReading
		var t youngModulusTarget
		if err = unmarshalBoth(readingsRaw, targetRaw, &rs, &t); err != nil {
			return
		}
		applyYoungModulusConfig(&t, userConfigRaw)
		score, passed, detail = ScoreYoungModulus(rs, t)
	case "hall_effect":
		var rs []hallEffectReading
		var t hallEffectTarget
		if err = unmarshalBoth(readingsRaw, targetRaw, &rs, &t); err != nil {
			return
		}
		applyHallEffectConfig(&t, userConfigRaw)
		score, passed, detail = ScoreHallEffect(rs, t)
	case "michelson":
		var rs []michelsonReading
		var t michelsonTarget
		if err = unmarshalBoth(readingsRaw, targetRaw, &rs, &t); err != nil {
			return
		}
		applyMichelsonConfig(&t, userConfigRaw)
		score, passed, detail = ScoreMichelson(rs, t)
	default:
		err = fmt.Errorf("unknown experiment: %s", code)
	}
	return
}

// applyNewtonConfig 用前端提交的实际实验参数覆盖 DB target 中的真值（滑块问题修复）。
func applyNewtonConfig(t *newtonTarget, userConfigRaw []byte) {
	if len(userConfigRaw) == 0 {
		return
	}
	var uc struct {
		LensRadiusMM *float64 `json:"lens_radius_mm"`
		WavelengthNM *float64 `json:"wavelength_nm"`
	}
	if json.Unmarshal(userConfigRaw, &uc) == nil {
		if uc.LensRadiusMM != nil && *uc.LensRadiusMM > 0 {
			t.LensRadiusMM = *uc.LensRadiusMM
		}
		if uc.WavelengthNM != nil && *uc.WavelengthNM > 0 {
			t.WavelengthNM = *uc.WavelengthNM
		}
	}
}

// applyPendulumConfig 用前端提交的实际实验参数覆盖 DB target 中的真值（滑块问题修复）。
func applyPendulumConfig(t *pendulumTarget, userConfigRaw []byte) {
	if len(userConfigRaw) == 0 {
		return
	}
	var uc struct {
		Gravity *float64 `json:"gravity"`
		LengthM *float64 `json:"length_m"`
	}
	if json.Unmarshal(userConfigRaw, &uc) == nil {
		if uc.Gravity != nil && *uc.Gravity > 0 {
			t.Gravity = *uc.Gravity
		}
		if uc.LengthM != nil && *uc.LengthM > 0 {
			t.LengthM = *uc.LengthM
		}
	}
}

// applyYoungModulusConfig 用前端提交的实际实验参数覆盖 DB target 中的真值。
func applyYoungModulusConfig(t *youngModulusTarget, userConfigRaw []byte) {
	if len(userConfigRaw) == 0 {
		return
	}
	var uc struct {
		ForceN      *float64 `json:"force_n"`
		LengthM     *float64 `json:"length_m"`
		DiameterM   *float64 `json:"diameter_m"`
		LeverArmM   *float64 `json:"lever_arm_m"`
		MirrorDistM *float64 `json:"mirror_dist_m"`
	}
	if json.Unmarshal(userConfigRaw, &uc) == nil {
		if uc.ForceN != nil && *uc.ForceN > 0 {
			t.ForceN = *uc.ForceN
		}
		if uc.LengthM != nil && *uc.LengthM > 0 {
			t.LengthM = *uc.LengthM
		}
		if uc.DiameterM != nil && *uc.DiameterM > 0 {
			t.DiameterM = *uc.DiameterM
		}
		if uc.LeverArmM != nil && *uc.LeverArmM > 0 {
			t.LeverArmM = *uc.LeverArmM
		}
		if uc.MirrorDistM != nil && *uc.MirrorDistM > 0 {
			t.MirrorDistM = *uc.MirrorDistM
		}
	}
}

// applyHallEffectConfig 用前端提交的实际实验参数覆盖 DB target 中的真值。
func applyHallEffectConfig(t *hallEffectTarget, userConfigRaw []byte) {
	if len(userConfigRaw) == 0 {
		return
	}
	var uc struct {
		BFieldT    *float64 `json:"b_field_t"`
		ThicknessM *float64 `json:"thickness_m"`
	}
	if json.Unmarshal(userConfigRaw, &uc) == nil {
		if uc.BFieldT != nil && *uc.BFieldT > 0 {
			t.BFieldT = *uc.BFieldT
		}
		if uc.ThicknessM != nil && *uc.ThicknessM > 0 {
			t.ThicknessM = *uc.ThicknessM
		}
	}
}

// applyMichelsonConfig 用前端提交的实际实验参数覆盖 DB target 中的真值。
func applyMichelsonConfig(t *michelsonTarget, userConfigRaw []byte) {
	if len(userConfigRaw) == 0 {
		return
	}
	var uc struct {
		WavelengthNM *float64 `json:"wavelength_nm"`
	}
	if json.Unmarshal(userConfigRaw, &uc) == nil {
		if uc.WavelengthNM != nil && *uc.WavelengthNM > 0 {
			t.WavelengthNM = *uc.WavelengthNM
		}
	}
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
