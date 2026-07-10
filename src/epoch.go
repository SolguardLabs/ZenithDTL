package zenithdtl

type EpochStatus string

const (
	EpochOpen   EpochStatus = "open"
	EpochClosed EpochStatus = "closed"
)

type EpochRecord struct {
	Epoch              Epoch       `json:"epoch"`
	Status             EpochStatus `json:"status"`
	OpenedObligations  int         `json:"openedObligations"`
	SettledObligations int         `json:"settledObligations"`
	OpenAmount         Amount      `json:"openAmount"`
	SettledAmount      Amount      `json:"settledAmount"`
	Fees               Amount      `json:"fees"`
	PendingWithdrawals Amount      `json:"pendingWithdrawals"`
	PlanEntries        int         `json:"planEntries"`
}

func (p *Protocol) CloseEpoch() error {
	closing := p.Epoch
	if err := p.Settlements.Expire(closing, p.Ledger, p.Operators, p.Reputation, p.Routes); err != nil {
		return err
	}
	p.Withdrawals.MarkReady(closing)
	next := closing + 1
	plan := p.PlanCapacity(next)
	p.ApplyCapacityPlan(plan)
	p.Reputation.ApplyDue(next)
	p.RefreshRecommendations(next)
	p.Operators.ApplyCapacityToIdleChannels(next, p.Reputation)
	p.ClosedEpoch = closing
	p.Epoch = next
	if p.Events != nil {
		stats := p.Settlements.Stats()
		p.Events.Add(closing, EventEpochClosed, ProtocolName, eventAttrs("openAmount", amountString(stats.OpenAmount), "settledAmount", amountString(stats.SettledAmount)))
		p.Events.Add(next, EventEpochOpened, ProtocolName, eventAttrs("previous", epochString(closing)))
	}
	return nil
}

func (p *Protocol) AdvanceEpochs(count int) error {
	for i := 0; i < count; i++ {
		if err := p.CloseEpoch(); err != nil {
			return err
		}
	}
	return nil
}

func (p *Protocol) EpochRecord() EpochRecord {
	stats := p.Settlements.Stats()
	planEntries := 0
	if plan, ok := p.CurrentPlan(); ok {
		planEntries = len(plan.Entries)
	}
	return EpochRecord{
		Epoch:              p.Epoch,
		Status:             EpochOpen,
		OpenedObligations:  stats.Opened,
		SettledObligations: stats.Settled,
		OpenAmount:         stats.OpenAmount,
		SettledAmount:      stats.SettledAmount,
		Fees:               stats.FeeAmount,
		PendingWithdrawals: p.Withdrawals.PendingAmount(p.Config.Asset),
		PlanEntries:        planEntries,
	}
}

func (p *Protocol) BootstrapCapacity() {
	plan := p.PlanCapacity(p.Epoch)
	p.ApplyCapacityPlan(plan)
	p.Reputation.ApplyPositiveDue(p.Epoch)
	p.RefreshRecommendations(p.Epoch)
}
