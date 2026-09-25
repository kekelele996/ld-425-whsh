package repository

import "gorm.io/gorm"

// TxRunner 在单个数据库事务中执行一组仓储操作。
type TxRunner interface {
	Transaction(fn func(tx *gorm.DB) error) error
}

type txRunner struct {
	db *gorm.DB
}

// NewTxRunner 构造事务执行器。
func NewTxRunner(db *gorm.DB) TxRunner {
	return &txRunner{db: db}
}

func (r *txRunner) Transaction(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}
