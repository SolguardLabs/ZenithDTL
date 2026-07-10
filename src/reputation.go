package zenithdtl

import "sort"

type ReputationReason string

const (
	ReasonSettlementSuccess ReputationReason = "settlement_success"
	ReasonTimelyProof       ReputationReason = "timely_proof"
	ReasonRouteQuality      ReputationReason = "route_quality"
	ReasonLatency           ReputationReason = "latency"
	ReasonMissedWindow      ReputationReason = "missed_window"
	ReasonRejectedReceipt   ReputationReason = "rejected_receipt"
	ReasonManualReview      ReputationReason = "manual_review"
	ReasonWithdrawalLoad    ReputationReason = "withdrawal_load"
)

type ReputationEntry struct {
	Epoch      Epoch            `json:"epoch"`
	Operator   OperatorID       `json:"operator"`
	Delta      int64            `json:"delta"`
	Reason     ReputationReason `json:"reason"`
	DeferredTo Epoch            `json:"deferredTo,omitempty"`
	Applied    bool             `json:"applied"`
}

type ScoreWindow struct {
	Epoch         Epoch  `json:"epoch"`
	Successful    int64  `json:"successful"`
	Failed        int64  `json:"failed"`
	Volume        Amount `json:"volume"`
	AverageMillis int64  `json:"averageMillis"`
}

type ScoreCard struct {
	Operator        OperatorID        `json:"operator"`
	Score           int64             `json:"score"`
	Baseline        int64             `json:"baseline"`
	PendingPositive int64             `json:"pendingPositive"`
	PendingNegative int64             `json:"pendingNegative"`
	LastUpdated     Epoch             `json:"lastUpdated"`
	Windows         []ScoreWindow     `json:"windows"`
	Labels          map[string]string `json:"labels"`
}

func NewScoreCard(operator OperatorID, baseline int64) *ScoreCard {
	score := clampScore(baseline)
	return &ScoreCard{
		Operator:    operator,
		Score:       score,
		Baseline:    score,
		LastUpdated: 0,
		Windows:     []ScoreWindow{},
		Labels:      map[string]string{},
	}
}

func (c *ScoreCard) Clone() ScoreCard {
	clone := *c
	clone.Windows = make([]ScoreWindow, len(c.Windows))
	copy(clone.Windows, c.Windows)
	clone.Labels = CloneStringMap(c.Labels)
	return clone
}

func (c *ScoreCard) RecordWindow(window ScoreWindow) {
	c.Windows = append(c.Windows, window)
	if len(c.Windows) > 8 {
		c.Windows = c.Windows[len(c.Windows)-8:]
	}
}

func (c *ScoreCard) Apply(delta int64, epoch Epoch) {
	c.Score = clampScore(c.Score + delta)
	c.LastUpdated = epoch
	if delta > 0 {
		if c.PendingPositive >= delta {
			c.PendingPositive -= delta
		} else {
			c.PendingPositive = 0
		}
	}
	if delta < 0 {
		abs := -delta
		if c.PendingNegative >= abs {
			c.PendingNegative -= abs
		} else {
			c.PendingNegative = 0
		}
	}
}

func (c *ScoreCard) Queue(delta int64) {
	if delta >= 0 {
		c.PendingPositive += delta
		return
	}
	c.PendingNegative += -delta
}

func (c *ScoreCard) QualityBand() string {
	switch {
	case c.Score >= 900:
		return "prime"
	case c.Score >= 760:
		return "senior"
	case c.Score >= 620:
		return "standard"
	case c.Score >= 480:
		return "watch"
	default:
		return "restricted"
	}
}

func (c *ScoreCard) RiskWeight() int64 {
	switch c.QualityBand() {
	case "prime":
		return 140
	case "senior":
		return 120
	case "standard":
		return 100
	case "watch":
		return 70
	default:
		return 35
	}
}

func (c *ScoreCard) RecentFailureRate() int64 {
	var success int64
	var failed int64
	for _, window := range c.Windows {
		success += window.Successful
		failed += window.Failed
	}
	total := success + failed
	if total == 0 {
		return 0
	}
	return failed * 10_000 / total
}

type ReputationBook struct {
	cards   map[OperatorID]*ScoreCard
	journal []ReputationEntry
	events  *EventLog
}

func NewReputationBook(events *EventLog) *ReputationBook {
	return &ReputationBook{
		cards:   map[OperatorID]*ScoreCard{},
		journal: []ReputationEntry{},
		events:  events,
	}
}

func (b *ReputationBook) Register(operator OperatorID, baseline int64) *ScoreCard {
	card := NewScoreCard(operator, baseline)
	b.cards[operator] = card
	if b.events != nil {
		b.events.Add(0, EventReputationUpdated, string(operator), eventAttrs("score", amountString(Amount(card.Score)), "band", card.QualityBand()))
	}
	return card
}

func (b *ReputationBook) Card(operator OperatorID) (*ScoreCard, error) {
	card, ok := b.cards[operator]
	if !ok {
		return nil, refError(ErrUnknownOperator, string(operator), "scorecard not found")
	}
	return card, nil
}

func (b *ReputationBook) Score(operator OperatorID) int64 {
	card, ok := b.cards[operator]
	if !ok {
		return 0
	}
	return card.Score
}

func (b *ReputationBook) Band(operator OperatorID) string {
	card, ok := b.cards[operator]
	if !ok {
		return "unknown"
	}
	return card.QualityBand()
}

