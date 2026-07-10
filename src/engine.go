package zenithdtl

type ProtocolConfig struct {
	NetworkID        string          `json:"networkId"`
	Asset            AssetID         `json:"asset"`
	StartingEpoch    Epoch           `json:"startingEpoch"`
	WithdrawalHold   Epoch           `json:"withdrawalHold"`
	Planner          CapacityPlanner `json:"planner"`
	Treasury         AccountID       `json:"treasury"`
	ConservativeMode bool            `json:"conservativeMode"`
}

func DefaultConfig() ProtocolConfig {
	return ProtocolConfig{
		NetworkID:      "zenith-simnet",
		Asset:          DefaultAsset,
		StartingEpoch:  1,
		WithdrawalHold: 1,
		Planner:        DefaultCapacityPlanner(),
		Treasury:       AccountID("treasury"),
	}
}

type Protocol struct {
	Config      ProtocolConfig
	Epoch       Epoch
	ClosedEpoch Epoch
	Events      *EventLog
	Ledger      *Ledger
	Reputation  *ReputationBook
	Operators   *OperatorRegistry
	Routes      *RouteRegistry
	Settlements *SettlementBook
	Withdrawals *WithdrawalBook
	Plans       []CapacityPlan
}

func NewProtocol(config ProtocolConfig) *Protocol {
	if config.Asset == "" {
		config.Asset = DefaultAsset
	}
	if config.StartingEpoch == 0 {
		config.StartingEpoch = 1
	}
	if config.WithdrawalHold == 0 {
		config.WithdrawalHold = 1
	}
	if config.Treasury == "" {
		config.Treasury = AccountID("treasury")
	}
	events := NewEventLog()
	protocol := &Protocol{
		Config:      config,
		Epoch:       config.StartingEpoch,
		ClosedEpoch: 0,
		Events:      events,
		Ledger:      NewLedger(events),
		Reputation:  NewReputationBook(events),
		Operators:   NewOperatorRegistry(events),
		Routes:      NewRouteRegistry(events),
		Settlements: NewSettlementBook(events),
		Withdrawals: NewWithdrawalBook(events),
		Plans:       []CapacityPlan{},
	}
	protocol.Ledger.EnsureAccount(config.Treasury, config.Asset)
	protocol.Events.Add(protocol.Epoch, EventEpochOpened, ProtocolName, eventAttrs("network", config.NetworkID))
	return protocol
}

func (p *Protocol) RegisterOperator(id OperatorID, account AccountID, score int64, stake Amount, base Amount, domains []DomainID, maxChannels int) error {
	if account == "" {
		account = AccountID(id)
	}
	p.Ledger.EnsureAccount(account, p.Config.Asset)
	p.Reputation.Register(id, score)
	return p.Operators.RegisterOperator(Operator{
		ID:              id,
		Account:         account,
		Status:          OperatorActive,
		Stake:           stake,
		BaseCapacity:    base,
		MaxChannels:     maxChannels,
		RegisteredEpoch: p.Epoch,
		Domains:         domains,
		Metadata:        map[string]string{"network": p.Config.NetworkID},
	})
}

func (p *Protocol) OpenVault(domain DomainID, owner AccountID, suffix string, funding Amount) (VaultID, error) {
	id := NewVaultID(domain, p.Config.Asset, suffix)
	if _, err := p.Ledger.OpenVault(id, domain, p.Config.Asset, owner); err != nil {
		return "", err
	}
	if funding > 0 {
		if err := p.Ledger.FundVault(p.Epoch, id, funding, "initial-funding"); err != nil {
			return "", err
		}
	}
	return id, nil
}

func (p *Protocol) OpenChannel(operator OperatorID, origin DomainID, destination DomainID, suffix string, mode ChannelMode, limit Amount, priority int) (ChannelID, error) {
	id := NewChannelID(operator, origin, destination, suffix)
	channel := Channel{
		ID:            id,
		Operator:      operator,
		Origin:        origin,
		Destination:   destination,
		Asset:         p.Config.Asset,
		Mode:          mode,
		State:         ChannelOpen,
		Epoch:         p.Epoch,
		Limit:         limit,
		LastHeartbeat: p.Epoch,
		Priority:      priority,
		Tags:          map[string]string{"network": p.Config.NetworkID},
	}
	if err := p.Operators.OpenChannel(channel); err != nil {
		return "", err
	}
	return id, nil
}

