package chanlun

import (
	"errors"
	"fmt"
	"math"
)

// ──────────────────────────────────────────────
// §12  配置校验与输入验证
// ──────────────────────────────────────────────

// Common errors.
var (
	ErrNilInput       = errors.New("chanlun: input is nil")
	ErrEmptyInput     = errors.New("chanlun: input is empty")
	ErrTooFewKlines   = errors.New("chanlun: need at least 3 klines")
	ErrInvalidConfig  = errors.New("chanlun: invalid config")
	ErrDivisionByZero = errors.New("chanlun: division by zero")
)

// ValidateConfig 校验引擎配置参数，返回所有发现的错误。
func ValidateConfig(cfg Config) error {
	var errs []string

	if cfg.BiMinKLineCount < 1 {
		errs = append(errs, "BiMinKLineCount must be >= 1")
	}
	if cfg.MACDFastPeriod < 2 {
		errs = append(errs, "MACDFastPeriod must be >= 2")
	}
	if cfg.MACDSlowPeriod < cfg.MACDFastPeriod+2 {
		errs = append(errs, "MACDSlowPeriod must be >= MACDFastPeriod + 2")
	}
	if cfg.MACDSignalPeriod < 1 {
		errs = append(errs, "MACDSignalPeriod must be >= 1")
	}
	if cfg.DIFReturnThreshold <= 0 || cfg.DIFReturnThreshold > 1 {
		errs = append(errs, "DIFReturnThreshold must be in (0, 1]")
	}
	if cfg.UpdateWindowSize < 0 {
		errs = append(errs, "UpdateWindowSize must be >= 0")
	}
	if cfg.NewBiMinPriceRatio < 0 || cfg.NewBiMinPriceRatio > 1 {
		errs = append(errs, "NewBiMinPriceRatio must be in [0, 1]")
	}

	switch cfg.DeviationForceMethod {
	case "slope", "macd_area", "combined":
		// valid
	default:
		errs = append(errs, fmt.Sprintf("invalid DeviationForceMethod: %q (want slope|macd_area|combined)", cfg.DeviationForceMethod))
	}

	if len(errs) > 0 {
		return fmt.Errorf("%w: %s", ErrInvalidConfig, joinStrings(errs, "; "))
	}
	return nil
}

// ValidateKlines 校验 Kline 输入是否合法。
func ValidateKlines(klines []Kline) error {
	if klines == nil {
		return ErrNilInput
	}
	if len(klines) == 0 {
		return ErrEmptyInput
	}
	if len(klines) < 3 {
		return ErrTooFewKlines
	}

	// 检查价格、流动性字段和时间顺序合法性
	for i, c := range klines {
		if math.IsNaN(c.Open) || math.IsNaN(c.High) || math.IsNaN(c.Low) || math.IsNaN(c.Close) {
			return fmt.Errorf("kline[%d]: NaN price", i)
		}
		if math.IsInf(c.Open, 0) || math.IsInf(c.High, 0) || math.IsInf(c.Low, 0) || math.IsInf(c.Close, 0) {
			return fmt.Errorf("kline[%d]: Inf price", i)
		}
		if math.IsNaN(c.BaseVolume) || math.IsNaN(c.QuoteVolume) || math.IsNaN(c.Turnover) {
			return fmt.Errorf("kline[%d]: NaN volume/turnover", i)
		}
		if math.IsInf(c.BaseVolume, 0) || math.IsInf(c.QuoteVolume, 0) || math.IsInf(c.Turnover, 0) {
			return fmt.Errorf("kline[%d]: Inf volume/turnover", i)
		}
		if c.High < c.Low {
			return fmt.Errorf("kline[%d]: High (%.2f) < Low (%.2f)", i, c.High, c.Low)
		}
		if c.BaseVolume < 0 || c.QuoteVolume < 0 || c.Turnover < 0 || c.TradeCount < 0 {
			return fmt.Errorf("kline[%d]: negative volume/turnover/trade count", i)
		}
		if i > 0 && !klines[i-1].Time.IsZero() && !c.Time.IsZero() && !c.Time.After(klines[i-1].Time.Time) {
			return fmt.Errorf("kline[%d]: time must be strictly ascending", i)
		}
	}

	return nil
}

// SafeDivide 安全除法，分母接近 0 时返回 fallback。
func SafeDivide(num, den, fallback float64) float64 {
	if math.Abs(den) < 1e-12 {
		return fallback
	}
	return num / den
}

// IsPriceValid 检查价格是否为合法正数。
func IsPriceValid(p float64) bool {
	return !math.IsNaN(p) && !math.IsInf(p, 0) && p > 0
}

// joinStrings 拼接字符串切片（替代 strings.Join 以避免额外导入）。
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for _, s := range strs[1:] {
		result += sep + s
	}
	return result
}
