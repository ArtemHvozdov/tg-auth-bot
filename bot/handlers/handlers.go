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
	"github.com/ArtemHvozdov/tg-auth-bot/storage"

	"time"

	"gopkg.in/telebot.v3"
)

var DataMutex sync.Mutex

// isAdmin checks if the user is a group admin
func isAdmin(bot *telebot.Bot, chatID int64, userID int64) bool {
	member, err := bot.ChatMemberOf(&telebot.Chat{ID: chatID}, &telebot.User{ID: userID})
	if err != nil {
		log.Printf("Error fetching user role: %v", err)
		return false
	}
	return member.Role == "administrator" || member.Role == "creator"
}

// Handler for /start
func StartHandler(bot *telebot.Bot) func(c telebot.Context) error {
    return func(c telebot.Context) error {
        userName := c.Sender().Username
        // if c.Sender().LastName != "" {
        //     userName += " " + c.Sender().Username
        // }

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
		// Шаг 1: Отправляем сообщение о необходимости добавить бота в группу с правами администратора
		msg := "To set me up for verification in your group, please add me to the group as an administrator and call the /check_admin command in the group."
		if err := c.Send(msg); err != nil {
			log.Printf("Error sending setup message: %v", err)
			return err
		}
		return nil
	}
}

// /check_admin command - check administrator rights in a group and verify verification parameters
func CheckAdminHandler(bot *telebot.Bot) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		chatID := c.Chat().ID // ID group chat (future: need rename this variable to groupID)
		userID := c.Sender().ID // ID admin user (future: need rename this variable to adminID)
		chatName := c.Chat().Title // Getting the name of the chat (group)
		userName := c.Sender().Username // Username

		storage.AddAdminUser(userID, chatID)

		log.Printf("User ID: %d, Chat ID: %d, Command received", userID, chatID)
		log.Printf("User's name: %s %s (@%s)", c.Sender().FirstName, c.Sender().LastName, c.Sender().Username)

		// // Schedule deletion of the message after 1 minute
		// go func() {
		// 	time.Sleep(1 * time.Minute)
		// 	if err := bot.Delete(msg); err != nil {
		// 		log.Printf("Error deleting message: %v", err)
		// 	}
		// }()

		msgContinueForAdmin, _ := bot.Send(&telebot.Chat{ID: chatID}, "Administrator, return to the private chat with me to continue configuring the settings")

		go func() {
			time.Sleep(1 * time.Minute)
			if err := bot.Delete(msgContinueForAdmin); err != nil {
				log.Printf("Error deleting continue for admins message: %v", err)
			}
		}()

		// Checking if the bot is an administrator in this group
		member, err := bot.ChatMemberOf(&telebot.Chat{ID: chatID}, &telebot.User{ID: bot.Me.ID})
		if err != nil {
			log.Printf("Error fetching bot's role in the group: %v", err)
			// Send a private message to the user
			msg := "I couldn't fetch my role in this group. Please make sure I am an administrator."
			if _, err := bot.Send(&telebot.User{ID: userID}, msg); err != nil {
				log.Printf("Error sending bot admin check message: %v", err)
				return err
			}
			return nil
		}

		// Logging the bot's role
		log.Printf("Bot's role in the group '%s': %s", chatName, member.Role)

		// Checking if the bot is an administrator
		if member.Role != "administrator" && member.Role != "creator" {
			msg := fmt.Sprintf("I am not an administrator in the group '%s'. Please promote me to an administrator.", chatName)
			if _, err := bot.Send(&telebot.User{ID: userID}, msg); err != nil {
				log.Printf("Error sending bot admin check message: %v", err)
				return err
			}
			return nil
		}

		// Checking if the user the bot is interacting with is an administrator
		memberUser, err := bot.ChatMemberOf(&telebot.Chat{ID: chatID}, &telebot.User{ID: userID})
		if err != nil {
			log.Printf("Error fetching user's role: %v", err)
			// Send a private message to the user
			msg := "I couldn't fetch your role in this group."
			if _, err := bot.Send(&telebot.User{ID: userID}, msg); err != nil {
				log.Printf("Error sending user admin check message: %v", err)
				return err
			}
			return nil
		}

		// Logging the user role
		log.Printf("User's role in the group '%s': %s", chatName, memberUser.Role)

		// Checking if the user is an administrator
		if memberUser.Role != "administrator" && memberUser.Role != "creator" {
			// We inform the user that he is not an administrator
			groupMsg := fmt.Sprintf("@%s, you are not an administrator in the group '%s'. You cannot configure me for this group.", userName, chatName)
			if _, err := bot.Send(&telebot.Chat{ID: chatID}, groupMsg); err != nil {
				log.Printf("Error sending message to group: %v", err)
				return err
			}
			return nil
		}

		// All checks were successful
		msg := fmt.Sprintf("I have confirmed your admin status and my role in the group '%s'. You can now proceed with the setup.", chatName)
		if _, err := bot.Send(&telebot.User{ID: userID}, msg); err != nil {
			log.Printf("Error sending success message to user: %v", err)
			return err
		}

		// Save parameters for restriction

		// Create buttons ''Block'' and ''Delete''
		btnBlock := telebot.InlineButton{
			Text: "Block",
			Unique: "block",
		}
		btnDelete := telebot.InlineButton{
			Text: "Delete",
			Unique: "delete",
		}
		// Create a keyboard with buttons
		inlineKeys := [][]telebot.InlineButton{{btnBlock, btnDelete}}
		keyboard := &telebot.ReplyMarkup{InlineKeyboard: inlineKeys}

		
		if _, err := bot.Send(&telebot.User{ID: userID}, "Select restriction type:", keyboard); err != nil {
			log.Printf("Error sending keyboard: %v", err)
			return err
		}

		bot.Handle(&btnBlock, func(c telebot.Context) error {
			storage.AddRestrictionType(chatID, "block")
			bot.Send(&telebot.User{ID: userID}, "Restriction type set to 'block'.")

			log.Println(storage.RestrictionType[chatID])

			time.Sleep(500*time.Millisecond)
			// Ask for verification parameters
			bot.Send(&telebot.User{ID: userID}, "To add verification parameters, call the command\n /add_verification_params")
			//askVerificationParams(bot, userID)
			return nil
		})

		bot.Handle(&btnDelete, func(c telebot.Context) error {
			storage.AddRestrictionType(chatID, "delete")
			bot.Send(&telebot.User{ID: userID}, "Restriction type set to 'delete'.")

			time.Sleep(500*time.Millisecond)
			// Ask for verification parameters
			bot.Send(&telebot.User{ID: userID}, "To add verification parameters, call the command /add_verification_params")
			//askVerificationParams(bot, userID)
			return nil
		})

		return nil
	}
}

