package zenithdtl

import "sort"

type SettlementIntent struct {
	Payer       AccountID `json:"payer"`
	Beneficiary AccountID `json:"beneficiary"`
	Asset       AssetID   `json:"asset"`
	Amount      Amount    `json:"amount"`
	Route       RouteID   `json:"route"`
	Memo        string    `json:"memo"`
}

type Obligation struct {
	ID             ObligationID         `json:"id"`
	Epoch          Epoch                `json:"epoch"`
	Payer          AccountID            `json:"payer"`
	Beneficiary    AccountID            `json:"beneficiary"`
	Asset          AssetID              `json:"asset"`
	Amount         Amount               `json:"amount"`
	Fee            Amount               `json:"fee"`
	Gross          Amount               `json:"gross"`
	Route          RouteID              `json:"route"`
	ChannelAmounts map[ChannelID]Amount `json:"channelAmounts"`
	Status         ObligationStatus     `json:"status"`
	OpenedAt       Epoch                `json:"openedAt"`
	SettledAt      Epoch                `json:"settledAt,omitempty"`
	ExpiresAt      Epoch                `json:"expiresAt"`
	Memo           string               `json:"memo,omitempty"`
}

func (o Obligation) Clone() Obligation {
	clone := o
	clone.ChannelAmounts = make(map[ChannelID]Amount, len(o.ChannelAmounts))
	for key, value := range o.ChannelAmounts {
		clone.ChannelAmounts[key] = value
	}
	return clone
}

func (o Obligation) IsOpen() bool {
	return o.Status == ObligationOpen
}

type SettlementStats struct {
	Opened        int    `json:"opened"`
	Settled       int    `json:"settled"`
	Cancelled     int    `json:"cancelled"`
	OpenAmount    Amount `json:"openAmount"`
	SettledAmount Amount `json:"settledAmount"`
	FeeAmount     Amount `json:"feeAmount"`
}

type SettlementBook struct {
	obligations map[ObligationID]*Obligation
	nonce       int
	events      *EventLog
}

func NewSettlementBook(events *EventLog) *SettlementBook {
	return &SettlementBook{
		obligations: map[ObligationID]*Obligation{},
		nonce:       1,
		events:      events,
	}
}

func (b *SettlementBook) Open(epoch Epoch, intent SettlementIntent, ledger *Ledger, registry *OperatorRegistry, routes *RouteRegistry, reputation *ReputationBook) (*Obligation, error) {
	if intent.Asset == "" {
		intent.Asset = DefaultAsset
	}
	if err := NonZeroAmount(intent.Amount, string(intent.Route)); err != nil {
		return nil, err
	}
	route, err := routes.Route(intent.Route)
	if err != nil {
		return nil, err
	}
	if err := routes.Validate(intent.Route, registry, reputation, intent.Amount); err != nil {
		return nil, err
	}
	fee, err := route.FeeFor(intent.Amount)
	if err != nil {
		return nil, err
	}
	gross, err := intent.Amount.Add(fee)
	if err != nil {
		return nil, err
	}
	if err := ledger.ReserveFromAccount(epoch, intent.Payer, intent.Asset, gross, string(intent.Route)); err != nil {
		return nil, err
	}
	channelAmounts := map[ChannelID]Amount{}
	for _, leg := range route.Legs {
		need, err := routeCapacityNeed(intent.Amount, leg.FeeBps)
		if err != nil {
			return nil, err
		}
		if err := registry.Admit(leg.Channel, need); err != nil {
			return nil, err
		}
		if err := registry.MarkReserved(leg.Channel, need); err != nil {
			return nil, err
		}
		channelAmounts[leg.Channel] = need
	}
	id := NewObligationID(epoch, intent.Route, b.nonce)
	b.nonce++
	obligation := &Obligation{
		ID:             id,
		Epoch:          epoch,
		Payer:          intent.Payer,
		Beneficiary:    intent.Beneficiary,
		Asset:          intent.Asset,
		Amount:         intent.Amount,
		Fee:            fee,
		Gross:          gross,
		Route:          intent.Route,
		ChannelAmounts: channelAmounts,
		Status:         ObligationOpen,
		OpenedAt:       epoch,
		ExpiresAt:      epoch + Epoch(route.Policy.SettlementWindow),
		Memo:           intent.Memo,
	}
	b.obligations[id] = obligation
	if b.events != nil {
		b.events.Add(epoch, EventObligationOpened, string(id), eventAttrs("route", string(intent.Route), "amount", amountString(intent.Amount), "fee", amountString(fee)))
	}
	return obligation, nil
}

func (b *SettlementBook) Obligation(id ObligationID) (*Obligation, error) {
	obligation, ok := b.obligations[id]
	if !ok {
		return nil, refError(ErrNotFound, string(id), "obligation not found")
	}
	return obligation, nil
}

