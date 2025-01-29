package storage_db

import (
	"encoding/json"
	// "errors"
	"fmt"

	//"log"
	"sync"

	bolt "go.etcd.io/bbolt"
	"gopkg.in/telebot.v3"
)

var (
	db         *bolt.DB
	DataMutex    sync.Mutex
	DataChanges = make(chan UserChangeEvent, 100)

	// storages
	UserStore = make(map[int64]*UserVerification)
	VerificationParamsMap = make(map[int64]GroupVerificationConfig)
	GroupSetupState = make(map[int64]int64)
	VerifiedUsersList = make(map[int64][]VerifiedUser)

)

// InitDB initializes the BoltDB database
func InitDB(dbPath string) error {
	var err error
	db, err = bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return err
	}

	// Create the main bucket if it doesn't exist
	return db.Update(func(tx *bolt.Tx) error {
		buckets := []string{
			"UserStore",				// []byte("UserStore")
			"VerificationParamsStore",  // []byte("VerificationParamsStore")
			"GroupSetupState",			// []byte("GroupSetupState")
			"VerifiedUsersList",		// []byte("VerifiedUsersList")
		}

		for _, bucket := range buckets {
			_, err := tx.CreateBucketIfNotExists([]byte(bucket))
			if err != nil {
				return fmt.Errorf("ошибка при создании bucket %s: %w", bucket, err)
			}
		}
		return nil
	})	
}

// CloseDB closes the BoltDB database
func CloseDB() error {
	if db != nil {
		return db.Close()
	}
	return nil
}

// Custoom structs

// Struct for the user verification
type UserVerification struct {
	UserID    int64
	Username  string
	GroupID   int64
	GroupName string
	IsPending bool
	Verified  bool
	SessionID int64
	RestrictStatus bool
	verifyMsg *VerifyMsg
	AuthToken string
	Role string
}

// Struct for the verification message
type VerifyMsg struct {
	msgId int
	msg *telebot.Message
}

// UserChangeEvent - user data change event structure for the channel
type UserChangeEvent struct {
	UserID int64              // ID user
	Data   *UserVerification  // New dara by user
}

// Struct for the config veroification params for the group
type GroupVerificationConfig struct {
	VerificationParams []VerificationParams
	ActiveIndex		int
	RestrictionType string // block | delete
}

// Struct for the parametrs of verification
type VerificationParams struct {
	CircuitID        string                 `json:"circuitId"`
	ID               uint32                 `json:"id"`
	Query            map[string]interface{} `json:"query"`
}

// Struct for the verified user
type VerifiedUser struct {
	User User
	TypesVerification []string
	AuthToken string
}

// Struct for the user for the struc ferified users
type User struct {
	ID       int64
	UserName string
	VerifiedToken string
}

// Functions for the UserStore

// AddOrUpdateUser - adds a new user or updates an existing one
func AddOrUpdateUser(userID int64, user *UserVerification) error {
	DataMutex.Lock()
	defer DataMutex.Unlock()

	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("UserStore"))
		if bucket == nil {
			return fmt.Errorf("bucket %s не найден", []byte("UserStore"))
		}

		// Сериализуем структуру UserVerification в JSON
		data, err := json.Marshal(user)
		if err != nil {
			return err
		}

		// Записываем данные в bucket
		return bucket.Put(itob(userID), data)
	})

	if err == nil {
		// Отправляем событие в канал
		DataChanges <- UserChangeEvent{
			UserID: userID,
			Data:   user,
		}
	}

	return err
}

// UpdateField - updates specified user fields
func UpdateField(userID int64, updateFunc func(*UserVerification)) error {
	DataMutex.Lock()
	defer DataMutex.Unlock()

	var user *UserVerification

	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("UserStore"))
		if bucket == nil {
			return fmt.Errorf("bucket %s не найден", []byte("UserStore"))
		}

		// Получаем текущие данные пользователя
		data := bucket.Get(itob(userID))
		if data == nil {
			return fmt.Errorf("пользователь %d не найден", userID)
		}

		user = &UserVerification{}
		if err := json.Unmarshal(data, user); err != nil {
			return err
		}

		// Обновляем данные пользователя через переданную функцию
		updateFunc(user)

		// Сохраняем обновленные данные обратно в bucket
		updatedData, err := json.Marshal(user)
		if err != nil {
			return err
		}

		return bucket.Put(itob(userID), updatedData)
	})

	if err == nil {
		// Отправляем событие в канал
		DataChanges <- UserChangeEvent{
			UserID: userID,
			Data:   user,
		}
	}

	return err
}

// DeleteUser - removes a user from the repository
func DeleteUser(userID int64) error {
	DataMutex.Lock()
	defer DataMutex.Unlock()

	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("UserStore"))
		if bucket == nil {
			return fmt.Errorf("bucket %s не найден", []byte("UserStore"))
		}

		// Удаляем пользователя
		return bucket.Delete(itob(userID))
	})

	if err == nil {
		// Отправляем событие о удалении пользователя
		DataChanges <- UserChangeEvent{
			UserID: userID,
			Data:   nil,
		}
	}

	return err
}

// GetUser - returns user data
func GetUser(userID int64) (*UserVerification, error) {
	DataMutex.Lock()
	defer DataMutex.Unlock()

	var user UserVerification

	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("UserStore"))
		if bucket == nil {
			return fmt.Errorf("bucket %s не найден", []byte("UserStore"))
		}

		data := bucket.Get(itob(userID))
		if data == nil {
			return fmt.Errorf("пользователь %d не найден", userID)
		}

		return json.Unmarshal(data, &user)
	})

	if err != nil {
		return nil, err
	}

	return &user, nil
}






// Helper functions

// itob - конвертирует int64 в байты (нужно для ключей в bbolt)
func itob(v int64) []byte {
	b := make([]byte, 8)
	b[0] = byte(v >> 56)
	b[1] = byte(v >> 48)
	b[2] = byte(v >> 40)
	b[3] = byte(v >> 32)
	b[4] = byte(v >> 24)
	b[5] = byte(v >> 16)
	b[6] = byte(v >> 8)
	b[7] = byte(v)
	return b
}