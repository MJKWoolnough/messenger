package main

import "github.com/rthornton128/goncurses"

var UI ui

type ui struct {
	window *goncurses.Window
}

func (u *ui) Init() error {
	var err error
	u.window, err = goncurses.Init()
	return err
}

func (u *ui) GetUserPass() (string, string, error) {
	u.window.Printf("Enter Username: ")
	username, err := u.window.GetString(50)
	if err != nil {
		return "", "", err
	}
	u.window.Printf("Enter Password: ")
	goncurses.Echo(false)
	password, err := u.window.GetString(50)
	if err != nil {
		return "", "", err
	}
	goncurses.Echo(true)
	u.window.Println()
	return username, password, nil
}

func (u *ui) ShowError(err string) {
	u.window.Clear()
	u.window.Println(err)
}

func (u *ui) Quit() {
	if u.window != nil {
		goncurses.End()
	}
}
