package service

import "fmt"

// notifyError is the central helper used by background flows to report failures.
// It always logs immediately, then queues a dedicated error email task.
func notifyError(message string, err error, fields ...ErrorEmailField) {
	if err != nil {
		log.Error("%s: %v", message, err)
	} else {
		log.Error("%s", message)
	}
	// Copy variadic fields to decouple queued task execution from caller-side slice reuse.
	fieldsCopy := append([]ErrorEmailField(nil), fields...)
	EnqueueEmailTask(EmailTask{
		Name: "send_error_email",
		Execute: func() error {
			return SendErrorEmail(message, err, fieldsCopy...)
		},
	})
}

func intField(v int) string {
	return fmt.Sprintf("%d", v)
}

func int64Field(v int64) string {
	return fmt.Sprintf("%d", v)
}
