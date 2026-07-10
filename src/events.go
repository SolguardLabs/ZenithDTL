package zenithdtl

import "sort"

const (
	EventAccountOpened      EventKind = "account.opened"
	EventVaultOpened        EventKind = "vault.opened"
	EventDeposit            EventKind = "ledger.deposit"
	EventReserve            EventKind = "ledger.reserve"
	EventRelease            EventKind = "ledger.release"
	EventDebit              EventKind = "ledger.debit"
	EventCredit             EventKind = "ledger.credit"
	EventOperatorRegistered EventKind = "operator.registered"
	EventOperatorUpdated    EventKind = "operator.updated"
	EventChannelOpened      EventKind = "channel.opened"
	EventChannelUpdated     EventKind = "channel.updated"
	EventRouteRegistered    EventKind = "route.registered"
	EventRouteValidated     EventKind = "route.validated"
	EventObligationOpened   EventKind = "obligation.opened"
	EventObligationSettled  EventKind = "obligation.settled"
	EventObligationClosed   EventKind = "obligation.closed"
	EventWithdrawalQueued   EventKind = "withdrawal.queued"
	EventWithdrawalReady    EventKind = "withdrawal.ready"
	EventWithdrawalPaid     EventKind = "withdrawal.paid"
	EventReputationUpdated  EventKind = "reputation.updated"
	EventEpochClosed        EventKind = "epoch.closed"
	EventEpochOpened        EventKind = "epoch.opened"
	EventCapacityPlanned    EventKind = "capacity.planned"
	EventCapacityAssigned   EventKind = "capacity.assigned"
)

type Event struct {
	Index      int               `json:"index"`
	Epoch      Epoch             `json:"epoch"`
	Kind       EventKind         `json:"kind"`
	Entity     string            `json:"entity"`
	Severity   Severity          `json:"severity"`
	Attributes map[string]string `json:"attributes"`
}

type EventLog struct {
	next   int
	events []Event
}

func NewEventLog() *EventLog {
	return &EventLog{next: 1, events: []Event{}}
}

func (l *EventLog) Add(epoch Epoch, kind EventKind, entity string, attrs map[string]string) Event {
	if l == nil {
		return Event{}
	}
	event := Event{
		Index:      l.next,
		Epoch:      epoch,
		Kind:       kind,
		Entity:     entity,
		Severity:   SeverityInfo,
		Attributes: StableAttributes(attrs),
	}
	l.next++
	l.events = append(l.events, event)
	return event
}

func (l *EventLog) Notice(epoch Epoch, kind EventKind, entity string, attrs map[string]string) Event {
	event := l.Add(epoch, kind, entity, attrs)
	if len(l.events) > 0 {
		l.events[len(l.events)-1].Severity = SeverityNotice
		event.Severity = SeverityNotice
	}
	return event
}

func (l *EventLog) Warning(epoch Epoch, kind EventKind, entity string, attrs map[string]string) Event {
	event := l.Add(epoch, kind, entity, attrs)
	if len(l.events) > 0 {
		l.events[len(l.events)-1].Severity = SeverityWarning
		event.Severity = SeverityWarning
	}
	return event
}

func (l *EventLog) Critical(epoch Epoch, kind EventKind, entity string, attrs map[string]string) Event {
	event := l.Add(epoch, kind, entity, attrs)
	if len(l.events) > 0 {
		l.events[len(l.events)-1].Severity = SeverityCritical
		event.Severity = SeverityCritical
	}
	return event
}

func (l *EventLog) All() []Event {
	if l == nil {
		return nil
	}
	result := make([]Event, len(l.events))
	copy(result, l.events)
	return result
}

func (l *EventLog) FilterByKind(kind EventKind) []Event {
	if l == nil {
		return nil
	}
	result := []Event{}
	for _, event := range l.events {
		if event.Kind == kind {
			result = append(result, event)
		}
	}
	return result
}

func (l *EventLog) FilterByEpoch(epoch Epoch) []Event {
	if l == nil {
		return nil
	}
	result := []Event{}
	for _, event := range l.events {
		if event.Epoch == epoch {
			result = append(result, event)
		}
	}
	return result
}

func (l *EventLog) Count(kind EventKind) int {
	if l == nil {
		return 0
	}
	count := 0
	for _, event := range l.events {
		if event.Kind == kind {
			count++
		}
	}
	return count
}

func (l *EventLog) Last() (Event, bool) {
	if l == nil || len(l.events) == 0 {
		return Event{}, false
	}
	return l.events[len(l.events)-1], true
}

func (l *EventLog) Compact(limit int) []Event {
	if l == nil {
		return nil
	}
	if limit <= 0 || len(l.events) <= limit {
		return l.All()
	}
	result := make([]Event, limit)
	copy(result, l.events[len(l.events)-limit:])
	return result
}

func (l *EventLog) Summary() map[string]int {
	result := map[string]int{}
	if l == nil {
		return result
	}
	for _, event := range l.events {
		result[string(event.Kind)]++
	}
	return result
}

func sortedEventKinds(summary map[string]int) []string {
	keys := make([]string, 0, len(summary))
	for key := range summary {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func eventAttrs(values ...string) map[string]string {
	attrs := map[string]string{}
	for i := 0; i+1 < len(values); i += 2 {
		attrs[values[i]] = values[i+1]
	}
	return attrs
}
