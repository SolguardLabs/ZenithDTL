package zenithdtl

import "sort"

type Operator struct {
	ID                OperatorID        `json:"id"`
	Account           AccountID         `json:"account"`
	Status            OperatorStatus    `json:"status"`
	Stake             Amount            `json:"stake"`
	BaseCapacity      Amount            `json:"baseCapacity"`
	AssignedCapacity  Amount            `json:"assignedCapacity"`
	RecommendedLimit  Amount            `json:"recommendedLimit"`
	MaxChannels       int               `json:"maxChannels"`
	RegisteredEpoch   Epoch             `json:"registeredEpoch"`
	LastCapacityEpoch Epoch             `json:"lastCapacityEpoch"`
	Domains           []DomainID        `json:"domains"`
	Metadata          map[string]string `json:"metadata"`
}

func (o Operator) AcceptsDomain(domain DomainID) bool {
	for _, item := range o.Domains {
		if item == domain {
			return true
		}
	}
	return false
}

func (o Operator) RouteWeight() int64 {
	if o.MaxChannels <= 0 {
		return 50
	}
	weight := int64(80 + o.MaxChannels*5)
	if o.Status == OperatorWatchlist {
		weight -= 20
	}
	if o.Status == OperatorRetiring {
		weight -= 35
	}
	return clampInt64(weight, 35, 145)
}

func (o Operator) Clone() Operator {
	clone := o
	clone.Domains = make([]DomainID, len(o.Domains))
	copy(clone.Domains, o.Domains)
	clone.Metadata = CloneStringMap(o.Metadata)
	return clone
}

type Channel struct {
	ID               ChannelID         `json:"id"`
	Operator         OperatorID        `json:"operator"`
	Origin           DomainID          `json:"origin"`
	Destination      DomainID          `json:"destination"`
	Asset            AssetID           `json:"asset"`
	Mode             ChannelMode       `json:"mode"`
	State            ChannelState      `json:"state"`
	Epoch            Epoch             `json:"epoch"`
	Limit            Amount            `json:"limit"`
	Used             Amount            `json:"used"`
	Reserved         Amount            `json:"reserved"`
	InFlight         Amount            `json:"inFlight"`
	LastHeartbeat    Epoch             `json:"lastHeartbeat"`
	Priority         int               `json:"priority"`
	CapacityRevision int               `json:"capacityRevision"`
	Tags             map[string]string `json:"tags"`
}

func (c Channel) Available() Amount {
	if !stateAcceptsAdmission(c.State) {
		return 0
	}
	occupied, _ := SumAmounts(c.Used, c.Reserved, c.InFlight)
	if c.Limit <= occupied {
		return 0
	}
	return c.Limit - occupied
}

func (c Channel) ActiveDebt() Amount {
	total, _ := SumAmounts(c.Reserved, c.InFlight)
	return total
}

func (c Channel) UtilizationBps() int64 {
	if c.Limit == 0 {
		return 0
	}
	occupied, _ := SumAmounts(c.Used, c.Reserved, c.InFlight)
	return int64(occupied) * 10_000 / int64(c.Limit)
}

func (c Channel) Clone() Channel {
	clone := c
	clone.Tags = CloneStringMap(c.Tags)
	return clone
}

func (c *Channel) Admit(amount Amount) error {
	if err := NonZeroAmount(amount, string(c.ID)); err != nil {
		return err
	}
	if !stateAcceptsAdmission(c.State) {
		return refError(ErrInvalidState, string(c.ID), "channel is not accepting admissions")
	}
	if c.Available() < amount {
		return refError(ErrInsufficientCapacity, string(c.ID), "channel capacity too low")
	}
	next, err := c.InFlight.Add(amount)
	if err != nil {
		return err
	}
	c.InFlight = next
	return nil
}

func (c *Channel) MarkReserved(amount Amount) error {
	if err := NonZeroAmount(amount, string(c.ID)); err != nil {
		return err
	}
	inFlight, err := c.InFlight.Sub(amount)
	if err != nil {
		return refError(ErrInvalidState, string(c.ID), "in-flight amount too low")
	}
	reserved, err := c.Reserved.Add(amount)
	if err != nil {
		return err
	}
	c.InFlight = inFlight
	c.Reserved = reserved
	return nil
}