// askVerificationParams sends a request for verification parameters to the admin
// func askVerificationParams(bot *telebot.Bot, userID int64) {
// 	bot.Send(&telebot.User{ID: userID}, "Send verification parameters in JSON format in this private chat. Example:\n\n"+
// 		"{\n"+
// 		"  \"circuitId\": \"AtomicQuerySigV2CircuitID\",\n"+
// 		"  \"id\": 1,\n"+
// 		"  \"query\": {\n"+
// 		"    \"allowedIssuers\": [\"*\"],\n"+
// 		"    \"context\": \"https://example.com/context\",\n"+
// 		"    \"type\": \"ExampleType\",\n"+
// 		"    \"credentialSubject\": {\n"+
// 		"      \"birthday\": {\"$lt\": 20000101}\n"+
// 		"    }\n"+
// 		"  }\n"+
// 		"}",
// 	)
// }


// A new user has joined the group
func NewUserJoinedHandler(bot *telebot.Bot) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		for _, member := range c.Message().UsersJoined {
			if isAdmin(bot, c.Chat().ID, member.ID) {
				log.Printf("Skipping admin user @%s (ID: %d)", member.Username, member.ID)
				continue
			}

			// Adding a new user to the repository
			newUser := &storage.UserVerification{
				UserID:    member.ID,
				Username:  member.Username,
				GroupID:   c.Chat().ID,
				GroupName: c.Chat().Title,
				IsPending: true,
				Verified:  false,
				SessionID: 0,
				RestrictStatus: true,
			}

			storage.AddOrUpdateUser(member.ID, newUser)

			log.Println("New user:", newUser)

			// Restrict the user if the restriction type is "block"
			if storage.RestrictionType[c.Chat().ID] == "block" {
				err := bot.Restrict(c.Chat(), &telebot.ChatMember{
					User: &telebot.User{ID: member.ID},
					Rights: telebot.Rights{
						CanSendMessages: false, // Complete ban on sending messages
					},
				})
				if err != nil {
					log.Println("Nes user handler")
					log.Printf("Failed to restrict user @%s (ID: %d): %s", member.Username, member.ID, err)
					continue
				}
			}

			log.Println("Bot Logs: new member -", newUser)

			btn := telebot.InlineButton{
				Text: "Verify your age",
				URL:  fmt.Sprintf("https://t.me/%s", bot.Me.Username),
			}

			inlineKeys := [][]telebot.InlineButton{{btn}}
			log.Printf("New member @%s added to verification queue.", member.Username)

			msg, err := bot.Send(
				c.Chat(),
				fmt.Sprintf("Hi, @%s! Please verify your age by clicking the button below.", member.Username),
				&telebot.ReplyMarkup{InlineKeyboard: inlineKeys},
			)
			if err != nil {
				log.Printf("Error sending verification message: %v", err)
				return err
			}
			// Save the message ID for further deletion
			storage.UserStore[member.ID].AddVerificationMsg(msg.ID, msg)

			go handleVerificationTimeout(bot, member.ID, c.Chat().ID)
		}

		return nil
	}
}

