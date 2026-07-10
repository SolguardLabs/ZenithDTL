package zenithdtl

import "sort"

type WithdrawalRequest struct {
	ID        WithdrawalID     `json:"id"`
	Epoch     Epoch            `json:"epoch"`
	Account   AccountID        `json:"account"`
	Vault     VaultID          `json:"vault"`
	Asset     AssetID          `json:"asset"`
	Amount    Amount           `json:"amount"`
	ReadyAt   Epoch            `json:"readyAt"`
	Status    WithdrawalStatus `json:"status"`
	Reference string           `json:"reference"`
}

func (w WithdrawalRequest) Clone() WithdrawalRequest {
	return w
}

type WithdrawalBook struct {
	requests map[WithdrawalID]*WithdrawalRequest
	nonce    int
	events   *EventLog
}

func NewWithdrawalBook(events *EventLog) *WithdrawalBook {
	return &WithdrawalBook{
		requests: map[WithdrawalID]*WithdrawalRequest{},
		nonce:    1,
		events:   events,
	}
}

func (b *WithdrawalBook) Queue(epoch Epoch, account AccountID, vaultID VaultID, amount Amount, ledger *Ledger, holdEpochs Epoch, ref string) (*WithdrawalRequest, error) {
	if holdEpochs == 0 {
		holdEpochs = 1
	}
	vault, err := ledger.Vault(vaultID)
	if err != nil {
		return nil, err
	}
	if vault.Owner != account {
		return nil, refError(ErrPolicy, string(vaultID), "account does not own vault")
	}
	if err := ledger.QueueVaultWithdrawal(epoch, vaultID, amount, ref); err != nil {
		return nil, err
	}
	id := NewWithdrawalID(epoch, account, b.nonce)
	b.nonce++
	request := &WithdrawalRequest{
		ID:        id,
		Epoch:     epoch,
		Account:   account,
		Vault:     vaultID,
		Asset:     vault.Asset,
		Amount:    amount,
		ReadyAt:   epoch + holdEpochs,
		Status:    WithdrawalQueued,
		Reference: ref,
	}
	b.requests[id] = request
	if b.events != nil {
		b.events.Add(epoch, EventWithdrawalQueued, string(id), eventAttrs("vault", string(vaultID), "amount", amountString(amount), "readyAt", epochString(request.ReadyAt)))
	}
	return request, nil
}

func (b *WithdrawalBook) Request(id WithdrawalID) (*WithdrawalRequest, error) {
	request, ok := b.requests[id]
	if !ok {
		return nil, refError(ErrNotFound, string(id), "withdrawal not found")
	}
	return request, nil
}

func (b *WithdrawalBook) MarkReady(epoch Epoch) {
	for _, request := range b.requests {
		if request.Status == WithdrawalQueued && request.ReadyAt <= epoch {
			request.Status = WithdrawalReady
			if b.events != nil {
				b.events.Add(epoch, EventWithdrawalReady, string(request.ID), eventAttrs("amount", amountString(request.Amount)))
			}
		}
	}
}

func (b *WithdrawalBook) Pay(epoch Epoch, id WithdrawalID, ledger *Ledger) error {
	request, err := b.Request(id)
	if err != nil {
		return err
	}
	if request.Status == WithdrawalQueued && request.ReadyAt <= epoch {
		request.Status = WithdrawalReady
	}
	if request.Status != WithdrawalReady {
		return refError(ErrInvalidState, string(id), "withdrawal is not ready")
	}
	if err := ledger.PayVaultWithdrawal(epoch, request.Vault, request.Account, request.Amount, string(request.ID)); err != nil {
		return err
	}
	request.Status = WithdrawalCompleted
	if b.events != nil {
		b.events.Add(epoch, EventWithdrawalPaid, string(id), eventAttrs("vault", string(request.Vault), "amount", amountString(request.Amount)))
	}
	return nil
}

func (b *WithdrawalBook) Reject(epoch Epoch, id WithdrawalID, ledger *Ledger, reason string) error {
	request, err := b.Request(id)
	if err != nil {
		return err
	}
	if request.Status == WithdrawalCompleted {
		return refError(ErrInvalidState, string(id), "withdrawal already completed")
	}
	vault, err := ledger.Vault(request.Vault)
	if err != nil {
		return err
	}
	next, err := vault.PendingWithdraw.Sub(request.Amount)
	if err != nil {
		return err
	}
	vault.PendingWithdraw = next
	request.Status = WithdrawalRejected
	if b.events != nil {
		b.events.Warning(epoch, EventWithdrawalReady, string(id), eventAttrs("reason", reason))
	}
	return nil
}

func (b *WithdrawalBook) PayReady(epoch Epoch, ledger *Ledger) error {
	b.MarkReady(epoch)
	for _, request := range b.Rows() {
		if request.Status == WithdrawalReady {
			if err := b.Pay(epoch, request.ID, ledger); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *WithdrawalBook) Rows() []WithdrawalRequest {
	rows := make([]WithdrawalRequest, 0, len(b.requests))
	for _, request := range b.requests {
		rows = append(rows, request.Clone())
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows
}

func (b *WithdrawalBook) PendingAmount(asset AssetID) Amount {
	var total Amount
	for _, request := range b.requests {
		if request.Asset == asset && (request.Status == WithdrawalQueued || request.Status == WithdrawalReady) {
			total += request.Amount
		}
	}
	return total
}

func (b *WithdrawalBook) CompletedAmount(asset AssetID) Amount {
	var total Amount
	for _, request := range b.requests {
		if request.Asset == asset && request.Status == WithdrawalCompleted {
			total += request.Amount
		}
	}
	return total
}
