package anonymous

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

type Command string

const (
	StartCommand           Command = "start"
	InfoCommand            Command = "info"
	LinkCommand            Command = "link"
	Username               Command = "username"
	TextMessage            Command = "text"
	ReplyCallback          Command = "reply-callback"
	OpenCallback           Command = "open-callback"
	SetUsernameCallback    Command = "set-username-callback"
	RemoveUserNameCallback Command = "remove-username-callback"
	CancelUserNameCallback Command = "cancel-username-callback"
)

type RootHandler struct {
	user     User
	userRepo UserRepository
}

func NewRootHandler() *RootHandler {
	return &RootHandler{}
}

func (r *RootHandler) init(commandName Command) handlers.Response {
	return func(b *gotgbot.Bot, ctx *ext.Context) error {
		return r.runCommand(b, ctx, commandName)
	}
}

func (r *RootHandler) runCommand(b *gotgbot.Bot, ctx *ext.Context, command Command) error {
	// create user repo
	userRepo, err := NewUserRepository()
	if err != nil {
		return fmt.Errorf("failed to init db repo: %w", err)
	}
	user, err := r.processUser(userRepo, ctx)

	if err != nil || user == nil {
		return fmt.Errorf("failed to process user: %w", err)
	}
	r.user = *user
	r.userRepo = *userRepo

	// Decide which function to call based on the command
	switch command {
	case StartCommand:
		return r.start(b, ctx)
	case InfoCommand:
		return r.info(b, ctx)
	case LinkCommand:
		return r.getLink(b, ctx)
	case Username:
		return r.manageUsername(b, ctx)
	case TextMessage:
		return r.processText(b, ctx)
	case ReplyCallback:
		return r.replyCallback(b, ctx)
	case OpenCallback:
		return r.openCallback(b, ctx)
	case SetUsernameCallback:
		return r.usernameCallback(b, ctx, "SET")
	case RemoveUserNameCallback:
		return r.usernameCallback(b, ctx, "REMOVE")
	case CancelUserNameCallback:
		return r.usernameCallback(b, ctx, "CANCEL")
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func (r *RootHandler) processUser(userRepo *UserRepository, ctx *ext.Context) (*User, error) {
	user, err := userRepo.readUserByUserId(ctx.EffectiveUser.Id)
	if err != nil {
		user, err = userRepo.createUser(ctx.EffectiveUser.Id)
		if err != nil {
			return nil, err
		}
	}

	return user, nil
}

func (r *RootHandler) start(b *gotgbot.Bot, ctx *ext.Context) error {
	args := ctx.Args()
	if len(args) == 1 && args[0] == "/start" {
		// Reset user state
		err := r.userRepo.resetUserState(r.user.UUID)
		if err != nil {
			return err
		}

		_, err = b.SendMessage(ctx.EffectiveChat.Id, "Welcome! Use /link command to get you link!", &gotgbot.SendMessageOpts{})
		if err != nil {
			return fmt.Errorf("failed to send bot info: %w", err)
		}
		return nil
	}
	if len(args) == 2 && args[0] == "/start" {

		var err error
		var receiverUser *User
		var identity string

		if strings.HasPrefix(args[1], "_") {
			username := args[1][1:]
			receiverUser, err = r.userRepo.readUserByUsername(username)
		} else {
			receiverUser, err = r.userRepo.readUserByUUID(args[1])
		}

		if err != nil || receiverUser == nil {
			_, err = b.SendMessage(ctx.EffectiveChat.Id, "User not found! Wrong link?", &gotgbot.SendMessageOpts{})
			if err != nil {
				return fmt.Errorf("failed to send bot info: %w", err)
			}
			return nil
		}

		if receiverUser.UUID == r.user.UUID {
			_, err = b.SendMessage(ctx.EffectiveChat.Id, "Do you really want to talk to yourself? So sad! try /random command to connect to someone else!", &gotgbot.SendMessageOpts{})
			if err != nil {
				return fmt.Errorf("failed to send bot info: %w", err)
			}
			return nil
		}

		// Set user state to sending
		err = r.userRepo.updateUser(r.user.UUID, map[string]interface{}{
			"State":       Sending,
			"ContactUUID": receiverUser.UUID,
		})
		if err != nil {
			return fmt.Errorf("failed to update user state: %w", err)
		}

		if receiverUser.Name != "" {
			identity = receiverUser.Name
		} else if receiverUser.Username != "" {
			identity = receiverUser.Username
		} else {
			identity = receiverUser.UUID
		}

		_, err = b.SendMessage(ctx.EffectiveChat.Id, fmt.Sprintf("You are sending message to:\n%s\n\nEnter your message:", identity), &gotgbot.SendMessageOpts{})
		if err != nil {
			return fmt.Errorf("failed to send bot info: %w", err)
		}
	}

	return nil
}

func (r *RootHandler) info(b *gotgbot.Bot, ctx *ext.Context) error {
	_, err := b.SendMessage(ctx.EffectiveChat.Id, "Bugfloyd Anonymous bot", &gotgbot.SendMessageOpts{})
	if err != nil {
		return fmt.Errorf("failed to send bot info: %w", err)
	}
	err = r.userRepo.resetUserState(r.user.UUID)
	if err != nil {
		return err
	}
	return nil
}

func (r *RootHandler) getLink(b *gotgbot.Bot, ctx *ext.Context) error {
	var link string
	if r.user.Username != "" {
		usernameLink := fmt.Sprintf("https://t.me/%s?start=_%s", b.User.Username, r.user.Username)
		uuidLink := fmt.Sprintf("https://t.me/%s?start=%s", b.User.Username, r.user.UUID)
		link = fmt.Sprintf("%s\n\nor:\n\n%s", usernameLink, uuidLink)
	} else {
		link = fmt.Sprintf("https://t.me/%s?start=%s", b.User.Username, r.user.UUID)
	}
	_, err := ctx.EffectiveMessage.Reply(b, link, nil)
	if err != nil {
		return err
	}
	err = r.userRepo.resetUserState(r.user.UUID)
	if err != nil {
		return err
	}
	return nil
}

func (r *RootHandler) processText(b *gotgbot.Bot, ctx *ext.Context) error {
	switch r.user.State {
	case Sending:
		return r.sendAnonymousMessage(b, ctx)
	case SettingUsername:
		return r.setUsername(b, ctx)
	default:
		return r.sendError(b, ctx, "Unknown Command")
	}
}

func (r *RootHandler) sendError(b *gotgbot.Bot, ctx *ext.Context, message string) error {
	errorMessage := fmt.Sprintf("Error: %s", message)
	_, err := ctx.EffectiveMessage.Reply(b, errorMessage, nil)
	if err != nil {
		return fmt.Errorf("failed to send error message: %w", err)
	}
	return nil
}

func (r *RootHandler) sendAnonymousMessage(b *gotgbot.Bot, ctx *ext.Context) error {
	receiver, err := r.userRepo.readUserByUUID(r.user.ContactUUID)
	if err != nil {
		return fmt.Errorf("failed to get receiver: %w", err)
	}

	var replyParameters *gotgbot.ReplyParameters
	msgText := "You have a new message."
	if r.user.ReplyMessageID != 0 {
		replyParameters = &gotgbot.ReplyParameters{
			MessageId:                r.user.ReplyMessageID,
			AllowSendingWithoutReply: true,
		}

		msgText = "New reply to your message."
	}

	// Reply to the sender
	deliveryMessage, err := ctx.EffectiveMessage.Reply(b, "Message sent", nil)
	if err != nil {
		return fmt.Errorf("failed to send message to sender: %w", err)
	}

	// Send the new message notification to the receiver
	_, err = b.SendMessage(receiver.UserID, msgText, &gotgbot.SendMessageOpts{
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
				{
					{
						Text:         "Open Message",
						CallbackData: fmt.Sprintf("o|%s|%d|%d", r.user.UUID, ctx.EffectiveMessage.MessageId, deliveryMessage.MessageId),
					},
				},
			},
		},
		ReplyParameters: replyParameters,
	})
	if err != nil {
		return fmt.Errorf("failed to send message to receiver: %w", err)
	}

	// Delete temp message has been sent from sender's chat
	if r.user.DeliveryMessageID != 0 {
		_, err = b.DeleteMessage(receiver.UserID, r.user.DeliveryMessageID, &gotgbot.DeleteMessageOpts{})
		if err != nil {
			fmt.Printf("failed to delete sender's temp message: %s", err)
		}
	}

	// Reset sender user
	err = r.userRepo.resetUserState(r.user.UUID)
	if err != nil {
		return err
	}

	return nil
}

