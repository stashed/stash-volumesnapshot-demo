package pkg

import (
	"fmt"
	"time"
)

func resumeDatabase() error {
	fmt.Println("Resuming Database.......")

	time.Sleep(30 * time.Second)

	fmt.Println("Database has been resumed successfully")
	return nil
}
