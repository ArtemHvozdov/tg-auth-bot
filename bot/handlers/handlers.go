package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	//"strconv"
	//"sync"
	"github.com/ArtemHvozdov/tg-auth-bot/auth"
	
	"github.com/ArtemHvozdov/tg-auth-bot/storage_db"

	"time"

	"gopkg.in/telebot.v3"
)

var DataMutex sync.Mutex

// isAdmin checks if the user is a group admin
func isAdmin(bot *telebot.Bot, chatID int64, userID int64) bool {
	member, err := bot.ChatMemberOf(&telebot.Chat{ID: chatID}, &telebot.User{ID: userID})
	if err != nil {
		log.Printf("Bot handlers log:(isAdmin func) - Error fetching user role: %v", err)
		return false
	}
	return member.Role == "administrator" || member.Role == "creator"
}

// Handler for /start
func StartHandler(bot *telebot.Bot) func(c telebot.Context) error {
    return func(c telebot.Context) error {
        userName := c.Sender().Username
       
        msg := fmt.Sprintf(
            "Hello, %s!\n\nIf you want to be verified, run the command /verify.\nIf you want to configure the bot to verify participants, run the command /setup.",
            userName,
        )
        return c.Send(msg)
    }
}
// /setup command - administrator initiates configuration
func SetupHandler(bot *telebot.Bot) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		// Step 1: Send a message about the need to add the bot to the group with administrator rights
		msg := "To set me up for verification in your group, please add me to the group as an administrator and call the /check_admin command in the group."
		if err := c.Send(msg); err != nil {
			log.Printf("Blog handler log: (SetupHandler func) - Error sending setup message: %v", err)
			return err
		}
		return nil
	}
}

// /check_admin command - check administrator rights in a group and verify verification parameters
func CheckAdminHandler(bot *telebot.Bot) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		// Check the type of chat
		if c.Chat().Type == telebot.ChatPrivate {
			// Send a message to the user indicating that this command is not available in private chats
			return c.Send("This command can only be used in group or supergroup chats.")
		}

		chatID := c.Chat().ID // ID group chat (future: need rename this variable to groupID)
		userID := c.Sender().ID // ID admin user (future: need rename this variable to adminID)
		chatName := c.Chat().Title // Getting the name of the chat (group)
		userName := c.Sender().Username // Username

		storage_db.AddAdminUser(userID, chatID)

		log.Printf("Bot handler log: (CheckAdminHandler func) - User ID: %d, Chat ID: %d, Command received", userID, chatID)
		log.Printf("Bot handler log: (CheckAdminHandler func) - User's name: %s %s (@%s)", c.Sender().FirstName, c.Sender().LastName, c.Sender().Username)

		// Send a message to the user
		msgContinueForAdmin, _ := bot.Send(&telebot.Chat{ID: chatID}, "Administrator, return to the private chat with me to continue configuring the settings")

		// Delete the message after 1 minute
		go func() {
			time.Sleep(1 * time.Minute)
			if err := bot.Delete(msgContinueForAdmin); err != nil {
				log.Printf("Bot handler log: (CheckAdminHandler func) - Error deleting continue for admins message: %v", err)
			}
		}()

		// Checking if the bot is an administrator in this group
		member, err := bot.ChatMemberOf(&telebot.Chat{ID: chatID}, &telebot.User{ID: bot.Me.ID})
		if err != nil {
			log.Printf("Bot handler log: (CheckAdminHandler func) - Bot handler log: (ChecnkAdminHandler func) - Error fetching bot's role in the group: %v", err)
			// Send a private message to the user
			msg := "I couldn't fetch my role in this group. Please make sure I am an administrator."
			if _, err := bot.Send(&telebot.User{ID: userID}, msg); err != nil {
				log.Printf("Bot handler log: (CheckAdminHandler func) - Error sending bot admin check message: %v", err)
				return err
			}
			return nil
		}

		// Logging the bot's role
		log.Printf("Bot handler log: (CheckAdminHandler func) - Bot's role in the group '%s': %s", chatName, member.Role)

		// Checking if the bot is an administrator
		if member.Role != "administrator" && member.Role != "creator" {
			msg := fmt.Sprintf("I am not an administrator in the group '%s'. Please promote me to an administrator.", chatName)
			if _, err := bot.Send(&telebot.User{ID: userID}, msg); err != nil {
				log.Printf("Bot handler log: (CheckAdminHandler func) - Error sending bot admin check message: %v", err)
				return err
			}
			return nil
		}

		// Checking if the user the bot is interacting with is an administrator
		memberUser, err := bot.ChatMemberOf(&telebot.Chat{ID: chatID}, &telebot.User{ID: userID})
		if err != nil {
			log.Printf("Bot handler log: (CheckAdminHandler func) - Error fetching user's role: %v", err)
			// Send a private message to the user
			msg := "I couldn't fetch your role in this group."
			if _, err := bot.Send(&telebot.User{ID: userID}, msg); err != nil {
				log.Printf("Bot handler log: (CheckAdminHandler func) - Error sending user admin check message: %v", err)
				return err
			}
			return nil
		}

		// Logging the user role
		log.Printf("Bot handler log: (CheckAdminHandler func) - User's role in the group '%s': %s", chatName, memberUser.Role)

		// Checking if the user is an administrator
		if memberUser.Role != "administrator" && memberUser.Role != "creator" {
			// We inform the user that he is not an administrator
			groupMsg := fmt.Sprintf("@%s, you are not an administrator in the group '%s'. You cannot configure me for this group.", userName, chatName)
			if _, err := bot.Send(&telebot.Chat{ID: chatID}, groupMsg); err != nil {
				log.Printf("Bot handler log: (CheckAdminHandler func) - Error sending message to group: %v", err)
				return err
			}
			return nil
		}

		// All checks were successful
		msg := fmt.Sprintf("I have confirmed your admin status and my role in the group '%s'. You can now proceed with the setup.", chatName)
		if _, err := bot.Send(&telebot.User{ID: userID}, msg); err != nil {
			log.Printf("Bot handler log: (CheckAdminHandler func) - Error sending success message to user: %v", err)
			return err
		}

		time.Sleep(700*time.Millisecond)
			// Ask for verification parameters
		bot.Send(&telebot.User{ID: userID}, "To add verification parameters, call the command\n /add_verification_params")

		return nil
	}
}