func (p *Protocol) RegisterSingleHopRoute(channelID ChannelID, name string, maxFee BasisPoints, minScore int64) (RouteID, error) {
	channel, err := p.Operators.Channel(channelID)
	if err != nil {
		return "", err
	}
	id := NewRouteID(channel.Origin, channel.Destination, name)
	route := BuildSingleHopRoute(id, name, channel.Clone(), maxFee, minScore, p.Epoch)
	if err := p.Routes.Register(route); err != nil {
		return "", err
	}
	return id, nil
}

func (p *Protocol) RegisterTwoHopRoute(firstID ChannelID, secondID ChannelID, name string, maxFee BasisPoints, minScore int64) (RouteID, error) {
	first, err := p.Operators.Channel(firstID)
	if err != nil {
		return "", err
	}
	second, err := p.Operators.Channel(secondID)
	if err != nil {
		return "", err
	}
	id := NewRouteID(first.Origin, second.Destination, name)
	route := BuildTwoHopRoute(id, name, first.Clone(), second.Clone(), maxFee, minScore, p.Epoch)
	if err := p.Routes.Register(route); err != nil {
		return "", err
	}
	return id, nil
}

func (p *Protocol) Deposit(account AccountID, amount Amount, ref string) error {
	return p.Ledger.Deposit(p.Epoch, account, p.Config.Asset, amount, ref)
}

func (p *Protocol) OpenObligation(intent SettlementIntent) (*Obligation, error) {
	if intent.Asset == "" {
		intent.Asset = p.Config.Asset
	}
	return p.Settlements.Open(p.Epoch, intent, p.Ledger, p.Operators, p.Routes, p.Reputation)
}

func (p *Protocol) Settle(id ObligationID) error {
	return p.Settlements.Settle(p.Epoch, id, p.Ledger, p.Operators, p.Routes, p.Reputation)
}

func (p *Protocol) SettleAllOpen() error {
	return p.Settlements.SettleAll(p.Epoch, p.Ledger, p.Operators, p.Routes, p.Reputation)
}

func (p *Protocol) QueueWithdrawal(account AccountID, vault VaultID, amount Amount, ref string) (*WithdrawalRequest, error) {
	return p.Withdrawals.Queue(p.Epoch, account, vault, amount, p.Ledger, p.Config.WithdrawalHold, ref)
}

func (p *Protocol) PayReadyWithdrawals() error {
	return p.Withdrawals.PayReady(p.Epoch, p.Ledger)
}

func (p *Protocol) PlanCapacity(epoch Epoch) CapacityPlan {
	plan := p.Config.Planner.Plan(epoch, p.Operators, p.Reputation)
	p.Plans = append(p.Plans, plan)
	if p.Events != nil {
		for _, entry := range plan.Entries {
			p.Events.Add(epoch, EventCapacityPlanned, string(entry.Operator), eventAttrs("limit", amountString(entry.Limit), "score", amountString(Amount(entry.Score)), "band", entry.Band))
		}
	}
	return plan
}

func (p *Protocol) ApplyCapacityPlan(plan CapacityPlan) {
	p.Operators.ApplyCapacityPlan(plan)
}

func (p *Protocol) RefreshRecommendations(epoch Epoch) {
	for _, operator := range p.Operators.OperatorRows() {
		limit := p.Reputation.RecommendedCapacity(operator.ID, operator.BaseCapacity, operator.Stake, operator.RouteWeight())
		_ = p.Operators.SetRecommendedLimit(epoch, operator.ID, limit)
	}
}

func (p *Protocol) CurrentPlan() (CapacityPlan, bool) {
	if len(p.Plans) == 0 {
		return CapacityPlan{}, false
	}
	return p.Plans[len(p.Plans)-1], true
}

func (p *Protocol) ValidateRoute(route RouteID, amount Amount) error {
	return p.Routes.Validate(route, p.Operators, p.Reputation, amount)
}

func (p *Protocol) OperatorExposure(operator OperatorID) Amount {
	exposures := p.Settlements.ExposureByOperator(p.Routes)
	return exposures[operator]
}

func (p *Protocol) ChannelExposure(channel ChannelID) Amount {
	exposures := p.Settlements.ExposureByChannel()
	return exposures[channel]
}

func (p *Protocol) SetOperatorStatus(operator OperatorID, status OperatorStatus) error {
	return p.Operators.UpdateStatus(p.Epoch, operator, status)
}

func (p *Protocol) ManualReview(operator OperatorID, severity Severity) {
	p.Reputation.MarkManualReview(p.Epoch, operator, severity, p.Epoch+1)
}

func (p *Protocol) RecordPerformance(operator OperatorID, success int64, failed int64, volume Amount, avgMillis int64) error {
	return p.Reputation.RecordPerformance(p.Epoch, operator, success, failed, volume, avgMillis)
}