// Handler /verify
func VerifyHandler(bot *telebot.Bot) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		userID := c.Sender().ID

		userData, exists := storage.GetUser(userID)
		if !exists || !userData.IsPending {
			log.Printf("User @%s (ID: %d) is not awaiting verification.", c.Sender().Username, userID)
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

		userGroupID := storage.UserStore[userID].GroupID

		// Получение конфигурации группы
		groupConfig, exists := storage.VerificationParamsMap[userGroupID]
		if !exists {
			log.Printf("Verification parameters not found for group ID: %d", userGroupID)
			return c.Send("Verification parameters are not configured for your group.")
		}

		// Проверяем, что активный индекс корректен
		if groupConfig.AcriveIndex < 0 || groupConfig.AcriveIndex >= len(groupConfig.VerificationParams) {
			log.Printf("Invalid active index for group ID: %d", userGroupID)
			return c.Send("Verification configuration error. Please contact the group administrator.")
		}

		// Получаем активные параметры верификации
		activeParams := groupConfig.VerificationParams[groupConfig.AcriveIndex]

		jsonData, _ := auth.GenerateAuthRequest(userID, activeParams)

		base64Data := base64.StdEncoding.EncodeToString(jsonData)

		// Create deeplink
		deepLink := fmt.Sprintf("https://wallet.privado.id/#i_m=%s", base64Data)

		// logs deeplinl
		log.Println("Deep Link:", deepLink)

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

	userData, exists := storage.GetUser(userID)
	if exists && userData.IsPending && !userData.Verified {
		log.Printf("User @%s (ID: %d) failed verification on time. Removing from group.", userData.Username, userID)
		bot.Ban(&telebot.Chat{ID: groupID}, &telebot.ChatMember{User: &telebot.User{ID: userID}})
		time.Sleep(1 * time.Second)
		bot.Unban(&telebot.Chat{ID: groupID}, &telebot.User{ID: userID})
		bot.Send(&telebot.User{ID: userID}, "You did not complete the verification on time and were removed from the group.")
		storage.DeleteUser(userID)
	}
}