// A new user has joined the group
func NewUserJoinedHandler(bot *telebot.Bot) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		for _, member := range c.Message().UsersJoined {
			if isAdmin(bot, c.Chat().ID, member.ID) {
				log.Printf("Bot handler log:(NewUserJoinedHandler) - Skipping admin user @%s (ID: %d)", member.Username, member.ID)
				continue
			}

			// Adding a new user to the repository
			newUser := &storage_db.UserVerification{
				UserID:    member.ID,
				Username:  member.Username,
				GroupID:   c.Chat().ID,
				GroupName: c.Chat().Title,
				IsPending: true,
				Verified:  false,
				SessionID: 0,
				RestrictStatus: true,
			}

			storage_db.AddOrUpdateUser(member.ID, newUser)

			log.Println("Bot handler log:(NewUserJoinedHandler) - New user:", newUser)

			typeRestriction, err := storage_db.GetRestrictionType(c.Chat().ID)
			if err != nil {
				log.Printf("Bot handler log:(NewUserJoinedHandler) - Error getting restriction type: %v", err)
				return err
			}

			log.Println("Bot handler log:(NewUserJoinedHandler) - New user handler was called")
			log.Println("Bot handler log:(NewUserJoinedHandler) - Type restriction:", typeRestriction)

			// Restrict the user if the restriction type is "block"
			if typeRestriction == "block" {
				err := bot.Restrict(c.Chat(), &telebot.ChatMember{
					User: &telebot.User{ID: member.ID},
					Rights: telebot.Rights{
						CanSendMessages: false, // Complete ban on sending messages
					},
				})
				if err != nil {
					log.Println("Bot handler log:(NewUserJoinedHandler) - New user handler")
					log.Printf("Bot handler log:(NewUserJoinedHandler) - Failed to restrict user @%s (ID: %d): %s", member.Username, member.ID, err)
					continue
				}
			}

			log.Println("Bot handler log:(NewUserJoinedHandler) - new member -", newUser)

			btn := telebot.InlineButton{
				Text: "Verify your age",
				URL:  fmt.Sprintf("https://t.me/%s", bot.Me.Username),
			}

			inlineKeys := [][]telebot.InlineButton{{btn}}
			log.Printf("Bot handler log:(NewUserJoinedHandler) - New member @%s added to verification queue.", member.Username)

			msg, err := bot.Send(
				c.Chat(),
				fmt.Sprintf("Hi, @%s! Please verify your age by clicking the button below and call /verify command.", member.Username),
				&telebot.ReplyMarkup{InlineKeyboard: inlineKeys},
			)
			if err != nil {
				log.Printf("Bot handler log:(NewUserJoinedHandler) - Error sending verification message: %v", err)
				return err
			}

			// Save the message ID for further deletion
			storage_db.AddVerificationMsg(member.ID, msg.ID, msg)

			go handleVerificationTimeout(bot, member.ID, c.Chat().ID)
		}

		return nil
	}
}

// Handler /verify
func VerifyHandler(bot *telebot.Bot) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		userID := c.Sender().ID

		userData, err := storage_db.GetUser(userID)
		if err != nil || !userData.IsPending {
			log.Printf("Bot handler log:(VerifyHandler) - User @%s (ID: %d) is not awaiting verification.", c.Sender().Username, userID)
			return c.Send("You are not awaiting verification in any group.")
		}

		msg := fmt.Sprintf(
			"Hi, @%s! To remain in the group \"%s\", you need to complete the verification process.",
			userData.Username,
			userData.GroupName,
		)
		if err := c.Send(msg); err != nil {
			return err
		}

		// userGroupID := storage_db.UserStore[userID].GroupID
		userGroupID, err := storage_db.GetUserGroupID(userID)
		if err != nil {
			log.Printf("Bot handler log:(VerifyHandler) - Error getting user group ID: %v", err)
			return err
		}

		// Get active verification parameters

		activeParams, _ := storage_db.GetActiveVerificationParams(userGroupID)

		log.Println("Bot handler log:(VerifyHandler) func GetActiveVerificationParams - Active verification parameters:", activeParams)

		groupConfig, err := storage_db.GetGroupConfigParams(userGroupID)
		if err != nil {
			log.Printf("Bot handler log:(VerifyHandler) - Error getting group configuration: %v", err)
		}

		params := groupConfig.VerificationParams[groupConfig.ActiveIndex]

		jsonData, _ := auth.GenerateAuthRequest(userID, params)

		base64Data := base64.StdEncoding.EncodeToString(jsonData)

		// Create deeplink
		deepLink := fmt.Sprintf("https://wallet.privado.id/#i_m=%s", base64Data)

		// logs deeplinl
		log.Println("Bot handler log:(VerifyHandler) - Deep Link:", deepLink)

		btn := telebot.InlineButton{
			Text: "Verify with Privado ID", // Text button
			URL:  deepLink,                // URL for redirect
		}

		// Creating markup with a button
		inlineKeyboard := &telebot.ReplyMarkup{}
		inlineKeyboard.InlineKeyboard = [][]telebot.InlineButton{{btn}}

		time.Sleep(2 * time.Second)
		// Send a message with a button
		return c.Send("Please click the button below to verify your age:", inlineKeyboard)
	}
}

