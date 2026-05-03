package persister

import "go.etcd.io/bbolt"

var (
	bucketName = []byte("raft")
	stateKey   = []byte("state")
	snapKey    = []byte("snapshot")
)

type BoltPersister struct {
	db *bbolt.DB
}

func New(path string) (*BoltPersister, error) {
	db, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketName)
		return err
	})
	if err != nil {
		db.Close()
		return nil, err
	}
	return &BoltPersister{db: db}, nil
}

func (p *BoltPersister) Save(state, snapshot []byte) {
	p.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketName)
		if state != nil {
			b.Put(stateKey, state)
		}
		if snapshot != nil {
			b.Put(snapKey, snapshot)
		}
		return nil
	})
}

func (p *BoltPersister) ReadRaftState() []byte {
	var out []byte
	p.db.View(func(tx *bbolt.Tx) error {
		v := tx.Bucket(bucketName).Get(stateKey)
		if v != nil {
			out = append([]byte(nil), v...)
		}
		return nil
	})
	return out
}

func (p *BoltPersister) ReadSnapshot() []byte {
	var out []byte
	p.db.View(func(tx *bbolt.Tx) error {
		v := tx.Bucket(bucketName).Get(snapKey)
		if v != nil {
			out = append([]byte(nil), v...)
		}
		return nil
	})
	return out
}

func (p *BoltPersister) RaftStateSize() int {
	var n int
	p.db.View(func(tx *bbolt.Tx) error {
		n = len(tx.Bucket(bucketName).Get(stateKey))
		return nil
	})
	return n
}

func (p *BoltPersister) Close() error {
	return p.db.Close()
}
