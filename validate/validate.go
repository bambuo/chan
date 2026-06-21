package validate

import (
	"errors"
	"fmt"
	"math"

	"github.com/bambuo/chan/types"
)

// Common errors.
var (
	ErrNilInput       = errors.New("chanlun: input is nil")
	ErrEmptyInput     = errors.New("chanlun: input is empty")
	ErrTooFewKlines   = errors.New("chanlun: need at least 3 klines")
	ErrInvalidConfig  = errors.New("chanlun: invalid config")
	ErrDivisionByZero = errors.New("chanlun: division by zero")
)

// Config 校验引擎配置参数，返回所有发现的错误。
func Config(cfg types.Config) error {
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
	if cfg.UpdateWindowSize < 0 {
		errs = append(errs, "UpdateWindowSize must be >= 0")
	}

	switch cfg.LeftSegMethod {
	case "peak", "all":
	default:
		errs = append(errs, fmt.Sprintf("invalid LeftSegMethod: %q (want peak|all)", cfg.LeftSegMethod))
	}
	switch cfg.ZsCombineMode {
	case "zs", "peak":
	default:
		errs = append(errs, fmt.Sprintf("invalid ZsCombineMode: %q (want zs|peak)", cfg.ZsCombineMode))
	}
	switch cfg.BspMacdAlgo {
	case "area", "peak", "full_area", "diff", "slope", "amp",
		"amount", "volumn", "amount_avg", "volumn_avg", "rsi":
	case "":
	default:
		errs = append(errs, fmt.Sprintf("invalid BspMacdAlgo: %q", cfg.BspMacdAlgo))
	}
	if cfg.Bsp3aMaxZsCnt < 1 {
		errs = append(errs, "Bsp3aMaxZsCnt must be >= 1")
	}
	if cfg.BspMaxBs2Rate > 1 {
		errs = append(errs, "BspMaxBs2Rate must be <= 1")
	}

	if len(errs) > 0 {
		return fmt.Errorf("%w: %s", ErrInvalidConfig, joinStrings(errs, "; "))
	}
	return nil
}

// Klines 校验 Kline 输入是否合法。
func Klines(klines []types.Kline) error {
	if klines == nil {
		return ErrNilInput
	}
	if len(klines) == 0 {
		return ErrEmptyInput
	}
	if len(klines) < 3 {
		return ErrTooFewKlines
	}

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

// Bi 校验笔的方向与价格一致性。
func Bi(bi types.Bi) error {
	if bi.KLineCount < 2 {
		return fmt.Errorf("%w: 笔 K 线数量=%d，需要至少 2 根", ErrTooFewKlines, bi.KLineCount)
	}
	if bi.IsUp() && bi.EndPrice <= bi.StartPrice {
		return fmt.Errorf("向上笔终点价格 %.2f 不高于起点 %.2f", bi.EndPrice, bi.StartPrice)
	}
	if bi.IsDown() && bi.EndPrice >= bi.StartPrice {
		return fmt.Errorf("向下笔终点价格 %.2f 不低于起点 %.2f", bi.EndPrice, bi.StartPrice)
	}
	if math.IsNaN(bi.StartPrice) || math.IsNaN(bi.EndPrice) {
		return fmt.Errorf("%w: Nan 价格", ErrInvalidConfig)
	}
	if bi.StartIndex < 0 || bi.EndIndex < 0 || bi.EndIndex < bi.StartIndex {
		return fmt.Errorf("笔索引范围无效 [%d, %d]", bi.StartIndex, bi.EndIndex)
	}
	return nil
}

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
