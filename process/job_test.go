package process

import (
	"fmt"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
)

func Test_Job(t *testing.T) {
	location, _ := time.LoadLocation("Europe/Bucharest")
	c := cron.New(cron.WithLocation(location))

	cronExpression := "59 15 * * 4" // 15:59 (3:59 PM) on Thursdays
	_, err := c.AddFunc(cronExpression, myJob)
	if err != nil {
		fmt.Println("Error adding cron job:", err)
		return
	}

	c.Start()

	// Keep the program running
	select {}
}

func myJob() {
	fmt.Println("This is my job")
}
