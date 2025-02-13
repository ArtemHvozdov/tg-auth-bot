package storage_db

import (
	"encoding/json"
	"fmt"

	"log"
	"sync"

	bolt "go.etcd.io/bbolt"
	"gopkg.in/telebot.v3"
)

var (
	db         *bolt.DB
	DataMutex    sync.Mutex
	DataChanges = make(chan UserChangeEvent, 100)

)

// InitDB initializes the BoltDB database
func InitDB(dbPath string) error {
	var err error
	log.Println("Opening database...")

	db, err = bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return err
	}
	log.Println("Database opened successfully")

	// Create the main bucket if it doesn't exist
	return db.Update(func(tx *bolt.Tx) error {
		log.Println("Creating buckets if not exists...")

		buckets := []string{
			"UserStore",
			"VerificationParamsStore",
			"GroupSetupState",
			"VerifiedUsersList",
		}

		for _, bucket := range buckets {
			_, err := tx.CreateBucketIfNotExists([]byte(bucket))
			if err != nil {
				return fmt.Errorf("ошибка при создании bucket %s: %w", bucket, err)
			}
		}
		log.Println("Buckets created successfully")
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
	VerifyMsg *VerifyMsg
	AuthToken string
	Role string
}

// UserChangeEvent - user data change event structure for the channel
type UserChangeEvent struct {
	UserID int64              // ID user
	Data   *UserVerification  // New dara by user
}

// Struct for the verification message
type VerifyMsg struct {
	MsgId int
	Msg *telebot.Message
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
	log.Println("UpdateField DB is called")

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
		log.Println("UpdateField DB sending event to the channel")
		// Sending an event to a channel
		DataChanges <- UserChangeEvent{
			UserID: userID,
			Data:   user,
		}
		log.Println("UpdateField DB logs: info updated user:") 
		log.Println("Name:", user.Username)
		log.Println("IsPending:", user.IsPending)
		log.Println("Verified:", user.Verified)
		log.Println("User role:", user.Role)
	}

	return err
}

// DeleteUser - removes a user from the repository
func DeleteUser(userID int64) error {
	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("UserStore"))
		if bucket == nil {
			return fmt.Errorf("bucket %s not found", []byte("UserStore"))
		}

		// Delete data by user
		return bucket.Delete(itob(userID))
	})

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

// Method for add verification message
func AddVerificationMsg(userID int64, msgID int, msg *telebot.Message) error {
	return db.Update(func(tx *bolt.Tx) error {
		// Open bucket UserStore
		bucket := tx.Bucket([]byte("UserStore"))
		if bucket == nil {
			log.Println("bucket UserStore not found")
			return nil
		}

		// Get user data from the database
		data := bucket.Get(itob(userID))
		if data == nil {
			log.Println("user ID not found")
			return nil
		}

		// Decode JSON to struct
		var user UserVerification
		if err := json.Unmarshal(data, &user); err != nil {
			return err
		}

		// Update verifyMsg
		user.VerifyMsg = &VerifyMsg{
			MsgId: msgID,
			Msg:   msg,
		}

		// Encode back to JSON
		updatedData, err := json.Marshal(user)
		if err != nil {
			return err
		}

		// Save updated data to the database
		if err := bucket.Put(itob(userID), updatedData); err != nil {
			return err
		}

		return nil
	})
}

func DeleteVerifyMessage(bot *telebot.Bot, userID int64) error {
	// Get user data from the database
	user, err := GetUser(userID)
	if err != nil {
		log.Printf("Failed to get user %d: %v", userID, err)
		return err
	}

	// Check if there is a verification message to delete
	if user.VerifyMsg.MsgId == 0 && user.VerifyMsg.Msg == nil {
		log.Printf("No verification message to delete for user %d", userID)
		return nil
	}

	// Delete the verification message
	err = bot.Delete(user.VerifyMsg.Msg)
	if err != nil {
		log.Printf("Failed to delete verification message for user %d: %v", userID, err)
		return err
	}

	log.Printf("Verification message deleted for user %d: message ID %d", userID, user.VerifyMsg.MsgId)

	// Upadate user data in the database
	user.VerifyMsg.MsgId = 0
	user.VerifyMsg.Msg = nil // panicc

	return nil
}


// GetUserGroupID return user's GroupID
func GetUserGroupID(userID int64) (int64, error) {
	var groupID int64

	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("UserStore"))
		if bucket == nil {
			return fmt.Errorf("bucket UserStore not found")
		}

		data := bucket.Get(itob(userID))
		if data == nil {
			return fmt.Errorf("user %d not found", userID)
		}

		var user UserVerification
		if err := json.Unmarshal(data, &user); err != nil {
			return fmt.Errorf("failed to unmarshal user data: %v", err)
		}

		groupID = user.GroupID
		return nil
	})

	if err != nil {
		return 0, err
	}

	return groupID, nil
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

