package storage_db_1

import (
	"encoding/json"
	"errors"
	

	//"log"
	"sync"

	bolt "go.etcd.io/bbolt"
)

var (
	db         *bolt.DB
	dbMutex    sync.Mutex
	bucketName = []byte("tg_auth_bot")
)

// InitDB initializes the BoltDB database
func InitDB(dbPath string) error {
	var err error
	db, err = bolt.Open(dbPath, 0666, nil)
	if err != nil {
		return err
	}

	// Create the main bucket if it doesn't exist
	return db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketName)
		return err
	})
}

// CloseDB closes the BoltDB database
func CloseDB() error {
	if db != nil {
		return db.Close()
	}
	return nil
}

// SaveData saves a key-value pair in the database
func SaveData(key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	dbMutex.Lock()
	defer dbMutex.Unlock()

	return db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			return errors.New("bucket not found")
		}
		return bucket.Put([]byte(key), data)
	})
}

// GetData retrieves data by key from the database
func GetData(key string, result interface{}) error {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	return db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			return errors.New("bucket not found")
		}

		data := bucket.Get([]byte(key))
		if data == nil {
			return errors.New("data not found")
		}

		return json.Unmarshal(data, result)
	})
}

// DeleteData removes a key-value pair from the database
func DeleteData(key string) error {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	return db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			return errors.New("bucket not found")
		}
		return bucket.Delete([]byte(key))
	})
}

// UserVerification represents user verification data
type UserVerification struct {
	UserID         int64
	Username       string
	GroupID        int64
	GroupName      string
	IsPending      bool
	Verified       bool
	SessionID      int64
	RestrictStatus bool
	AuthToken      string
	Role           string
	VerifyMsg      *VerifyMsg
}

// VerifyMsg represents a verification message
type VerifyMsg struct {
	MsgID int
	Msg   string
}

// AddVerificationMsg saves a verification message to a user
func (uv *UserVerification) AddVerificationMsg(msgID int, msg string) {
	uv.VerifyMsg = &VerifyMsg{
		MsgID: msgID,
		Msg:   msg,
	}
}

// DeleteVerificationMsg removes the verification message from a user
func (uv *UserVerification) DeleteVerificationMsg() {
	uv.VerifyMsg = nil
}

// AddOrUpdateUser saves or updates a user's verification data
func AddOrUpdateUser(userID int64, user *UserVerification) error {
	key := userKey(userID)
	return SaveData(key, user)
}

// GetUser retrieves a user's verification data by user ID
func GetUser(userID int64) (*UserVerification, error) {
	key := userKey(userID)
	var user UserVerification
	err := GetData(key, &user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdateUserVerificationStatus updates a user's verification status by user ID
func UpdateUserVerificationStatus(userID int64, updateFunc func(*UserVerification)) error {
	key := userKey(userID)

	dbMutex.Lock()
	defer dbMutex.Unlock()

	// Открываем транзакцию для чтения и обновления данных
	return db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			return errors.New("bucket not found")
		}

		// Получаем текущие данные пользователя
		data := bucket.Get([]byte(key))
		if data == nil {
			return errors.New("user not found")
		}

		// Десериализуем данные пользователя
		var user UserVerification
		if err := json.Unmarshal(data, &user); err != nil {
			return err
		}

		// Применяем функцию обновления
		updateFunc(&user)

		// Сериализуем обновлённые данные обратно
		updatedData, err := json.Marshal(user)
		if err != nil {
			return err
		}

		// Сохраняем обновлённые данные в хранилище
		return bucket.Put([]byte(key), updatedData)
	})
}

// DeleteUser removes a user's verification data by user ID
func DeleteUser(userID int64) error {
	key := userKey(userID)
	return DeleteData(key)
}

// GroupVerificationConfig represents verification configuration for a group
type GroupVerificationConfig struct {
	VerificationParams []VerificationParams
	ActiveIndex        int
	RestrictionType    string // block | delete
}

