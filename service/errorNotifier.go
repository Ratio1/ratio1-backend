package service

import "fmt"

func notifyError(message string, err error, fields ...ErrorEmailField) {
	if err != nil {
		log.Error("%s: %v", message, err)
	} else {
		log.Error("%s", message)
	}

	if emailErr := SendErrorEmail(message, err, fields...); emailErr != nil {
		log.Error("failed to send backend error alert email: %v", emailErr)
	}
}

func intField(v int) string {
	return fmt.Sprintf("%d", v)
}

func int64Field(v int64) string {
	return fmt.Sprintf("%d", v)
}
