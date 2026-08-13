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

// ---- 第二批新增实验评分目标结构 ----

type photoelectricTarget struct {
	Method       string  `json:"method"`          // planck_fit
	PlanckConst  float64 `json:"planck_const"`    // 普朗克常数真值 (J·s)
	WorkFunction float64 `json:"work_function"`   // 逸出功真值 (J)
	PassScore    int     `json:"pass_score"`
}

type frankHertzTarget struct {
	Method          string  `json:"method"`             // excitation_fit
	ExcitationPotV  float64 `json:"excitation_pot_v"`   // 激发电位真值 (V)
	PassScore       int     `json:"pass_score"`
}

type gratingTarget struct {
	Method         string  `json:"method"`           // grating_fit
	GratingConstM  float64 `json:"grating_const_m"`  // 光栅常数真值 (m)
	PassScore      int     `json:"pass_score"`
}

// ---- 第三批新增实验评分目标结构 ----

type oilDropTarget struct {
	Method     string  `json:"method"`       // oil_drop_fit
	Elementary float64 `json:"elementary"`   // 元电荷真值 (C)
	PassScore  int     `json:"pass_score"`
}

type polarizationTarget struct {
	Method    string  `json:"method"`      // malus_fit
	PassScore int     `json:"pass_score"`
}

