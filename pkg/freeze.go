package pkg

import (
	"fmt"
	"time"
)

func freezeDatabase() error {
	fmt.Println("Freezing Database.......")

	time.Sleep(30 * time.Second)

	fmt.Println("Database has been frozen successfully")
	return nil
}
