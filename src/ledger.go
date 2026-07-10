package zenithdtl

import "sort"

type AccountBalance struct {
	Account  AccountID `json:"account"`
	Asset    AssetID   `json:"asset"`
	Free     Amount    `json:"free"`
	Reserved Amount    `json:"reserved"`
	Locked   Amount    `json:"locked"`
}

func (b AccountBalance) Total() Amount {
	total, _ := SumAmounts(b.Free, b.Reserved, b.Locked)
	return total
}

type Vault struct {
	ID              VaultID   `json:"id"`
	Domain          DomainID  `json:"domain"`
	Asset           AssetID   `json:"asset"`
	Owner           AccountID `json:"owner"`
	Cash            Amount    `json:"cash"`
	Reserved        Amount    `json:"reserved"`
	Released        Amount    `json:"released"`
	PendingWithdraw Amount    `json:"pendingWithdraw"`
}

func (v Vault) Available() Amount {
	available := v.Cash
	if available < v.Reserved {
		return 0
	}
	available -= v.Reserved
	if available < v.PendingWithdraw {
		return 0
	}
	return available - v.PendingWithdraw
}

func (v Vault) Liability() Amount {
	total, _ := SumAmounts(v.Reserved, v.PendingWithdraw)
	return total
}

type LedgerEntry struct {
	Index     int        `json:"index"`
	Epoch     Epoch      `json:"epoch"`
	Account   AccountID  `json:"account,omitempty"`
	Vault     VaultID    `json:"vault,omitempty"`
	Asset     AssetID    `json:"asset"`
	Amount    Amount     `json:"amount"`
	Side      LedgerSide `json:"side"`
	Reference string     `json:"reference"`
}

type Ledger struct {
	accounts map[AccountID]map[AssetID]*AccountBalance
	vaults   map[VaultID]*Vault
	entries  []LedgerEntry
	next     int
	events   *EventLog
}

func NewLedger(events *EventLog) *Ledger {
	return &Ledger{
		accounts: map[AccountID]map[AssetID]*AccountBalance{},
		vaults:   map[VaultID]*Vault{},
		entries:  []LedgerEntry{},
		next:     1,
		events:   events,
	}
}

func (l *Ledger) EnsureAccount(account AccountID, asset AssetID) *AccountBalance {
	if _, ok := l.accounts[account]; !ok {
		l.accounts[account] = map[AssetID]*AccountBalance{}
	}
	if _, ok := l.accounts[account][asset]; !ok {
		l.accounts[account][asset] = &AccountBalance{Account: account, Asset: asset}
		if l.events != nil {
			l.events.Add(0, EventAccountOpened, string(account), eventAttrs("asset", string(asset)))
		}
	}
	return l.accounts[account][asset]
}

func (l *Ledger) HasAccount(account AccountID, asset AssetID) bool {
	if _, ok := l.accounts[account]; !ok {
		return false
	}
	_, ok := l.accounts[account][asset]
	return ok
}

func (l *Ledger) Account(account AccountID, asset AssetID) (AccountBalance, error) {
	if !l.HasAccount(account, asset) {
		return AccountBalance{}, refError(ErrUnknownAccount, string(account), "account balance not found")
	}
	return *l.accounts[account][asset], nil
}

func (l *Ledger) OpenVault(id VaultID, domain DomainID, asset AssetID, owner AccountID) (*Vault, error) {
	if id == "" || domain == "" || asset == "" || owner == "" {
		return nil, newError(ErrInvalidState, "vault requires id, domain, asset and owner")
	}
	if _, ok := l.vaults[id]; ok {
		return nil, refError(ErrDuplicate, string(id), "vault already exists")
	}
	vault := &Vault{ID: id, Domain: domain, Asset: asset, Owner: owner}
	l.vaults[id] = vault
	l.EnsureAccount(owner, asset)
	if l.events != nil {
		l.events.Add(0, EventVaultOpened, string(id), eventAttrs("domain", string(domain), "asset", string(asset)))
	}
	return vault, nil
}