// Store change listener
func ListenForStorageChanges(bot *telebot.Bot) {
	go func() {
		for event := range storage.DataChanges {
			userID := event.UserID
			data := event.Data

			if data == nil {
				// User was delete
				log.Printf("User ID: %d was removed from the store.", userID)
				continue
			}

			groupChatID := storage.UserStore[userID].GroupID
			typeRestriction := storage.RestrictionType[groupChatID]

			userIsAdminGroup := checkUserAsAdminInGroup(userID, groupChatID)

			if !data.IsPending {
				if data.Verified {
					// Successful verification
					log.Printf("User @%s (ID: %d) passed verification.", data.Username, userID)
					
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
							log.Println("Listen for storage changes handler")
							log.Printf("Failed to restrict user @%s (ID: %d): %s", storage.UserStore[userID].Username, userID, err)
							continue
						}
					}
					 
					if !userIsAdminGroup {
						bot.Send(&telebot.User{ID: userID}, "You have successfully passed verification and can stay in the group.")

						// Delete the verification message
						storage.UserStore[userID].DeleteVerifyMessage(bot)
						log.Println("Verification message deleted for user:", userID)
					}

					if userIsAdminGroup {
						params, exists := storage.VerificationParamsMap[groupChatID]
						if !exists {
							log.Printf("Verification parameters are not set for the group '%s'.", storage.UserStore[userID].GroupName)
							return
						}

						// Get the active verification parameter
						activeIndex := params.AcriveIndex
						if activeIndex < 0 || activeIndex >= len(params.VerificationParams) {
							log.Printf("Invalid active index for group '%s'.", storage.UserStore[userID].GroupName)
							bot.Send(&telebot.User{ID: userID}, "The active verification parameter is not set correctly.")
							return
						}

						activeParams := params.VerificationParams[activeIndex]
						// formattedParams, err := json.MarshalIndent(activeParams, "", "  ")
						// if err != nil {
						// 	log.Printf("Failed to format active verification parameter: %v", err)
						// 	return
						// }

						// Combine active parameter with type restriction
						result := map[string]interface{}{
							"activeVerificationParam": activeParams,
							"typeRestriction":         typeRestriction,
						}

						formattedResult, err := json.MarshalIndent(result, "", "  ")
						if err != nil {
							log.Printf("Failed to format result: %v", err)
							return
						}

						// Get the user's token
						tokenStr, errGettingToken := GetAuthTokenFromAdmin(groupChatID, userID)
						if !errGettingToken {
							log.Printf("Failed to get token for user %d", userID)
							return
						}

						// Create txt file for write token
						fileName := fmt.Sprintf("token_%d.txt", userID)
						err = os.WriteFile(fileName, []byte(tokenStr), 0644)
						if err != nil {
							log.Printf("Error writing AuthToken to file: %v", err)
							bot.Send(&telebot.User{ID: userID}, "Failed to create file with AuthToken.")
						}

						defer os.Remove(fileName) // Remove the file after sending

						//bot.Send(&telebot.User{ID: userID}, fmt.Sprintf("Here are the current verification parameters:\n```\n%s\n```\n Type restriction new members: %s.\n", string(formattedParams), typeRestriction), &telebot.SendOptions{ParseMode: telebot.ModeMarkdown})

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
							log.Printf("Error sending file: %v", err)
						} else {
							// Remove the file after successfully sending it
							if err := os.Remove(fileName); err != nil {
								log.Printf("Error deleting file: %v", err)
							}
						}

						time.Sleep(500*time.Millisecond)

						bot.Send(&telebot.User{ID: userID}, "The test was successful. The parameters are configured correctly, the verification process is working.")
						storage.DeleteUser(userID)
						storage.RemoveVerifiedUser(groupChatID, userID)
					}
				} else {
					// Verification failed
					log.Printf("User @%s (ID: %d) failed verification. Removing from group.", data.Username, userID)
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
            log.Printf("Unhandled chat type: %s", chatType)
            return nil
        }
    }
}

func handleGroupMessage(bot *telebot.Bot, c telebot.Context, userID int64) error {
	chatGroupId := c.Chat().ID
	typeRestriction := storage.RestrictionType[chatGroupId]

	if typeRestriction == "delete" {
		userData, exists := storage.GetUser(userID)
		if !exists || userData.IsPending {
			// Delete the user's message
			if err := bot.Delete(c.Message()); err != nil {
				log.Printf("Failed to delete message from @%s (ID: %d): %v", c.Sender().Username, userID, err)
			} else {
				log.Printf("Message from @%s (ID: %d) deleted (user awaiting verification).", c.Sender().Username, userID)
			}
		}
		return nil
	}

    return nil
}

