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

// ScorePendulum 单摆：由测得周期反算 g，与真值比对。
func ScorePendulum(readings []pendulumReading, t pendulumTarget) (score int, passed bool, detail string) {
	if len(readings) == 0 || t.Gravity <= 0 {
		return 0, false, "读数或目标参数无效"
	}
	r := readings[0]
	rel := math.Abs(r.CalcG-t.Gravity) / t.Gravity
	score = scoreFromRelErr(rel)
	passed = score >= t.PassScore
	detail = fmt.Sprintf("g=%.2f(真值%.2f), 相对误差%.2f%%", r.CalcG, t.Gravity, rel*100)
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