func (l *Ledger) Vault(id VaultID) (*Vault, error) {
	vault, ok := l.vaults[id]
	if !ok {
		return nil, refError(ErrUnknownVault, string(id), "vault not found")
	}
	return vault, nil
}

func (l *Ledger) Deposit(epoch Epoch, account AccountID, asset AssetID, amount Amount, ref string) error {
	if err := NonZeroAmount(amount, ref); err != nil {
		return err
	}
	balance := l.EnsureAccount(account, asset)
	next, err := balance.Free.Add(amount)
	if err != nil {
		return err
	}
	balance.Free = next
	l.record(epoch, account, "", asset, amount, SideCredit, ref)
	if l.events != nil {
		l.events.Add(epoch, EventDeposit, string(account), eventAttrs("asset", string(asset), "amount", amountString(amount), "ref", ref))
	}
	return nil
}

func (l *Ledger) FundVault(epoch Epoch, vaultID VaultID, amount Amount, ref string) error {
	if err := NonZeroAmount(amount, ref); err != nil {
		return err
	}
	vault, err := l.Vault(vaultID)
	if err != nil {
		return err
	}
	next, err := vault.Cash.Add(amount)
	if err != nil {
		return err
	}
	vault.Cash = next
	l.record(epoch, vault.Owner, vault.ID, vault.Asset, amount, SideCredit, ref)
	if l.events != nil {
		l.events.Add(epoch, EventCredit, string(vault.ID), eventAttrs("asset", string(vault.Asset), "amount", amountString(amount), "ref", ref))
	}
	return nil
}

func (l *Ledger) Debit(epoch Epoch, account AccountID, asset AssetID, amount Amount, ref string) error {
	if err := NonZeroAmount(amount, ref); err != nil {
		return err
	}
	balance := l.EnsureAccount(account, asset)
	next, err := balance.Free.Sub(amount)
	if err != nil {
		return refError(ErrInsufficientBalance, string(account), "free balance too low")
	}
	balance.Free = next
	l.record(epoch, account, "", asset, amount, SideDebit, ref)
	if l.events != nil {
		l.events.Add(epoch, EventDebit, string(account), eventAttrs("asset", string(asset), "amount", amountString(amount), "ref", ref))
	}
	return nil
}

func (l *Ledger) Credit(epoch Epoch, account AccountID, asset AssetID, amount Amount, ref string) error {
	if amount < 0 {
		return refError(ErrInvalidAmount, ref, "amount must not be negative")
	}
	if amount == 0 {
		return nil
	}
	balance := l.EnsureAccount(account, asset)
	next, err := balance.Free.Add(amount)
	if err != nil {
		return err
	}
	balance.Free = next
	l.record(epoch, account, "", asset, amount, SideCredit, ref)
	if l.events != nil {
		l.events.Add(epoch, EventCredit, string(account), eventAttrs("asset", string(asset), "amount", amountString(amount), "ref", ref))
	}
	return nil
}

func (l *Ledger) ReserveFromAccount(epoch Epoch, account AccountID, asset AssetID, amount Amount, ref string) error {
	if err := NonZeroAmount(amount, ref); err != nil {
		return err
	}
	balance := l.EnsureAccount(account, asset)
	free, err := balance.Free.Sub(amount)
	if err != nil {
		return refError(ErrInsufficientBalance, string(account), "free balance too low")
	}
	reserved, err := balance.Reserved.Add(amount)
	if err != nil {
		return err
	}
	balance.Free = free
	balance.Reserved = reserved
	l.record(epoch, account, "", asset, amount, SideDebit, ref)
	if l.events != nil {
		l.events.Add(epoch, EventReserve, string(account), eventAttrs("asset", string(asset), "amount", amountString(amount), "ref", ref))
	}
	return nil
}