// Handling verification timeout
func handleVerificationTimeout(bot *telebot.Bot, userID, groupID int64) {
	time.Sleep(10 * time.Minute)

	userData, err := storage_db.GetUser(userID)
	if err == nil && userData.IsPending && !userData.Verified {
		log.Printf("Bot handler log:(handleVerificationTimeout) - User @%s (ID: %d) failed verification on time. Removing from group.", userData.Username, userID)
		bot.Ban(&telebot.Chat{ID: groupID}, &telebot.ChatMember{User: &telebot.User{ID: userID}})
		time.Sleep(1 * time.Second)
		bot.Unban(&telebot.Chat{ID: groupID}, &telebot.User{ID: userID})
		bot.Send(&telebot.User{ID: userID}, "You did not complete the verification on time and were removed from the group.")
		storage_db.DeleteUser(userID)
	}
}

// Store change listener
func ListenForstorage_dbChanges(bot *telebot.Bot) {
	go func() { // panic: runtime error: invalid memory address or nil pointer dereference
		for event := range storage_db.DataChanges {
			userID := event.UserID
			data := event.Data

			if data == nil {
				// User was delete
				log.Printf("Bot handler log:(ListenForstorage_dbChanges) - User ID: %d was removed from the store.", userID)
				data, _ = storage_db.GetUser(userID)
				if data == nil {
					log.Println("Bot handler log:(ListenForstorage_dbChanges) - Error getting user data")
				}
				continue
			}

			groupChatID := data.GroupID

			typeRestriction, err := storage_db.GetRestrictionType(groupChatID)
			if err != nil {
				log.Printf("Bot handler log:(ListenForstorage_dbChanges) - Error getting restriction type from: %v", err)
				continue
			}

			userIsAdminGroup := checkUserAsAdminInGroup(userID, groupChatID)

			if !data.IsPending {
				if data.Verified {
					// Successful verification
					log.Printf("Bot handler log:(ListenForstorage_dbChanges) - User @%s (ID: %d) passed verification.", data.Username, userID)
					
					// Restrict the user
					if typeRestriction == "block" && !userIsAdminGroup {
						err := bot.Restrict(&telebot.Chat{ID: groupChatID}, &telebot.ChatMember{
							User: &telebot.User{ID: userID},
							Rights: telebot.Rights{
								CanSendMessages: true, // Full permission to send messages
								CanSendMedia:    true, // Full permission to send media files
								CanSendOther:    true, // Full permission to send other messages
							},
						})
						if err != nil {
							log.Println("Bot handler log:(ListenForstorage_dbChanges) - Listen for storage_db changes handler")
							log.Printf("Bot handler log:(ListenForstorage_dbChanges) - Failed to restrict user @%s (ID: %d): %s", data.Username, userID, err)
							continue
						}
					}
					 
					if !userIsAdminGroup {
						bot.Send(&telebot.User{ID: userID}, "You have successfully passed verification and can stay in the group.")

						// Delete the verification message
						storage_db.DeleteVerifyMessage(bot, userID)
						log.Println("Bot handler log:(ListenForstorage_dbChanges) - Verification message deleted for user:", userID)
					}

					if userIsAdminGroup {
						activeParams, err := storage_db.GetActiveVerificationParams(groupChatID)
						if err != nil {
							log.Printf("Bot handler log:(ListenForstorage_dbChanges) - Error getting active verification parameters: %v", err)
							return
						}
						
						// Combine active parameter with type restriction
						result := map[string]interface{}{
							"activeVerificationParam": activeParams,
							"typeRestriction":         typeRestriction,
						}

						formattedResult, err := json.MarshalIndent(result, "", "  ")
						if err != nil {
							log.Printf("Bot handler log:(ListenForstorage_dbChanges) - Failed to format result: %v", err)
							return
						}

						// Get the user's token
						tokenStr, errGettingToken := GetAuthTokenFromAdmin(groupChatID, userID)
						if !errGettingToken {
							log.Printf("Bot handler log:(ListenForstorage_dbChanges) - Failed to get token for user %d", userID)
							return
						}

						// Create txt file for write token
						fileName := fmt.Sprintf("token_%d.txt", userID)
						err = os.WriteFile(fileName, []byte(tokenStr), 0644)
						if err != nil {
							log.Printf("Bot handler log:(ListenForstorage_dbChanges) - Error writing AuthToken to file: %v", err)
							bot.Send(&telebot.User{ID: userID}, "Failed to create file with AuthToken.")
						}

						defer os.Remove(fileName) // Remove the file after sending

						bot.Send(
							&telebot.User{ID: userID},
							fmt.Sprintf("Here is the current verification parameter being tested:\n```\n%s\n```\n", string(formattedResult)),
							&telebot.SendOptions{ParseMode: telebot.ModeMarkdown},
						)

						time.Sleep(1*time.Second)

						// Send the file to the chat
						file := &telebot.Document{
							File:     telebot.FromDisk(fileName),
							FileName: fileName,
						}

						if _, err := bot.Send(&telebot.User{ID: userID}, file); err != nil {
							log.Printf("Bot handler log:(ListenForstorage_dbChanges) - Error sending file: %v", err)
						} else {
							// Remove the file after successfully sending it
							if err := os.Remove(fileName); err != nil {
								log.Printf("Bot handler log:(ListenForstorage_dbChanges) - Error deleting file: %v", err)
							}
						}

						time.Sleep(500*time.Millisecond)

						bot.Send(&telebot.User{ID: userID}, "The test was successful. The parameters are configured correctly, the verification process is working.")
						storage_db.DeleteUser(userID)
						storage_db.RemoveVerifiedUser(groupChatID, userID)
					}
				} else {
					// Verification failed
					log.Printf("Bot handler log:(ListenForstorage_dbChanges) - User @%s (ID: %d) failed verification. Removing from group.", data.Username, userID)
					group := &telebot.Chat{ID: data.GroupID}
					user := &telebot.User{ID: userID}
					bot.Ban(group, &telebot.ChatMember{User: user})
					time.Sleep(1 * time.Second)
					bot.Unban(group, user)
					bot.Send(user, "You failed verification and were removed from the group.")
				}
			}
		}
	}()
}

