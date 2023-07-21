package main

import (
	"time"

	"github.com/getlantern/notifier"
)

func main() {
	n := notify.NewNotifications()

	msg := &notify.Notification{
		Title:    "Super Important",
		Message:  "Free the Internet",
		Sender:   "com.getlantern.lantern",
		ClickURL: "https://www.getlantern.org",
		IconURL:  "https://www.getlantern.org/static/images/favicon.png",
	}

	n.Notify(msg)
	time.Sleep(3 * time.Second)
}
