package main

import (
	"errors"
	"fmt"
	"github.com/chzyer/readline"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	port       int    = 11211
	host       string = "localhost"
	conn       net.Conn
	commandMap map[string]Command
	forFlush   bool
)

type Command func(...string) interface{}

// Get ...
func Get(k ...string) interface{} {
	if len(k) < 1 {
		return ""
	}
	cmd := "get " + k[0]
	r, _, _ := exec(cmd)
	return r
}

// Set set a key
func Set(k ...string) interface{} {
	if len(k) < 2 {
		return false
	}
	return true
}

// Add add a new key
func Add(k ...string) interface{} {
	if len(k) < 2 {
		return false
	}
	return true
}

// Del a key
func Del(k ...string) interface{} {
	if len(k) < 1 {
		return false
	}
	cmd := "del " + k[0]
	exec(cmd)
	return true
}

// Stats get stats
func Stats(p ...string) interface{} {
	if !IsConnected() {
		panic("connect error")
	}
	var c string
	if len(p) > 0 {
		c = "stats " + p[0]
	} else {
		c = "stats"
	}

	res, _, err := exec(c)

	if err != nil {
		return err
	}
	return res
}

// Keys find keys match the parameter
func Keys(k ...string) interface{} {
	var keys []string
	var search *regexp.Regexp
	var e error
	length := 0
	l := len(k)
	if l >= 1 {
		if len(k[0]) > 0 && k[0][0] != '*' {
			k[0] = "^" + k[0]
		}
		if strings.IndexByte(k[0], '*') >= 0 {
			k[0] = strings.Replace(k[0], "*", ".+", -1)
		}
		search, e = regexp.Compile(k[0])
		if e != nil {
			fmt.Println("condition invalid")
			return ""
		}
		if l > 1 {
			length, _ = strconv.Atoi(k[1])
		}
		ret, n, _ := exec("stats items")
		if n == 0 {
			return ""
		}
		lines := strings.Split(ret, "\n")
		idMap := make(map[string]int)
		for _, line := range lines {
			if strings.HasPrefix(line, "STAT") {
				tmp := strings.Split(line, ":")
				idMap[tmp[1]] = 1
			}
		}
		ll := 0
	STOP_FIND:
		for k, _ := range idMap {
			cmd := "stats cachedump " + k + " 0"
			r, nn, _ := exec(cmd)
			if nn == 0 {
				continue
			}
			itemsList := strings.Split(r, "\n")
			for _, item := range itemsList {
				if !strings.HasPrefix(item, "ITEM") {
					continue
				}
				t := strings.Split(item, " ")
				if search != nil && !search.Match([]byte(t[1])) {
					continue
				}
				keys = append(keys, t[1])
				ll++
				if length != 0 && ll > l {
					break STOP_FIND
				}
			}
		}
	}
	if !forFlush {
		keysString := strings.Join(keys, "\n")
		return keysString
	}
	return keys
}

// Flush keys
func Flush(k ...string) interface{} {
	if len(k) != 1 {
		return false
	}
	forFlush = true
	r := Keys(k...)
	forFlush = false
	for _, k := range r.([]string) {
		Del(k)
	}
	return true
}

// exec send a command to server and receive the response
func exec(c string) (ret string, n int, err error) {
	c += "\r\n"
	if _, e := conn.Write([]byte(c)); err != nil {
		err = e
		return
	}
	for {
		b := make([]byte, 256)
		l, e := conn.Read(b)
		n += l
		ret += string(b)
		if e != nil || l < len(b) {
			break
		}
	}
	return
}

// GetInput input terminal
func GetInput(addr string) (string, error) {
	prpt := fmt.Sprintf("(%s) > ", addr)
	rl, err := readline.NewEx(&readline.Config{Prompt: prpt, HistoryFile: "/tmp/mem_cli_history.tmp"})
	defer rl.Close()
	if err != nil {
		return "", err
	}
	line, _ := rl.Readline()
	return line, nil
}

func main() {
	var err error
	switch len(os.Args) {
	case 2:
		args := strings.Split(os.Args[1], ":")
		host = args[0]
		if len(args) >= 2 {
			port, err = strconv.Atoi(args[1])
			if err != nil {
				fmt.Println("port invalid")
				os.Exit(1)
			}
		} else {
			port = 11211
		}
	case 3:
		host = os.Args[1]
		port, err = strconv.Atoi(os.Args[2])
		if err != nil {
			fmt.Print("port invalid")
			os.Exit(2)
		}
	}

	commandMap = make(map[string]Command)
	commandMap["get"] = Get
	commandMap["set"] = Set
	commandMap["add"] = Add
	commandMap["del"] = Del
	commandMap["stats"] = Stats
	commandMap["keys"] = Keys
	commandMap["flush"] = Flush

	cre, _ := regexp.Compile("\\s+")

	addr := fmt.Sprintf("%s:%d", host, port)
	err = ConnectToServer(addr)
	if err != nil {
		if nerr, ok := err.(net.Error); ok {
			if nerr.Timeout() {
				fmt.Println("connect timeout")
			} else {
				fmt.Println("unknow error: " + nerr.Error())
			}
		} else {
			fmt.Println("unknow error: " + err.Error())
		}
		goto EXIT
	}
	defer conn.Close()
STOPLOOP:
	for {
		in, err := GetInput(addr)
		if err != nil {
			fmt.Println("error: " + err.Error())
			break
		}
		params := cre.Split(in, -1)
		if len(params) < 1 || len(params[0]) < 1 {
			continue
		}
		switch strings.ToLower(params[0]) {
		case "quit", "exit":
			break STOPLOOP
		default:
			c := params[0]
			if cmd, ok := commandMap[c]; !ok {
				fmt.Println("command not support")
			} else {
				r := cmd(params[1:]...)
				fmt.Println(r)
			}
		}
	}
EXIT:
	fmt.Println("Connection closed, Bye.")
}

// ConnectToServer create a connection to th giving server
func ConnectToServer(addr string) error {
	var err error
	conn, err = net.DialTimeout("tcp", addr, time.Second*5)
	if err != nil {
		if nerr, ok := err.(net.Error); ok {
			if nerr.Timeout() {
				return errors.New("timeout")
			} else {
				return nerr
			}
		} else {
			return errors.New("unknow error")
		}
	}
	return nil
}

// IsConnected check the connection to the memcached server is available
func IsConnected() bool {
	return conn != nil
}