type soundSpeedTarget struct {
	Method    string  `json:"method"`       // sound_speed_fit
	SpeedMS   float64 `json:"speed_ms"`     // 声速真值 (m/s)
	FreqHz    float64 `json:"freq_hz"`      // 超声波频率 (Hz)
	PassScore int     `json:"pass_score"`
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

// ---- 第二批新增实验读数结构 ----

type photoelectricReading struct {
	FrequencyHz float64 `json:"frequency_hz"` // 光频率 (Hz)
	StopV       float64 `json:"stop_v"`       // 遏止电压 (V)
}

type frankHertzReading struct {
	VoltageV float64 `json:"voltage_v"` // 加速电压 (V)
	CurrentA float64 `json:"current_a"` // 板极电流 (A)
}

type gratingReading struct {
	OrderM     int     `json:"order"`     // 衍射级次 (±1,±2...)
	AngleRad   float64 `json:"angle_rad"` // 衍射角 (rad)
}

// ---- 第三批新增实验读数结构 ----

type oilDropReading struct {
	VoltageV  float64 `json:"voltage_v"`  // 平衡电压 (V)
	DistanceM float64 `json:"distance_m"` // 油滴下落距离 (m)
	TimeS     float64 `json:"time_s"`     // 油滴下落时间 (s)
}

type polarizationReading struct {
	AngleDeg  float64 `json:"angle_deg"`  // 检偏器角度 (°)
	Intensity float64 `json:"intensity"`  // 透射光强 (相对值)
}

type soundSpeedReading struct {
	OrderN    int     `json:"order"`     // 共振点序号
	PositionM float64 `json:"position_m"` // 反射镜位置 (m)
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

// ScorePhotoelectric 光电效应：由多组 (ν, V_s) 数据线性拟合
// V_s = (h/e)·ν - W/e，斜率 = h/e -> h = slope·e，与真值比对。
func ScorePhotoelectric(readings []photoelectricReading, t photoelectricTarget) (score int, passed bool, detail string) {
	if len(readings) < 2 || t.PlanckConst <= 0 {
		return 0, false, "读数或目标参数无效"
	}
	const e = 1.602e-19 // 电子电荷 (C)

	// 过滤无效读数，收集 (ν, V_s) 对
	type xy struct{ x, y float64 }
	var pts []xy
	for _, r := range readings {
		if r.FrequencyHz <= 0 || math.IsNaN(r.FrequencyHz) || math.IsInf(r.FrequencyHz, 0) {
			continue
		}
		if math.IsNaN(r.StopV) || math.IsInf(r.StopV, 0) {
			continue
		}
		pts = append(pts, xy{r.FrequencyHz, r.StopV})
	}
	if len(pts) < 2 {
		return 0, false, "有效读数不足"
	}

	// 最小二乘线性拟合 y = a·x + b
	var sumX, sumY, sumXY, sumX2 float64
	n := float64(len(pts))
	for _, p := range pts {
		sumX += p.x
		sumY += p.y
		sumXY += p.x * p.y
		sumX2 += p.x * p.x
	}
	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0, false, "数据线性退化"
	}
	slope := (n*sumXY - sumX*sumY) / denom // = h/e

	h := slope * e // 普朗克常数 (J·s)
	if h <= 0 || math.IsNaN(h) || math.IsInf(h, 0) {
		return 0, false, "拟合结果无效"
	}
	rel := math.Abs(h-t.PlanckConst) / t.PlanckConst
	score = scoreFromRelErr(rel)
	passed = score >= t.PassScore
	detail = fmt.Sprintf("h=%.4e J·s(真值%.4e J·s), 相对误差%.2f%%", h, t.PlanckConst, rel*100)
	return
}

// ScoreFrankHertz 弗兰克-赫兹：从 I-V 曲线找峰位，
// 相邻峰电压差即激发电位，取平均与真值比对。
func ScoreFrankHertz(readings []frankHertzReading, t frankHertzTarget) (score int, passed bool, detail string) {
	if len(readings) < 4 || t.ExcitationPotV <= 0 {
		return 0, false, "读数或目标参数无效"
	}

	// 过滤无效读数，按电压排序
	var valid []frankHertzReading
	for _, r := range readings {
		if r.VoltageV <= 0 || r.CurrentA < 0 ||
			math.IsNaN(r.VoltageV) || math.IsInf(r.VoltageV, 0) ||
			math.IsNaN(r.CurrentA) || math.IsInf(r.CurrentA, 0) {
			continue
		}
		valid = append(valid, r)
	}
	if len(valid) < 4 {
		return 0, false, "有效读数不足"
	}
	// 按 VoltageV 升序排序（简单插入排序）
	for i := 1; i < len(valid); i++ {
		for j := i; j > 0 && valid[j].VoltageV < valid[j-1].VoltageV; j-- {
			valid[j], valid[j-1] = valid[j-1], valid[j]
		}
	}

	// 找局部极大值（峰位）：使用窗口法
	// 对每个点，检查它是否是 ±window 范围内的最大值
	window := 3
	if window > len(valid)/4 {
		window = len(valid) / 4
	}
	if window < 1 {
		window = 1
	}
	var peaks []float64
	for i := window; i < len(valid)-window; i++ {
		isPeak := true
		for j := i - window; j <= i+window; j++ {
			if j == i {
				continue
			}
			if valid[j].CurrentA >= valid[i].CurrentA {
				isPeak = false
				break
			}
		}
		if isPeak {
			// 避免相邻点重复检测同一个峰
			if len(peaks) == 0 || valid[i].VoltageV-peaks[len(peaks)-1] > t.ExcitationPotV*0.3 {
				peaks = append(peaks, valid[i].VoltageV)
			}
		}
	}
	if len(peaks) < 2 {
		return 0, false, "峰位不足，无法计算激发电位"
	}

	// 相邻峰电压差取平均
	var sumDelta float64
	count := 0
	for i := 1; i < len(peaks); i++ {
		d := peaks[i] - peaks[i-1]
		if d > 0 {
			sumDelta += d
			count++
		}
	}
	if count == 0 {
		return 0, false, "峰位差无效"
	}
	avgV := sumDelta / float64(count)
	rel := math.Abs(avgV-t.ExcitationPotV) / t.ExcitationPotV
	score = scoreFromRelErr(rel)
	passed = score >= t.PassScore
	detail = fmt.Sprintf("激发电位=%.2fV(真值%.2fV), 相对误差%.2f%%", avgV, t.ExcitationPotV, rel*100)
	return
}

// ScoreDiffractionGrating 光栅衍射：d·sin(θ) = k·λ，
// 由多组 (k, θ) 数据和已知 λ 反算光栅常数 d，取平均与真值比对。
func ScoreDiffractionGrating(readings []gratingReading, t gratingTarget) (score int, passed bool, detail string) {
	if len(readings) == 0 || t.GratingConstM <= 0 {
		return 0, false, "读数或目标参数无效"
	}
	const lambdaM = 589.3e-9 // 钠光波长 (m)，与种子 config 一致

	var sumD float64
	count := 0
	for _, r := range readings {
		// 数据校验：级次须为非零整数，角度须为正有限值
		if r.OrderM == 0 || r.AngleRad <= 0 ||
			math.IsNaN(r.AngleRad) || math.IsInf(r.AngleRad, 0) {
			continue
		}
		if r.AngleRad >= math.Pi/2 {
			continue // 衍射角不能 ≥ 90°
		}
		// d = k·λ / sin(θ)，取绝对值
		d := math.Abs(float64(r.OrderM)) * lambdaM / math.Sin(r.AngleRad)
		if d > 0 && !math.IsNaN(d) && !math.IsInf(d, 0) {
			sumD += d
			count++
		}
	}
	if count == 0 {
		return 0, false, "无有效读数"
	}
	avgD := sumD / float64(count)
	rel := math.Abs(avgD-t.GratingConstM) / t.GratingConstM
	score = scoreFromRelErr(rel)
	passed = score >= t.PassScore
	detail = fmt.Sprintf("d=%.2e m(真值%.2e m), 相对误差%.2f%%", avgD, t.GratingConstM, rel*100)
	return
}

// ScoreOilDrop 密立根油滴：由平衡电压 V、下落距离 l、下落时间 t 求电荷量
// q = (18πηl / V) · √(l / (2ρg t²))，其中 η 空气粘滞系数、ρ 油密度、g 重力加速度
// 多组数据求最大公约数即元电荷 e，与真值比对。
func ScoreOilDrop(readings []oilDropReading, t oilDropTarget) (score int, passed bool, detail string) {
	if len(readings) == 0 || t.Elementary <= 0 {
		return 0, false, "读数或目标参数无效"
	}
	const (
		eta = 1.83e-5  // 空气粘滞系数 (Pa·s)
		rho = 850.0    // 油密度 (kg/m³)
		g   = 9.8      // 重力加速度 (m/s²)
	)

	// 对每组数据计算电荷量 q
	var charges []float64
	for _, r := range readings {
		if r.VoltageV <= 0 || r.DistanceM <= 0 || r.TimeS <= 0 ||
			math.IsNaN(r.VoltageV) || math.IsInf(r.VoltageV, 0) ||
			math.IsNaN(r.DistanceM) || math.IsInf(r.DistanceM, 0) ||
			math.IsNaN(r.TimeS) || math.IsInf(r.TimeS, 0) {
			continue
		}
		// q = (18πηl / V) · √(l / (2ρgt²))
		vTerm := r.DistanceM / r.TimeS // 终端速度
		a := r.DistanceM
		b := 2 * rho * g * r.TimeS * r.TimeS
		if b <= 0 {
			continue
		}
		radius := math.Sqrt(a / b) // 油滴半径
		// 修正：q = 6πηr(v1+v2)/V ，此处用简化平衡法
		// 平衡时 mg = qE = qV/d，粘滞阻力 f = 6πηrv
		// q = (6πηrv) · d / V ，d 为极板间距（取 6mm = 0.006m）
		const plateDist = 0.006
		q := (6 * math.Pi * eta * radius * vTerm * plateDist) / r.VoltageV
		if q > 0 && !math.IsNaN(q) && !math.IsInf(q, 0) {
			charges = append(charges, q)
		}
	}
	if len(charges) == 0 {
		return 0, false, "无有效读数"
	}

	// 每组电荷量除以元电荷真值，四舍五入取整得到带电量子数 n
	// 然后用 q/n 反算元电荷，取平均
	var sumE float64
	count := 0
	for _, q := range charges {
		n := math.Round(q / t.Elementary)
		if n < 1 {
			n = 1
		}
		eMeasured := q / n
		if eMeasured > 0 && !math.IsNaN(eMeasured) && !math.IsInf(eMeasured, 0) {
			sumE += eMeasured
			count++
		}
	}
	if count == 0 {
		return 0, false, "电荷量计算无效"
	}
	avgE := sumE / float64(count)
	rel := math.Abs(avgE-t.Elementary) / t.Elementary
	score = scoreFromRelErr(rel)
	passed = score >= t.PassScore
	detail = fmt.Sprintf("e=%.4e C(真值%.4e C), 相对误差%.2f%%", avgE, t.Elementary, rel*100)
	return
}

// ScorePolarization 偏振光：验证马吕斯定律 I = I₀cos²(θ)
// 由多组 (θ, I) 数据拟合 I = I₀cos²(θ)，与理论曲线比对。
// 评分方式：归一化后计算与理论曲线的平均相对偏差。
func ScorePolarization(readings []polarizationReading, t polarizationTarget) (score int, passed bool, detail string) {
	if len(readings) < 3 {
		return 0, false, "读数或目标参数无效"
	}

	// 过滤无效读数
	var valid []polarizationReading
	for _, r := range readings {
		if r.AngleDeg < 0 || r.AngleDeg > 180 ||
			math.IsNaN(r.AngleDeg) || math.IsInf(r.AngleDeg, 0) ||
			math.IsNaN(r.Intensity) || math.IsInf(r.Intensity, 0) || r.Intensity < 0 {
			continue
		}
		valid = append(valid, r)
	}
	if len(valid) < 3 {
		return 0, false, "有效读数不足"
	}

	// 求 I₀ = 最大光强（θ=0 附近）
	maxI := 0.0
	for _, r := range valid {
		if r.Intensity > maxI {
			maxI = r.Intensity
		}
	}
	if maxI <= 0 {
		return 0, false, "最大光强无效"
	}

	// 计算每组数据的相对偏差：|I_measured - I₀cos²(θ)| / I₀
	var sumRel float64
	count := 0
	for _, r := range valid {
		thetaRad := r.AngleDeg * math.Pi / 180
		theoryI := maxI * math.Cos(thetaRad) * math.Cos(thetaRad)
		rel := math.Abs(r.Intensity-theoryI) / maxI
		sumRel += rel
		count++
	}
	if count == 0 {
		return 0, false, "计算失败"
	}
	avgRel := sumRel / float64(count)
	score = scoreFromRelErr(avgRel)
	passed = score >= t.PassScore
	detail = fmt.Sprintf("马吕斯定律平均偏差%.2f%%", avgRel*100)
	return
}

// ScoreSoundSpeed 声速测量：驻波法测相邻共振点距离 = λ/2
// 由多组共振点位置数据，逐差法求 λ/2，乘以频率得声速 v = λ·f，与真值比对。
func ScoreSoundSpeed(readings []soundSpeedReading, t soundSpeedTarget) (score int, passed bool, detail string) {
	if len(readings) < 3 || t.SpeedMS <= 0 || t.FreqHz <= 0 {
		return 0, false, "读数或目标参数无效"
	}

	// 过滤无效读数
	var valid []soundSpeedReading
	for _, r := range readings {
		if r.PositionM <= 0 || math.IsNaN(r.PositionM) || math.IsInf(r.PositionM, 0) {
			continue
		}
		valid = append(valid, r)
	}
	if len(valid) < 3 {
		return 0, false, "有效读数不足"
	}
	// 按 PositionM 排序
	for i := 1; i < len(valid); i++ {
		for j := i; j > 0 && valid[j].PositionM < valid[j-1].PositionM; j-- {
			valid[j], valid[j-1] = valid[j-1], valid[j]
		}
	}

	// 逐差法：相邻共振点间距 = λ/2
	// 取前后半段对应项之差求平均
	n := len(valid) / 2
	if n < 1 {
		n = 1
	}
	var sumDelta float64
	count := 0
	for i := 0; i < n && i+n < len(valid); i++ {
		d := valid[i+n].PositionM - valid[i].PositionM
		if d > 0 {
			sumDelta += d
			count++
		}
	}
	if count == 0 {
		return 0, false, "逐差法无有效差值"
	}
	// avgDelta = n × (λ/2)，所以 λ = 2 × avgDelta / n
	avgDelta := sumDelta / float64(count)
	halfLambda := avgDelta / float64(n) // λ/2
	lambda := 2 * halfLambda
	if lambda <= 0 {
		return 0, false, "波长计算无效"
	}
	v := lambda * t.FreqHz
	rel := math.Abs(v-t.SpeedMS) / t.SpeedMS
	score = scoreFromRelErr(rel)
	passed = score >= t.PassScore
	detail = fmt.Sprintf("v=%.2f m/s(真值%.2f m/s), 相对误差%.2f%%", v, t.SpeedMS, rel*100)
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
	case "photoelectric":
		var rs []photoelectricReading
		var t photoelectricTarget
		if err = unmarshalBoth(readingsRaw, targetRaw, &rs, &t); err != nil {
			return
		}
		score, passed, detail = ScorePhotoelectric(rs, t)
	case "frank_hertz":
		var rs []frankHertzReading
		var t frankHertzTarget
		if err = unmarshalBoth(readingsRaw, targetRaw, &rs, &t); err != nil {
			return
		}
		score, passed, detail = ScoreFrankHertz(rs, t)
	case "diffraction_grating":
		var rs []gratingReading
		var t gratingTarget
		if err = unmarshalBoth(readingsRaw, targetRaw, &rs, &t); err != nil {
			return
		}
		applyGratingConfig(&t, userConfigRaw)
		score, passed, detail = ScoreDiffractionGrating(rs, t)
	case "oil_drop":
		var rs []oilDropReading
		var t oilDropTarget
		if err = unmarshalBoth(readingsRaw, targetRaw, &rs, &t); err != nil {
			return
		}
		score, passed, detail = ScoreOilDrop(rs, t)
	case "polarization":
		var rs []polarizationReading
		var t polarizationTarget
		if err = unmarshalBoth(readingsRaw, targetRaw, &rs, &t); err != nil {
			return
		}
		score, passed, detail = ScorePolarization(rs, t)
	case "sound_speed":
		var rs []soundSpeedReading
		var t soundSpeedTarget
		if err = unmarshalBoth(readingsRaw, targetRaw, &rs, &t); err != nil {
			return
		}
		applySoundSpeedConfig(&t, userConfigRaw)
		score, passed, detail = ScoreSoundSpeed(rs, t)
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

// applyGratingConfig 用前端提交的实际实验参数覆盖 DB target 中的真值。
func applyGratingConfig(t *gratingTarget, userConfigRaw []byte) {
	if len(userConfigRaw) == 0 {
		return
	}
	var uc struct {
		GratingConstM *float64 `json:"grating_const_m"`
	}
	if json.Unmarshal(userConfigRaw, &uc) == nil {
		if uc.GratingConstM != nil && *uc.GratingConstM > 0 {
			t.GratingConstM = *uc.GratingConstM
		}
	}
}

// applySoundSpeedConfig 用前端提交的实际实验参数覆盖 DB target 中的真值。
func applySoundSpeedConfig(t *soundSpeedTarget, userConfigRaw []byte) {
	if len(userConfigRaw) == 0 {
		return
	}
	var uc struct {
		FreqHz *float64 `json:"freq_hz"`
	}
	if json.Unmarshal(userConfigRaw, &uc) == nil {
		if uc.FreqHz != nil && *uc.FreqHz > 0 {
			t.FreqHz = *uc.FreqHz
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