func (c *Channel) Settle(amount Amount) error {
	if err := NonZeroAmount(amount, string(c.ID)); err != nil {
		return err
	}
	reserved, err := c.Reserved.Sub(amount)
	if err != nil {
		return refError(ErrInvalidState, string(c.ID), "reserved amount too low")
	}
	used, err := c.Used.Add(amount)
	if err != nil {
		return err
	}
	c.Reserved = reserved
	c.Used = used
	return nil
}

func (c *Channel) Release(amount Amount) error {
	if err := NonZeroAmount(amount, string(c.ID)); err != nil {
		return err
	}
	if c.InFlight >= amount {
		next, err := c.InFlight.Sub(amount)
		if err != nil {
			return err
		}
		c.InFlight = next
		return nil
	}
	reserved, err := c.Reserved.Sub(amount)
	if err != nil {
		return refError(ErrInvalidState, string(c.ID), "channel amount too low")
	}
	c.Reserved = reserved
	return nil
}

func (c *Channel) RebaseLimit(limit Amount, epoch Epoch) {
	if limit < c.ActiveDebt() {
		limit = c.ActiveDebt()
	}
	c.Limit = limit
	c.Epoch = epoch
	c.CapacityRevision++
}

type CapacityEntry struct {
	Operator OperatorID `json:"operator"`
	Epoch    Epoch      `json:"epoch"`
	Limit    Amount     `json:"limit"`
	Score    int64      `json:"score"`
	Band     string     `json:"band"`
}

type CapacityPlan struct {
	Epoch   Epoch           `json:"epoch"`
	Entries []CapacityEntry `json:"entries"`
}

func (p CapacityPlan) LimitFor(operator OperatorID) Amount {
	for _, entry := range p.Entries {
		if entry.Operator == operator {
			return entry.Limit
		}
	}
	return 0
}

type CapacityPlanner struct {
	MinLimit            Amount      `json:"minLimit"`
	MaxLimit            Amount      `json:"maxLimit"`
	WatchlistMax        Amount      `json:"watchlistMax"`
	SuspendedMax        Amount      `json:"suspendedMax"`
	RetiringMax         Amount      `json:"retiringMax"`
	ChannelSpreadBps    BasisPoints `json:"channelSpreadBps"`
	EmergencyHaircutBps BasisPoints `json:"emergencyHaircutBps"`
}

func DefaultCapacityPlanner() CapacityPlanner {
	return CapacityPlanner{
		MinLimit:            5_000,
		MaxLimit:            400_000,
		WatchlistMax:        75_000,
		SuspendedMax:        0,
		RetiringMax:         25_000,
		ChannelSpreadBps:    8_000,
		EmergencyHaircutBps: 2_500,
	}
}

