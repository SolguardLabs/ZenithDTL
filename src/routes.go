package zenithdtl

import "sort"

type RoutePolicy struct {
	Asset             AssetID     `json:"asset"`
	Origin            DomainID    `json:"origin"`
	Destination       DomainID    `json:"destination"`
	MaxFeeBps         BasisPoints `json:"maxFeeBps"`
	MinOperatorScore  int64       `json:"minOperatorScore"`
	MaxHops           int         `json:"maxHops"`
	RequireDistinct   bool        `json:"requireDistinctOperators"`
	SettlementWindow  int         `json:"settlementWindow"`
	WithdrawalAllowed bool        `json:"withdrawalAllowed"`
}

func DefaultRoutePolicy(asset AssetID, origin DomainID, destination DomainID) RoutePolicy {
	return RoutePolicy{
		Asset:             asset,
		Origin:            origin,
		Destination:       destination,
		MaxFeeBps:         80,
		MinOperatorScore:  500,
		MaxHops:           3,
		RequireDistinct:   true,
		SettlementWindow:  2,
		WithdrawalAllowed: true,
	}
}

type RouteLeg struct {
	Index       int         `json:"index"`
	Channel     ChannelID   `json:"channel"`
	Operator    OperatorID  `json:"operator"`
	Origin      DomainID    `json:"origin"`
	Destination DomainID    `json:"destination"`
	FeeBps      BasisPoints `json:"feeBps"`
	MinCapacity Amount      `json:"minCapacity"`
}

type Route struct {
	ID        RouteID           `json:"id"`
	Name      string            `json:"name"`
	Policy    RoutePolicy       `json:"policy"`
	Legs      []RouteLeg        `json:"legs"`
	Enabled   bool              `json:"enabled"`
	CreatedAt Epoch             `json:"createdAt"`
	Tags      map[string]string `json:"tags"`
}

func (r Route) Clone() Route {
	clone := r
	clone.Legs = make([]RouteLeg, len(r.Legs))
	copy(clone.Legs, r.Legs)
	clone.Tags = CloneStringMap(r.Tags)
	return clone
}

func (r Route) TotalFeeBps() BasisPoints {
	var total BasisPoints
	for _, leg := range r.Legs {
		total += leg.FeeBps
	}
	return total
}

func (r Route) Path() []DomainID {
	if len(r.Legs) == 0 {
		return []DomainID{r.Policy.Origin, r.Policy.Destination}
	}
	path := []DomainID{r.Legs[0].Origin}
	for _, leg := range r.Legs {
		path = append(path, leg.Destination)
	}
	return path
}

func (r Route) HopCount() int {
	return len(r.Legs)
}

func (r Route) FeeFor(amount Amount) (Amount, error) {
	return amount.MulBpsCeil(r.TotalFeeBps())
}

type RouteRegistry struct {
	routes map[RouteID]*Route
	events *EventLog
}

func NewRouteRegistry(events *EventLog) *RouteRegistry {
	return &RouteRegistry{
		routes: map[RouteID]*Route{},
		events: events,
	}
}

func (r *RouteRegistry) Register(route Route) error {
	if route.ID == "" {
		return newError(ErrInvalidRoute, "route requires id")
	}
	if _, ok := r.routes[route.ID]; ok {
		return refError(ErrDuplicate, string(route.ID), "route already registered")
	}
	if route.Policy.Asset == "" {
		route.Policy.Asset = DefaultAsset
	}
	if route.Policy.MaxHops == 0 {
		route.Policy.MaxHops = 3
	}
	if route.Policy.MaxFeeBps == 0 {
		route.Policy.MaxFeeBps = 80
	}
	if route.Tags == nil {
		route.Tags = map[string]string{}
	}
	route.Enabled = true
	for i := range route.Legs {
		route.Legs[i].Index = i
	}
	r.routes[route.ID] = &route
	if r.events != nil {
		r.events.Add(route.CreatedAt, EventRouteRegistered, string(route.ID), eventAttrs("origin", string(route.Policy.Origin), "destination", string(route.Policy.Destination)))
	}
	return nil
}

func (r *RouteRegistry) Route(id RouteID) (*Route, error) {
	route, ok := r.routes[id]
	if !ok {
		return nil, refError(ErrUnknownRoute, string(id), "route not found")
	}
	return route, nil
}

