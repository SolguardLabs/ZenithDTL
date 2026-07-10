package zenithdtl

import "sort"

type ScenarioDescriptor struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Focus       string `json:"focus"`
}

type scenarioRefs struct {
	Atlas         OperatorID
	Boreal        OperatorID
	Cypher        OperatorID
	Delta         OperatorID
	UserA         AccountID
	UserB         AccountID
	UserC         AccountID
	Treasury      AccountID
	AtlasAccount  AccountID
	BorealAccount AccountID
	CypherAccount AccountID
	DeltaAccount  AccountID
	AtlasVault    VaultID
	BorealVault   VaultID
	CypherVault   VaultID
	DeltaVault    VaultID
	AtlasAB       ChannelID
	AtlasBA       ChannelID
	BorealBG      ChannelID
	BorealGB      ChannelID
	CypherGD      ChannelID
	DeltaAD       ChannelID
	RouteAB       RouteID
	RouteBA       RouteID
	RouteAG       RouteID
	RouteAD       RouteID
	RouteGD       RouteID
}

func ListScenarios() []ScenarioDescriptor {
	scenarios := []ScenarioDescriptor{
		{
			Name:        "balanced-reputation",
			Description: "multi-operator settlement with reputation-weighted capacity bands",
			Focus:       "reputation",
		},
		{
			Name:        "capacity-limits",
			Description: "admission checks across warm and hot settlement channels",
			Focus:       "limits",
		},
		{
			Name:        "penalty-cycle",
			Description: "expired settlement windows and deferred score adjustments",
			Focus:       "penalties",
		},
		{
			Name:        "route-withdrawal",
			Description: "route validation with vault withdrawal lifecycle",
			Focus:       "routes-withdrawals",
		},
	}
	sort.Slice(scenarios, func(i, j int) bool { return scenarios[i].Name < scenarios[j].Name })
	return scenarios
}

func RunScenario(name string) (*Protocol, error) {
	switch name {
	case "balanced-reputation":
		return scenarioBalancedReputation()
	case "capacity-limits":
		return scenarioCapacityLimits()
	case "penalty-cycle":
		return scenarioPenaltyCycle()
	case "route-withdrawal":
		return scenarioRouteWithdrawal()
	default:
		return nil, refError(ErrNotFound, name, "scenario not found")
	}
}

func RunScenarioReport(name string, includeEvents bool) (Report, error) {
	protocol, err := RunScenario(name)
	if err != nil {
		return Report{}, err
	}
	report := BuildReport(protocol, name, includeEvents)
	return report, ValidateReport(report)
}

func ValidateScenario(name string) ValidationResult {
	protocol, err := RunScenario(name)
	if err != nil {
		return ValidationResult{OK: false, Scenario: name, Errors: []string{err.Error()}}
	}
	return ValidateProtocol(protocol, name)
}

