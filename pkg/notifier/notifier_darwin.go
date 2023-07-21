package notify

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
	"time"

	"github.com/getlantern/notifier/osx"
	"github.com/skratchdot/open-golang/open"
)

func newNotifier() (Notifier, error) {
	dir, err := ioutil.TempDir("", "terminal-notifier")
	if err != nil {
		return nil, err
	}
	if err := osx.RestoreAssets(dir, "terminal-notifier.app"); err != nil {
		return nil, err
	}
	fullPath := dir + "/terminal-notifier.app/Contents/MacOS/terminal-notifier"
	return &darwinNotifier{path: fullPath}, nil
}

type darwinNotifier struct {
	path string
}

// Notify sends a notification to the user.
func (n *darwinNotifier) Notify(msg *Notification) error {
	timeout := msg.AutoDismissAfter
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	args := []string{
		"-message", msg.Message,
		"-title", msg.Title,
		"-timeout", fmt.Sprintf("%d", int(timeout.Seconds())),
	}
	if msg.Sender != "" {
		args = append(args, "-sender", msg.Sender)
		// override IconURL
		msg.IconURL = ""
	}
	if msg.ClickURL != "" {
		label := msg.ClickLabel
		if label == "" {
			label = "Open"
		}
		args = append(args, "-actions", label)
	}
	if msg.IconURL != "" {
		args = append(args, "-appIcon", msg.IconURL)
	}
	cmd := exec.Command(n.path, args...)
	res, err := cmd.CombinedOutput()
	if err != nil {
		log.Errorf("Could not run command %w", err)
		return err
	}
	result := string(res)
	log.Debugf("Received result: %v", result)

	if msg.OnClick != nil {
		msg.OnClick(result)
	}

	// Note we can't just look for the result being the string "Open" here because
	// it's the label on the button and can be in any language.
	if msg.ClickURL != "" && result != "" && !strings.Contains(result, "@") {
		open.Start(msg.ClickURL)
	}
	return nil
}