// VerificationParams represents parameters for verification
type VerificationParams struct {
	CircuitID string                 `json:"circuitId"`
	ID        uint32                 `json:"id"`
	Query     map[string]interface{} `json:"query"`
}

// AddOrUpdateGroupConfig saves or updates a group's verification config
func AddOrUpdateGroupConfig(groupID int64, config *GroupVerificationConfig) error {
	key := groupKey(groupID)
	return SaveData(key, config)
}

// GetGroupConfig retrieves a group's verification config by group ID
func GetGroupConfig(groupID int64) (*GroupVerificationConfig, error) {
	key := groupKey(groupID)
	var config GroupVerificationConfig
	err := GetData(key, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// GetRestrictionType retrieves the restriction type for a given group ID from the database
func GetRestrictionType(groupID int64) (string, error) {
	// Получаем конфигурацию группы
	config, err := GetGroupConfig(groupID)
	if err != nil {
		if errors.Is(err, bolt.ErrBucketNotFound) || err.Error() == "data not found" {
			// Если конфигурация группы отсутствует, возвращаем значение по умолчанию
			return "none", nil
		}
		return "", err // Возвращаем ошибку, если произошла другая проблема
	}

	// Если конфигурация найдена, возвращаем RestrictionType
	return config.RestrictionType, err
}

// SaveVerificationMessage saves a verification message for a user
func SaveVerificationMessage(userID int64, msgID int, msg string) error {
	// Получаем данные пользователя
	user, err := GetUser(userID)
	if err != nil {
		if errors.Is(err, bolt.ErrBucketNotFound) || err.Error() == "data not found" {
			return errors.New("user not found")
		}
		return err
	}

	// Добавляем верификационное сообщение
	user.AddVerificationMsg(msgID, msg)

	// Сохраняем обновленные данные пользователя в базе данных
	return AddOrUpdateUser(userID, user)
}



// DeleteGroupConfig removes a group's verification config by group ID
func DeleteGroupConfig(groupID int64) error {
	key := groupKey(groupID)
	return DeleteData(key)
}

// GroupSetupState represents admin-to-group mapping
var GroupSetupState = struct {
	State map[int64]int64
	Mutex sync.Mutex
}{State: make(map[int64]int64)}

// AddAdminUser maps an admin to a group
func AddAdminUser(userID, groupID int64) {
	GroupSetupState.Mutex.Lock()
	defer GroupSetupState.Mutex.Unlock()
	GroupSetupState.State[userID] = groupID
}

// GetGroupIDFromAdmin retrieves the group ID associated with an admin
func GetGroupIDFromAdmin(userID int64) int64 {
	GroupSetupState.Mutex.Lock()
	defer GroupSetupState.Mutex.Unlock()
	return GroupSetupState.State[userID]
}

// Helper to generate a unique key for a user
func userKey(userID int64) string {
	return "user_" + string(rune(userID))
}

// Helper to generate a unique key for a group
func groupKey(groupID int64) string {
	return "group_" + string(rune(groupID))
}

// VerifiedUser представляет информацию о верифицированном пользователе
type VerifiedUser struct {
	UserID        int64
	Username      string
	AuthToken     string
	VerificationType string
	AdminToken    string
}

// AddVerifiedUser сохраняет данные о верифицированном пользователе в базе данных
// func AddVerifiedUser(groupID, userID int64, username, authToken, verificationType, adminToken string) error {
// 	key := verifiedUserKey(groupID, userID)

// 	user := VerifiedUser{
// 		UserID:          userID,
// 		Username:        username,
// 		AuthToken:       authToken,
// 		VerificationType: verificationType,
// 		AdminToken:      adminToken,
// 	}

// 	return SaveData(key, user)
// }

// verifiedUserKey генерирует уникальный ключ для верифицированного пользователя
// func verifiedUserKey(groupID, userID int64) string {
// 	return fmt.Sprintf("verified_%d_%d", groupID, userID)
// }