func (r *RootHandler) openCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	cb := ctx.Update.CallbackQuery
	split := strings.Split(cb.Data, "|")
	if len(split) != 4 {
		return fmt.Errorf("invalid callback data: %s", cb.Data)
	}
	uuid := split[1]
	sender, err := r.userRepo.readUserByUUID(uuid)
	if err != nil {
		return fmt.Errorf("failed to get receiver: %w", err)
	}

	// Send callback answer to telegram
	_, err = cb.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
		Text: "Message opened!",
	})
	if err != nil {
		return fmt.Errorf("failed to answer callback: %w", err)
	}

	sendersDeliveryMessageID, err := strconv.ParseInt(split[3], 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse sender's message ID: %w", err)
	}

	// Copy the sender's message to the receiver
	senderMessageID, err := strconv.ParseInt(split[2], 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse message ID: %w", err)
	}
	var replyMessageID int64
	if ctx.EffectiveMessage.ReplyToMessage != nil {
		replyMessageID = ctx.EffectiveMessage.ReplyToMessage.MessageId
	}
	_, err = b.CopyMessage(ctx.EffectiveChat.Id, sender.UserID, senderMessageID, &gotgbot.CopyMessageOpts{
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
				{
					{
						Text:         "Reply",
						CallbackData: fmt.Sprintf("r|%s|%d|%d", sender.UUID, senderMessageID, sendersDeliveryMessageID),
					},
				},
			},
		},
		ReplyParameters: &gotgbot.ReplyParameters{
			MessageId:                replyMessageID,
			AllowSendingWithoutReply: true,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send message to receiver: %w", err)
	}

	// Edit delivery message in sender's chat: Sent -> Opened
	_, _, err = b.EditMessageText("Your message have been seen", &gotgbot.EditMessageTextOpts{
		ChatId:    sender.UserID,
		MessageId: sendersDeliveryMessageID,
	})
	if err != nil {
		fmt.Println("failed to edit delivery message: %w", err)
	}

	// react with eyes emoji to senderMessageID
	_, err = b.SetMessageReaction(sender.UserID, senderMessageID, &gotgbot.SetMessageReactionOpts{
		Reaction: []gotgbot.ReactionType{
			gotgbot.ReactionTypeEmoji{
				Emoji: "👀",
			},
		},
		IsBig: true,
	})

	if err != nil {
		fmt.Println("failed to react to sender's message: %w", err)
	}

	// Delete message with "Open" button
	_, err = cb.Message.Delete(b, &gotgbot.DeleteMessageOpts{})
	if err != nil {
		fmt.Println("failed to delete message: %w", err)
	}

	return nil
}