func handlePrivateMessage(bot *telebot.Bot, c telebot.Context) error {
	userID := c.Sender().ID

	groupChatID := storage.GroupSetupState[userID]
	if groupChatID == 0 {
		log.Println("Group not set up for user:", userID)
		return nil
	}

	groupChat, _ := bot.ChatByID(groupChatID)
	if groupChat == nil {
		log.Printf("Failed to fetch group chat by ID: %d", groupChatID)
		return nil
	}

	groupChatName := groupChat.Title
	if groupChatName == "" {
		log.Printf("Failed to fetch group chat name by ID: %d", groupChatID)
		return nil
	}

    var params storage.VerificationParams

    // Parse JSON from the admin's message
	if err := json.Unmarshal([]byte(c.Text()), &params); err != nil {
		log.Printf("Failed to parse JSON: %v", err)
		bot.Send(c.Sender(), "Invalid JSON format. Please ensure your parameters match the expected structure.")
		return nil
	}

    // Validate required fields in parsed JSON
	if params.CircuitID == "" || params.ID == 0 || params.Query == nil {
		log.Println("JSON does not contain all required fields.")
		bot.Send(c.Sender(), "Missing required fields in JSON. Please include 'circuitId', 'id', and 'query'.")
		return nil
	}

	// Получение или инициализация конфигурации группы
	groupConfig, exists := storage.VerificationParamsMap[groupChatID]
	if !exists {
		groupConfig = storage.GroupVerificationConfig{
			VerificationParams: []storage.VerificationParams{},
			AcriveIndex:        -1, // Изначально ни один параметр не активен
		}
	}
	// Проверка: первый ли это параметр верификации
	isFirstParameter := len(groupConfig.VerificationParams) == 0

	// Добавление нового параметра верификации
	groupConfig.VerificationParams = append(groupConfig.VerificationParams, params)

	// Установка ActiveIndex в 0, если это первый параметр
	if isFirstParameter {
		groupConfig.AcriveIndex = 0
	}

	// Сохранение параметров в хранилище
	storage.VerificationParamsMap[groupChatID] = groupConfig
	log.Printf("Verification parameters set for group '%s': %+v", groupChatName, params)

	// Отправка сообщения в зависимости от количества параметров
	if isFirstParameter {
		bot.Send(c.Sender(), fmt.Sprintf("Verification parameters have been successfully set for the group '%s'.", groupChatName))
	} else {
		bot.Send(c.Sender(), "Additional verification parameters have been added.")
	}
	return nil
		
}

// Handler for /test_verification
func TestVerificationHandler(bot *telebot.Bot) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		userID := c.Sender().ID
		var groupChatID int64

		// Determine where the handler was called: in a group or in a private chat
		if c.Chat().Type == telebot.ChatPrivate {
			// Check if there is a saved group for this administrator
			if groupID, exists := storage.GroupSetupState[userID]; exists {
				groupChatID = groupID
			} else {
				return c.Send("You need to specify a group for verification setup.")
			}
		} else {
			groupChatID = c.Chat().ID
		}

		log.Println("Group Chat ID:", groupChatID)

		// Check if the user is an administrator of the group
		if !isAdmin(bot, groupChatID, userID) {
			return c.Send("You are not an administrator in this group.")
		}

		// Create a record for the admin's test verification
		adminUser := &storage.UserVerification{
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

		storage.AddOrUpdateUser(userID, adminUser)

		// Check if verification parameters are set for the group
		// params, exists := storage.VerificationParamsMap[groupChatID]
		// if !exists {
		// 	//storage.RemoveUser(userID) // Remove the test record if parameters are not set
		// 	return c.Send("Verification parameters have not been set for this group.")
		// }

		// Получение конфигурации группы
		groupConfig, exists := storage.VerificationParamsMap[groupChatID]
		if !exists {
			log.Printf("Verification parameters not found for group ID: %d", groupChatID)
			return c.Send("Verification parameters are not configured for your group.")
		}

		// Проверяем, что активный индекс корректен
		if groupConfig.AcriveIndex < 0 || groupConfig.AcriveIndex >= len(groupConfig.VerificationParams) {
			log.Printf("Invalid active index for group ID: %d", groupChatID)
			return c.Send("Verification configuration error. Please contact the group administrator.")
		}

		// Получаем активные параметры верификации
		params := groupConfig.VerificationParams[groupConfig.AcriveIndex]

		// Определяем тип текущей верификации
		verificationType := "unknown"
		if queryType, ok := params.Query["type"].(string); ok {
			verificationType = queryType
		}

		// Generate a test request for verification
		jsonData, err := auth.GenerateAuthRequest(userID, params)
		if err != nil {
			log.Printf("Error generating auth request: %v", err)
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
			log.Printf("Error sending verification message: %v", err)
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

		// Check if the group is set up for this user
		targetChatGroupID, exists := storage.GroupSetupState[userID]
		if !exists {
			return c.Send("You need to set up a group for verification.")
		}

		// Get chat data
		chat, err := bot.ChatByID(targetChatGroupID)
		if err != nil {
			log.Printf("Error fetching chat: %v", err)
			return c.Send("Failed to fetch chat information.")
		}
		targetChatGroupName := chat.Title

		// Get the list of verified users for the group
		storage.DataMutex.Lock()
		verifiedUsers, groupExists := storage.VerifiedUsersList[targetChatGroupID]
		storage.DataMutex.Unlock()

		// If the list for the group is empty or the group does not exist
		if !groupExists || len(verifiedUsers) == 0 {
			return c.Send(fmt.Sprintf("No verified users in the group '%s'.", targetChatGroupName))
		}

		// Forming a message with a list of verified users
		msg := fmt.Sprintf("Verified users in the group '%s':\n\n", targetChatGroupName)
		for _, verifiedUser := range verifiedUsers {
			msg += fmt.Sprintf("@%s - %s\n", verifiedUser.User.UserName, verifiedUser.TypeVerification)
		}

		// Send a message
		return c.Send(msg)
	}
}

