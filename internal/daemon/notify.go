package daemon

// Notify shows a desktop notification, best effort.
//
// Every platform implementation shells out to something already present on the
// system rather than taking a dependency, and none of them is allowed to fail
// a cycle: the log is the reliable channel, the notification is the courtesy
// on top of it.
func Notify(title, message string) {
	if err := notify(title, message); err != nil {
		Log("notification failed: %v", err)
	}
}
