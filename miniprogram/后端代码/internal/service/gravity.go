package service

import "math"

// LocalGravity 由纬度和海拔计算当地重力加速度。
//
// WGS84 正常重力公式（椭球形式）+ 自由空气改正：
//
//	g(φ,h) = 9.7803267715·(1 + 0.0052790414·sin²φ + 0.0000232718·sin⁴φ + 0.0000001262·sin⁶φ)
//	         − 3.086×10⁻⁶·h
//
// 校验点：φ=0 → 9.78033；φ=45° → 9.80620；φ=90° → 9.83219。
//
// 必须与前端 utils/region.js 的 localGravity() 保持完全一致，
// 否则学生按当地 g 算出的结果会被判错。
func LocalGravity(latDeg, altM float64) float64 {
	phi := latDeg * math.Pi / 180
	s := math.Sin(phi)
	s2 := s * s
	s4 := s2 * s2
	s6 := s4 * s2
	g := 9.7803267715 * (1 + 0.0052790414*s2 + 0.0000232718*s4 + 0.0000001262*s6)
	return g - 3.086e-6*altM
}

// plausibleLocation 判断客户端上报的经纬度/海拔是否在合理范围内。
// 越界（或根本没传）时调用方回退到实验配置里的 gravity 真值，
// 避免伪造坐标把评分基准拖到任意值。
func plausibleLocation(latDeg, altM float64) bool {
	// 纬度取全球有人区范围；海拔上界取青藏高原可达高度
	if latDeg < -60 || latDeg > 75 {
		return false
	}
	if altM < -500 || altM > 6000 {
		return false
	}
	return true
}