func seedNetwork() (*Protocol, scenarioRefs, error) {
	config := DefaultConfig()
	config.NetworkID = "zenith-local-audit"
	config.Planner.MinLimit = 10_000
	config.Planner.MaxLimit = 450_000
	protocol := NewProtocol(config)
	refs := scenarioRefs{
		Atlas:         OperatorID("atlas"),
		Boreal:        OperatorID("boreal"),
		Cypher:        OperatorID("cypher"),
		Delta:         OperatorID("delta"),
		UserA:         AccountID("client-alpha"),
		UserB:         AccountID("merchant-beta"),
		UserC:         AccountID("clearing-gamma"),
		Treasury:      AccountID("treasury"),
		AtlasAccount:  AccountID("op-atlas"),
		BorealAccount: AccountID("op-boreal"),
		CypherAccount: AccountID("op-cypher"),
		DeltaAccount:  AccountID("op-delta"),
	}
	if err := protocol.RegisterOperator(refs.Atlas, refs.AtlasAccount, 930, 180_000, 110_000, []DomainID{"alpha", "beta", "gamma"}, 7); err != nil {
		return nil, refs, err
	}
	if err := protocol.RegisterOperator(refs.Boreal, refs.BorealAccount, 790, 125_000, 92_000, []DomainID{"beta", "gamma"}, 5); err != nil {
		return nil, refs, err
	}
	if err := protocol.RegisterOperator(refs.Cypher, refs.CypherAccount, 645, 70_000, 74_000, []DomainID{"gamma", "delta"}, 4); err != nil {
		return nil, refs, err
	}
	if err := protocol.RegisterOperator(refs.Delta, refs.DeltaAccount, 560, 45_000, 58_000, []DomainID{"alpha", "delta"}, 3); err != nil {
		return nil, refs, err
	}
	if err := protocol.Deposit(refs.UserA, 1_200_000, "client-prefund"); err != nil {
		return nil, refs, err
	}
	if err := protocol.Deposit(refs.UserB, 320_000, "merchant-prefund"); err != nil {
		return nil, refs, err
	}
	if err := protocol.Deposit(refs.UserC, 240_000, "clearing-prefund"); err != nil {
		return nil, refs, err
	}
	var err error
	refs.AtlasVault, err = protocol.OpenVault("alpha", refs.AtlasAccount, "atlas-primary", 360_000)
	if err != nil {
		return nil, refs, err
	}
	refs.BorealVault, err = protocol.OpenVault("beta", refs.BorealAccount, "boreal-primary", 280_000)
	if err != nil {
		return nil, refs, err
	}
	refs.CypherVault, err = protocol.OpenVault("gamma", refs.CypherAccount, "cypher-primary", 210_000)
	if err != nil {
		return nil, refs, err
	}
	refs.DeltaVault, err = protocol.OpenVault("delta", refs.DeltaAccount, "delta-primary", 160_000)
	if err != nil {
		return nil, refs, err
	}
	refs.AtlasAB, err = protocol.OpenChannel(refs.Atlas, "alpha", "beta", "atlas-ab", ChannelHot, 120_000, 8)
	if err != nil {
		return nil, refs, err
	}
	refs.AtlasBA, err = protocol.OpenChannel(refs.Atlas, "beta", "alpha", "atlas-ba", ChannelWarm, 90_000, 6)
	if err != nil {
		return nil, refs, err
	}
	refs.BorealBG, err = protocol.OpenChannel(refs.Boreal, "beta", "gamma", "boreal-bg", ChannelHot, 100_000, 7)
	if err != nil {
		return nil, refs, err
	}
	refs.BorealGB, err = protocol.OpenChannel(refs.Boreal, "gamma", "beta", "boreal-gb", ChannelWarm, 75_000, 5)
	if err != nil {
		return nil, refs, err
	}
	refs.CypherGD, err = protocol.OpenChannel(refs.Cypher, "gamma", "delta", "cypher-gd", ChannelWarm, 70_000, 4)
	if err != nil {
		return nil, refs, err
	}
	refs.DeltaAD, err = protocol.OpenChannel(refs.Delta, "alpha", "delta", "delta-ad", ChannelBridge, 48_000, 3)
	if err != nil {
		return nil, refs, err
	}
	protocol.BootstrapCapacity()
	refs.RouteAB, err = protocol.RegisterSingleHopRoute(refs.AtlasAB, "atlas-direct", 75, 650)
	if err != nil {
		return nil, refs, err
	}
	refs.RouteBA, err = protocol.RegisterSingleHopRoute(refs.AtlasBA, "atlas-return", 70, 650)
	if err != nil {
		return nil, refs, err
	}
	refs.RouteAG, err = protocol.RegisterTwoHopRoute(refs.AtlasAB, refs.BorealBG, "alpha-gamma-mesh", 95, 620)
	if err != nil {
		return nil, refs, err
	}
	refs.RouteAD, err = protocol.RegisterSingleHopRoute(refs.DeltaAD, "delta-bridge", 95, 500)
	if err != nil {
		return nil, refs, err
	}
	refs.RouteGD, err = protocol.RegisterSingleHopRoute(refs.CypherGD, "cypher-forward", 90, 550)
	if err != nil {
		return nil, refs, err
	}
	return protocol, refs, nil
}