func (r *RootHandler) replyCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	cb := ctx.Update.CallbackQuery
	split := strings.Split(cb.Data, "|")
	if len(split) != 4 {
		return fmt.Errorf("invalid callback data: %s", cb.Data)
	}
	receiverUUID := split[1]
	messageID, err := strconv.ParseInt(split[2], 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse message ID: %w", err)
	}
	sendersDeliveryMessageID, err := strconv.ParseInt(split[3], 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse message ID: %w", err)
	}

	// Store the message id in the user and set status to replying
	err = r.userRepo.updateUser(r.user.UUID, map[string]interface{}{
		"State":             Sending,
		"ContactUUID":       receiverUUID,
		"ReplyMessageID":    messageID,
		"DeliveryMessageID": sendersDeliveryMessageID,
	})
	if err != nil {
		return fmt.Errorf("failed to update user state: %w", err)
	}

	// Send callback answer to telegram
	_, err = cb.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
		Text: "Replying to message...",
	})
	if err != nil {
		return fmt.Errorf("failed to answer callback: %w", err)
	}

	// Send reply instruction
	_, err = ctx.EffectiveMessage.Reply(b, "Reply to this message:", nil)
	if err != nil {
		return fmt.Errorf("failed to send reply message: %w", err)
	}

	return nil
}

func (r *RootHandler) manageUsername(b *gotgbot.Bot, ctx *ext.Context) error {
	var text string
	var buttons [][]gotgbot.InlineKeyboardButton

	if r.user.Username != "" {
		text = fmt.Sprintf("Your current username is: %s", r.user.Username)
		buttons = [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         "Change",
					CallbackData: "u",
				},
				{
					Text:         "Remove",
					CallbackData: "ru",
				},
				{
					Text:         "Cancel",
					CallbackData: "cu",
				},
			},
		}
	} else {
		text = "You don't have a username!"
		buttons = [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         "Set one",
					CallbackData: "u",
				},
				{
					Text:         "Cancel",
					CallbackData: "cu",
				},
			},
		}
	}

	_, err := b.SendMessage(ctx.EffectiveChat.Id, text, &gotgbot.SendMessageOpts{
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: buttons,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send username info: %w", err)
	}

	return nil
}

