package notify

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNotify(t *testing.T) {
	n := NewNotifications()

	msg := &Notification{
		Title:    "Your Lantern time is up",
		Message:  "You have reached your data cap limit",
		Sender:   "com.getlantern.lantern",
		ClickURL: "https://www.getlantern.org",
		IconURL:  "https://www.getlantern.org/static/images/favicon.png",
	}

	err := n.Notify(msg)
	assert.Nil(t, err, "got an error notifying user")
	time.Sleep(3 * time.Second)
}