func (r *RouteRegistry) Routes() []Route {
	rows := make([]Route, 0, len(r.routes))
	for _, route := range r.routes {
		rows = append(rows, route.Clone())
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows
}

func (r *RouteRegistry) Validate(id RouteID, registry *OperatorRegistry, reputation *ReputationBook, amount Amount) error {
	route, err := r.Route(id)
	if err != nil {
		return err
	}
	if !route.Enabled {
		return refError(ErrInvalidRoute, string(id), "route is disabled")
	}
	if route.Policy.Asset == "" || route.Policy.Origin == "" || route.Policy.Destination == "" {
		return refError(ErrInvalidRoute, string(id), "route policy is incomplete")
	}
	if len(route.Legs) == 0 {
		return refError(ErrInvalidRoute, string(id), "route has no legs")
	}
	if route.Policy.MaxHops > 0 && len(route.Legs) > route.Policy.MaxHops {
		return refError(ErrInvalidRoute, string(id), "route exceeds hop limit")
	}
	if route.TotalFeeBps() > route.Policy.MaxFeeBps {
		return refError(ErrPolicy, string(id), "route fee exceeds policy")
	}
	seenOperators := map[OperatorID]bool{}
	expectedOrigin := route.Policy.Origin
	for _, leg := range route.Legs {
		if leg.Origin != expectedOrigin {
			return refError(ErrInvalidRoute, string(id), "route leg origin does not connect")
		}
		if !validDomainPath(leg.Origin, leg.Destination) {
			return refError(ErrInvalidRoute, string(id), "route leg path is invalid")
		}
		channel, err := registry.Channel(leg.Channel)
		if err != nil {
			return err
		}
		if channel.Operator != leg.Operator || channel.Origin != leg.Origin || channel.Destination != leg.Destination {
			return refError(ErrInvalidRoute, string(leg.Channel), "route leg does not match channel")
		}
		if !sameAsset(channel.Asset, route.Policy.Asset) {
			return refError(ErrInvalidRoute, string(leg.Channel), "route asset does not match channel")
		}
		operator, err := registry.Operator(leg.Operator)
		if err != nil {
			return err
		}
		if !statusAcceptsAdmission(operator.Status) {
			return refError(ErrPolicy, string(operator.ID), "operator status does not accept route")
		}
		if reputation.Score(leg.Operator) < route.Policy.MinOperatorScore {
			return refError(ErrPolicy, string(operator.ID), "operator score below route threshold")
		}
		if route.Policy.RequireDistinct && seenOperators[leg.Operator] {
			return refError(ErrPolicy, string(id), "route requires distinct operators")
		}
		seenOperators[leg.Operator] = true
		needed, err := routeCapacityNeed(amount, leg.FeeBps)
		if err != nil {
			return err
		}
		if leg.MinCapacity > 0 {
			needed = maxAmount(needed, leg.MinCapacity)
		}
		if channel.Available() < needed {
			return refError(ErrInsufficientCapacity, string(channel.ID), "route channel capacity too low")
		}
		expectedOrigin = leg.Destination
	}
	if expectedOrigin != route.Policy.Destination {
		return refError(ErrInvalidRoute, string(id), "route final destination mismatch")
	}
	if r.events != nil {
		r.events.Add(0, EventRouteValidated, string(id), eventAttrs("amount", amountString(amount), "hops", amountString(Amount(len(route.Legs)))))
	}
	return nil
}

func routeCapacityNeed(amount Amount, feeBps BasisPoints) (Amount, error) {
	fee, err := amount.MulBpsCeil(feeBps)
	if err != nil {
		return 0, err
	}
	return amount.Add(fee)
}

func (r *RouteRegistry) BestRoute(origin DomainID, destination DomainID, asset AssetID, registry *OperatorRegistry, reputation *ReputationBook, amount Amount) (RouteID, error) {
	candidates := []Route{}
	for _, route := range r.Routes() {
		if route.Policy.Origin == origin && route.Policy.Destination == destination && route.Policy.Asset == asset && route.Enabled {
			if err := r.Validate(route.ID, registry, reputation, amount); err == nil {
				candidates = append(candidates, route)
			}
		}
	}
	if len(candidates) == 0 {
		return "", refError(ErrUnknownRoute, string(origin)+"->"+string(destination), "no route available")
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].TotalFeeBps() == candidates[j].TotalFeeBps() {
			return candidates[i].HopCount() < candidates[j].HopCount()
		}
		return candidates[i].TotalFeeBps() < candidates[j].TotalFeeBps()
	})
	return candidates[0].ID, nil
}

func BuildSingleHopRoute(id RouteID, name string, channel Channel, maxFee BasisPoints, minScore int64, epoch Epoch) Route {
	policy := DefaultRoutePolicy(channel.Asset, channel.Origin, channel.Destination)
	policy.MaxFeeBps = maxFee
	policy.MinOperatorScore = minScore
	return Route{
		ID:        id,
		Name:      name,
		Policy:    policy,
		CreatedAt: epoch,
		Enabled:   true,
		Legs: []RouteLeg{
			{
				Index:       0,
				Channel:     channel.ID,
				Operator:    channel.Operator,
				Origin:      channel.Origin,
				Destination: channel.Destination,
				FeeBps:      minAmountBps(maxFee, 35),
				MinCapacity: 0,
			},
		},
		Tags: map[string]string{"class": "single-hop"},
	}
}

func BuildTwoHopRoute(id RouteID, name string, first Channel, second Channel, maxFee BasisPoints, minScore int64, epoch Epoch) Route {
	policy := DefaultRoutePolicy(first.Asset, first.Origin, second.Destination)
	policy.MaxFeeBps = maxFee
	policy.MinOperatorScore = minScore
	policy.MaxHops = 4
	return Route{
		ID:        id,
		Name:      name,
		Policy:    policy,
		CreatedAt: epoch,
		Enabled:   true,
		Legs: []RouteLeg{
			{
				Index:       0,
				Channel:     first.ID,
				Operator:    first.Operator,
				Origin:      first.Origin,
				Destination: first.Destination,
				FeeBps:      minAmountBps(maxFee/2, 30),
			},
			{
				Index:       1,
				Channel:     second.ID,
				Operator:    second.Operator,
				Origin:      second.Origin,
				Destination: second.Destination,
				FeeBps:      minAmountBps(maxFee/2, 30),
			},
		},
		Tags: map[string]string{"class": "two-hop"},
	}
}

func minAmountBps(left BasisPoints, right BasisPoints) BasisPoints {
	if left < right {
		return left
	}
	return right
}
