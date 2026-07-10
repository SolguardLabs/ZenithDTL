package zenithdtl

import "fmt"

const (
	ZeroAmount Amount = 0
	MaxAmount  Amount = 9_000_000_000_000_000
	BpsDenom   int64  = 10_000
)

func NewAmount(value int64) (Amount, error) {
	if value < 0 {
		return 0, newError(ErrInvalidAmount, "amount must not be negative")
	}
	if value > int64(MaxAmount) {
		return 0, newError(ErrInvalidAmount, "amount exceeds protocol maximum")
	}
	return Amount(value), nil
}

func MustAmount(value int64) Amount {
	amount, err := NewAmount(value)
	if err != nil {
		panic(err)
	}
	return amount
}

func (a Amount) Int64() int64 {
	return int64(a)
}

func (a Amount) IsZero() bool {
	return a == 0
}

func (a Amount) IsPositive() bool {
	return a > 0
}

func (a Amount) Add(other Amount) (Amount, error) {
	if other < 0 {
		return 0, newError(ErrInvalidAmount, "cannot add negative amount")
	}
	if a > MaxAmount-other {
		return 0, newError(ErrInvalidAmount, "amount addition overflow")
	}
	return a + other, nil
}

func (a Amount) Sub(other Amount) (Amount, error) {
	if other < 0 {
		return 0, newError(ErrInvalidAmount, "cannot subtract negative amount")
	}
	if a < other {
		return 0, newError(ErrInvalidAmount, "amount subtraction underflow")
	}
	return a - other, nil
}

func (a Amount) Mul(value int64) (Amount, error) {
	if value < 0 {
		return 0, newError(ErrInvalidAmount, "cannot multiply by negative value")
	}
	if a == 0 || value == 0 {
		return 0, nil
	}
	if int64(a) > int64(MaxAmount)/value {
		return 0, newError(ErrInvalidAmount, "amount multiplication overflow")
	}
	return Amount(int64(a) * value), nil
}

func (a Amount) MulBps(bps BasisPoints) (Amount, error) {
	if bps < 0 {
		return 0, newError(ErrInvalidAmount, "basis points must not be negative")
	}
	product := int64(a) * int64(bps)
	if a != 0 && product/int64(a) != int64(bps) {
		return 0, newError(ErrInvalidAmount, "basis point multiplication overflow")
	}
	return Amount(product / BpsDenom), nil
}

func (a Amount) MulBpsCeil(bps BasisPoints) (Amount, error) {
	if bps < 0 {
		return 0, newError(ErrInvalidAmount, "basis points must not be negative")
	}
	product := int64(a) * int64(bps)
	if a != 0 && product/int64(a) != int64(bps) {
		return 0, newError(ErrInvalidAmount, "basis point multiplication overflow")
	}
	if product == 0 {
		return 0, nil
	}
	return Amount((product + BpsDenom - 1) / BpsDenom), nil
}

func (a Amount) Percent(value int64) (Amount, error) {
	if value < 0 {
		return 0, newError(ErrInvalidAmount, "percent must not be negative")
	}
	return a.MulBps(BasisPoints(value * 100))
}

func (a Amount) String() string {
	return fmt.Sprintf("%d", a)
}

func SumAmounts(values ...Amount) (Amount, error) {
	var total Amount
	for _, value := range values {
		next, err := total.Add(value)
		if err != nil {
			return 0, err
		}
		total = next
	}
	return total, nil
}

func SafeRatio(numerator Amount, denominator Amount, scale int64) int64 {
	if denominator == 0 || scale <= 0 {
		return 0
	}
	return int64(numerator) * scale / int64(denominator)
}

func SplitAmount(total Amount, weights []int64) ([]Amount, error) {
	if len(weights) == 0 {
		return nil, nil
	}
	var sum int64
	for _, weight := range weights {
		if weight < 0 {
			return nil, newError(ErrInvalidAmount, "split weight must not be negative")
		}
		sum += weight
	}
	if sum == 0 {
		result := make([]Amount, len(weights))
		return result, nil
	}
	result := make([]Amount, len(weights))
	var assigned Amount
	for i, weight := range weights {
		if i == len(weights)-1 {
			remaining, err := total.Sub(assigned)
			if err != nil {
				return nil, err
			}
			result[i] = remaining
			break
		}
		part := Amount(int64(total) * weight / sum)
		result[i] = part
		next, err := assigned.Add(part)
		if err != nil {
			return nil, err
		}
		assigned = next
	}
	return result, nil
}

func AmountFromString(value string) (Amount, error) {
	var parsed int64
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		return 0, newError(ErrInvalidAmount, "amount must be an integer")
	}
	return NewAmount(parsed)
}

func NonZeroAmount(value Amount, ref string) error {
	if value <= 0 {
		return refError(ErrInvalidAmount, ref, "amount must be positive")
	}
	return nil
}

func AmountWithin(value Amount, min Amount, max Amount, ref string) error {
	if value < min || value > max {
		return wrapRef(ErrInvalidAmount, ref, "amount %d outside [%d, %d]", value, min, max)
	}
	return nil
}

func NormalizeDust(value Amount, threshold Amount) Amount {
	if value < threshold {
		return 0
	}
	return value
}

func WeightedCapacity(base Amount, reputationScore int64, stake Amount, routeWeight int64) Amount {
	score := clampScore(reputationScore)
	weighted := int64(base)
	weighted += int64(base) * score / 1_000
	weighted += int64(stake) / 5
	weighted = weighted * clampInt64(routeWeight, 25, 150) / 100
	if weighted < 0 {
		return 0
	}
	if weighted > int64(MaxAmount) {
		return MaxAmount
	}
	return Amount(weighted)
}
