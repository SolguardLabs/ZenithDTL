package zenithdtl

import "sort"

type OperatorReport struct {
	ID                  OperatorID     `json:"id"`
	Account             AccountID      `json:"account"`
	Status              OperatorStatus `json:"status"`
	Score               int64          `json:"score"`
	Band                string         `json:"band"`
	PendingPositive     int64          `json:"pendingPositive"`
	PendingNegative     int64          `json:"pendingNegative"`
	Stake               Amount         `json:"stake"`
	BaseCapacity        Amount         `json:"baseCapacity"`
	AssignedCapacity    Amount         `json:"assignedCapacity"`
	RecommendedCapacity Amount         `json:"recommendedCapacity"`
	ActiveDebt          Amount         `json:"activeDebt"`
	Channels            int            `json:"channels"`
}

type ChannelReport struct {
	ID             ChannelID    `json:"id"`
	Operator       OperatorID   `json:"operator"`
	Origin         DomainID     `json:"origin"`
	Destination    DomainID     `json:"destination"`
	Asset          AssetID      `json:"asset"`
	Mode           ChannelMode  `json:"mode"`
	State          ChannelState `json:"state"`
	Epoch          Epoch        `json:"epoch"`
	Limit          Amount       `json:"limit"`
	Used           Amount       `json:"used"`
	Reserved       Amount       `json:"reserved"`
	InFlight       Amount       `json:"inFlight"`
	Available      Amount       `json:"available"`
	UtilizationBps int64        `json:"utilizationBps"`
}

type RouteReport struct {
	ID          RouteID     `json:"id"`
	Name        string      `json:"name"`
	Origin      DomainID    `json:"origin"`
	Destination DomainID    `json:"destination"`
	Asset       AssetID     `json:"asset"`
	Enabled     bool        `json:"enabled"`
	Hops        int         `json:"hops"`
	FeeBps      BasisPoints `json:"feeBps"`
	Path        []DomainID  `json:"path"`
}

type SummaryReport struct {
	ConservationOk       bool            `json:"conservationOk"`
	CurrentEpoch         Epoch           `json:"currentEpoch"`
	AccountFree          Amount          `json:"accountFree"`
	AccountReserved      Amount          `json:"accountReserved"`
	VaultCash            Amount          `json:"vaultCash"`
	VaultReserved        Amount          `json:"vaultReserved"`
	PendingWithdrawals   Amount          `json:"pendingWithdrawals"`
	CompletedWithdrawals Amount          `json:"completedWithdrawals"`
	Settlement           SettlementStats `json:"settlement"`
	EventCounts          map[string]int  `json:"eventCounts"`
	PrimeOperators       []OperatorID    `json:"primeOperators"`
}

type Report struct {
	Protocol    string              `json:"protocol"`
	Network     string              `json:"network"`
	Scenario    string              `json:"scenario"`
	Epoch       Epoch               `json:"epoch"`
	EpochRecord EpochRecord         `json:"epochRecord"`
	Accounts    []AccountBalance    `json:"accounts"`
	Vaults      []Vault             `json:"vaults"`
	Operators   []OperatorReport    `json:"operators"`
	Channels    []ChannelReport     `json:"channels"`
	Routes      []RouteReport       `json:"routes"`
	Obligations []Obligation        `json:"obligations"`
	Withdrawals []WithdrawalRequest `json:"withdrawals"`
	Reputation  []ScoreCard         `json:"reputation"`
	Plans       []CapacityPlan      `json:"plans"`
	Summary     SummaryReport       `json:"summary"`
	Events      []Event             `json:"events,omitempty"`
}

