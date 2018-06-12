package main // import "vimagination.zapto.org/messenger/cmd/messenger"

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"

	"vimagination.zapto.org/messenger"
)

func e(explain string, err error) {
	if err != nil {
		cc <- struct{}{}
		<-cc
		fmt.Fprintf(os.Stderr, "%s: %s\n", explain, err)
		os.Exit(3)
	}
}

type Config struct {
	Client             *messenger.Client
	SaveCredentials    bool
	Username, Password string
	Aliases            map[uint]string
}

var (
	cc = make(chan struct{})
)

func main() {

	go func() {
		sc := make(chan os.Signal, 1)
		signal.Notify(sc, os.Interrupt)
		var end bool
		select {
		case <-sc:
			end = true
		case <-cc:
		}
		UI.Quit()
		signal.Stop(sc)
		close(sc)
		close(cc)
		if end {
			os.Exit(0)
		}
	}()

	e("error initialising ncurses", UI.Init())

	var configFile string
	usr, err := user.Current()
	e("error getting user information", err)
	flag.StringVar(&configFile, "config", filepath.Join(usr.HomeDir, ".messengerConfig"), "path to configuration file")
	flag.Parse()
	var config Config
	f, err := os.Open(configFile)
	if !os.IsNotExist(err) {
		e("error opening configuration file", err)
		e("error decoding configuration file", json.NewDecoder(f).Decode(&config))
		e("error closing config file (reading)", f.Close())
	}

	if config.Client != nil {
		if err = config.Client.Resume(); err == messenger.ErrInvalidCookies {
			config.Client = nil
		} else if err != nil {
			e("error validating cookies", err)
		}
	}
	if config.Client == nil && config.Username != "" {
		config.Client, err = messenger.Login(config.Username, config.Password)
		if err != messenger.ErrInvalidLogin {
			e("error logging in with saved credentials", err)
		}
	}
	if config.Client == nil {
		var username, password string
		for {
			username, password, err = UI.GetUserPass()
			e("error getting username/password: ", err)
			config.Client, err = messenger.Login(username, password)
			if err == messenger.ErrInvalidLogin {
				UI.ShowError("Invalid Login Credentials")
				continue
			}
			e("error logging in", err)
			break
		}
		if config.SaveCredentials {
			config.Username = username
			config.Password = password
		}
	}
	f, err = os.Create(configFile)
	e("error opening config file for writing", err)
	e("error encoding config file", json.NewEncoder(f).Encode(config))
	e("error closing config file (writing)", f.Close())

	cc <- struct{}{}
	<-cc
}