func (p CapacityPlanner) Plan(epoch Epoch, registry *OperatorRegistry, reputation *ReputationBook) CapacityPlan {
	entries := []CapacityEntry{}
	for _, operator := range registry.OperatorRows() {
		score := reputation.Score(operator.ID)
		limit := reputation.RecommendedCapacity(operator.ID, operator.BaseCapacity, operator.Stake, operator.RouteWeight())
		if limit < p.MinLimit && statusAcceptsAdmission(operator.Status) {
			limit = p.MinLimit
		}
		if limit > p.MaxLimit {
			limit = p.MaxLimit
		}
		switch operator.Status {
		case OperatorWatchlist:
			limit = minAmount(limit, p.WatchlistMax)
		case OperatorRetiring:
			limit = minAmount(limit, p.RetiringMax)
		case OperatorSuspended, OperatorQuarantined:
			limit = minAmount(limit, p.SuspendedMax)
		}
		entries = append(entries, CapacityEntry{
			Operator: operator.ID,
			Epoch:    epoch,
			Limit:    limit,
			Score:    score,
			Band:     reputation.Band(operator.ID),
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Operator < entries[j].Operator })
	return CapacityPlan{Epoch: epoch, Entries: entries}
}

type OperatorRegistry struct {
	operators map[OperatorID]*Operator
	channels  map[ChannelID]*Channel
	events    *EventLog
}

func NewOperatorRegistry(events *EventLog) *OperatorRegistry {
	return &OperatorRegistry{
		operators: map[OperatorID]*Operator{},
		channels:  map[ChannelID]*Channel{},
		events:    events,
	}
}

func (r *OperatorRegistry) RegisterOperator(operator Operator) error {
	if operator.ID == "" || operator.Account == "" {
		return newError(ErrInvalidState, "operator requires id and account")
	}
	if _, ok := r.operators[operator.ID]; ok {
		return refError(ErrDuplicate, string(operator.ID), "operator already registered")
	}
	if operator.Status == "" {
		operator.Status = OperatorActive
	}
	if operator.MaxChannels == 0 {
		operator.MaxChannels = 4
	}
	if operator.Metadata == nil {
		operator.Metadata = map[string]string{}
	}
	operator.Domains = normalizeDomains(operator.Domains)
	r.operators[operator.ID] = &operator
	if r.events != nil {
		r.events.Add(operator.RegisteredEpoch, EventOperatorRegistered, string(operator.ID), eventAttrs("account", string(operator.Account), "status", string(operator.Status)))
	}
	return nil
}

func (r *OperatorRegistry) Operator(id OperatorID) (*Operator, error) {
	operator, ok := r.operators[id]
	if !ok {
		return nil, refError(ErrUnknownOperator, string(id), "operator not found")
	}
	return operator, nil
}

func (r *OperatorRegistry) UpdateStatus(epoch Epoch, id OperatorID, status OperatorStatus) error {
	operator, err := r.Operator(id)
	if err != nil {
		return err
	}
	operator.Status = status
	if r.events != nil {
		r.events.Add(epoch, EventOperatorUpdated, string(id), eventAttrs("status", string(status)))
	}
	return nil
}

func (r *OperatorRegistry) SetRecommendedLimit(epoch Epoch, id OperatorID, limit Amount) error {
	operator, err := r.Operator(id)
	if err != nil {
		return err
	}
	operator.RecommendedLimit = limit
	operator.LastCapacityEpoch = epoch
	return nil
}

func (r *OperatorRegistry) OpenChannel(channel Channel) error {
	if channel.ID == "" {
		return newError(ErrInvalidState, "channel requires id")
	}
	if _, ok := r.channels[channel.ID]; ok {
		return refError(ErrDuplicate, string(channel.ID), "channel already exists")
	}
	operator, err := r.Operator(channel.Operator)
	if err != nil {
		return err
	}
	if !operator.AcceptsDomain(channel.Origin) || !operator.AcceptsDomain(channel.Destination) {
		return refError(ErrPolicy, string(channel.ID), "operator domain set does not cover channel")
	}
	if !validDomainPath(channel.Origin, channel.Destination) {
		return refError(ErrInvalidRoute, string(channel.ID), "channel domain path is invalid")
	}
	if channel.Asset == "" {
		channel.Asset = DefaultAsset
	}
	if channel.State == "" {
		channel.State = ChannelOpen
	}
	if channel.Mode == "" {
		channel.Mode = ChannelWarm
	}
	if channel.Tags == nil {
		channel.Tags = map[string]string{}
	}
	r.channels[channel.ID] = &channel
	if r.events != nil {
		r.events.Add(channel.Epoch, EventChannelOpened, string(channel.ID), eventAttrs("operator", string(channel.Operator), "limit", amountString(channel.Limit)))
	}
	return nil
}

func (r *OperatorRegistry) Channel(id ChannelID) (*Channel, error) {
	channel, ok := r.channels[id]
	if !ok {
		return nil, refError(ErrUnknownChannel, string(id), "channel not found")
	}
	return channel, nil
}

func (r *OperatorRegistry) ChannelsForOperator(id OperatorID) []Channel {
	rows := []Channel{}
	for _, channel := range r.channels {
		if channel.Operator == id {
			rows = append(rows, channel.Clone())
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows
}

func (r *OperatorRegistry) ActiveDebt(operator OperatorID) Amount {
	var total Amount
	for _, channel := range r.channels {
		if channel.Operator == operator {
			total += channel.ActiveDebt()
		}
	}
	return total
}

func (r *OperatorRegistry) AssignedLimit(operator OperatorID) Amount {
	var total Amount
	for _, channel := range r.channels {
		if channel.Operator == operator && stateAcceptsAdmission(channel.State) {
			total += channel.Limit
		}
	}
	return total
}

func (r *OperatorRegistry) Admit(channelID ChannelID, amount Amount) error {
	channel, err := r.Channel(channelID)
	if err != nil {
		return err
	}
	operator, err := r.Operator(channel.Operator)
	if err != nil {
		return err
	}
	if !statusAcceptsAdmission(operator.Status) {
		return refError(ErrPolicy, string(operator.ID), "operator is not accepting settlement")
	}
	return channel.Admit(amount)
}

func (r *OperatorRegistry) MarkReserved(channelID ChannelID, amount Amount) error {
	channel, err := r.Channel(channelID)
	if err != nil {
		return err
	}
	return channel.MarkReserved(amount)
}

func (r *OperatorRegistry) Settle(channelID ChannelID, amount Amount) error {
	channel, err := r.Channel(channelID)
	if err != nil {
		return err
	}
	return channel.Settle(amount)
}

func (r *OperatorRegistry) Release(channelID ChannelID, amount Amount) error {
	channel, err := r.Channel(channelID)
	if err != nil {
		return err
	}
	return channel.Release(amount)
}

func (r *OperatorRegistry) ApplyCapacityPlan(plan CapacityPlan) {
	for _, entry := range plan.Entries {
		operator, ok := r.operators[entry.Operator]
		if !ok {
			continue
		}
		operator.AssignedCapacity = entry.Limit
		operator.RecommendedLimit = entry.Limit
		operator.LastCapacityEpoch = plan.Epoch
		channels := r.channelPointers(entry.Operator)
		if len(channels) == 0 {
			continue
		}
		weights := make([]int64, len(channels))
		for i, channel := range channels {
			weights[i] = modeWeight(channel.Mode) + int64(channel.Priority*5)
			if channel.State == ChannelDraining {
				weights[i] /= 2
			}
			if weights[i] < 10 {
				weights[i] = 10
			}
		}
		parts, err := SplitAmount(entry.Limit, weights)
		if err != nil {
			continue
		}
		for i, channel := range channels {
			channel.RebaseLimit(parts[i], plan.Epoch)
			if r.events != nil {
				r.events.Add(plan.Epoch, EventCapacityAssigned, string(channel.ID), eventAttrs("operator", string(channel.Operator), "limit", amountString(parts[i])))
			}
		}
	}
}

func (r *OperatorRegistry) ApplyCapacityToIdleChannels(epoch Epoch, reputation *ReputationBook) {
	for _, operator := range r.OperatorRows() {
		cardScore := reputation.Score(operator.ID)
		limit := reputation.RecommendedCapacity(operator.ID, operator.BaseCapacity, operator.Stake, operator.RouteWeight())
		if cardScore < 450 {
			limit = minAmount(limit, 20_000)
		}
		channels := r.channelPointers(operator.ID)
		if len(channels) == 0 {
			continue
		}
		idle := []*Channel{}
		for _, channel := range channels {
			if channel.ActiveDebt() == 0 && channel.InFlight == 0 {
				idle = append(idle, channel)
			}
		}
		if len(idle) == 0 {
			continue
		}
		parts, err := SplitAmount(limit, repeatWeight(len(idle), 100))
		if err != nil {
			continue
		}
		for i, channel := range idle {
			channel.RebaseLimit(parts[i], epoch)
			if r.events != nil {
				r.events.Add(epoch, EventCapacityAssigned, string(channel.ID), eventAttrs("operator", string(channel.Operator), "limit", amountString(parts[i]), "mode", "idle-refresh"))
			}
		}
	}
}

func (r *OperatorRegistry) Heartbeat(epoch Epoch, channelID ChannelID) error {
	channel, err := r.Channel(channelID)
	if err != nil {
		return err
	}
	channel.LastHeartbeat = epoch
	if channel.State == ChannelPending {
		channel.State = ChannelOpen
	}
	if r.events != nil {
		r.events.Add(epoch, EventChannelUpdated, string(channel.ID), eventAttrs("heartbeat", epochString(epoch)))
	}
	return nil
}

func (r *OperatorRegistry) OperatorRows() []Operator {
	rows := make([]Operator, 0, len(r.operators))
	for _, operator := range r.operators {
		clone := operator.Clone()
		clone.AssignedCapacity = r.AssignedLimit(operator.ID)
		rows = append(rows, clone)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows
}

func (r *OperatorRegistry) ChannelRows() []Channel {
	rows := make([]Channel, 0, len(r.channels))
	for _, channel := range r.channels {
		rows = append(rows, channel.Clone())
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows
}

func (r *OperatorRegistry) channelPointers(operator OperatorID) []*Channel {
	rows := []*Channel{}
	for _, channel := range r.channels {
		if channel.Operator == operator {
			rows = append(rows, channel)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows
}

func normalizeDomains(values []DomainID) []DomainID {
	seen := map[DomainID]bool{}
	result := []DomainID{}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func repeatWeight(count int, value int64) []int64 {
	result := make([]int64, count)
	for i := range result {
		result[i] = value
	}
	return result
}