func BuildReport(p *Protocol, scenario string, includeEvents bool) Report {
	operators := buildOperatorReports(p)
	channels := buildChannelReports(p)
	routes := buildRouteReports(p)
	events := []Event(nil)
	if includeEvents {
		events = p.Events.All()
	}
	summary := SummaryReport{
		ConservationOk:       p.Ledger.Conservation(p.Config.Asset),
		CurrentEpoch:         p.Epoch,
		AccountFree:          p.Ledger.TotalAccountFree(p.Config.Asset),
		AccountReserved:      p.Ledger.TotalAccountReserved(p.Config.Asset),
		VaultCash:            p.Ledger.TotalVaultCash(p.Config.Asset),
		VaultReserved:        p.Ledger.TotalVaultReserved(p.Config.Asset),
		PendingWithdrawals:   p.Withdrawals.PendingAmount(p.Config.Asset),
		CompletedWithdrawals: p.Withdrawals.CompletedAmount(p.Config.Asset),
		Settlement:           p.Settlements.Stats(),
		EventCounts:          p.Events.Summary(),
		PrimeOperators:       p.Reputation.PrimeOperators(760),
	}
	return Report{
		Protocol:    ProtocolName,
		Network:     p.Config.NetworkID,
		Scenario:    scenario,
		Epoch:       p.Epoch,
		EpochRecord: p.EpochRecord(),
		Accounts:    p.Ledger.AccountRows(),
		Vaults:      p.Ledger.VaultRows(),
		Operators:   operators,
		Channels:    channels,
		Routes:      routes,
		Obligations: p.Settlements.Rows(),
		Withdrawals: p.Withdrawals.Rows(),
		Reputation:  p.Reputation.Cards(),
		Plans:       clonePlans(p.Plans),
		Summary:     summary,
		Events:      events,
	}
}

func buildOperatorReports(p *Protocol) []OperatorReport {
	rows := []OperatorReport{}
	for _, operator := range p.Operators.OperatorRows() {
		rows = append(rows, OperatorReport{
			ID:                  operator.ID,
			Account:             operator.Account,
			Status:              operator.Status,
			Score:               p.Reputation.Score(operator.ID),
			Band:                p.Reputation.Band(operator.ID),
			PendingPositive:     p.Reputation.PendingPositive(operator.ID),
			PendingNegative:     p.Reputation.PendingNegative(operator.ID),
			Stake:               operator.Stake,
			BaseCapacity:        operator.BaseCapacity,
			AssignedCapacity:    p.Operators.AssignedLimit(operator.ID),
			RecommendedCapacity: operator.RecommendedLimit,
			ActiveDebt:          p.Operators.ActiveDebt(operator.ID),
			Channels:            len(p.Operators.ChannelsForOperator(operator.ID)),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows
}

func buildChannelReports(p *Protocol) []ChannelReport {
	rows := []ChannelReport{}
	for _, channel := range p.Operators.ChannelRows() {
		rows = append(rows, ChannelReport{
			ID:             channel.ID,
			Operator:       channel.Operator,
			Origin:         channel.Origin,
			Destination:    channel.Destination,
			Asset:          channel.Asset,
			Mode:           channel.Mode,
			State:          channel.State,
			Epoch:          channel.Epoch,
			Limit:          channel.Limit,
			Used:           channel.Used,
			Reserved:       channel.Reserved,
			InFlight:       channel.InFlight,
			Available:      channel.Available(),
			UtilizationBps: channel.UtilizationBps(),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows
}

func buildRouteReports(p *Protocol) []RouteReport {
	rows := []RouteReport{}
	for _, route := range p.Routes.Routes() {
		rows = append(rows, RouteReport{
			ID:          route.ID,
			Name:        route.Name,
			Origin:      route.Policy.Origin,
			Destination: route.Policy.Destination,
			Asset:       route.Policy.Asset,
			Enabled:     route.Enabled,
			Hops:        route.HopCount(),
			FeeBps:      route.TotalFeeBps(),
			Path:        route.Path(),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows
}

func clonePlans(plans []CapacityPlan) []CapacityPlan {
	result := make([]CapacityPlan, len(plans))
	for i, plan := range plans {
		result[i].Epoch = plan.Epoch
		result[i].Entries = make([]CapacityEntry, len(plan.Entries))
		copy(result[i].Entries, plan.Entries)
	}
	return result
}

func ValidateReport(report Report) error {
	if report.Protocol != ProtocolName {
		return newError(ErrInvalidState, "unexpected protocol")
	}
	if report.Network == "" || report.Scenario == "" {
		return newError(ErrInvalidState, "report identity missing")
	}
	if !report.Summary.ConservationOk {
		return newError(ErrInvalidState, "ledger conservation failed")
	}
	for _, route := range report.Routes {
		if route.Hops <= 0 || len(route.Path) < 2 {
			return refError(ErrInvalidRoute, string(route.ID), "route report path is incomplete")
		}
	}
	return nil
}
