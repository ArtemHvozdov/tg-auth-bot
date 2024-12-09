package storage

import "sync"

type UserVerification struct {
	UserID    int64
	Username  string
	GroupID   int64
	GroupName string
	IsPending bool
	Verified  bool
	SessionID int64
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
	ID               string                 `json:"id"`
	Query            map[string]interface{} `json:"query"`
}

// Struct paramets: ID Group - auth parametrs
var VerificationParamsMap = make(map[int64]VerificationParams)