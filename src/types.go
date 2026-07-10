package zenithdtl

import (
	"fmt"
	"strings"
)

type Epoch uint64
type Amount int64
type BasisPoints int64

type AccountID string
type AssetID string
type OperatorID string
type ChannelID string
type VaultID string
type RouteID string
type ObligationID string
type WithdrawalID string
type DomainID string
type EventKind string
type LedgerSide string

const (
	ProtocolName         = "ZenithDTL"
	DefaultAsset AssetID = "ZUSD"

	SideDebit  LedgerSide = "debit"
	SideCredit LedgerSide = "credit"
)

type OperatorStatus string

const (
	OperatorActive      OperatorStatus = "active"
	OperatorWatchlist   OperatorStatus = "watchlist"
	OperatorSuspended   OperatorStatus = "suspended"
	OperatorRetiring    OperatorStatus = "retiring"
	OperatorQuarantined OperatorStatus = "quarantined"
)

type ChannelMode string

const (
	ChannelHot      ChannelMode = "hot"
	ChannelWarm     ChannelMode = "warm"
	ChannelCold     ChannelMode = "cold"
	ChannelBridge   ChannelMode = "bridge"
	ChannelInternal ChannelMode = "internal"
)

type ChannelState string

const (
	ChannelOpen       ChannelState = "open"
	ChannelDraining   ChannelState = "draining"
	ChannelClosed     ChannelState = "closed"
	ChannelPaused     ChannelState = "paused"
	ChannelPending    ChannelState = "pending"
	ChannelDeprecated ChannelState = "deprecated"
)

type ObligationStatus string

const (
	ObligationOpen      ObligationStatus = "open"
	ObligationSettled   ObligationStatus = "settled"
	ObligationCancelled ObligationStatus = "cancelled"
	ObligationExpired   ObligationStatus = "expired"
)

type WithdrawalStatus string

const (
	WithdrawalQueued    WithdrawalStatus = "queued"
	WithdrawalReady     WithdrawalStatus = "ready"
	WithdrawalCompleted WithdrawalStatus = "completed"
	WithdrawalRejected  WithdrawalStatus = "rejected"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityNotice   Severity = "notice"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

type ErrorCode string

const (
	ErrInvalidAmount        ErrorCode = "invalid_amount"
	ErrInsufficientBalance  ErrorCode = "insufficient_balance"
	ErrInsufficientCapacity ErrorCode = "insufficient_capacity"
	ErrUnknownOperator      ErrorCode = "unknown_operator"
	ErrUnknownChannel       ErrorCode = "unknown_channel"
	ErrUnknownRoute         ErrorCode = "unknown_route"
	ErrUnknownVault         ErrorCode = "unknown_vault"
	ErrUnknownAccount       ErrorCode = "unknown_account"
	ErrInvalidRoute         ErrorCode = "invalid_route"
	ErrInvalidState         ErrorCode = "invalid_state"
	ErrPolicy               ErrorCode = "policy"
	ErrDuplicate            ErrorCode = "duplicate"
	ErrNotFound             ErrorCode = "not_found"
	ErrEpoch                ErrorCode = "epoch"
)

type ZenithError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Ref     string    `json:"ref,omitempty"`
}

func (e ZenithError) Error() string {
	if e.Ref == "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Ref)
}

func newError(code ErrorCode, message string) error {
	return ZenithError{Code: code, Message: message}
}

func refError(code ErrorCode, ref string, message string) error {
	return ZenithError{Code: code, Message: message, Ref: ref}
}

func wrapRef(code ErrorCode, ref string, format string, args ...any) error {
	return ZenithError{Code: code, Message: fmt.Sprintf(format, args...), Ref: ref}
}

func isBlank(value string) bool {
	return strings.TrimSpace(value) == ""
}

func canonical(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func idPart(value string) string {
	trimmed := canonical(value)
	trimmed = strings.ReplaceAll(trimmed, " ", "-")
	trimmed = strings.ReplaceAll(trimmed, "_", "-")
	return trimmed
}

func NewAccountID(value string) AccountID {
	return AccountID(idPart(value))
}

func NewOperatorID(value string) OperatorID {
	return OperatorID(idPart(value))
}

func NewChannelID(operator OperatorID, origin DomainID, destination DomainID, suffix string) ChannelID {
	return ChannelID(fmt.Sprintf("ch-%s-%s-%s-%s", operator, origin, destination, idPart(suffix)))
}

func NewRouteID(origin DomainID, destination DomainID, suffix string) RouteID {
	return RouteID(fmt.Sprintf("rt-%s-%s-%s", origin, destination, idPart(suffix)))
}

func NewVaultID(domain DomainID, asset AssetID, suffix string) VaultID {
	return VaultID(fmt.Sprintf("vault-%s-%s-%s", domain, asset, idPart(suffix)))
}

func NewObligationID(epoch Epoch, route RouteID, nonce int) ObligationID {
	return ObligationID(fmt.Sprintf("ob-%06d-%s-%04d", epoch, route, nonce))
}

func NewWithdrawalID(epoch Epoch, account AccountID, nonce int) WithdrawalID {
	return WithdrawalID(fmt.Sprintf("wd-%06d-%s-%04d", epoch, account, nonce))
}

func validDomainPath(origin DomainID, destination DomainID) bool {
	return origin != "" && destination != "" && origin != destination
}

func sameAsset(left AssetID, right AssetID) bool {
	return left != "" && left == right
}

func statusAcceptsAdmission(status OperatorStatus) bool {
	return status == OperatorActive || status == OperatorWatchlist
}

func stateAcceptsAdmission(state ChannelState) bool {
	return state == ChannelOpen || state == ChannelDraining
}

func modeWeight(mode ChannelMode) int64 {
	switch mode {
	case ChannelHot:
		return 120
	case ChannelWarm:
		return 100
	case ChannelBridge:
		return 90
	case ChannelInternal:
		return 80
	case ChannelCold:
		return 65
	default:
		return 50
	}
}

func statusWeight(status OperatorStatus) int64 {
	switch status {
	case OperatorActive:
		return 100
	case OperatorWatchlist:
		return 75
	case OperatorRetiring:
		return 40
	case OperatorSuspended:
		return 0
	case OperatorQuarantined:
		return 0
	default:
		return 50
	}
}

func clampInt64(value int64, min int64, max int64) int64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func clampScore(value int64) int64 {
	return clampInt64(value, 0, 1_000)
}

func minAmount(left Amount, right Amount) Amount {
	if left < right {
		return left
	}
	return right
}

func maxAmount(left Amount, right Amount) Amount {
	if left > right {
		return left
	}
	return right
}

func boolLabel(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func epochString(epoch Epoch) string {
	return fmt.Sprintf("%d", epoch)
}

func amountString(amount Amount) string {
	return fmt.Sprintf("%d", amount)
}