func UnifiedHandler(bot *telebot.Bot) func(c telebot.Context) error {
    return func(c telebot.Context) error {
        userID := c.Sender().ID
        chatType := c.Chat().Type

        switch chatType {
        case telebot.ChatGroup, telebot.ChatSuperGroup:
            return handleGroupMessage(bot, c, userID)

        case telebot.ChatPrivate:
            return handlePrivateMessage(bot, c)

        default:
            log.Printf("Bot handler log:(UnifiedHandler) - Unhandled chat type: %s", chatType)
            return nil
        }
    }
}

// Handle group messages
func handleGroupMessage(bot *telebot.Bot, c telebot.Context, userID int64) error {
	chatGroupId := c.Chat().ID
	typeRestriction, err := storage_db.GetRestrictionType(chatGroupId)
	if err != nil {
		log.Printf("Bot handler log:(handleGroupMessage) - Error getting restriction type: %v", err)
		return err
	}

	if typeRestriction == "delete" {
		log.Println("Bot handler log:(handleGroupMessage) - Handle group message, if type == delete")
		userData, err := storage_db.GetUser(userID)
		if err != nil || userData.IsPending {
			// Delete the user's message
			if err := bot.Delete(c.Message()); err != nil {
				log.Printf("Bot handler log:(handleGroupMessage) - Failed to delete message from @%s (ID: %d): %v", c.Sender().Username, userID, err)
			} else {
				log.Printf("Bot handler log:(handleGroupMessage) - Message from @%s (ID: %d) deleted (user awaiting verification).", c.Sender().Username, userID)
			}
		}
		return nil
	}

    return nil
}

func handlePrivateMessage(bot *telebot.Bot, c telebot.Context) error {
	userID := c.Sender().ID

	//groupChatID := storage_db.GroupSetupState[userID]
	groupChatID, err := storage_db.GetIdGroupFromGroupSetupState(userID)
	if groupChatID == 0 || err != nil {
		log.Println("Bot handler log:(handlePrivateMessage) - Group not set up for user:", userID)
		return nil
	}

	groupChat, _ := bot.ChatByID(groupChatID)
	if groupChat == nil {
		log.Printf("Bot handler log:(handlePrivateMessage) - Failed to fetch group chat by ID: %d", groupChatID)
		return nil
	}

	groupChatName := groupChat.Title
	if groupChatName == "" {
		log.Printf("Bot handler log:(handlePrivateMessage) - Failed to fetch group chat name by ID: %d", groupChatID)
		return nil
	}

    var params storage_db.VerificationParams

    // Parse JSON from the admin's message
	if err := json.Unmarshal([]byte(c.Text()), &params); err != nil {
		log.Printf("Bot handler log:(handlePrivateMessage) - Failed to parse JSON: %v", err)
		bot.Send(c.Sender(), "Invalid JSON format. Please ensure your parameters match the expected structure.")
		return nil
	}

    // Validate required fields in parsed JSON
	if params.CircuitID == "" || params.ID == 0 || params.Query == nil {
		log.Println("Bot handler log:(handlePrivateMessage) - JSON does not contain all required fields.")
		bot.Send(c.Sender(), "Missing required fields in JSON. Please include 'circuitId', 'id', and 'query'.")
		return nil
	}

	// // Save parameters to storage_db
	storage_db.SaveVerificationParams(groupChatID, params)
	log.Printf("Bot handler log:(handlePrivateMessage) - Verification parameters set for group '%s': %+v", groupChatName, params)
	bot.Send(c.Sender(), "JSON verification parameters have been add for the group.")

	// Send a message depending on the number of parameters
	restrictionType, _ := storage_db.GetRestrictionType(groupChatID)
	groupConfig, _ := storage_db.GetGroupConfigParams(groupChatID)
	if restrictionType == "" {
		time.Sleep(200*time.Millisecond)

		// bot.Send(c.Sender(), "To set restriction parameters for new subscribers, call the command\n /add_type_restriction")
		bot.Send(c.Sender(), "To set restriction parameters for new subscribers")

		// Immediately ask for restriction type
        return AddRestrictionTypeFunc(bot, c, groupChatID, groupChatName, len(groupConfig.VerificationParams) == 1)
	} else {
		bot.Send(c.Sender(), "Another verification parameter has been added.")
	}
	
	return nil
		
}

