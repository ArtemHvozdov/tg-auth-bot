package storage_db

import (
	"encoding/json"
	// "errors"
	"fmt"

	"log"
	"sync"

	bolt "go.etcd.io/bbolt"
	//"gopkg.in/telebot.v3"
)

var (
	db         *bolt.DB
	DataMutex    sync.Mutex
	DataChanges = make(chan UserChangeEvent, 100)

	// storages
	// UserStore = make(map[int64]*UserVerification)
	// VerificationParamsMap = make(map[int64]GroupVerificationConfig)
	// GroupSetupState = make(map[int64]int64)
	//VerifiedUsersList = make(map[int64][]VerifiedUser)

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
	//verifyMsg *VerifyMsg
	AuthToken string
	Role string
}

// Struct for the verification message
// type VerifyMsg struct {
// 	msgId int
// 	msg *telebot.Message
// }

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
			return fmt.Errorf("bucket %s not found", []byte("UserStore"))
		}

		// Serialize the UserVerification structure to JSON
		data, err := json.Marshal(user)
		if err != nil {
			return err
		}

		// Write data to the bucket
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
			return fmt.Errorf("bucket %s not found", []byte("UserStore"))
		}

		// Getting current user data
		data := bucket.Get(itob(userID))
		if data == nil {
			return fmt.Errorf("User %d not found", userID)
		}

		user = &UserVerification{}
		if err := json.Unmarshal(data, user); err != nil {
			return err
		}

		// Update user data using the passed function
		updateFunc(user)

		// Save updated data back to the bucket
		updatedData, err := json.Marshal(user)
		if err != nil {
			return err
		}

		return bucket.Put(itob(userID), updatedData)
	})

	if err == nil {
		// Sending an event to a channel
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
			return fmt.Errorf("bucket %s not found", []byte("UserStore"))
		}

		// Delete data by user
		return bucket.Delete(itob(userID))
	})

	if err == nil {
		// Send delete event (Data becomes nil)
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
			return fmt.Errorf("bucket %s not found", []byte("UserStore"))
		}

		data := bucket.Get(itob(userID))
		if data == nil {
			return fmt.Errorf("User %d not found", userID)
		}

		return json.Unmarshal(data, &user)
	})

	if err != nil {
		return nil, err
	}

	return &user, nil
}

// ========================
// Functions for the VerificationParamsStore

// Add restriction type to group
func AddRestrictionType(groupID int64, restrictionType string) error {
	return db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("VerificationParamsStore"))
		if bucket == nil {
			return fmt.Errorf("bucket VerificationParamsStore not found")
		}

		// Read the current value from the database
		data := bucket.Get(itob(groupID))
		var groupConfig GroupVerificationConfig

		if data != nil {
			if err := json.Unmarshal(data, &groupConfig); err != nil {
				return fmt.Errorf("error parsing JSON: %w", err)
			}
		}

		// Update RestrictionType
		groupConfig.RestrictionType = restrictionType

		// Encode back to JSON and save to the database
		encoded, err := json.Marshal(groupConfig)
		if err != nil {
			return fmt.Errorf("error encoding JSON: %w", err)
		}

		return bucket.Put(itob(groupID), encoded)
	})
}

// Get restriction type from group
func GetRestrictionType(groupID int64) (string, error) {
	var restrictionType string

	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("VerificationParamsStore"))
		if bucket == nil {
			return fmt.Errorf("bucket VerificationParamsStore not found")
		}

		// Get the data from the bucket
		data := bucket.Get(itob(groupID))
		if data == nil {
			return nil // The group wasn't find, return empty value
		}

		var groupConfig GroupVerificationConfig
		if err := json.Unmarshal(data, &groupConfig); err != nil {
			return fmt.Errorf("error parsing JSON: %w", err)
		}

		restrictionType = groupConfig.RestrictionType
		return nil
	})

	return restrictionType, err
}

// ========================
// Functions for the GroupSetupState

// AddAdminUser - adds a new admin user to the GroupSetupState
func AddAdminUser(userID, groupID int64) error {
	return db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("GroupSetupState"))
		if bucket == nil {
			return fmt.Errorf("bucket GroupSetupState not found")
		}

		// Save groupID as a value using userID as a key
		return bucket.Put(itob(userID), itob(groupID))
	})
}

