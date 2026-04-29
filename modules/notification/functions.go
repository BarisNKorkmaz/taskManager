package notification

import (
	"fmt"
	"time"

	"github.com/BarisNKorkmaz/taskManager/database"
	"github.com/BarisNKorkmaz/taskManager/middleware"
	"github.com/BarisNKorkmaz/taskManager/modules/task"
	"github.com/robfig/cron/v3"
)

func SendDailyTaskReminderPushes() (fatalErr error, errors []error) {

	var tokens []DeviceToken
	users := make(map[uint]map[string]any)

	if tx := database.FetchActiveDeviceTokens(&DeviceToken{}, &tokens); tx.Error != nil {
		middleware.Log.Error("failed fetch active device tokens", "err", tx.Error.Error())
		return tx.Error, nil
	}

	for _, token := range tokens {
		var count int

		if mapLen, err := task.GenerateDailyOccs(token.UserID); err != nil {
			middleware.Log.Error("Generate daily occs failed", "err", err, "userId", token.UserID)
			errors = append(errors, err)
			continue
		} else {
			count = mapLen
		}

		users[token.UserID] = map[string]any{
			"deviceToken": token.Token,
			"occCount":    count,
		}

	}

	for key, value := range users {
		if _, err := SendPushToToken(value["deviceToken"].(string), key, "Good Morning!", fmt.Sprintf("You have %d tasks waiting for you today. Let’s get started and make progress.", value["occCount"])); err != nil {
			middleware.Log.Error("failed push notification", "err", err, "userId", key)
			errors = append(errors, err)
			continue
		}
	}

	if len(errors) == 0 {
		errors = nil
		fatalErr = nil
	}
	return
}

func SendWeeklyReportNotification() (errors []error) {
	var tokens []DeviceToken

	if tx := database.FetchActiveDeviceTokens(&DeviceToken{}, &tokens); tx.Error != nil {
		middleware.Log.Error("failed fetch active device tokens", "err", tx.Error.Error())
		errors = append(errors, tx.Error)
		return
	}

	for _, token := range tokens {
		if _, err := SendPushToToken(token.Token, token.UserID, "Your Weekly Report is Ready!", "See how you performed this week"); err != nil {
			errors = append(errors, err)
			continue
		}
	}

	if len(errors) == 0 {
		errors = nil
	}

	return errors
}

func StartScheduler() {
	location := time.Now().Location()

	c := cron.New(cron.WithLocation(location))

	_, err := c.AddFunc("0 7 * * *", func() {
		middleware.Log.Info("CRON daily report func is starting.. ")

		err, errs := SendDailyTaskReminderPushes()
		if err != nil {
			middleware.Log.Error("CRON FATAl daily push notification can't sended", "err", err)
		}
		if errs != nil {
			middleware.Log.Error("CRON daily push notifications errors", "errs", errs)
		}

		middleware.Log.Info("CRON completed")

	})

	if err != nil {
		middleware.Log.Error("CRON add function error", "err", err)
		return
	}

	_, err = c.AddFunc("0 1 * * 1", func() {
		middleware.Log.Info("CRON weekly report func is starting..")
		if errs := SendWeeklyReportNotification(); errs != nil {
			middleware.Log.Error("CRON weekly push notifications errors", "errs", errs)
		}
	})

	if err != nil {
		middleware.Log.Error("CRON add function error", "err", err)
		return
	}

	c.Start()
	middleware.Log.Info("CRON scheduler successfully started..")

}