func (l *Ledger) ReleaseToAccount(epoch Epoch, account AccountID, asset AssetID, amount Amount, ref string) error {
	if err := NonZeroAmount(amount, ref); err != nil {
		return err
	}
	balance := l.EnsureAccount(account, asset)
	reserved, err := balance.Reserved.Sub(amount)
	if err != nil {
		return refError(ErrInsufficientBalance, string(account), "reserved balance too low")
	}
	free, err := balance.Free.Add(amount)
	if err != nil {
		return err
	}
	balance.Reserved = reserved
	balance.Free = free
	l.record(epoch, account, "", asset, amount, SideCredit, ref)
	if l.events != nil {
		l.events.Add(epoch, EventRelease, string(account), eventAttrs("asset", string(asset), "amount", amountString(amount), "ref", ref))
	}
	return nil
}

func (l *Ledger) ConsumeReserved(epoch Epoch, account AccountID, asset AssetID, amount Amount, ref string) error {
	if err := NonZeroAmount(amount, ref); err != nil {
		return err
	}
	balance := l.EnsureAccount(account, asset)
	reserved, err := balance.Reserved.Sub(amount)
	if err != nil {
		return refError(ErrInsufficientBalance, string(account), "reserved balance too low")
	}
	balance.Reserved = reserved
	l.record(epoch, account, "", asset, amount, SideDebit, ref)
	return nil
}

func (l *Ledger) ReserveVault(epoch Epoch, vaultID VaultID, amount Amount, ref string) error {
	if err := NonZeroAmount(amount, ref); err != nil {
		return err
	}
	vault, err := l.Vault(vaultID)
	if err != nil {
		return err
	}
	if vault.Available() < amount {
		return refError(ErrInsufficientBalance, string(vaultID), "vault available cash too low")
	}
	next, err := vault.Reserved.Add(amount)
	if err != nil {
		return err
	}
	vault.Reserved = next
	l.record(epoch, vault.Owner, vault.ID, vault.Asset, amount, SideDebit, ref)
	if l.events != nil {
		l.events.Add(epoch, EventReserve, string(vault.ID), eventAttrs("amount", amountString(amount), "ref", ref))
	}
	return nil
}

func (l *Ledger) ReleaseVault(epoch Epoch, vaultID VaultID, amount Amount, ref string) error {
	if err := NonZeroAmount(amount, ref); err != nil {
		return err
	}
	vault, err := l.Vault(vaultID)
	if err != nil {
		return err
	}
	reserved, err := vault.Reserved.Sub(amount)
	if err != nil {
		return refError(ErrInsufficientBalance, string(vaultID), "vault reserved cash too low")
	}
	vault.Reserved = reserved
	l.record(epoch, vault.Owner, vault.ID, vault.Asset, amount, SideCredit, ref)
	if l.events != nil {
		l.events.Add(epoch, EventRelease, string(vault.ID), eventAttrs("amount", amountString(amount), "ref", ref))
	}
	return nil
}

func (l *Ledger) SettleFromVault(epoch Epoch, vaultID VaultID, amount Amount, ref string) error {
	if err := NonZeroAmount(amount, ref); err != nil {
		return err
	}
	vault, err := l.Vault(vaultID)
	if err != nil {
		return err
	}
	reserved, err := vault.Reserved.Sub(amount)
	if err != nil {
		return refError(ErrInsufficientBalance, string(vaultID), "vault reserved cash too low")
	}
	cash, err := vault.Cash.Sub(amount)
	if err != nil {
		return refError(ErrInsufficientBalance, string(vaultID), "vault cash too low")
	}
	released, err := vault.Released.Add(amount)
	if err != nil {
		return err
	}
	vault.Reserved = reserved
	vault.Cash = cash
	vault.Released = released
	l.record(epoch, vault.Owner, vault.ID, vault.Asset, amount, SideDebit, ref)
	return nil
}

func (l *Ledger) QueueVaultWithdrawal(epoch Epoch, vaultID VaultID, amount Amount, ref string) error {
	if err := NonZeroAmount(amount, ref); err != nil {
		return err
	}
	vault, err := l.Vault(vaultID)
	if err != nil {
		return err
	}
	if vault.Available() < amount {
		return refError(ErrInsufficientBalance, string(vaultID), "vault available cash too low")
	}
	next, err := vault.PendingWithdraw.Add(amount)
	if err != nil {
		return err
	}
	vault.PendingWithdraw = next
	l.record(epoch, vault.Owner, vault.ID, vault.Asset, amount, SideDebit, ref)
	return nil
}