// GetIdGroupFromGroupSetapState - returns the group ID by the admin user ID
func GetIdGroupFromGroupSetupState(userID int64) (int64, error) {
	var groupID int64

	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("GroupSetupState"))
		if bucket == nil {
			return fmt.Errorf("bucket GroupSetupState not found")
		}

		// Get the value by key
		data := bucket.Get(itob(userID))
		if data == nil {
			return nil // If there is no record, return 0
		}

		groupID = btoi(data)
		return nil
	})

	return groupID, err
}

// ========================

// Functions for the VerifiedUsersList

// AddVerifiedUser - add user to verified list in database
func AddVerifiedUser(groupID int64, userID int64, userName string, VerifiedToken string, typeVerification string, authToken string) {
	db.Update(func(tx *bolt.Tx) error {
		// Get the VerifiedUsersList bucket
		bucket := tx.Bucket([]byte("VerifiedUsersList"))

		// Create a new VerifiedUser object
		user := User{
			ID:       userID,
			UserName: userName,
		}

		// Create the verifiedUser object
		verifiedUser := VerifiedUser{
			User:             user,
			TypesVerification: []string{typeVerification},
			AuthToken:         authToken,
		}

		// Serialize the VerifiedUser into JSON
		data, err := json.Marshal(verifiedUser)
		if err != nil {
			return err
		}

		// Get the existing list of users for the group
		groupBucket := bucket.Bucket(itob(groupID))
		if groupBucket == nil {
			// If group not exists, create a new list
			groupBucket, err = bucket.CreateBucket(itob(groupID))
			if err != nil {
				return err
			}
		}

		// Check if the user exists in the group's list
		userBucket := groupBucket.Bucket(itob(userID))
		if userBucket != nil {
			// User exists, update their verification types and auth token
			var existingUser VerifiedUser
			err := json.Unmarshal(userBucket.Get([]byte("user_data")), &existingUser)
			if err != nil {
				return err
			}
			// Update types of verification
			existingUser.TypesVerification = appendIfNotExists(existingUser.TypesVerification, typeVerification)
			existingUser.AuthToken = authToken

			// Re-serialize the updated user and save it back to the database
			data, err = json.Marshal(existingUser)
			if err != nil {
				return err
			}
		}

		// Add or update the user data in the group bucket
		err = groupBucket.Put(itob(userID), data)
		if err != nil {
			return err
		}

		return nil
	})
}


// RemoveVerifiedUser - removes a user from VerifiedUsersList by group ID and user ID in database
func RemoveVerifiedUser(groupID int64, userID int64) {
	db.Update(func(tx *bolt.Tx) error {
		// Get the VerifiedUsersList bucket
		bucket := tx.Bucket([]byte("VerifiedUsersList"))

		// Get the group bucket
		groupBucket := bucket.Bucket(itob(groupID))
		if groupBucket == nil {
			log.Printf("Group %d not found in VerifiedUsersList", groupID)
			return nil
		}

		// Get the user bucket
		userBucket := groupBucket.Bucket(itob(userID))
		if userBucket == nil {
			log.Printf("User %d not found in group %d", userID, groupID)
			return nil
		}

		// Delete the user bucket
		err := groupBucket.DeleteBucket(itob(userID))
		if err != nil {
			return err
		}

		// If the group is now empty, remove the group
		if groupBucketStats := groupBucket.Stats(); groupBucketStats.KeyN == 0 {
			err := bucket.DeleteBucket(itob(groupID))
			if err != nil {
				return err
			}
		}

		return nil
	})
}



// Helper functions

// itob - converts int64 to bytes (needed for keys in bbolt)
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

// btoi - converts bytes back to int64 (needed for retrieving keys in bbolt)
func btoi(b []byte) int64 {
	return int64(b[0])<<56 |
		int64(b[1])<<48 |
		int64(b[2])<<40 |
		int64(b[3])<<32 |
		int64(b[4])<<24 |
		int64(b[5])<<16 |
		int64(b[6])<<8 |
		int64(b[7])
}

// Helper function to append a string to a slice if it doesn't already exist
func appendIfNotExists(slice []string, item string) []string {
	for _, existingItem := range slice {
		if existingItem == item {
			return slice
		}
	}
	return append(slice, item)
}