func (b *SettlementBook) Settle(epoch Epoch, id ObligationID, ledger *Ledger, registry *OperatorRegistry, routes *RouteRegistry, reputation *ReputationBook) error {
	obligation, err := b.Obligation(id)
	if err != nil {
		return err
	}
	if obligation.Status != ObligationOpen {
		return refError(ErrInvalidState, string(id), "obligation is not open")
	}
	route, err := routes.Route(obligation.Route)
	if err != nil {
		return err
	}
	if err := ledger.ConsumeReserved(epoch, obligation.Payer, obligation.Asset, obligation.Gross, string(id)); err != nil {
		return err
	}
	if err := ledger.Credit(epoch, obligation.Beneficiary, obligation.Asset, obligation.Amount, string(id)); err != nil {
		return err
	}
	feeRemainder := obligation.Fee
	for _, leg := range route.Legs {
		allocated, ok := obligation.ChannelAmounts[leg.Channel]
		if !ok {
			return refError(ErrInvalidState, string(id), "channel allocation missing")
		}
		if err := registry.Settle(leg.Channel, allocated); err != nil {
			return err
		}
		feePart, err := obligation.Amount.MulBpsCeil(leg.FeeBps)
		if err != nil {
			return err
		}
		operator, err := registry.Operator(leg.Operator)
		if err != nil {
			return err
		}
		if feePart > feeRemainder {
			feePart = feeRemainder
		}
		feeRemainder, err = feeRemainder.Sub(feePart)
		if err != nil {
			return err
		}
		if err := ledger.Credit(epoch, operator.Account, obligation.Asset, feePart, string(id)); err != nil {
			return err
		}
		reputation.RewardSettlement(epoch, leg.Operator, obligation.Amount, epoch+1)
	}
	if feeRemainder > 0 {
		if err := ledger.Credit(epoch, AccountID("treasury"), obligation.Asset, feeRemainder, string(id)); err != nil {
			return err
		}
	}
	obligation.Status = ObligationSettled
	obligation.SettledAt = epoch
	if b.events != nil {
		b.events.Add(epoch, EventObligationSettled, string(id), eventAttrs("amount", amountString(obligation.Amount), "fee", amountString(obligation.Fee)))
	}
	return nil
}

func (b *SettlementBook) Cancel(epoch Epoch, id ObligationID, ledger *Ledger, registry *OperatorRegistry, reason string) error {
	obligation, err := b.Obligation(id)
	if err != nil {
		return err
	}
	if obligation.Status != ObligationOpen {
		return refError(ErrInvalidState, string(id), "obligation is not open")
	}
	if err := ledger.ReleaseToAccount(epoch, obligation.Payer, obligation.Asset, obligation.Gross, string(id)); err != nil {
		return err
	}
	for channel, amount := range obligation.ChannelAmounts {
		if err := registry.Release(channel, amount); err != nil {
			return err
		}
	}
	obligation.Status = ObligationCancelled
	obligation.SettledAt = epoch
	if b.events != nil {
		b.events.Warning(epoch, EventObligationClosed, string(id), eventAttrs("reason", reason))
	}
	return nil
}

func (b *SettlementBook) Expire(epoch Epoch, ledger *Ledger, registry *OperatorRegistry, reputation *ReputationBook, routes *RouteRegistry) error {
	for _, obligation := range b.openRows() {
		if obligation.ExpiresAt > epoch {
			continue
		}
		if err := ledger.ReleaseToAccount(epoch, obligation.Payer, obligation.Asset, obligation.Gross, string(obligation.ID)); err != nil {
			return err
		}
		route, err := routes.Route(obligation.Route)
		if err != nil {
			return err
		}
		for _, leg := range route.Legs {
			amount := obligation.ChannelAmounts[leg.Channel]
			if err := registry.Release(leg.Channel, amount); err != nil {
				return err
			}
			reputation.PenalizeMissedWindow(epoch, leg.Operator, obligation.Amount, epoch+1)
		}
		live, _ := b.Obligation(obligation.ID)
		live.Status = ObligationExpired
		live.SettledAt = epoch
		if b.events != nil {
			b.events.Warning(epoch, EventObligationClosed, string(obligation.ID), eventAttrs("reason", "expired"))
		}
	}
	return nil
}

func (b *SettlementBook) OpenRows() []Obligation {
	return b.openRows()
}

func (b *SettlementBook) openRows() []Obligation {
	rows := []Obligation{}
	for _, obligation := range b.obligations {
		if obligation.Status == ObligationOpen {
			rows = append(rows, obligation.Clone())
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows
}

func (b *SettlementBook) Rows() []Obligation {
	rows := make([]Obligation, 0, len(b.obligations))
	for _, obligation := range b.obligations {
		rows = append(rows, obligation.Clone())
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows
}

func (b *SettlementBook) Stats() SettlementStats {
	stats := SettlementStats{}
	for _, obligation := range b.obligations {
		switch obligation.Status {
		case ObligationOpen:
			stats.Opened++
			stats.OpenAmount += obligation.Amount
		case ObligationSettled:
			stats.Settled++
			stats.SettledAmount += obligation.Amount
			stats.FeeAmount += obligation.Fee
		case ObligationCancelled, ObligationExpired:
			stats.Cancelled++
		}
	}
	return stats
}

func (b *SettlementBook) ExposureByOperator(routes *RouteRegistry) map[OperatorID]Amount {
	result := map[OperatorID]Amount{}
	for _, obligation := range b.obligations {
		if obligation.Status != ObligationOpen {
			continue
		}
		route, err := routes.Route(obligation.Route)
		if err != nil {
			continue
		}
		for _, leg := range route.Legs {
			result[leg.Operator] += obligation.ChannelAmounts[leg.Channel]
		}
	}
	return result
}

func (b *SettlementBook) ExposureByChannel() map[ChannelID]Amount {
	result := map[ChannelID]Amount{}
	for _, obligation := range b.obligations {
		if obligation.Status != ObligationOpen {
			continue
		}
		for channel, amount := range obligation.ChannelAmounts {
			result[channel] += amount
		}
	}
	return result
}

func (b *SettlementBook) SettleAll(epoch Epoch, ledger *Ledger, registry *OperatorRegistry, routes *RouteRegistry, reputation *ReputationBook) error {
	for _, obligation := range b.OpenRows() {
		if err := b.Settle(epoch, obligation.ID, ledger, registry, routes, reputation); err != nil {
			return err
		}
	}
	return nil
}