// Unified logic to set restriction type add_type_restriction_func
func AddRestrictionTypeFunc(bot *telebot.Bot, c telebot.Context, groupChatID int64, groupChatName string, isFirstParameter bool) error {
    // Create buttons ''Block'' and ''Delete''
    btnBlock := telebot.InlineButton{
        Text:   "Block",
        Unique: "block",
    }
    btnDelete := telebot.InlineButton{
        Text:   "Delete",
        Unique: "delete",
    }
    // Create a keyboard with buttons
    inlineKeys := [][]telebot.InlineButton{{btnBlock, btnDelete}}
    keyboard := &telebot.ReplyMarkup{InlineKeyboard: inlineKeys}

    if _, err := bot.Send(c.Sender(), "Select restriction type:", keyboard); err != nil {
        log.Printf("Bot handler log:(AddRestrictionTypeFunc) - Error sending keyboard: %v", err)
        return err
    }

    bot.Handle(&btnBlock, func(c telebot.Context) error {
        storage_db.AddRestrictionType(groupChatID, "block")
        c.Send("Restriction type set to 'block'.")

		groupConfig, _ := storage_db.GetGroupConfigParams(groupChatID)
		// Logs paprams for the group
		log.Println("Bot handler log:(AddRestrictionTypeFunc, TR: block) - Function add restriction type")
		log.Println("Bot handler log:(AddRestrictionTypeFunc, TR: block) - Group params:", groupConfig)

        // Send a success message
        if isFirstParameter {
            c.Send(fmt.Sprintf("Verification parameters have been successfully set for the group '%s'.", groupChatName))
        } else {
            c.Send("Another verification parameter has been added.")
        }
        return nil
    })

    bot.Handle(&btnDelete, func(c telebot.Context) error {
        storage_db.AddRestrictionType(groupChatID, "delete")
        c.Send("Restriction type set to 'delete'.")

		groupConfig, _ := storage_db.GetGroupConfigParams(groupChatID)

		// Logs paprams for the group
		log.Println("Bot handler log:(AddRestrictionTypeFunc, TR: delete) - Function add restriction type")
		log.Println("Bot handler log:(AddRestrictionTypeFunc, TR: delete) - Group params:", groupConfig)

        // Send a success message
        if isFirstParameter {
            c.Send(fmt.Sprintf("Verification parameters have been successfully set for the group '%s'.", groupChatName))
        } else {
            c.Send("Another verification parameter has been added.")
        }
        return nil
    })

    return nil
}

// Handler for /set_type_restriction
func SetTypeRestrictionHandler(bot *telebot.Bot) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		userID := c.Sender().ID

		// Check if the group is set up for this user
		targetChatGroupID, err := storage_db.GetIdGroupFromGroupSetupState(userID)
		if err != nil {
			log.Println("Bot handler log:(SetTypeRestrictionHandler) - Bot handlers logs: Group not set up for user:", userID)
			return c.Send("You need to specify a group for restriction setup.")
		}

		// Get the group chat by ID
		chat, err := bot.ChatByID(targetChatGroupID)
		if err != nil {
			log.Printf("Bot handler log:(SetTypeRestrictionHandler) - Error fetching chat: %v", err)
			return c.Send("Failed to fetch chat information.")
		}
		groupChatName := chat.Title

		// Check if the user is an administrator of the group
		if !isAdmin(bot, targetChatGroupID, userID) {
			return c.Send("You are not an administrator in this group.")
		}

		// Fetch current restriction type
		currentRestriction, _ := storage_db.GetRestrictionType(targetChatGroupID)
		if currentRestriction == "" {
			currentRestriction = "Not set"
		}

		// Update button text based on the current restriction
		blockText := "Block"
		deleteText := "Delete"
		if currentRestriction == "block" {
			blockText += " (active)"
		} else if currentRestriction == "delete" {
			deleteText += " (active)"
		}

		// Create buttons
		btnBlock := telebot.InlineButton{
			Text:   blockText,
			Unique: fmt.Sprintf("change_block_%d", targetChatGroupID), // Unique ID per group
		}
		btnDelete := telebot.InlineButton{
			Text:   deleteText,
			Unique: fmt.Sprintf("change_delete_%d", targetChatGroupID), // Unique ID per group
		}

		// Create a keyboard with buttons
		inlineKeys := [][]telebot.InlineButton{{btnBlock, btnDelete}}
		keyboard := &telebot.ReplyMarkup{InlineKeyboard: inlineKeys}

		// Send current restriction type and options to change it
		if _, err := bot.Send(c.Sender(), fmt.Sprintf(
			"Current restriction type for the group '%s': %s.\n\nSelect a new restriction type:",
			groupChatName, currentRestriction), keyboard); err != nil {
			log.Printf("Bot handler log:(SetTypeRestrictionHandler) - Error sending keyboard: %v", err)
			return err
		}

		// Define button handlers
		bot.Handle(&btnBlock, func(c telebot.Context) error {
			// Check if the current restriction type is already "block"
			if currentRestriction == "block" {
				_, err := bot.Send(c.Sender(), "The restriction type is already set to 'Block'.")
				return	err
			}

			// Update the restriction type
			storage_db.AddRestrictionType(targetChatGroupID, "block")

			// Get config verification params for the group
			groupConfig, err := storage_db.GetGroupConfigParams(targetChatGroupID)
			if err != nil {
				log.Printf("Bot handler log:(SetTypeRestrictionHandler) - Bot handlers log: Error fetching group configuration: %v", err)
				return c.Send("Failed to fetch group configuration.")
			}
			
			// Logs paprams for the group durin change restriction type
			log.Println("Bot handler log:(SetTypeRestrictionHandler) - Function set restriction type")
			log.Println("Bot handler log:(SetTypeRestrictionHandler) - Group params:", groupConfig)

			// Send a confirmation message without deleting or editing the keyboard message
			_, err = bot.Send(c.Sender(), fmt.Sprintf("Restriction type for group '%s' has been changed to 'block'.", groupChatName))
			return err
		})

		bot.Handle(&btnDelete, func(c telebot.Context) error {
			// Check if the current restriction type is already "delete"
			if currentRestriction == "delete" {
				_, err := bot.Send(c.Sender(), "The restriction type is already set to 'Block'.")
				return err
			}

			// Update the restriction type
			storage_db.AddRestrictionType(targetChatGroupID, "delete")

			// Get config verification params for the group
			groupConfig, err := storage_db.GetGroupConfigParams(targetChatGroupID)
			if err != nil {
				log.Printf("Bot handlers log: Error fetching group configuration: %v", err)
				return c.Send("Failed to fetch group configuration.")
			}

			// Logs paprams for the group durin change restriction type
			log.Println("Bot handler log:(SetTypeRestrictionHandler) - Function set restriction type")
			log.Println("Bot handler log:(SetTypeRestrictionHandler) - Group params:", groupConfig)

			// Send a confirmation message without deleting or editing the keyboard message
			_, err = bot.Send(c.Sender(), fmt.Sprintf("Restriction type for group '%s' has been changed to 'delete'.", groupChatName))
			return err
		})

		return nil
	}
}
 
