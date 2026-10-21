package zenithdtl

import "sort"

// CapacityDigest reconciles the operator view with its channel book.
type CapacityDigest struct {
	Operator           OperatorID `json:"operator"`
	Epoch              Epoch      `json:"epoch"`
	Score              int64      `json:"score"`
	Band               string     `json:"band"`
	PlannedLimit       Amount     `json:"plannedLimit"`
	RecommendedLimit   Amount     `json:"recommendedLimit"`
	AssignedLimit      Amount     `json:"assignedLimit"`
	OpenExposure       Amount     `json:"openExposure"`
	ChannelHeadroom    Amount     `json:"channelHeadroom"`
	AssignedHeadroom   Amount     `json:"assignedHeadroom"`
	UtilizationBps     int64      `json:"utilizationBps"`
	ChannelCount       int        `json:"channelCount"`
	PendingAdjustments int64      `json:"pendingAdjustments"`
}

func BuildCapacityDigest(p *Protocol) []CapacityDigest {
	rows := make([]CapacityDigest, 0)
	plan, _ := p.CurrentPlan()
	for _, operator := range p.Operators.OperatorRows() {
		channels := p.Operators.ChannelsForOperator(operator.ID)
		available := Amount(0)
		for _, channel := range channels {
			available += channel.Available()
		}
		assigned := p.Operators.AssignedLimit(operator.ID)
		active := p.OperatorExposure(operator.ID)
		headroom := Amount(0)
		if assigned > active {
			headroom = assigned - active
		}
		utilization := int64(0)
		if assigned > 0 {
			utilization = int64(active) * 10_000 / int64(assigned)
		}
		rows = append(rows, CapacityDigest{
			Operator: operator.ID, Epoch: p.Epoch,
			Score: p.Reputation.Score(operator.ID), Band: p.Reputation.Band(operator.ID),
			PlannedLimit: plan.LimitFor(operator.ID), RecommendedLimit: operator.RecommendedLimit,
			AssignedLimit: assigned, OpenExposure: active, ChannelHeadroom: available,
			AssignedHeadroom: headroom, UtilizationBps: utilization,
			ChannelCount: len(channels), PendingAdjustments: p.Reputation.PendingPositive(operator.ID) - p.Reputation.PendingNegative(operator.ID),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Operator < rows[j].Operator })
	return rows
}