// AddVerificationParamsHandler handles adding verification parameters in a single step
func AddVerificationParamsHandler(bot *telebot.Bot) func(c telebot.Context) error {
    return func(c telebot.Context) error {
        userID := c.Sender().ID

        // Check if the user is linked to a group setup
        groupChatID := storage.GroupSetupState[userID]
        if groupChatID == 0 {
            log.Println("Group not set up for user:", userID)
            return c.Send("You are not associated with any group. Use /setup first.")
        }

        groupChat, _ := bot.ChatByID(groupChatID)
        if groupChat == nil {
            log.Printf("Failed to fetch group chat by ID: %d", groupChatID)
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

// ListVerificationParamsHandler displays the list of added verification parameters
func ListVerificationParamsHandler(bot *telebot.Bot) func(c telebot.Context) error {
    return func(c telebot.Context) error {
        userID := c.Sender().ID

        // Check if the user is linked to a group setup
        groupChatID := storage.GroupSetupState[userID]
        if groupChatID == 0 {
            log.Println("Group not set up for user:", userID)
            return c.Send("You are not associated with any group. Use /setup first.")
        }

        // Fetch group configuration
        groupConfig, exists := storage.VerificationParamsMap[groupChatID]
        if !exists || len(groupConfig.VerificationParams) == 0 {
            log.Println("No verification parameters found for group:", groupChatID)
            return c.Send("No verification parameters have been added yet. Use /add_verification_params to add one.")
        }

        // Build the list of verification parameters
        var response strings.Builder
        response.WriteString("Verification parameters for the group:\n\n")
        for i, param := range groupConfig.VerificationParams {
            activeMarker := ""
            if i == groupConfig.AcriveIndex {
                activeMarker = " (active)"
            }

            // Extract the "type" field from the "query"
            queryType := "unknown" // Default value in case "type" is missing
            if query, ok := param.Query["type"]; ok {
                if queryStr, ok := query.(string); ok {
                    queryType = queryStr
                }
            }

            response.WriteString(fmt.Sprintf("%d. Type: %s%s\n", i+1, queryType, activeMarker))
        }

        // Send the list to the admin
        return c.Send(response.String())
    }
}

// Handler to switch active verification parameters
func SetActiveVerificationParamsHandler(bot *telebot.Bot) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		userID := c.Sender().ID
		var groupChatID int64

		// Determine where the handler was called: in a group or in a private chat
		if c.Chat().Type == telebot.ChatPrivate {
			// Check if there is a saved group for this administrator
			if groupID, exists := storage.GroupSetupState[userID]; exists {
				groupChatID = groupID
			} else {
				return c.Send("You need to specify a group for verification setup.")
			}
		} else {
			groupChatID = c.Chat().ID
		}

		// Check if the user is an administrator of the group
		if !isAdmin(bot, groupChatID, userID) {
			return c.Send("You are not an administrator in this group.")
		}

		// Fetch the group's verification parameters
		groupConfig, exists := storage.VerificationParamsMap[groupChatID]
		if !exists || len(groupConfig.VerificationParams) == 0 {
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
			if i == groupConfig.AcriveIndex {
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
				if groupConfig.AcriveIndex == index {
					_, err := bot.Send(c.Sender(), fmt.Sprintf(
						"The selected verification type '%s' is already active.",
						groupConfig.VerificationParams[index].Query["type"],
					))
					return err
				}

				// Update the active index
				groupConfig.AcriveIndex = index
				storage.VerificationParamsMap[groupChatID] = groupConfig

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
	if storage.GroupSetupState[userID] == groupID {
		return true
	} else {
		return false
	}
}

func GetAuthTokenFromAdmin(groupID int64, userID int64) (string, bool) {
	DataMutex.Lock()
	defer DataMutex.Unlock()

	// Check if the user list exists for the group
	users, exists := storage.VerifiedUsersList[groupID]
	if !exists {
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
