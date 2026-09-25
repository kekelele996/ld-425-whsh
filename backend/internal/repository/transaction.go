package repository

import "gorm.io/gorm"

// TransactionManager 事务管理器，用于跨聚合（施工节点 + 节点用料）的原子更新。
type TransactionManager interface {
	RunInTx(fn func(tx *gorm.DB) error) error
}

type transactionManager struct {
	db *gorm.DB
}

// NewTransactionManager 构造事务管理器。
func NewTransactionManager(db *gorm.DB) TransactionManager {
	return &transactionManager{db: db}
}

func (m *transactionManager) RunInTx(fn func(tx *gorm.DB) error) error {
	return m.db.Transaction(fn)
}
