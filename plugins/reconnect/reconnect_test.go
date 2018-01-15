package reconnect_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/jinzhu/gorm/plugins/reconnect"
	"github.com/jinzhu/gorm/tests"
)

func TestReconnect(t *testing.T) {
	DB, err := tests.OpenTestConnection()
	DB.Use(reconnect.New(nil))

	if err != nil {
		t.Error(err)
	}

	for {
		fmt.Println("111")
		var user User
		fmt.Println(DB.Find(&user))
		time.Sleep(time.Second)
	}
}

type User struct {
	gorm.Model
	Name string
}
