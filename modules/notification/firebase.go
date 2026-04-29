package notification

import (
	"context"
	"errors"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/BarisNKorkmaz/taskManager/database"
	"google.golang.org/api/option"
)

var MessagingClient *messaging.Client

func InitFirebase(path string) error {

	opt := option.WithCredentialsFile(path)
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		return err
	}

	if MessagingClient, err = app.Messaging(context.Background()); err != nil {
		return err
	}

	if MessagingClient == nil {
		return errors.New("Failed initializing message client")
	}

	return nil
}

func SendPushToToken(token string, userId uint, title string, body string) (string, error) {

	if MessagingClient == nil || token == "" {
		return "", errors.New("messaging client not initialized or token is empty")
	}
	message := messaging.Message{
		Token: token,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
	}

	id, err := MessagingClient.Send(context.Background(), &message)
	if err != nil {
		if messaging.IsUnregistered(err) {
			database.DeactivateDeviceToken(database.DB, token, 3, &DeviceToken{})
		}
		return "", err
	}

	return id, nil
}
