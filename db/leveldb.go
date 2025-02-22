package db

import (
	"fmt"

	"github.com/syndtr/goleveldb/leveldb"
)

type LevelDBClient struct {
	DB *leveldb.DB
}

func NewLevelDBClient(dbPath string) (*LevelDBClient, error) {
	db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		fmt.Errorf("leveldb 열기 실패: %v", err)
	}
	return &LevelDBClient{DB: db}, nil
}

func (c *LevelDBClient) Put(key, value []byte) error {
	err := c.DB.Put(key, value, nil)
	if err != nil {
		return fmt.Errorf("데이터 저장 실패: %v", err)
	}
	return nil
}

func (c *LevelDBClient) Get(key []byte) ([]byte, error) {
	value, err := c.DB.Get(key, nil) 
	if err == leveldb.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed not found data %v", err)
	}
	return value, nil
}

func (c *LevelDBClient) Delete(key []byte) error {
	err := c.DB.Delete(key, nil)
	if err != nil {
		return fmt.Errorf("데이터 삭제 실패: %v", err)
	}
	return nil
}

func (c *LevelDBClient) Close() error {
	return c.DB.Close()
}