func (l *Ledger) PayVaultWithdrawal(epoch Epoch, vaultID VaultID, account AccountID, amount Amount, ref string) error {
	if err := NonZeroAmount(amount, ref); err != nil {
		return err
	}
	vault, err := l.Vault(vaultID)
	if err != nil {
		return err
	}
	pending, err := vault.PendingWithdraw.Sub(amount)
	if err != nil {
		return refError(ErrInsufficientBalance, string(vaultID), "pending withdrawal too low")
	}
	cash, err := vault.Cash.Sub(amount)
	if err != nil {
		return refError(ErrInsufficientBalance, string(vaultID), "vault cash too low")
	}
	vault.PendingWithdraw = pending
	vault.Cash = cash
	if err := l.Credit(epoch, account, vault.Asset, amount, ref); err != nil {
		return err
	}
	l.record(epoch, account, vault.ID, vault.Asset, amount, SideCredit, ref)
	return nil
}

func (l *Ledger) record(epoch Epoch, account AccountID, vault VaultID, asset AssetID, amount Amount, side LedgerSide, ref string) {
	l.entries = append(l.entries, LedgerEntry{
		Index:     l.next,
		Epoch:     epoch,
		Account:   account,
		Vault:     vault,
		Asset:     asset,
		Amount:    amount,
		Side:      side,
		Reference: ref,
	})
	l.next++
}

func (l *Ledger) Entries() []LedgerEntry {
	result := make([]LedgerEntry, len(l.entries))
	copy(result, l.entries)
	return result
}

func (l *Ledger) AccountRows() []AccountBalance {
	rows := []AccountBalance{}
	for _, byAsset := range l.accounts {
		for _, balance := range byAsset {
			rows = append(rows, *balance)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Account == rows[j].Account {
			return rows[i].Asset < rows[j].Asset
		}
		return rows[i].Account < rows[j].Account
	})
	return rows
}

func (l *Ledger) VaultRows() []Vault {
	rows := make([]Vault, 0, len(l.vaults))
	for _, vault := range l.vaults {
		rows = append(rows, *vault)
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].ID < rows[j].ID
	})
	return rows
}

func (l *Ledger) TotalAccountFree(asset AssetID) Amount {
	var total Amount
	for _, byAsset := range l.accounts {
		if balance, ok := byAsset[asset]; ok {
			total += balance.Free
		}
	}
	return total
}

func (l *Ledger) TotalAccountReserved(asset AssetID) Amount {
	var total Amount
	for _, byAsset := range l.accounts {
		if balance, ok := byAsset[asset]; ok {
			total += balance.Reserved
		}
	}
	return total
}

func (l *Ledger) TotalVaultCash(asset AssetID) Amount {
	var total Amount
	for _, vault := range l.vaults {
		if vault.Asset == asset {
			total += vault.Cash
		}
	}
	return total
}

func (l *Ledger) TotalVaultReserved(asset AssetID) Amount {
	var total Amount
	for _, vault := range l.vaults {
		if vault.Asset == asset {
			total += vault.Reserved
		}
	}
	return total
}

func (l *Ledger) TotalPendingWithdraw(asset AssetID) Amount {
	var total Amount
	for _, vault := range l.vaults {
		if vault.Asset == asset {
			total += vault.PendingWithdraw
		}
	}
	return total
}

func (l *Ledger) Conservation(asset AssetID) bool {
	accountFree := l.TotalAccountFree(asset)
	accountReserved := l.TotalAccountReserved(asset)
	vaultCash := l.TotalVaultCash(asset)
	if accountFree < 0 || accountReserved < 0 || vaultCash < 0 {
		return false
	}
	return true
}

func (l *Ledger) AccountIDs() []AccountID {
	keys := make([]AccountID, 0, len(l.accounts))
	for key := range l.accounts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

func (l *Ledger) VaultIDs() []VaultID {
	keys := make([]VaultID, 0, len(l.vaults))
	for key := range l.vaults {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}
