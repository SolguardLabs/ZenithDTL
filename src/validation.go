package zenithdtl

type ValidationResult struct {
	OK       bool     `json:"ok"`
	Scenario string   `json:"scenario"`
	Checks   []string `json:"checks"`
	Errors   []string `json:"errors"`
}

func ValidateProtocol(p *Protocol, scenario string) ValidationResult {
	result := ValidationResult{
		OK:       true,
		Scenario: scenario,
		Checks:   []string{},
		Errors:   []string{},
	}
	result.check("ledger conservation", p.Ledger.Conservation(p.Config.Asset), "ledger conservation failed")
	result.check("routes registered", len(p.Routes.Routes()) > 0, "no routes registered")
	result.check("operators registered", len(p.Operators.OperatorRows()) > 0, "no operators registered")
	result.check("channels registered", len(p.Operators.ChannelRows()) > 0, "no channels registered")
	for _, route := range p.Routes.Routes() {
		ok := route.Policy.Asset == p.Config.Asset && route.HopCount() > 0 && route.TotalFeeBps() <= route.Policy.MaxFeeBps
		result.check("route "+string(route.ID), ok, "route validation failed: "+string(route.ID))
	}
	for _, channel := range p.Operators.ChannelRows() {
		ok := channel.Limit >= channel.ActiveDebt()
		result.check("channel "+string(channel.ID), ok, "channel limit below active debt: "+string(channel.ID))
	}
	for _, withdrawal := range p.Withdrawals.Rows() {
		ok := withdrawal.Amount > 0 && withdrawal.Asset == p.Config.Asset
		result.check("withdrawal "+string(withdrawal.ID), ok, "withdrawal row invalid: "+string(withdrawal.ID))
	}
	return result
}

func (r *ValidationResult) check(name string, ok bool, message string) {
	if ok {
		r.Checks = append(r.Checks, name)
		return
	}
	r.OK = false
	r.Errors = append(r.Errors, message)
}

func (r ValidationResult) Error() error {
	if r.OK {
		return nil
	}
	if len(r.Errors) == 0 {
		return newError(ErrInvalidState, "validation failed")
	}
	return newError(ErrInvalidState, r.Errors[0])
}