// Handler for /test_verification
func TestVerificationHandler(bot *telebot.Bot) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		userID := c.Sender().ID
		var groupChatID int64

		// Determine where the handler was called: in a group or in a private chat
		if c.Chat().Type == telebot.ChatPrivate {
			// Check if there is a saved group for this administrator
			groupID, _ := storage_db.GetIdGroupFromGroupSetupState(userID)
			if groupID != 0 {
				groupChatID = groupID
			} else {
				return c.Send("You need to specify a group for verification setup.")
			}
		} else {
			groupChatID = c.Chat().ID
		}

		log.Println("Bot handler log:(TestVerificationHandler) - Group Chat ID:", groupChatID)

		// Check if the user is an administrator of the group
		if !isAdmin(bot, groupChatID, userID) {
			return c.Send("You are not an administrator in this group.")
		}

		// Create a record for the admin's test verification
		adminUser := &storage_db.UserVerification{
			UserID:         userID,
			Username:       c.Sender().Username,
			GroupID:        groupChatID,
			GroupName:      c.Chat().Title,
			IsPending:      true,
			Verified:       false,
			SessionID:      0,
			RestrictStatus: false,
			Role : 			"admin",
		}

		storage_db.AddOrUpdateUser(userID, adminUser)

		groupConfig, err := storage_db.GetGroupConfigParams(groupChatID)
		if err != nil {
			log.Printf("Bot handler log:(TestVerificationHandler) - Bot handlers log: Error fetching group configuration: %v", err)
			return c.Send("Verification parameters are not configured for your group.")
		}

		// Check that the active index is valid
		if groupConfig.ActiveIndex < 0 || groupConfig.ActiveIndex >= len(groupConfig.VerificationParams) {
			log.Printf("Bot handler log:(TestVerificationHandler) - Invalid active index for group ID: %d", groupChatID)
			return c.Send("Verification configuration error. Please contact the group administrator.")
		}

		// Get active verification parameters
		params := groupConfig.VerificationParams[groupConfig.ActiveIndex]

		// Determine the type of current verification
		verificationType := "unknown"
		if queryType, ok := params.Query["type"].(string); ok {
			verificationType = queryType
		}

		// Generate a test request for verification
		jsonData, err := auth.GenerateAuthRequest(userID, params)
		if err != nil {
			log.Printf("Bot handler log:(TestVerificationHandler) - Error generating auth request: %v", err)
			return c.Send("Failed to generate verification request. Please try again later.")
		}

		base64Data := base64.StdEncoding.EncodeToString(jsonData)
		deepLink := fmt.Sprintf("https://wallet.privado.id/#i_m=%s", base64Data)

		btn := telebot.InlineButton{
			Text: fmt.Sprintf("Test verify (%s)", verificationType),
			URL:  deepLink,
		}

		// Creating markup with a button
		inlineKeyboard := &telebot.ReplyMarkup{}
		inlineKeyboard.InlineKeyboard = [][]telebot.InlineButton{{btn}}

		// Send a message with a link for test verification
		_, err = bot.Send(c.Sender(), "Please test verify your age by clicking the link below:", inlineKeyboard)
		if err != nil {
			log.Printf("Bot handler log:(TestVerificationHandler) - Error sending verification message: %v", err)
			return c.Send("Failed to send verification link. Please check your private messages.")
		}

		if c.Chat().Type == telebot.ChatSuperGroup || c.Chat().Type == telebot.ChatGroup {
			msg, err := bot.Send(c.Chat(), "A verification link has been sent to your private messages. Please check your inbox.")
			//msg, err := c.Send("A verification link has been sent to your private messages. Please check your inbox.")
			if err != nil {
				log.Printf("Error sending group message: %v", err)
				return err
			}

			// Schedule deletion of the message after 1 minute
			go func() {
				time.Sleep(1 * time.Minute)
				if err := bot.Delete(msg); err != nil {
					log.Printf("Error deleting message: %v", err)
				}
			}()
		}

		return nil
	}
}

// heandler for /verified_users_list
func VerifiedUsersListHeandler(bot *telebot.Bot) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		userID := c.Sender().ID

		targetChatGroupID, err := storage_db.GetIdGroupFromGroupSetupState(userID)
		if err != nil {
			log.Println("Bot handler log:(VerifiedUsersListHeandler) - Bot handlers log: Group not set up for user:", userID)
			return c.Send("You need to set up a group for verification.")
		}

		// Get chat data
		chat, err := bot.ChatByID(targetChatGroupID)
		if err != nil {
			log.Printf("Bot handler log:(VerifiedUsersListHeandler) - Error fetching chat: %v", err)
			return c.Send("Failed to fetch chat information.")
		}
		targetChatGroupName := chat.Title

		verifiedUsers, err := storage_db.GetVerifiedUsersList(targetChatGroupID)
		if err != nil {
			log.Printf("Bot handler log:(VerifiedUsersListHeandler) - Bot handlers log: Error fetching verified users list: %v", err)
		}

		// If the list for the group is empty or the group does not exist
		if err !=nil || len(verifiedUsers) == 0 {
			return c.Send(fmt.Sprintf("No verified users in the group '%s'.", targetChatGroupName))
		}
		
		// Forming a message with a list of verified users
		msg := fmt.Sprintf("Verified users in the group '%s':\n\n", targetChatGroupName)
		for _, verifiedUser := range verifiedUsers {
			// Combine all verification types into a comma-separated string
			types := strings.Join(verifiedUser.TypesVerification, ", ")
			msg += fmt.Sprintf("@%s - %s\n", verifiedUser.User.UserName, types)
		}

		// Send a message
		return c.Send(msg)
	}
}