func (r *RootHandler) usernameCallback(b *gotgbot.Bot, ctx *ext.Context, action string) error {
	cb := ctx.Update.CallbackQuery

	// Remove username command buttons
	_, _, err := cb.Message.EditReplyMarkup(b, &gotgbot.EditMessageReplyMarkupOpts{})
	if err != nil {
		return fmt.Errorf("failed to update username message markup: %w", err)
	}

	if action == "CANCEL" {
		// Send callback answer to telegram
		_, err = cb.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text: "Never mind!",
		})
		if err != nil {
			return fmt.Errorf("failed to answer callback: %w", err)
		}
		// Reset sender user
		err = r.userRepo.resetUserState(r.user.UUID)
		if err != nil {
			return err
		}
	} else if action == "SET" {
		err := r.userRepo.updateUser(r.user.UUID, map[string]interface{}{
			"State":             SettingUsername,
			"ContactUUID":       nil,
			"ReplyMessageID":    nil,
			"DeliveryMessageID": nil,
		})
		if err != nil {
			return fmt.Errorf("failed to update user state: %w", err)
		}

		// Send reply instruction
		_, err = ctx.EffectiveMessage.Reply(b, "Create a username that starts with a letter, includes 3-20 characters, and may contain letters, numbers, or underscores (_). Usernames are automatically converted to lowercase. \n\nEnter new username:", nil)
		if err != nil {
			return fmt.Errorf("failed to send reply message: %w", err)
		}

		// Send callback answer to telegram
		_, err = cb.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text: "Setting username...",
		})
		if err != nil {
			return fmt.Errorf("failed to answer callback: %w", err)
		}
	} else if action == "REMOVE" {
		err := r.userRepo.updateUser(r.user.UUID, map[string]interface{}{
			"State":             Idle,
			"Username":          nil,
			"ContactUUID":       nil,
			"ReplyMessageID":    nil,
			"DeliveryMessageID": nil,
		})
		if err != nil {
			return fmt.Errorf("failed to remove username: %w", err)
		}

		_, _, err = cb.Message.EditText(b, "Username has been removed!", &gotgbot.EditMessageTextOpts{})
		if err != nil {
			return fmt.Errorf("failed to update username message text: %w", err)
		}

		// Send callback answer to telegram
		_, err = cb.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
			Text: "Username removed!",
		})
		if err != nil {
			return fmt.Errorf("failed to answer callback: %w", err)
		}
	}

	return nil
}

func (r *RootHandler) setUsername(b *gotgbot.Bot, ctx *ext.Context) error {
	username := ctx.EffectiveMessage.Text

	if isValidUsername(username) == false {
		// Send username instruction
		_, err := ctx.EffectiveMessage.Reply(b, "The entered username is not valid. Enter another one:", nil)
		if err != nil {
			return fmt.Errorf("failed to send reply message: %w", err)
		}
		return nil
	}

	// Convert to lowercase
	username = strings.ToLower(username)

	existingUser, err := r.userRepo.readUserByUsername(username)
	if err != nil || existingUser == nil {
		err := r.userRepo.updateUser(r.user.UUID, map[string]interface{}{
			"Username":          username,
			"State":             Idle,
			"ContactUUID":       nil,
			"ReplyMessageID":    nil,
			"DeliveryMessageID": nil,
		})
		if err != nil {
			return fmt.Errorf("failed to update username: %w", err)
		}

		// Send username instruction
		_, err = ctx.EffectiveMessage.Reply(b, fmt.Sprintf("Username has been set: %s", username), nil)
		if err != nil {
			return fmt.Errorf("failed to send reply message: %w", err)
		}
	} else {
		var text string
		if existingUser.UUID != r.user.UUID {
			text = "The entered username exists. Enter another one:"
		} else {
			text = "You already own this username silly! If you want to change it, run the username command once more!"

			// Reset sender user
			err = r.userRepo.resetUserState(r.user.UUID)
			if err != nil {
				return err
			}
		}
		// Send username instruction
		_, err = ctx.EffectiveMessage.Reply(b, text, nil)
		if err != nil {
			return fmt.Errorf("failed to send reply message: %w", err)
		}
	}

	return nil
}

func isValidUsername(username string) bool {
	// Check length
	if len(username) < 3 || len(username) > 20 {
		return false
	}

	// Regular expression to check valid characters
	// ^[a-zA-Z0-9_]+$
	// This checks the string consists only of English letters, digits, and underscores
	re := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	if !re.MatchString(username) {
		return false
	}

	// Check first character (not a digit or underscore)
	firstChar := username[0]
	if firstChar == '_' || ('0' <= firstChar && firstChar <= '9') {
		return false
	}

	return true
}
