package customer

import (
	"context"
	"fmt"

	"react-api/internal/utils"

	"github.com/jackc/pgx/v5"
)

type OperationResult struct {
	TransactionID string `json:"transaction_id,omitempty"`
	ReferenceID   string `json:"reference_id,omitempty"`
	Amount        int64  `json:"amount"`
	Balance       int64  `json:"balance"`
}

func accountForUser(ctx context.Context, tx pgx.Tx, userID string) (id string, balance int64, err error) {
	err = tx.QueryRow(ctx, `SELECT a.id,a.balance FROM accounts a JOIN users u ON u.id=a.user_id WHERE a.user_id=$1 AND u.deleted_at IS NULL AND u.status='active' FOR UPDATE`, userID).Scan(&id, &balance)
	return
}

func Deposit(ctx context.Context, tx pgx.Tx, actorID, customerID string, amount int64, note string) (OperationResult, error) {
	accountID, before, err := accountForUser(ctx, tx, customerID)
	if err != nil {
		return OperationResult{}, fmt.Errorf("rekening customer tidak ditemukan")
	}
	after := before + amount
	if _, err := tx.Exec(ctx, `UPDATE accounts SET balance=$1,updated_at=now() WHERE id=$2`, after, accountID); err != nil {
		return OperationResult{}, err
	}
	id, _ := utils.NewCUID2()
	_, err = tx.Exec(ctx, `INSERT INTO transactions (id,actor_id,customer_id,destination_account_id,type,direction,amount,balance_before,balance_after,note) VALUES ($1,$2,$3,$4,'deposit','in',$5,$6,$7,$8)`, id, actorID, customerID, accountID, amount, before, after, note)
	return OperationResult{TransactionID: id, Amount: amount, Balance: after}, err
}

func Withdraw(ctx context.Context, tx pgx.Tx, actorID, customerID string, amount int64, note string) (OperationResult, error) {
	accountID, before, err := accountForUser(ctx, tx, customerID)
	if err != nil {
		return OperationResult{}, fmt.Errorf("rekening customer tidak ditemukan")
	}
	if before < amount {
		return OperationResult{}, fmt.Errorf("saldo tidak mencukupi")
	}
	after := before - amount
	if _, err := tx.Exec(ctx, `UPDATE accounts SET balance=$1,updated_at=now() WHERE id=$2`, after, accountID); err != nil {
		return OperationResult{}, err
	}
	id, _ := utils.NewCUID2()
	_, err = tx.Exec(ctx, `INSERT INTO transactions (id,actor_id,customer_id,source_account_id,type,direction,amount,balance_before,balance_after,note) VALUES ($1,$2,$3,$4,'withdraw','out',$5,$6,$7,$8)`, id, actorID, customerID, accountID, amount, before, after, note)
	return OperationResult{TransactionID: id, Amount: amount, Balance: after}, err
}

func Transfer(ctx context.Context, tx pgx.Tx, actorID, fromCustomerID, toCustomerID string, amount int64, note string) (OperationResult, error) {
	if fromCustomerID == toCustomerID {
		return OperationResult{}, fmt.Errorf("tujuan transfer tidak boleh sama")
	}
	rows, err := tx.Query(ctx, `SELECT a.id,a.user_id,a.balance FROM accounts a JOIN users u ON u.id=a.user_id WHERE a.user_id IN ($1,$2) AND u.deleted_at IS NULL AND u.status='active' ORDER BY a.id FOR UPDATE`, fromCustomerID, toCustomerID)
	if err != nil {
		return OperationResult{}, err
	}
	defer rows.Close()
	balances := map[string]int64{}
	accountIDs := map[string]string{}
	for rows.Next() {
		var accountID, userID string
		var balance int64
		if err := rows.Scan(&accountID, &userID, &balance); err != nil {
			return OperationResult{}, err
		}
		accountIDs[userID] = accountID
		balances[userID] = balance
	}
	fromAccountID, ok := accountIDs[fromCustomerID]
	if !ok {
		return OperationResult{}, fmt.Errorf("rekening sumber tidak ditemukan")
	}
	toAccountID, ok := accountIDs[toCustomerID]
	if !ok {
		return OperationResult{}, fmt.Errorf("rekening tujuan tidak ditemukan")
	}
	beforeFrom := balances[fromCustomerID]
	beforeTo := balances[toCustomerID]
	if beforeFrom < amount {
		return OperationResult{}, fmt.Errorf("saldo tidak mencukupi")
	}
	afterFrom := beforeFrom - amount
	afterTo := beforeTo + amount
	if _, err := tx.Exec(ctx, `UPDATE accounts SET balance=$1,updated_at=now() WHERE id=$2`, afterFrom, fromAccountID); err != nil {
		return OperationResult{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE accounts SET balance=$1,updated_at=now() WHERE id=$2`, afterTo, toAccountID); err != nil {
		return OperationResult{}, err
	}
	referenceID, _ := utils.NewCUID2()
	outID, _ := utils.NewCUID2()
	inID, _ := utils.NewCUID2()
	_, err = tx.Exec(ctx, `INSERT INTO transactions (id,actor_id,customer_id,source_account_id,destination_account_id,type,direction,amount,balance_before,balance_after,reference_id,note) VALUES ($1,$2,$3,$4,$5,'transfer','out',$6,$7,$8,$9,$10)`, outID, actorID, fromCustomerID, fromAccountID, toAccountID, amount, beforeFrom, afterFrom, referenceID, note)
	if err != nil {
		return OperationResult{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO transactions (id,actor_id,customer_id,source_account_id,destination_account_id,type,direction,amount,balance_before,balance_after,reference_id,note) VALUES ($1,$2,$3,$4,$5,'transfer','in',$6,$7,$8,$9,$10)`, inID, actorID, toCustomerID, fromAccountID, toAccountID, amount, beforeTo, afterTo, referenceID, note)
	return OperationResult{TransactionID: outID, ReferenceID: referenceID, Amount: amount, Balance: afterFrom}, err
}