// Handler for delering all verified users /delete_all_verified_users for the group
func DeleteAllVerifiedUsersHandler(bot *telebot.Bot) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		userID := c.Sender().ID

		targetChatGroupID, err := storage_db.GetIdGroupFromGroupSetupState(userID)
		if err != nil {
			log.Println("Bot handler log:(DeleteAllVerifiedUsersHandler) - Bot handlers log: Group not set up for user:", userID)
			return c.Send("You need to set up a group for verification.")
		}

		// Get chat data
		chat, err := bot.ChatByID(targetChatGroupID)
		if err != nil {
			log.Printf("Bot handler log:(DeleteAllVerifiedUsersHandler) - Error fetching chat: %v", err)
			return c.Send("Failed to fetch chat information.")
		}
		targetChatGroupName := chat.Title

		verifiedUsers, err := storage_db.GetVerifiedUsersList(targetChatGroupID)
		if err != nil {
			log.Printf("Bot handler log:(DeleteAllVerifiedUsersHandler) - Bot handlers log: Error fetching verified users list: %v", err)
		}

		// If the list for the group is empty or the group does not exist
		if err != nil || len(verifiedUsers) == 0 {
			return c.Send(fmt.Sprintf("No verified users in the group '%s'.", targetChatGroupName))
		}

		storage_db.DeleteAllVerifiedUsers(targetChatGroupID)

		return c.Send(fmt.Sprintf("All verified users have been deleted for the group '%s'.", targetChatGroupName))
	}
}

// AddVerificationParamsHandler handles adding verification parameters in a single step /add_verification_params
func AddVerificationParamsHandler(bot *telebot.Bot) func(c telebot.Context) error {
    return func(c telebot.Context) error {
        userID := c.Sender().ID

		groupChatID, err := storage_db.GetIdGroupFromGroupSetupState(userID)
		if err != nil {
			log.Println("Bot handler log:(AddVerificationParamsHandler) - Bot handlers log: Group not set up for user:", userID)
			return c.Send("You are not associated with any group. Use /setup first.")
		}

        groupChat, _ := bot.ChatByID(groupChatID)
        if groupChat == nil {
            log.Printf("Bot handler log:(AddVerificationParamsHandler) - Failed to fetch group chat by ID: %d", groupChatID)
            return c.Send("Failed to fetch the group chat. Please try again.")
        }

        // Request verification parameters
        exampleJSON := "{\n" +
            "  \"circuitId\": \"AtomicQuerySigV2CircuitID\",\n" +
            "  \"id\": 1,\n" +
            "  \"query\": {\n" +
            "    \"allowedIssuers\": [\"*\"],\n" +
            "    \"context\": \"https://example.com/context\",\n" +
            "    \"type\": \"ExampleType\",\n" +
            "    \"credentialSubject\": {\n" +
            "      \"birthday\": {\"$lt\": 20000101}\n" +
            "    }\n" +
            "  }\n" +
            "}"
        return c.Send("Please send verification parameters in JSON format. Example:\n\n" + exampleJSON)
    }
}

// Handler deleting all verification parameters for a group /delete_all_verification_params
func DeleteAllVerificationParamsHandler(bot *telebot.Bot) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		userID := c.Sender().ID

		groupChatID, err := storage_db.GetIdGroupFromGroupSetupState(userID)
		if err != nil {
			log.Println("Bot handler log:(DeleteAllVerificationParamsHandler) - Bot handlers log: Bot handlers log: Group not set up for user:", userID)
			return c.Send("You are not associated with any group. Use /setup first.")
		}

		groupChat, _ := bot.ChatByID(groupChatID)
		if groupChat == nil {
			log.Printf("Bot handler log:(DeleteAllVerificationParamsHandler) - Failed to fetch group chat by ID: %d", groupChatID)
			return c.Send("Failed to fetch the group chat. Please try again.")
		}

		storage_db.DeleteAllVerificationParams(groupChatID)

		// Notify the user
		return c.Send("All verification parameters have been successfully cleared for this group.")
	}
}