func (b *ReputationBook) RecordPerformance(epoch Epoch, operator OperatorID, success int64, failed int64, volume Amount, avgMillis int64) error {
	card, err := b.Card(operator)
	if err != nil {
		return err
	}
	card.RecordWindow(ScoreWindow{
		Epoch:         epoch,
		Successful:    success,
		Failed:        failed,
		Volume:        volume,
		AverageMillis: avgMillis,
	})
	delta := success*3 - failed*17
	if avgMillis > 2_500 {
		delta -= 12
	}
	if avgMillis > 5_000 {
		delta -= 20
	}
	if volume > 50_000 {
		delta += 8
	}
	if failed == 0 && success >= 4 {
		delta += 15
	}
	b.QueueDelta(epoch, operator, delta, ReasonSettlementSuccess, epoch)
	return nil
}

func (b *ReputationBook) QueueDelta(epoch Epoch, operator OperatorID, delta int64, reason ReputationReason, deferredTo Epoch) {
	card, ok := b.cards[operator]
	if !ok {
		card = b.Register(operator, 500)
	}
	card.Queue(delta)
	entry := ReputationEntry{
		Epoch:      epoch,
		Operator:   operator,
		Delta:      delta,
		Reason:     reason,
		DeferredTo: deferredTo,
		Applied:    false,
	}
	b.journal = append(b.journal, entry)
	if b.events != nil {
		kind := EventReputationUpdated
		attrs := eventAttrs("delta", amountString(Amount(delta)), "reason", string(reason), "deferredTo", epochString(deferredTo))
		if delta < 0 {
			b.events.Warning(epoch, kind, string(operator), attrs)
		} else {
			b.events.Add(epoch, kind, string(operator), attrs)
		}
	}
}

func (b *ReputationBook) ApplyDue(epoch Epoch) {
	for i := range b.journal {
		entry := &b.journal[i]
		if entry.Applied || entry.DeferredTo > epoch {
			continue
		}
		card, ok := b.cards[entry.Operator]
		if !ok {
			continue
		}
		card.Apply(entry.Delta, epoch)
		entry.Applied = true
		if b.events != nil {
			b.events.Add(epoch, EventReputationUpdated, string(entry.Operator), eventAttrs("score", amountString(Amount(card.Score)), "band", card.QualityBand(), "applied", string(entry.Reason)))
		}
	}
}

func (b *ReputationBook) ApplyPositiveDue(epoch Epoch) {
	for i := range b.journal {
		entry := &b.journal[i]
		if entry.Applied || entry.Delta < 0 || entry.DeferredTo > epoch {
			continue
		}
		card, ok := b.cards[entry.Operator]
		if !ok {
			continue
		}
		card.Apply(entry.Delta, epoch)
		entry.Applied = true
	}
}

func (b *ReputationBook) PendingNegative(operator OperatorID) int64 {
	card, ok := b.cards[operator]
	if !ok {
		return 0
	}
	return card.PendingNegative
}

func (b *ReputationBook) PendingPositive(operator OperatorID) int64 {
	card, ok := b.cards[operator]
	if !ok {
		return 0
	}
	return card.PendingPositive
}

func (b *ReputationBook) RecommendedCapacity(operator OperatorID, base Amount, stake Amount, routeWeight int64) Amount {
	card, ok := b.cards[operator]
	if !ok {
		return 0
	}
	adjusted := card.Score
	if card.PendingNegative > 0 && card.LastUpdated > 0 {
		adjusted -= card.PendingNegative / 2
	}
	return WeightedCapacity(base, clampScore(adjusted), stake, routeWeight)
}

func (b *ReputationBook) Cards() []ScoreCard {
	rows := make([]ScoreCard, 0, len(b.cards))
	for _, card := range b.cards {
		rows = append(rows, card.Clone())
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Operator < rows[j].Operator })
	return rows
}

func (b *ReputationBook) Journal() []ReputationEntry {
	result := make([]ReputationEntry, len(b.journal))
	copy(result, b.journal)
	sort.Slice(result, func(i, j int) bool {
		if result[i].Epoch == result[j].Epoch {
			return result[i].Operator < result[j].Operator
		}
		return result[i].Epoch < result[j].Epoch
	})
	return result
}

func (b *ReputationBook) Operators() []OperatorID {
	keys := make([]OperatorID, 0, len(b.cards))
	for key := range b.cards {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

func (b *ReputationBook) PenalizeMissedWindow(epoch Epoch, operator OperatorID, amount Amount, deferredTo Epoch) {
	delta := int64(-45)
	if amount > 25_000 {
		delta -= 20
	}
	if amount > 75_000 {
		delta -= 35
	}
	b.QueueDelta(epoch, operator, delta, ReasonMissedWindow, deferredTo)
}

func (b *ReputationBook) RewardSettlement(epoch Epoch, operator OperatorID, amount Amount, deferredTo Epoch) {
	delta := int64(10)
	if amount > 10_000 {
		delta += 5
	}
	if amount > 50_000 {
		delta += 8
	}
	b.QueueDelta(epoch, operator, delta, ReasonSettlementSuccess, deferredTo)
}

func (b *ReputationBook) MarkManualReview(epoch Epoch, operator OperatorID, severity Severity, deferredTo Epoch) {
	delta := int64(-15)
	if severity == SeverityCritical {
		delta = -120
	}
	if severity == SeverityWarning {
		delta = -45
	}
	b.QueueDelta(epoch, operator, delta, ReasonManualReview, deferredTo)
}

func (b *ReputationBook) NetPending(operator OperatorID) int64 {
	card, ok := b.cards[operator]
	if !ok {
		return 0
	}
	return card.PendingPositive - card.PendingNegative
}

func (b *ReputationBook) PrimeOperators(minScore int64) []OperatorID {
	result := []OperatorID{}
	for _, card := range b.cards {
		if card.Score >= minScore {
			result = append(result, card.Operator)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
