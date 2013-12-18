package quickbase

import (
	"testing"
	"fmt"
	"github.com/kless/goconfig/config"
)

func read_config(path string, t *testing.T) (string, string, string) {
	c, err := config.ReadDefault(path)
	if err != nil {
		t.Error(err.Error())
		return "", "", ""
	}
	url, err := c.String("connection", "url")
	if err != nil {
		t.Error(err.Error())
		return "", "", ""
	}
	username, err := c.String("connection", "username")
	if err != nil {
		t.Error(err.Error())
		return "", "", ""
	}
	password, err := c.String("connection", "password")
	if err != nil {
		t.Error(err.Error())
		return "", "", ""
	}
	return url, username, password
}

func TestAuthentication(t *testing.T) {
	println("foobaz")
	url, username, password := read_config("/home/buhl/.quickbase", t)
	ticket, err := Authenticate(url, username, password)
	if err != nil {
		fmt.Println(err)
		t.Error(err.Error())
		return
	}
	_ = ticket
}