// ListVerificationParamsHandler displays the list of added verification parameters /list_verification_params
func ListVerificationParamsHandler(bot *telebot.Bot) func(c telebot.Context) error {
    return func(c telebot.Context) error {
        userID := c.Sender().ID

		groupChatID, err := storage_db.GetIdGroupFromGroupSetupState(userID)
		if err != nil {
			log.Println("Bot handler log:(ListVerificationParamsHandler) - Group not set up for user:", userID)
			return c.Send("You are not associated with any group. Use /setup first.")
		}

		groupConfig, err := storage_db.GetGroupConfigParams(groupChatID)
		if err != nil || len(groupConfig.VerificationParams) == 0 {
			log.Println("Bot handler log:(ListVerificationParamsHandler) - No verification parameters found for group:", groupChatID)
			return c.Send("No verification parameters have been added yet. Use /add_verification_params to add one.")
		}

        // Fetch restriction type
        restrictionType := groupConfig.RestrictionType
        if restrictionType == "" {
            restrictionType = "Not set"
        }

        // Build the response
        var response strings.Builder
        response.WriteString("*Verification parameters for the group:*\n\n")
        for i, param := range groupConfig.VerificationParams {
            activeMarker := ""
            if i == groupConfig.ActiveIndex {
                activeMarker = " (active)"
            }

            // Extract the "type" field from the "query"
            queryType := "unknown" // Default value in case "type" is missing
            if query, ok := param.Query["type"]; ok {
                if queryStr, ok := query.(string); ok {
                    queryType = queryStr
                }
            }

            // Add the type header
            response.WriteString(fmt.Sprintf("*Type:* `%s`%s\n\n", queryType, activeMarker))

            // Convert the parameter to JSON with indentation
            formattedJSON, err := json.MarshalIndent(param, "", "    ")
            if err != nil {
                log.Printf("Bot handler log:(ListVerificationParamsHandler) - Failed to format JSON for param %d: %v", i+1, err)
                response.WriteString("Error formatting JSON\n\n")
                continue
            }

            // Add the JSON as a code block
            response.WriteString("```\n")
            response.WriteString(string(formattedJSON))
            response.WriteString("\n```\n\n")
        }

        // Add the restriction type information
        response.WriteString("*Restriction type for new members:* ")
        response.WriteString(fmt.Sprintf("`%s`", restrictionType))

        // Send the list to the admin
        return c.Send(response.String(), telebot.ModeMarkdown)
    }
}

// Handler to switch active verification parameters /set_active_verification_params
func SetActiveVerificationParamsHandler(bot *telebot.Bot) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		userID := c.Sender().ID
		var groupChatID int64

		// Determine where the handler was called: in a group or in a private chat
		if c.Chat().Type == telebot.ChatPrivate {
			groupID, err := storage_db.GetIdGroupFromGroupSetupState(userID)
			if err != nil {
				log.Println("Bot handler log:(SetActiveVerificationParamsHandler) - Group not set up for user:", userID)
				return c.Send("You need to specify a group for verification setup.")
			} else if groupID == 0 {
				log.Println("Bot handler log:(SetActiveVerificationParamsHandler) - Id group = 0. Group not set up for user:", userID)
				return c.Send("You need to specify a group for verification setup.")
			} else {
				groupChatID = groupID
			}
		} else {
			groupChatID = c.Chat().ID
		}

		// Check if the user is an administrator of the group
		if !isAdmin(bot, groupChatID, userID) {
			return c.Send("You are not an administrator in this group.")
		}

		groupConfig, err := storage_db.GetGroupConfigParams(groupChatID)
		if err != nil {
			log.Printf("Bot handler log:(SetActiveVerificationParamsHandler) - Error fetching group configuration: %v", err)
			return c.Send("No verification parameters have been set for this group. Error fetching group configuration")
		} else if len(groupConfig.VerificationParams) == 0 {
			return c.Send("No verification parameters have been set for this group.")
		}

		// If there is only one verification parameter, notify the admin
		if len(groupConfig.VerificationParams) == 1 {
			return c.Send("Only one verification type is available. Switching is not possible.")
		}

		// Generate buttons for all verification types
		inlineKeyboard := &telebot.ReplyMarkup{}
		for i, param := range groupConfig.VerificationParams {
			text := fmt.Sprintf("%d. %s", i+1, param.Query["type"])
			if i == groupConfig.ActiveIndex {
				text += " (active)"
			}

			btn := telebot.InlineButton{
				Text:   text,
				Unique: fmt.Sprintf("switch_param_%d", i),
			}
			// Register callback handler for the button
			bot.Handle(&btn, func(c telebot.Context) error {
				// Handle button click
				index := i

				// Validate the index
				if index < 0 || index >= len(groupConfig.VerificationParams) {
					return c.Respond(&telebot.CallbackResponse{
						Text: "Invalid selection.",
					})
				}

				// If the selected index is already active, notify the admin
				if groupConfig.ActiveIndex == index {
					_, err := bot.Send(c.Sender(), fmt.Sprintf(
						"The selected verification type '%s' is already active.",
						groupConfig.VerificationParams[index].Query["type"],
					))
					return err
				}

				storage_db.SetActiveVerificationParams(groupChatID, index)

				// Notify the admin of the change
				typeStr, ok := groupConfig.VerificationParams[index].Query["type"].(string)
				if !ok {
					typeStr = "Unknown type"
				}

				bot.Send(c.Sender(), fmt.Sprintf("Verification type '%s' has been set as active.", typeStr))

				// Respond to the callback to clear the loading state on the button
				return c.Respond()
			})

			inlineKeyboard.InlineKeyboard = append(inlineKeyboard.InlineKeyboard, []telebot.InlineButton{btn})
		}

		// Send the list of options to the admin
		return c.Send("Select the verification type to activate:", inlineKeyboard)
	}
}


func checkUserAsAdminInGroup(userID, groupID int64) bool {
	groupIdByUser, err := storage_db.GetIdGroupFromGroupSetupState(userID)
	if err != nil {
		log.Println("Bot handler log:(checkUserAsAdminInGroup) - Group not set up for user:", userID)
		return false
	}
	if groupIdByUser == groupID {
		return true
	} else {
		return false
	}
}

func GetAuthTokenFromAdmin(groupID int64, userID int64) (string, bool) {
	DataMutex.Lock()
	defer DataMutex.Unlock()

	users, err := storage_db.GetVerifiedUsersList(groupID)
	if err != nil {
		return "", false // Group not found
	}

	// Search for the user by userID
	for _, verifiedUser := range users {
		if verifiedUser.User.ID == userID {
			return verifiedUser.AuthToken, true // Token found
		}
	}

	return "", false // User not found
}
