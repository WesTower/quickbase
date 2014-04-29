package quickbase_test

import (
	"testing"
	"fmt"
	"os"
	"github.com/WesTower/freezing-avenger/lib/quickbase"
)

func TestAuthentication(t *testing.T) {
	if _, err := authenticate(); err != nil {
		fmt.Println(err)
		t.Error(err.Error())
		return
	}
}

func authenticate() (ticket quickbase.Ticket, err error) {
	return quickbase.Authenticate(os.Getenv("QUICKBASE_URL"),
		os.Getenv("QUICKBASE_USERNAME"),
		os.Getenv("QUICKBASE_PASSWORD"))
}