// GetVerificationType gets value "type" from VerificationParam
func GetVerificationType(groupID int64) (string, error) {
	var verificationType string

	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("VerificationParamsStore"))
		if bucket == nil {
			log.Println("bucket VerificationParamsStore not found")
			return nil
		}

		groupData := bucket.Get(itob(groupID))
		if groupData == nil {
			log.Println("group ID not found")
			return nil
		}

		var configGroupParams GroupVerificationConfig
		if err := json.Unmarshal(groupData, &configGroupParams); err != nil {
			return err
		}

		if len(configGroupParams.VerificationParams) == 0 {
			log.Println("no verification parameters found")
			return nil
		}

		activeIndex := configGroupParams.ActiveIndex
		if activeIndex >= len(configGroupParams.VerificationParams) {
			log.Println("active index out of range")
			return nil
		}

		params := configGroupParams.VerificationParams[activeIndex]
		verificationType = params.Query["type"].(string)
		return nil
	})

	if err != nil {
		return "", err
	}

	return verificationType, nil
}

// SaveVerificationParams save parametrs veriofication to DB
func SaveVerificationParams(groupID int64, params VerificationParams) error {
	return db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("VerificationParamsStore"))
		if err != nil {
			return err
		}

		groupData := bucket.Get([]byte(itob(groupID)))
		var groupConfig GroupVerificationConfig

		if groupData != nil {
			if err := json.Unmarshal(groupData, &groupConfig); err != nil {
				return err
			}
		} else {
			groupConfig = GroupVerificationConfig{
				VerificationParams: []VerificationParams{},
				ActiveIndex:        -1, // No active params
			}
		}

		// Add the new verification params to the group
		groupConfig.VerificationParams = append(groupConfig.VerificationParams, params)

		// Set the active index if it's the first params
		if groupConfig.ActiveIndex == -1 {
			groupConfig.ActiveIndex = 0
		}

		// Serialize the updated data
		updatedData, err := json.Marshal(groupConfig)
		if err != nil {
			return err
		}

		// Save the updated data to the database
		return bucket.Put([]byte(itob(groupID)), updatedData)
	})
}

// Delete all verification params for the group using groupID
func DeleteAllVerificationParams(groupID int64) error {
	return db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("VerificationParamsStore"))
		if bucket == nil {
			return fmt.Errorf("bucket VerificationParamsStore not found")
		}

		return bucket.Delete(itob(groupID))
	})
}

// Ger active verification params
func GetActiveVerificationParams(groupID int64) (VerificationParams, error) {
	var params VerificationParams

	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("VerificationParamsStore"))
		if bucket == nil {
			return fmt.Errorf("bucket VerificationParamsStore not found")
		}

		groupData := bucket.Get([]byte(itob(groupID)))
		if groupData == nil {
			return fmt.Errorf("group ID %v not found", groupID)
		}

		var groupConfig GroupVerificationConfig
		if err := json.Unmarshal(groupData, &groupConfig); err != nil {
			return err
		}

		if groupConfig.ActiveIndex == -1 {
			return fmt.Errorf("no active verification params found")
		}

		params = groupConfig.VerificationParams[groupConfig.ActiveIndex]
		return nil
	})

	if err != nil {
		return VerificationParams{}, err
	}

	return params, nil
}


// Get group config parametrs
func GetGroupConfigParams(groupID int64) (GroupVerificationConfig, error) {
	var groupConfig GroupVerificationConfig

	err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("VerificationParamsStore"))
		if bucket == nil {
			return fmt.Errorf("bucket VerificationParamsStore not found")
		}

		groupData := bucket.Get([]byte(itob(groupID)))
		if groupData == nil {
			return fmt.Errorf("group ID %v not found", groupID)
		}

		if err := json.Unmarshal(groupData, &groupConfig); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return GroupVerificationConfig{}, err
	}

	return groupConfig, nil
}

// SetActiveVerificationParams set active verification params
func SetActiveVerificationParams(groupID int64, index int) error {
	return db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("VerificationParamsStore"))
		if bucket == nil {
			return fmt.Errorf("bucket VerificationParamsStore not found")
		}

		// Get the group data from the DB
		groupData := bucket.Get([]byte(itob(groupID)))
		var groupConfig GroupVerificationConfig

		if groupData != nil {
			if err := json.Unmarshal(groupData, &groupConfig); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("group ID %v not found", groupID)
		}

		// Set new active index
		groupConfig.ActiveIndex = index

		updatedData, err := json.Marshal(groupConfig)
		if err != nil {
			return err
		}

		// Save the updated data to the DB
		return bucket.Put([]byte(itob(groupID)), updatedData)
	})
}