func scenarioBalancedReputation() (*Protocol, error) {
	protocol, refs, err := seedNetwork()
	if err != nil {
		return nil, err
	}
	first, err := protocol.OpenObligation(SettlementIntent{
		Payer:       refs.UserA,
		Beneficiary: refs.UserB,
		Amount:      65_000,
		Route:       refs.RouteAB,
		Memo:        "merchant settlement batch a",
	})
	if err != nil {
		return nil, err
	}
	second, err := protocol.OpenObligation(SettlementIntent{
		Payer:       refs.UserA,
		Beneficiary: refs.UserC,
		Amount:      42_000,
		Route:       refs.RouteAG,
		Memo:        "clearing transfer",
	})
	if err != nil {
		return nil, err
	}
	if err := protocol.Settle(first.ID); err != nil {
		return nil, err
	}
	if err := protocol.Settle(second.ID); err != nil {
		return nil, err
	}
	if err := protocol.RecordPerformance(refs.Atlas, 7, 0, 107_000, 1_200); err != nil {
		return nil, err
	}
	if err := protocol.RecordPerformance(refs.Boreal, 3, 0, 42_000, 1_700); err != nil {
		return nil, err
	}
	if err := protocol.CloseEpoch(); err != nil {
		return nil, err
	}
	if err := protocol.PayReadyWithdrawals(); err != nil {
		return nil, err
	}
	return protocol, nil
}

func scenarioCapacityLimits() (*Protocol, error) {
	protocol, refs, err := seedNetwork()
	if err != nil {
		return nil, err
	}
	one, err := protocol.OpenObligation(SettlementIntent{
		Payer:       refs.UserA,
		Beneficiary: refs.UserB,
		Amount:      58_000,
		Route:       refs.RouteAB,
		Memo:        "same-day card settlement",
	})
	if err != nil {
		return nil, err
	}
	two, err := protocol.OpenObligation(SettlementIntent{
		Payer:       refs.UserA,
		Beneficiary: refs.UserC,
		Amount:      37_000,
		Route:       refs.RouteAG,
		Memo:        "gamma treasury top-up",
	})
	if err != nil {
		return nil, err
	}
	if err := protocol.Settle(one.ID); err != nil {
		return nil, err
	}
	if err := protocol.RecordPerformance(refs.Atlas, 2, 0, 58_000, 1_050); err != nil {
		return nil, err
	}
	if err := protocol.RecordPerformance(refs.Boreal, 1, 0, 37_000, 1_900); err != nil {
		return nil, err
	}
	if _, err := protocol.OpenObligation(SettlementIntent{
		Payer:       refs.UserB,
		Beneficiary: refs.UserA,
		Amount:      18_000,
		Route:       refs.RouteBA,
		Memo:        "partial return",
	}); err != nil {
		return nil, err
	}
	_ = two
	return protocol, nil
}

func scenarioPenaltyCycle() (*Protocol, error) {
	protocol, refs, err := seedNetwork()
	if err != nil {
		return nil, err
	}
	if _, err := protocol.OpenObligation(SettlementIntent{
		Payer:       refs.UserA,
		Beneficiary: refs.UserC,
		Amount:      24_000,
		Route:       refs.RouteGD,
		Memo:        "gamma-delta scheduled release",
	}); err != nil {
		return nil, err
	}
	if err := protocol.RecordPerformance(refs.Cypher, 1, 1, 24_000, 3_200); err != nil {
		return nil, err
	}
	if err := protocol.CloseEpoch(); err != nil {
		return nil, err
	}
	if err := protocol.CloseEpoch(); err != nil {
		return nil, err
	}
	if err := protocol.CloseEpoch(); err != nil {
		return nil, err
	}
	if err := protocol.SetOperatorStatus(refs.Cypher, OperatorWatchlist); err != nil {
		return nil, err
	}
	return protocol, nil
}

func scenarioRouteWithdrawal() (*Protocol, error) {
	protocol, refs, err := seedNetwork()
	if err != nil {
		return nil, err
	}
	obligation, err := protocol.OpenObligation(SettlementIntent{
		Payer:       refs.UserA,
		Beneficiary: refs.UserC,
		Amount:      31_000,
		Route:       refs.RouteAG,
		Memo:        "route audit payment",
	})
	if err != nil {
		return nil, err
	}
	if err := protocol.Settle(obligation.ID); err != nil {
		return nil, err
	}
	if _, err := protocol.QueueWithdrawal(refs.AtlasAccount, refs.AtlasVault, 42_000, "operator-liquidity-cycle"); err != nil {
		return nil, err
	}
	if _, err := protocol.QueueWithdrawal(refs.BorealAccount, refs.BorealVault, 35_000, "operator-liquidity-cycle"); err != nil {
		return nil, err
	}
	if err := protocol.CloseEpoch(); err != nil {
		return nil, err
	}
	if err := protocol.PayReadyWithdrawals(); err != nil {
		return nil, err
	}
	if err := protocol.ValidateRoute(refs.RouteAG, 12_000); err != nil {
		return nil, err
	}
	return protocol, nil
}
