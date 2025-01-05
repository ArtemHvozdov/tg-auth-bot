package storage

import (
	"log"
	"sync"

	"gopkg.in/telebot.v3"
)

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

type VerifyMsg struct {
	msgId int
	msg *telebot.Message
}

// Method for add verification message
func (u *UserVerification) AddVerificationMsg(msgId int, msg *telebot.Message) {
	u.verifyMsg = &VerifyMsg{
		msgId: msgId,
		msg: msg,
	}
}

// Method for delete verification message
func (uv *UserVerification) DeleteVerifyMessage(bot *telebot.Bot) error {
	if uv.verifyMsg != nil && uv.verifyMsg.msg != nil {
		err := bot.Delete(uv.verifyMsg.msg)
		if err != nil {
			log.Printf("Failed to delete verification message for user %d: %v", uv.UserID, err)
			return err
		}
		log.Printf("Verification message deleted for user %d: message ID %d", uv.UserID, uv.verifyMsg.msgId)
		uv.verifyMsg = nil
	}
	return nil
}

var (
	UserStore = make(map[int64]*UserVerification)
	DataMutex        sync.Mutex // Mutex for synchronization
	DataChanges = make(chan UserChangeEvent, 100)
)

// UserChangeEvent - user data change event structure
type UserChangeEvent struct {
	UserID int64              // ID user
	Data   *UserVerification  // New dara by user
}

// AddOrUpdateUser - adds a new user or updates an existing one
func AddOrUpdateUser(userID int64, user *UserVerification) {
	DataMutex.Lock()
	defer DataMutex.Unlock()

	UserStore[userID] = user

	// Sending an event to a channel
	DataChanges <- UserChangeEvent{
		UserID: userID,
		Data:   user,
	}
}

// UpdateField - updates specified user fields
func UpdateField(userID int64, updateFunc func(*UserVerification)) {
	DataMutex.Lock()
	defer DataMutex.Unlock()

	// Getting user data
	user, exists := UserStore[userID]
	if !exists {
		return
	}

	// We update data using the function
	updateFunc(user)

	// Sending an event to a channel
	DataChanges <- UserChangeEvent{
		UserID: userID,
		Data:   user,
	}
}

// DeleteUser - removes a user from the repository
func DeleteUser(userID int64) {
	DataMutex.Lock()
	defer DataMutex.Unlock()

	// remove data by user
	delete(UserStore, userID)

	// Send delete event (Data becomes nil)
	DataChanges <- UserChangeEvent{
		UserID: userID,
		Data:   nil,
	}
}

// GetUser - returns user data
func GetUser(userID int64) (*UserVerification, bool) {
	DataMutex.Lock()
	defer DataMutex.Unlock()

	user, exists := UserStore[userID]
	return user, exists
}


// Struct for the parametrs of verification
type VerificationParams struct {
	CircuitID        string                 `json:"circuitId"`
	ID               uint32                 `json:"id"`
	Query            map[string]interface{} `json:"query"`
}

// Struct paramets: ID Group - auth parametrs
var VerificationParamsMap = make(map[int64]VerificationParams)

// Admin User ID - Group ID
var GroupSetupState = make(map[int64]int64)

func AddAdminUser(userID, groupID int64) {
	DataMutex.Lock()
	defer DataMutex.Unlock()

	GroupSetupState[userID] = groupID
}

func GetIdGroupFromGroupSetapState(userID int64) int64 {
	DataMutex.Lock()
	defer DataMutex.Unlock()

	groupID, exists := GroupSetupState[userID]
	if !exists {
		return 0
	}
	return groupID
}

// RestrictionType - type of restriction
// ID Chat Group -> Restriction Type ( block | delete )
var RestrictionType = make(map[int64]string)

func AddRestrictionType(groupID int64, restrictionType string) {
	DataMutex.Lock()
	defer DataMutex.Unlock()

	RestrictionType[groupID] = restrictionType
}

func GetRestrictionType(groupID int64) string {
	DataMutex.Lock()
	defer DataMutex.Unlock()

	restrictionType, exists := RestrictionType[groupID]
	if !exists {
		return ""
	}
	return restrictionType
}

// VerifiedUsersList - list of verified users
// Id Chat Group -> User Data 
var VerifiedUsersList = make(map[int64][]VerifiedUser)

type User struct {
	ID       int64
	UserName string
	VerifiedToken string
}

// User data
type VerifiedUser struct {
	User User
	TypeVerification string
	AuthToken string
}

// AddVerifiedUser - add user to verified list
func AddVerifiedUser(groupID int64, userID int64, userName string, VerifiedToken string, typeVerification string, authToken string) {
	DataMutex.Lock()
	defer DataMutex.Unlock()

	user := User{
		ID: userID,
		UserName: userName,
	}

	// Create a record for the verified user
	verifiedUser := VerifiedUser{
		User:            user,
		TypeVerification: typeVerification,
		AuthToken:        authToken,
	}

	// Add a new verified user to the group's list
	VerifiedUsersList[groupID] = append(VerifiedUsersList[groupID], verifiedUser)
}

// RemoveVerifiedUser removes a user from VerifiedUsersList by group ID and user ID
func RemoveVerifiedUser(groupID int64, userID int64)  {
	DataMutex.Lock()
	defer DataMutex.Unlock()

	// Check if the group exists in VerifiedUsersList
	users, exists := VerifiedUsersList[groupID]
	if !exists {
		log.Printf("Group %d not found in VerifiedUsersList", groupID)
		return
	}

	// Find the index of the user in the group's list
	index := -1
	for i, user := range users {
		if user.User.ID == userID {
			index = i
			break
		}
	}

	// If the user is not found, return false
	if index == -1 {
		log.Printf("User %d not found in group %d", userID, groupID)
		return 
	}

	// Remove the user from the list
	VerifiedUsersList[groupID] = append(users[:index], users[index+1:]...)

	// If the group's list is empty after removal, delete the group from VerifiedUsersList
	if len(VerifiedUsersList[groupID]) == 0 {
		delete(VerifiedUsersList, groupID)
	}
}