// ========================
// Functions for the GroupSetupState

// AddAdminUser - adds a new admin user to the GroupSetupState
func AddAdminUser(userID, groupID int64) error {
	return db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("GroupSetupState"))
		if bucket == nil {
			return fmt.Errorf("DB logs: bucket GroupSetupState not found")
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
			return fmt.Errorf("DB logs: bucket GroupSetupState not found")
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
		log.Println("AddVerifiedUser DB is called")
		log.Println("Agruments func AddVerifiedUser:")
		log.Println("groupID:", groupID)
		log.Println("userID:", userID)
		log.Println("userName:", userName)
		log.Println("VerifiedToken:", VerifiedToken)
		log.Println("typeVerification:", typeVerification)
		if authToken != "" {
			log.Println("authToken: there is it! All OK!",)
			// Opitional
			// log.Println("authToken:", authToken)
		} else {
			log.Println("authToken: parameter is empty")
		}

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

		log.Printf("AddVerifiedUser logs: User ID %d added/updated in group %d with data: %s", userID, groupID, string(data)) // LOG

		return nil
	})
}

// RemoveVerifiedUser - removes a user from VerifiedUsersList by group ID and user ID in database
func RemoveVerifiedUser(groupID int64, userID int64) {
	if db == nil {
		log.Println("ERROR: Database not initialized")
		return
	}

	err := db.Update(func(tx *bolt.Tx) error {
		log.Println("RemoveVerifiedUser DB is called")
		log.Printf("Arguments func RemoveVerifiedUser: groupID=%d, userID=%d", groupID, userID)

		bucket := tx.Bucket([]byte("VerifiedUsersList"))
		if bucket == nil {
			log.Println("ERROR: VerifiedUsersList bucket not found")
			return nil
		}

		groupBucket := bucket.Bucket(itob(groupID))
		if groupBucket == nil {
			log.Printf("Group %d not found in VerifiedUsersList", groupID)
			return nil
		}

		// Check if the user exists in the group's list
		userData := groupBucket.Get(itob(userID))
		if userData == nil {
			log.Printf("User %d not found in group %d", userID, groupID)
			return nil
		}

		// Remove the user from the group
		err := groupBucket.Delete(itob(userID))
		if err != nil {
			log.Printf("ERROR: Failed to remove user %d from group %d: %v", userID, groupID, err)
			return err
		}
		log.Printf("User %d removed from group %d", userID, groupID)

		// Check if the group is empty after the user removal
		if groupBucket.Stats().KeyN == 0 {
			err := bucket.DeleteBucket(itob(groupID))
			if err != nil {
				log.Printf("ERROR: Failed to remove empty group %d: %v", groupID, err)
				return err
			}
			log.Printf("Group %d removed from VerifiedUsersList (empty after user removal)", groupID)
		}

		return nil
	})

	if err != nil {
		log.Printf("ERROR: RemoveVerifiedUser failed: %v", err)
	}
}



// Delete all verified users for the group using groupID
func DeleteAllVerifiedUsers(groupID int64) {
	db.Update(func(tx *bolt.Tx) error {
		// Get the VerifiedUsersList bucket
		bucket := tx.Bucket([]byte("VerifiedUsersList"))

		// Delete the group bucket
		err := bucket.DeleteBucket(itob(groupID))
		if err != nil {
			return err
		}

		return nil
	})
}

// GetVerifiedUsersList - returns a list of verified users for the group
func GetVerifiedUsersList(groupID int64) ([]VerifiedUser, error) {
	var users []VerifiedUser

	err := db.View(func(tx *bolt.Tx) error {
		// Get the VerifiedUsersList bucket
		bucket := tx.Bucket([]byte("VerifiedUsersList"))
		if bucket == nil {
			return fmt.Errorf("bucket VerifiedUsersList not found")
		}

		// Get the group bucket
		groupBucket := bucket.Bucket(itob(groupID))
		if groupBucket == nil {
			return fmt.Errorf("group %d not found in VerifiedUsersList", groupID)
		}

		// Iterate over the users in the group
		err := groupBucket.ForEach(func(k, v []byte) error {
			if v == nil {
				return nil
			}
		
			var user VerifiedUser
			if err := json.Unmarshal(v, &user); err != nil {
				log.Printf("Error decoding user %s in group %d: %v", k, groupID, err)
				return nil // Пропускаем ошибку, но не останавливаем функцию
			}
		
			users = append(users, user)
			return nil
		})

		return err
	})

	if err != nil {
		return nil, err
	}

	return users, nil
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